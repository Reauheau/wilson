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
	code_intelligence "wilson/capabilities/code_intelligence" // Code generation
	contextpkg "wilson/context"
	"wilson/llm"
	"wilson/lsp"

	_ "github.com/mattn/go-sqlite3"
	_ "wilson/capabilities/code_intelligence/analysis" // Code intelligence tools
	_ "wilson/capabilities/code_intelligence/ast"      // AST tools
	_ "wilson/capabilities/code_intelligence/build"    // Build tools
	_ "wilson/capabilities/code_intelligence/quality"  // Quality tools
	_ "wilson/capabilities/context"                    // Context tools
	_ "wilson/capabilities/filesystem"                 // Filesystem tools
)

// TestE2E_FileProcessorGeneration tests Wilson's ability to generate a file processor with error handling
// This validates:
// 1. File I/O operations with proper error handling
// 2. Stdlib-only dependencies
// 3. Clean resource management (defer)
// 4. Compilation succeeds
func TestE2E_FileProcessorGeneration(t *testing.T) {
	// Setup test environment
	tmpDir := filepath.Join("testdata", "file_processor_test")

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

	// Initialize LSP manager for code intelligence
	lspManager := lsp.NewManager()
	code_intelligence.SetLSPManager(lspManager)
	defer lspManager.StopAll()

	// Initialize LLM manager
	llmMgr := llm.NewManager()
	code_intelligence.SetLLMManager(llmMgr) // Required for generate_code tool

	err = llmMgr.RegisterLLM(llm.PurposeCode, llm.Config{
		Provider: "ollama",
		Model:    "qwen2.5-coder:14b",
	})
	if err != nil {
		t.Fatalf("Failed to register LLM: %v", err)
	}

	// Also register chat LLM (required for tool execution)
	err = llmMgr.RegisterLLM(llm.PurposeChat, llm.Config{
		Provider: "ollama",
		Model:    "qwen2.5-coder:14b",
	})
	if err != nil {
		t.Fatalf("Failed to register chat LLM: %v", err)
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

	managerAgent.StartFeedbackProcessing(context.Background())

	// Execute task: Create file word counter
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	task := &agent.Task{
		ID:          "test-file-processor",
		Type:        agent.TaskTypeCode,
		Description: fmt.Sprintf("Create a file word counter in %s that counts words in a text file with proper error handling", absPath),
		Input: map[string]interface{}{
			"project_path": absPath,
			"file_type":    "implementation",
		},
		Status:    agent.TaskPending,
		CreatedAt: time.Now(),
	}

	t.Logf("Starting file processor generation task...")
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

	// Find the generated Go file (might be main.go or main_test.go)
	var goFile string
	files, _ := filepath.Glob(filepath.Join(absPath, "*.go"))
	if len(files) == 0 {
		t.Fatalf("No Go files were created")
	}
	goFile = files[0]

	// Read generated code
	content, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", goFile, err)
	}

	codeStr := string(content)
	t.Logf("Generated code length: %d bytes", len(content))

	// Validate code quality
	t.Run("CodeQuality", func(t *testing.T) {
		// Check for stdlib-only imports
		if strings.Contains(codeStr, "github.com/") {
			t.Error("Code contains external dependencies (should use stdlib only)")
		}

		// Check for essential stdlib imports for file I/O
		if !strings.Contains(codeStr, "os") {
			t.Error("Code missing os package import")
		}

		// Check for proper error handling
		if !strings.Contains(codeStr, "if err") {
			t.Error("Code missing error handling")
		}

		// Check for resource cleanup (defer)
		if !strings.Contains(codeStr, "defer") && strings.Contains(codeStr, "Close()") {
			t.Error("Code should use defer for resource cleanup")
		}

		// Check for file operations
		if !strings.Contains(codeStr, "Open") && !strings.Contains(codeStr, "ReadFile") {
			t.Error("Code missing file read operations")
		}

		// Check for word counting logic
		if !strings.Contains(codeStr, "word") || !strings.Contains(codeStr, "count") {
			t.Error("Code missing word counting logic")
		}
	})

	// Verify compilation succeeds
	t.Run("Compilation", func(t *testing.T) {
		cmd := exec.Command("go", "build", "-o", "processor", ".")
		cmd.Dir = absPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Compilation failed: %v\nOutput: %s", err, string(output))
			return
		}

		// Check binary was created
		binaryPath := filepath.Join(absPath, "processor")
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

	t.Logf("✅ File processor generation test passed")
}
