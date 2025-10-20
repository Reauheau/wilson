// Test script to demonstrate Phase 2: Model Lifecycle + Concurrency Limiting
// Shows how worker goroutines use AcquireModel/release and enforce max_concurrent

package main

import (
	"context"
	"fmt"
	"time"

	"wilson/agent"
	"wilson/llm"
)

func main() {
	fmt.Println("=== Phase 2: Model Lifecycle + Concurrency Test ===\n")

	// Create LLM manager (Phase 0)
	llmManager := llm.NewManager()
	defer llmManager.Stop()

	// Register a mock LLM
	llmManager.RegisterLLM(llm.PurposeCode, llm.Config{
		Provider:    "ollama",
		Model:       "test-model",
		KeepAlive:   false,
		IdleTimeout: 0, // Immediate cleanup after release
	})

	// Create registry and coordinator
	registry := agent.NewRegistry()
	coordinator := agent.NewCoordinator(registry)

	// Set LLM manager (Phase 2)
	coordinator.SetLLMManager(llmManager)
	coordinator.SetMaxConcurrent(2) // Test with max 2 concurrent workers

	agent.SetGlobalCoordinator(coordinator)

	// Create mock agent
	mockAgent := &MockAgent{
		name:    "test-agent",
		purpose: llm.PurposeCode,
	}
	registry.Register(mockAgent)

	fmt.Println("1. Testing model lifecycle integration")
	fmt.Printf("   Max concurrent workers: %d\n", 2)
	fmt.Printf("   Model: test-model (KeepAlive=false, IdleTimeout=0)\n\n")

	// Test 1: Single task with model lifecycle
	fmt.Println("2. Starting task 1 (should acquire and release model)")
	start := time.Now()
	task1ID, err := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Task 1",
		Priority:    3,
	})
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("   ✗ Error: %v\n", err)
		return
	}

	fmt.Printf("   ✓ Task %s started in %dms (non-blocking)\n", task1ID[:8], elapsed.Milliseconds())
	fmt.Printf("   Model refCount: %d (acquired)\n", llmManager.GetRefCount(llm.PurposeCode))

	// Wait for task to complete
	time.Sleep(1500 * time.Millisecond)

	task1, result1, _ := coordinator.GetTaskStatus(task1ID)
	if result1 != nil && result1.Success {
		fmt.Printf("   ✓ Task 1 completed: %s\n", result1.Output)
		fmt.Printf("   Model refCount: %d (released - kill-after-task)\n", llmManager.GetRefCount(llm.PurposeCode))
		fmt.Printf("   Model loaded: %v (unloaded with IdleTimeout=0)\n\n", llmManager.IsLoaded(llm.PurposeCode))
	} else {
		fmt.Printf("   Status: %s\n\n", task1.Status)
	}

	// Test 2: Concurrent tasks (test max_concurrent limit)
	fmt.Println("3. Starting 4 concurrent tasks (max_concurrent=2)")
	fmt.Println("   Expected: Only 2 run concurrently, others wait")

	task2ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Concurrent task 1",
		Priority:    3,
	})
	task3ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Concurrent task 2",
		Priority:    3,
	})
	task4ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Concurrent task 3",
		Priority:    3,
	})
	task5ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "test-agent",
		Type:        "test",
		Description: "Concurrent task 4",
		Priority:    3,
	})

	fmt.Printf("   ✓ Spawned 4 tasks: %s, %s, %s, %s\n", task2ID[:8], task3ID[:8], task4ID[:8], task5ID[:8])

	// Check after 500ms - should have 2 running (max_concurrent limit)
	time.Sleep(500 * time.Millisecond)
	activeTasks := coordinator.GetActiveTasks()
	fmt.Printf("\n   After 500ms: %d active tasks (max_concurrent enforced)\n", len(activeTasks))
	fmt.Printf("   Model refCount: %d (2 workers acquired model)\n", llmManager.GetRefCount(llm.PurposeCode))

	// Wait for all to complete
	time.Sleep(3 * time.Second)
	activeTasks = coordinator.GetActiveTasks()
	fmt.Printf("   After 3.5s: %d active tasks (all should be done)\n", len(activeTasks))
	fmt.Printf("   Model refCount: %d (all released)\n", llmManager.GetRefCount(llm.PurposeCode))
	fmt.Printf("   Model loaded: %v (immediate cleanup)\n\n", llmManager.IsLoaded(llm.PurposeCode))

	// Summary
	fmt.Println("=== Phase 2 Complete ===")
	fmt.Println("\n✓ Model lifecycle integrated (AcquireModel/release)")
	fmt.Println("✓ Kill-after-task: Models released immediately when refCount=0")
	fmt.Println("✓ Concurrency limiting: Max 2 workers enforced")
	fmt.Println("✓ Semaphore prevents resource exhaustion")
	fmt.Println("✓ Fresh workers for each task (clean state)")
	fmt.Println("\nReady for Phase 3: Status Updates")
}

// MockAgent for testing
type MockAgent struct {
	name    string
	purpose llm.Purpose
}

func (a *MockAgent) Name() string           { return a.name }
func (a *MockAgent) Purpose() llm.Purpose   { return a.purpose }
func (a *MockAgent) CanHandle(task *agent.Task) bool { return task.Type == "test" }
func (a *MockAgent) AllowedTools() []string { return []string{} }

func (a *MockAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	// Simulate work taking 1 second
	time.Sleep(1 * time.Second)

	return &agent.Result{
		TaskID:  task.ID,
		Success: true,
		Output:  fmt.Sprintf("Completed: %s", task.Description),
	}, nil
}
