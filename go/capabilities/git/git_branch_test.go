package git

import (
	"testing"
)

func TestParseGitBranch(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int // number of branches
	}{
		{
			name: "Current and other branches",
			output: `* master                abc1234 Latest commit
  feature-branch        def5678 Work in progress
  bugfix/issue-123      ghi9012 Fix critical bug`,
			expected: 3,
		},
		{
			name: "With remote branches",
			output: `* master                    abc1234 Latest commit
  remotes/origin/master     abc1234 Latest commit
  remotes/origin/develop    def5678 Development branch`,
			expected: 3,
		},
		{
			name:     "Single branch",
			output:   `* master  abc1234 Initial commit`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitBranch(tt.output)
			if len(result) != tt.expected {
				t.Errorf("Expected %d branches, got %d", tt.expected, len(result))
			}

			// Check structure
			for i, branch := range result {
				if _, ok := branch["name"]; !ok {
					t.Errorf("Branch %d missing 'name' field", i)
				}
				if _, ok := branch["commit"]; !ok {
					t.Errorf("Branch %d missing 'commit' field", i)
				}
				if _, ok := branch["current"]; !ok {
					t.Errorf("Branch %d missing 'current' field", i)
				}
				if _, ok := branch["remote"]; !ok {
					t.Errorf("Branch %d missing 'remote' field", i)
				}
			}

			// First branch should be current (marked with *)
			if len(result) > 0 {
				if current, ok := result[0]["current"].(bool); !ok || !current {
					t.Error("First branch should be marked as current")
				}
			}
		})
	}
}
