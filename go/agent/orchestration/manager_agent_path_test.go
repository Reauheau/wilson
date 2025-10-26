package orchestration

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractProjectPath tests path extraction from user requests
func TestExtractProjectPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		request  string
		expected string
	}{
		{
			name:     "go to with tilde",
			request:  "could you go to ~/wilsontestdir and create a go file",
			expected: filepath.Join(home, "wilsontestdir"),
		},
		{
			name:     "in with tilde",
			request:  "create a file in ~/myproject",
			expected: filepath.Join(home, "myproject"),
		},
		{
			name:     "at with tilde",
			request:  "at ~/workspace create files",
			expected: filepath.Join(home, "workspace"),
		},
		{
			name:     "absolute path",
			request:  "create files in /tmp/test",
			expected: "/tmp/test",
		},
		{
			name:     "relative path",
			request:  "create files in ../parent",
			expected: "../parent",
		},
		{
			name:     "no path specified",
			request:  "create a calculator",
			expected: ".",
		},
		{
			name:     "to without path indicator",
			request:  "move to implementation",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProjectPath(tt.request)
			if result != tt.expected {
				t.Errorf("extractProjectPath(%q) = %q, want %q", tt.request, result, tt.expected)
			}
		})
	}
}
