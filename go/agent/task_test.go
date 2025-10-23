package agent

import (
	"testing"
)

// TestManagedTask_StateTransitions tests basic state changes
func TestManagedTask_StateTransitions(t *testing.T) {
	task := NewManagedTask("Test Task", "Test description", ManagedTaskTypeCode)

	// Initial state should be NEW
	if task.Status != ManagedTaskStatusNew {
		t.Errorf("Initial status should be NEW, got %v", task.Status)
	}

	// Test Block
	task.Block("waiting for dependency")
	if task.Status != ManagedTaskStatusBlocked {
		t.Errorf("After Block(), status should be BLOCKED, got %v", task.Status)
	}
	if task.Metadata["block_reason"] == "" {
		t.Error("block_reason should be set in Metadata after Block()")
	}

	// Test Unblock
	task.Unblock()
	if task.Status != ManagedTaskStatusReady {
		t.Errorf("After Unblock(), status should be READY, got %v", task.Status)
	}
	if task.Metadata["block_reason"] != nil {
		t.Error("block_reason should be cleared from Metadata after Unblock()")
	}

	// Test Assign
	task.Assign("TestAgent")
	if task.Status != ManagedTaskStatusAssigned {
		t.Errorf("After Assign(), status should be ASSIGNED, got %v", task.Status)
	}
	if task.AssignedTo != "TestAgent" {
		t.Errorf("AssignedTo should be 'TestAgent', got %v", task.AssignedTo)
	}
}

// TestManagedTask_RequestReview tests review workflow
func TestManagedTask_RequestReview(t *testing.T) {
	task := NewManagedTask("Test Task", "Test description", ManagedTaskTypeCode)

	task.RequestReview("ReviewerAgent")

	if task.Status != ManagedTaskStatusInReview {
		t.Errorf("After RequestReview(), status should be IN_REVIEW, got %v", task.Status)
	}
	if task.Reviewer != "ReviewerAgent" {
		t.Errorf("Reviewer should be 'ReviewerAgent', got %v", task.Reviewer)
	}
	if task.ReviewStatus != ReviewStatusPending {
		t.Errorf("ReviewStatus should be PENDING, got %v", task.ReviewStatus)
	}
}

// TestNewManagedTask tests task creation
func TestNewManagedTask(t *testing.T) {
	title := "Test Task"
	description := "Test description"
	taskType := ManagedTaskTypeCode

	task := NewManagedTask(title, description, taskType)

	if task.Title != title {
		t.Errorf("Title = %v, want %v", task.Title, title)
	}
	if task.Description != description {
		t.Errorf("Description = %v, want %v", task.Description, description)
	}
	if task.Type != taskType {
		t.Errorf("Type = %v, want %v", task.Type, taskType)
	}
	if task.Status != ManagedTaskStatusNew {
		t.Errorf("Initial status should be NEW, got %v", task.Status)
	}
}
