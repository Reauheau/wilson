package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wilson/agent"
	"wilson/agent/agents"
	"wilson/agent/orchestration"
	contextpkg "wilson/context"
	"wilson/llm"

	_ "github.com/mattn/go-sqlite3"
)

// TestE2E_HTTPServerGeneration tests Wilson's ability to generate a working HTTP server
// This validates:
// 1. Code generation with stdlib-only dependencies
// 2. LSP auto-diagnostics working
// 3. Compilation succeeds
// 4. Generated code follows best practices
func TestE2E_HTTPServerGeneration(t *testing.T) {
	// Setup test environment
	tmpDir := filepath.Join("testdata", "http_server_test")

	// Clean previous run
	os.RemoveAll(tmpDir)
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	absPath, _ := filepath.Abs(tmpDir)
	t.Logf("Test directory: %s", absPath)

	// Initialize test database
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize context manager
	contextMgr, err := contextpkg.NewManager(dbPath, true)
	if err != nil {
		t.Fatalf("Failed to create context manager: %v", err)
	}
	defer contextMgr.Close()

	// Initialize LLM manager
	llmMgr := llm.NewManager()
	err = llmMgr.RegisterLLM(llm.PurposeCode, llm.Config{
		Provider: "ollama",
		Model:    "qwen2.5-coder:14b",
	})
	if err != nil {
		t.Fatalf("Failed to register LLM: %v", err)
	}

	// Create agent system
	agentRegistry := agent.NewRegistry()
	codeAgent := agents.NewCodeAgent(llmMgr, contextMgr)
	agentRegistry.Register(codeAgent)

	coordinator := orchestration.NewCoordinator(agentRegistry)
	coordinator.SetLLMManager(llmMgr)

	managerAgent := orchestration.NewManagerAgent(db)
	managerAgent.SetLLMManager(llmMgr)
	managerAgent.SetRegistry(agentRegistry)
	coordinator.SetManager(managerAgent)

	agent.SetGlobalRegistry(agentRegistry)
	orchestration.SetGlobalCoordinator(coordinator)

	// Start feedback processing
	managerAgent.StartFeedbackProcessing(context.Background())

	// Execute task: Create HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	task := &agent.Task{
		ID:          "test-http-server",
		Type:        agent.TaskTypeCode,
		Description: fmt.Sprintf("Create a simple HTTP server in %s with /status endpoint that returns JSON with uptime", absPath),
		Input: map[string]interface{}{
			"project_path": absPath,
			"file_type":    "implementation",
		},
		Status:    agent.TaskStatusNew,
		CreatedAt: time.Now(),
	}

	t.Logf("Starting HTTP server generation task...")
	result, err := codeAgent.Execute(ctx, task)

	// Validate result
	if err != nil {
		t.Fatalf("Task execution failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Task failed: %s", result.Error)
	}

	t.Logf("Task completed successfully")
	t.Logf("Tools executed: %v", result.Metadata["tools_executed"])

	// Verify generated file exists
	mainFile := filepath.Join(absPath, "main.go")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		t.Fatalf("main.go was not created")
	}

	// Read generated code
	content, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	codeStr := string(content)
	t.Logf("Generated code length: %d bytes", len(content))

	// Validate code quality
	t.Run("CodeQuality", func(t *testing.T) {
		// Check for stdlib-only imports
		if strings.Contains(codeStr, "github.com/") {
			t.Error("Code contains external dependencies (should use stdlib only)")
		}

		// Check for essential stdlib imports
		if !strings.Contains(codeStr, "net/http") {
			t.Error("Code missing net/http import")
		}
		if !strings.Contains(codeStr, "encoding/json") {
			t.Error("Code missing encoding/json import")
		}

		// Check for proper HTTP handler
		if !strings.Contains(codeStr, "http.HandleFunc") {
			t.Error("Code missing http.HandleFunc (expected stdlib routing)")
		}

		// Check for /status endpoint
		if !strings.Contains(codeStr, "/status") {
			t.Error("Code missing /status endpoint")
		}

		// Check for uptime tracking
		if !strings.Contains(codeStr, "uptime") || !strings.Contains(codeStr, "time.") {
			t.Error("Code missing uptime tracking")
		}
	})

	// Verify compilation succeeds
	t.Run("Compilation", func(t *testing.T) {
		cmd := exec.Command("go", "build", "-o", "server", ".")
		cmd.Dir = absPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Compilation failed: %v\nOutput: %s", err, string(output))
		}

		// Check binary was created
		binaryPath := filepath.Join(absPath, "server")
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			t.Error("Binary was not created")
		}
	})

	// Verify LSP diagnostics were called
	t.Run("LSPDiagnostics", func(t *testing.T) {
		toolsExecuted, ok := result.Metadata["tools_executed"].([]string)
		if !ok {
			t.Fatal("tools_executed metadata missing")
		}

		hasGetDiagnostics := false
		for _, tool := range toolsExecuted {
			if tool == "get_diagnostics" {
				hasGetDiagnostics = true
				break
			}
		}

		if !hasGetDiagnostics {
			t.Error("get_diagnostics was not called (LSP auto-diagnostics not working)")
		}
	})

	t.Logf("✅ HTTP server generation test passed")
}
