package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"wilson/agent/chat"
	"wilson/config"
	"wilson/core/registry"
	chatinterface "wilson/interface/chat"
	"wilson/ollama"
	"wilson/session"
	"wilson/setup"
	"wilson/ui"

	_ "wilson/capabilities/code_intelligence/analysis"
	_ "wilson/capabilities/code_intelligence/ast"
	_ "wilson/capabilities/code_intelligence/build"
	_ "wilson/capabilities/code_intelligence/quality"
	_ "wilson/capabilities/context"
	_ "wilson/capabilities/filesystem"
	_ "wilson/capabilities/git"
	_ "wilson/capabilities/orchestration"
	_ "wilson/capabilities/system"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Warning: failed to load config, using defaults: %v\n", err)
		cfg = config.Get() // Get default config
	}

	// Bootstrap Wilson system
	bootstrap, err := setup.Initialize(ctx, cfg)
	if err != nil {
		fmt.Printf("Startup failed: %v\n", err)
		return
	}
	defer bootstrap.Cleanup()

	if err := ollama.Init(ctx, cfg.Ollama.Model); err != nil {
		fmt.Println("Startup failed:", err)
		return
	}
	defer ollama.Shutdown()

	// Clean startup banner
	fmt.Println("\n=== Wilson ===")

	// Show active models
	if cfg.LLMs != nil && len(cfg.LLMs) > 0 {
		fmt.Println("Active models:")
		for name, llmCfg := range cfg.LLMs {
			fmt.Printf("  %s: %s\n", name, llmCfg.Model)
		}
	}

	fmt.Printf("\nType -help to see available tools\n")
	fmt.Println("What can I help you with?")

	// Create chat interface
	chatUI := chatinterface.NewInterface()

	// Create executor for tool execution
	executor := registry.NewExecutor()

	// Set up status handler for tool execution
	executor.StatusHandler = func(toolName string, phase string) {
		switch phase {
		case "executing":
			chatUI.ShowThinking(fmt.Sprintf("Executing: %s", toolName))
		case "completed", "error":
			chatUI.ClearStatus()
		}
	}

	// Create conversation history (20 turns = 40 messages max)
	history := session.NewHistory(20)

	// Create chat handler with the agent
	chatHandler := chat.NewChatHandler(bootstrap.ChatAgent, history, executor)

	// Track completed tasks for notifications
	completedTasks := make(map[string]bool)

	// Chat loop
	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			fmt.Println("\nGraceful shutdown.")
			return
		default:
		}

		// Check for completed background tasks and notify
		completedTasks = chatUI.CheckAndNotifyCompletedTasks(completedTasks)

		// Read user input
		userInput, err := chatUI.ReadInput()
		if err != nil {
			chatUI.DisplayError(err)
			return
		}

		// Check for EOF
		if userInput == "" {
			break
		}

		// Check for exit commands
		if userInput == "exit" || userInput == "quit" {
			chatHandler.ClearHistory()
			fmt.Println("Goodbye!")
			return
		}

		// Check for help command
		if userInput == "-help" || userInput == "--help" || userInput == "help" {
			ui.PrintToolHelp(registry.GetEnabledTools())
			continue
		}

		// Set user query for audit logging
		executor.SetUserQuery(userInput)

		// Show thinking indicator
		chatUI.ShowThinking("Wilson is thinking...")

		// Process the request through chat handler
		response, err := chatHandler.HandleChat(ctx, userInput)

		// Clear status
		chatUI.ClearStatus()

		if err != nil {
			chatUI.DisplayError(err)
		} else if response != nil && response.Success {
			// Display tool execution status if tool was used
			if response.ToolUsed != "" {
				if response.ToolCancelled {
					chatUI.DisplayToolExecution(response.ToolUsed, "cancelled")
				} else {
					chatUI.DisplayToolExecution(response.ToolUsed, "completed")
				}
			}

			// Display response if we have one
			if response.Text != "" {
				chatUI.DisplayResponse(response.Text)
			}
		}

		chatUI.PrintSeparator()
	}
}
