package base

import (
	"fmt"
	"time"
)

// TaskContext is the complete execution context for a task
// Passed directly to all layers - no text parsing ever needed
type TaskContext struct {
	// Identity
	TaskID   string
	TaskKey  string // e.g., TASK-001
	ParentID string

	// Core task info
	Description string
	Type        string
	Priority    int

	// Execution parameters (the critical part!)
	ProjectPath string                 // Pre-extracted, absolute path
	Input       map[string]interface{} // Raw input map

	// Dependencies & relationships
	DependsOn        []string // Task keys
	DependencyFiles  []string // Files created by dependencies
	DependencyErrors []string // Errors from dependencies to learn from

	// Feedback context (for self-improvement)
	PreviousAttempts int
	PreviousErrors   []ExecutionError

	// Artifacts & history (temporarily using simple types)
	CreatedFiles  []string // Files this task created
	ModifiedFiles []string // Files this task modified

	// Metadata
	CreatedAt   time.Time
	StartedAt   time.Time
	LastAttempt time.Time
}

// ExecutionError captures rich error context for learning
type ExecutionError struct {
	Timestamp   time.Time
	Agent       string
	Phase       string // "precondition", "execution", "verification"
	ErrorType   string // "missing_file", "compile_error", "path_error"
	Message     string
	FilePath    string                 // Where error occurred
	LineNumber  int                    // If applicable
	CodeSnippet string                 // If applicable
	Suggestion  string                 // How to fix
	Metadata    map[string]interface{} // Extensible for any additional context
}

// Note: NewTaskContext has been moved to orchestration package
// since it depends on ManagedTask which is orchestration-specific

// AddError records an execution error for learning
func (tc *TaskContext) AddError(err ExecutionError) {
	tc.PreviousErrors = append(tc.PreviousErrors, err)
	tc.PreviousAttempts++
	tc.LastAttempt = time.Now()
}

// AddCreatedFile tracks file creation
func (tc *TaskContext) AddCreatedFile(path string) {
	tc.CreatedFiles = append(tc.CreatedFiles, path)
}

// AddModifiedFile tracks file modification
func (tc *TaskContext) AddModifiedFile(path string) {
	tc.ModifiedFiles = append(tc.ModifiedFiles, path)
}

// ShouldRetry determines if task should retry based on history
func (tc *TaskContext) ShouldRetry(maxAttempts int) bool {
	return tc.PreviousAttempts < maxAttempts
}

// GetErrorPatterns analyzes error history for patterns
func (tc *TaskContext) GetErrorPatterns() []string {
	patterns := make(map[string]int)
	for _, err := range tc.PreviousErrors {
		patterns[err.ErrorType]++
	}

	var result []string
	for errType, count := range patterns {
		if count > 1 {
			result = append(result, fmt.Sprintf("%s (x%d)", errType, count))
		}
	}
	return result
}

// HasErrors returns true if there are previous errors
func (tc *TaskContext) HasErrors() bool {
	return len(tc.PreviousErrors) > 0
}

// GetLastError returns the most recent error, or nil if none
func (tc *TaskContext) GetLastError() *ExecutionError {
	if len(tc.PreviousErrors) == 0 {
		return nil
	}
	return &tc.PreviousErrors[len(tc.PreviousErrors)-1]
}
