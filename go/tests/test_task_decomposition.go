package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"wilson/agent"
	"wilson/config"
	contextpkg "wilson/context"
	"wilson/llm"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize LLM Manager
	llmMgr := llm.NewManager()
	if cfg.LLMs != nil {
		for name, llmCfg := range cfg.LLMs {
			var purpose llm.Purpose
			switch name {
			case "chat":
				purpose = llm.PurposeChat
			case "analysis":
				purpose = llm.PurposeAnalysis
			case "code":
				purpose = llm.PurposeCode
			default:
				continue
			}

			config := llm.Config{
				Provider:    llmCfg.Provider,
				Model:       llmCfg.Model,
				Temperature: llmCfg.Temperature,
				BaseURL:     llmCfg.BaseURL,
				APIKey:      llmCfg.APIKey,
				Fallback:    llmCfg.Fallback,
				Options:     llmCfg.Options,
			}

			if err := llmMgr.RegisterLLM(purpose, config); err != nil {
				fmt.Printf("Warning: Failed to register %s LLM: %v\n", name, err)
			}
		}
	}

	// Initialize Context Manager
	contextMgr, err := contextpkg.NewManager(cfg.Context.DBPath, cfg.Context.AutoStore)
	if err != nil {
		fmt.Printf("Error creating context manager: %v\n", err)
		os.Exit(1)
	}
	defer contextMgr.Close()

	// Create agent registry
	agentRegistry := agent.NewRegistry()

	// Register agents
	chatAgent := agent.NewChatAgent(llmMgr, contextMgr)
	analysisAgent := agent.NewAnalysisAgent(llmMgr, contextMgr)
	codeAgent := agent.NewCodeAgent(llmMgr, contextMgr)
	testAgent := agent.NewTestAgent(llmMgr, contextMgr)
	reviewAgent := agent.NewReviewAgent(llmMgr, contextMgr)

	agentRegistry.Register(chatAgent)
	agentRegistry.Register(analysisAgent)
	agentRegistry.Register(codeAgent)
	agentRegistry.Register(testAgent)
	agentRegistry.Register(reviewAgent)

	// Initialize Manager Agent
	db := contextMgr.GetDB()
	if db == nil {
		fmt.Println("Error: Failed to get database connection")
		os.Exit(1)
	}

	managerAgent := agent.NewManagerAgent(db)
	managerAgent.SetLLMManager(llmMgr)
	managerAgent.SetRegistry(agentRegistry)

	fmt.Println("=== Wilson Task Decomposition Test ===\n")

	// Test 1: Simple request without "test" keyword
	fmt.Println("Test 1: Decompose 'create a calculator in Go'")
	parentTask1, subtasks1, err := managerAgent.DecomposeTask(ctx, "create a calculator in Go")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Created parent task: %s\n", parentTask1.TaskKey)
		fmt.Printf("✓ Created %d subtasks:\n", len(subtasks1))
		for i, task := range subtasks1 {
			fmt.Printf("  %d. [%s] %s (depends on: %v)\n", i+1, task.Type, task.Title, task.DependsOn)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Test 2: Request with "test" keyword
	fmt.Println("\nTest 2: Decompose 'create a calculator in Go and write tests'")
	parentTask2, subtasks2, err := managerAgent.DecomposeTask(ctx, "create a calculator in Go and write tests")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Created parent task: %s\n", parentTask2.TaskKey)
		fmt.Printf("✓ Created %d subtasks:\n", len(subtasks2))
		for i, task := range subtasks2 {
			fmt.Printf("  %d. [%s] %s (depends on: %v)\n", i+1, task.Type, task.Title, task.DependsOn)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Test 3: Request with "test" + "build" keywords
	fmt.Println("\nTest 3: Decompose 'create a calculator, write tests, and build'")
	parentTask3, subtasks3, err := managerAgent.DecomposeTask(ctx, "create a calculator, write tests, and build")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Created parent task: %s\n", parentTask3.TaskKey)
		fmt.Printf("✓ Created %d subtasks:\n", len(subtasks3))
		for i, task := range subtasks3 {
			fmt.Printf("  %d. [%s] %s (depends on: %v)\n", i+1, task.Type, task.Title, task.DependsOn)
		}
	}

	fmt.Println("\n=== Task Decomposition Test Complete ===")

	// Show statistics
	stats, err := managerAgent.GetQueueStatistics()
	if err == nil {
		fmt.Printf("\nQueue Statistics:\n")
		fmt.Printf("  Total: %d\n", stats.Total)
		fmt.Printf("  New: %d\n", stats.New)
		fmt.Printf("  Ready: %d\n", stats.Ready)
		fmt.Printf("  In Progress: %d\n", stats.InProgress)
		fmt.Printf("  Done: %d\n", stats.Done)
		fmt.Printf("  Failed: %d\n", stats.Failed)
	}
}
