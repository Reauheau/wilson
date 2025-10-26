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

// LSPGoToDefinitionTool finds where a symbol is defined using LSP
type LSPGoToDefinitionTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPGoToDefinitionTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "go_to_definition",
		Description: "Find where a function, variable, or type is defined. More accurate than grep - uses language server's symbol table.",
		Category:    CategoryAI,
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File containing the symbol reference",
				Example:     "agent/code_agent.go",
			},
			{
				Name:        "line",
				Type:        "number",
				Required:    true,
				Description: "Line number (1-based) where symbol appears",
				Example:     "109",
			},
			{
				Name:        "character",
				Type:        "number",
				Required:    false,
				Description: "Character position (0-based) on the line. If not provided, will search for symbol on line.",
				Example:     "15",
			},
		},
		Examples: []string{
			`{"tool": "go_to_definition", "arguments": {"file": "agent/code_agent.go", "line": 109, "character": 15}}`,
			`{"tool": "go_to_definition", "arguments": {"file": "main.go", "line": 42}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPGoToDefinitionTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line is required")
	}
	return nil
}

// Execute finds the definition of a symbol
func (t *LSPGoToDefinitionTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Call LSP go-to-definition
	locations, err := client.GoToDefinition(ctx, fileURI, line, character)
	if err != nil {
		return "", fmt.Errorf("go-to-definition failed: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"query": map[string]interface{}{
			"file":      filePath,
			"line":      line + 1, // Convert back to 1-based
			"character": character,
		},
		"found":            len(locations) > 0,
		"definition_count": len(locations),
	}

	if len(locations) == 0 {
		result["message"] = "No definition found at this location"
	} else {
		// Extract first location (primary definition)
		loc := locations[0]
		defFile := strings.TrimPrefix(loc.URI, "file://")

		result["definition"] = map[string]interface{}{
			"file":      defFile,
			"line":      loc.Range.Start.Line + 1, // 1-based for display
			"character": loc.Range.Start.Character,
			"uri":       loc.URI,
		}

		// Try to read a preview of the definition
		if defContent, err := os.ReadFile(defFile); err == nil {
			lines := strings.Split(string(defContent), "\n")
			if loc.Range.Start.Line < len(lines) {
				preview := lines[loc.Range.Start.Line]
				// Trim excessive whitespace
				preview = strings.TrimSpace(preview)
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				result["preview"] = preview
			}
		}

		// Add message
		result["message"] = fmt.Sprintf("Found definition in %s at line %d",
			filepath.Base(defFile), loc.Range.Start.Line+1)

		// If multiple definitions, include them all
		if len(locations) > 1 {
			allLocs := []map[string]interface{}{}
			for _, l := range locations {
				allLocs = append(allLocs, map[string]interface{}{
					"file":      strings.TrimPrefix(l.URI, "file://"),
					"line":      l.Range.Start.Line + 1,
					"character": l.Range.Start.Character,
				})
			}
			result["all_definitions"] = allLocs
		}
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func init() {
	tool := &LSPGoToDefinitionTool{}
	registry.Register(tool)
}
