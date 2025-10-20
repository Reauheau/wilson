package ctxtools

import (
	"context"
	"fmt"

	ctx "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type SearchArtifactsTool struct{}

func (t *SearchArtifactsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "search_artifacts",
		Description:     "Search for artifacts across all contexts using full-text search",
		Category:        CategoryContext,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "query",
				Type:        "string",
				Required:    true,
				Description: "Search query (full-text search)",
				Example:     "ollama API",
			},
			{
				Name:        "context_key",
				Type:        "string",
				Required:    false,
				Description: "Limit search to specific context",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Required:    false,
				Description: "Maximum results to return (default: 10)",
			},
		},
		Examples: []string{
			`{"tool": "search_artifacts", "arguments": {"query": "ollama API endpoints"}}`,
		},
	}
}

func (t *SearchArtifactsTool) Validate(args map[string]interface{}) error {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return fmt.Errorf("query parameter is required")
	}
	return nil
}

func (t *SearchArtifactsTool) Execute(ctxArg context.Context, args map[string]interface{}) (string, error) {
	manager := ctx.GetGlobalManager()
	if manager == nil {
		return "", fmt.Errorf("context manager not initialized")
	}

	query, _ := args["query"].(string)
	contextKey, _ := args["context_key"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	req := ctx.SearchArtifactsRequest{
		Query:      query,
		ContextKey: contextKey,
		Limit:      limit,
	}

	results, err := manager.SearchArtifacts(req)
	if err != nil {
		return "", fmt.Errorf("failed to search artifacts: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No artifacts found matching '%s'", query), nil
	}

	output := fmt.Sprintf("Found %d artifacts matching '%s':\n\n", len(results), query)

	for i, result := range results {
		// Truncate content
		content := result.Artifact.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		output += fmt.Sprintf("%d. [%s] in context '%s'\n", i+1, result.Artifact.Type, result.Context.Title)
		output += fmt.Sprintf("   %s\n", content)
		output += fmt.Sprintf("   Context: %s | Agent: %s | Created: %s\n\n",
			result.Context.Key, result.Artifact.Agent, result.Artifact.CreatedAt.Format("2006-01-02 15:04"))
	}

	return output, nil
}

func init() {
	registry.Register(&SearchArtifactsTool{})
}
