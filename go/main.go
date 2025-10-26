package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"wilson/agent"
	"wilson/agent/agents"
	"wilson/agent/chat"
	"wilson/agent/orchestration"
	"wilson/config"
	contextpkg "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
	chatinterface "wilson/interface/chat"
	"wilson/llm"
	"wilson/lsp"
	"wilson/mcp"
	"wilson/ollama"
	"wilson/session"

	code_intelligence "wilson/capabilities/code_intelligence" // Code generation (needs SetLLMManager)
	_ "wilson/capabilities/code_intelligence/analysis"        // Code intelligence: Analysis tools (Phase 3)
	_ "wilson/capabilities/code_intelligence/ast"             // Code intelligence: AST tools (Phase 1)
	_ "wilson/capabilities/code_intelligence/build"           // Code intelligence: Build tools (Phase 2)
	_ "wilson/capabilities/code_intelligence/quality"         // Code intelligence: Quality tools (Phase 4)
	_ "wilson/capabilities/context"                           // Context management tools
	_ "wilson/capabilities/filesystem"                        // Filesystem tools
	_ "wilson/capabilities/orchestration"                     // Multi-agent orchestration tools
	_ "wilson/capabilities/system"                            // System tools
	"wilson/capabilities/web"                                 // Web tools (need SetLLMManager)
)

func printHelp(tools []Tool) {
	fmt.Println("\n=== Available Tools ===")

	// Group by category
	categories := make(map[ToolCategory][]Tool)
	for _, tool := range tools {
		meta := tool.Metadata()
		categories[meta.Category] = append(categories[meta.Category], tool)
	}

	// Print by category
	categoryOrder := []ToolCategory{"filesystem", "web", "context", "agent", "system"}
	for _, cat := range categoryOrder {
		if tools, ok := categories[cat]; ok && len(tools) > 0 {
			fmt.Printf("\n%s (%d):\n", cat, len(tools))
			for _, tool := range tools {
				meta := tool.Metadata()
				fmt.Printf("  %-20s %s\n", meta.Name, meta.Description)
			}
		}
	}
	fmt.Println()
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Warning: failed to load config, using defaults: %v\n", err)
		cfg = config.Get() // Get default config
	}

	// Initialize LLM Manager
	llmMgr := initializeLLMManager(ctx, cfg)
	if llmMgr == nil {
		fmt.Println("Warning: Failed to initialize LLM manager, some features may not work")
	}

	// Initialize LSP Manager for code intelligence
	lspManager := lsp.NewManager()
	code_intelligence.SetLSPManager(lspManager)
	defer lspManager.StopAll()

	// Initialize Context Store (silent)
	contextMgr := initializeContextManager(cfg)
	if contextMgr != nil {
		defer contextMgr.Close()
	}

	// Initialize MCP Client (Phase 1)
	mcpClient := initializeMCPClient(ctx, cfg)
	if mcpClient != nil {
		defer mcpClient.Close()
	}

	// Initialize Agent System (silent)
	chatAgent := initializeAgentSystem(llmMgr, contextMgr)

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
	fmt.Println("What can I help you with?\n")

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
	chatHandler := chat.NewChatHandler(chatAgent, history, executor)

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
			printHelp(registry.GetEnabledTools())
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

// initializeLLMManager creates and configures the LLM manager
func initializeLLMManager(ctx context.Context, cfg *config.Config) *llm.Manager {
	manager := llm.NewManager()

	// Register LLMs from configuration
	if cfg.LLMs != nil {
		for name, llmCfg := range cfg.LLMs {
			// Convert config name to Purpose
			var purpose llm.Purpose
			switch name {
			case "chat":
				purpose = llm.PurposeChat
			case "analysis":
				purpose = llm.PurposeAnalysis
			case "code":
				purpose = llm.PurposeCode
			case "vision":
				purpose = llm.PurposeVision
			default:
				fmt.Printf("Warning: Unknown LLM purpose '%s', skipping\n", name)
				continue
			}

			// Create LLM config
			config := llm.Config{
				Provider:    llmCfg.Provider,
				Model:       llmCfg.Model,
				Temperature: llmCfg.Temperature,
				BaseURL:     llmCfg.BaseURL,
				APIKey:      llmCfg.APIKey,
				Fallback:    llmCfg.Fallback,
				Options:     llmCfg.Options,
			}

			// Register the LLM (silent)
			if err := manager.RegisterLLM(purpose, config); err != nil {
				fmt.Printf("Warning: Failed to register %s LLM: %v\n", name, err)
			}
		}
	}

	// Set the LLM manager for tools that need it
	web.SetLLMManager(manager)
	code_intelligence.SetLLMManager(manager)

	return manager
}

