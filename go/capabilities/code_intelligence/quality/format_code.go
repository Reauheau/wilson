package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// FormatCodeTool automatically formats Go code
type FormatCodeTool struct{}

func init() {
	registry.Register(&FormatCodeTool{})
}

func (t *FormatCodeTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "format_code",
		Description:     "Automatically format Go code using gofmt and goimports. Fixes indentation, spacing, and organizes imports.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskModerate,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "File or directory to format",
				Example:     "agent/code_agent.go",
			},
			{
				Name:        "organize_imports",
				Type:        "boolean",
				Required:    false,
				Description: "Use goimports to organize imports (default: true)",
				Example:     "true",
			},
		},
		Examples: []string{
			`{"tool": "format_code", "arguments": {"path": "."}}`,
			`{"tool": "format_code", "arguments": {"path": "agent/code_agent.go"}}`,
			`{"tool": "format_code", "arguments": {"path": ".", "organize_imports": false}}`,
		},
	}
}

func (t *FormatCodeTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	return nil
}

func (t *FormatCodeTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)

	organizeImports := true
	if oi, ok := input["organize_imports"].(bool); ok {
		organizeImports = oi
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	// Format the code
	result := &FormatResult{
		Path:            path,
		OrganizeImports: organizeImports,
		Changes:         []FileChange{},
	}

	if info.IsDir() {
		err = t.formatDirectory(ctx, absPath, organizeImports, result)
	} else {
		err = t.formatFile(ctx, absPath, organizeImports, result)
	}

	if err != nil {
		return "", fmt.Errorf("failed to format code: %w", err)
	}

	// Build summary
	result.FilesFormatted = len(result.Changes)
	result.NoChangesNeeded = result.FilesChecked - result.FilesFormatted

	// Build JSON result
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// FormatResult holds formatting results
type FormatResult struct {
	Path             string       `json:"path"`
	OrganizeImports  bool         `json:"organize_imports"`
	FilesChecked     int          `json:"files_checked"`
	FilesFormatted   int          `json:"files_formatted"`
	NoChangesNeeded  int          `json:"no_changes_needed"`
	Changes          []FileChange `json:"changes"`
	FormatterUsed    string       `json:"formatter_used"`
}

// FileChange describes changes made to a file
type FileChange struct {
	File    string `json:"file"`
	Changes string `json:"changes"`
}

// formatDirectory formats all Go files in a directory
func (t *FormatCodeTool) formatDirectory(ctx context.Context, dirPath string, organizeImports bool, result *FormatResult) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only format .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Format the file
		result.FilesChecked++
		return t.formatFile(ctx, path, organizeImports, result)
	})
}

// formatFile formats a single Go file
func (t *FormatCodeTool) formatFile(ctx context.Context, filePath string, organizeImports bool, result *FormatResult) error {
	// Read original content
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var formattedContent []byte
	var formatter string

	if organizeImports {
		// Try goimports first (formats + organizes imports)
		formattedContent, err = t.runGoimports(ctx, filePath)
		if err != nil {
			// Fall back to gofmt if goimports not available
			formattedContent, err = t.runGofmt(ctx, filePath)
			formatter = "gofmt"
		} else {
			formatter = "goimports"
		}
	} else {
		// Use gofmt only
		formattedContent, err = t.runGofmt(ctx, filePath)
		formatter = "gofmt"
	}

	if err != nil {
		return fmt.Errorf("failed to format file %s: %w", filePath, err)
	}

	// Compare and write if changed
	if string(originalContent) != string(formattedContent) {
		// Write formatted content
		if err := os.WriteFile(filePath, formattedContent, 0644); err != nil {
			return fmt.Errorf("failed to write formatted file: %w", err)
		}

		// Determine what changed
		changes := []string{}
		if formatter == "goimports" {
			changes = append(changes, "formatted code", "organized imports")
		} else {
			changes = append(changes, "formatted code")
		}

		result.Changes = append(result.Changes, FileChange{
			File:    filePath,
			Changes: strings.Join(changes, ", "),
		})
		result.FormatterUsed = formatter
	}

	return nil
}

// runGofmt runs gofmt on a file
func (t *FormatCodeTool) runGofmt(ctx context.Context, filePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gofmt", filePath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gofmt failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return output, nil
}

// runGoimports runs goimports on a file
func (t *FormatCodeTool) runGoimports(ctx context.Context, filePath string) ([]byte, error) {
	// Check if goimports is available
	if _, err := exec.LookPath("goimports"); err != nil {
		return nil, fmt.Errorf("goimports not found")
	}

	cmd := exec.CommandContext(ctx, "goimports", filePath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("goimports failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return output, nil
}
