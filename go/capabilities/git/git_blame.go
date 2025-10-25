package git

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type GitBlameTool struct{}

func (t *GitBlameTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "git_blame",
		Description:     "Show who last modified each line of a file",
		Category:        "git",
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File to blame",
				Example:     "main.go",
			},
			{
				Name:        "start_line",
				Type:        "number",
				Required:    false,
				Description: "Start line number (defaults to 1)",
				Example:     "10",
			},
			{
				Name:        "end_line",
				Type:        "number",
				Required:    false,
				Description: "End line number (defaults to end of file)",
				Example:     "20",
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
			`{"tool": "git_blame", "arguments": {"file": "main.go"}}`,
			`{"tool": "git_blame", "arguments": {"file": "main.go", "start_line": 10, "end_line": 20}}`,
		},
	}
}

func (t *GitBlameTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"].(string); !ok {
		return fmt.Errorf("file parameter is required")
	}
	return nil
}

func (t *GitBlameTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get file
	file, ok := args["file"].(string)
	if !ok {
		return "", fmt.Errorf("file parameter required")
	}

	// Build git blame command
	blameArgs := []string{"blame", "--porcelain"}

	// Add line range if specified
	if startLine, ok := args["start_line"].(float64); ok {
		endLine := startLine
		if el, ok := args["end_line"].(float64); ok {
			endLine = el
		}
		blameArgs = append(blameArgs, fmt.Sprintf("-L%d,%d", int(startLine), int(endLine)))
	}

	blameArgs = append(blameArgs, file)

	// Run git blame
	output, err := RunGitCommandInDir(gitRoot, blameArgs...)
	if err != nil {
		return "", fmt.Errorf("git blame failed: %w", err)
	}

	// Parse output into structured format
	blameLines := parseGitBlame(output)

	// Return JSON
	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"file":  file,
		"lines": blameLines,
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

// parseGitBlame parses porcelain format output from git blame
func parseGitBlame(output string) []map[string]interface{} {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := []map[string]interface{}{}

	// Regex to parse commit hash line: <hash> <orig_line> <final_line> <num_lines>
	commitLineRegex := regexp.MustCompile(`^([a-f0-9]{40})\s+(\d+)\s+(\d+)(?:\s+(\d+))?`)

	var currentCommit string
	var currentAuthor string
	var currentDate string
	var currentLine int

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Match commit line
		if matches := commitLineRegex.FindStringSubmatch(line); matches != nil {
			currentCommit = matches[1][:8] // Short hash
			currentLine, _ = fmt.Sscanf(matches[3], "%d")

			// Look ahead for author and date
			for j := i + 1; j < len(lines) && j < i+15; j++ {
				if strings.HasPrefix(lines[j], "author ") {
					currentAuthor = strings.TrimPrefix(lines[j], "author ")
				} else if strings.HasPrefix(lines[j], "author-time ") {
					// Could parse timestamp here, but we'll use author-time as-is
					currentDate = strings.TrimPrefix(lines[j], "author-time ")
				} else if strings.HasPrefix(lines[j], "\t") {
					// Found the actual code line
					code := strings.TrimPrefix(lines[j], "\t")
					result = append(result, map[string]interface{}{
						"line":   currentLine,
						"commit": currentCommit,
						"author": currentAuthor,
						"date":   currentDate,
						"code":   code,
					})
					i = j // Skip to this line
					break
				}
			}
		}
	}

	return result
}

func init() {
	registry.Register(&GitBlameTool{})
}
