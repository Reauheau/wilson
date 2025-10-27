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

// LSPFindImplementationsTool finds all implementations of an interface/type using LSP
type LSPFindImplementationsTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPFindImplementationsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "find_implementations",
		Description: "Find all types that implement an interface or abstract type. Supports Go, Python, JavaScript, TypeScript, Rust. Critical for understanding polymorphism and architectural impact.",
		Category:    CategoryAI,
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File containing the interface/type reference",
				Example:     "agent/base/base.go",
			},
			{
				Name:        "line",
				Type:        "number",
				Required:    true,
				Description: "Line number (1-based) where interface/type appears",
				Example:     "15",
			},
			{
				Name:        "character",
				Type:        "number",
				Required:    false,
				Description: "Character position (0-based) on the line",
				Example:     "10",
			},
		},
		Examples: []string{
			`{"tool": "find_implementations", "arguments": {"file": "agent/base/base.go", "line": 15}}`,
			`{"tool": "find_implementations", "arguments": {"file": "interfaces/handler.go", "line": 42, "character": 5}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPFindImplementationsTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line is required")
	}
	return nil
}

// Execute finds all implementations of an interface/type
func (t *LSPFindImplementationsTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if packageLSPManager == nil {
		return "", fmt.Errorf("LSP manager not initialized")
	}

	filePath := args["file"].(string)

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

	// Get line (convert from 1-based to 0-based)
	line := int(args["line"].(float64)) - 1
	if line < 0 {
		return "", fmt.Errorf("line number must be >= 1")
	}

	// Get character position (0-based, default to 0)
	character := 0
	if charVal, ok := args["character"]; ok {
		character = int(charVal.(float64))
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

	// Convert to file:// URI and detect language
	fileURI := "file://" + filePath
	languageID := getLanguageID(filePath)

	// Open document
	if err := client.OpenDocument(ctx, fileURI, languageID, string(content)); err != nil {
		return "", fmt.Errorf("failed to open document: %w", err)
	}

	// Find implementations
	locations, err := client.FindImplementations(ctx, fileURI, line, character)
	if err != nil {
		return "", fmt.Errorf("failed to find implementations: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"query": map[string]interface{}{
			"file":      filePath,
			"line":      line + 1, // Convert back to 1-based
			"character": character,
		},
		"found":                len(locations) > 0,
		"implementation_count": len(locations),
		"implementations":      []map[string]interface{}{},
	}

	// Process locations
	implList := []map[string]interface{}{}
	for _, loc := range locations {
		// Extract file path from URI
		implFile := strings.TrimPrefix(loc.URI, "file://")

		// Try to read preview (first line of implementation)
		preview := ""
		if implContent, err := os.ReadFile(implFile); err == nil {
			lines := strings.Split(string(implContent), "\n")
			if loc.Range.Start.Line < len(lines) {
				preview = strings.TrimSpace(lines[loc.Range.Start.Line])
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
			}
		}

		implList = append(implList, map[string]interface{}{
			"file":    implFile,
			"line":    loc.Range.Start.Line + 1, // Convert to 1-based
			"preview": preview,
		})
	}

	result["implementations"] = implList

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func init() {
	registry.Register(&LSPFindImplementationsTool{})
}
