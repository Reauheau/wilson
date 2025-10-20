package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wilson/agent"
	contextpkg "wilson/context"
	"wilson/core/registry"
	"wilson/llm"
)

// TestContext provides environment for agent tests
type TestContext struct {
	T           *testing.T
	WorkDir     string
	FixtureDir  string
	SnapshotDir string
	TempDir     string
	Ctx         context.Context
	Cancel      context.CancelFunc

	// Wilson components (only initialized if needed)
	LLMManager  *llm.Manager
	ContextMgr  *contextpkg.Manager
	AgentRegistry *agent.Registry

	// Flags for what to initialize
	NeedsLLM     bool
	NeedsContext bool
	NeedsAgents  bool
}

// TestRunner manages test execution with setup/cleanup
type TestRunner struct {
	ctx *TestContext
}

// NewTestRunner creates a new test runner
func NewTestRunner(t *testing.T) *TestRunner {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	// Create temp directory for this test
	tempDir, err := os.MkdirTemp("", "wilson_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Get current directory to resolve fixture paths
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Resolve fixture directory relative to project root
	fixtureDir := filepath.Join(cwd, "tests", "fixtures")
	// If running from tests/ subdirectory, adjust
	if !fileExists(fixtureDir) {
		// Try parent directory
		fixtureDir = filepath.Join(cwd, "..", "fixtures")
		if !fileExists(fixtureDir) {
			// Try from go/ root
			fixtureDir = filepath.Join(cwd, "..", "..", "go", "tests", "fixtures")
		}
	}

	testCtx := &TestContext{
		T:           t,
		WorkDir:     filepath.Join(tempDir, "work"),
		FixtureDir:  fixtureDir,
		SnapshotDir: filepath.Join(fixtureDir, "code", "snapshots"),
		TempDir:     tempDir,
		Ctx:         ctx,
		Cancel:      cancel,
	}

	// Create work directory
	if err := os.MkdirAll(testCtx.WorkDir, 0755); err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}

	return &TestRunner{ctx: testCtx}
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WithLLM enables LLM manager for this test
func (r *TestRunner) WithLLM() *TestRunner {
	r.ctx.NeedsLLM = true
	return r
}

// WithContext enables context manager for this test
func (r *TestRunner) WithContext() *TestRunner {
	r.ctx.NeedsContext = true
	return r
}

// WithAgents enables agent system for this test
func (r *TestRunner) WithAgents() *TestRunner {
	r.ctx.NeedsAgents = true
	r.ctx.NeedsLLM = true      // Agents need LLM
	r.ctx.NeedsContext = true  // Agents need Context
	return r
}

// Setup initializes required components
func (r *TestRunner) Setup() error {
	if r.ctx.NeedsLLM {
		// Initialize LLM manager (simplified for testing)
		r.ctx.LLMManager = llm.NewManager()
		// Could register test LLMs here
	}

	if r.ctx.NeedsContext {
		// Initialize context manager with temp DB
		dbPath := filepath.Join(r.ctx.TempDir, "test_memory.db")
		mgr, err := contextpkg.NewManager(dbPath, false)
		if err != nil {
			return fmt.Errorf("failed to init context manager: %w", err)
		}
		r.ctx.ContextMgr = mgr
	}

	if r.ctx.NeedsAgents {
		// Initialize agent registry
		r.ctx.AgentRegistry = agent.NewRegistry()
		// Could register agents here if needed
	}

	return nil
}

// Cleanup removes temp files and closes resources
func (r *TestRunner) Cleanup() {
	if r.ctx.Cancel != nil {
		r.ctx.Cancel()
	}

	if r.ctx.ContextMgr != nil {
		r.ctx.ContextMgr.Close()
	}

	if r.ctx.TempDir != "" {
		os.RemoveAll(r.ctx.TempDir)
	}
}

// Context returns the test context
func (r *TestRunner) Context() *TestContext {
	return r.ctx
}

// CopyFixture copies a fixture directory to the working directory
func (r *TestRunner) CopyFixture(fixtureName string) error {
	src := filepath.Join(r.ctx.FixtureDir, fixtureName)
	dst := r.ctx.WorkDir

	return copyDir(src, dst)
}

// ReadFile reads a file from the working directory
func (r *TestRunner) ReadFile(filename string) (string, error) {
	path := filepath.Join(r.ctx.WorkDir, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// WriteFile writes a file to the working directory
func (r *TestRunner) WriteFile(filename, content string) error {
	path := filepath.Join(r.ctx.WorkDir, filename)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// FileExists checks if a file exists in the working directory
func (r *TestRunner) FileExists(filename string) bool {
	path := filepath.Join(r.ctx.WorkDir, filename)
	_, err := os.Stat(path)
	return err == nil
}

// ExecuteTool executes a tool with given arguments
func (r *TestRunner) ExecuteTool(toolName string, args map[string]interface{}) (string, error) {
	tool, err := registry.GetTool(toolName)
	if err != nil {
		return "", fmt.Errorf("tool not found: %w", err)
	}

	// Adjust paths to be relative to work directory if needed
	if path, ok := args["path"].(string); ok {
		if !filepath.IsAbs(path) {
			args["path"] = filepath.Join(r.ctx.WorkDir, path)
		}
	}

	return tool.Execute(r.ctx.Ctx, args)
}

// RunInWorkDir executes a function with working directory set
func (r *TestRunner) RunInWorkDir(fn func() error) error {
	oldDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(r.ctx.WorkDir); err != nil {
		return err
	}

	return fn()
}

// Helper: copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, content, info.Mode())
	})
}
