package orchestration

import (
	"context"
	"fmt"

	"wilson/agent/orchestration"
	"wilson/core/registry"
	. "wilson/core/types"
)

type OrchestrateCodeTaskTool struct{}

func (t *OrchestrateCodeTaskTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "orchestrate_code_task",
		Description:     "Route code/execution tasks to ManagerAgent for intelligent orchestration (auto-detects if decomposition needed)",
		Category:        "agent",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "request",
				Type:        "string",
				Required:    true,
				Description: "Full user request describing what needs to be done",
				Example:     "create a Go program that opens Spotify, write tests, and build",
			},
		},
		Examples: []string{
			`{"tool": "orchestrate_code_task", "arguments": {"request": "create a calculator in Go with tests"}}`,
			`{"tool": "orchestrate_code_task", "arguments": {"request": "build a CLI tool to fetch GitHub repos"}}`,
		},
	}
}

func (t *OrchestrateCodeTaskTool) Validate(args map[string]interface{}) error {
	request, ok := args["request"].(string)
	if !ok || request == "" {
		return fmt.Errorf("request parameter is required")
	}
	return nil
}

func (t *OrchestrateCodeTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	request, _ := args["request"].(string)

	// ✅ DEBUG: Log every orchestration call with stack trace hint
	fmt.Printf("\n[ORCHESTRATE_CODE_TASK] Called with request: %s\n", request)
	fmt.Printf("[ORCHESTRATE_CODE_TASK] This will trigger HandleUserRequest\n\n")

	coordinator := orchestration.GetGlobalCoordinator()
	if coordinator == nil {
		return "", fmt.Errorf("agent coordinator not initialized")
	}

	manager := coordinator.GetManager()
	if manager == nil {
		return "", fmt.Errorf("manager agent not initialized")
	}

	// Route to ManagerAgent - it decides decompose vs single-agent
	result, err := manager.HandleUserRequest(ctx, request)
	if err != nil {
		return "", fmt.Errorf("orchestration failed: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("task failed: %s", result.Error)
	}

	return result.Output, nil
}

func init() {
	registry.Register(&OrchestrateCodeTaskTool{})
}
