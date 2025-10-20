package unit

import (
	"testing"

	"wilson/tests/framework"

	// Import tools to register them
	_ "wilson/capabilities/code_intelligence/ast"
)

// TestParseFile tests the parse_file tool
func TestParseFile(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a test Go file
	code := `package main

import "fmt"

// User represents a user
type User struct {
	Name string
	Age  int
}

// Greet prints a greeting
func Greet(name string) {
	fmt.Printf("Hello, %s!\n", name)
}

func main() {
	Greet("World")
}
`
	err := runner.WriteFile("test.go", code)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run parse_file tool
	result, err := runner.ExecuteTool("parse_file", map[string]interface{}{
		"path": runner.Context().WorkDir + "/test.go",
	})
	framework.AssertNoError(t, err, "parse_file failed")

	// Should contain AST information
	framework.AssertContains(t, result, "User", "Should find User struct")
	framework.AssertContains(t, result, "Greet", "Should find Greet function")
}

// TestFindSymbol tests the find_symbol tool
func TestFindSymbol(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create test files
	code := `package mypackage

type Calculator struct {
	value int
}

func (c *Calculator) Add(x int) {
	c.value += x
}

func NewCalculator() *Calculator {
	return &Calculator{}
}
`
	err := runner.WriteFile("calculator.go", code)
	framework.AssertNoError(t, err, "Failed to write calculator.go")

	// Run find_symbol for Calculator struct
	result, err := runner.ExecuteTool("find_symbol", map[string]interface{}{
		"path":   runner.Context().WorkDir,
		"symbol": "Calculator",
	})
	framework.AssertNoError(t, err, "find_symbol failed")

	// Should find the Calculator definition
	framework.AssertContains(t, result, "Calculator", "Should find Calculator")
	framework.AssertContains(t, result, "calculator.go", "Should indicate file location")
}

// TestAnalyzeStructure tests the analyze_structure tool
func TestAnalyzeStructure(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a multi-file project structure
	err := runner.WriteFile("go.mod", "module testproject\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Create models file
	models := `package testproject

type User struct {
	ID   int
	Name string
}
`
	err = runner.WriteFile("models.go", models)
	framework.AssertNoError(t, err, "Failed to write models.go")

	// Create service file
	service := `package testproject

type UserService struct {
	users []User
}

func (s *UserService) GetUser(id int) *User {
	for _, u := range s.users {
		if u.ID == id {
			return &u
		}
	}
	return nil
}
`
	err = runner.WriteFile("service.go", service)
	framework.AssertNoError(t, err, "Failed to write service.go")

	// Run analyze_structure
	result, err := runner.ExecuteTool("analyze_structure", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})
	framework.AssertNoError(t, err, "analyze_structure failed")

	// Should analyze project structure
	framework.AssertContains(t, result, "User", "Should find User struct")
	framework.AssertContains(t, result, "UserService", "Should find UserService")
}

// TestAnalyzeImports tests the analyze_imports tool
func TestAnalyzeImports(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create test file with various imports
	code := `package main

import (
	"fmt"
	"os"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("test")
	os.Exit(0)
}
`
	err := runner.WriteFile("test.go", code)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run analyze_imports
	result, err := runner.ExecuteTool("analyze_imports", map[string]interface{}{
		"path": runner.Context().WorkDir + "/test.go",
	})
	framework.AssertNoError(t, err, "analyze_imports failed")

	// Should list imports
	framework.AssertContains(t, result, "fmt", "Should find fmt import")
	framework.AssertContains(t, result, "os", "Should find os import")
	framework.AssertContains(t, result, "database/sql", "Should find database/sql import")
}

// TestFindSymbolOnMockProject tests finding symbols in the mock project
func TestFindSymbolOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Find the user struct
	result, err := runner.ExecuteTool("find_symbol", map[string]interface{}{
		"path":   runner.Context().WorkDir,
		"symbol": "user",
	})
	framework.AssertNoError(t, err, "find_symbol failed")

	// Should find user in models.go
	framework.AssertContains(t, result, "models.go", "Should find user in models.go")
}

// TestAnalyzeStructureOnMockProject tests structure analysis on mock project
func TestAnalyzeStructureOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Analyze the structure
	result, err := runner.ExecuteTool("analyze_structure", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})
	framework.AssertNoError(t, err, "analyze_structure failed")

	// Should find key components
	framework.AssertContains(t, result, "user", "Should find user struct")
	framework.AssertContains(t, result, "UserService", "Should find UserService")

	// Should identify functions
	framework.AssertContains(t, result, "GetUser", "Should find GetUser function")
	framework.AssertContains(t, result, "CreateUser", "Should find CreateUser function")

	t.Logf("Structure analysis: %s", result)
}

// TestParseFileWithSyntaxError tests parse_file with invalid syntax
func TestParseFileWithSyntaxError(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create file with syntax error
	badCode := `package main

func main() {
	fmt.Println("unclosed string
}
`
	err := runner.WriteFile("bad.go", badCode)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Run parse_file - should handle error gracefully
	result, err := runner.ExecuteTool("parse_file", map[string]interface{}{
		"path": runner.Context().WorkDir + "/bad.go",
	})

	// Should either error or return error information
	if err != nil {
		framework.AssertContains(t, err.Error(), "syntax", "Error should mention syntax")
	} else {
		framework.AssertContains(t, result, "error", "Result should indicate error")
	}
}

// TestFindSymbolNotFound tests find_symbol when symbol doesn't exist
func TestFindSymbolNotFound(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create simple file
	code := `package main

func main() {}
`
	err := runner.WriteFile("test.go", code)
	framework.AssertNoError(t, err, "Failed to write test file")

	// Look for non-existent symbol
	result, err := runner.ExecuteTool("find_symbol", map[string]interface{}{
		"path":   runner.Context().WorkDir,
		"symbol": "NonExistentType",
	})

	// Should handle gracefully
	if err == nil {
		// Result should indicate not found
		t.Logf("Find symbol result: %s", result)
	} else {
		framework.AssertContains(t, err.Error(), "not found", "Error should indicate symbol not found")
	}
}
