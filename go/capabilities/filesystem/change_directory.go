package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// ChangeDirectoryTool changes the current working directory
type ChangeDirectoryTool struct{}

func (t *ChangeDirectoryTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "change_directory",
		Description:     "Changes the current working directory. Path must be relative to workspace root.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "relative path from workspace root to change to (use '.' for workspace root, '..' for parent)",
				Example:     "go",
			},
		},
		Examples: []string{
			`{"tool": "change_directory", "arguments": {"path": "go"}}`,
			`{"tool": "change_directory", "arguments": {"path": "."}}`,
			`{"tool": "change_directory", "arguments": {"path": ".."}}`,
		},
	}
}

func (t *ChangeDirectoryTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required and cannot be empty")
	}
	return nil
}

func (t *ChangeDirectoryTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		return "", fmt.Errorf("path is required")
	}

	workspace := GetSafeWorkspace()

	// Get current working directory to show in result
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "unknown"
	}

	// Handle special case for "."
	if pathArg == "." {
		pathArg = workspace
	}

	// Resolve the path
	fullPath := ResolvePath(pathArg, workspace)

	// Security check - ensure path is within workspace
	if !strings.HasPrefix(fullPath, workspace) {
		return "", fmt.Errorf("access denied: cannot navigate outside workspace (attempted: %s, workspace: %s)", fullPath, workspace)
	}

	// Check if directory exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory '%s' does not exist", pathArg)
		}
		return "", fmt.Errorf("error accessing directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("'%s' is not a directory", pathArg)
	}

	// Change directory
	if err := os.Chdir(fullPath); err != nil {
		return "", fmt.Errorf("failed to change directory: %w", err)
	}

	// Get new working directory for confirmation
	newDir, err := os.Getwd()
	if err != nil {
		newDir = fullPath
	}

	// Calculate relative path from workspace for display
	relPath, err := filepath.Rel(workspace, newDir)
	if err != nil {
		relPath = newDir
	}
	if relPath == "." {
		relPath = "workspace root"
	}

	result := map[string]interface{}{
		"success":      true,
		"previous_dir": currentDir,
		"current_dir":  newDir,
		"relative":     relPath,
		"message":      fmt.Sprintf("Changed directory to '%s'", relPath),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func init() {
	registry.Register(&ChangeDirectoryTool{})
}
