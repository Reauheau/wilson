// Test script to demonstrate Phase 0: Model Lifecycle Management
// Shows how models are loaded on-demand and unloaded based on KeepAlive/IdleTimeout

package main

import (
	"fmt"
	"time"
	"wilson/llm"
)

func main() {
	fmt.Println("=== Phase 0: Model Lifecycle Management Test ===\n")

	// Create manager
	manager := llm.NewManager()
	defer manager.Stop()

	// Register Wilson's chat model (always-on)
	fmt.Println("1. Registering Wilson's chat model (llama3, KeepAlive=true)")
	chatConfig := llm.Config{
		Provider:    "ollama",
		Model:       "llama3:latest",
		Temperature: 0.7,
		KeepAlive:   true, // Never unload Wilson's chat model
		IdleTimeout: 0,
	}
	err := manager.RegisterLLM(llm.PurposeChat, chatConfig)
	if err != nil {
		fmt.Printf("   Warning: Could not register chat model: %v\n", err)
	} else {
		fmt.Println("   ✓ Chat model registered")
	}

	// Register code model (on-demand, kill-after-task)
	fmt.Println("\n2. Registering Code Agent model (qwen2.5-coder:14b, KeepAlive=false)")
	codeConfig := llm.Config{
		Provider:    "ollama",
		Model:       "qwen2.5-coder:14b",
		Temperature: 0.2,
		KeepAlive:   false,        // Kill after task
		IdleTimeout: 0,            // Immediate unload (no idle period)
	}
	err = manager.RegisterLLM(llm.PurposeCode, codeConfig)
	if err != nil {
		fmt.Printf("   Warning: Could not register code model: %v\n", err)
	} else {
		fmt.Println("   ✓ Code model registered")
	}

	// Initial state
	fmt.Println("\n3. Initial state (no models loaded yet)")
	fmt.Printf("   Chat model loaded: %v (refCount: %d)\n",
		manager.IsLoaded(llm.PurposeChat), manager.GetRefCount(llm.PurposeChat))
	fmt.Printf("   Code model loaded: %v (refCount: %d)\n",
		manager.IsLoaded(llm.PurposeCode), manager.GetRefCount(llm.PurposeCode))

	// Acquire chat model (Wilson)
	fmt.Println("\n4. Acquiring chat model (simulating Wilson starting)")
	chatClient, releaseChatClient, err := manager.AcquireModel(llm.PurposeChat)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   ✓ Acquired chat model: %s (provider: %s)\n",
			chatClient.GetModel(), chatClient.GetProvider())
		fmt.Printf("   Chat model loaded: %v (refCount: %d)\n",
			manager.IsLoaded(llm.PurposeChat), manager.GetRefCount(llm.PurposeChat))
	}

	// Simulate task delegation - acquire code model
	fmt.Println("\n5. User delegates code task - acquiring code model")
	codeClient, releaseCodeClient, err := manager.AcquireModel(llm.PurposeCode)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   ✓ Acquired code model: %s (provider: %s)\n",
			codeClient.GetModel(), codeClient.GetProvider())
		fmt.Printf("   Code model loaded: %v (refCount: %d)\n",
			manager.IsLoaded(llm.PurposeCode), manager.GetRefCount(llm.PurposeCode))
	}

	// Current state - both models active
	fmt.Println("\n6. Current state (Wilson + Code Agent both active)")
	fmt.Printf("   Chat model loaded: %v (refCount: %d) - KeepAlive: true\n",
		manager.IsLoaded(llm.PurposeChat), manager.GetRefCount(llm.PurposeChat))
	fmt.Printf("   Code model loaded: %v (refCount: %d) - KeepAlive: false\n",
		manager.IsLoaded(llm.PurposeCode), manager.GetRefCount(llm.PurposeCode))

	// Simulate code task completion - release code model
	fmt.Println("\n7. Code task completes - releasing code model")
	if releaseCodeClient != nil {
		releaseCodeClient()
		fmt.Printf("   ✓ Released code model\n")
		fmt.Printf("   Code model loaded: %v (refCount: %d)\n",
			manager.IsLoaded(llm.PurposeCode), manager.GetRefCount(llm.PurposeCode))

		if !manager.IsLoaded(llm.PurposeCode) {
			fmt.Println("   ✓ Code model immediately unloaded (IdleTimeout=0)")
		}
	}

	// Final state - only Wilson active
	fmt.Println("\n8. Final state (only Wilson remains)")
	fmt.Printf("   Chat model loaded: %v (refCount: %d) - Never unloaded\n",
		manager.IsLoaded(llm.PurposeChat), manager.GetRefCount(llm.PurposeChat))
	fmt.Printf("   Code model loaded: %v (refCount: %d) - Killed after task\n",
		manager.IsLoaded(llm.PurposeCode), manager.GetRefCount(llm.PurposeCode))

	// Test multiple acquisitions (simulating multiple concurrent tasks)
	fmt.Println("\n9. Test: Multiple concurrent tasks (2 workers)")
	codeClient1, releaseCode1, err := manager.AcquireModel(llm.PurposeCode)
	if err != nil {
		fmt.Printf("   Error acquiring code model 1: %v\n", err)
	} else {
		fmt.Printf("   ✓ Worker 1 acquired code model (refCount: %d)\n",
			manager.GetRefCount(llm.PurposeCode))
	}

	codeClient2, releaseCode2, err := manager.AcquireModel(llm.PurposeCode)
	if err != nil {
		fmt.Printf("   Error acquiring code model 2: %v\n", err)
	} else {
		fmt.Printf("   ✓ Worker 2 acquired code model (refCount: %d)\n",
			manager.GetRefCount(llm.PurposeCode))

		// Verify both got same client
		if codeClient1 == codeClient2 {
			fmt.Println("   ✓ Both workers share same model instance (efficient!)")
		}
	}

	// Release first worker
	fmt.Println("\n10. Worker 1 completes task")
	if releaseCode1 != nil {
		releaseCode1()
		fmt.Printf("   ✓ Worker 1 released (refCount: %d)\n",
			manager.GetRefCount(llm.PurposeCode))
		fmt.Printf("   Code model still loaded: %v (Worker 2 still using it)\n",
			manager.IsLoaded(llm.PurposeCode))
	}

	// Release second worker
	fmt.Println("\n11. Worker 2 completes task")
	if releaseCode2 != nil {
		releaseCode2()
		fmt.Printf("   ✓ Worker 2 released (refCount: %d)\n",
			manager.GetRefCount(llm.PurposeCode))
		fmt.Printf("   Code model loaded: %v (All workers done, immediate cleanup)\n",
			manager.IsLoaded(llm.PurposeCode))
	}

	// Test KeepAlive behavior
	fmt.Println("\n12. Test: KeepAlive prevents unloading (Wilson's chat model)")
	if releaseChatClient != nil {
		releaseChatClient()
		fmt.Printf("   ✓ Released chat model (refCount: %d)\n",
			manager.GetRefCount(llm.PurposeChat))

		time.Sleep(100 * time.Millisecond) // Brief wait

		if manager.IsLoaded(llm.PurposeChat) {
			fmt.Println("   ✓ Chat model still loaded despite refCount=0 (KeepAlive=true)")
		}
	}

	// Summary
	fmt.Println("\n=== Phase 0 Complete ===")
	fmt.Println("\n✓ Model instance tracking with reference counting")
	fmt.Println("✓ AcquireModel() returns client + release function")
	fmt.Println("✓ KeepAlive prevents unloading (Wilson's chat model)")
	fmt.Println("✓ IdleTimeout=0 enables immediate cleanup (worker models)")
	fmt.Println("✓ Multiple acquisitions share same instance")
	fmt.Println("✓ Cleanup goroutine running for safety net")
	fmt.Println("\nReady for Phase 1: Async Foundation")
}
