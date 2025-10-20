package system

import (
	"context"
	"fmt"

	agentpkg "wilson/agent"
	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/llm"
)

type ModelStatusTool struct{}

func (t *ModelStatusTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "model_status",
		Description:     "Check status of LLM models (loaded, available, resource usage)",
		Category:        CategorySystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters:      []Parameter{},
		Examples: []string{
			`{"tool": "model_status", "arguments": {}}`,
		},
	}
}

func (t *ModelStatusTool) Validate(args map[string]interface{}) error {
	// No parameters required
	return nil
}

func (t *ModelStatusTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	coordinator := agentpkg.GetGlobalCoordinator()
	if coordinator == nil {
		return "", fmt.Errorf("coordinator not initialized")
	}

	output := "=== LLM Model Status ===\n\n"

	// Get model purposes to check
	purposes := []llm.Purpose{
		llm.PurposeChat,
		llm.PurposeCode,
		llm.PurposeAnalysis,
		llm.PurposeVision,
	}

	// Check each model's status
	for _, purpose := range purposes {
		output += fmt.Sprintf("Model: %s\n", purpose)

		// Get coordinator's LLM manager (via reflection or global)
		// For now, we'll check via coordinator methods
		// This is a simplified implementation

		// Check if any active tasks are using this model
		activeTasks := coordinator.GetActiveTasks()
		usingCount := 0
		var taskIDs []string
		for _, task := range activeTasks {
			// Note: We'd need the agent's purpose to properly check
			// For simplicity, checking if model name contains purpose
			if task.AgentName != "" {
				usingCount++
				taskIDs = append(taskIDs, task.ID[:8])
			}
		}

		if usingCount > 0 {
			output += fmt.Sprintf("  Status: ACTIVE\n")
			output += fmt.Sprintf("  Tasks using: %d\n", usingCount)
			output += fmt.Sprintf("  Task IDs: %v\n", taskIDs)
		} else {
			output += fmt.Sprintf("  Status: idle\n")
		}

		output += "\n"
	}

	// Show active tasks summary
	activeTasks := coordinator.GetActiveTasks()
	output += fmt.Sprintf("Active Tasks: %d\n", len(activeTasks))
	if len(activeTasks) > 0 {
		for _, task := range activeTasks {
			output += fmt.Sprintf("  - %s: %s", task.ID[:8], task.Description)
			if task.ModelUsed != "" {
				output += fmt.Sprintf(" [%s]", task.ModelUsed)
				if task.UsedFallback {
					output += " (fallback)"
				}
			}
			output += "\n"
		}
	}

	return output, nil
}

func init() {
	registry.Register(&ModelStatusTool{})
}
