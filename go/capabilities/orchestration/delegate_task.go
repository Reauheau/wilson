package orchestration

import (
	"context"
	"fmt"
	"wilson/agent/orchestration"

	agentpkg "wilson/agent"
	"wilson/core/registry"
	. "wilson/core/types"
)

type DelegateTaskTool struct{}

func (t *DelegateTaskTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "delegate_task",
		Description:     "Delegate a task to another agent (analysis, code)",
		Category:        "agent",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "to_agent",
				Type:        "string",
				Required:    true,
				Description: "Target agent: analysis, code",
				Example:     "analysis",
			},
			{
				Name:        "task_type",
				Type:        "string",
				Required:    true,
				Description: "Task type: research, analysis, code, summary",
				Example:     "research",
			},
			{
				Name:        "description",
				Type:        "string",
				Required:    true,
				Description: "Task description",
			},
			{
				Name:        "context_key",
				Type:        "string",
				Required:    false,
				Description: "Context to work in (uses active if not specified)",
			},
		},
		Examples: []string{
			`{"tool": "delegate_task", "arguments": {"to_agent": "analysis", "task_type": "research", "description": "Research Ollama API endpoints"}}`,
		},
	}
}

func (t *DelegateTaskTool) Validate(args map[string]interface{}) error {
	toAgent, ok := args["to_agent"].(string)
	if !ok || toAgent == "" {
		return fmt.Errorf("to_agent parameter is required")
	}

	taskType, ok := args["task_type"].(string)
	if !ok || taskType == "" {
		return fmt.Errorf("task_type parameter is required")
	}

	description, ok := args["description"].(string)
	if !ok || description == "" {
		return fmt.Errorf("description parameter is required")
	}

	return nil
}

func (t *DelegateTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	coordinator := orchestration.GetGlobalCoordinator()
	if coordinator == nil {
		return "", fmt.Errorf("agent coordinator not initialized")
	}

	toAgent, _ := args["to_agent"].(string)
	taskType, _ := args["task_type"].(string)
	description, _ := args["description"].(string)
	contextKey, _ := args["context_key"].(string)

	req := agentpkg.DelegationRequest{
		ToAgent:     toAgent,
		ContextKey:  contextKey,
		Type:        taskType,
		Description: description,
		Priority:    3,
	}

	// Use async delegation - returns immediately, execution in background
	taskID, err := coordinator.DelegateTaskAsync(ctx, req)
	if err != nil {
		return "", fmt.Errorf("delegation failed: %w", err)
	}

	// Return immediately with task ID - Wilson never blocks!
	output := fmt.Sprintf("✓ Task %s started (delegated to %s agent)\n", taskID, toAgent)
	output += fmt.Sprintf("  Type: %s\n", taskType)
	output += fmt.Sprintf("  Description: %s\n", description)
	output += fmt.Sprintf("\nTask is running in background. Use check_task_progress to monitor.\n")

	return output, nil
}

func init() {
	registry.Register(&DelegateTaskTool{})
}
