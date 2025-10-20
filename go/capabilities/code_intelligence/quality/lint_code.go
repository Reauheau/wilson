package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// LintCodeTool runs linters to check code quality
type LintCodeTool struct{}

func init() {
	registry.Register(&LintCodeTool{})
}

func (t *LintCodeTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "lint_code",
		Description:     "Run Go linters (go vet, staticcheck if available) to check code quality, style, and best practices.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Package path to lint (default: current directory)",
				Example:     "./agent",
			},
			{
				Name:        "linters",
				Type:        "string",
				Required:    false,
				Description: "Comma-separated list of linters: vet,staticcheck (default: vet)",
				Example:     "vet,staticcheck",
			},
		},
		Examples: []string{
			`{"tool": "lint_code", "arguments": {}}`,
			`{"tool": "lint_code", "arguments": {"path": "./agent"}}`,
			`{"tool": "lint_code", "arguments": {"path": ".", "linters": "vet,staticcheck"}}`,
		},
	}
}

func (t *LintCodeTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *LintCodeTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	linters := "vet"
	if l, ok := input["linters"].(string); ok && l != "" {
		linters = l
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	// Run linters
	result := &LintResult{
		Path:    path,
		Linters: strings.Split(linters, ","),
		Issues:  []LintIssue{},
	}

	for _, linter := range result.Linters {
		linter = strings.TrimSpace(linter)
		switch linter {
		case "vet":
			if err := t.runGoVet(ctx, absPath, result); err != nil {
				return "", err
			}
		case "staticcheck":
			if err := t.runStaticcheck(ctx, absPath, result); err != nil {
				// Don't fail if staticcheck not available
				if !strings.Contains(err.Error(), "not found") {
					return "", err
				}
			}
		}
	}

	// Summarize
	result.TotalIssues = len(result.Issues)
	result.Passed = result.TotalIssues == 0

	// Group by severity
	for _, issue := range result.Issues {
		switch issue.Severity {
		case "error":
			result.Errors++
		case "warning":
			result.Warnings++
		default:
			result.Info++
		}
	}

	// Build JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// LintResult holds linting results
type LintResult struct {
	Path        string      `json:"path"`
	Linters     []string    `json:"linters"`
	TotalIssues int         `json:"total_issues"`
	Errors      int         `json:"errors"`
	Warnings    int         `json:"warnings"`
	Info        int         `json:"info"`
	Issues      []LintIssue `json:"issues"`
	Passed      bool        `json:"passed"`
}

// LintIssue represents a single linting issue
type LintIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Linter   string `json:"linter"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// runGoVet runs go vet
func (t *LintCodeTool) runGoVet(ctx context.Context, path string, result *LintResult) error {
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()

	// go vet returns non-zero exit code if issues found
	if err != nil && len(output) == 0 {
		return fmt.Errorf("go vet failed: %w", err)
	}

	// Parse output
	lines := strings.Split(string(output), "\n")
	// Pattern: ./path/file.go:line:column: message
	re := regexp.MustCompile(`^(\.\/)?([^:]+):(\d+):(\d+):\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if matches != nil && len(matches) >= 6 {
			var lineNum, colNum int
			fmt.Sscanf(matches[3], "%d", &lineNum)
			fmt.Sscanf(matches[4], "%d", &colNum)

			result.Issues = append(result.Issues, LintIssue{
				File:     matches[2],
				Line:     lineNum,
				Column:   colNum,
				Linter:   "vet",
				Message:  matches[5],
				Severity: "warning",
			})
		}
	}

	return nil
}

// runStaticcheck runs staticcheck if available
func (t *LintCodeTool) runStaticcheck(ctx context.Context, path string, result *LintResult) error {
	// Check if staticcheck is available
	if _, err := exec.LookPath("staticcheck"); err != nil {
		return fmt.Errorf("staticcheck not found")
	}

	cmd := exec.CommandContext(ctx, "staticcheck", "./...")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()

	// staticcheck returns non-zero exit code if issues found
	if err != nil && len(output) == 0 {
		return fmt.Errorf("staticcheck failed: %w", err)
	}

	// Parse output
	lines := strings.Split(string(output), "\n")
	// Pattern: path/file.go:line:column: message (SA1000)
	re := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if matches != nil && len(matches) >= 5 {
			var lineNum, colNum int
			fmt.Sscanf(matches[2], "%d", &lineNum)
			fmt.Sscanf(matches[3], "%d", &colNum)

			result.Issues = append(result.Issues, LintIssue{
				File:     matches[1],
				Line:     lineNum,
				Column:   colNum,
				Linter:   "staticcheck",
				Message:  matches[4],
				Severity: "warning",
			})
		}
	}

	return nil
}
