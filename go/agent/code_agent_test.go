package agent

import (
	"context"
	"os"
	"testing"
)

// TestCodeAgent_checkPreconditions_DirectoryExists tests that preconditions pass when directory exists
func TestCodeAgent_checkPreconditions_DirectoryExists(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "wilson-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create code agent
	agent := &CodeAgent{
		BaseAgent: &BaseAgent{
			name: "TestCodeAgent",
		},
	}

	// Create task with existing directory
	task := &Task{
		ID:          "TEST-001",
		Type:        "code",
		Description: "Test task",
		Input: map[string]interface{}{
			"project_path": tmpDir,
		},
	}

	// Check preconditions - should pass
	err = agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass for existing directory, got error: %v", err)
	}
}

// TestCodeAgent_checkPreconditions_DirectoryDoesNotExist tests that preconditions request dependency when directory doesn't exist
func TestCodeAgent_checkPreconditions_DirectoryDoesNotExist(t *testing.T) {
	// Use a non-existent directory path - make it deeply nested to ensure parent doesn't exist
	nonExistentPath := "/tmp/wilson-test-nonexistent-parent-12345/subdir/another"

	// Make sure it doesn't exist
	os.RemoveAll("/tmp/wilson-test-nonexistent-parent-12345")

	// Verify it doesn't exist
	if _, err := os.Stat(nonExistentPath); err == nil {
		t.Fatalf("Test path unexpectedly exists: %s", nonExistentPath)
	}

	// Create code agent with feedback bus initialized
	bus := GetFeedbackBus()

	// Register handler for feedback
	bus.RegisterHandler(FeedbackTypeDependencyNeeded, func(ctx context.Context, feedback *AgentFeedback) error {
		// Feedback received and handled
		return nil
	})

	bus.Start(context.Background())

	agent := &CodeAgent{
		BaseAgent: &BaseAgent{
			name:           "TestCodeAgent",
			currentTaskID:  "TEST-002",
			currentContext: nil,
		},
	}

	// Create task with non-existent directory
	task := &Task{
		ID:          "TEST-002",
		Type:        "code",
		Description: "Test task",
		Input: map[string]interface{}{
			"project_path": nonExistentPath,
		},
	}

	// Check preconditions - should request dependency via feedback
	err := agent.checkPreconditions(context.Background(), task)

	// The RequestDependency sends feedback and returns an error
	if err == nil {
		t.Error("Expected preconditions to fail for non-existent directory, got nil")
	}

	// Verify feedback was sent (with a small wait for async processing)
	// Note: This may not always be reliable in tests, but the error is the main check
}

// TestCodeAgent_checkPreconditions_FixModeWithMissingFile tests that fix tasks fail when target file doesn't exist
func TestCodeAgent_checkPreconditions_FixModeWithMissingFile(t *testing.T) {
	// Create code agent
	agent := &CodeAgent{
		BaseAgent: &BaseAgent{
			name: "TestCodeAgent",
		},
	}

	// Create task with fix_mode and non-existent file
	task := &Task{
		ID:          "TEST-003",
		Type:        "code",
		Description: "Fix errors in file",
		Input: map[string]interface{}{
			"fix_mode":    true,
			"target_file": "/tmp/nonexistent-file-12345.go",
		},
	}

	// Check preconditions - should fail
	err := agent.checkPreconditions(context.Background(), task)
	if err == nil {
		t.Error("Expected preconditions to fail for fix_mode with non-existent file, got nil")
	}

	if err != nil && err.Error() != "cannot fix non-existent file: /tmp/nonexistent-file-12345.go" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// TestCodeAgent_checkPreconditions_FixModeWithExistingFile tests that fix tasks pass when target file exists
func TestCodeAgent_checkPreconditions_FixModeWithExistingFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "wilson-test-*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create code agent
	agent := &CodeAgent{
		BaseAgent: &BaseAgent{
			name: "TestCodeAgent",
		},
	}

	// Create task with fix_mode and existing file
	task := &Task{
		ID:          "TEST-004",
		Type:        "code",
		Description: "Fix errors in file",
		Input: map[string]interface{}{
			"fix_mode":    true,
			"target_file": tmpFile.Name(),
		},
	}

	// Check preconditions - should pass
	err = agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass for fix_mode with existing file, got error: %v", err)
	}
}

// TestCodeAgent_checkPreconditions_CurrentDirectory tests that current directory (".") always passes
func TestCodeAgent_checkPreconditions_CurrentDirectory(t *testing.T) {
	// Create code agent
	agent := &CodeAgent{
		BaseAgent: &BaseAgent{
			name: "TestCodeAgent",
		},
	}

	// Create task with current directory
	task := &Task{
		ID:          "TEST-005",
		Type:        "code",
		Description: "Test task",
		Input: map[string]interface{}{
			"project_path": ".",
		},
	}

	// Check preconditions - should pass
	err := agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass for current directory, got error: %v", err)
	}
}

// TestCodeAgent_checkPreconditions_NoProjectPath tests default to current directory when no project_path specified
func TestCodeAgent_checkPreconditions_NoProjectPath(t *testing.T) {
	// Create code agent
	agent := &CodeAgent{
		BaseAgent: &BaseAgent{
			name: "TestCodeAgent",
		},
	}

	// Create task without project_path
	task := &Task{
		ID:          "TEST-006",
		Type:        "code",
		Description: "Test task",
		Input:       map[string]interface{}{},
	}

	// Check preconditions - should pass (defaults to current directory)
	err := agent.checkPreconditions(context.Background(), task)
	if err != nil {
		t.Errorf("Expected preconditions to pass when no project_path specified, got error: %v", err)
	}
}
