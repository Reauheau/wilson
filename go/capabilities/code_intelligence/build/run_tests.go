package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"wilson/core/registry"
	. "wilson/core/types"
)

// RunTestsTool runs Go tests and captures results
type RunTestsTool struct{}

func init() {
	registry.Register(&RunTestsTool{})
}

func (t *RunTestsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "run_tests",
		Description:     "Run Go tests and capture results. Returns pass/fail status, failed test details, and coverage information.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Path to package or file to test - relative or absolute (default: current directory)",
				Example:     "./agent or /full/path/to/package",
			},
			{
				Name:        "test_name",
				Type:        "string",
				Required:    false,
				Description: "Run only tests matching this pattern (regex)",
				Example:     "TestCodeAgent",
			},
			{
				Name:        "coverage",
				Type:        "boolean",
				Required:    false,
				Description: "Enable coverage reporting (default: false)",
				Example:     "true",
			},
			{
				Name:        "verbose",
				Type:        "boolean",
				Required:    false,
				Description: "Verbose output (default: false)",
				Example:     "true",
			},
		},
		Examples: []string{
			`{"tool": "run_tests", "arguments": {}}`,
			`{"tool": "run_tests", "arguments": {"path": "agent", "verbose": true}}`,
			`{"tool": "run_tests", "arguments": {"path": ".", "test_name": "TestCodeAgent", "coverage": true}}`,
		},
	}
}

func (t *RunTestsTool) Validate(args map[string]interface{}) error {
	// Path is optional and can be relative or absolute
	// No validation needed - go test handles both
	return nil
}

func (t *RunTestsTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get parameters
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	testName := ""
	if tn, ok := input["test_name"].(string); ok && tn != "" {
		testName = tn
	}

	coverage := false
	if c, ok := input["coverage"].(bool); ok {
		coverage = c
	}

	verbose := false
	if v, ok := input["verbose"].(bool); ok {
		verbose = v
	}

	// Build command
	args := []string{"test"}

	if verbose {
		args = append(args, "-v")
	}

	if coverage {
		args = append(args, "-cover")
	}

	if testName != "" {
		args = append(args, "-run", testName)
	}

	args = append(args, path)

	// BUGFIX: Run go mod tidy before tests to prevent "missing go.sum entry" errors
	// This ensures go.sum is up-to-date with go.mod, especially important when
	// code has been generated/modified by Wilson and may reference new or removed dependencies

	// ✅ FIX: Always convert to absolute path for working directory
	// This ensures go mod tidy runs in the correct directory, even if path is "."
	absPath := path
	if !filepath.IsAbs(path) {
		var err error
		absPath, err = filepath.Abs(path)
		if err != nil {
			fmt.Printf("[run_tests] Warning: Could not resolve absolute path for %s: %v\n", path, err)
			absPath = path // Fallback to original
		}
	}

	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = absPath // ✅ Always set working directory, never leave empty
	tidyOutput, tidyErr := tidyCmd.CombinedOutput()
	if tidyErr != nil {
		// Log but don't fail - the test might still work or reveal the real issue
		fmt.Printf("[run_tests] Warning: go mod tidy failed: %v\nOutput: %s\n", tidyErr, string(tidyOutput))
	}

	// Execute tests
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = absPath // ✅ FIX: Set working directory for test execution too
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	success := err == nil
	outputStr := string(output)

	// Parse test results
	testResults := parseTestResults(outputStr)

	// Build result
	result := map[string]interface{}{
		"success":        success,
		"path":           path,
		"duration_ms":    duration.Milliseconds(),
		"tests_run":      testResults.TotalTests,
		"tests_passed":   testResults.PassedTests,
		"tests_failed":   testResults.FailedTests,
		"tests_skipped":  testResults.SkippedTests,
		"failed_tests":   testResults.Failures,
		"coverage":       testResults.Coverage,
		"output":         outputStr,
		"command":        fmt.Sprintf("go %s", strings.Join(args, " ")),
	}

	if success {
		result["message"] = fmt.Sprintf("✅ All tests passed! (%d tests, %dms)", testResults.TotalTests, duration.Milliseconds())
	} else {
		result["message"] = fmt.Sprintf("❌ %d test(s) failed out of %d", testResults.FailedTests, testResults.TotalTests)
	}

	// Convert to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// TestResults holds parsed test execution results
type TestResults struct {
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	Coverage     string
	Failures     []TestFailure
}

// TestFailure represents a failed test
type TestFailure struct {
	TestName string `json:"test_name"`
	Package  string `json:"package"`
	Output   string `json:"output"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// parseTestResults parses go test output into structured format
func parseTestResults(output string) TestResults {
	results := TestResults{
		Failures: []TestFailure{},
	}

	lines := strings.Split(output, "\n")

	// Regex patterns
	passPattern := regexp.MustCompile(`^ok\s+(\S+)\s+(\d+\.\d+)s`)
	failPattern := regexp.MustCompile(`^FAIL\s+(\S+)\s+(\d+\.\d+)s`)
	testPassPattern := regexp.MustCompile(`^--- PASS: (\S+)`)
	testFailPattern := regexp.MustCompile(`^--- FAIL: (\S+)`)
	testSkipPattern := regexp.MustCompile(`^--- SKIP: (\S+)`)
	coveragePattern := regexp.MustCompile(`coverage:\s+(\d+\.\d+%)\s+of statements`)

	var currentPackage string
	var currentFailure *TestFailure

	for _, line := range lines {
		// Check for package pass/fail
		if match := passPattern.FindStringSubmatch(line); match != nil {
			currentPackage = match[1]
		} else if match := failPattern.FindStringSubmatch(line); match != nil {
			currentPackage = match[1]
		}

		// Check for individual test results
		if match := testPassPattern.FindStringSubmatch(line); match != nil {
			results.TotalTests++
			results.PassedTests++
		} else if match := testFailPattern.FindStringSubmatch(line); match != nil {
			results.TotalTests++
			results.FailedTests++

			// Start capturing failure details
			currentFailure = &TestFailure{
				TestName: match[1],
				Package:  currentPackage,
				Output:   "",
			}
			results.Failures = append(results.Failures, *currentFailure)
		} else if match := testSkipPattern.FindStringSubmatch(line); match != nil {
			results.TotalTests++
			results.SkippedTests++
		}

		// Capture failure output
		if currentFailure != nil && strings.HasPrefix(line, "    ") {
			currentFailure.Output += line + "\n"

			// Try to extract file:line from error messages
			// Example: agent/code_agent_test.go:42: assertion failed
			fileLinePattern := regexp.MustCompile(`(\S+\.go):(\d+):`)
			if match := fileLinePattern.FindStringSubmatch(line); match != nil {
				currentFailure.File = match[1]
				fmt.Sscanf(match[2], "%d", &currentFailure.Line)
			}
		}

		// Check for coverage
		if match := coveragePattern.FindStringSubmatch(line); match != nil {
			results.Coverage = match[1]
		}

		// Reset current failure when we hit a new test
		if strings.HasPrefix(line, "--- ") {
			currentFailure = nil
		}
	}

	return results
}
