package feedback

import (
	"testing"
)

// TestAnalyzeCompileError_MissingImport tests detection of missing imports
func TestAnalyzeCompileError_MissingImport(t *testing.T) {
	errorMsg := `user.go:17:10: undefined: fmt`

	analysis := AnalyzeCompileError(errorMsg)

	if analysis.Severity != ErrorSeveritySimple {
		t.Errorf("Expected simple severity, got %s", analysis.Severity)
	}

	if analysis.ErrorType != "missing_import_or_typo" {
		t.Errorf("Expected missing_import_or_typo, got %s", analysis.ErrorType)
	}

	if !analysis.Fixable {
		t.Error("Expected error to be fixable")
	}

	if analysis.FilesCount != 1 {
		t.Errorf("Expected 1 file, got %d", analysis.FilesCount)
	}
}

// TestAnalyzeCompileError_Typo tests detection of typos
func TestAnalyzeCompileError_Typo(t *testing.T) {
	errorMsg := `handler.go:25:3: undeclared name: usre (did you mean user?)`

	analysis := AnalyzeCompileError(errorMsg)

	if analysis.Severity != ErrorSeveritySimple {
		t.Errorf("Expected simple severity, got %s", analysis.Severity)
	}

	if analysis.ErrorType != "missing_import_or_typo" {
		t.Errorf("Expected missing_import_or_typo, got %s", analysis.ErrorType)
	}
}

// TestAnalyzeCompileError_SyntaxError tests detection of syntax errors
func TestAnalyzeCompileError_SyntaxError(t *testing.T) {
	errorMsg := `validator.go:42:15: expected ';', found 'EOF'`

	analysis := AnalyzeCompileError(errorMsg)

	if analysis.Severity != ErrorSeveritySimple {
		t.Errorf("Expected simple severity, got %s", analysis.Severity)
	}

	if analysis.ErrorType != "syntax_error" {
		t.Errorf("Expected syntax_error, got %s", analysis.ErrorType)
	}
}

// TestAnalyzeCompileError_TypeMismatch tests detection of type errors
func TestAnalyzeCompileError_TypeMismatch(t *testing.T) {
	errorMsg := `user.go:30:15: cannot use userID (variable of type string) as int value in argument to GetUser`

	analysis := AnalyzeCompileError(errorMsg)

	if analysis.Severity != ErrorSeveritySimple {
		t.Errorf("Expected simple severity, got %s", analysis.Severity)
	}

	if analysis.ErrorType != "type_error" {
		t.Errorf("Expected type_error, got %s", analysis.ErrorType)
	}
}

// TestAnalyzeCompileError_MultipleFiles tests detection of multi-file errors
func TestAnalyzeCompileError_MultipleFiles(t *testing.T) {
	errorMsg := `user.go:17:10: undefined: fmt
handler.go:25:3: undefined: fmt
validator.go:15:8: undefined: fmt`

	analysis := AnalyzeCompileError(errorMsg)

	if analysis.Severity != ErrorSeverityComplex {
		t.Errorf("Expected complex severity, got %s", analysis.Severity)
	}

	if analysis.ErrorType != "multi_file_error" {
		t.Errorf("Expected multi_file_error, got %s", analysis.ErrorType)
	}

	if analysis.FilesCount != 3 {
		t.Errorf("Expected 3 files, got %d", analysis.FilesCount)
	}
}

// TestAnalyzeCompileError_TooManyErrors tests detection of many errors
func TestAnalyzeCompileError_TooManyErrors(t *testing.T) {
	errorMsg := `user.go:10:5: error 1
user.go:15:10: error 2
user.go:20:3: error 3
user.go:25:7: error 4
user.go:30:12: error 5
user.go:35:8: error 6`

	analysis := AnalyzeCompileError(errorMsg)

	if analysis.Severity != ErrorSeverityComplex {
		t.Errorf("Expected complex severity for many errors, got %s", analysis.Severity)
	}

	if analysis.ErrorCount <= 5 {
		t.Errorf("Expected more than 5 errors, got %d", analysis.ErrorCount)
	}
}

// TestCountAffectedFiles tests file counting
func TestCountAffectedFiles(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		want     int
	}{
		{
			name:     "single file",
			errorMsg: `user.go:17:10: undefined: fmt`,
			want:     1,
		},
		{
			name: "three files",
			errorMsg: `user.go:17:10: error
handler.go:25:3: error
validator.go:15:8: error`,
			want: 3,
		},
		{
			name: "same file multiple times",
			errorMsg: `user.go:17:10: error
user.go:25:5: error
user.go:30:8: error`,
			want: 1,
		},
		{
			name:     "no files",
			errorMsg: `some generic error message`,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countAffectedFiles(tt.errorMsg)
			if got != tt.want {
				t.Errorf("countAffectedFiles() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCountErrors tests error counting
func TestCountErrors(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		wantMin  int
	}{
		{
			name:     "single error",
			errorMsg: `user.go:17:10: undefined: fmt`,
			wantMin:  1,
		},
		{
			name: "multiple errors",
			errorMsg: `user.go:17:10: error 1
user.go:25:5: error 2
user.go:30:8: error 3`,
			wantMin: 3,
		},
		{
			name:     "generic error",
			errorMsg: `compilation failed`,
			wantMin:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countErrors(tt.errorMsg)
			if got < tt.wantMin {
				t.Errorf("countErrors() = %d, want at least %d", got, tt.wantMin)
			}
		})
	}
}

// TestFormatFixPrompt tests prompt generation
func TestFormatFixPrompt(t *testing.T) {
	analysis := &CompileErrorAnalysis{
		Severity:   ErrorSeveritySimple,
		ErrorType:  "missing_import_or_typo",
		FilesCount: 1,
		ErrorCount: 1,
		Fixable:    true,
		Suggestion: "Add missing import",
	}

	errorMsg := `user.go:17:10: undefined: fmt`
	prompt := analysis.FormatFixPrompt(errorMsg)

	// Verify prompt contains key elements
	if !contains(prompt, "Compilation failed") {
		t.Error("Prompt should contain 'Compilation failed'")
	}

	if !contains(prompt, errorMsg) {
		t.Error("Prompt should contain the error message")
	}

	if !contains(prompt, "Type: missing_import_or_typo") {
		t.Error("Prompt should contain error type")
	}

	if !contains(prompt, "modify_file") {
		t.Error("Prompt should mention modify_file tool")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
