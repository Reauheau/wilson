package feedback

import (
	"context"
	"testing"
	"time"

	"wilson/agent"
)

// TestFeedbackBus_SendReceive tests basic feedback sending and receiving
func TestFeedbackBus_SendReceive(t *testing.T) {
	bus := GetFeedbackBus()

	received := false
	var receivedFeedback *agent.AgentFeedback

	// Register handler
	bus.RegisterHandler(agent.FeedbackTypeDependencyNeeded, func(ctx context.Context, feedback *agent.AgentFeedback) error {
		received = true
		receivedFeedback = feedback
		return nil
	})

	// Start bus
	ctx := context.Background()
	bus.Start(ctx)

	// Send feedback
	feedback := &agent.AgentFeedback{
		TaskID:       "TASK-001",
		AgentName:    "TestAgent",
		FeedbackType: agent.FeedbackTypeDependencyNeeded,
		Severity:     agent.FeedbackSeverityCritical,
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
		feedbackChan: make(chan *agent.AgentFeedback, 1),
		handlers:     make(map[agent.FeedbackType]FeedbackHandler),
	}

	// Fill the buffer
	feedback1 := &agent.AgentFeedback{
		TaskID:       "TASK-001",
		FeedbackType: agent.FeedbackTypeDependencyNeeded,
	}
	_ = bus.Send(feedback1)

	// Try to send another (should timeout)
	feedback2 := &agent.AgentFeedback{
		TaskID:       "TASK-002",
		FeedbackType: agent.FeedbackTypeDependencyNeeded,
	}
	err := bus.Send(feedback2)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err.Error() != "feedback bus timeout" {
		t.Errorf("Expected 'feedback bus timeout', got %s", err.Error())
	}
}

// TestFeedbackBus_SetDatabase tests setting database on feedback bus
func TestFeedbackBus_SetDatabase(t *testing.T) {
	bus := &FeedbackBus{
		feedbackChan: make(chan *agent.AgentFeedback, 10),
		handlers:     make(map[agent.FeedbackType]FeedbackHandler),
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
		feedbackChan: make(chan *agent.AgentFeedback, 10),
		handlers:     make(map[agent.FeedbackType]FeedbackHandler),
		db:           nil, // No database
	}

	feedback := &agent.AgentFeedback{
		TaskID:       "TASK-001",
		AgentName:    "TestAgent",
		FeedbackType: agent.FeedbackTypeDependencyNeeded,
		Severity:     agent.FeedbackSeverityCritical,
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
		feedbackChan: make(chan *agent.AgentFeedback, 10),
		handlers:     make(map[agent.FeedbackType]FeedbackHandler),
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
		feedbackChan: make(chan *agent.AgentFeedback, 10),
		handlers:     make(map[agent.FeedbackType]FeedbackHandler),
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
