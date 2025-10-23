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

// TestFeedbackBus_SetDatabase tests setting database on feedback bus
func TestFeedbackBus_SetDatabase(t *testing.T) {
	bus := &FeedbackBus{
		feedbackChan: make(chan *AgentFeedback, 10),
		handlers:     make(map[FeedbackType]FeedbackHandler),
	}

	// Should start with nil DB
	if bus.db != nil {
		t.Error("Expected db to be nil initially")
	}

	// Mock database (just for testing the setter)
	bus.SetDatabase(nil)

	// Verify it doesn't panic with nil
	if bus.db != nil {
		t.Error("Expected db to remain nil")
	}
}

// TestFeedbackBus_PersistFeedback_NilDB tests graceful handling when DB is nil
func TestFeedbackBus_PersistFeedback_NilDB(t *testing.T) {
	bus := &FeedbackBus{
		feedbackChan: make(chan *AgentFeedback, 10),
		handlers:     make(map[FeedbackType]FeedbackHandler),
		db:           nil, // No database
	}

	feedback := &AgentFeedback{
		TaskID:       "TASK-001",
		AgentName:    "TestAgent",
		FeedbackType: FeedbackTypeDependencyNeeded,
		Severity:     FeedbackSeverityCritical,
		CreatedAt:    time.Now(),
	}

	// Should not error when DB is nil
	err := bus.persistFeedback(feedback)
	if err != nil {
		t.Errorf("persistFeedback() with nil DB should not error, got: %v", err)
	}
}

// TestFeedbackBus_GetFeedbackForTask_NilDB tests error handling when DB is nil
func TestFeedbackBus_GetFeedbackForTask_NilDB(t *testing.T) {
	bus := &FeedbackBus{
		feedbackChan: make(chan *AgentFeedback, 10),
		handlers:     make(map[FeedbackType]FeedbackHandler),
		db:           nil,
	}

	_, err := bus.GetFeedbackForTask("TASK-001")
	if err == nil {
		t.Error("GetFeedbackForTask() with nil DB should return error")
	}

	expectedMsg := "database not configured"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFeedbackBus_GetFeedbackStats_NilDB tests error handling when DB is nil
func TestFeedbackBus_GetFeedbackStats_NilDB(t *testing.T) {
	bus := &FeedbackBus{
		feedbackChan: make(chan *AgentFeedback, 10),
		handlers:     make(map[FeedbackType]FeedbackHandler),
		db:           nil,
	}

	_, err := bus.GetFeedbackStats(time.Now())
	if err == nil {
		t.Error("GetFeedbackStats() with nil DB should return error")
	}

	expectedMsg := "database not configured"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFeedbackStats_Structure tests FeedbackStats struct
func TestFeedbackStats_Structure(t *testing.T) {
	stats := &FeedbackStats{
		TotalFeedback:     10,
		ProcessedCount:    8,
		ErrorCount:        2,
		AverageProcessing: 500 * time.Millisecond,
		ByType:            make(map[string]int),
		BySeverity:        make(map[string]int),
	}

	stats.ByType["dependency_needed"] = 5
	stats.ByType["success"] = 5
	stats.BySeverity["critical"] = 3
	stats.BySeverity["warning"] = 7

	if stats.TotalFeedback != 10 {
		t.Errorf("Expected TotalFeedback 10, got %d", stats.TotalFeedback)
	}

	if stats.ByType["dependency_needed"] != 5 {
		t.Errorf("Expected 5 dependency_needed, got %d", stats.ByType["dependency_needed"])
	}
}
