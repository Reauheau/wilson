package ctxtools

import (
	"context"
	"fmt"

	ctx "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type ListContextsTool struct{}

func (t *ListContextsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "list_contexts",
		Description:     "List all contexts with optional status filtering",
		Category:        CategoryContext,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "status",
				Type:        "string",
				Required:    false,
				Description: "Filter by status: active, completed, archived",
				Example:     "active",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Required:    false,
				Description: "Maximum results (default: 20)",
			},
		},
		Examples: []string{
			`{"tool": "list_contexts", "arguments": {"status": "active"}}`,
		},
	}
}

func (t *ListContextsTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *ListContextsTool) Execute(ctxArg context.Context, args map[string]interface{}) (string, error) {
	manager := ctx.GetGlobalManager()
	if manager == nil {
		return "", fmt.Errorf("context manager not initialized")
	}

	status, _ := args["status"].(string)
	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	contexts, err := manager.ListContexts(status, limit)
	if err != nil {
		return "", fmt.Errorf("failed to list contexts: %w", err)
	}

	if len(contexts) == 0 {
		if status != "" {
			return fmt.Sprintf("No contexts found with status '%s'", status), nil
		}
		return "No contexts found. Create one with create_context tool.", nil
	}

	output := fmt.Sprintf("Contexts (%d):\n\n", len(contexts))

	activeContext := manager.GetActiveContext()

	for i, context := range contexts {
		active := ""
		if context.Key == activeContext {
			active = " [ACTIVE]"
		}

		output += fmt.Sprintf("%d. %s%s\n", i+1, context.Title, active)
		output += fmt.Sprintf("   Key: %s | Type: %s | Status: %s\n", context.Key, context.Type, context.Status)
		output += fmt.Sprintf("   Updated: %s | Artifacts: %d\n", context.UpdatedAt.Format("2006-01-02 15:04"), len(context.Artifacts))

		if context.Description != "" {
			desc := context.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			output += fmt.Sprintf("   %s\n", desc)
		}

		output += "\n"
	}

	return output, nil
}

func init() {
	registry.Register(&ListContextsTool{})
}
