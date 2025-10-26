package chatbot_test

import (
	"testing"
	"wilson/agent/chat"
)

func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected chat.Intent
	}{
		// Chat intents
		{
			name:     "Greeting",
			input:    "Hello, how are you?",
			expected: chat.IntentChat,
		},
		{
			name:     "Weather question",
			input:    "What's the weather like?",
			expected: chat.IntentChat,
		},
		{
			name:     "Joke request",
			input:    "Tell me a joke",
			expected: chat.IntentChat,
		},
		{
			name:     "Personal statement",
			input:    "I'm feeling good today",
			expected: chat.IntentChat,
		},
		{
			name:     "Explanation request",
			input:    "Can you explain what LLMs are?",
			expected: chat.IntentChat,
		},

		// Tool intents
		{
			name:     "List files",
			input:    "list files in the current directory",
			expected: chat.IntentTool,
		},
		{
			name:     "Show go files",
			input:    "show me all go files",
			expected: chat.IntentTool,
		},
		{
			name:     "Read file",
			input:    "read main.go",
			expected: chat.IntentTool,
		},
		{
			name:     "Find test files",
			input:    "find all test files",
			expected: chat.IntentTool,
		},
		{
			name:     "Search function",
			input:    "search for main function",
			expected: chat.IntentTool,
		},
		{
			name:     "Delete file",
			input:    "delete old.txt",
			expected: chat.IntentTool,
		},
		{
			name:     "Run tests",
			input:    "run the tests",
			expected: chat.IntentTool,
		},
		{
			name:     "Create directory",
			input:    "create a dir called testdir",
			expected: chat.IntentTool,
		},
		{
			name:     "Make directory full path",
			input:    "could you create a dir in /Users/foo called bar?",
			expected: chat.IntentTool,
		},
		{
			name:     "Create folder",
			input:    "create folder myproject",
			expected: chat.IntentTool,
		},
		{
			name:     "Make directory",
			input:    "make directory output",
			expected: chat.IntentTool,
		},

		// Delegation intents
		{
			name:     "Build CLI tool",
			input:    "build a CLI tool for task management",
			expected: chat.IntentDelegate,
		},
		{
			name:     "Implement feature",
			input:    "implement user authentication",
			expected: chat.IntentDelegate,
		},
		{
			name:     "Refactor system",
			input:    "refactor the agent system",
			expected: chat.IntentDelegate,
		},
		{
			name:     "Fix bug",
			input:    "fix the bug in the parser",
			expected: chat.IntentDelegate,
		},
		{
			name:     "Create web scraper",
			input:    "create a web scraper",
			expected: chat.IntentDelegate,
		},
		{
			name:     "Add feature",
			input:    "add feature for exporting data",
			expected: chat.IntentDelegate,
		},
	}

	passed := 0
	failed := 0

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.ClassifyIntent(tt.input)
			if result != tt.expected {
				t.Errorf("Input: %q\n  Expected: %s\n  Got: %s",
					tt.input, tt.expected, result)
				failed++
			} else {
				passed++
			}
		})
	}

	// Summary
	t.Logf("Passed: %d/%d", passed, len(tests))
	t.Logf("Failed: %d/%d", failed, len(tests))

	if failed > 0 {
		t.Fatalf("Intent classification failed %d/%d tests", failed, len(tests))
	}
}

func TestIntentString(t *testing.T) {
	tests := []struct {
		intent   chat.Intent
		expected string
	}{
		{chat.IntentChat, "chat"},
		{chat.IntentTool, "tool"},
		{chat.IntentDelegate, "delegate"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.intent.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Note: Internal heuristic tests removed - covered by TestClassifyIntent
