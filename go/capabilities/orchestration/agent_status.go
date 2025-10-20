package orchestration

import (
	"context"
	"fmt"

	agentpkg "wilson/agent"
	"wilson/core/registry"
	. "wilson/core/types"
)

type AgentStatusTool struct{}

func (t *AgentStatusTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "agent_status",
		Description:     "Check status of agents and tasks",
		Category:        "agent",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "task_id",
				Type:        "string",
				Required:    false,
				Description: "Task ID to check (if not provided, lists all agents)",
			},
		},
		Examples: []string{
			`{"tool": "agent_status", "arguments": {}}`,
		},
	}
}

func (t *AgentStatusTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *AgentStatusTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	coordinator := agentpkg.GetGlobalCoordinator()
	registry := agentpkg.GetGlobalRegistry()

	if coordinator == nil || registry == nil {
		return "", fmt.Errorf("agent system not initialized")
	}

	taskID, hasTaskID := args["task_id"].(string)

	if hasTaskID && taskID != "" {
		// Check specific task
		task, result, err := coordinator.GetTaskStatus(taskID)
		if err != nil {
			return "", err
		}

		output := fmt.Sprintf("Task: %s\n", task.ID)
		output += fmt.Sprintf("  Type: %s\n", task.Type)
		output += fmt.Sprintf("  Description: %s\n", task.Description)
		output += fmt.Sprintf("  Status: %s\n", task.Status)
		output += fmt.Sprintf("  Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))

		if result != nil {
			output += fmt.Sprintf("\nResult:\n")
			output += fmt.Sprintf("  Success: %v\n", result.Success)
			output += fmt.Sprintf("  Agent: %s\n", result.Agent)
			if result.Error != "" {
				output += fmt.Sprintf("  Error: %s\n", result.Error)
			}
			if len(result.Artifacts) > 0 {
				output += fmt.Sprintf("  Artifacts: %v\n", result.Artifacts)
			}
		}

		return output, nil
	}

	// List all agents
	agentInfos := registry.ListInfo()

	output := fmt.Sprintf("Available Agents (%d):\n\n", len(agentInfos))

	for _, info := range agentInfos {
		output += fmt.Sprintf("• %s (%s)\n", info.Name, info.Purpose)
		output += fmt.Sprintf("  Status: %s\n", info.Status)
		if len(info.AllowedTools) > 0 && info.AllowedTools[0] != "*" {
			output += fmt.Sprintf("  Tools: %v\n", info.AllowedTools)
		} else {
			output += "  Tools: all\n"
		}
		output += "\n"
	}

	// List recent tasks
	tasks := coordinator.ListTasks()
	if len(tasks) > 0 {
		output += fmt.Sprintf("Recent Tasks (%d):\n\n", len(tasks))
		count := 0
		for i := len(tasks) - 1; i >= 0 && count < 5; i-- {
			task := tasks[i]
			output += fmt.Sprintf("%d. [%s] %s - %s\n", count+1, task.Status, task.Type, task.Description[:min(50, len(task.Description))])
			count++
		}
	}

	return output, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	registry.Register(&AgentStatusTool{})
}
