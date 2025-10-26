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

// LSPFindReferencesTool finds all references to a symbol using LSP
type LSPFindReferencesTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPFindReferencesTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "find_references",
		Description: "Find all places where a symbol is used across the codebase. Critical for understanding impact before changes.",
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
				Description: "Line number (1-based) where symbol is defined or used",
				Example:     "89",
			},
			{
				Name:        "character",
				Type:        "number",
				Required:    false,
				Description: "Character position (0-based) on the line",
				Example:     "15",
			},
			{
				Name:        "include_declaration",
				Type:        "boolean",
				Required:    false,
				Description: "Include the symbol's declaration in results (default: true)",
				Example:     "true",
			},
		},
		Examples: []string{
			`{"tool": "find_references", "arguments": {"file": "agent/base.go", "line": 89, "character": 15}}`,
			`{"tool": "find_references", "arguments": {"file": "main.go", "line": 42, "include_declaration": false}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPFindReferencesTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line is required")
	}
	return nil
}

// Execute finds all references to a symbol
func (t *LSPFindReferencesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get include_declaration flag (default true)
	includeDeclaration := true
	if inclVal, ok := args["include_declaration"]; ok {
		includeDeclaration = inclVal.(bool)
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

	// Call LSP find-references
	locations, err := client.FindReferences(ctx, fileURI, line, character, includeDeclaration)
	if err != nil {
		return "", fmt.Errorf("find-references failed: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"query": map[string]interface{}{
			"file":                filePath,
			"line":                line + 1, // Convert back to 1-based
			"character":           character,
			"include_declaration": includeDeclaration,
		},
		"found":           len(locations) > 0,
		"reference_count": len(locations),
	}

	if len(locations) == 0 {
		result["message"] = "No references found for this symbol"
	} else {
		// Group references by file
		fileGroups := make(map[string][]map[string]interface{})
		filesCount := 0

		for _, loc := range locations {
			refFile := strings.TrimPrefix(loc.URI, "file://")

			refInfo := map[string]interface{}{
				"line":      loc.Range.Start.Line + 1,
				"character": loc.Range.Start.Character,
			}

			// Try to read context (the line containing the reference)
			if refContent, err := os.ReadFile(refFile); err == nil {
				lines := strings.Split(string(refContent), "\n")
				if loc.Range.Start.Line < len(lines) {
					context := strings.TrimSpace(lines[loc.Range.Start.Line])
					if len(context) > 80 {
						context = context[:80] + "..."
					}
					refInfo["context"] = context
				}
			}

			if _, exists := fileGroups[refFile]; !exists {
				filesCount++
			}
			fileGroups[refFile] = append(fileGroups[refFile], refInfo)
		}

		result["files_count"] = filesCount
		result["references"] = fileGroups

		// Add summary
		result["summary"] = fmt.Sprintf("Found %d reference(s) across %d file(s)",
			len(locations), filesCount)

		// Add top files (most references)
		topFiles := []map[string]interface{}{}
		for file, refs := range fileGroups {
			topFiles = append(topFiles, map[string]interface{}{
				"file":  file,
				"count": len(refs),
			})
		}
		result["top_files"] = topFiles
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func init() {
	tool := &LSPFindReferencesTool{}
	registry.Register(tool)
}
