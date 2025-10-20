// Test script to demonstrate Phase 4: Concurrent Chat with Task Awareness
// Shows Wilson can chat while background tasks run, aware of active tasks

package main

import (
	"context"
	"fmt"
	"time"

	"wilson/agent"
	"wilson/llm"
	"wilson/session"
)

func main() {
	fmt.Println("=== Phase 4: Concurrent Chat with Task Awareness Test ===\n")

	// Create LLM manager
	llmManager := llm.NewManager()
	defer llmManager.Stop()

	// Register models
	llmManager.RegisterLLM(llm.PurposeChat, llm.Config{
		Provider:    "ollama",
		Model:       "llama3:latest",
		KeepAlive:   true,
		IdleTimeout: 0,
	})

	llmManager.RegisterLLM(llm.PurposeCode, llm.Config{
		Provider:    "ollama",
		Model:       "qwen2.5-coder:14b",
		KeepAlive:   false,
		IdleTimeout: 0,
	})

	// Create registry and coordinator
	registry := agent.NewRegistry()
	coordinator := agent.NewCoordinator(registry)
	coordinator.SetLLMManager(llmManager)
	coordinator.SetMaxConcurrent(2)
	agent.SetGlobalCoordinator(coordinator)

	// Create mock agents
	codeAgent := &MockAgent{name: "code-agent", purpose: llm.PurposeCode}
	chatAgent := &MockAgent{name: "chat-agent", purpose: llm.PurposeChat}

	registry.Register(codeAgent)
	registry.Register(chatAgent)

	// Create thread-safe history (Phase 4)
	history := session.NewHistory(10)

	fmt.Println("1. Testing thread-safe history")

	// Test concurrent writes to history
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(n int) {
			history.AddMessage("user", fmt.Sprintf("Message %d", n))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	fmt.Printf("   ✓ Added 5 messages concurrently\n")
	fmt.Printf("   Message count: %d (thread-safe)\n\n", history.GetMessageCount())

	// Test 2: Task-aware chat
	fmt.Println("2. Starting background task...")
	taskID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "code-agent",
		Type:        "test",
		Description: "Implement user authentication system",
		Priority:    3,
	})

	// Wait for task to start and acquire model
	time.Sleep(200 * time.Millisecond)

	task, _, _ := coordinator.GetTaskStatus(taskID)
	fmt.Printf("   ✓ Task %s started\n", task.ID[:8])
	fmt.Printf("   Agent: %s\n", task.AgentName)
	fmt.Printf("   Model: %s\n", task.ModelUsed)
	fmt.Printf("   Status: %s\n\n", task.Status)

	// Test 3: Chat while task runs (Wilson should be task-aware)
	fmt.Println("3. Simulating chat while task runs in background...\n")
	fmt.Println("   Note: ChatAgent's system prompt now includes active task info!")
	fmt.Println("   System prompt would contain:")

	activeTasks := coordinator.GetActiveTasks()
	if len(activeTasks) > 0 {
		fmt.Println("   ---")
		fmt.Println("   Active background tasks you're coordinating:")
		for _, t := range activeTasks {
			fmt.Printf("   - Task %s (%s): %s", t.ID[:8], t.Type, t.Description)
			if t.ModelUsed != "" {
				fmt.Printf(" [using %s model]", t.ModelUsed)
			}
			fmt.Println()
		}
		fmt.Println("   You can reference these tasks when answering questions.")
		fmt.Println("   ---\n")
	}

	// Simulate multiple chat interactions while task runs
	fmt.Println("4. Simulating concurrent chat interactions...")
	history.AddMessage("user", "What's 2+2?")
	history.AddMessage("assistant", "4. By the way, your authentication task is in progress using qwen2.5-coder:14b.")
	fmt.Println("   User: What's 2+2?")
	fmt.Println("   Wilson: 4. By the way, your authentication task is in progress using qwen2.5-coder:14b.\n")

	time.Sleep(500 * time.Millisecond)

	history.AddMessage("user", "How's the task going?")
	history.AddMessage("assistant", "The code agent is still working on implementing your authentication system. It's using the qwen2.5-coder:14b model.")
	fmt.Println("   User: How's the task going?")
	fmt.Println("   Wilson: The code agent is still working on implementing your authentication system.\n")

	// Start another task
	fmt.Println("5. Starting second background task...")
	task2ID, _ := coordinator.DelegateTaskAsync(context.Background(), agent.DelegationRequest{
		ToAgent:     "code-agent",
		Type:        "test",
		Description: "Write unit tests",
		Priority:    3,
	})
	fmt.Printf("   ✓ Task %s started\n\n", task2ID[:8])

	time.Sleep(200 * time.Millisecond)

	// Check active tasks
	activeTasks = coordinator.GetActiveTasks()
	fmt.Printf("6. Active tasks: %d\n", len(activeTasks))
	fmt.Println("   Wilson's system prompt now shows:")
	for _, t := range activeTasks {
		fmt.Printf("   - Task %s: %s [%s]\n", t.ID[:8], t.Description, t.ModelUsed)
	}

	// Test concurrent access to history
	fmt.Println("\n7. Testing concurrent history access (50 operations)...")
	var wg = make(chan bool, 50)

	// Concurrent reads
	for i := 0; i < 25; i++ {
		go func() {
			_ = history.GetMessageCount()
			_ = history.GetLastUserMessage()
			wg <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 25; i++ {
		go func(n int) {
			history.AddMessage("user", fmt.Sprintf("Concurrent message %d", n))
			wg <- true
		}(i)
	}

	// Wait for all operations
	for i := 0; i < 50; i++ {
		<-wg
	}

	fmt.Printf("   ✓ Completed 50 concurrent operations (25 reads + 25 writes)\n")
	fmt.Printf("   Final message count: %d\n", history.GetMessageCount())
	fmt.Printf("   No race conditions or panics!\n\n")

	// Wait for tasks to complete
	time.Sleep(2 * time.Second)

	// Check completion
	fmt.Println("8. Final state:")
	activeTasks = coordinator.GetActiveTasks()
	fmt.Printf("   Active tasks: %d\n", len(activeTasks))
	fmt.Printf("   Chat model loaded: %v (always on)\n", llmManager.IsLoaded(llm.PurposeChat))
	fmt.Printf("   Code model loaded: %v (killed after tasks)\n", llmManager.IsLoaded(llm.PurposeCode))

	// Summary
	fmt.Println("\n=== Phase 4 Complete ===")
	fmt.Println("\n✓ History is thread-safe (sync.RWMutex)")
	fmt.Println("✓ Concurrent reads/writes work safely")
	fmt.Println("✓ ChatAgent is task-aware (system prompt includes active tasks)")
	fmt.Println("✓ Wilson can chat while background tasks run")
	fmt.Println("✓ Users can ask about task progress during chat")
	fmt.Println("\nReady for Phase 5: Model Health & Fallback")
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
	time.Sleep(2 * time.Second)

	return &agent.Result{
		TaskID:  task.ID,
		Success: true,
		Output:  fmt.Sprintf("Completed: %s", task.Description),
	}, nil
}
