package code_intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/lsp"
)

// LSPDiagnosticsTool gets compiler errors, warnings, and hints from LSP
// This is CRITICAL - it prevents Wilson from creating broken code
type LSPDiagnosticsTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPDiagnosticsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get_diagnostics",
		Description: "Get real-time diagnostics (errors, warnings, hints) from language server. Supports Go, Python, JavaScript, TypeScript, Rust. Use after every code change to catch issues immediately.",
		Category:    CategoryAI, // Code intelligence is AI-powered analysis
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Path to file to check",
				Example:     "main.go",
			},
		},
		Examples: []string{
			`{"tool": "get_diagnostics", "arguments": {"path": "main.go"}}`,
			`{"tool": "get_diagnostics", "arguments": {"path": "src/handler.go"}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPDiagnosticsTool) Validate(args map[string]interface{}) error {
	if _, ok := args["path"]; !ok {
		return fmt.Errorf("path is required")
	}
	return nil
}

// Execute gets diagnostics for a file
func (t *LSPDiagnosticsTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if packageLSPManager == nil {
		return "", fmt.Errorf("LSP manager not initialized")
	}

	filePath := args["path"].(string)

	// Make path absolute
	if !filepath.IsAbs(filePath) {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		filePath = absPath
	}

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// Get LSP client for this file
	client, err := packageLSPManager.GetClientForFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get LSP client: %w", err)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Convert to file:// URI
	fileURI := "file://" + filePath

	// Detect language ID
	languageID := getLanguageID(filePath)

	// Open document (or update if already open)
	if err := client.OpenDocument(ctx, fileURI, languageID, string(content)); err != nil {
		return "", fmt.Errorf("failed to open document: %w", err)
	}

	// ✅ ROBUST FIX (Phase 1C): Wait for diagnostics with adaptive polling
	// Instead of fixed 500ms delay, poll with exponential backoff up to 2s
	// This is faster when gopls is quick (50-100ms) and more reliable when slow
	diagnostics := waitForDiagnosticsWithTimeout(client, fileURI, 2*time.Second)

	// Build result
	result := map[string]interface{}{
		"file":          filePath,
		"uri":           fileURI,
		"has_errors":    false,
		"error_count":   0,
		"warning_count": 0,
		"hint_count":    0,
		"diagnostics":   []map[string]interface{}{},
	}

	// Process diagnostics
	diagList := []map[string]interface{}{}
	errorCount := 0
	warningCount := 0
	hintCount := 0

	for _, diag := range diagnostics {
		severity := "unknown"
		switch diag.Severity {
		case lsp.SeverityError:
			severity = "error"
			errorCount++
		case lsp.SeverityWarning:
			severity = "warning"
			warningCount++
		case lsp.SeverityHint:
			severity = "hint"
			hintCount++
		case lsp.SeverityInformation:
			severity = "info"
		}

		diagList = append(diagList, map[string]interface{}{
			"severity":  severity,
			"line":      diag.Range.Start.Line + 1, // Convert to 1-based for user display
			"character": diag.Range.Start.Character,
			"message":   diag.Message,
			"source":    diag.Source,
			"code":      diag.Code,
		})
	}

	result["diagnostics"] = diagList
	result["error_count"] = errorCount
	result["warning_count"] = warningCount
	result["hint_count"] = hintCount
	result["has_errors"] = errorCount > 0

	// Add summary
	if len(diagnostics) == 0 {
		result["summary"] = "No issues found"
	} else {
		result["summary"] = fmt.Sprintf("Found %d error(s), %d warning(s), %d hint(s)",
			errorCount, warningCount, hintCount)
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// getLanguageID returns the LSP language ID for a file
func getLanguageID(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".rs":
		return "rust"
	default:
		return strings.TrimPrefix(ext, ".")
	}
}

// waitForDiagnosticsWithTimeout polls for diagnostics with exponential backoff
// This is more robust than a fixed delay because:
// - Faster when gopls is quick (50-100ms typical)
// - More reliable when gopls is slow (up to timeout)
// - Adapts to system load automatically
func waitForDiagnosticsWithTimeout(client *lsp.Client, uri string, timeout time.Duration) []lsp.Diagnostic {
	deadline := time.Now().Add(timeout)
	backoff := 50 * time.Millisecond
	maxBackoff := 500 * time.Millisecond
	attempts := 0

	for time.Now().Before(deadline) {
		diagnostics := client.GetDiagnostics(uri)
		attempts++

		// Strategy: Wait at least 2 poll attempts before accepting results
		// This ensures gopls had reasonable time to process
		// Empty diagnostics after 2 attempts = "no errors" (valid result)
		if attempts >= 2 && diagnostics != nil {
			return diagnostics
		}

		// Not ready yet, wait with exponential backoff
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	// Timeout reached - return whatever we have
	// This is graceful degradation, not an error
	// Worst case: we fall back to compilation for verification
	return client.GetDiagnostics(uri)
}

// Package-level LSP manager (shared with other LSP tools)
var packageLSPManager *lsp.Manager

// SetLSPManager sets the LSP manager for code intelligence tools
func SetLSPManager(manager *lsp.Manager) {
	packageLSPManager = manager
}

func init() {
	// Register tool - LSP manager will be set via SetLSPManager
	tool := &LSPDiagnosticsTool{}
	registry.Register(tool)
}
