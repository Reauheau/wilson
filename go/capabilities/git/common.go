package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// FindGitRoot finds the git repository root starting from the given path
// Returns the absolute path to the git root, or an error if not in a git repo
func FindGitRoot(startPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or any parent): %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RunGitCommand executes a git command with the given arguments
// Returns the combined stdout/stderr output
func RunGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git command failed: %w", err)
	}
	return string(output), nil
}

// RunGitCommandInDir executes a git command in a specific directory
func RunGitCommandInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git command failed: %w", err)
	}
	return string(output), nil
}

// IsGitRepo checks if the given path is inside a git repository
func IsGitRepo(path string) bool {
	_, err := FindGitRoot(path)
	return err == nil
}
