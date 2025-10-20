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

type SubmitReviewTool struct{}

func init() {
	registry.Register(&SubmitReviewTool{})
}

func (t *SubmitReviewTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "submit_review",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Submit review findings for a task",
		Parameters: []Parameter{
			{
				Name:        "review_id",
				Type:        "number",
				Required:    true,
				Description: "Review ID to submit findings for",
			},
			{
				Name:        "status",
				Type:        "string",
				Required:    true,
				Description: "Review status: approved, needs_changes, rejected",
			},
			{
				Name:        "findings",
				Type:        "array",
				Required:    false,
				Description: "Array of findings: [{category, severity, issue, location}]",
			},
			{
				Name:        "comments",
				Type:        "string",
				Required:    true,
				Description: "Overall review comments and summary",
			},
			{
				Name:        "required_changes",
				Type:        "array",
				Required:    false,
				Description: "List of specific changes required (if needs_changes)",
			},
		},
		Examples: []string{
			`{"tool": "submit_review", "arguments": {"review_id": 1, "status": "approved", "comments": "Code looks good"}}`,
		},
	}
}

func (t *SubmitReviewTool) Validate(args map[string]interface{}) error {
	reviewIDFloat, ok := args["review_id"].(float64)
	if !ok || reviewIDFloat <= 0 {
		return fmt.Errorf("review_id is required")
	}

	status, ok := args["status"].(string)
	if !ok || status == "" {
		return fmt.Errorf("status is required")
	}

	comments, ok := args["comments"].(string)
	if !ok || comments == "" {
		return fmt.Errorf("comments are required")
	}

	validStatuses := map[string]bool{
		"approved":      true,
		"needs_changes": true,
		"rejected":      true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s (must be: approved, needs_changes, rejected)", status)
	}

	return nil
}

