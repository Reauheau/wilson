package agent

import (
	"context"
	"testing"
)

// TestReviewAgent_checkPreconditions_WithDependencyFiles tests that preconditions pass when code is available
func TestReviewAgent_checkPreconditions_WithDependencyFiles(t *testing.T) {
	// Create review agent with TaskContext containing dependency files
	agent := &ReviewAgent{
		BaseAgent: &BaseAgent{
			name: "TestReviewAgent",
			currentContext: &TaskContext{
				DependencyFiles: []string{
					"main.go",
					"handler.go",
					"utils.go",
				},
			},
		},
	}

	// Create task
	task := &Task{
		ID:          "TEST-001",
		Type:        "review",
		Description: "Review code",
	}

	// Check preconditions - should pass
	err := agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass with dependency files, got error: %v", err)
	}
}

// TestReviewAgent_checkPreconditions_NoDependencyFiles tests that preconditions fail when no code is available
func TestReviewAgent_checkPreconditions_NoDependencyFiles(t *testing.T) {
	// Initialize feedback bus for the test
	bus := GetFeedbackBus()
	bus.RegisterHandler(FeedbackTypeDependencyNeeded, func(ctx context.Context, feedback *AgentFeedback) error {
		return nil
	})
	bus.Start(context.Background())

	// Create review agent with TaskContext but no dependency files
	agent := &ReviewAgent{
		BaseAgent: &BaseAgent{
			name:          "TestReviewAgent",
			currentTaskID: "TEST-002",
			currentContext: &TaskContext{
				DependencyFiles: []string{}, // Empty - no code to review
			},
		},
	}

	// Create task
	task := &Task{
		ID:          "TEST-002",
		Type:        "review",
		Description: "Review code",
	}

	// Check preconditions - should fail and request dependency
	err := agent.checkPreconditions(context.Background(), task)
	if err == nil {
		t.Error("Expected preconditions to fail when no dependency files available, got nil")
	}

	// Verify error message contains expected text
	if err != nil && err.Error() != "dependency needed: No code artifacts found to review" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// TestReviewAgent_checkPreconditions_NilContext tests that preconditions pass when currentContext is nil
func TestReviewAgent_checkPreconditions_NilContext(t *testing.T) {
	// Create review agent with nil currentContext (shouldn't happen in practice, but handle gracefully)
	agent := &ReviewAgent{
		BaseAgent: &BaseAgent{
			name:           "TestReviewAgent",
			currentContext: nil,
		},
	}

	// Create task
	task := &Task{
		ID:          "TEST-003",
		Type:        "review",
		Description: "Review code",
	}

	// Check preconditions - should pass (nil context is allowed)
	err := agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass with nil context, got error: %v", err)
	}
}

// TestReviewAgent_checkPreconditions_SingleFile tests that preconditions pass with a single dependency file
func TestReviewAgent_checkPreconditions_SingleFile(t *testing.T) {
	// Create review agent with TaskContext containing one dependency file
	agent := &ReviewAgent{
		BaseAgent: &BaseAgent{
			name: "TestReviewAgent",
			currentContext: &TaskContext{
				DependencyFiles: []string{"main.go"},
			},
		},
	}

	// Create task
	task := &Task{
		ID:          "TEST-004",
		Type:        "review",
		Description: "Review code",
	}

	// Check preconditions - should pass
	err := agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass with single dependency file, got error: %v", err)
	}
}

// TestReviewAgent_checkPreconditions_MultipleFiles tests that preconditions pass with multiple dependency files
func TestReviewAgent_checkPreconditions_MultipleFiles(t *testing.T) {
	// Create review agent with TaskContext containing multiple dependency files
	agent := &ReviewAgent{
		BaseAgent: &BaseAgent{
			name: "TestReviewAgent",
			currentContext: &TaskContext{
				DependencyFiles: []string{
					"main.go",
					"handler.go",
					"service.go",
					"repository.go",
					"model.go",
				},
			},
		},
	}

	// Create task
	task := &Task{
		ID:          "TEST-005",
		Type:        "review",
		Description: "Review multi-file project",
	}

	// Check preconditions - should pass
	err := agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass with multiple dependency files, got error: %v", err)
	}
}
