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

type UpdateTaskProgressTool struct{}

func init() {
	registry.Register(&UpdateTaskProgressTool{})
}

func (t *UpdateTaskProgressTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "update_task_progress",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Update task progress and status during execution",
		Parameters: []Parameter{
			{
				Name:        "task_key",
				Type:        "string",
				Required:    true,
				Description: "Task key to update",
			},
			{
				Name:        "status",
				Type:        "string",
				Required:    true,
				Description: "New status: in_progress, completed",
			},
			{
				Name:        "progress_percent",
				Type:        "number",
				Required:    false,
				Description: "Progress percentage (0-100)",
			},
			{
				Name:        "progress_note",
				Type:        "string",
				Required:    false,
				Description: "Note about current progress",
			},
		},
		Examples: []string{
			`{"tool": "update_task_progress", "arguments": {"task_key": "TASK-001", "status": "in_progress", "progress_percent": 50}}`,
		},
	}
}

func (t *UpdateTaskProgressTool) Validate(args map[string]interface{}) error {
	taskKey, ok := args["task_key"].(string)
	if !ok || taskKey == "" {
		return fmt.Errorf("task_key is required")
	}

	status, ok := args["status"].(string)
	if !ok || status == "" {
		return fmt.Errorf("status is required")
	}

	validStatuses := map[string]bool{
		"in_progress": true,
		"completed":   true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s (must be: in_progress, completed)", status)
	}

	return nil
}

func (t *UpdateTaskProgressTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments
	taskKey, _ := args["task_key"].(string)
	status, _ := args["status"].(string)

	// Get optional parameters
	progressPercent := -1.0
	if progFloat, ok := args["progress_percent"].(float64); ok {
		if progFloat < 0 || progFloat > 100 {
			return "", fmt.Errorf("progress_percent must be between 0 and 100")
		}
		progressPercent = progFloat
	}

	progressNote, _ := args["progress_note"].(string)

	// Get current task info
	var currentStatus string
	var assignedTo sql.NullString
	err := db.QueryRowContext(ctx,
		"SELECT status, assigned_to FROM tasks WHERE task_key = ?",
		taskKey,
	).Scan(&currentStatus, &assignedTo)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("task not found: %s", taskKey)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}

	// Validate state transition
	validTransitions := map[string][]string{
		"claimed":       {"in_progress"},
		"in_progress":   {"in_progress", "completed"},
		"needs_changes": {"in_progress"},
	}

	allowedNext, ok := validTransitions[currentStatus]
	if !ok {
		return "", fmt.Errorf("cannot update progress from status: %s", currentStatus)
	}

	allowed := false
	for _, next := range allowedNext {
		if next == status {
			allowed = true
			break
		}
	}

	if !allowed {
		return "", fmt.Errorf("invalid state transition: %s → %s", currentStatus, status)
	}

	// Build update query
	updates := []string{"status = ?"}
	updateArgs := []interface{}{status}

	// Add started_at if moving to in_progress for first time
	if status == "in_progress" && currentStatus == "claimed" {
		updates = append(updates, "started_at = ?")
		updateArgs = append(updateArgs, time.Now())
	}

	// Add completed_at if moving to completed
	if status == "completed" {
		updates = append(updates, "completed_at = ?")
		updateArgs = append(updateArgs, time.Now())
	}

	// Update metadata with progress info
	if progressPercent >= 0 || progressNote != "" {
		// Get existing metadata
		var metadataJSON sql.NullString
		err := db.QueryRowContext(ctx,
			"SELECT metadata FROM tasks WHERE task_key = ?",
			taskKey,
		).Scan(&metadataJSON)
		if err != nil {
			return "", fmt.Errorf("failed to get metadata: %w", err)
		}

		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		} else {
			metadata = make(map[string]interface{})
		}

		// Update progress fields
		if progressPercent >= 0 {
			metadata["progress_percent"] = progressPercent
		}
		if progressNote != "" {
			metadata["progress_note"] = progressNote
			metadata["progress_updated_at"] = time.Now().Format(time.RFC3339)
		}

		metadataBytes, _ := json.Marshal(metadata)
		updates = append(updates, "metadata = ?")
		updateArgs = append(updateArgs, string(metadataBytes))
	}

	// Add WHERE clause
	updateQuery := "UPDATE tasks SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		updateQuery += ", " + updates[i]
	}
	updateQuery += " WHERE task_key = ?"
	updateArgs = append(updateArgs, taskKey)

	// Execute update
	_, err = db.ExecContext(ctx, updateQuery, updateArgs...)
	if err != nil {
		return "", fmt.Errorf("failed to update task: %w", err)
	}

	// Send notification on status change
	if status != currentStatus {
		agent := "unknown"
		if assignedTo.Valid {
			agent = assignedTo.String
		}

		messageContent := fmt.Sprintf("%s updated task %s: %s → %s", agent, taskKey, currentStatus, status)
		if progressNote != "" {
			messageContent += fmt.Sprintf("\nNote: %s", progressNote)
		}

		_, err = db.ExecContext(ctx,
			`INSERT INTO agent_communications (from_agent, to_agent, message_type, content, context_ref, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			agent,
			"Manager",
			"task_progress",
			messageContent,
			taskKey,
			time.Now(),
		)
		if err != nil {
			// Non-fatal, just log
			fmt.Printf("Warning: failed to send notification: %v\n", err)
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"task_key":        taskKey,
		"status":          status,
		"previous_status": currentStatus,
		"message":         fmt.Sprintf("✓ Task %s updated to %s", taskKey, status),
	}

	if progressPercent >= 0 {
		response["progress_percent"] = progressPercent
	}

	if progressNote != "" {
		response["progress_note"] = progressNote
	}

	if status == "completed" {
		response["next_step"] = "Request review or unblock dependent tasks"
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
