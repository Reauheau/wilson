package verifiers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	framework "wilson/tests/framework"
)

// CodeVerifier defines verification criteria for code
type CodeVerifier struct {
	// Compilation requirements
	MustCompile      bool
	AllowCompileWarnings bool

	// Test requirements
	TestsMustPass    bool
	TestsMustExist   bool
	MinCoverage      float64  // 0-100

	// Quality requirements
	MaxSecurityIssues int
	MaxComplexity     int
	MaxFunctionLength int
	MustBeFormatted   bool

	// Custom checks
	CustomChecks []func(t *testing.T, runner *framework.TestRunner) error
}

// DefaultCodeVerifier returns a verifier with sensible defaults
func DefaultCodeVerifier() *CodeVerifier {
	return &CodeVerifier{
		MustCompile:       true,
		TestsMustPass:     true,
		MaxSecurityIssues: 0,
		MaxComplexity:     15,
		MaxFunctionLength: 100,
		MustBeFormatted:   true,
		CustomChecks:      []func(*testing.T, *framework.TestRunner) error{},
	}
}

// Verify runs all verification checks
func (v *CodeVerifier) Verify(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	// Run checks in order of importance
	if v.MustCompile {
		if err := v.verifyCompiles(t, runner); err != nil {
			return fmt.Errorf("compilation check failed: %w", err)
		}
	}

	if v.TestsMustPass || v.TestsMustExist {
		if err := v.verifyTests(t, runner); err != nil {
			return fmt.Errorf("test check failed: %w", err)
		}
	}

	if v.MaxSecurityIssues >= 0 {
		if err := v.verifySecur(t, runner); err != nil {
			return fmt.Errorf("security check failed: %w", err)
		}
	}

	if v.MaxComplexity > 0 {
		if err := v.verifyComplexity(t, runner); err != nil {
			return fmt.Errorf("complexity check failed: %w", err)
		}
	}

	if v.MustBeFormatted {
		if err := v.verifyFormatted(t, runner); err != nil {
			return fmt.Errorf("format check failed: %w", err)
		}
	}

	// Run custom checks
	for i, check := range v.CustomChecks {
		if err := check(t, runner); err != nil {
			return fmt.Errorf("custom check %d failed: %w", i+1, err)
		}
	}

	return nil
}

// verifyCompiles checks if code compiles
func (v *CodeVerifier) verifyCompiles(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	result, err := runner.ExecuteTool("compile", map[string]interface{}{
		"path": ".",
	})

	if err != nil {
		return fmt.Errorf("compile tool failed: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return fmt.Errorf("failed to parse compile result: %w", err)
	}

	success, _ := data["success"].(bool)
	if !success {
		errorCount, _ := data["error_count"].(float64)
		return fmt.Errorf("compilation failed with %d errors", int(errorCount))
	}

	return nil
}

// verifyTests checks if tests exist and pass
func (v *CodeVerifier) verifyTests(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	result, err := runner.ExecuteTool("run_tests", map[string]interface{}{
		"package": "./...",
	})

	if err != nil {
		// Tests might not exist yet
		if v.TestsMustExist {
			return fmt.Errorf("tests do not exist or cannot run: %w", err)
		}
		return nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return fmt.Errorf("failed to parse test result: %w", err)
	}

	if v.TestsMustPass {
		allPassed, _ := data["all_passed"].(bool)
		if !allPassed {
			failedCount, _ := data["failed_count"].(float64)
			return fmt.Errorf("%d tests failed", int(failedCount))
		}
	}

	// Check coverage if specified
	if v.MinCoverage > 0 {
		coverageResult, err := runner.ExecuteTool("coverage_check", map[string]interface{}{
			"package":      "./...",
			"min_coverage": v.MinCoverage,
		})

		if err != nil {
			return fmt.Errorf("coverage check failed: %w", err)
		}

		var covData map[string]interface{}
		if err := json.Unmarshal([]byte(coverageResult), &covData); err != nil {
			return fmt.Errorf("failed to parse coverage result: %w", err)
		}

		passedThreshold, _ := covData["passed_threshold"].(bool)
		if !passedThreshold {
			totalCoverage, _ := covData["total_coverage"].(string)
			return fmt.Errorf("coverage %s%% below minimum %v%%", totalCoverage, v.MinCoverage)
		}
	}

	return nil
}

// verifySecur checks for security issues
func (v *CodeVerifier) verifySecurity(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
		"path": ".",
	})

	if err != nil {
		return fmt.Errorf("security scan failed: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return fmt.Errorf("failed to parse security result: %w", err)
	}

	vulnerabilitiesFound, _ := data["vulnerabilities_found"].(float64)
	if int(vulnerabilitiesFound) > v.MaxSecurityIssues {
		return fmt.Errorf("found %d security issues (max: %d)",
			int(vulnerabilitiesFound), v.MaxSecurityIssues)
	}

	return nil
}

