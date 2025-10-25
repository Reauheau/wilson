package git

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitStatusTool struct{}

func (t *GitStatusTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_status",
		Description:     "Show git status: modified, staged, and untracked files",
		Category:        "git",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Working directory (defaults to current)",
				Example:     "/path/to/repo",
			},
		},
		Examples: []string{
			`{"tool": "git_status", "arguments": {}}`,
			`{"tool": "git_status", "arguments": {"path": "/path/to/repo"}}`,
		},
	}
}

func (t *GitStatusTool) Validate(args map[string]interface{}) error {
	// No required arguments
	return nil
}

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Run git status --porcelain=v1 --branch
	output, err := RunGitCommandInDir(gitRoot, "status", "--porcelain=v1", "--branch")
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	// Parse output into structured format
	result := parseGitStatus(output)
	result["git_root"] = gitRoot

	// Return JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

// parseGitStatus parses git status --porcelain=v1 output
// Format: XY filename
// X = staged status, Y = working tree status
// ## = branch info
func parseGitStatus(output string) map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	modified := []string{}
	staged := []string{}
	untracked := []string{}
	deleted := []string{}
	renamed := []string{}
	branch := "unknown"
	ahead := 0
	behind := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse branch line: ## branch_name [ahead N, behind M]
		if strings.HasPrefix(line, "##") {
			branchInfo := strings.TrimPrefix(line, "## ")
			// Extract branch name (before ...)
			if idx := strings.Index(branchInfo, "..."); idx != -1 {
				branch = branchInfo[:idx]
			} else if idx := strings.Index(branchInfo, " "); idx != -1 {
				branch = branchInfo[:idx]
			} else {
				branch = branchInfo
			}
			// Parse ahead/behind (if present in [ahead N, behind M] format)
			if strings.Contains(branchInfo, "[") {
				bracketStart := strings.Index(branchInfo, "[")
				bracketEnd := strings.Index(branchInfo, "]")
				if bracketEnd > bracketStart {
					bracketContent := branchInfo[bracketStart+1 : bracketEnd]
					// Split by comma to handle "ahead N, behind M"
					parts := strings.Split(bracketContent, ",")
					for _, part := range parts {
						part = strings.TrimSpace(part)
						if strings.HasPrefix(part, "ahead ") {
							fmt.Sscanf(part, "ahead %d", &ahead)
						} else if strings.HasPrefix(part, "behind ") {
							fmt.Sscanf(part, "behind %d", &behind)
						}
					}
				}
			}
			continue
		}

		// Parse file status lines (at least 3 characters: XY<space>filename)
		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		filename := strings.TrimSpace(line[3:])

		// Extract status codes
		stagedStatus := statusCode[0]
		workingStatus := statusCode[1]

		// Untracked files
		if statusCode == "??" {
			untracked = append(untracked, filename)
			continue
		}

		// Staged changes (index modified)
		if stagedStatus != ' ' && stagedStatus != '?' {
			staged = append(staged, filename)

			// Track renames
			if stagedStatus == 'R' {
				renamed = append(renamed, filename)
			}
		}

		// Working tree changes
		if workingStatus == 'M' {
			modified = append(modified, filename)
		} else if workingStatus == 'D' {
			deleted = append(deleted, filename)
		}

		// Deleted from index
		if stagedStatus == 'D' && workingStatus != 'D' {
			deleted = append(deleted, filename)
		}
	}

	// Remove duplicates from deleted (can appear in both staged and working)
	deleted = uniqueStrings(deleted)

	clean := len(modified) == 0 && len(staged) == 0 && len(untracked) == 0 && len(deleted) == 0

	return map[string]interface{}{
		"branch":    branch,
		"ahead":     ahead,
		"behind":    behind,
		"modified":  modified,
		"staged":    staged,
		"untracked": untracked,
		"deleted":   deleted,
		"renamed":   renamed,
		"clean":     clean,
	}
}

// uniqueStrings removes duplicates from a string slice
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, str := range input {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}
	return result
}

func init() {
	registry.Register(&GitStatusTool{})
}
