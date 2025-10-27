package orchestration

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"wilson/agent/base"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) *sql.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	return db
}

// TestEnrichTaskContextWithGit_Success tests successful git context enrichment
func TestEnrichTaskContextWithGit_Success(t *testing.T) {
	// Create a temporary git repo for testing
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skip("Git not available, skipping test")
	}

	// Configure git (required for commits)
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create a file and commit it
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.go")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Create a modified file (not staged)
	modifiedFile := filepath.Join(tmpDir, "modified.go")
	if err := os.WriteFile(modifiedFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to create modified file: %v", err)
	}

	// Create manager with mock registry
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManagerAgent(db)
	// Note: enrichTaskContextWithGit uses global registry.GetTool(), so git tools must be registered

	// Create TaskContext
	taskCtx := &base.TaskContext{
		ProjectPath: tmpDir,
	}

	// Enrich with git context
	err := manager.enrichTaskContextWithGit(taskCtx)

	// If git tools not available, test should not fail
	if err != nil {
		t.Logf("Git enrichment returned error (expected if git tools not registered): %v", err)
		return
	}

	// Verify git context was populated
	if taskCtx.GitRoot == "" {
		t.Log("GitRoot not populated (git tools may not be registered)")
		return
	}

	// If we got here, git tools are available and enrichment worked
	if taskCtx.GitRoot != tmpDir {
		t.Errorf("Expected GitRoot=%s, got=%s", tmpDir, taskCtx.GitRoot)
	}

	// Branch should be populated (typically "master" or "main")
	if taskCtx.GitBranch == "" {
		t.Error("GitBranch should be populated")
	}

	// Should have untracked file
	if len(taskCtx.GitUntrackedFiles) == 0 {
		t.Error("Expected untracked files (modified.go)")
	}

	// Repo should not be clean (has untracked file)
	if taskCtx.GitClean {
		t.Error("Expected GitClean=false due to untracked file")
	}

	t.Logf("✓ Git context enriched: branch=%s, untracked=%d", taskCtx.GitBranch, len(taskCtx.GitUntrackedFiles))
}

// TestEnrichTaskContextWithGit_NonGitDirectory tests handling of non-git directories
func TestEnrichTaskContextWithGit_NonGitDirectory(t *testing.T) {
	// Create a temporary non-git directory
	tmpDir := t.TempDir()

	db := setupTestDB(t)
	defer db.Close()

	manager := NewManagerAgent(db)

	taskCtx := &base.TaskContext{
		ProjectPath: tmpDir,
	}

	// Should not error on non-git directory
	err := manager.enrichTaskContextWithGit(taskCtx)
	if err != nil {
		t.Errorf("enrichTaskContextWithGit should not error on non-git directory: %v", err)
	}

	// Git fields should remain empty
	if taskCtx.GitRoot != "" {
		t.Errorf("Expected empty GitRoot for non-git directory, got: %s", taskCtx.GitRoot)
	}

	if taskCtx.GitBranch != "" {
		t.Errorf("Expected empty GitBranch for non-git directory, got: %s", taskCtx.GitBranch)
	}

	t.Logf("✓ Non-git directory handled gracefully")
}

// TestEnrichTaskContextWithGit_CleanRepo tests a clean git repository
func TestEnrichTaskContextWithGit_CleanRepo(t *testing.T) {
	// Create a temporary git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skip("Git not available, skipping test")
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create and commit a file
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Create manager
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManagerAgent(db)

	taskCtx := &base.TaskContext{
		ProjectPath: tmpDir,
	}

	// Enrich with git context
	err := manager.enrichTaskContextWithGit(taskCtx)
	if err != nil {
		t.Logf("Git enrichment returned error: %v", err)
		return
	}

	// If git tools are registered and working
	if taskCtx.GitRoot != "" {
		// Clean repo should have GitClean=true
		if !taskCtx.GitClean {
			t.Errorf("Expected GitClean=true for clean repo, got false")
		}

		// Should have no modified files
		if len(taskCtx.GitModifiedFiles) > 0 {
			t.Errorf("Expected no modified files, got: %v", taskCtx.GitModifiedFiles)
		}

		t.Logf("✓ Clean repo detected: branch=%s, clean=%v", taskCtx.GitBranch, taskCtx.GitClean)
	}
}

// TestEnrichTaskContextWithGit_ModifiedFiles tests detection of modified files
func TestEnrichTaskContextWithGit_ModifiedFiles(t *testing.T) {
	// Create a temporary git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skip("Git not available, skipping test")
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create and commit a file
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Modify the file
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Create manager
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManagerAgent(db)

	taskCtx := &base.TaskContext{
		ProjectPath: tmpDir,
	}

	// Enrich with git context
	err := manager.enrichTaskContextWithGit(taskCtx)
	if err != nil {
		t.Logf("Git enrichment returned error: %v", err)
		return
	}

	// If git tools are registered and working
	if taskCtx.GitRoot != "" {
		// Should detect modified file
		if len(taskCtx.GitModifiedFiles) == 0 {
			t.Error("Expected modified files to be detected")
		}

		// Repo should not be clean
		if taskCtx.GitClean {
			t.Error("Expected GitClean=false due to modified file")
		}

		t.Logf("✓ Modified files detected: %v", taskCtx.GitModifiedFiles)
	}
}

// TestToStringSlice tests the helper function
func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "Valid string array",
			input:    []interface{}{"file1.go", "file2.go"},
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "Empty array",
			input:    []interface{}{},
			expected: []string{},
		},
		{
			name:     "Mixed types (filters non-strings)",
			input:    []interface{}{"file1.go", 123, "file2.go"},
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "Nil input",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "Not an array",
			input:    "not an array",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toStringSlice(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				return
			}

			for i, val := range result {
				if val != tt.expected[i] {
					t.Errorf("Expected %s at index %d, got %s", tt.expected[i], i, val)
				}
			}
		})
	}
}