// initializeMCPClient creates and initializes the MCP client
func initializeMCPClient(ctx context.Context, cfg *config.Config) *mcp.Client {
	if !cfg.MCP.Enabled {
		return nil
	}

	client := mcp.NewClient(cfg.MCP)
	if err := client.Initialize(ctx); err != nil {
		fmt.Printf("Warning: Failed to initialize MCP client: %v\n", err)
		return nil
	}

	// List available tools
	tools := client.ListTools()
	if len(tools) > 0 {
		// Register MCP tools with Wilson's tool registry
		registerMCPTools(client)

		// Show clean summary
		serverNames := client.GetServerNames()
		if len(serverNames) > 0 {
			fmt.Printf("[MCP] Active: %s (%d tools)\n", strings.Join(serverNames, ", "), len(tools))
		}
	}

	return client
}

// registerMCPTools registers all MCP tools with Wilson's registry
func registerMCPTools(client *mcp.Client) {
	tools := client.ListTools()

	for _, mcpTool := range tools {
		// Create a bridge for each MCP tool
		bridge := mcp.NewMCPToolBridge(client, mcpTool.ServerName, mcpTool)

		// Register with Wilson's tool registry (silent)
		registry.Register(bridge)
	}
}

// initializeContextManager creates and configures the context manager
func initializeContextManager(cfg *config.Config) *contextpkg.Manager {
	if !cfg.Context.Enabled {
		return nil
	}

	// Create context manager
	manager, err := contextpkg.NewManager(cfg.Context.DBPath, cfg.Context.AutoStore)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize context store: %v\n", err)
		return nil
	}

	// Set as global manager
	contextpkg.SetGlobalManager(manager)

	// Create or get default session context (silent)
	if cfg.Context.DefaultContext != "" {
		sessionKey := fmt.Sprintf("%s-%s", cfg.Context.DefaultContext, time.Now().Format("2006-01-02"))
		_, err := manager.GetOrCreateContext(
			sessionKey,
			contextpkg.TypeSession,
			fmt.Sprintf("Session %s", time.Now().Format("2006-01-02")),
		)
		if err != nil {
			fmt.Printf("Warning: Failed to create default context: %v\n", err)
		} else {
			manager.SetActiveContext(sessionKey)
		}
	}

	return manager
}

// initializeAgentSystem creates and configures the agent system (silent)
func initializeAgentSystem(llmMgr *llm.Manager, contextMgr *contextpkg.Manager) *agents.ChatAgent {
	if llmMgr == nil || contextMgr == nil {
		return nil
	}

	// Create agent registry
	agentRegistry := agent.NewRegistry()

	// Register agents
	chatAgent := agents.NewChatAgent(llmMgr, contextMgr)
	analysisAgent := agents.NewAnalysisAgent(llmMgr, contextMgr)
	codeAgent := agents.NewCodeAgent(llmMgr, contextMgr)
	testAgent := agents.NewTestAgent(llmMgr, contextMgr)
	reviewAgent := agents.NewReviewAgent(llmMgr, contextMgr)

	_ = agentRegistry.Register(chatAgent)
	_ = agentRegistry.Register(analysisAgent)
	_ = agentRegistry.Register(codeAgent)
	_ = agentRegistry.Register(testAgent)
	_ = agentRegistry.Register(reviewAgent)

	// Create coordinator
	coordinator := orchestration.NewCoordinator(agentRegistry)

	// Set LLM manager for model lifecycle (Phase 2)
	coordinator.SetLLMManager(llmMgr)

	// Initialize Manager Agent with task queue
	// Use same database as context store for tasks
	db := contextMgr.GetDB()
	if db != nil {
		managerAgent := orchestration.NewManagerAgent(db)
		managerAgent.SetLLMManager(llmMgr)
		managerAgent.SetRegistry(agentRegistry)
		coordinator.SetManager(managerAgent)

		// ✅ START FEEDBACK PROCESSING (Phase 1)
		managerAgent.StartFeedbackProcessing(context.Background())
	}

	// Configure max concurrent workers (default: 2 for 16GB RAM)
	// coordinator.SetMaxConcurrent(2) // Can be configured via config.yaml

	// Set globals
	agent.SetGlobalRegistry(agentRegistry)
	orchestration.SetGlobalCoordinator(coordinator)

	return chatAgent
}
