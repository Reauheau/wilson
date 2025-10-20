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

type ClaimTaskTool struct{}

func init() {
	registry.Register(&ClaimTaskTool{})
}

func (t *ClaimTaskTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "claim_task",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Atomically claim a task to work on it (prevents race conditions)",
		Parameters: []Parameter{
			{
				Name:        "task_key",
				Type:        "string",
				Required:    true,
				Description: "Task key to claim",
			},
			{
				Name:        "agent_name",
				Type:        "string",
				Required:    true,
				Description: "Name of the agent claiming the task",
			},
		},
		Examples: []string{
			`{"tool": "claim_task", "arguments": {"task_key": "TASK-001", "agent_name": "CodeAgent"}}`,
		},
	}
}

func (t *ClaimTaskTool) Validate(args map[string]interface{}) error {
	taskKey, ok := args["task_key"].(string)
	if !ok || taskKey == "" {
		return fmt.Errorf("task_key is required")
	}

	agentName, ok := args["agent_name"].(string)
	if !ok || agentName == "" {
		return fmt.Errorf("agent_name is required")
	}

	return nil
}

func (t *ClaimTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments
	taskKey, _ := args["task_key"].(string)
	agentName, _ := args["agent_name"].(string)

	// Atomically claim the task
	// This UPDATE will only succeed if status is still 'ready'
	// Prevents race condition where two agents claim same task
	result, err := db.ExecContext(ctx,
		`UPDATE tasks
		SET status = 'claimed',
		    assigned_to = ?,
		    assigned_at = ?
		WHERE task_key = ?
		  AND status = 'ready'`,
		agentName,
		time.Now(),
		taskKey,
	)
	if err != nil {
		return "", fmt.Errorf("failed to claim task: %w", err)
	}

	// Check if claim was successful
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("failed to check claim result: %w", err)
	}

	// Prepare response
	var response map[string]interface{}

	if rowsAffected == 0 {
		// Task was not claimed (either doesn't exist, not ready, or already claimed)

		// Check why it failed
		var currentStatus string
		var currentAssignee sql.NullString
		err := db.QueryRowContext(ctx,
			"SELECT status, assigned_to FROM tasks WHERE task_key = ?",
			taskKey,
		).Scan(&currentStatus, &currentAssignee)

		if err == sql.ErrNoRows {
			response = map[string]interface{}{
				"task_key": taskKey,
				"claimed":  false,
				"agent":    agentName,
				"reason":   "task_not_found",
				"message":  fmt.Sprintf("✗ Task %s not found", taskKey),
			}
		} else if err == nil {
			// Task exists but couldn't be claimed
			if currentStatus == "claimed" || currentStatus == "in_progress" {
				assignee := "unknown"
				if currentAssignee.Valid {
					assignee = currentAssignee.String
				}
				response = map[string]interface{}{
					"task_key":      taskKey,
					"claimed":       false,
					"agent":         agentName,
					"reason":        "already_claimed",
					"current_agent": assignee,
					"message":       fmt.Sprintf("✗ Task already claimed by %s", assignee),
				}
			} else {
				response = map[string]interface{}{
					"task_key":       taskKey,
					"claimed":        false,
					"agent":          agentName,
					"reason":         "not_ready",
					"current_status": currentStatus,
					"message":        fmt.Sprintf("✗ Task not ready to claim (status: %s)", currentStatus),
				}
			}
		} else {
			return "", fmt.Errorf("failed to check task status: %w", err)
		}
	} else {
		// Successfully claimed!

		// Get task details
		var title string
		var taskType string
		var description sql.NullString
		err := db.QueryRowContext(ctx,
			"SELECT title, type, description FROM tasks WHERE task_key = ?",
			taskKey,
		).Scan(&title, &taskType, &description)
		if err != nil {
			return "", fmt.Errorf("failed to get task details: %w", err)
		}

		response = map[string]interface{}{
			"task_key":  taskKey,
			"claimed":   true,
			"agent":     agentName,
			"title":     title,
			"type":      taskType,
			"message":   fmt.Sprintf("✓ Task %s claimed successfully", taskKey),
			"next_step": "Update status to 'in_progress' and begin work",
		}

		if description.Valid {
			response["description"] = description.String
		}

		// Send notification to Manager
		_, err = db.ExecContext(ctx,
			`INSERT INTO agent_communications (from_agent, to_agent, message_type, content, context_ref, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			agentName,
			"Manager",
			"task_claimed",
			fmt.Sprintf("%s claimed task %s: %s", agentName, taskKey, title),
			taskKey,
			time.Now(),
		)
		if err != nil {
			// Non-fatal, just log
			fmt.Printf("Warning: failed to send notification: %v\n", err)
		}
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
