package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type SearchFilesTool struct{}

func (t *SearchFilesTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "search_files",
		Description:     "Search for files by name pattern in workspace",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "pattern",
				Type:        "string",
				Required:    true,
				Description: "filename pattern to search for",
				Example:     "*.go",
			},
		},
		Examples: []string{
			`{"tool": "search_files", "arguments": {"pattern": "*.go"}}`,
			`{"tool": "search_files", "arguments": {"pattern": "*.py"}}`,
			`{"tool": "search_files", "arguments": {"pattern": "main.go"}}`,
		},
	}
}

func (t *SearchFilesTool) Validate(args map[string]interface{}) error {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return fmt.Errorf("pattern parameter is required")
	}
	return nil
}

func (t *SearchFilesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern parameter required")
	}

	workspace := GetSafeWorkspace()
	var matches []string
	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and venv
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "venv") {
			return filepath.SkipDir
		}

		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			relPath, _ := filepath.Rel(workspace, path)
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error searching files: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Sprintf("No files found matching pattern: %s", pattern), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d file(s) matching '%s':\n", len(matches), pattern))
	for _, match := range matches {
		result.WriteString(fmt.Sprintf("  - %s\n", match))
	}

	return result.String(), nil
}

func init() {
	registry.Register(&SearchFilesTool{})
}
