package code_intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/lsp"
)

// LSPHoverTool gets hover information (signature, docs) for a symbol
type LSPHoverTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPHoverTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get_hover_info",
		Description: "Get documentation, type information, and function signatures for a symbol. Supports Go, Python, JavaScript, TypeScript, Rust. Fast way to understand code without reading files.",
		Category:    CategoryAI,
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File containing the symbol",
				Example:     "agent/base.go",
			},
			{
				Name:        "line",
				Type:        "number",
				Required:    true,
				Description: "Line number (1-based) where symbol appears",
				Example:     "89",
			},
			{
				Name:        "character",
				Type:        "number",
				Required:    false,
				Description: "Character position (0-based) on the line",
				Example:     "15",
			},
		},
		Examples: []string{
			`{"tool": "get_hover_info", "arguments": {"file": "agent/base.go", "line": 89, "character": 15}}`,
			`{"tool": "get_hover_info", "arguments": {"file": "main.go", "line": 42}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPHoverTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line is required")
	}
	return nil
}

// Execute gets hover information for a symbol
func (t *LSPHoverTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get LSP client
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
	languageID := getLanguageID(filePath)

	// Open document
	if err := client.OpenDocument(ctx, fileURI, languageID, string(content)); err != nil {
		return "", fmt.Errorf("failed to open document: %w", err)
	}

	// Call LSP hover
	hover, err := client.GetHover(ctx, fileURI, line, character)
	if err != nil {
		return "", fmt.Errorf("hover failed: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"query": map[string]interface{}{
			"file":      filePath,
			"line":      line + 1, // Convert back to 1-based
			"character": character,
		},
		"found": hover != nil && hover.Contents.Value != "",
	}

	if hover == nil || hover.Contents.Value == "" {
		result["message"] = "No hover information available at this location"
	} else {
		result["content"] = hover.Contents.Value
		result["content_kind"] = hover.Contents.Kind

		// If range is provided, include it
		if hover.Range != nil {
			result["range"] = map[string]interface{}{
				"start": map[string]interface{}{
					"line":      hover.Range.Start.Line + 1,
					"character": hover.Range.Start.Character,
				},
				"end": map[string]interface{}{
					"line":      hover.Range.End.Line + 1,
					"character": hover.Range.End.Character,
				},
			}
		}

		result["message"] = "Hover information retrieved successfully"
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func init() {
	tool := &LSPHoverTool{}
	registry.Register(tool)
}
