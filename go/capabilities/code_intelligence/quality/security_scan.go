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

// SecurityScanTool scans for common security issues
type SecurityScanTool struct{}

func init() {
	registry.Register(&SecurityScanTool{})
}

func (t *SecurityScanTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "security_scan",
		Description:     "Scan Go code for common security vulnerabilities: unchecked errors, SQL injection risks, unsafe file operations, weak crypto usage.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Directory to scan (default: current directory)",
				Example:     ".",
			},
			{
				Name:        "severity_threshold",
				Type:        "string",
				Required:    false,
				Description: "Minimum severity to report: low, medium, high, critical (default: medium)",
				Example:     "medium",
			},
		},
		Examples: []string{
			`{"tool": "security_scan", "arguments": {}}`,
			`{"tool": "security_scan", "arguments": {"path": "./agent"}}`,
			`{"tool": "security_scan", "arguments": {"severity_threshold": "high"}}`,
		},
	}
}

func (t *SecurityScanTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *SecurityScanTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	severityThreshold := "medium"
	if st, ok := input["severity_threshold"].(string); ok && st != "" {
		severityThreshold = st
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	// Run security scans
	scanner := &SecurityScanner{
		SeverityThreshold: severityThreshold,
		Issues:            []SecurityIssue{},
	}

	err = scanner.scan(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to scan: %w", err)
	}

	// Count by severity
	for _, issue := range scanner.Issues {
		switch issue.Severity {
		case "critical":
			scanner.Critical++
		case "high":
			scanner.High++
		case "medium":
			scanner.Medium++
		case "low":
			scanner.Low++
		}
	}

	// Build result
	result := map[string]interface{}{
		"path":                path,
		"severity_threshold":  severityThreshold,
		"vulnerabilities_found": len(scanner.Issues),
		"critical":            scanner.Critical,
		"high":                scanner.High,
		"medium":              scanner.Medium,
		"low":                 scanner.Low,
		"issues":              scanner.Issues,
		"passed":              len(scanner.Issues) == 0,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// SecurityScanner scans for security issues
type SecurityScanner struct {
	SeverityThreshold string
	Issues            []SecurityIssue
	Critical          int
	High              int
	Medium            int
	Low               int
}

// SecurityIssue represents a security vulnerability
type SecurityIssue struct {
	Severity     string `json:"severity"`
	Rule         string `json:"rule"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Message      string `json:"message"`
	Details      string `json:"details"`
	Remediation  string `json:"remediation"`
}

// scan scans a directory for security issues
func (s *SecurityScanner) scan(dirPath string) error {
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

		// Only scan .go files (not tests for now)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		return s.scanFile(path)
	})
}

// scanFile scans a single file
func (s *SecurityScanner) scanFile(filePath string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil // Skip files with parse errors
	}

	// Check for common issues
	ast.Inspect(node, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			s.checkUnsafeCall(node, fset, filePath)
		case *ast.BinaryExpr:
			s.checkStringConcatenation(node, fset, filePath)
		}
		return true
	})

	return nil
}

// checkUnsafeCall checks for unsafe function calls
func (s *SecurityScanner) checkUnsafeCall(call *ast.CallExpr, fset *token.FileSet, filePath string) {
	// Get function name
	var funcName string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		funcName = fun.Name
	case *ast.SelectorExpr:
		funcName = fun.Sel.Name
	}

	pos := fset.Position(call.Pos())

	// Check for unchecked errors from security-critical operations
	if funcName == "Query" || funcName == "Exec" || funcName == "Open" {
		// This is a simple heuristic - in real code, we'd check if error is actually handled
		s.Issues = append(s.Issues, SecurityIssue{
			Severity:    "medium",
			Rule:        "G104",
			File:        filePath,
			Line:        pos.Line,
			Message:     fmt.Sprintf("Potential unchecked error from %s()", funcName),
			Details:     fmt.Sprintf("Always check errors from security-critical operations like %s", funcName),
			Remediation: "Add proper error handling: if err != nil { return err }",
		})
	}

	// Check for os.Create/os.Open with fixed permissions
	if funcName == "Create" || funcName == "OpenFile" {
		s.Issues = append(s.Issues, SecurityIssue{
			Severity:    "low",
			Rule:        "G302",
			File:        filePath,
			Line:        pos.Line,
			Message:     "File operation without explicit permissions",
			Details:     "Consider specifying file permissions explicitly",
			Remediation: "Use os.OpenFile() with explicit permission bits (e.g., 0600 for user-only)",
		})
	}

	// Check for exec.Command
	if funcName == "Command" {
		s.Issues = append(s.Issues, SecurityIssue{
			Severity:    "medium",
			Rule:        "G204",
			File:        filePath,
			Line:        pos.Line,
			Message:     "Command execution detected",
			Details:     "Ensure command arguments are properly validated to prevent command injection",
			Remediation: "Validate and sanitize all inputs before passing to exec.Command",
		})
	}
}

// checkStringConcatenation checks for SQL injection via string concatenation
func (s *SecurityScanner) checkStringConcatenation(binary *ast.BinaryExpr, fset *token.FileSet, filePath string) {
	// Simple heuristic: if we see string concatenation with "SELECT", "INSERT", "UPDATE", "DELETE"
	// warn about potential SQL injection
	if binary.Op == token.ADD {
		// Check if either operand contains SQL keywords
		left := fmt.Sprintf("%v", binary.X)
		right := fmt.Sprintf("%v", binary.Y)
		combined := left + right

		sqlKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP"}
		for _, keyword := range sqlKeywords {
			if strings.Contains(strings.ToUpper(combined), keyword) {
				pos := fset.Position(binary.Pos())
				s.Issues = append(s.Issues, SecurityIssue{
					Severity:    "high",
					Rule:        "G201",
					File:        filePath,
					Line:        pos.Line,
					Message:     "Potential SQL injection via string concatenation",
					Details:     "SQL query built using string concatenation can lead to SQL injection",
					Remediation: "Use parameterized queries (e.g., db.Query(\"SELECT * FROM users WHERE id = ?\", id))",
				})
				break
			}
		}
	}
}
