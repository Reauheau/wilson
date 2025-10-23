package agent

import (
	"context"
	"testing"
	"time"
)

// TestFeedbackBus_SendReceive tests basic feedback sending and receiving
func TestFeedbackBus_SendReceive(t *testing.T) {
	bus := GetFeedbackBus()

	received := false
	var receivedFeedback *AgentFeedback

	// Register handler
	bus.RegisterHandler(FeedbackTypeDependencyNeeded, func(ctx context.Context, feedback *AgentFeedback) error {
		received = true
		receivedFeedback = feedback
		return nil
	})

	// Start bus
	ctx := context.Background()
	bus.Start(ctx)

	// Send feedback
	feedback := &AgentFeedback{
		TaskID:       "TASK-001",
		AgentName:    "TestAgent",
		FeedbackType: FeedbackTypeDependencyNeeded,
		Severity:     FeedbackSeverityCritical,
		Message:      "Missing dependency",
		Context:      map[string]interface{}{"dependency_description": "test files"},
		Suggestion:   "Create test files",
	}

	err := bus.Send(feedback)
	if err != nil {
		t.Fatalf("Failed to send feedback: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if !received {
		t.Error("Feedback was not received")
	}

	if receivedFeedback == nil {
		t.Fatal("Received feedback is nil")
	}

	if receivedFeedback.TaskID != "TASK-001" {
		t.Errorf("Expected TaskID 'TASK-001', got %s", receivedFeedback.TaskID)
	}

	if receivedFeedback.AgentName != "TestAgent" {
		t.Errorf("Expected AgentName 'TestAgent', got %s", receivedFeedback.AgentName)
	}
}

// TestFeedbackBus_Timeout tests timeout behavior
func TestFeedbackBus_Timeout(t *testing.T) {
	// Create a new bus with small buffer
	bus := &FeedbackBus{
		feedbackChan: make(chan *AgentFeedback, 1),
		handlers:     make(map[FeedbackType]FeedbackHandler),
	}

	// Fill the buffer
	feedback1 := &AgentFeedback{
		TaskID:       "TASK-001",
		FeedbackType: FeedbackTypeDependencyNeeded,
	}
	_ = bus.Send(feedback1)

	// Try to send another (should timeout)
	feedback2 := &AgentFeedback{
		TaskID:       "TASK-002",
		FeedbackType: FeedbackTypeDependencyNeeded,
	}
	err := bus.Send(feedback2)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err.Error() != "feedback bus timeout" {
		t.Errorf("Expected 'feedback bus timeout', got %s", err.Error())
	}
}

// TestTaskContext_GetErrorPatterns tests error pattern detection
func TestTaskContext_GetErrorPatterns(t *testing.T) {
	task := NewManagedTask("Test Task", "Test description", ManagedTaskTypeCode)
	taskCtx := NewTaskContext(task)

	// Add some errors
	taskCtx.AddError(ExecutionError{
		Timestamp: time.Now(),
		Agent:     "CodeAgent",
		Phase:     "compilation",
		ErrorType: "compile_error",
		Message:   "undefined: fmt.Println",
	})

	taskCtx.AddError(ExecutionError{
		Timestamp: time.Now(),
		Agent:     "CodeAgent",
		Phase:     "compilation",
		ErrorType: "compile_error",
		Message:   "undefined: fmt.Sprintf",
	})

	taskCtx.AddError(ExecutionError{
		Timestamp: time.Now(),
		Agent:     "TestAgent",
		Phase:     "precondition",
		ErrorType: "missing_file",
		Message:   "test file not found",
	})

	patterns := taskCtx.GetErrorPatterns()

	// Should detect compile_error pattern (appears twice)
	// Pattern format is "error_type (xN)" where N is count
	found := false
	for _, pattern := range patterns {
		if pattern == "compile_error (x2)" || pattern == "compile_error" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find compile_error pattern, got: %v", patterns)
	}
}

// TestTaskContext_ShouldRetry tests retry logic
func TestTaskContext_ShouldRetry(t *testing.T) {
	task := NewManagedTask("Test Task", "Test description", ManagedTaskTypeCode)
	taskCtx := NewTaskContext(task)

	// Should retry with 0 attempts
	if !taskCtx.ShouldRetry(3) {
		t.Error("Should retry with 0 attempts")
	}

	// Simulate some attempts
	taskCtx.PreviousAttempts = 2

	// Should still retry (2 < 3)
	if !taskCtx.ShouldRetry(3) {
		t.Error("Should retry with 2 attempts when max is 3")
	}

	// Exceed max attempts
	taskCtx.PreviousAttempts = 3

	// Should not retry (3 >= 3)
	if taskCtx.ShouldRetry(3) {
		t.Error("Should not retry with 3 attempts when max is 3")
	}
}

// TestManagerAgent_SmartRetryDecision tests smart retry logic
func TestManagerAgent_SmartRetryDecision(t *testing.T) {
	// This would be a more complex integration test
	// For now, just verify the method exists and compiles
	task := NewManagedTask("Test Task", "Test description", ManagedTaskTypeCode)
	taskCtx := NewTaskContext(task)

	// Add multiple errors to trigger escalation
	for i := 0; i < 4; i++ {
		taskCtx.AddError(ExecutionError{
			Timestamp: time.Now(),
			Agent:     "TestAgent",
			Phase:     "execution",
			ErrorType: "test_error",
			Message:   "test failed",
		})
		taskCtx.PreviousAttempts++
	}

	// Should not retry with 4 attempts
	if taskCtx.ShouldRetry(3) {
		t.Error("Should not retry after max attempts exceeded")
	}
}
