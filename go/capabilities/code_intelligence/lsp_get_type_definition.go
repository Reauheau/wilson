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

// LSPGetTypeDefinitionTool finds the type definition of a variable using LSP
type LSPGetTypeDefinitionTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPGetTypeDefinitionTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get_type_definition",
		Description: "Jump to the type definition of a variable. Supports Go, Python, JavaScript, TypeScript, Rust. Essential for understanding data structures and generating correct code.",
		Category:    CategoryAI,
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File containing the variable",
				Example:     "agent/executor.go",
			},
			{
				Name:        "line",
				Type:        "number",
				Required:    true,
				Description: "Line number (1-based) where variable appears",
				Example:     "105",
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
			`{"tool": "get_type_definition", "arguments": {"file": "agent/executor.go", "line": 105}}`,
			`{"tool": "get_type_definition", "arguments": {"file": "main.go", "line": 42, "character": 15}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPGetTypeDefinitionTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line is required")
	}
	return nil
}

// Execute finds the type definition of a variable
func (t *LSPGetTypeDefinitionTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get type definition
	locations, err := client.GetTypeDefinition(ctx, fileURI, line, character)
	if err != nil {
		return "", fmt.Errorf("failed to get type definition: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"query": map[string]interface{}{
			"file":      filePath,
			"line":      line + 1, // Convert back to 1-based
			"character": character,
		},
		"found": len(locations) > 0,
	}

	if len(locations) > 0 {
		loc := locations[0] // Use first location (typically only one)

		// Extract file path from URI
		typeFile := strings.TrimPrefix(loc.URI, "file://")

		// Try to read preview (first line of type definition)
		preview := ""
		if typeContent, err := os.ReadFile(typeFile); err == nil {
			lines := strings.Split(string(typeContent), "\n")
			if loc.Range.Start.Line < len(lines) {
				preview = strings.TrimSpace(lines[loc.Range.Start.Line])
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
			}
		}

		result["type_definition"] = map[string]interface{}{
			"file":    typeFile,
			"line":    loc.Range.Start.Line + 1, // Convert to 1-based
			"preview": preview,
		}
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func init() {
	registry.Register(&LSPGetTypeDefinitionTool{})
}
