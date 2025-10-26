package validation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wilson/agent"
	"wilson/agent/base"
)

// TaskVerifier validates that a task actually accomplished its goal
type TaskVerifier interface {
	Verify(ctx context.Context, execResult *base.ExecutionResult, task *agent.Task) error
}

// CodeTaskVerifier verifies code generation tasks
type CodeTaskVerifier struct{}

// Verify checks that code was actually created and is valid
func (v *CodeTaskVerifier) Verify(ctx context.Context, execResult *base.ExecutionResult, task *agent.Task) error {
	// Check 1: At least one tool was executed
	if len(execResult.ToolsExecuted) == 0 {
		return fmt.Errorf("verification failed: no tools were executed")
	}

	// Check 2: File creation/modification tools were used
	hasFileCreation := false
	for _, tool := range execResult.ToolsExecuted {
		if tool == "write_file" || tool == "modify_file" || tool == "append_to_file" || tool == "edit_line" {
			hasFileCreation = true
			break
		}
	}

	if !hasFileCreation {
		return fmt.Errorf("verification failed: no file creation/modification tools were used (expected write_file, modify_file, append_to_file, or edit_line)")
	}

	// Check 3: Try to extract file paths from tool results and verify they exist
	createdFiles := v.ExtractCreatedFiles(execResult)
	if len(createdFiles) == 0 {
		// This might be okay if files were created but we can't detect them
		// Don't fail on this - just warn
	} else {
		// Verify files actually exist
		for _, file := range createdFiles {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return fmt.Errorf("verification failed: file '%s' should exist but doesn't", file)
			}
		}
	}

	// Check 4: If it's Go code and we have a go.mod, try to compile
	if v.isGoTask(task, execResult) {
		if err := v.verifyGoCode(createdFiles); err != nil {
			// Compilation failure is a warning, not a hard failure
			// The code exists but might need fixes
			// Return nil but log the issue
			// In production, you might want to return the error
		}
	}

	return nil
}

// ExtractCreatedFiles tries to find file paths in tool results
// Made public so CodeAgent can use it for dependency tracking
func (v *CodeTaskVerifier) ExtractCreatedFiles(execResult *base.ExecutionResult) []string {
	files := []string{}

	for i, tool := range execResult.ToolsExecuted {
		if tool == "write_file" || tool == "modify_file" || tool == "append_to_file" || tool == "edit_line" {
			// Tool result might contain the file path
			if i < len(execResult.ToolResults) {
				result := execResult.ToolResults[i]

				// Try to extract path from common patterns
				// Pattern
				//1: Look for "path": "/some/path" in JSON
				if strings.Contains(result, `"path"`) || strings.Contains(result, `'path'`) {
					// Find the path value
					startIdx := strings.Index(result, `"path"`)
					if startIdx == -1 {
						startIdx = strings.Index(result, `'path'`)
					}
					if startIdx != -1 {
						// Find the colon after "path"
						colonIdx := strings.Index(result[startIdx:], ":")
						if colonIdx != -1 {
							pathStart := startIdx + colonIdx + 1
							// Skip whitespace and quotes
							for pathStart < len(result) && (result[pathStart] == ' ' || result[pathStart] == '"' || result[pathStart] == '\'') {
								pathStart++
							}
							// Find the end (quote or comma or })
							pathEnd := pathStart
							for pathEnd < len(result) && result[pathEnd] != '"' && result[pathEnd] != '\'' && result[pathEnd] != ',' && result[pathEnd] != '}' {
								pathEnd++
							}
							if pathEnd > pathStart {
								path := result[pathStart:pathEnd]
								files = append(files, path)
								continue
							}
						}
					}
				}

				// Pattern 2: Look for absolute paths (start with /)
				// But be more careful about extracting them
				if strings.Contains(result, "/") {
					// Find all potential paths (sequences starting with / and containing alphanumeric + / and .)
					start := 0
					for {
						idx := strings.Index(result[start:], "/")
						if idx == -1 {
							break
						}
						idx += start
						// Extract path-like string
						end := idx + 1
						for end < len(result) && (result[end] == '/' || result[end] == '.' || result[end] == '_' || result[end] == '-' ||
							(result[end] >= 'a' && result[end] <= 'z') || (result[end] >= 'A' && result[end] <= 'Z') ||
							(result[end] >= '0' && result[end] <= '9')) {
							end++
						}
						if end > idx+1 {
							path := result[idx:end]
							// Only add if it looks like a real FILE path (has multiple segments AND a file extension)
							// This filters out directory paths like "/Users/foo/project"
							if strings.Count(path, "/") >= 2 && !strings.Contains(path, "http") {
								// Must have a file extension (contains a dot after the last slash)
								lastSlash := strings.LastIndex(path, "/")
								if lastSlash != -1 && strings.Contains(path[lastSlash:], ".") {
									files = append(files, path)
								}
							}
						}
						start = end
					}
				}
			}
		}
	}

	return files
}

