package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"wilson/core/registry"
	. "wilson/core/types"
)

type GetReviewStatusTool struct{}

func init() {
	registry.Register(&GetReviewStatusTool{})
}

func (t *GetReviewStatusTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "get_review_status",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Get review status and findings for a task",
		Parameters: []Parameter{
			{
				Name:        "task_key",
				Type:        "string",
				Required:    false,
				Description: "Task key to get review status for",
			},
			{
				Name:        "review_id",
				Type:        "number",
				Required:    false,
				Description: "Review ID to get status for (alternative to task_key)",
			},
		},
		Examples: []string{
			`{"tool": "get_review_status", "arguments": {"task_key": "TASK-001"}}`,
			`{"tool": "get_review_status", "arguments": {"review_id": 1}}`,
		},
	}
}

func (t *GetReviewStatusTool) Validate(args map[string]interface{}) error {
	taskKey, hasTaskKey := args["task_key"].(string)
	_, hasReviewID := args["review_id"].(float64)

	if !hasTaskKey && !hasReviewID {
		return fmt.Errorf("either task_key or review_id is required")
	}

	if hasTaskKey && taskKey == "" {
		return fmt.Errorf("task_key cannot be empty")
	}

	return nil
}

func (t *GetReviewStatusTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments - need either task_key or review_id
	taskKey, _ := args["task_key"].(string)
	reviewIDFloat, hasReviewID := args["review_id"].(float64)

	// Query based on what we have
	var query string
	var queryArgs []interface{}

	if hasReviewID {
		query = `
			SELECT r.id, r.task_id, t.task_key, r.reviewer_agent, r.review_type,
			       r.status, r.findings, r.comments, r.created_at,
			       t.status as task_status, t.review_status as task_review_status
			FROM task_reviews r
			JOIN tasks t ON r.task_id = t.id
			WHERE r.id = ?
		`
		queryArgs = []interface{}{int(reviewIDFloat)}
	} else {
		query = `
			SELECT r.id, r.task_id, t.task_key, r.reviewer_agent, r.review_type,
			       r.status, r.findings, r.comments, r.created_at,
			       t.status as task_status, t.review_status as task_review_status
			FROM task_reviews r
			JOIN tasks t ON r.task_id = t.id
			WHERE t.task_key = ?
			ORDER BY r.created_at DESC
			LIMIT 1
		`
		queryArgs = []interface{}{taskKey}
	}

	var reviewID int
	var taskID int
	var returnTaskKey string
	var reviewerAgent string
	var reviewType string
	var status string
	var findingsJSON sql.NullString
	var comments sql.NullString
	var createdAt string
	var taskStatus string
	var taskReviewStatus sql.NullString

	err := db.QueryRowContext(ctx, query, queryArgs...).Scan(
		&reviewID,
		&taskID,
		&returnTaskKey,
		&reviewerAgent,
		&reviewType,
		&status,
		&findingsJSON,
		&comments,
		&createdAt,
		&taskStatus,
		&taskReviewStatus,
	)

	if err == sql.ErrNoRows {
		if hasReviewID {
			return "", fmt.Errorf("review not found: %d", int(reviewIDFloat))
		}
		return "", fmt.Errorf("no review found for task: %s", taskKey)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get review: %w", err)
	}

	// Parse findings if present
	var findings []interface{}
	if findingsJSON.Valid && findingsJSON.String != "" {
		if err := json.Unmarshal([]byte(findingsJSON.String), &findings); err != nil {
			// If can't parse, just leave empty
			findings = []interface{}{}
		}
	}

	// Count findings by severity
	findingsBySeverity := map[string]int{
		"critical": 0,
		"error":    0,
		"warning":  0,
		"info":     0,
	}
	for _, finding := range findings {
		if findingMap, ok := finding.(map[string]interface{}); ok {
			if severity, ok := findingMap["severity"].(string); ok {
				findingsBySeverity[severity]++
			}
		}
	}

	// Get required changes if task status is needs_changes
	var requiredChanges []string
	if taskStatus == "needs_changes" {
		var metadataJSON sql.NullString
		err := db.QueryRowContext(ctx,
			"SELECT metadata FROM tasks WHERE id = ?",
			taskID,
		).Scan(&metadataJSON)
		if err == nil && metadataJSON.Valid {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
				if changes, ok := metadata["required_changes"].([]interface{}); ok {
					for _, change := range changes {
						if changeStr, ok := change.(string); ok {
							requiredChanges = append(requiredChanges, changeStr)
						}
					}
				}
			}
		}
	}

	// Build response
	response := map[string]interface{}{
		"review_id":            reviewID,
		"task_key":             returnTaskKey,
		"task_status":          taskStatus,
		"review_status":        status,
		"reviewer":             reviewerAgent,
		"review_type":          reviewType,
		"findings_count":       len(findings),
		"findings_by_severity": findingsBySeverity,
		"created_at":           createdAt,
	}

	// Add comments if present
	if comments.Valid {
		response["comments"] = comments.String
	}

	// Add task review status if present
	if taskReviewStatus.Valid {
		response["task_review_status"] = taskReviewStatus.String
	}

	// Add findings if present
	if len(findings) > 0 {
		response["findings"] = findings
	}

	// Add required changes if present
	if len(requiredChanges) > 0 {
		response["required_changes"] = requiredChanges
	}

	// Add status-specific messages
	switch status {
	case "pending":
		response["message"] = "Review is in progress"
	case "approved":
		response["message"] = "✓ Task has been approved!"
	case "needs_changes":
		response["message"] = fmt.Sprintf("⚠ Task needs changes (%d issues found)", len(findings))
	case "rejected":
		response["message"] = "✗ Task has been rejected"
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
