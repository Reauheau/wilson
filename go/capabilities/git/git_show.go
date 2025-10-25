package git

import (
	"context"
	"fmt"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitShowTool struct{}

func (t *GitShowTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_show",
		Description:     "Show commit details including diff",
		Category:        "git",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "commit",
				Type:        "string",
				Required:    false,
				Description: "Commit hash or reference (defaults to HEAD)",
				Example:     "abc123",
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
			`{"tool": "git_show", "arguments": {}}`,
			`{"tool": "git_show", "arguments": {"commit": "HEAD~1"}}`,
			`{"tool": "git_show", "arguments": {"commit": "abc123"}}`,
		},
	}
}

func (t *GitShowTool) Validate(args map[string]interface{}) error {
	// No required arguments
	return nil
}

func (t *GitShowTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get commit reference (default to HEAD)
	commit := "HEAD"
	if c, ok := args["commit"].(string); ok && c != "" {
		commit = c
	}

	// Run git show
	output, err := RunGitCommandInDir(gitRoot, "show", commit)
	if err != nil {
		return "", fmt.Errorf("git show failed (invalid commit?): %w", err)
	}

	return output, nil
}

func init() {
	registry.Register(&GitShowTool{})
}
