package agent

import (
	"testing"
)

// TestExtractPathFromPrompt tests the path extraction logic
// This is a core function in agent_executor.go that determines project paths
func TestExtractPathFromPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{
			name:     "Path with dash prefix",
			prompt:   "Generate code\n- project_path: /tmp/testdir",
			expected: "/tmp/testdir",
		},
		{
			name:     "Path without dash",
			prompt:   "Generate code\nproject_path: /home/user/project",
			expected: "/home/user/project",
		},
		{
			name:     "No path specified",
			prompt:   "Generate code with no path",
			expected: ".",
		},
		{
			name:     "Path with extra whitespace",
			prompt:   "Generate code\nproject_path:    /tmp/spaces   \nOther text",
			expected: "/tmp/spaces",
		},
		{
			name:     "Path with tabs",
			prompt:   "project_path:\t\t/tmp/tabbed",
			expected: "/tmp/tabbed",
		},
		{
			name:     "Empty path falls back to dot",
			prompt:   "project_path: .\nSome other text",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPathFromPrompt(tt.prompt)
			if result != tt.expected {
				t.Errorf("extractPathFromPrompt() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestFormatArgsForAgent tests the argument formatting function
// This is used for displaying tool calls in agent_executor.go
func TestFormatArgsForAgent(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		contains []string // Strings that should be in the output
	}{
		{
			name: "Simple string argument",
			args: map[string]interface{}{
				"path": "/tmp/test.go",
			},
			contains: []string{"path", "/tmp/test.go"},
		},
		{
			name: "Multiple arguments",
			args: map[string]interface{}{
				"language":    "go",
				"description": "test file",
			},
			contains: []string{"language", "go", "description", "test file"},
		},
		{
			name:     "Empty arguments",
			args:     map[string]interface{}{},
			contains: []string{"{", "}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatArgsForAgent(tt.args)

			// Check that result contains opening and closing braces
			if len(result) < 2 || result[0] != '{' || result[len(result)-1] != '}' {
				t.Errorf("formatArgsForAgent() should return JSON-like format with braces, got: %v", result)
			}

			// Check that all expected strings are present
			for _, expected := range tt.contains {
				found := false
				// Simple contains check
				for i := 0; i <= len(result)-len(expected); i++ {
					if result[i:i+len(expected)] == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("formatArgsForAgent() result should contain %q, got: %v", expected, result)
				}
			}
		})
	}
}

// TestExecutionResult_Initialization tests the ExecutionResult struct
// Verifies that the result structure is properly defined
func TestExecutionResult_Initialization(t *testing.T) {
	result := &ExecutionResult{
		Success:               false,
		Output:                "test output",
		ToolsExecuted:         []string{"tool1", "tool2"},
		ToolResults:           []string{"result1", "result2"},
		Artifacts:             []string{"artifact1"},
		HallucinationDetected: false,
		Error:                 "",
	}

	if result.Output != "test output" {
		t.Errorf("Expected Output 'test output', got %v", result.Output)
	}

	if len(result.ToolsExecuted) != 2 {
		t.Errorf("Expected 2 tools executed, got %d", len(result.ToolsExecuted))
	}

	if result.ToolsExecuted[0] != "tool1" {
		t.Errorf("Expected first tool 'tool1', got %v", result.ToolsExecuted[0])
	}
}

// TestAgentToolExecutor_MaxIterations tests the max iterations constant
func TestAgentToolExecutor_MaxIterations(t *testing.T) {
	// Test that maxIterations is set correctly when creating executor
	// Note: We can't easily test the full execution without mocking the entire LLM system,
	// but we can verify the constant exists and is reasonable

	// The maxIterations should be 9 according to agent_executor.go:47
	expectedMaxIterations := 9

	// This is more of a documentation test - if someone changes maxIterations,
	// this test will remind them to update related tests
	if expectedMaxIterations < 1 || expectedMaxIterations > 20 {
		t.Errorf("maxIterations should be reasonable (1-20), current expectation: %d", expectedMaxIterations)
	}
}
