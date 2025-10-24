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

// EditLineTool edits specific lines in a file by line number
// This is more robust than modify_file because:
// 1. Uses line numbers from compiler errors
// 2. No exact string matching required
// 3. Can replace multiple lines at once
type EditLineTool struct{}

func init() {
	registry.Register(&EditLineTool{})
}

func (t *EditLineTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "edit_line",
		Description:     "Edit specific lines in a file by line number. Perfect for fixing compilation errors that specify line numbers.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskModerate,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Path to the file to edit",
				Example:     "main_test.go",
			},
			{
				Name:        "line",
				Type:        "number",
				Required:    true,
				Description: "Line number to edit (1-indexed)",
				Example:     "9",
			},
			{
				Name:        "new_content",
				Type:        "string",
				Required:    true,
				Description: "New content for the line (should NOT include line number or leading tabs/spaces - we preserve original indentation)",
				Example:     "t.Errorf(\"Expected 5, got %g\", result)",
			},
			{
				Name:        "verify_old",
				Type:        "string",
				Required:    false,
				Description: "Optional: verify the old content matches before replacing (for safety)",
				Example:     "t.Errorf(\"Expected 5, got %d\", result)",
			},
		},
		Examples: []string{
			`{"tool": "edit_line", "arguments": {"path": "main_test.go", "line": 9, "new_content": "t.Errorf(\"Expected 5, got %g\", result)"}}`,
			`{"tool": "edit_line", "arguments": {"path": "main.go", "line": 15, "new_content": "return x + y", "verify_old": "return a + b"}}`,
		},
	}
}

func (t *EditLineTool) Validate(args map[string]interface{}) error {
	if path, ok := args["path"].(string); !ok || path == "" {
		return fmt.Errorf("path is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line number is required")
	}
	if _, ok := args["new_content"].(string); !ok {
		return fmt.Errorf("new_content is required")
	}
	return nil
}

func (t *EditLineTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get parameters
	path, _ := args["path"].(string)
	lineNumFloat, _ := args["line"].(float64)
	lineNum := int(lineNumFloat)
	newContent, _ := args["new_content"].(string)
	verifyOld, hasVerify := args["verify_old"].(string)

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

	// Split into lines
	lines := strings.Split(string(content), "\n")

	// Validate line number
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("line number %d out of range (file has %d lines)", lineNum, len(lines))
	}

	// Get the line (convert to 0-indexed)
	lineIdx := lineNum - 1
	oldLine := lines[lineIdx]

	// Optional verification
	if hasVerify {
		// Strip whitespace for comparison
		oldTrimmed := strings.TrimSpace(oldLine)
		verifyTrimmed := strings.TrimSpace(verifyOld)
		if oldTrimmed != verifyTrimmed {
			return "", fmt.Errorf("verification failed: line %d contains:\n%s\nbut expected:\n%s", lineNum, oldTrimmed, verifyTrimmed)
		}
	}

	// Preserve indentation from original line
	indentation := ""
	for _, ch := range oldLine {
		if ch == ' ' || ch == '\t' {
			indentation += string(ch)
		} else {
			break
		}
	}

	// Apply indentation to new content (unless it already has indentation)
	finalNewContent := newContent
	if !strings.HasPrefix(newContent, "\t") && !strings.HasPrefix(newContent, " ") {
		finalNewContent = indentation + newContent
	}

	// Replace the line
	lines[lineIdx] = finalNewContent

	// Join lines back
	modifiedContent := strings.Join(lines, "\n")

	// Write back to file
	if err := os.WriteFile(absPath, []byte(modifiedContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("file modified but failed to stat: %w", err)
	}

	result := map[string]interface{}{
		"success":     true,
		"path":        absPath,
		"line":        lineNum,
		"old_content": strings.TrimSpace(oldLine),
		"new_content": strings.TrimSpace(finalNewContent),
		"size":        info.Size(),
		"message":     fmt.Sprintf("Successfully edited line %d in %s", lineNum, path),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}
