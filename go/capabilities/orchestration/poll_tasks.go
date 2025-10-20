package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"wilson/core/registry"
	. "wilson/core/types"
)

type PollTasksTool struct{}

func init() {
	registry.Register(&PollTasksTool{})
}

func (t *PollTasksTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "poll_tasks",
		Category:        CategoryOrchestration,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Description:     "Poll for available tasks to work on (autonomous coordination)",
		Parameters: []Parameter{
			{
				Name:        "agent_name",
				Type:        "string",
				Required:    true,
				Description: "Name of the agent polling for tasks",
			},
			{
				Name:        "task_types",
				Type:        "array",
				Required:    false,
				Description: "Task types this agent can handle (e.g., ['code', 'refactor'])",
			},
			{
				Name:        "max_tasks",
				Type:        "number",
				Required:    false,
				Description: "Maximum number of tasks to return (default: 5)",
			},
			{
				Name:        "priority_threshold",
				Type:        "number",
				Required:    false,
				Description: "Minimum priority level (default: 0)",
			},
		},
		Examples: []string{
			`{"tool": "poll_tasks", "arguments": {"agent_name": "CodeAgent", "task_types": ["code"], "max_tasks": 5}}`,
		},
	}
}

func (t *PollTasksTool) Validate(args map[string]interface{}) error {
	agentName, ok := args["agent_name"].(string)
	if !ok || agentName == "" {
		return fmt.Errorf("agent_name is required")
	}
	return nil
}

func (t *PollTasksTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get database from context
	db, ok := ctx.Value("db").(*sql.DB)
	if !ok {
		return "", fmt.Errorf("database not found in context")
	}

	// Extract arguments
	agentName, _ := args["agent_name"].(string)

	// Parse task types
	var taskTypes []string
	if taskTypesRaw, ok := args["task_types"].([]interface{}); ok {
		for _, t := range taskTypesRaw {
			if typeStr, ok := t.(string); ok {
				taskTypes = append(taskTypes, typeStr)
			}
		}
	}

	// Get max tasks
	maxTasks := 5
	if maxFloat, ok := args["max_tasks"].(float64); ok {
		maxTasks = int(maxFloat)
	}

	// Get priority threshold
	priorityThreshold := 0
	if prioFloat, ok := args["priority_threshold"].(float64); ok {
		priorityThreshold = int(prioFloat)
	}

	// Build query
	query := `
		SELECT id, task_key, title, description, type, priority,
		       depends_on, created_at, metadata
		FROM tasks
		WHERE status = 'ready'
		  AND dor_met = 1
		  AND priority >= ?
	`
	queryArgs := []interface{}{priorityThreshold}

	// Add task type filter if specified
	if len(taskTypes) > 0 {
		placeholders := ""
		for i := range taskTypes {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			queryArgs = append(queryArgs, taskTypes[i])
		}
		query += fmt.Sprintf(" AND type IN (%s)", placeholders)
	}

	// Order by priority and age
	query += " ORDER BY priority DESC, created_at ASC LIMIT ?"
	queryArgs = append(queryArgs, maxTasks)

	// Execute query
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return "", fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	// Collect tasks
	var availableTasks []map[string]interface{}

	for rows.Next() {
		var id int
		var taskKey string
		var title string
		var description sql.NullString
		var taskType string
		var priority int
		var dependsOnJSON sql.NullString
		var createdAt string
		var metadataJSON sql.NullString

		err := rows.Scan(&id, &taskKey, &title, &description, &taskType, &priority,
			&dependsOnJSON, &createdAt, &metadataJSON)
		if err != nil {
			continue
		}

		// Check dependencies are actually satisfied
		if dependsOnJSON.Valid && dependsOnJSON.String != "[]" && dependsOnJSON.String != "" {
			var dependsOn []string
			if err := json.Unmarshal([]byte(dependsOnJSON.String), &dependsOn); err == nil {
				// Verify all dependencies are done
				allDone := true
				for _, depKey := range dependsOn {
					var depStatus string
					err := db.QueryRowContext(ctx,
						"SELECT status FROM tasks WHERE task_key = ?", depKey,
					).Scan(&depStatus)
					if err != nil || depStatus != "done" {
						allDone = false
						break
					}
				}

				// Skip if dependencies not met
				if !allDone {
					continue
				}
			}
		}

		// Parse metadata
		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		}

		// Extract estimated effort if available
		estimatedEffort := "unknown"
		if metadata != nil {
			if effort, ok := metadata["estimated_effort"].(string); ok {
				estimatedEffort = effort
			}
		}

		task := map[string]interface{}{
			"task_key":         taskKey,
			"title":            title,
			"type":             taskType,
			"priority":         priority,
			"created_at":       createdAt,
			"estimated_effort": estimatedEffort,
		}

		if description.Valid {
			task["description"] = description.String
		}

		// Parse and include dependencies
		if dependsOnJSON.Valid && dependsOnJSON.String != "" && dependsOnJSON.String != "[]" {
			var deps []string
			if err := json.Unmarshal([]byte(dependsOnJSON.String), &deps); err == nil {
				task["dependencies"] = deps
			} else {
				task["dependencies"] = []string{}
			}
		} else {
			task["dependencies"] = []string{}
		}

		availableTasks = append(availableTasks, task)
	}

	// Prepare response
	response := map[string]interface{}{
		"agent":           agentName,
		"available_tasks": availableTasks,
		"count":           len(availableTasks),
	}

	if len(availableTasks) == 0 {
		response["message"] = "No available tasks found"
		response["suggestion"] = "Check back later or adjust filters"
	} else if len(availableTasks) == 1 {
		response["message"] = "Found 1 available task"
	} else {
		response["message"] = fmt.Sprintf("Found %d available tasks", len(availableTasks))
	}

	// Add polling info
	response["poll_info"] = map[string]interface{}{
		"task_types_filter":   taskTypes,
		"priority_threshold":  priorityThreshold,
		"max_tasks_requested": maxTasks,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonResponse), nil
}
