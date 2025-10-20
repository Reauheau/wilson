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

type RequestReviewTool struct{}

func init() {
	registry.Register(&RequestReviewTool{})
}

func (t *RequestReviewTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "request_review",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Request review of a completed task",
		Parameters: []Parameter{
			{
				Name:        "task_key",
				Type:        "string",
				Required:    true,
				Description: "Task key to request review for (e.g., TASK-123)",
			},
			{
				Name:        "review_type",
				Type:        "string",
				Required:    false,
				Description: "Type of review: quality (default), security, performance",
			},
			{
				Name:        "notes",
				Type:        "string",
				Required:    false,
				Description: "Notes for the reviewer about the implementation",
			},
		},
		Examples: []string{
			`{"tool": "request_review", "arguments": {"task_key": "TASK-001", "review_type": "quality"}}`,
		},
	}
}

func (t *RequestReviewTool) Validate(args map[string]interface{}) error {
	taskKey, ok := args["task_key"].(string)
	if !ok || taskKey == "" {
		return fmt.Errorf("task_key is required")
	}
	return nil
}

func (t *RequestReviewTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments
	taskKey, _ := args["task_key"].(string)

	reviewType, _ := args["review_type"].(string)
	if reviewType == "" {
		reviewType = "quality"
	}

	notes, _ := args["notes"].(string)

	// Validate review type
	validTypes := map[string]bool{
		"quality":     true,
		"security":    true,
		"performance": true,
		"code":        true,
	}
	if !validTypes[reviewType] {
		return "", fmt.Errorf("invalid review_type: %s (must be: quality, security, performance, code)", reviewType)
	}

	// Get task
	var taskID int
	var status string
	var assignedTo string
	err := db.QueryRowContext(ctx,
		"SELECT id, status, assigned_to FROM tasks WHERE task_key = ?",
		taskKey,
	).Scan(&taskID, &status, &assignedTo)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("task not found: %s", taskKey)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get task: %w", err)
	}

	// Validate task status
	if status != "completed" {
		return "", fmt.Errorf("task must be in 'completed' status to request review (current: %s)", status)
	}

	// Create review record
	result, err := db.ExecContext(ctx,
		`INSERT INTO task_reviews (task_id, reviewer_agent, review_type, status, comments, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		taskID,
		"Review", // Assign to Review Agent
		reviewType,
		"pending",
		notes,
		time.Now(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create review: %w", err)
	}

	reviewID, _ := result.LastInsertId()

	// Update task status to in_review
	_, err = db.ExecContext(ctx,
		`UPDATE tasks
		SET status = ?, review_status = ?
		WHERE id = ?`,
		"in_review",
		"pending",
		taskID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to update task status: %w", err)
	}

	// Send notification to Review Agent
	_, err = db.ExecContext(ctx,
		`INSERT INTO agent_communications (from_agent, to_agent, message_type, content, context_ref, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		assignedTo,
		"Review",
		"review_request",
		fmt.Sprintf("Review requested for %s (%s review). Notes: %s", taskKey, reviewType, notes),
		taskKey,
		time.Now(),
	)
	if err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to send notification: %v\n", err)
	}

	// Prepare response
	response := map[string]interface{}{
		"review_id":   reviewID,
		"task_key":    taskKey,
		"review_type": reviewType,
		"status":      "pending",
		"message":     fmt.Sprintf("✓ Review requested for %s. Review ID: %d", taskKey, reviewID),
		"next_step":   "Review Agent will evaluate the task and provide feedback",
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
