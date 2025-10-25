package git

import (
	"testing"
)

func TestParseGitStatus(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected map[string]interface{}
	}{
		{
			name: "Clean working tree",
			output: `## master
`,
			expected: map[string]interface{}{
				"branch":    "master",
				"ahead":     0,
				"behind":    0,
				"modified":  []string{},
				"staged":    []string{},
				"untracked": []string{},
				"deleted":   []string{},
				"renamed":   []string{},
				"clean":     true,
			},
		},
		{
			name: "Modified files",
			output: `## master
 M go/agent/code_agent.go
 M go/agent/manager_agent.go
`,
			expected: map[string]interface{}{
				"branch":    "master",
				"modified":  []string{"go/agent/code_agent.go", "go/agent/manager_agent.go"},
				"staged":    []string{},
				"untracked": []string{},
				"clean":     false,
			},
		},
		{
			name: "Staged files",
			output: `## master
M  README.md
A  new_file.go
`,
			expected: map[string]interface{}{
				"branch": "master",
				"staged": []string{"README.md", "new_file.go"},
				"clean":  false,
			},
		},
		{
			name: "Untracked files",
			output: `## master
?? test.txt
?? debug.log
`,
			expected: map[string]interface{}{
				"branch":    "master",
				"untracked": []string{"test.txt", "debug.log"},
				"clean":     false,
			},
		},
		{
			name: "Mixed changes",
			output: `## feature-branch
M  staged.go
 M modified.go
?? untracked.txt
`,
			expected: map[string]interface{}{
				"branch":    "feature-branch",
				"staged":    []string{"staged.go"},
				"modified":  []string{"modified.go"},
				"untracked": []string{"untracked.txt"},
				"clean":     false,
			},
		},
		{
			name: "Branch ahead/behind",
			output: `## master...origin/master [ahead 2, behind 1]
`,
			expected: map[string]interface{}{
				"branch": "master",
				"ahead":  2,
				"behind": 1,
				"clean":  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatus(tt.output)

			// Check expected keys
			for key, expectedVal := range tt.expected {
				actualVal, ok := result[key]
				if !ok {
					t.Errorf("Missing key: %s", key)
					continue
				}

				// Type-specific comparisons
				switch expected := expectedVal.(type) {
				case string:
					if actual, ok := actualVal.(string); !ok || actual != expected {
						t.Errorf("Key %s: expected %v, got %v", key, expected, actual)
					}
				case int:
					if actual, ok := actualVal.(int); !ok || actual != expected {
						t.Errorf("Key %s: expected %v, got %v", key, expected, actual)
					}
				case bool:
					if actual, ok := actualVal.(bool); !ok || actual != expected {
						t.Errorf("Key %s: expected %v, got %v", key, expected, actual)
					}
				case []string:
					actual, ok := actualVal.([]string)
					if !ok {
						t.Errorf("Key %s: expected []string, got %T", key, actualVal)
						continue
					}
					if len(actual) != len(expected) {
						t.Errorf("Key %s: expected length %d, got %d", key, len(expected), len(actual))
						continue
					}
					for i, exp := range expected {
						if i >= len(actual) || actual[i] != exp {
							t.Errorf("Key %s[%d]: expected %v, got %v", key, i, exp, actual[i])
						}
					}
				}
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "No duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "With duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
			}
		})
	}
}