func (t *SubmitReviewTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments
	reviewIDFloat, _ := args["review_id"].(float64)
	reviewID := int(reviewIDFloat)
	status, _ := args["status"].(string)
	comments, _ := args["comments"].(string)

	// Get findings if provided
	var findingsJSON string
	if findings, ok := args["findings"].([]interface{}); ok && len(findings) > 0 {
		findingsBytes, err := json.Marshal(findings)
		if err != nil {
			return "", fmt.Errorf("failed to marshal findings: %w", err)
		}
		findingsJSON = string(findingsBytes)
	}

	// Get required changes if provided
	var requiredChanges []string
	if changes, ok := args["required_changes"].([]interface{}); ok {
		for _, change := range changes {
			if changeStr, ok := change.(string); ok {
				requiredChanges = append(requiredChanges, changeStr)
			}
		}
	}

	// Get task info from review
	var taskID int
	var taskKey string
	var assignedTo string
	err := db.QueryRowContext(ctx,
		`SELECT t.id, t.task_key, t.assigned_to
		FROM task_reviews r
		JOIN tasks t ON r.task_id = t.id
		WHERE r.id = ?`,
		reviewID,
	).Scan(&taskID, &taskKey, &assignedTo)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("review not found: %d", reviewID)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get review: %w", err)
	}

	// Update review record
	_, err = db.ExecContext(ctx,
		`UPDATE task_reviews
		SET status = ?, findings = ?, comments = ?
		WHERE id = ?`,
		status,
		findingsJSON,
		comments,
		reviewID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to update review: %w", err)
	}

	// Update task based on review result
	var taskStatus string
	var reviewStatus string
	var dodMet bool

	switch status {
	case "approved":
		taskStatus = "done"
		reviewStatus = "approved"
		dodMet = true
	case "needs_changes":
		taskStatus = "needs_changes"
		reviewStatus = "needs_changes"
		dodMet = false
	case "rejected":
		taskStatus = "rejected"
		reviewStatus = "rejected"
		dodMet = false
	}

	// Store required changes in task metadata if needs_changes
	var metadataJSON string
	if status == "needs_changes" && len(requiredChanges) > 0 {
		metadata := map[string]interface{}{
			"required_changes": requiredChanges,
			"review_id":        reviewID,
		}
		metadataBytes, _ := json.Marshal(metadata)
		metadataJSON = string(metadataBytes)
	}

	// Update task
	query := `UPDATE tasks
		SET status = ?, review_status = ?, review_comments = ?, reviewer = ?, dod_met = ?`
	params := []interface{}{taskStatus, reviewStatus, comments, "Review", dodMet}

	if metadataJSON != "" {
		query += `, metadata = ?`
		params = append(params, metadataJSON)
	}

	if status == "approved" {
		query += `, completed_at = ?`
		params = append(params, time.Now())
	}

	query += ` WHERE id = ?`
	params = append(params, taskID)

	_, err = db.ExecContext(ctx, query, params...)
	if err != nil {
		return "", fmt.Errorf("failed to update task: %w", err)
	}

	// Send notification to assigned agent
	var notificationContent string
	switch status {
	case "approved":
		notificationContent = fmt.Sprintf("✓ Your task %s has been APPROVED! Excellent work.", taskKey)
	case "needs_changes":
		changesStr := ""
		if len(requiredChanges) > 0 {
			changesStr = fmt.Sprintf("\nRequired changes:\n")
			for i, change := range requiredChanges {
				changesStr += fmt.Sprintf("  %d. %s\n", i+1, change)
			}
		}
		notificationContent = fmt.Sprintf("⚠ Task %s NEEDS CHANGES before approval.%s\nComments: %s",
			taskKey, changesStr, comments)
	case "rejected":
		notificationContent = fmt.Sprintf("✗ Task %s has been REJECTED. Comments: %s", taskKey, comments)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO agent_communications (from_agent, to_agent, message_type, content, context_ref, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"Review",
		assignedTo,
		"review_result",
		notificationContent,
		taskKey,
		time.Now(),
	)
	if err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to send notification: %v\n", err)
	}

	// If approved, unblock dependent tasks
	if status == "approved" {
		// Get tasks that depend on this one
		rows, err := db.QueryContext(ctx,
			`SELECT id, task_key, depends_on FROM tasks WHERE status = 'blocked'`,
		)
		if err == nil {
			defer rows.Close()

			for rows.Next() {
				var depTaskID int
				var depTaskKey string
				var dependsOnJSON string

				if err := rows.Scan(&depTaskID, &depTaskKey, &dependsOnJSON); err != nil {
					continue
				}

				// Parse depends_on array
				var dependsOn []string
				if err := json.Unmarshal([]byte(dependsOnJSON), &dependsOn); err != nil {
					continue
				}

				// Check if this task was blocking the dependent task
				isBlocking := false
				for _, dep := range dependsOn {
					if dep == taskKey {
						isBlocking = true
						break
					}
				}

				if isBlocking {
					// Check if all dependencies are now done
					allDone := true
					for _, dep := range dependsOn {
						var depStatus string
						err := db.QueryRowContext(ctx,
							"SELECT status FROM tasks WHERE task_key = ?", dep,
						).Scan(&depStatus)
						if err != nil || depStatus != "done" {
							allDone = false
							break
						}
					}

					// Unblock if all dependencies done
					if allDone {
						_, _ = db.ExecContext(ctx,
							"UPDATE tasks SET status = 'ready' WHERE id = ?",
							depTaskID,
						)
					}
				}
			}
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"review_id":     reviewID,
		"task_key":      taskKey,
		"review_status": status,
		"task_status":   taskStatus,
		"message":       notificationContent,
	}

	if len(requiredChanges) > 0 {
		response["changes_required"] = len(requiredChanges)
		response["required_changes"] = requiredChanges
	}

	if status == "approved" {
		response["next_step"] = "Task completed! Check for unblocked dependent tasks."
	} else if status == "needs_changes" {
		response["next_step"] = "Agent should fix issues and request review again."
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
