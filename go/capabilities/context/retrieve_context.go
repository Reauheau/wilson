package ctxtools

import (
	"context"
	"fmt"

	ctx "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type RetrieveContextTool struct{}

func (t *RetrieveContextTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "retrieve_context",
		Description:     "Retrieve a context with all its artifacts and notes",
		Category:        CategoryContext,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "key",
				Type:        "string",
				Required:    true,
				Description: "Context key to retrieve",
				Example:     "ollama-research",
			},
		},
		Examples: []string{
			`{"tool": "retrieve_context", "arguments": {"key": "ollama-research"}}`,
		},
	}
}

func (t *RetrieveContextTool) Validate(args map[string]interface{}) error {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return fmt.Errorf("key parameter is required")
	}
	return nil
}

func (t *RetrieveContextTool) Execute(ctxArg context.Context, args map[string]interface{}) (string, error) {
	manager := ctx.GetGlobalManager()
	if manager == nil {
		return "", fmt.Errorf("context manager not initialized")
	}

	key, _ := args["key"].(string)

	context, err := manager.GetContext(key)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve context: %w", err)
	}

	result := fmt.Sprintf("Context: %s\n", context.Title)
	result += fmt.Sprintf("Key: %s | Type: %s | Status: %s\n", context.Key, context.Type, context.Status)
	result += fmt.Sprintf("Created: %s | Updated: %s\n\n", context.CreatedAt.Format("2006-01-02 15:04"), context.UpdatedAt.Format("2006-01-02 15:04"))

	if context.Description != "" {
		result += fmt.Sprintf("Description: %s\n\n", context.Description)
	}

	result += fmt.Sprintf("=== Artifacts (%d) ===\n", len(context.Artifacts))
	for i, artifact := range context.Artifacts {
		// Truncate content
		content := artifact.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}

		result += fmt.Sprintf("\n%d. [%s] by %s\n", i+1, artifact.Type, artifact.Agent)
		result += fmt.Sprintf("   %s\n", content)
		result += fmt.Sprintf("   (Created: %s)\n", artifact.CreatedAt.Format("2006-01-02 15:04"))
	}

	if len(context.Notes) > 0 {
		result += fmt.Sprintf("\n=== Agent Notes (%d) ===\n", len(context.Notes))
		for i, note := range context.Notes {
			result += fmt.Sprintf("\n%d. %s → %s:\n", i+1, note.FromAgent, note.ToAgent)
			result += fmt.Sprintf("   %s\n", note.Note)
		}
	}

	return result, nil
}

func init() {
	registry.Register(&RetrieveContextTool{})
}
