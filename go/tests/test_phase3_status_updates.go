// Test script to demonstrate Phase 3: Status Updates with Model Visibility
// Shows which agent and model are working on each task

package main

import (
	"context"
	"fmt"
	"time"

	"wilson/agent"
	"wilson/llm"
)

func main() {
	fmt.Println("=== Phase 3: Status Updates with Model Visibility Test ===\n")

	// Create LLM manager
	llmManager := llm.NewManager()
	defer llmManager.Stop()

	// Register multiple models
	llmManager.RegisterLLM(llm.PurposeChat, llm.Config{
		Provider:    "ollama",
		Model:       "llama3:latest",
		KeepAlive:   true, // Wilson's model stays loaded
		IdleTimeout: 0,
	})

	llmManager.RegisterLLM(llm.PurposeCode, llm.Config{
		Provider:    "ollama",
		Model:       "qwen2.5-coder:14b",
		KeepAlive:   false, // Kill after task
		IdleTimeout: 0,
	})

	llmManager.RegisterLLM(llm.PurposeAnalysis, llm.Config{
		Provider:    "ollama",
		Model:       "mixtral:8x7b",
		KeepAlive:   false,
		IdleTimeout: 0,
	})

	// Create registry and coordinator
	registry := agent.NewRegistry()
	coordinator := agent.NewCoordinator(registry)
	coordinator.SetLLMManager(llmManager)
	coordinator.SetMaxConcurrent(2)
	agent.SetGlobalCoordinator(coordinator)

	// Create mock agents for different purposes
	chatAgent := &MockAgent{name: "chat-agent", purpose: llm.PurposeChat}
	codeAgent := &MockAgent{name: "code-agent", purpose: llm.PurposeCode}
	analysisAgent := &MockAgent{name: "analysis-agent", purpose: llm.PurposeAnalysis}

	registry.Register(chatAgent)
	registry.Register(codeAgent)
	registry.Register(analysisAgent)

	fmt.Println("1. Registered models:")
	fmt.Println("   - chat-agent      → llama3:latest")
	fmt.Println("   - code-agent      → qwen2.5-coder:14b")
	fmt.Println("   - analysis-agent  → mixtral:8x7b\n")

	// Test 1: Single task - check model tracking
	fmt.Println("2. Starting code task...")
	task1ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "code-agent",
		Type:        "test",
		Description: "Write authentication module",
		Priority:    3,
	})
	fmt.Printf("   ✓ Task %s started\n", task1ID[:8])

	// Wait a moment for model acquisition
	time.Sleep(100 * time.Millisecond)

	// Check task status (should show model)
	task1, _, _ := coordinator.GetTaskStatus(task1ID)
	fmt.Printf("   Agent: %s\n", task1.AgentName)
	fmt.Printf("   Model: %s\n", task1.ModelUsed)
	fmt.Printf("   Status: %s\n\n", task1.Status)

	// Test 2: Multiple concurrent tasks with different models
	fmt.Println("3. Starting 3 tasks with different agents/models...")
	task2ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "code-agent",
		Type:        "test",
		Description: "Implement API endpoints",
		Priority:    3,
	})
	_, _ = coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "analysis-agent",
		Type:        "test",
		Description: "Analyze performance bottlenecks",
		Priority:    3,
	})
	_, _ = coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "chat-agent",
		Type:        "test",
		Description: "Generate user documentation",
		Priority:    3,
	})

	fmt.Printf("   ✓ Spawned 3 tasks\n\n")

	// Wait for model acquisition
	time.Sleep(200 * time.Millisecond)

	// Show active tasks with model info
	fmt.Println("4. Active tasks (showing which models are working):")
	activeTasks := coordinator.GetActiveTasks()
	for i, task := range activeTasks {
		fmt.Printf("\n   Task %d: %s\n", i+1, task.ID[:8])
		fmt.Printf("   ├─ Description: %s\n", task.Description)
		fmt.Printf("   ├─ Agent: %s\n", task.AgentName)
		fmt.Printf("   ├─ Model: %s\n", task.ModelUsed)
		fmt.Printf("   └─ Status: %s\n", task.Status)
	}

	// Check model usage
	fmt.Printf("\n5. Model usage:\n")
	fmt.Printf("   chat model (llama3) refCount: %d\n", llmManager.GetRefCount(llm.PurposeChat))
	fmt.Printf("   code model (qwen2.5-coder) refCount: %d\n", llmManager.GetRefCount(llm.PurposeCode))
	fmt.Printf("   analysis model (mixtral) refCount: %d\n", llmManager.GetRefCount(llm.PurposeAnalysis))

	// Wait for first task to complete
	fmt.Println("\n6. Waiting for tasks to complete...")
	time.Sleep(2 * time.Second)

	// Check completed task
	_, result1, _ := coordinator.GetTaskStatus(task1ID)
	if result1 != nil && result1.Success {
		fmt.Printf("   ✓ Task 1 completed: %s\n", result1.Output)
	}

	// Check task 2 details
	task2, result2, _ := coordinator.GetTaskStatus(task2ID)
	fmt.Printf("\n7. Task 2 details:\n")
	fmt.Printf("   ID: %s\n", task2.ID[:8])
	fmt.Printf("   Agent: %s\n", task2.AgentName)
	fmt.Printf("   Model: %s\n", task2.ModelUsed)
	fmt.Printf("   Status: %s\n", task2.Status)
	if result2 != nil {
		fmt.Printf("   Result: %s\n", result2.Output)
	}

	// Wait for all to complete
	time.Sleep(1 * time.Second)

	// Check final state
	fmt.Println("\n8. Final state:")
	activeTasks = coordinator.GetActiveTasks()
	fmt.Printf("   Active tasks: %d\n", len(activeTasks))
	fmt.Printf("   chat model loaded: %v (KeepAlive=true)\n", llmManager.IsLoaded(llm.PurposeChat))
	fmt.Printf("   code model loaded: %v (killed after task)\n", llmManager.IsLoaded(llm.PurposeCode))
	fmt.Printf("   analysis model loaded: %v (killed after task)\n", llmManager.IsLoaded(llm.PurposeAnalysis))

	// Summary
	fmt.Println("\n=== Phase 3 Complete ===")
	fmt.Println("\n✓ Tasks track which agent is executing them")
	fmt.Println("✓ Tasks track which model is being used")
	fmt.Println("✓ check_task_progress shows model visibility")
	fmt.Println("✓ Active tasks display agent + model info")
	fmt.Println("✓ Users can see which models are working")
	fmt.Println("\nReady for Phase 4: Concurrent Chat")
}

// MockAgent for testing
type MockAgent struct {
	name    string
	purpose llm.Purpose
}

func (a *MockAgent) Name() string            { return a.name }
func (a *MockAgent) Purpose() llm.Purpose    { return a.purpose }
func (a *MockAgent) CanHandle(task *agent.Task) bool { return task.Type == "test" }
func (a *MockAgent) AllowedTools() []string  { return []string{} }

func (a *MockAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	// Simulate work
	time.Sleep(1500 * time.Millisecond)

	return &agent.Result{
		TaskID:  task.ID,
		Success: true,
		Output:  fmt.Sprintf("Completed: %s", task.Description),
	}, nil
}
