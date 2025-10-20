package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"wilson/core/registry"
	. "wilson/core/types"
)

type GetTaskQueueTool struct{}

func init() {
	registry.Register(&GetTaskQueueTool{})
}

func (t *GetTaskQueueTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "get_task_queue",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "View current state of the task queue with statistics",
		Parameters: []Parameter{
			{
				Name:        "status",
				Type:        "string",
				Required:    false,
				Description: "Filter by status (e.g., 'ready', 'in_progress', 'blocked')",
			},
			{
				Name:        "assigned_to",
				Type:        "string",
				Required:    false,
				Description: "Filter by assigned agent",
			},
			{
				Name:        "type",
				Type:        "string",
				Required:    false,
				Description: "Filter by task type (e.g., 'code', 'test', 'review')",
			},
			{
				Name:        "show_details",
				Type:        "boolean",
				Required:    false,
				Description: "Include detailed task information (default: true)",
			},
		},
		Examples: []string{
			`{"tool": "get_task_queue", "arguments": {"status": "ready"}}`,
			`{"tool": "get_task_queue", "arguments": {"assigned_to": "CodeAgent", "show_details": true}}`,
		},
	}
}

func (t *GetTaskQueueTool) Validate(args map[string]interface{}) error {
	// All parameters are optional
	return nil
}

func (t *GetTaskQueueTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract filters
	statusFilter, _ := args["status"].(string)
	assignedToFilter, _ := args["assigned_to"].(string)
	typeFilter, _ := args["type"].(string)
	showDetails := true
	if showBool, ok := args["show_details"].(bool); ok {
		showDetails = showBool
	}

	// Get overall statistics
	statsQuery := `
		SELECT
			COUNT(CASE WHEN status = 'new' THEN 1 END) as new_count,
			COUNT(CASE WHEN status = 'ready' THEN 1 END) as ready_count,
			COUNT(CASE WHEN status = 'claimed' THEN 1 END) as claimed_count,
			COUNT(CASE WHEN status = 'in_progress' THEN 1 END) as in_progress_count,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = 'in_review' THEN 1 END) as in_review_count,
			COUNT(CASE WHEN status = 'needs_changes' THEN 1 END) as needs_changes_count,
			COUNT(CASE WHEN status = 'blocked' THEN 1 END) as blocked_count,
			COUNT(CASE WHEN status = 'done' THEN 1 END) as done_count,
			COUNT(*) as total_count
		FROM tasks
	`

	var stats struct {
		New          int
		Ready        int
		Claimed      int
		InProgress   int
		Completed    int
		InReview     int
		NeedsChanges int
		Blocked      int
		Done         int
		Total        int
	}

	err := db.QueryRowContext(ctx, statsQuery).Scan(
		&stats.New,
		&stats.Ready,
		&stats.Claimed,
		&stats.InProgress,
		&stats.Completed,
		&stats.InReview,
		&stats.NeedsChanges,
		&stats.Blocked,
		&stats.Done,
		&stats.Total,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get statistics: %w", err)
	}

	// Build query for detailed task list
	query := `
		SELECT task_key, title, type, status, priority, assigned_to, created_at
		FROM tasks
		WHERE 1=1
	`
	var queryArgs []interface{}

	if statusFilter != "" {
		query += " AND status = ?"
		queryArgs = append(queryArgs, statusFilter)
	}

	if assignedToFilter != "" {
		query += " AND assigned_to = ?"
		queryArgs = append(queryArgs, assignedToFilter)
	}

	if typeFilter != "" {
		query += " AND type = ?"
		queryArgs = append(queryArgs, typeFilter)
	}

	// Order by priority and status
	query += " ORDER BY priority DESC, created_at ASC"

	// Get task list
	var tasks []map[string]interface{}

	if showDetails {
		rows, err := db.QueryContext(ctx, query, queryArgs...)
		if err != nil {
			return "", fmt.Errorf("failed to query tasks: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var taskKey string
			var title string
			var taskType string
			var status string
			var priority int
			var assignedTo sql.NullString
			var createdAt string

			err := rows.Scan(&taskKey, &title, &taskType, &status, &priority, &assignedTo, &createdAt)
			if err != nil {
				continue
			}

			task := map[string]interface{}{
				"task_key":   taskKey,
				"title":      title,
				"type":       taskType,
				"status":     status,
				"priority":   priority,
				"created_at": createdAt,
			}

			if assignedTo.Valid {
				task["assigned_to"] = assignedTo.String
			}

			tasks = append(tasks, task)
		}
	}

	// Prepare response
	response := map[string]interface{}{
		"queue_statistics": map[string]interface{}{
			"new":           stats.New,
			"ready":         stats.Ready,
			"claimed":       stats.Claimed,
			"in_progress":   stats.InProgress,
			"completed":     stats.Completed,
			"in_review":     stats.InReview,
			"needs_changes": stats.NeedsChanges,
			"blocked":       stats.Blocked,
			"done":          stats.Done,
			"total":         stats.Total,
		},
		"active_tasks":    stats.Claimed + stats.InProgress,
		"pending_tasks":   stats.Ready + stats.New,
		"completed_tasks": stats.Done,
	}

	if showDetails {
		response["tasks"] = tasks
		response["tasks_shown"] = len(tasks)
	}

	// Add filter info
	if statusFilter != "" || assignedToFilter != "" || typeFilter != "" {
		filters := make(map[string]string)
		if statusFilter != "" {
			filters["status"] = statusFilter
		}
		if assignedToFilter != "" {
			filters["assigned_to"] = assignedToFilter
		}
		if typeFilter != "" {
			filters["type"] = typeFilter
		}
		response["filters_applied"] = filters
	}

	// Add insights
	var insights []string
	if stats.Ready > 0 {
		insights = append(insights, fmt.Sprintf("%d tasks ready to be claimed", stats.Ready))
	}
	if stats.InProgress > 0 {
		insights = append(insights, fmt.Sprintf("%d tasks actively being worked on", stats.InProgress))
	}
	if stats.Blocked > 0 {
		insights = append(insights, fmt.Sprintf("%d tasks blocked by dependencies", stats.Blocked))
	}
	if stats.InReview > 0 {
		insights = append(insights, fmt.Sprintf("%d tasks awaiting review", stats.InReview))
	}
	if stats.NeedsChanges > 0 {
		insights = append(insights, fmt.Sprintf("%d tasks need changes after review", stats.NeedsChanges))
	}

	if len(insights) > 0 {
		response["insights"] = insights
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
