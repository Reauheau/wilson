package agent

import (
	"fmt"
	"strings"
)

// ErrorSeverity indicates whether an error can be fixed in place or needs a separate task
type ErrorSeverity string

const (
	ErrorSeveritySimple  ErrorSeverity = "simple"  // Fix in place with iterative loop
	ErrorSeverityComplex ErrorSeverity = "complex" // Need separate fix task
)

// CompileErrorAnalysis contains analysis of a compilation error
type CompileErrorAnalysis struct {
	Severity   ErrorSeverity
	ErrorType  string // "missing_import", "typo", "syntax", "logic", "multi_file"
	Fixable    bool
	Suggestion string
	FilesCount int
	ErrorCount int
}

// AnalyzeCompileError analyzes a compilation error and determines how to handle it
func AnalyzeCompileError(errorMsg string) *CompileErrorAnalysis {
	analysis := &CompileErrorAnalysis{
		FilesCount: countAffectedFiles(errorMsg),
		ErrorCount: countErrors(errorMsg),
	}

	// Multiple files affected = complex (needs separate task)
	if analysis.FilesCount > 1 {
		analysis.Severity = ErrorSeverityComplex
		analysis.ErrorType = "multi_file_error"
		analysis.Fixable = true
		analysis.Suggestion = "Create fix task to address errors across multiple files"
		return analysis
	}

	// Too many errors = complex (needs careful analysis)
	if analysis.ErrorCount > 5 {
		analysis.Severity = ErrorSeverityComplex
		analysis.ErrorType = "multiple_errors"
		analysis.Fixable = true
		analysis.Suggestion = "Create fix task to systematically address all errors"
		return analysis
	}

	// Check for simple, auto-fixable errors
	errorLower := strings.ToLower(errorMsg)

	// Missing import or undefined identifier (most common)
	if strings.Contains(errorLower, "undefined:") ||
		strings.Contains(errorLower, "undeclared name:") ||
		strings.Contains(errorLower, "not declared") {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "missing_import_or_typo"
		analysis.Fixable = true
		analysis.Suggestion = "Add missing import or fix variable/function name"
		return analysis
	}

	// Missing package
	if strings.Contains(errorLower, "package") &&
		(strings.Contains(errorLower, "not in goroot") ||
			strings.Contains(errorLower, "not found")) {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "missing_package"
		analysis.Fixable = true
		analysis.Suggestion = "Add missing package import"
		return analysis
	}

	// Syntax errors (usually simple)
	if strings.Contains(errorLower, "expected") ||
		strings.Contains(errorLower, "syntax error") ||
		strings.Contains(errorLower, "missing") && strings.Contains(errorLower, "';'") {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "syntax_error"
		analysis.Fixable = true
		analysis.Suggestion = "Fix syntax error (missing semicolon, bracket, etc.)"
		return analysis
	}

	// Type mismatch (usually simple)
	if strings.Contains(errorLower, "cannot use") ||
		strings.Contains(errorLower, "type mismatch") ||
		strings.Contains(errorLower, "cannot convert") {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "type_error"
		analysis.Fixable = true
		analysis.Suggestion = "Fix type mismatch or add type conversion"
		return analysis
	}

	// Return/assignment errors (simple)
	if strings.Contains(errorLower, "too many arguments") ||
		strings.Contains(errorLower, "not enough arguments") ||
		strings.Contains(errorLower, "too many return values") ||
		strings.Contains(errorLower, "not enough return values") {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "argument_mismatch"
		analysis.Fixable = true
		analysis.Suggestion = "Fix function call or return statement"
		return analysis
	}

	// Single file, single error = probably simple
	if analysis.FilesCount <= 1 && analysis.ErrorCount <= 2 {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "single_error"
		analysis.Fixable = true
		analysis.Suggestion = "Analyze and fix the error"
		return analysis
	}

	// Default: treat as complex if uncertain
	analysis.Severity = ErrorSeverityComplex
	analysis.ErrorType = "complex_error"
	analysis.Fixable = false
	analysis.Suggestion = "Error requires careful analysis - create separate fix task"
	return analysis
}

