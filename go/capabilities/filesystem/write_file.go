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

// WriteFileTool writes content to a file
type WriteFileTool struct{}

func init() {
	registry.Register(&WriteFileTool{})
}

func (t *WriteFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "write_file",
		Description:     "Writes content to a file. Creates the file if it doesn't exist, or overwrites if it does.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskModerate, // Writing files has moderate risk
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Relative path to the file to write",
				Example:     "output/result.txt",
			},
			{
				Name:        "content",
				Type:        "string",
				Required:    true,
				Description: "Content to write to the file",
				Example:     "Hello, World!",
			},
			{
				Name:        "create_dirs",
				Type:        "boolean",
				Required:    false,
				Description: "Create parent directories if they don't exist (default: true)",
				Example:     "true",
			},
		},
		Examples: []string{
			`{"tool": "write_file", "arguments": {"path": "output/result.txt", "content": "Hello World"}}`,
		},
	}
}

func (t *WriteFileTool) Validate(args map[string]interface{}) error {
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

func (t *WriteFileTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
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

	// Check create_dirs flag (default true)
	createDirs := true
	if cd, ok := input["create_dirs"].(bool); ok {
		createDirs = cd
	}

	// Get workspace and resolve path
	workspace := GetSafeWorkspace()
	absPath := ResolvePath(path, workspace)

	// Security check
	if !filepath.HasPrefix(absPath, workspace) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	// Create parent directories if needed
	if createDirs {
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("file written but failed to stat: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"path":    absPath,
		"size":    info.Size(),
		"message": fmt.Sprintf("Successfully wrote %d bytes to %s", info.Size(), path),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}
