package orchestration

import (
	"fmt"
	"os"

	"wilson/agent/base"
)

// NewTaskContext creates a TaskContext from a ManagedTask
func NewTaskContext(task *ManagedTask) *base.TaskContext {
	ctx := &base.TaskContext{
		TaskID:         fmt.Sprintf("%d", task.ID),
		TaskKey:        task.TaskKey,
		Description:    task.Description,
		Type:           string(task.Type),
		Priority:       task.Priority,
		Input:          task.Input,
		DependsOn:      task.DependsOn,
		CreatedAt:      task.CreatedAt,
		CreatedFiles:   make([]string, 0),
		ModifiedFiles:  make([]string, 0),
		PreviousErrors: make([]base.ExecutionError, 0),
	}

	// Extract and validate project path
	if projectPath, ok := task.Input["project_path"].(string); ok && projectPath != "" {
		ctx.ProjectPath = projectPath
	} else {
		ctx.ProjectPath = "." // Safe default
	}

	// Extract dependency files if available
	if depFiles, ok := task.Input["dependency_files"].([]string); ok {
		ctx.DependencyFiles = depFiles
	}

	// Validate path exists (non-fatal, just log warning)
	if ctx.ProjectPath != "." {
		if _, err := os.Stat(ctx.ProjectPath); os.IsNotExist(err) {
			fmt.Printf("[TaskContext] Warning: Project path does not exist: %s\n", ctx.ProjectPath)
		}
	}

	return ctx
}
