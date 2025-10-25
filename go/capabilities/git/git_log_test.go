package git

import (
	"testing"
)

func TestParseGitLog(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int // number of commits
	}{
		{
			name: "Three commits",
			output: `abc1234|John Doe|2 days ago|Fix compilation error
def5678|Jane Smith|1 week ago|Add new feature
ghi9012|Bob Johnson|2 weeks ago|Initial commit`,
			expected: 3,
		},
		{
			name:     "Single commit",
			output:   `abc1234|John Doe|2 days ago|Fix bug`,
			expected: 1,
		},
		{
			name:     "Empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "Commit with pipe in message",
			output:   `abc1234|John Doe|2 days ago|Fix bug | add tests`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitLog(tt.output)
			if len(result) != tt.expected {
				t.Errorf("Expected %d commits, got %d", tt.expected, len(result))
			}

			// Validate structure of first commit if present
			if len(result) > 0 {
				commit := result[0]
				if _, ok := commit["hash"]; !ok {
					t.Error("Commit missing 'hash' field")
				}
				if _, ok := commit["author"]; !ok {
					t.Error("Commit missing 'author' field")
				}
				if _, ok := commit["date"]; !ok {
					t.Error("Commit missing 'date' field")
				}
				if _, ok := commit["message"]; !ok {
					t.Error("Commit missing 'message' field")
				}
			}
		})
	}
}
