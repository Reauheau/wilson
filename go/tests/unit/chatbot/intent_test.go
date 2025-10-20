package chatbot_test

import (
	"testing"
	"wilson/agent"
)

func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected agent.Intent
	}{
		// Chat intents
		{
			name:     "Greeting",
			input:    "Hello, how are you?",
			expected: agent.IntentChat,
		},
		{
			name:     "Weather question",
			input:    "What's the weather like?",
			expected: agent.IntentChat,
		},
		{
			name:     "Joke request",
			input:    "Tell me a joke",
			expected: agent.IntentChat,
		},
		{
			name:     "Personal statement",
			input:    "I'm feeling good today",
			expected: agent.IntentChat,
		},
		{
			name:     "Explanation request",
			input:    "Can you explain what LLMs are?",
			expected: agent.IntentChat,
		},

		// Tool intents
		{
			name:     "List files",
			input:    "list files in the current directory",
			expected: agent.IntentTool,
		},
		{
			name:     "Show go files",
			input:    "show me all go files",
			expected: agent.IntentTool,
		},
		{
			name:     "Read file",
			input:    "read main.go",
			expected: agent.IntentTool,
		},
		{
			name:     "Find test files",
			input:    "find all test files",
			expected: agent.IntentTool,
		},
		{
			name:     "Search function",
			input:    "search for main function",
			expected: agent.IntentTool,
		},
		{
			name:     "Delete file",
			input:    "delete old.txt",
			expected: agent.IntentTool,
		},
		{
			name:     "Run tests",
			input:    "run the tests",
			expected: agent.IntentTool,
		},
		{
			name:     "Create directory",
			input:    "create a dir called testdir",
			expected: agent.IntentTool,
		},
		{
			name:     "Make directory full path",
			input:    "could you create a dir in /Users/foo called bar?",
			expected: agent.IntentTool,
		},
		{
			name:     "Create folder",
			input:    "create folder myproject",
			expected: agent.IntentTool,
		},
		{
			name:     "Make directory",
			input:    "make directory output",
			expected: agent.IntentTool,
		},

		// Delegation intents
		{
			name:     "Build CLI tool",
			input:    "build a CLI tool for task management",
			expected: agent.IntentDelegate,
		},
		{
			name:     "Implement feature",
			input:    "implement user authentication",
			expected: agent.IntentDelegate,
		},
		{
			name:     "Refactor system",
			input:    "refactor the agent system",
			expected: agent.IntentDelegate,
		},
		{
			name:     "Fix bug",
			input:    "fix the bug in the parser",
			expected: agent.IntentDelegate,
		},
		{
			name:     "Create web scraper",
			input:    "create a web scraper",
			expected: agent.IntentDelegate,
		},
		{
			name:     "Add feature",
			input:    "add feature for exporting data",
			expected: agent.IntentDelegate,
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
		intent   agent.Intent
		expected string
	}{
		{agent.IntentChat, "chat"},
		{agent.IntentTool, "tool"},
		{agent.IntentDelegate, "delegate"},
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
