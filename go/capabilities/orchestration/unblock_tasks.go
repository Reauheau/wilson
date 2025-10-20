package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"wilson/core/registry"
	. "wilson/core/types"
)

type UnblockTasksTool struct{}

func init() {
	registry.Register(&UnblockTasksTool{})
}

func (t *UnblockTasksTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "unblock_tasks",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Unblock tasks that depend on a completed task",
		Parameters: []Parameter{
			{
				Name:        "completed_task_key",
				Type:        "string",
				Required:    true,
				Description: "Task key that was just completed",
			},
		},
		Examples: []string{
			`{"tool": "unblock_tasks", "arguments": {"completed_task_key": "TASK-001"}}`,
		},
	}
}

func (t *UnblockTasksTool) Validate(args map[string]interface{}) error {
	completedTaskKey, ok := args["completed_task_key"].(string)
	if !ok || completedTaskKey == "" {
		return fmt.Errorf("completed_task_key is required")
	}
	return nil
}

func (t *UnblockTasksTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments
	completedTaskKey, _ := args["completed_task_key"].(string)

	// Verify the task is actually completed/done
	var taskStatus string
	err := db.QueryRowContext(ctx,
		"SELECT status FROM tasks WHERE task_key = ?",
		completedTaskKey,
	).Scan(&taskStatus)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("task not found: %s", completedTaskKey)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}

	// Note: We'll accept 'completed' or 'done' as valid
	if taskStatus != "completed" && taskStatus != "done" {
		return "", fmt.Errorf("task %s is not completed (status: %s)", completedTaskKey, taskStatus)
	}

	// Find all tasks that might depend on this one
	// Query all blocked tasks
	rows, err := db.QueryContext(ctx,
		"SELECT id, task_key, depends_on FROM tasks WHERE status = 'blocked'",
	)
	if err != nil {
		return "", fmt.Errorf("failed to query blocked tasks: %w", err)
	}
	defer rows.Close()

	var unblockedTasks []string
	var stillBlockedTasks []string

	for rows.Next() {
		var taskID int
		var taskKey string
		var dependsOnJSON string

		if err := rows.Scan(&taskID, &taskKey, &dependsOnJSON); err != nil {
			continue
		}

		// Parse depends_on array
		var dependsOn []string
		if dependsOnJSON != "" && dependsOnJSON != "[]" {
			if err := json.Unmarshal([]byte(dependsOnJSON), &dependsOn); err != nil {
				continue
			}
		}

		// Check if this task depends on the completed task
		dependsOnCompleted := false
		for _, dep := range dependsOn {
			if dep == completedTaskKey {
				dependsOnCompleted = true
				break
			}
		}

		if !dependsOnCompleted {
			// This blocked task doesn't depend on our completed task
			continue
		}

		// Check if ALL dependencies are now satisfied
		allDone := true
		var blockingDeps []string

		for _, depKey := range dependsOn {
			var depStatus string
			err := db.QueryRowContext(ctx,
				"SELECT status FROM tasks WHERE task_key = ?", depKey,
			).Scan(&depStatus)
			if err != nil || depStatus != "done" {
				allDone = false
				if depStatus != "done" {
					blockingDeps = append(blockingDeps, fmt.Sprintf("%s (%s)", depKey, depStatus))
				}
			}
		}

		if allDone {
			// Unblock this task!
			_, err := db.ExecContext(ctx,
				"UPDATE tasks SET status = 'ready' WHERE id = ?",
				taskID,
			)
			if err != nil {
				fmt.Printf("Warning: failed to unblock task %s: %v\n", taskKey, err)
				continue
			}

			unblockedTasks = append(unblockedTasks, taskKey)

			// Send notification
			_, err = db.ExecContext(ctx,
				`INSERT INTO agent_communications (from_agent, to_agent, message_type, content, context_ref, created_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				"System",
				nil, // Broadcast
				"task_unblocked",
				fmt.Sprintf("Task %s is now ready! Dependency %s completed.", taskKey, completedTaskKey),
				taskKey,
				time.Now(),
			)
			if err != nil {
				fmt.Printf("Warning: failed to send notification: %v\n", err)
			}
		} else {
			// Still blocked by other dependencies
			stillBlockedTasks = append(stillBlockedTasks, taskKey)
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"completed_task":  completedTaskKey,
		"unblocked_tasks": unblockedTasks,
		"unblocked_count": len(unblockedTasks),
	}

	if len(stillBlockedTasks) > 0 {
		response["still_blocked_tasks"] = stillBlockedTasks
		response["still_blocked_count"] = len(stillBlockedTasks)
	}

	if len(unblockedTasks) == 0 {
		response["message"] = "No tasks were unblocked"
	} else if len(unblockedTasks) == 1 {
		response["message"] = fmt.Sprintf("✓ Unblocked 1 task: %s", unblockedTasks[0])
	} else {
		response["message"] = fmt.Sprintf("✓ Unblocked %d tasks", len(unblockedTasks))
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
