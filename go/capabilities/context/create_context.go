package ctxtools

import (
	"context"
	"fmt"

	ctx "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type CreateContextTool struct{}

func (t *CreateContextTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "create_context",
		Description:     "Create a new context for storing related work (tasks, research, etc.)",
		Category:        CategoryContext,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "key",
				Type:        "string",
				Required:    true,
				Description: "Unique identifier for the context",
				Example:     "ollama-integration",
			},
			{
				Name:        "type",
				Type:        "string",
				Required:    false,
				Description: "Context type: task, research, analysis, code, session",
				Example:     "research",
			},
			{
				Name:        "title",
				Type:        "string",
				Required:    true,
				Description: "Human-readable title",
				Example:     "Ollama API Integration Research",
			},
			{
				Name:        "description",
				Type:        "string",
				Required:    false,
				Description: "Detailed description of the context",
			},
		},
		Examples: []string{
			`{"tool": "create_context", "arguments": {"key": "ollama-research", "type": "research", "title": "Research Ollama API"}}`,
		},
	}
}

func (t *CreateContextTool) Validate(args map[string]interface{}) error {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return fmt.Errorf("key parameter is required")
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		return fmt.Errorf("title parameter is required")
	}

	return nil
}

func (t *CreateContextTool) Execute(ctxArg context.Context, args map[string]interface{}) (string, error) {
	manager := ctx.GetGlobalManager()
	if manager == nil {
		return "", fmt.Errorf("context manager not initialized")
	}

	key, _ := args["key"].(string)
	title, _ := args["title"].(string)
	contextType, _ := args["type"].(string)
	description, _ := args["description"].(string)

	if contextType == "" {
		contextType = ctx.TypeTask
	}

	req := ctx.CreateContextRequest{
		Key:         key,
		Type:        contextType,
		Title:       title,
		Description: description,
		CreatedBy:   "user",
	}

	context, err := manager.CreateContext(req, true)
	if err != nil {
		return "", fmt.Errorf("failed to create context: %w", err)
	}

	result := fmt.Sprintf("✓ Created context: %s\n", context.Title)
	result += fmt.Sprintf("  Key: %s\n", context.Key)
	result += fmt.Sprintf("  Type: %s\n", context.Type)
	result += fmt.Sprintf("  Status: %s\n", context.Status)
	result += "\nThis context is now active. All tool results will be stored here."

	return result, nil
}

func init() {
	registry.Register(&CreateContextTool{})
}
