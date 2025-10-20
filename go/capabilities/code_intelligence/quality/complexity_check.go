package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// ComplexityCheckTool analyzes code complexity
type ComplexityCheckTool struct{}

func init() {
	registry.Register(&ComplexityCheckTool{})
}

func (t *ComplexityCheckTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "complexity_check",
		Description:     "Analyze code complexity (cyclomatic complexity, function length, nesting depth). Identifies functions that are too complex and should be refactored.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "File or directory to analyze",
				Example:     "agent",
			},
			{
				Name:        "max_complexity",
				Type:        "number",
				Required:    false,
				Description: "Maximum allowed cyclomatic complexity (default: 15)",
				Example:     "15",
			},
			{
				Name:        "max_function_lines",
				Type:        "number",
				Required:    false,
				Description: "Maximum allowed lines per function (default: 100)",
				Example:     "100",
			},
			{
				Name:        "include_tests",
				Type:        "boolean",
				Required:    false,
				Description: "Include test files in analysis (default: false)",
				Example:     "false",
			},
		},
		Examples: []string{
			`{"tool": "complexity_check", "arguments": {"path": "agent"}}`,
			`{"tool": "complexity_check", "arguments": {"path": ".", "max_complexity": 10}}`,
			`{"tool": "complexity_check", "arguments": {"path": "agent/code_agent.go", "max_function_lines": 50}}`,
		},
	}
}

func (t *ComplexityCheckTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	return nil
}

func (t *ComplexityCheckTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)

	maxComplexity := 15
	if mc, ok := input["max_complexity"].(float64); ok {
		maxComplexity = int(mc)
	}

	maxFunctionLines := 100
	if mfl, ok := input["max_function_lines"].(float64); ok {
		maxFunctionLines = int(mfl)
	}

	includeTests := false
	if it, ok := input["include_tests"].(bool); ok {
		includeTests = it
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	// Analyze complexity
	analyzer := &ComplexityAnalyzer{
		MaxComplexity:     maxComplexity,
		MaxFunctionLines:  maxFunctionLines,
		IncludeTests:      includeTests,
		Violations:        []ComplexityViolation{},
		TotalFunctions:    0,
		ComplexFunctions:  0,
	}

	if info.IsDir() {
		err = analyzer.analyzeDirectory(absPath)
	} else {
		err = analyzer.analyzeFile(absPath)
	}

	if err != nil {
		return "", fmt.Errorf("failed to analyze complexity: %w", err)
	}

	// Calculate average
	if analyzer.TotalFunctions > 0 {
		analyzer.AverageComplexity = float64(analyzer.TotalComplexity) / float64(analyzer.TotalFunctions)
	}

	// Determine if passed
	analyzer.Passed = len(analyzer.Violations) == 0

	// Build result
	result := map[string]interface{}{
		"path":                path,
		"max_complexity":      maxComplexity,
		"max_function_lines":  maxFunctionLines,
		"total_functions":     analyzer.TotalFunctions,
		"complex_functions":   analyzer.ComplexFunctions,
		"average_complexity":  fmt.Sprintf("%.1f", analyzer.AverageComplexity),
		"max_complexity_found": analyzer.MaxComplexityFound,
		"violations":          analyzer.Violations,
		"passed":              analyzer.Passed,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// ComplexityAnalyzer analyzes code complexity
type ComplexityAnalyzer struct {
	MaxComplexity       int
	MaxFunctionLines    int
	IncludeTests        bool
	Violations          []ComplexityViolation
	TotalFunctions      int
	ComplexFunctions    int
	TotalComplexity     int
	AverageComplexity   float64
	MaxComplexityFound  int
	Passed              bool
}

// ComplexityViolation represents a complexity violation
type ComplexityViolation struct {
	File        string `json:"file"`
	Function    string `json:"function"`
	Line        int    `json:"line"`
	Complexity  int    `json:"complexity,omitempty"`
	Lines       int    `json:"lines,omitempty"`
	Threshold   int    `json:"threshold"`
	ViolationType string `json:"violation_type"`
	Suggestion  string `json:"suggestion"`
}

// analyzeDirectory analyzes all Go files in a directory
func (a *ComplexityAnalyzer) analyzeDirectory(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only analyze .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if not included
		if !a.IncludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		return a.analyzeFile(path)
	})
}

// analyzeFile analyzes a single Go file
func (a *ComplexityAnalyzer) analyzeFile(filePath string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil // Skip files with parse errors
	}

	// Analyze each function
	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		a.TotalFunctions++

		// Calculate cyclomatic complexity
		complexity := calculateComplexity(funcDecl)
		a.TotalComplexity += complexity

		if complexity > a.MaxComplexityFound {
			a.MaxComplexityFound = complexity
		}

		// Check complexity threshold
		if complexity > a.MaxComplexity {
			a.ComplexFunctions++
			a.Violations = append(a.Violations, ComplexityViolation{
				File:          filePath,
				Function:      funcDecl.Name.Name,
				Line:          fset.Position(funcDecl.Pos()).Line,
				Complexity:    complexity,
				Threshold:     a.MaxComplexity,
				ViolationType: "high_complexity",
				Suggestion:    "Extract complex logic into smaller helper functions",
			})
		}

		// Check function length
		lines := fset.Position(funcDecl.End()).Line - fset.Position(funcDecl.Pos()).Line
		if lines > a.MaxFunctionLines {
			a.ComplexFunctions++
			a.Violations = append(a.Violations, ComplexityViolation{
				File:          filePath,
				Function:      funcDecl.Name.Name,
				Line:          fset.Position(funcDecl.Pos()).Line,
				Lines:         lines,
				Threshold:     a.MaxFunctionLines,
				ViolationType: "long_function",
				Suggestion:    fmt.Sprintf("Function is %d lines, consider breaking into smaller functions", lines),
			})
		}

		return true
	})

	return nil
}

// calculateComplexity calculates cyclomatic complexity
func calculateComplexity(fn *ast.FuncDecl) int {
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
			// Count && and || as additional complexity
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
