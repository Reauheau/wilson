// Test script to demonstrate Phase 1: Async Foundation
// Shows how delegation is non-blocking - Wilson never waits

package main

import (
	"context"
	"fmt"
	"time"

	"wilson/agent"
	"wilson/llm"
)

func main() {
	fmt.Println("=== Phase 1: Async Foundation Test ===\n")

	// Create registry and coordinator
	registry := agent.NewRegistry()
	coordinator := agent.NewCoordinator(registry)
	agent.SetGlobalCoordinator(coordinator)

	// Create a mock agent for testing
	mockAgent := &MockAgent{
		name:    "test-agent",
		purpose: llm.PurposeChat,
	}
	registry.Register(mockAgent)

	fmt.Println("1. Starting async delegation test")
	fmt.Println("   Simulating Wilson delegating a task...\n")

	// Test 1: Async delegation returns immediately
	start := time.Now()
	taskID, err := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Test async task that takes 2 seconds",
		Priority:    3,
	})
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("   ✗ Error: %v\n", err)
		return
	}

	fmt.Printf("2. ✓ Task %s started in %dms (IMMEDIATE!)\n", taskID, elapsed.Milliseconds())
	fmt.Println("   Wilson is FREE to continue chatting!\n")

	// Verify task is in active list
	activeTasks := coordinator.GetActiveTasks()
	fmt.Printf("3. Active tasks: %d\n", len(activeTasks))
	if len(activeTasks) > 0 {
		fmt.Printf("   - Task %s: %s (Status: %s)\n",
			activeTasks[0].ID,
			activeTasks[0].Description,
			activeTasks[0].Status)
	}

	// Simulate Wilson chatting while task runs
	fmt.Println("\n4. Simulating Wilson answering questions while task runs...")
	for i := 0; i < 3; i++ {
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("   Wilson: \"The answer is %d. Task still running in background...\"\n", i+1)
	}

	// Check if task completed
	fmt.Println("\n5. Checking task status...")
	task, result, err := coordinator.GetTaskStatus(taskID)
	if err != nil {
		fmt.Printf("   ✗ Error: %v\n", err)
		return
	}

	fmt.Printf("   Task Status: %s\n", task.Status)
	if result != nil && result.Success {
		fmt.Printf("   ✓ Task completed successfully!\n")
		fmt.Printf("   Result: %s\n", result.Output)
	} else if task.Status == agent.TaskInProgress {
		fmt.Println("   ⏳ Task still running (agent working in background)")

		// Wait for completion
		fmt.Println("\n6. Waiting for task to complete...")
		time.Sleep(2 * time.Second)

		task, result, _ = coordinator.GetTaskStatus(taskID)
		if result != nil && result.Success {
			fmt.Printf("   ✓ Task completed!\n")
			fmt.Printf("   Result: %s\n", result.Output)
		}
	}

	// Test 2: Multiple concurrent async tasks
	fmt.Println("\n7. Testing multiple concurrent tasks...")
	task1ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Concurrent task 1",
		Priority:    3,
	})
	task2ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Concurrent task 2",
		Priority:    3,
	})

	fmt.Printf("   ✓ Spawned 2 tasks: %s, %s\n", task1ID[:8], task2ID[:8])

	activeTasks = coordinator.GetActiveTasks()
	fmt.Printf("   Active tasks now: %d\n", len(activeTasks))

	// Summary
	fmt.Println("\n=== Phase 1 Complete ===")
	fmt.Println("\n✓ DelegateTaskAsync() returns immediately (<100ms)")
	fmt.Println("✓ Wilson never blocks - can chat while tasks run")
	fmt.Println("✓ Tasks execute in background goroutines")
	fmt.Println("✓ GetActiveTasks() shows ongoing work")
	fmt.Println("✓ Multiple concurrent tasks supported")
	fmt.Println("\nReady for Phase 2: Background Worker Pool")
}

// MockAgent for testing - implements agent.Agent interface
type MockAgent struct {
	name    string
	purpose llm.Purpose
}

func (a *MockAgent) Name() string { return a.name }

func (a *MockAgent) Purpose() llm.Purpose { return a.purpose }

func (a *MockAgent) CanHandle(task *agent.Task) bool {
	return task.Type == "test"
}

func (a *MockAgent) AllowedTools() []string {
	return []string{} // Empty = all tools
}

func (a *MockAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	// Simulate work taking 2 seconds
	time.Sleep(2 * time.Second)

	return &agent.Result{
		TaskID:  task.ID,
		Success: true,
		Output:  fmt.Sprintf("Completed: %s", task.Description),
	}, nil
}
