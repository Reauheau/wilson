package git

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitStashTool struct{}

func (t *GitStashTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_stash",
		Description:     "Stash changes temporarily",
		Category:        "git",
		RiskLevel:       RiskModerate,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "action",
				Type:        "string",
				Required:    false,
				Description: "Action: 'save', 'pop', 'list', 'show' (default 'list')",
				Example:     "save",
			},
			{
				Name:        "message",
				Type:        "string",
				Required:    false,
				Description: "Message for stash (for 'save' action)",
				Example:     "Work in progress",
			},
			{
				Name:        "stash_id",
				Type:        "string",
				Required:    false,
				Description: "Stash ID for 'pop' or 'show' (e.g., 'stash@{0}')",
				Example:     "stash@{0}",
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
			`{"tool": "git_stash", "arguments": {"action": "list"}}`,
			`{"tool": "git_stash", "arguments": {"action": "save", "message": "WIP"}}`,
			`{"tool": "git_stash", "arguments": {"action": "pop"}}`,
		},
	}
}

func (t *GitStashTool) Validate(args map[string]interface{}) error {
	if action, ok := args["action"].(string); ok {
		if action != "save" && action != "pop" && action != "list" && action != "show" {
			return fmt.Errorf("action must be 'save', 'pop', 'list', or 'show'")
		}
	}
	return nil
}

func (t *GitStashTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get action (default to "list")
	action := "list"
	if a, ok := args["action"].(string); ok && a != "" {
		action = a
	}

	switch action {
	case "save":
		message := "WIP"
		if m, ok := args["message"].(string); ok && m != "" {
			message = m
		}
		return t.saveStash(gitRoot, message)
	case "pop":
		stashID := "stash@{0}"
		if s, ok := args["stash_id"].(string); ok && s != "" {
			stashID = s
		}
		return t.popStash(gitRoot, stashID)
	case "list":
		return t.listStash(gitRoot)
	case "show":
		stashID := "stash@{0}"
		if s, ok := args["stash_id"].(string); ok && s != "" {
			stashID = s
		}
		return t.showStash(gitRoot, stashID)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *GitStashTool) saveStash(gitRoot, message string) (string, error) {
	output, err := RunGitCommandInDir(gitRoot, "stash", "save", message)
	if err != nil {
		return "", fmt.Errorf("failed to save stash: %w", err)
	}

	return fmt.Sprintf("Stash saved: %s", strings.TrimSpace(output)), nil
}

func (t *GitStashTool) popStash(gitRoot, stashID string) (string, error) {
	output, err := RunGitCommandInDir(gitRoot, "stash", "pop", stashID)
	if err != nil {
		return "", fmt.Errorf("failed to pop stash: %w", err)
	}

	return fmt.Sprintf("Stash applied: %s", strings.TrimSpace(output)), nil
}

func (t *GitStashTool) listStash(gitRoot string) (string, error) {
	output, err := RunGitCommandInDir(gitRoot, "stash", "list")
	if err != nil {
		return "", fmt.Errorf("failed to list stashes: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return "No stashes found.", nil
	}

	// Parse output into structured format
	stashes := parseGitStashList(output)

	// Return JSON
	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"stashes": stashes,
		"count":   len(stashes),
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (t *GitStashTool) showStash(gitRoot, stashID string) (string, error) {
	output, err := RunGitCommandInDir(gitRoot, "stash", "show", "-p", stashID)
	if err != nil {
		return "", fmt.Errorf("failed to show stash: %w", err)
	}

	return output, nil
}

// parseGitStashList parses git stash list output
// Format: stash@{N}: WIP on branch: hash message
func parseGitStashList(output string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	stashes := []map[string]string{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Split on first colon to get stash ID and description
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		stashID := strings.TrimSpace(parts[0])
		description := strings.TrimSpace(parts[1])

		stash := map[string]string{
			"id":          stashID,
			"description": description,
		}
		stashes = append(stashes, stash)
	}

	return stashes
}

func init() {
	registry.Register(&GitStashTool{})
}
