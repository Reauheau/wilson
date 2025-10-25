package git

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindGitRoot tests finding git repository root
func TestFindGitRoot(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir", "nested")

	// Initialize as a real git repo (git init)
	// Note: Requires git to be installed
	// Skip test if git not available
	if _, err := RunGitCommandInDir(tmpDir, "init"); err != nil {
		t.Skip("Git not available, skipping test")
	}

	// Create nested subdirectories
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirs: %v", err)
	}

	t.Run("Find from root", func(t *testing.T) {
		root, err := FindGitRoot(tmpDir)
		if err != nil {
			t.Errorf("Expected to find git root, got error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("Expected root %s, got %s", tmpDir, root)
		}
	})

	t.Run("Find from nested directory", func(t *testing.T) {
		root, err := FindGitRoot(subDir)
		if err != nil {
			t.Errorf("Expected to find git root from nested dir, got error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("Expected root %s, got %s", tmpDir, root)
		}
	})

	t.Run("Not a git repo", func(t *testing.T) {
		// Create a separate temp dir that's NOT inside the git repo
		notGitDir := t.TempDir()
		notGitDir, err := filepath.EvalSymlinks(notGitDir)
		if err != nil {
			t.Fatalf("Failed to resolve symlinks: %v", err)
		}

		_, err = FindGitRoot(notGitDir)
		if err == nil {
			t.Error("Expected error for non-git directory, got nil")
		}
	})
}

// TestIsGitRepo tests checking if directory is in a git repo
func TestIsGitRepo(t *testing.T) {
	// Create a temporary directory and init git
	tmpDir := t.TempDir()
	if _, err := RunGitCommandInDir(tmpDir, "init"); err != nil {
		t.Skip("Git not available, skipping test")
	}

	t.Run("Is git repo", func(t *testing.T) {
		if !IsGitRepo(tmpDir) {
			t.Error("Expected IsGitRepo to return true")
		}
	})

	t.Run("Not git repo", func(t *testing.T) {
		// Create a separate temp dir that's NOT inside the git repo
		notGitDir := t.TempDir()
		if IsGitRepo(notGitDir) {
			t.Error("Expected IsGitRepo to return false")
		}
	})
}
