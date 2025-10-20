package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// MakeDirectoryTool creates a new directory or directories
type MakeDirectoryTool struct{}

func (t *MakeDirectoryTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "make_directory",
		Description:     "Creates a new directory. Can create nested directories if parent_dirs is true.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "relative path from workspace root for the directory to create",
				Example:     "new_folder",
			},
			{
				Name:        "parent_dirs",
				Type:        "boolean",
				Required:    false,
				Description: "create parent directories if they don't exist (default: true, like mkdir -p)",
				Default:     "true",
				Example:     "true",
			},
		},
		Examples: []string{
			`{"tool": "make_directory", "arguments": {"path": "new_folder"}}`,
			`{"tool": "make_directory", "arguments": {"path": "parent/child/grandchild", "parent_dirs": true}}`,
		},
	}
}

func (t *MakeDirectoryTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required and cannot be empty")
	}
	return nil
}

func (t *MakeDirectoryTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		return "", fmt.Errorf("path is required")
	}

	// Get parent_dirs flag (default true)
	parentDirs := true
	if pd, ok := args["parent_dirs"].(bool); ok {
		parentDirs = pd
	}

	workspace := GetSafeWorkspace()
	fullPath := ResolvePath(pathArg, workspace)

	// Security check - ensure path is within workspace
	if !strings.HasPrefix(fullPath, workspace) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	// Check if directory already exists
	if info, err := os.Stat(fullPath); err == nil {
		if info.IsDir() {
			result := map[string]interface{}{
				"success": true,
				"path":    pathArg,
				"message": fmt.Sprintf("Directory '%s' already exists", pathArg),
				"existed": true,
			}
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), nil
		}
		return "", fmt.Errorf("path '%s' exists but is not a directory", pathArg)
	}

	// Create the directory
	var err error
	if parentDirs {
		err = os.MkdirAll(fullPath, 0755)
	} else {
		err = os.Mkdir(fullPath, 0755)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"path":    pathArg,
		"message": fmt.Sprintf("Successfully created directory '%s'", pathArg),
		"existed": false,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func init() {
	registry.Register(&MakeDirectoryTool{})
}
