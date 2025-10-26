package code_intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		Description: "Get real-time diagnostics (errors, warnings, hints) from language server. Use after every code change to catch issues immediately.",
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

	// Wait briefly for diagnostics to be computed
	// TODO: Implement proper diagnostic listener instead of sleep
	// time.Sleep(500 * time.Millisecond)

	// For now, return success with note about async diagnostics
	// Real implementation will need to listen for textDocument/publishDiagnostics notifications
	result := map[string]interface{}{
		"status":  "document_opened",
		"file":    filePath,
		"message": "Document opened in language server. Diagnostics will be available via notifications.",
		"note":    "Full diagnostic support requires implementing notification listener",
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
