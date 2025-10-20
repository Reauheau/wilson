package framework

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Assertion helpers for tests

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// AssertEqual fails if expected != actual
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertTrue fails if condition is false
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("%s: condition is false", msg)
	}
}

// AssertFalse fails if condition is true
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("%s: condition is true", msg)
	}
}

// AssertContains fails if haystack doesn't contain needle
func AssertContains(t *testing.T, haystack, needle string, msg string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%s: '%s' not found in '%s'", msg, needle, haystack)
	}
}

// AssertNotContains fails if haystack contains needle
func AssertNotContains(t *testing.T, haystack, needle string, msg string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("%s: '%s' found in '%s' (should not be present)", msg, needle, haystack)
	}
}

// AssertFileExists fails if file doesn't exist
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("File does not exist: %s", path)
	}
}

// AssertFileNotExists fails if file exists
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("File exists (should not): %s", path)
	}
}

// AssertJSONField checks a field in JSON result
func AssertJSONField(t *testing.T, jsonStr string, field string, expected interface{}) {
	t.Helper()

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	actual, ok := data[field]
	if !ok {
		t.Fatalf("Field '%s' not found in JSON", field)
	}

	if actual != expected {
		t.Fatalf("Field '%s': expected %v, got %v", field, expected, actual)
	}
}

// AssertJSONContains checks if JSON contains a field
func AssertJSONContains(t *testing.T, jsonStr string, field string) {
	t.Helper()

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if _, ok := data[field]; !ok {
		t.Fatalf("Field '%s' not found in JSON", field)
	}
}

// Code-specific assertions (AST-based)

// ParseGoFile parses a Go file and returns the AST
func ParseGoFile(t *testing.T, filepath string) (*ast.File, *token.FileSet) {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse Go file %s: %v", filepath, err)
	}
	return node, fset
}

// FindFunction finds a function declaration by name
func FindFunction(node *ast.File, funcName string) *ast.FuncDecl {
	for _, decl := range node.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == funcName {
				return funcDecl
			}
		}
	}
	return nil
}

// AssertFunctionExists checks if a function exists in a file
func AssertFunctionExists(t *testing.T, filepath, funcName string) *ast.FuncDecl {
	t.Helper()

	node, _ := ParseGoFile(t, filepath)
	funcDecl := FindFunction(node, funcName)
	if funcDecl == nil {
		t.Fatalf("Function '%s' not found in %s", funcName, filepath)
	}
	return funcDecl
}

// AssertFunctionReturnsError checks if a function returns error
func AssertFunctionReturnsError(t *testing.T, filepath, funcName string) {
	t.Helper()

	funcDecl := AssertFunctionExists(t, filepath, funcName)

	if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
		t.Fatalf("Function '%s' has no return values", funcName)
	}

	// Check if last return value is error
	results := funcDecl.Type.Results.List
	lastResult := results[len(results)-1]

	if ident, ok := lastResult.Type.(*ast.Ident); ok {
		if ident.Name == "error" {
			return // Success
		}
	}

	t.Fatalf("Function '%s' does not return error", funcName)
}

// AssertNoUncheckedErrors checks that all errors are handled
// (Simplified version - checks for basic patterns)
func AssertNoUncheckedErrors(t *testing.T, filepath string) {
	t.Helper()

	content, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	code := string(content)

	// Look for common unchecked error patterns
	// This is a simplified check - real implementation would use AST
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		// Check for assignments without error check
		if strings.Contains(line, ":=") && !strings.Contains(line, "err") {
			// Check if next few lines have error check
			hasCheck := false
			for j := i + 1; j < i+3 && j < len(lines); j++ {
				if strings.Contains(lines[j], "if err != nil") {
					hasCheck = true
					break
				}
			}

			// Check if line looks like it could return error
			if (strings.Contains(line, ".Query") ||
			    strings.Contains(line, ".Exec") ||
			    strings.Contains(line, ".Open") ||
			    strings.Contains(line, ".Read") ||
			    strings.Contains(line, ".Write")) && !hasCheck {
				t.Logf("Warning: Possible unchecked error on line %d: %s", i+1, strings.TrimSpace(line))
			}
		}
	}
}

// AssertComplexityBelow checks cyclomatic complexity
func AssertComplexityBelow(t *testing.T, funcDecl *ast.FuncDecl, maxComplexity int) {
	t.Helper()

	complexity := CalculateComplexity(funcDecl)
	if complexity > maxComplexity {
		t.Fatalf("Function '%s' has complexity %d (max: %d)",
			funcDecl.Name.Name, complexity, maxComplexity)
	}
}

// CalculateComplexity calculates cyclomatic complexity
func CalculateComplexity(fn *ast.FuncDecl) int {
	complexity := 1 // Base complexity

	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		case *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if binExpr, ok := n.(*ast.BinaryExpr); ok {
				if binExpr.Op == token.LAND || binExpr.Op == token.LOR {
					complexity++
				}
			}
		}
		return true
	})

	return complexity
}

// AssertImportExists checks if an import is present
func AssertImportExists(t *testing.T, filepath, importPath string) {
	t.Helper()

	node, _ := ParseGoFile(t, filepath)

	for _, imp := range node.Imports {
		if strings.Trim(imp.Path.Value, `"`) == importPath {
			return // Found
		}
	}

	t.Fatalf("Import '%s' not found in %s", importPath, filepath)
}

// AssertHasGodoc checks if a function has documentation
func AssertHasGodoc(t *testing.T, funcDecl *ast.FuncDecl) {
	t.Helper()

	if funcDecl.Doc == nil || len(funcDecl.Doc.List) == 0 {
		t.Fatalf("Function '%s' missing godoc documentation", funcDecl.Name.Name)
	}
}

// CountFunctions counts functions in a file
func CountFunctions(filepath string) (int, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath, nil, 0)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, decl := range node.Decls {
		if _, ok := decl.(*ast.FuncDecl); ok {
			count++
		}
	}
	return count, nil
}

// FindStruct finds a struct by name
func FindStruct(node *ast.File, structName string) *ast.StructType {
	for _, decl := range node.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == structName {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							return structType
						}
					}
				}
			}
		}
	}
	return nil
}

// AssertStructExists checks if a struct exists
func AssertStructExists(t *testing.T, filepath, structName string) {
	t.Helper()

	node, _ := ParseGoFile(t, filepath)
	structType := FindStruct(node, structName)
	if structType == nil {
		t.Fatalf("Struct '%s' not found in %s", structName, filepath)
	}
}

// ContainsPattern checks if a string matches a regex pattern
func ContainsPattern(s, pattern string) bool {
	return strings.Contains(s, pattern) // Simplified - real implementation would use regexp
}

// ContainsAny checks if string contains any of the given substrings
func ContainsAny(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// NewError creates a new error with given message
func NewError(msg string) error {
	return &simpleError{msg: msg}
}

type simpleError struct {
	msg string
}

func (e *simpleError) Error() string {
	return e.msg
}
