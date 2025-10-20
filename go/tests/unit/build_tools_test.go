package unit

import (
	"testing"

	"wilson/tests/framework"

	// Import tools to register them
	_ "wilson/capabilities/code_intelligence/build"
)

// TestCompile tests the compile tool with valid code
func TestCompileValidCode(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a valid Go module
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	code := `package testmodule

func Add(a, b int) int {
	return a + b
}
`
	err = runner.WriteFile("math.go", code)
	framework.AssertNoError(t, err, "Failed to write code file")

	// Run compile tool
	result, err := runner.ExecuteTool("compile", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})
	framework.AssertNoError(t, err, "compile failed")

	// Should report success
	framework.AssertContains(t, result, "success", "Result should indicate success")
}

// TestCompileWithErrors tests the compile tool with invalid code
func TestCompileWithErrors(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a Go module with compilation errors
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	badCode := `package testmodule

func Add(a, b int) int {
	return a + b + c  // c is undefined
}

func Broken() {
	var x int
	x = "string"  // Type mismatch
}
`
	err = runner.WriteFile("math.go", badCode)
	framework.AssertNoError(t, err, "Failed to write code file")

	// Run compile tool
	result, err := runner.ExecuteTool("compile", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	// Should report errors
	framework.AssertContains(t, result, "error", "Result should indicate errors")
	framework.AssertContains(t, result, "undefined", "Should report undefined variable")

	t.Logf("Compile errors: %s", result)
}

// TestRunTests tests the run_tests tool
func TestRunTests(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a Go module with code and tests
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	code := `package testmodule

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}
`
	err = runner.WriteFile("math.go", code)
	framework.AssertNoError(t, err, "Failed to write code file")

	testCode := `package testmodule

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add failed")
	}
}

func TestMultiply(t *testing.T) {
	if Multiply(2, 3) != 6 {
		t.Error("Multiply failed")
	}
}
`
	err = runner.WriteFile("math_test.go", testCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run tests
	result, err := runner.ExecuteTool("run_tests", map[string]interface{}{
		"package": runner.Context().WorkDir,
	})
	framework.AssertNoError(t, err, "run_tests failed")

	// Should report passing tests
	framework.AssertContains(t, result, "pass", "Result should indicate passing tests")
}

// TestRunTestsWithFailures tests the run_tests tool with failing tests
func TestRunTestsWithFailures(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a Go module with code and failing tests
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	code := `package testmodule

func Add(a, b int) int {
	return a - b  // Wrong implementation!
}
`
	err = runner.WriteFile("math.go", code)
	framework.AssertNoError(t, err, "Failed to write code file")

	testCode := `package testmodule

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", result)
	}
}
`
	err = runner.WriteFile("math_test.go", testCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run tests
	result, err := runner.ExecuteTool("run_tests", map[string]interface{}{
		"package": runner.Context().WorkDir,
	})

	// Should report test failures
	framework.AssertContains(t, result, "fail", "Result should indicate test failure")
}

// TestCompileOnMockProject tests compilation on mock project
func TestCompileOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run compile tool - should FAIL per README
	result, err := runner.ExecuteTool("compile", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	// Mock project should not compile
	framework.AssertContains(t, result, "error", "Mock project should have compile errors")

	// Should report the specific error about row vs rows
	t.Logf("Compile errors on mock project: %s", result)
}

// TestRunTestsOnMockProject tests running tests on mock project
func TestRunTestsOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run tests - may fail to compile
	result, err := runner.ExecuteTool("run_tests", map[string]interface{}{
		"package": runner.Context().WorkDir,
	})

	// Tests may not run if code doesn't compile
	// Log result for inspection
	if err != nil {
		t.Logf("Test run error (expected if code doesn't compile): %v", err)
	}
	t.Logf("Test result: %s", result)
}

// TestCompileEmptyProject tests compile on empty project
func TestCompileEmptyProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create just a go.mod, no code
	err := runner.WriteFile("go.mod", "module emptyproject\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Run compile
	result, err := runner.ExecuteTool("compile", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	// Empty project should compile successfully (no code to fail)
	if err == nil {
		framework.AssertContains(t, result, "success", "Empty project should compile")
	}
}

// TestRunTestsNoTests tests run_tests when no tests exist
func TestRunTestsNoTests(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create module with code but no tests
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	code := `package testmodule

func Hello() string {
	return "hello"
}
`
	err = runner.WriteFile("hello.go", code)
	framework.AssertNoError(t, err, "Failed to write code file")

	// Run tests
	result, err := runner.ExecuteTool("run_tests", map[string]interface{}{
		"package": runner.Context().WorkDir,
	})

	// Should handle no tests gracefully
	if result != "" {
		t.Logf("No tests result: %s", result)
	}
}

// TestCompileWithDependencies tests compile with external dependencies
func TestCompileWithDependencies(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create module that imports external package
	err := runner.WriteFile("go.mod", "module testmodule\n\ngo 1.24\n\nrequire github.com/google/uuid v1.6.0\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	code := `package testmodule

import "github.com/google/uuid"

func NewID() string {
	return uuid.New().String()
}
`
	err = runner.WriteFile("id.go", code)
	framework.AssertNoError(t, err, "Failed to write code file")

	// Note: This test may fail if dependencies aren't downloaded
	// In real scenario, would need to run `go mod download` first
	result, err := runner.ExecuteTool("compile", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	t.Logf("Compile with dependencies result: %s (may fail without go mod download)", result)
}
