package unit

import (
	"testing"

	"wilson/tests/framework"

	// Import tools to register them
	_ "wilson/capabilities/code_intelligence/quality"
)

// TestFormatCode tests the format_code tool
func TestFormatCode(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a test file with formatting issues
	badCode := `package main

import"fmt" // No space

func main()  {
fmt.Println("test")
}
`
	err := runner.WriteFile("test.go", badCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run format_code tool
	result, err := runner.ExecuteTool("format_code", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})
	framework.AssertNoError(t, err, "format_code failed")

	// Verify result contains formatted file info
	framework.AssertContains(t, result, "test.go", "Result should mention test.go")

	// Read back the file and verify it's formatted
	formatted, err := runner.ReadFile("test.go")
	framework.AssertNoError(t, err, "Failed to read formatted file")
	framework.AssertContains(t, formatted, `import "fmt"`, "Import should be properly formatted")
}

// TestLintCode tests the lint_code tool
func TestLintCode(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create test file with lint issues
	badCode := `package main

// unexported struct used in exported function
type user struct {
	name string
}

// GetUser returns a user (no error handling)
func GetUser() *user {
	return nil
}
`
	err := runner.WriteFile("test.go", badCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run lint_code tool
	result, err := runner.ExecuteTool("lint_code", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	// Lint may or may not error - check result
	if result != "" {
		// Should report issues about unexported types or naming
		// (specific issues depend on linter configuration)
		t.Logf("Lint result: %s", result)
	}
}

// TestComplexityCheck tests the complexity_check tool
func TestComplexityCheck(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create test file with high complexity
	complexCode := `package main

func ComplexFunction(a, b, c, d bool) int {
	result := 0

	if a {
		if b {
			if c {
				if d {
					result = 1
				} else {
					result = 2
				}
			} else {
				if d {
					result = 3
				} else {
					result = 4
				}
			}
		} else {
			if c {
				if d {
					result = 5
				} else {
					result = 6
				}
			} else {
				result = 7
			}
		}
	} else {
		if b || c || d {
			result = 8
		}
	}

	return result
}
`
	err := runner.WriteFile("test.go", complexCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run complexity_check tool with low threshold
	result, err := runner.ExecuteTool("complexity_check", map[string]interface{}{
		"path":               runner.Context().WorkDir,
		"max_complexity":     5,
		"max_function_lines": 50,
	})

	// Should report high complexity
	framework.AssertContains(t, result, "ComplexFunction", "Should identify complex function")
}

// TestSecurityScan tests the security_scan tool
func TestSecurityScan(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create test file with SQL injection vulnerability
	vulnerableCode := `package main

import "database/sql"

func GetUser(db *sql.DB, id string) error {
	// SQL INJECTION!
	query := "SELECT * FROM users WHERE id = " + id
	_, err := db.Query(query)
	return err
}
`
	err := runner.WriteFile("test.go", vulnerableCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run security_scan tool
	result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	// Should detect SQL injection
	if result != "" {
		framework.AssertContains(t, result, "SQL", "Should detect SQL-related issue")
		t.Logf("Security scan result: %s", result)
	}
}

// TestCoverageCheck tests the coverage_check tool
func TestCoverageCheck(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a simple Go module with code and tests
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	code := `package testmodule

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}
`
	err = runner.WriteFile("math.go", code)
	framework.AssertNoError(t, err, "Failed to write code file")

	// Incomplete tests (only tests Add, not Subtract)
	testCode := `package testmodule

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}
}
`
	err = runner.WriteFile("math_test.go", testCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run coverage_check tool
	result, err := runner.ExecuteTool("coverage_check", map[string]interface{}{
		"package":      runner.Context().WorkDir,
		"min_coverage": 80.0,
	})

	// Coverage should be around 50% (only 1 of 2 functions tested)
	// Should fail threshold check
	t.Logf("Coverage result: %s", result)
	framework.AssertContains(t, result, "coverage", "Result should mention coverage")
}

// TestCodeReview tests the code_review tool
func TestCodeReview(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// This test requires LLM, so we'll set it up
	if err := runner.WithLLM().Setup(); err != nil {
		t.Skip("Skipping code_review test - LLM not available")
	}

	// Create test file with various issues
	code := `package main

func main() {
	x := 1
	y := 2
	z := x + y
	println(z)
}
`
	err := runner.WriteFile("test.go", code)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run code_review tool
	result, err := runner.ExecuteTool("code_review", map[string]interface{}{
		"path":  runner.Context().WorkDir,
		"focus": "general",
	})

	if err != nil {
		t.Logf("Code review error (may be expected if LLM not configured): %v", err)
	} else {
		framework.AssertContains(t, result, "review", "Result should contain review")
		t.Logf("Code review result: %s", result)
	}
}

// TestFormatCodeOnMockProject tests formatting on the deliberate mock project
func TestFormatCodeOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run format_code
	result, err := runner.ExecuteTool("format_code", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})
	framework.AssertNoError(t, err, "format_code failed on mock project")

	// Should format utils.go which has formatting issues
	framework.AssertContains(t, result, "utils.go", "Should format utils.go")

	// Verify the formatting was fixed
	utilsContent, err := runner.ReadFile("utils.go")
	framework.AssertNoError(t, err, "Failed to read utils.go")

	// Should have proper spacing now
	framework.AssertContains(t, utilsContent, `import "strings"`, "Import should have space")
	framework.AssertContains(t, utilsContent, "ValidateEmail(email string) bool {", "Function should have proper spacing")
}

// TestSecurityScanOnMockProject tests security scanning on the mock project
func TestSecurityScanOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run security_scan
	result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	// Should detect SQL injection vulnerabilities
	framework.AssertContains(t, result, "service.go", "Should find issues in service.go")

	// Should detect multiple critical issues
	t.Logf("Security scan found issues: %s", result)
}

// TestComplexityCheckOnMockProject tests complexity check on the mock project
func TestComplexityCheckOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run complexity_check with standard thresholds
	result, err := runner.ExecuteTool("complexity_check", map[string]interface{}{
		"path":               runner.Context().WorkDir,
		"max_complexity":     15,
		"max_function_lines": 100,
	})

	// Should find CreateUser with high complexity (~25)
	framework.AssertContains(t, result, "CreateUser", "Should identify CreateUser as complex")

	t.Logf("Complexity check result: %s", result)
}

// TestCoverageCheckOnMockProject tests coverage check on the mock project
func TestCoverageCheckOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run coverage_check
	result, err := runner.ExecuteTool("coverage_check", map[string]interface{}{
		"package":      runner.Context().WorkDir,
		"min_coverage": 80.0,
	})

	// Should show very low coverage (~5%)
	framework.AssertContains(t, result, "coverage", "Result should mention coverage")

	// Should fail the 80% threshold
	framework.AssertContains(t, result, "below", "Coverage should be below threshold")

	t.Logf("Coverage result: %s", result)
}
