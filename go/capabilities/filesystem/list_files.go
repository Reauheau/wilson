package filesystem

import (
	"context"
	"fmt"
	"os"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type ListFilesTool struct{}

func (t *ListFilesTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "list_files",
		Description:     "List all files and directories in a given path (relative to workspace)",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "relative path from workspace root",
				Default:     ".",
				Example:     "go",
			},
		},
		Examples: []string{
			`{"tool": "list_files", "arguments": {"path": "go"}}`,
			`{"tool": "list_files", "arguments": {"path": "."}}`,
		},
	}
}

func (t *ListFilesTool) Validate(args map[string]interface{}) error {
	// Path is optional, defaults to "."
	if path, ok := args["path"].(string); ok {
		if path == "" {
			return fmt.Errorf("path cannot be empty")
		}
	}
	return nil
}

func (t *ListFilesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		pathArg = "."
	}

	workspace := GetSafeWorkspace()
	fullPath := ResolvePath(pathArg, workspace)

	// Security check - must be within workspace
	if !strings.HasPrefix(fullPath, workspace) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("error reading directory: %w", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of %s:\n", pathArg))
	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("  [DIR]  %s\n", entry.Name()))
		} else {
			info, _ := entry.Info()
			result.WriteString(fmt.Sprintf("  [FILE] %s (%d bytes)\n", entry.Name(), info.Size()))
		}
	}

	return result.String(), nil
}

func init() {
	registry.Register(&ListFilesTool{})
}