// countAffectedFiles counts how many unique files are mentioned in the error
func countAffectedFiles(errorMsg string) int {
	files := make(map[string]bool)
	lines := strings.Split(errorMsg, "\n")

	for _, line := range lines {
		// Look for patterns like: "path/file.go:10:5:"
		if idx := strings.Index(line, ".go:"); idx != -1 {
			// Extract everything before ".go:"
			filePath := line[:idx+3] // Include ".go"
			// Get just the filename without path (for deduplication)
			parts := strings.Split(filePath, "/")
			if len(parts) > 0 {
				files[parts[len(parts)-1]] = true
			}
		}
	}

	return len(files)
}

// countErrors counts approximate number of distinct errors
func countErrors(errorMsg string) int {
	// Count lines that look like error messages
	// Usually start with file path or contain "error:"
	lines := strings.Split(errorMsg, "\n")
	count := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Error lines usually contain ":" and either a file path or "error:"
		if (strings.Contains(line, ".go:") && strings.Count(line, ":") >= 3) ||
			strings.Contains(strings.ToLower(line), "error:") {
			count++
		}
	}

	// If no structured errors found, count as 1
	if count == 0 {
		count = 1
	}

	return count
}

// FormatFixPrompt creates a user-friendly prompt for the LLM to fix the error
func (a *CompileErrorAnalysis) FormatFixPrompt(errorMsg string) string {
	var prompt strings.Builder

	prompt.WriteString("Compilation failed with the following error:\n\n")
	prompt.WriteString("```\n")
	prompt.WriteString(errorMsg)
	prompt.WriteString("\n```\n\n")

	prompt.WriteString(fmt.Sprintf("Error Analysis:\n"))
	prompt.WriteString(fmt.Sprintf("- Type: %s\n", a.ErrorType))
	prompt.WriteString(fmt.Sprintf("- Severity: %s\n", a.Severity))
	prompt.WriteString(fmt.Sprintf("- Affected files: %d\n", a.FilesCount))
	prompt.WriteString(fmt.Sprintf("- Number of errors: %d\n\n", a.ErrorCount))

	prompt.WriteString(fmt.Sprintf("Suggestion: %s\n\n", a.Suggestion))

	prompt.WriteString("Please fix the error by:\n")
	switch a.ErrorType {
	case "missing_import_or_typo":
		prompt.WriteString("1. Identifying the undefined identifier\n")
		prompt.WriteString("2. Adding the missing import statement if needed\n")
		prompt.WriteString("3. Or correcting the typo in the variable/function name\n")
		prompt.WriteString("4. Using modify_file to update the code\n")

	case "missing_package":
		prompt.WriteString("1. Identifying the missing package\n")
		prompt.WriteString("2. Adding the correct import statement\n")
		prompt.WriteString("3. Using modify_file to add the import\n")

	case "syntax_error":
		prompt.WriteString("1. Locating the syntax error\n")
		prompt.WriteString("2. Fixing the missing bracket, semicolon, or other syntax issue\n")
		prompt.WriteString("3. Using modify_file to correct the syntax\n")

	case "type_error":
		prompt.WriteString("1. Understanding the type mismatch\n")
		prompt.WriteString("2. Adding type conversion or fixing the type\n")
		prompt.WriteString("3. Using modify_file to update the code\n")

	case "argument_mismatch":
		prompt.WriteString("1. Checking the function signature\n")
		prompt.WriteString("2. Adjusting the number of arguments or return values\n")
		prompt.WriteString("3. Using modify_file to fix the call or return\n")

	default:
		prompt.WriteString("1. Analyzing the error message carefully\n")
		prompt.WriteString("2. Determining the root cause\n")
		prompt.WriteString("3. Applying the appropriate fix\n")
		prompt.WriteString("4. Using modify_file to update the code\n")
	}

	return prompt.String()
}
