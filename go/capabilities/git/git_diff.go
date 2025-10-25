package git

import (
	"context"
	"fmt"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitDiffTool struct{}

func (t *GitDiffTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_diff",
		Description:     "Show changes in files (unstaged or staged)",
		Category:        "git",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    false,
				Description: "Specific file to diff (shows all if not specified)",
				Example:     "go/agent/code_agent.go",
			},
			{
				Name:        "staged",
				Type:        "boolean",
				Required:    false,
				Description: "Show staged changes instead of unstaged (default false)",
				Example:     "true",
			},
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Working directory (defaults to current)",
				Example:     "/path/to/repo",
			},
		},
		Examples: []string{
			`{"tool": "git_diff", "arguments": {}}`,
			`{"tool": "git_diff", "arguments": {"file": "main.go"}}`,
			`{"tool": "git_diff", "arguments": {"staged": true}}`,
		},
	}
}

func (t *GitDiffTool) Validate(args map[string]interface{}) error {
	// No required arguments
	return nil
}

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get working directory
	workDir := "."
	if path, ok := args["path"].(string); ok && path != "" {
		workDir = path
	}

	// Check if we're in a git repo
	gitRoot, err := FindGitRoot(workDir)
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	// Build git diff command
	diffArgs := []string{"diff"}

	// Check if showing staged changes
	if staged, ok := args["staged"].(bool); ok && staged {
		diffArgs = append(diffArgs, "--cached")
	}

	// Add specific file if provided
	if file, ok := args["file"].(string); ok && file != "" {
		diffArgs = append(diffArgs, "--", file)
	}

	// Run git diff
	output, err := RunGitCommandInDir(gitRoot, diffArgs...)
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	// If no changes, return helpful message
	if output == "" {
		if staged, ok := args["staged"].(bool); ok && staged {
			return "No staged changes to show.", nil
		}
		return "No unstaged changes to show.", nil
	}

	return output, nil
}

func init() {
	registry.Register(&GitDiffTool{})
}
