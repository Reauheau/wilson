package git

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitLogTool struct{}

func (t *GitLogTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_log",
		Description:     "View commit history",
		Category:        "git",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "max_count",
				Type:        "number",
				Required:    false,
				Description: "Maximum number of commits to show (default 10)",
				Example:     "20",
			},
			{
				Name:        "file",
				Type:        "string",
				Required:    false,
				Description: "Show commits for specific file only",
				Example:     "main.go",
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
			`{"tool": "git_log", "arguments": {}}`,
			`{"tool": "git_log", "arguments": {"max_count": 5}}`,
			`{"tool": "git_log", "arguments": {"file": "main.go", "max_count": 20}}`,
		},
	}
}

func (t *GitLogTool) Validate(args map[string]interface{}) error {
	// No required arguments
	return nil
}

func (t *GitLogTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get max count (default 10)
	maxCount := 10
	if count, ok := args["max_count"].(float64); ok {
		maxCount = int(count)
	}

	// Build git log command with custom format
	// Format: HASH|AUTHOR|DATE|MESSAGE
	logArgs := []string{
		"log",
		fmt.Sprintf("-n%d", maxCount),
		"--pretty=format:%H|%an|%ar|%s",
	}

	// Add specific file if provided
	if file, ok := args["file"].(string); ok && file != "" {
		logArgs = append(logArgs, "--", file)
	}

	// Run git log
	output, err := RunGitCommandInDir(gitRoot, logArgs...)
	if err != nil {
		return "", fmt.Errorf("git log failed: %w", err)
	}

	// Parse output into structured format
	commits := parseGitLog(output)

	// Return JSON
	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"commits": commits,
		"count":   len(commits),
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

// parseGitLog parses the custom format output from git log
func parseGitLog(output string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	commits := []map[string]string{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		commit := map[string]string{
			"hash":    parts[0],
			"author":  parts[1],
			"date":    parts[2],
			"message": parts[3],
		}
		commits = append(commits, commit)
	}

	return commits
}

func init() {
	registry.Register(&GitLogTool{})
}
