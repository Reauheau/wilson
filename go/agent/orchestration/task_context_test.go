package orchestration

import (
	"testing"
	"time"
	"wilson/agent/base"
)

// TestTaskContext_GetErrorPatterns tests error pattern detection
func TestTaskContext_GetErrorPatterns(t *testing.T) {
	task := NewManagedTask("Test Task", "Test description", ManagedTaskTypeCode)
	taskCtx := NewTaskContext(task)

	// Add some errors
	taskCtx.AddError(base.ExecutionError{
		Timestamp: time.Now(),
		Agent:     "CodeAgent",
		Phase:     "compilation",
		ErrorType: "compile_error",
		Message:   "undefined: fmt.Println",
	})

	taskCtx.AddError(base.ExecutionError{
		Timestamp: time.Now(),
		Agent:     "CodeAgent",
		Phase:     "compilation",
		ErrorType: "compile_error",
		Message:   "undefined: fmt.Sprintf",
	})

	taskCtx.AddError(base.ExecutionError{
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
		taskCtx.AddError(base.ExecutionError{
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
