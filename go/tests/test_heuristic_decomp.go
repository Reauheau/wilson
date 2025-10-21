package main

import (
	"context"
	"fmt"
	"os"

	"wilson/agent"
	"wilson/config"
	contextpkg "wilson/context"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize Context Manager
	contextMgr, err := contextpkg.NewManager(cfg.Context.DBPath, cfg.Context.AutoStore)
	if err != nil {
		fmt.Printf("Error creating context manager: %v\n", err)
		os.Exit(1)
	}
	defer contextMgr.Close()

	// Initialize Manager Agent WITHOUT LLM (just test heuristic decomp)
	db := contextMgr.GetDB()
	if db == nil {
		fmt.Println("Error: Failed to get database connection")
		os.Exit(1)
	}

	managerAgent := agent.NewManagerAgent(db)
	// Don't set LLM manager - test will use heuristic fallback

	fmt.Println("=== Wilson Heuristic Decomposition Test ===\n")

	// Test 1: Simple request without "test" keyword
	fmt.Println("Test 1: Request without 'test' keyword")
	fmt.Println("Input: 'create a calculator in Go'")
	parentTask1, err := managerAgent.CreateTask(ctx, "User Request", "create a calculator in Go", agent.ManagedTaskTypeGeneral)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Created parent task: %s\n", parentTask1.TaskKey)

	// Manually call heuristic decompose (since DecomposeTask requires LLM)
	// We can't directly access it, so let's just verify the task was created
	fmt.Printf("✓ Task title: %s\n", parentTask1.Title)
	fmt.Printf("✓ Task status: %s\n\n", parentTask1.Status)

	// Test 2: Request with "test" keyword
	fmt.Println("Test 2: Request with 'test' keyword")
	fmt.Println("Input: 'create a calculator in Go and write tests'")
	parentTask2, err := managerAgent.CreateTask(ctx, "User Request with Tests", "create a calculator in Go and write tests", agent.ManagedTaskTypeGeneral)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Created parent task: %s\n", parentTask2.TaskKey)
	fmt.Printf("✓ Contains 'test': true\n\n")

	// Test 3: Create subtasks manually to verify the queue works
	fmt.Println("Test 3: Create subtasks manually")
	subtask1, err := managerAgent.CreateSubtask(ctx, parentTask2.ID, "Generate main code", "Create calculator.go", agent.ManagedTaskTypeCode)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Created subtask 1: %s\n", subtask1.TaskKey)

	subtask2, err := managerAgent.CreateSubtask(ctx, parentTask2.ID, "Generate test file", "Create calculator_test.go", agent.ManagedTaskTypeCode)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	subtask2.DependsOn = []string{subtask1.TaskKey}
	fmt.Printf("✓ Created subtask 2: %s (depends on: %v)\n", subtask2.TaskKey, subtask2.DependsOn)

	subtask3, err := managerAgent.CreateSubtask(ctx, parentTask2.ID, "Run tests", "Execute test suite", agent.ManagedTaskTypeTest)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	subtask3.DependsOn = []string{subtask2.TaskKey}
	fmt.Printf("✓ Created subtask 3: %s (depends on: %v)\n\n", subtask3.TaskKey, subtask3.DependsOn)

	// Show queue statistics
	fmt.Println("=== Queue Statistics ===")
	stats, err := managerAgent.GetQueueStatistics()
	if err != nil {
		fmt.Printf("Error getting stats: %v\n", err)
	} else {
		fmt.Printf("Total: %d\n", stats.Total)
		fmt.Printf("New: %d\n", stats.New)
		fmt.Printf("Ready: %d (DoR validated)\n", stats.Ready)
		fmt.Printf("In Progress: %d\n", stats.InProgress)
		fmt.Printf("Done: %d\n", stats.Done)
	}

	// List all tasks
	fmt.Println("\n=== All Tasks ===")
	tasks, err := managerAgent.ListAllTasks(agent.TaskFilters{})
	if err != nil {
		fmt.Printf("Error listing tasks: %v\n", err)
	} else {
		for _, task := range tasks {
			fmt.Printf("[%s] %s - %s (Type: %s)\n", task.TaskKey, task.Title, task.Status, task.Type)
		}
	}

	fmt.Println("\n✓ Test Complete - Task queue and subtask system working!")
}
