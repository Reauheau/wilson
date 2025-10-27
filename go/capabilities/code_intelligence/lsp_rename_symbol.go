package code_intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/lsp"
)

// LSPRenameSymbolTool renames symbols across workspace using LSP
type LSPRenameSymbolTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPRenameSymbolTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "rename_symbol",
		Description: "Safely rename a symbol (function, variable, type, method) across entire workspace. Supports Go, Python, JavaScript, TypeScript, Rust. Uses LSP to ensure all references are updated.",
		Category:    CategoryAI,
		RiskLevel:   RiskModerate, // Modifies multiple files
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File containing the symbol to rename",
				Example:     "agent/executor.go",
			},
			{
				Name:        "line",
				Type:        "number",
				Required:    true,
				Description: "Line number (1-based) where symbol appears",
				Example:     "42",
			},
			{
				Name:        "character",
				Type:        "number",
				Required:    false,
				Description: "Character position (0-based) on the line",
				Example:     "10",
			},
			{
				Name:        "new_name",
				Type:        "string",
				Required:    true,
				Description: "New name for the symbol",
				Example:     "ExecuteTask",
			},
			{
				Name:        "validate_only",
				Type:        "boolean",
				Required:    false,
				Description: "Only validate rename is possible, don't apply changes",
				Example:     "false",
			},
		},
		Examples: []string{
			`{"tool": "rename_symbol", "arguments": {"file": "agent/executor.go", "line": 42, "new_name": "ExecuteTask"}}`,
			`{"tool": "rename_symbol", "arguments": {"file": "main.go", "line": 15, "character": 5, "new_name": "processUserData", "validate_only": true}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPRenameSymbolTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	if _, ok := args["line"]; !ok {
		return fmt.Errorf("line is required")
	}
	if _, ok := args["new_name"]; !ok {
		return fmt.Errorf("new_name is required")
	}

	// Validate new_name is a valid identifier
	newName := args["new_name"].(string)
	if len(newName) == 0 {
		return fmt.Errorf("new_name cannot be empty")
	}
	// Basic identifier validation (alphanumeric + underscore, no spaces)
	for _, ch := range newName {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_') {
			return fmt.Errorf("new_name must be a valid identifier (alphanumeric + underscore only)")
		}
	}

	return nil
}

// Execute performs the rename operation
func (t *LSPRenameSymbolTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	newName := args["new_name"].(string)

	// Check if validate-only mode
	validateOnly := false
	if val, ok := args["validate_only"]; ok {
		validateOnly = val.(bool)
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

	// Step 1: Validate rename is possible
	prepareResult, err := client.PrepareRename(ctx, fileURI, line, character)
	if err != nil {
		return "", fmt.Errorf("cannot rename symbol: %w", err)
	}

	// Extract current symbol name from file content
	lines := strings.Split(string(content), "\n")
	currentName := ""
	if prepareResult.Range.Start.Line < len(lines) {
		lineText := lines[prepareResult.Range.Start.Line]
		start := prepareResult.Range.Start.Character
		end := prepareResult.Range.End.Character
		if start >= 0 && end <= len(lineText) && start < end {
			currentName = lineText[start:end]
		}
	}

	// If validate-only mode, return validation result
	if validateOnly {
		result := map[string]interface{}{
			"can_rename":   true,
			"current_name": currentName,
			"new_name":     newName,
			"location": map[string]interface{}{
				"file": filePath,
				"line": line + 1,
			},
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		return string(output), nil
	}

	// Step 2: Perform the rename
	edit, err := client.RenameSymbol(ctx, fileURI, line, character, newName)
	if err != nil {
		return "", fmt.Errorf("rename failed: %w", err)
	}

	// Step 3: Apply the workspace edits
	filesModified := make(map[string]int)
	totalEdits := 0

	// Handle both Changes and DocumentChanges formats
	editsToApply := make(map[string][]lsp.TextEdit)

	// Prefer DocumentChanges if available (newer LSP format)
	if len(edit.DocumentChanges) > 0 {
		for _, docEdit := range edit.DocumentChanges {
			uri := docEdit.TextDocument.URI
			editsToApply[uri] = docEdit.Edits
		}
	} else {
		// Fall back to Changes format
		editsToApply = edit.Changes
	}

	for uri, edits := range editsToApply {
		// Extract file path from URI
		targetFile := strings.TrimPrefix(uri, "file://")

		// Read target file
		targetContent, err := os.ReadFile(targetFile)
		if err != nil {
			return "", fmt.Errorf("failed to read file for editing %s: %w", targetFile, err)
		}

		// Apply edits (in reverse order to maintain offsets)
		fileLines := strings.Split(string(targetContent), "\n")

		// Sort edits by position (reverse order) to apply from bottom to top
		sortedEdits := make([]lsp.TextEdit, len(edits))
		copy(sortedEdits, edits)
		sort.Slice(sortedEdits, func(i, j int) bool {
			// Sort in reverse: later lines first, then later characters
			if sortedEdits[i].Range.Start.Line != sortedEdits[j].Range.Start.Line {
				return sortedEdits[i].Range.Start.Line > sortedEdits[j].Range.Start.Line
			}
			return sortedEdits[i].Range.Start.Character > sortedEdits[j].Range.Start.Character
		})

		// Apply edits from bottom to top
		for _, edit := range sortedEdits {
			if edit.Range.Start.Line < len(fileLines) {
				line := fileLines[edit.Range.Start.Line]
				start := edit.Range.Start.Character
				end := edit.Range.End.Character

				if start >= 0 && end <= len(line) && start <= end {
					newLine := line[:start] + edit.NewText + line[end:]
					fileLines[edit.Range.Start.Line] = newLine
					totalEdits++
				}
			}
		}

		// Write back to file
		newContent := strings.Join(fileLines, "\n")
		if err := os.WriteFile(targetFile, []byte(newContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write renamed file %s: %w", targetFile, err)
		}

		filesModified[targetFile] = len(edits)
	}

	// Build result
	result := map[string]interface{}{
		"success":        true,
		"old_name":       currentName,
		"new_name":       newName,
		"files_modified": len(filesModified),
		"total_edits":    totalEdits,
		"files":          []map[string]interface{}{},
	}

	filesList := []map[string]interface{}{}
	for file, editCount := range filesModified {
		filesList = append(filesList, map[string]interface{}{
			"file":       file,
			"edit_count": editCount,
		})
	}
	result["files"] = filesList

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func init() {
	registry.Register(&LSPRenameSymbolTool{})
}