// verifyComplexity checks code complexity
func (v *CodeVerifier) verifyComplexity(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	result, err := runner.ExecuteTool("complexity_check", map[string]interface{}{
		"path":                ".",
		"max_complexity":      v.MaxComplexity,
		"max_function_lines":  v.MaxFunctionLength,
	})

	if err != nil {
		return fmt.Errorf("complexity check failed: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return fmt.Errorf("failed to parse complexity result: %w", err)
	}

	passed, _ := data["passed"].(bool)
	if !passed {
		complexFunctions, _ := data["complex_functions"].(float64)
		return fmt.Errorf("%d functions exceed complexity thresholds", int(complexFunctions))
	}

	return nil
}

// verifyFormatted checks if code is formatted
func (v *CodeVerifier) verifyFormatted(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	result, err := runner.ExecuteTool("format_code", map[string]interface{}{
		"path": ".",
	})

	if err != nil {
		return fmt.Errorf("format check failed: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return fmt.Errorf("failed to parse format result: %w", err)
	}

	filesFormatted, _ := data["files_formatted"].(float64)
	if filesFormatted > 0 {
		return fmt.Errorf("%d files needed formatting (code was not properly formatted)",
			int(filesFormatted))
	}

	return nil
}

// Helper functions for custom checks

// VerifyErrorReturn creates a check for error return
func VerifyErrorReturn(funcName string) func(*testing.T, *framework.TestRunner) error {
	return func(t *testing.T, runner *framework.TestRunner) error {
		t.Helper()

		// Find the file containing the function (simplified - assumes service.go)
		_ = runner.RunInWorkDir(func() error {
			// Would use find_symbol tool here in real implementation
			return nil
		})

		// For now, just check service.go
		filepath := runner.Context().WorkDir + "/service.go"
		framework.AssertFunctionReturnsError(t, filepath, funcName)
		return nil
	}
}

// VerifyAllErrorsChecked creates a check for error handling
func VerifyAllErrorsChecked(filepath string) func(*testing.T, *framework.TestRunner) error {
	return func(t *testing.T, runner *framework.TestRunner) error {
		t.Helper()
		fullPath := runner.Context().WorkDir + "/" + filepath
		framework.AssertNoUncheckedErrors(t, fullPath)
		return nil
	}
}

// VerifyCompileSucceeds runs compile and asserts success
func VerifyCompileSucceeds(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	// Run in work directory
	return runner.RunInWorkDir(func() error {
		cmd := exec.Command("go", "build", "./...")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("compile failed: %s\n%s", err, string(output))
		}
		return nil
	})
}

// VerifyNoSQLInjection checks for SQL injection vulnerabilities
func VerifyNoSQLInjection(t *testing.T, runner *framework.TestRunner) error {
	t.Helper()

	// Run security scan
	result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
		"path": ".",
	})

	if err != nil {
		return err
	}

	// Check for SQL injection issues
	if strings.Contains(result, "SQL injection") || strings.Contains(result, "G201") {
		return fmt.Errorf("SQL injection vulnerability found")
	}

	return nil
}

// VerifyUsesParameterizedQuery checks if code uses parameterized queries
func VerifyUsesParameterizedQuery(funcName string) func(*testing.T, *framework.TestRunner) error {
	return func(t *testing.T, runner *framework.TestRunner) error {
		t.Helper()

		// Read the file (simplified - assumes service.go)
		content, err := runner.ReadFile("service.go")
		if err != nil {
			return err
		}

		// Look for parameterized query pattern
		if !strings.Contains(content, "?") && !strings.Contains(content, "$1") {
			return fmt.Errorf("function %s does not use parameterized queries", funcName)
		}

		// Should not have string concatenation in SQL
		if strings.Contains(content, `"SELECT`) && strings.Contains(content, ` + `) {
			return fmt.Errorf("function %s uses string concatenation in SQL query", funcName)
		}

		return nil
	}
}

// Fix typo in verifySecur method name
func (v *CodeVerifier) verifySecur(t *testing.T, runner *framework.TestRunner) error {
	return v.verifySecurity(t, runner)
}
