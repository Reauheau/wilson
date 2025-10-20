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

// ModifyFileTool modifies a file by replacing content
type ModifyFileTool struct{}

func init() {
	registry.Register(&ModifyFileTool{})
}

func (t *ModifyFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "modify_file",
		Description:     "Modifies an existing file by replacing old content with new content.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskModerate, // Modifying files has moderate risk
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Relative path to the file to modify",
				Example:     "src/config.go",
			},
			{
				Name:        "old_content",
				Type:        "string",
				Required:    true,
				Description: "Content to find and replace (must exist in file)",
				Example:     "const Port = 8080",
			},
			{
				Name:        "new_content",
				Type:        "string",
				Required:    true,
				Description: "Content to replace with",
				Example:     "const Port = 3000",
			},
			{
				Name:        "replace_all",
				Type:        "boolean",
				Required:    false,
				Description: "Replace all occurrences (default: false)",
				Example:     "false",
			},
		},
		Examples: []string{
			`{"tool": "modify_file", "arguments": {"path": "config.go", "old_content": "port = 8080", "new_content": "port = 3000"}}`,
		},
	}
}

func (t *ModifyFileTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	oldContent, ok := args["old_content"].(string)
	if !ok || oldContent == "" {
		return fmt.Errorf("old_content parameter is required")
	}
	_, ok = args["new_content"].(string)
	if !ok {
		return fmt.Errorf("new_content parameter is required")
	}
	return nil
}

func (t *ModifyFileTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get path
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Get old_content
	oldContent, ok := input["old_content"].(string)
	if !ok || oldContent == "" {
		return "", fmt.Errorf("old_content is required")
	}

	// Get new_content
	newContent, ok := input["new_content"].(string)
	if !ok {
		return "", fmt.Errorf("new_content is required")
	}

	// Get replace_all flag (default false)
	replaceAll := false
	if ra, ok := input["replace_all"].(bool); ok {
		replaceAll = ra
	}

	// Get workspace and resolve path
	workspace := GetSafeWorkspace()
	absPath := ResolvePath(path, workspace)

	// Security check
	if !strings.HasPrefix(absPath, workspace) {
		return "", fmt.Errorf("access denied: path outside workspace")
	}

	// Check file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	originalContent := string(content)

	// Check if old_content exists
	if !strings.Contains(originalContent, oldContent) {
		return "", fmt.Errorf("old_content not found in file")
	}

	// Replace content
	var modifiedContent string
	var replacements int
	if replaceAll {
		modifiedContent = strings.ReplaceAll(originalContent, oldContent, newContent)
		replacements = strings.Count(originalContent, oldContent)
	} else {
		modifiedContent = strings.Replace(originalContent, oldContent, newContent, 1)
		replacements = 1
	}

	// Write modified content
	if err := os.WriteFile(absPath, []byte(modifiedContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write modified file: %w", err)
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("file modified but failed to stat: %w", err)
	}

	result := map[string]interface{}{
		"success":       true,
		"path":          absPath,
		"replacements":  replacements,
		"size":          info.Size(),
		"original_size": len(originalContent),
		"modified_size": len(modifiedContent),
		"size_change":   len(modifiedContent) - len(originalContent),
		"message":       fmt.Sprintf("Successfully made %d replacement(s) in %s", replacements, path),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}
