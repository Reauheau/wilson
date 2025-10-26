package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"wilson/core/registry"
	. "wilson/core/types"
)

// CompileTool compiles Go code and captures errors
type CompileTool struct{}

func init() {
	registry.Register(&CompileTool{})
}

func (t *CompileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "compile",
		Description:     "Compile Go code and capture compilation errors. Returns success status, errors with file/line/column locations, and compiler output.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Path to package or file to compile (default: current directory)",
				Example:     "agent",
			},
			{
				Name:        "build_tags",
				Type:        "string",
				Required:    false,
				Description: "Build tags to pass to go build (comma-separated)",
				Example:     "integration,test",
			},
		},
		Examples: []string{
			`{"tool": "compile", "arguments": {}}`,
			`{"tool": "compile", "arguments": {"path": "agent"}}`,
			`{"tool": "compile", "arguments": {"path": ".", "build_tags": "integration"}}`,
		},
	}
}

func (t *CompileTool) Validate(args map[string]interface{}) error {
	// Path is optional, defaults to current directory
	// ✅ FIX: Accept both relative AND absolute paths
	// We need absolute paths to compile code in different directories (e.g., ~/wilsontestdir)
	// Validation just checks it's a valid path string
	if path, ok := args["path"].(string); ok && path != "" {
		// Just verify it's not empty - can be relative or absolute
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("path cannot be empty")
		}
	}
	return nil
}

func (t *CompileTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get path (default to current directory)
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	// Get build tags
	buildTags := ""
	if tags, ok := input["build_tags"].(string); ok && tags != "" {
		buildTags = tags
	}

	// Make absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// ✅ ROBUST FIX: Detect if directory contains test files
	// If so, use "go test -c" to compile tests, not just "go build"
	hasTestFiles, err := containsTestFiles(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to check for test files: %w", err)
	}

	// Build the command
	var args []string
	if hasTestFiles {
		// Use "go test -c" to compile tests without running them
		// This catches test file compilation errors
		fmt.Printf("[Compile] Detected test files in %s - using 'go test -c'\n", absPath)
		args = []string{"test", "-c", "-o", "/dev/null"}
		if buildTags != "" {
			args = append(args, "-tags", buildTags)
		}
		// ✅ FIX: Use "." since we'll set working directory to absPath
		args = append(args, ".")
	} else {
		// Regular build for non-test code
		fmt.Printf("[Compile] No test files in %s - using 'go build'\n", absPath)
		args = []string{"build", "-o", "/dev/null"}
		if buildTags != "" {
			args = append(args, "-tags", buildTags)
		}
		// ✅ FIX: Use "." since we'll set working directory to absPath
		args = append(args, ".")
	}

	// Execute compilation
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	// ✅ FIX: Set working directory to the target directory
	// This prevents "directory outside main module" errors
	cmd.Dir = absPath
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	success := err == nil
	outputStr := string(output)

	// Parse errors from output
	var errors []CompileError
	if !success {
		errors = parseCompileErrors(outputStr, absPath)
	}

	// Build result
	result := map[string]interface{}{
		"success":       success,
		"path":          path,
		"duration_ms":   duration.Milliseconds(),
		"error_count":   len(errors),
		"errors":        errors,
		"output":        outputStr,
		"command":       fmt.Sprintf("go %s", strings.Join(args, " ")),
	}

	if success {
		result["message"] = fmt.Sprintf("✅ Compilation successful! (%dms)", duration.Milliseconds())
	} else {
		result["message"] = fmt.Sprintf("❌ Compilation failed with %d error(s)", len(errors))
	}

	// Convert to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// CompileError represents a structured compilation error
type CompileError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	Type    string `json:"type"` // syntax, type, undefined, etc.
}

// parseCompileErrors parses Go compiler error messages into structured format
func parseCompileErrors(output string, basePath string) []CompileError {
	var errors []CompileError

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse error format: file.go:line:column: error message
		// Example: agent/code_agent.go:42:2: undefined: foo
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}

		file := parts[0]
		lineNum := 0
		colNum := 0
		message := parts[3]

		// Parse line number
		fmt.Sscanf(parts[1], "%d", &lineNum)
		// Parse column number
		fmt.Sscanf(parts[2], "%d", &colNum)

		// Make file path relative if possible
		if relPath, err := filepath.Rel(basePath, file); err == nil {
			file = relPath
		}

		// Determine error type from message
		errorType := classifyError(message)

		errors = append(errors, CompileError{
			File:    file,
			Line:    lineNum,
			Column:  colNum,
			Message: strings.TrimSpace(message),
			Type:    errorType,
		})
	}

	return errors
}

// classifyError determines the type of compilation error
// containsTestFiles checks if a directory contains Go test files
func containsTestFiles(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_test.go") {
			return true, nil
		}
	}
	return false, nil
}

func classifyError(message string) string {
	message = strings.ToLower(message)

	if strings.Contains(message, "syntax error") || strings.Contains(message, "expected") {
		return "syntax"
	}
	if strings.Contains(message, "undefined:") || strings.Contains(message, "not defined") {
		return "undefined"
	}
	if strings.Contains(message, "cannot use") || strings.Contains(message, "type mismatch") {
		return "type"
	}
	if strings.Contains(message, "imported and not used") {
		return "unused_import"
	}
	if strings.Contains(message, "declared and not used") {
		return "unused_variable"
	}
	if strings.Contains(message, "missing return") {
		return "missing_return"
	}
	if strings.Contains(message, "too many") || strings.Contains(message, "not enough") {
		return "argument_count"
	}

	return "other"
}
