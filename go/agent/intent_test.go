package agent

import (
	"testing"
)

// TestClassifyIntent_BasicCases tests the core classification logic
func TestClassifyIntent_BasicCases(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected Intent
	}{
		// Simple chat
		{
			name:     "Greeting",
			message:  "hello",
			expected: IntentChat,
		},
		{
			name:     "Question",
			message:  "how are you?",
			expected: IntentChat,
		},
		// Tool operations
		{
			name:     "List files",
			message:  "list files in the directory",
			expected: IntentTool,
		},
		{
			name:     "Read file",
			message:  "read the main.go file",
			expected: IntentTool,
		},
		// Tool operations with clear keywords
		{
			name:     "Search files",
			message:  "search for functions in the code",
			expected: IntentTool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyIntent(tt.message)
			if result != tt.expected {
				t.Errorf("ClassifyIntent(%q) = %v, want %v", tt.message, result, tt.expected)
			}
		})
	}
}

// TestIntent_String tests the string representation
func TestIntent_String(t *testing.T) {
	tests := []struct {
		intent   Intent
		expected string
	}{
		{IntentChat, "chat"},
		{IntentTool, "tool"},
		{IntentDelegate, "delegate"},
		{IntentCode, "code"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.intent.String()
			if result != tt.expected {
				t.Errorf("Intent.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}
