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

	// Too many errors = complex UNLESS they're all the same type
	if analysis.ErrorCount > 5 {
		// ✅ SMART: Check if all errors are the same type (e.g., all "undefined")
		// If so, they're just multiple simple fixes, not complex
		allUndefined := true
		allFormatString := true
		lines := strings.Split(errorMsg, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.Contains(line, ".go:") {
				if !strings.Contains(line, "undefined:") {
					allUndefined = false
				}
				if !strings.Contains(line, "format %") || !strings.Contains(line, "has arg") {
					allFormatString = false
				}
			}
		}

		// If all errors are the same simple type, treat as simple
		if allUndefined {
			analysis.Severity = ErrorSeveritySimple
			analysis.ErrorType = "multiple_undefined"
			analysis.Fixable = true
			analysis.Suggestion = "Fix each undefined identifier using edit_line"
			return analysis
		}
		if allFormatString {
			analysis.Severity = ErrorSeveritySimple
			analysis.ErrorType = "multiple_format_errors"
			analysis.Fixable = true
			analysis.Suggestion = "Fix each format string using edit_line"
			return analysis
		}

		// Mixed error types = truly complex
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
		strings.Contains(errorLower, "cannot convert") ||
		strings.Contains(errorLower, "wrong type") ||
		strings.Contains(errorLower, "format %") && strings.Contains(errorLower, "has arg") {
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

	// ✅ NEW: Unused import/variable (very simple, common issue)
	if strings.Contains(errorLower, "imported and not used") ||
		strings.Contains(errorLower, "declared and not used") ||
		strings.Contains(errorLower, "declared but not used") {
		analysis.Severity = ErrorSeveritySimple
		analysis.ErrorType = "unused_import_or_variable"
		analysis.Fixable = true
		analysis.Suggestion = "Remove unused import or variable declaration"
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

	prompt.WriteString("**CRITICAL: Use edit_line tool ONLY - no generate_code, no modify_file**\n\n")
	prompt.WriteString("How to fix:\n")
	prompt.WriteString("1. Extract line number from error message (format: './file.go:LINE:COL: message')\n")
	prompt.WriteString("2. You can see the file content above - identify the problematic line\n")
	prompt.WriteString("3. Use edit_line to fix that specific line:\n")
	prompt.WriteString("   {\"tool\": \"edit_line\", \"arguments\": {\"path\": \"file.go\", \"line\": LINE, \"new_content\": \"corrected line\"}}\n\n")

	prompt.WriteString("Error-specific guidance:\n")
	switch a.ErrorType {
	case "missing_import_or_typo":
		prompt.WriteString("• Undefined identifier - either missing import OR typo\n")
		prompt.WriteString("• Missing import: Add import at top (find import block line, use edit_line to add)\n")
		prompt.WriteString("• Typo: Fix the name on the error line\n")

	case "missing_package":
		prompt.WriteString("• Find import block (usually lines 3-10)\n")
		prompt.WriteString("• Use edit_line to add missing import line\n")

	case "syntax_error":
		prompt.WriteString("• Fix syntax on the exact line in error message\n")
		prompt.WriteString("• Add missing bracket/semicolon/quote using edit_line\n")

	case "type_error":
		if strings.Contains(errorMsg, "format %") && strings.Contains(errorMsg, "has arg") {
			prompt.WriteString("• Printf format error: %d (int), %g (float), %f (float), %s (string)\n")
			prompt.WriteString("• Change format specifier on error line (e.g., %d → %g for float64)\n")
		} else {
			prompt.WriteString("• Type mismatch - add conversion or fix type on error line\n")
		}

	case "argument_mismatch":
		prompt.WriteString("• Function call has wrong argument count\n")
		prompt.WriteString("• Fix on error line - add/remove arguments to match signature\n")

	case "unused_import_or_variable":
		prompt.WriteString("• Find unused import/variable line number\n")
		prompt.WriteString("• Use edit_line to remove that line (or comment it out)\n")

	case "multiple_undefined":
		prompt.WriteString("• Multiple undefined names - check source files for correct names\n")
		prompt.WriteString("• Use edit_line multiple times (once per error line)\n")

	case "multiple_format_errors":
		prompt.WriteString("• Multiple format errors - fix each line\n")
		prompt.WriteString("• Use edit_line multiple times: %d → %g for float64\n")

	default:
		prompt.WriteString("• Analyze error and fix the specific line mentioned\n")
	}

	prompt.WriteString("\n**ONE TOOL ONLY: edit_line with line number from error**\n")

	return prompt.String()
}