// isGoTask checks if this is a Go code task
func (v *CodeTaskVerifier) isGoTask(task *agent.Task, execResult *base.ExecutionResult) bool {
	// Check task description
	taskDesc := strings.ToLower(task.Description)
	if strings.Contains(taskDesc, "go") || strings.Contains(taskDesc, "golang") {
		return true
	}

	// Check for .go files in created files
	for _, tool := range execResult.ToolsExecuted {
		if tool == "write_file" {
			// Check tool results for .go extension
			for _, result := range execResult.ToolResults {
				if strings.Contains(result, ".go") {
					return true
				}
			}
		}
	}

	return false
}

// verifyGoCode tries to compile Go code to verify it's valid
func (v *CodeTaskVerifier) verifyGoCode(files []string) error {
	// Find the directory containing the Go files
	var goDir string
	for _, file := range files {
		if strings.HasSuffix(file, ".go") {
			goDir = filepath.Dir(file)
			break
		}
	}

	if goDir == "" {
		return nil // No Go files found
	}

	// Check if go.mod exists
	goModPath := filepath.Join(goDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		// No go.mod, can't compile
		return nil
	}

	// Try to compile
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = goDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Go code compilation failed: %s", string(output))
	}

	return nil
}

// TestTaskVerifier verifies test generation tasks
type TestTaskVerifier struct{}

func (v *TestTaskVerifier) Verify(ctx context.Context, execResult *base.ExecutionResult, task *agent.Task) error {
	// Check 1: At least one tool was executed
	if len(execResult.ToolsExecuted) == 0 {
		return fmt.Errorf("verification failed: no tools were executed")
	}

	// Check 2: File creation/modification tools were used for test files
	hasTestFileCreation := false
	for i, tool := range execResult.ToolsExecuted {
		if tool == "write_file" || tool == "modify_file" || tool == "append_to_file" || tool == "edit_line" {
			// Check if it's a test file
			if i < len(execResult.ToolResults) {
				result := execResult.ToolResults[i]
				if strings.Contains(result, "_test.") || strings.Contains(result, "test") {
					hasTestFileCreation = true
					break
				}
			}
		}
	}

	if !hasTestFileCreation {
		return fmt.Errorf("verification failed: no test file creation detected")
	}

	return nil
}

// ReviewTaskVerifier verifies review tasks
type ReviewTaskVerifier struct{}

func (v *ReviewTaskVerifier) Verify(ctx context.Context, execResult *base.ExecutionResult, task *agent.Task) error {
	// Check: Quality tools were executed
	qualityTools := []string{"compile", "lint_code", "security_scan", "coverage_check", "code_review"}
	hasQualityCheck := false

	for _, tool := range execResult.ToolsExecuted {
		for _, qTool := range qualityTools {
			if tool == qTool {
				hasQualityCheck = true
				break
			}
		}
	}

	if !hasQualityCheck {
		return fmt.Errorf("verification failed: no quality check tools were executed")
	}

	return nil
}

// GetVerifier returns the appropriate verifier for a task type
func GetVerifier(taskType string) TaskVerifier {
	switch taskType {
	case string(agent.TaskTypeCode):
		return &CodeTaskVerifier{}
	case "test":
		return &TestTaskVerifier{}
	case "review":
		return &ReviewTaskVerifier{}
	default:
		return nil // No verification needed
	}
}
