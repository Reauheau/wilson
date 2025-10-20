package ctxtools

import (
	"context"
	"fmt"

	ctx "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type StoreArtifactTool struct{}

func (t *StoreArtifactTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "store_artifact",
		Description:     "Store an artifact (finding, analysis, code) in the current or specified context",
		Category:        CategoryContext,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "content",
				Type:        "string",
				Required:    true,
				Description: "The content to store",
			},
			{
				Name:        "type",
				Type:        "string",
				Required:    true,
				Description: "Artifact type: web_search, analysis, summary, code, etc.",
				Example:     "analysis",
			},
			{
				Name:        "context_key",
				Type:        "string",
				Required:    false,
				Description: "Context to store in (uses active context if not specified)",
			},
			{
				Name:        "source",
				Type:        "string",
				Required:    false,
				Description: "Source of the artifact (URL, tool name, etc.)",
			},
		},
		Examples: []string{
			`{"tool": "store_artifact", "arguments": {"type": "analysis", "content": "Key findings: ..."}}`,
		},
	}
}

func (t *StoreArtifactTool) Validate(args map[string]interface{}) error {
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return fmt.Errorf("content parameter is required")
	}

	artifactType, ok := args["type"].(string)
	if !ok || artifactType == "" {
		return fmt.Errorf("type parameter is required")
	}

	return nil
}

func (t *StoreArtifactTool) Execute(ctxArg context.Context, args map[string]interface{}) (string, error) {
	manager := ctx.GetGlobalManager()
	if manager == nil {
		return "", fmt.Errorf("context manager not initialized")
	}

	content, _ := args["content"].(string)
	artifactType, _ := args["type"].(string)
	contextKey, _ := args["context_key"].(string)
	source, _ := args["source"].(string)

	req := ctx.StoreArtifactRequest{
		ContextKey: contextKey,
		Type:       artifactType,
		Content:    content,
		Source:     source,
		Agent:      "user",
	}

	artifact, err := manager.StoreArtifact(req)
	if err != nil {
		return "", fmt.Errorf("failed to store artifact: %w", err)
	}

	// Truncate content for display
	displayContent := content
	if len(displayContent) > 100 {
		displayContent = displayContent[:100] + "..."
	}

	result := fmt.Sprintf("✓ Stored artifact #%d\n", artifact.ID)
	result += fmt.Sprintf("  Type: %s\n", artifact.Type)
	result += fmt.Sprintf("  Content: %s\n", displayContent)
	result += fmt.Sprintf("  Context: %s\n", contextKey)

	return result, nil
}

func init() {
	registry.Register(&StoreArtifactTool{})
}
