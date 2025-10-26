package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLSPBasicWorkflow tests the basic LSP workflow
func TestLSPBasicWorkflow(t *testing.T) {
	// Create a temporary Go file with an error
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Write a file with a deliberate error (undefined variable)
	testCode := `package main

import "fmt"

func main() {
	fmt.Println(undefinedVar)  // This should trigger a diagnostic
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Initialize go.mod in temp directory
	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create LSP manager
	manager := NewManager()

	// Get gopls client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := manager.GetClient(ctx, "go")
	if err != nil {
		t.Fatalf("Failed to get gopls client: %v", err)
	}
	defer manager.StopAll()

	// Initialize with temp directory as root
	if err := client.Initialize(ctx, "file://"+tmpDir); err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	t.Logf("✓ gopls client started and initialized")

	// Open the document
	fileURI := "file://" + testFile
	if err := client.OpenDocument(ctx, fileURI, "go", testCode); err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	t.Logf("✓ Document opened: %s", testFile)

	// Wait a moment for gopls to process
	time.Sleep(2 * time.Second)

	t.Logf("✓ LSP basic workflow completed successfully")
}

// TestLSPGoToDefinition tests go-to-definition functionality
func TestLSPGoToDefinition(t *testing.T) {
	// Create a temporary Go file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	testCode := `package main

import "fmt"

func myFunc() {
	fmt.Println("hello")
}

func main() {
	myFunc()  // Go to definition of myFunc
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Initialize go.mod
	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create LSP manager and client
	manager := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := manager.GetClient(ctx, "go")
	if err != nil {
		t.Fatalf("Failed to get gopls client: %v", err)
	}
	defer manager.StopAll()

	if err := client.Initialize(ctx, "file://"+tmpDir); err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Open document
	fileURI := "file://" + testFile
	if err := client.OpenDocument(ctx, fileURI, "go", testCode); err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	// Wait for gopls to index
	time.Sleep(2 * time.Second)

	// Request go-to-definition for myFunc() call on line 9
	// Position is 0-based: line 9 (0-indexed = 8), character 1 (on 'myFunc')
	locations, err := client.GoToDefinition(ctx, fileURI, 9, 1)
	if err != nil {
		t.Fatalf("GoToDefinition failed: %v", err)
	}

	if len(locations) == 0 {
		t.Fatalf("Expected at least one definition location, got none")
	}

	t.Logf("✓ Found definition at: %s (line %d)", locations[0].URI, locations[0].Range.Start.Line)

	// Verify it points to the function definition (line 4)
	if locations[0].Range.Start.Line != 4 {
		t.Errorf("Expected definition on line 4, got line %d", locations[0].Range.Start.Line)
	}
}

// TestLSPCacheBasics tests the response cache
func TestLSPCacheBasics(t *testing.T) {
	cache := NewResponseCache()

	// Test Set/Get
	cache.Set("key1", "value1")
	val, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit, got miss")
	}
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val)
	}

	// Test Invalidate
	cache.Invalidate("key1")
	_, ok = cache.Get("key1")
	if ok {
		t.Error("Expected cache miss after invalidation, got hit")
	}

	// Test Clear
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")
	cache.Clear()
	_, ok = cache.Get("key2")
	if ok {
		t.Error("Expected cache miss after clear, got hit")
	}

	t.Logf("✓ Cache operations working correctly")
}

// TestLSPDiagnostics tests diagnostic notifications
func TestLSPDiagnostics(t *testing.T) {
	// Create a temporary Go file with errors
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Write a file with intentional errors
	testCode := `package main

import "fmt"

func main() {
	fmt.Println(undefinedVar)  // undefined variable error
	unusedVar := 42            // unused variable warning
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Initialize go.mod
	goModContent := "module test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create LSP manager and client
	manager := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := manager.GetClient(ctx, "go")
	if err != nil {
		t.Fatalf("Failed to get gopls client: %v", err)
	}
	defer manager.StopAll()

	if err := client.Initialize(ctx, "file://"+tmpDir); err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Open document
	fileURI := "file://" + testFile
	if err := client.OpenDocument(ctx, fileURI, "go", testCode); err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	// Wait for diagnostics to be processed
	time.Sleep(2 * time.Second)

	// Get diagnostics
	diagnostics := client.GetDiagnostics(fileURI)

	if len(diagnostics) == 0 {
		t.Fatal("Expected diagnostics for file with errors, got none")
	}

	t.Logf("✓ Received %d diagnostic(s)", len(diagnostics))

	// Verify we have at least one error (undefinedVar)
	hasError := false
	for _, diag := range diagnostics {
		t.Logf("  - [%s] Line %d: %s", severityToString(diag.Severity), diag.Range.Start.Line+1, diag.Message)
		if diag.Severity == SeverityError {
			hasError = true
		}
	}

	if !hasError {
		t.Error("Expected at least one error diagnostic")
	}

	t.Logf("✓ Diagnostics working correctly")
}

func severityToString(sev DiagnosticSeverity) string {
	switch sev {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityInformation:
		return "INFO"
	case SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}
