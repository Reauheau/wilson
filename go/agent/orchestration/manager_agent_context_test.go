package orchestration

import (
	"os"
	"testing"
	"wilson/agent/base"
)

// TestExtractFilenameFromError tests extracting filenames from Go compile errors
func TestExtractFilenameFromError(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		expected string
	}{
		{
			name:     "Simple compile error",
			errorMsg: "main.go:10:5: undefined: fmt",
			expected: "main.go",
		},
		{
			name:     "Full path compile error",
			errorMsg: "/Users/user/project/handler.go:25:10: syntax error",
			expected: "/Users/user/project/handler.go",
		},
		{
			name:     "Multi-line error with first file",
			errorMsg: "main.go:10:5: undefined: fmt\nhelper.go:5:1: syntax error",
			expected: "main.go",
		},
		{
			name:     "Error with relative path",
			errorMsg: "pkg/utils/validator.go:42:3: type mismatch",
			expected: "pkg/utils/validator.go",
		},
		{
			name:     "No .go file in error",
			errorMsg: "build failed: exit status 1",
			expected: "",
		},
		{
			name:     "URL in error (should be ignored)",
			errorMsg: "http://example.com/file.go:10:5: error",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFilenameFromError(tt.errorMsg)
			if result != tt.expected {
				t.Errorf("extractFilenameFromError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestLoadRequiredFiles_FixMode tests loading file content for fix mode tasks
func TestLoadRequiredFiles_FixMode(t *testing.T) {
	// Create a temporary file with content
	tmpFile, err := os.CreateTemp("", "wilson-test-*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testContent := "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}\n"
	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create manager and task context
	manager := &ManagerAgent{}
	taskCtx := &base.TaskContext{
		Input: map[string]interface{}{
			"fix_mode":    true,
			"target_file": tmpFile.Name(),
		},
	}

	// Load required files
	err = manager.loadRequiredFiles(taskCtx)
	if err != nil {
		t.Errorf("loadRequiredFiles() returned error: %v", err)
	}

	// Verify file content was loaded
	if content, ok := taskCtx.Input["file_content"].(string); ok {
		if content != testContent {
			t.Errorf("Expected file_content = %q, got %q", testContent, content)
		}
	} else {
		t.Error("file_content not found in taskCtx.Input")
	}
}

// TestLoadRequiredFiles_CompileError tests loading file content from compile error
func TestLoadRequiredFiles_CompileError(t *testing.T) {
	// Create a temporary file with content
	tmpFile, err := os.CreateTemp("", "wilson-test-*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testContent := "package main\n\nfunc test() error {\n\treturn nil\n}\n"
	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create manager and task context with compile error
	manager := &ManagerAgent{}
	compileError := tmpFile.Name() + ":3:10: undefined: somethingMissing"
	taskCtx := &base.TaskContext{
		Input: map[string]interface{}{
			"compile_error": compileError,
		},
	}

	// Load required files
	err = manager.loadRequiredFiles(taskCtx)
	if err != nil {
		t.Errorf("loadRequiredFiles() returned error: %v", err)
	}

	// Verify file content was loaded
	if content, ok := taskCtx.Input["file_content"].(string); ok {
		if content != testContent {
			t.Errorf("Expected file_content = %q, got %q", testContent, content)
		}
	} else {
		t.Error("file_content not found in taskCtx.Input")
	}
}

// TestLoadRequiredFiles_FixModeTakesPrecedence tests that fix_mode takes precedence over compile_error
func TestLoadRequiredFiles_FixModeTakesPrecedence(t *testing.T) {
	// Create two temporary files
	fixFile, err := os.CreateTemp("", "wilson-fix-*.go")
	if err != nil {
		t.Fatalf("Failed to create fix file: %v", err)
	}
	defer os.Remove(fixFile.Name())

	fixContent := "package main\n\nfunc fixThis() {}\n"
	if _, err := fixFile.WriteString(fixContent); err != nil {
		t.Fatalf("Failed to write to fix file: %v", err)
	}
	fixFile.Close()

	compileFile, err := os.CreateTemp("", "wilson-compile-*.go")
	if err != nil {
		t.Fatalf("Failed to create compile file: %v", err)
	}
	defer os.Remove(compileFile.Name())

	compileContent := "package main\n\nfunc compileError() {}\n"
	if _, err := compileFile.WriteString(compileContent); err != nil {
		t.Fatalf("Failed to write to compile file: %v", err)
	}
	compileFile.Close()

	// Create manager and task context with both fix_mode and compile_error
	manager := &ManagerAgent{}
	compileError := compileFile.Name() + ":3:10: some error"
	taskCtx := &base.TaskContext{
		Input: map[string]interface{}{
			"fix_mode":      true,
			"target_file":   fixFile.Name(),
			"compile_error": compileError,
		},
	}

	// Load required files
	err = manager.loadRequiredFiles(taskCtx)
	if err != nil {
		t.Errorf("loadRequiredFiles() returned error: %v", err)
	}

	// Verify fix_mode file content was loaded (not compile_error file)
	if content, ok := taskCtx.Input["file_content"].(string); ok {
		if content != fixContent {
			t.Errorf("Expected fix file content, but got different content")
		}
		if content == compileContent {
			t.Error("Got compile error file content instead of fix file content")
		}
	} else {
		t.Error("file_content not found in taskCtx.Input")
	}
}

// TestLoadRequiredFiles_NoFiles tests that method handles missing files gracefully
func TestLoadRequiredFiles_NoFiles(t *testing.T) {
	manager := &ManagerAgent{}

	tests := []struct {
		name    string
		taskCtx *base.TaskContext
	}{
		{
			name: "Empty input",
			taskCtx: &base.TaskContext{
				Input: map[string]interface{}{},
			},
		},
		{
			name: "Fix mode with non-existent file",
			taskCtx: &base.TaskContext{
				Input: map[string]interface{}{
					"fix_mode":    true,
					"target_file": "/tmp/nonexistent-file-12345.go",
				},
			},
		},
		{
			name: "Compile error with non-existent file",
			taskCtx: &base.TaskContext{
				Input: map[string]interface{}{
					"compile_error": "/tmp/nonexistent-file-67890.go:10:5: error",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not return error even if files don't exist
			err := manager.loadRequiredFiles(tt.taskCtx)
			if err != nil {
				t.Errorf("loadRequiredFiles() should not return error for missing files, got: %v", err)
			}

			// file_content should not be set
			if _, exists := tt.taskCtx.Input["file_content"]; exists {
				t.Error("file_content should not be set for non-existent files")
			}
		})
	}
}

// TestLoadRequiredFiles_EmptyCompileError tests handling of empty compile error string
func TestLoadRequiredFiles_EmptyCompileError(t *testing.T) {
	manager := &ManagerAgent{}
	taskCtx := &base.TaskContext{
		Input: map[string]interface{}{
			"compile_error": "",
		},
	}

	err := manager.loadRequiredFiles(taskCtx)
	if err != nil {
		t.Errorf("loadRequiredFiles() returned error: %v", err)
	}

	// file_content should not be set
	if _, exists := taskCtx.Input["file_content"]; exists {
		t.Error("file_content should not be set for empty compile error")
	}
}
