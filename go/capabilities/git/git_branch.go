package git

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitBranchTool struct{}

func (t *GitBranchTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_branch",
		Description:     "List branches or get current branch",
		Category:        "git",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "action",
				Type:        "string",
				Required:    false,
				Description: "Action: 'list' or 'current' (default 'current')",
				Example:     "list",
			},
			{
				Name:        "remote",
				Type:        "boolean",
				Required:    false,
				Description: "Include remote branches (for 'list' action)",
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
			`{"tool": "git_branch", "arguments": {}}`,
			`{"tool": "git_branch", "arguments": {"action": "list"}}`,
			`{"tool": "git_branch", "arguments": {"action": "list", "remote": true}}`,
		},
	}
}

func (t *GitBranchTool) Validate(args map[string]interface{}) error {
	if action, ok := args["action"].(string); ok {
		if action != "list" && action != "current" {
			return fmt.Errorf("action must be 'list' or 'current'")
		}
	}
	return nil
}

func (t *GitBranchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get action (default to "current")
	action := "current"
	if a, ok := args["action"].(string); ok && a != "" {
		action = a
	}

	switch action {
	case "current":
		return t.getCurrentBranch(gitRoot)
	case "list":
		remote := false
		if r, ok := args["remote"].(bool); ok {
			remote = r
		}
		return t.listBranches(gitRoot, remote)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *GitBranchTool) getCurrentBranch(gitRoot string) (string, error) {
	// Run git branch --show-current
	output, err := RunGitCommandInDir(gitRoot, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(output)
	if branch == "" {
		// Detached HEAD state
		return "HEAD (detached)", nil
	}

	return branch, nil
}

func (t *GitBranchTool) listBranches(gitRoot string, includeRemote bool) (string, error) {
	branchArgs := []string{"branch", "-v"}
	if includeRemote {
		branchArgs = append(branchArgs, "-a")
	}

	// Run git branch
	output, err := RunGitCommandInDir(gitRoot, branchArgs...)
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	// Parse output into structured format
	branches := parseGitBranch(output)

	// Return JSON
	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"branches": branches,
		"count":    len(branches),
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

// parseGitBranch parses git branch -v output
// Format: * branch_name  hash message
//
//	branch_name  hash message
func parseGitBranch(output string) []map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := []map[string]interface{}{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Check if current branch (marked with *)
		isCurrent := strings.HasPrefix(line, "*")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)

		// Split into parts: branch_name hash message
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		branchName := parts[0]
		commitHash := parts[1]
		message := ""
		if len(parts) > 2 {
			message = strings.Join(parts[2:], " ")
		}

		// Determine if remote
		isRemote := strings.HasPrefix(branchName, "remotes/")
		if isRemote {
			branchName = strings.TrimPrefix(branchName, "remotes/")
		}

		branch := map[string]interface{}{
			"name":    branchName,
			"commit":  commitHash,
			"message": message,
			"current": isCurrent,
			"remote":  isRemote,
		}
		branches = append(branches, branch)
	}

	return branches
}

func init() {
	registry.Register(&GitBranchTool{})
}
