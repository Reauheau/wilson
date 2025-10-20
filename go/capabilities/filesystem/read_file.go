package filesystem

import (
	"context"
	"fmt"
	"os"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type ReadFileTool struct{}

func (t *ReadFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "read_file",
		Description:     "Read the contents of a file (relative to workspace)",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "relative file path from workspace root",
				Example:     "go/main.go",
			},
		},
		Examples: []string{
			`{"tool": "read_file", "arguments": {"path": "go/main.go"}}`,
		},
	}
}

func (t *ReadFileTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	return nil
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path parameter required")
	}

	workspace := GetSafeWorkspace()
	fullPath := ResolvePath(pathArg, workspace)

	// Security check
	if !strings.HasPrefix(fullPath, workspace) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Limit file size to prevent overwhelming the model
	if len(content) > 10000 {
		content = content[:10000]
		return string(content) + "\n... (file truncated, showing first 10KB)", nil
	}

	return string(content), nil
}

func init() {
	registry.Register(&ReadFileTool{})
}
