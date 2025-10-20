package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"wilson/core/registry"
	. "wilson/core/types"
)

// AppendToFileTool appends content to the end of a file
type AppendToFileTool struct{}

func init() {
	registry.Register(&AppendToFileTool{})
}

func (t *AppendToFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "append_to_file",
		Description:     "Appends content to the end of an existing file. Perfect for adding new functions, imports, or sections.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskModerate,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Relative path to the file to append to",
				Example:     "src/utils.go",
			},
			{
				Name:        "content",
				Type:        "string",
				Required:    true,
				Description: "Content to append to the end of the file",
				Example:     "\nfunc NewHelper() string {\n\treturn \"helper\"\n}\n",
			},
		},
		Examples: []string{
			`{"tool": "append_to_file", "arguments": {"path": "utils.go", "content": "\nfunc Helper() {}\n"}}`,
		},
	}
}

func (t *AppendToFileTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	_, ok = args["content"].(string)
	if !ok {
		return fmt.Errorf("content parameter is required")
	}
	return nil
}

func (t *AppendToFileTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get path
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Get content
	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// Get workspace and resolve path
	workspace := GetSafeWorkspace()
	absPath := ResolvePath(path, workspace)

	// Security check
	if !filepath.HasPrefix(absPath, workspace) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	// Check file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s (use write_file to create new files)", path)
	}

	// Read existing content
	existingContent, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	originalSize := len(existingContent)

	// Append content
	newContent := append(existingContent, []byte(content)...)

	// Write back
	if err := os.WriteFile(absPath, newContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("file modified but failed to stat: %w", err)
	}

	result := map[string]interface{}{
		"success":       true,
		"path":          absPath,
		"original_size": originalSize,
		"appended_size": len(content),
		"new_size":      info.Size(),
		"message":       fmt.Sprintf("Successfully appended %d bytes to %s", len(content), path),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}
