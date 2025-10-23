package agent

import (
	"testing"
)

// TestHasCriticalIssues tests critical issue detection
func TestHasCriticalIssues(t *testing.T) {
	tests := []struct {
		name     string
		results  []*ValidationResult
		expected bool
	}{
		{
			name: "No issues",
			results: []*ValidationResult{
				{Passed: true, Severity: SeverityInfo},
				{Passed: true, Severity: SeverityInfo},
			},
			expected: false,
		},
		{
			name: "Warning only",
			results: []*ValidationResult{
				{Passed: false, Severity: SeverityWarning},
			},
			expected: false,
		},
		{
			name: "Critical issue",
			results: []*ValidationResult{
				{Passed: false, Severity: SeverityCritical},
			},
			expected: true,
		},
		{
			name: "Mixed with critical",
			results: []*ValidationResult{
				{Passed: true, Severity: SeverityInfo},
				{Passed: false, Severity: SeverityWarning},
				{Passed: false, Severity: SeverityCritical},
			},
			expected: true,
		},
		{
			name:     "Empty results",
			results:  []*ValidationResult{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasCriticalIssues(tt.results)
			if result != tt.expected {
				t.Errorf("HasCriticalIssues() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetFailedChecks tests failed check extraction
func TestGetFailedChecks(t *testing.T) {
	tests := []struct {
		name          string
		results       []*ValidationResult
		expectedCount int
		shouldContain string
	}{
		{
			name: "No failures",
			results: []*ValidationResult{
				{Passed: true, ToolUsed: "test1"},
				{Passed: true, ToolUsed: "test2"},
			},
			expectedCount: 0,
		},
		{
			name: "One failure",
			results: []*ValidationResult{
				{Passed: true, ToolUsed: "test1"},
				{Passed: false, ToolUsed: "test2"},
			},
			expectedCount: 1,
			shouldContain: "test2",
		},
		{
			name: "Multiple failures",
			results: []*ValidationResult{
				{Passed: false, ToolUsed: "test1"},
				{Passed: true, ToolUsed: "test2"},
				{Passed: false, ToolUsed: "test3"},
			},
			expectedCount: 2,
		},
		{
			name:          "Empty results",
			results:       []*ValidationResult{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFailedChecks(tt.results)
			if len(result) != tt.expectedCount {
				t.Errorf("GetFailedChecks() returned %d items, want %d", len(result), tt.expectedCount)
			}
			if tt.shouldContain != "" {
				found := false
				for _, check := range result {
					if check == tt.shouldContain {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetFailedChecks() should contain %q, got %v", tt.shouldContain, result)
				}
			}
		})
	}
}

// TestValidationResult_Structure tests the validation result data structure
func TestValidationResult_Structure(t *testing.T) {
	result := &ValidationResult{
		Passed:   false,
		Severity: SeverityError,
		Message:  "test error",
		Details:  map[string]interface{}{"key": "value"},
		ToolUsed: "test_tool",
	}

	if result.Passed {
		t.Error("Expected Passed to be false")
	}
	if result.Severity != SeverityError {
		t.Errorf("Expected Severity %v, got %v", SeverityError, result.Severity)
	}
	if result.Message != "test error" {
		t.Errorf("Expected Message 'test error', got %v", result.Message)
	}
	if result.ToolUsed != "test_tool" {
		t.Errorf("Expected ToolUsed 'test_tool', got %v", result.ToolUsed)
	}
}

// TestSeverityConstants tests that severity constants are defined
func TestSeverityConstants(t *testing.T) {
	severities := []string{
		SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityCritical,
	}

	for _, severity := range severities {
		if severity == "" {
			t.Errorf("Severity constant should not be empty")
		}
	}
}
