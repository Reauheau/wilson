package orchestration

import (
	"context"
	"fmt"

	"wilson/agent/orchestration"
	"wilson/core/registry"
	. "wilson/core/types"
)

type CheckTaskProgressTool struct{}

func (t *CheckTaskProgressTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "check_task_progress",
		Description:     "Check the progress of a background task",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "task_id",
				Type:        "string",
				Required:    false,
				Description: "Task ID to check (omit to see all active tasks)",
				Example:     "abc-123-def",
			},
		},
		Examples: []string{
			`{"tool": "check_task_progress", "arguments": {"task_id": "abc-123"}}`,
			`{"tool": "check_task_progress", "arguments": {}}`,
		},
	}
}

func (t *CheckTaskProgressTool) Validate(args map[string]interface{}) error {
	// task_id is optional - no validation needed
	return nil
}

func (t *CheckTaskProgressTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	coordinator := orchestration.GetGlobalCoordinator()
	if coordinator == nil {
		return "", fmt.Errorf("agent coordinator not initialized")
	}

	taskID, hasTaskID := args["task_id"].(string)

	// If task_id provided, show specific task
	if hasTaskID && taskID != "" {
		task, result, err := coordinator.GetTaskStatus(taskID)
		if err != nil {
			return "", fmt.Errorf("task not found: %s", taskID)
		}

		output := fmt.Sprintf("Task: %s\n", task.ID)
		output += fmt.Sprintf("  Type: %s\n", task.Type)
		output += fmt.Sprintf("  Status: %s\n", task.Status)
		output += fmt.Sprintf("  Description: %s\n", task.Description)
		output += fmt.Sprintf("  Created: %s\n", task.CreatedAt.Format("15:04:05"))

		// Phase 3: Show which agent and model are working on this task
		if task.AgentName != "" {
			output += fmt.Sprintf("  Agent: %s\n", task.AgentName)
		}
		if task.ModelUsed != "" {
			output += fmt.Sprintf("  Model: %s", task.ModelUsed)
			// Phase 5: Show if fallback was used
			if task.UsedFallback {
				output += " (fallback) ⚠️"
			}
			output += "\n"
		}

		if result != nil {
			output += fmt.Sprintf("  Completed: %s\n", result.CompletedAt.Format("15:04:05"))
			if result.Success {
				output += fmt.Sprintf("\n✓ Result:\n%s\n", result.Output)
				if len(result.Artifacts) > 0 {
					output += fmt.Sprintf("\nArtifacts: %v\n", result.Artifacts)
				}
			} else {
				output += fmt.Sprintf("\n✗ Error:\n%s\n", result.Error)
			}
		} else {
			output += "\n⏳ Task is still running...\n"
		}

		return output, nil
	}

	// Otherwise, show all active tasks
	activeTasks := coordinator.GetActiveTasks()

	if len(activeTasks) == 0 {
		return "No active background tasks.", nil
	}

	output := fmt.Sprintf("Active Tasks (%d):\n\n", len(activeTasks))
	for i, task := range activeTasks {
		output += fmt.Sprintf("%d. Task %s\n", i+1, task.ID)
		output += fmt.Sprintf("   Type: %s | Status: %s\n", task.Type, task.Status)
		output += fmt.Sprintf("   Description: %s\n", task.Description)

		// Phase 3: Show agent and model for active tasks
		if task.AgentName != "" && task.ModelUsed != "" {
			output += fmt.Sprintf("   Working: %s using %s\n", task.AgentName, task.ModelUsed)
		}
		output += "\n"
	}

	return output, nil
}

func init() {
	registry.Register(&CheckTaskProgressTool{})
}
