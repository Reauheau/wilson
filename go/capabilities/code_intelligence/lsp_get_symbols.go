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

// LSPGetSymbolsTool gets all symbols in a document using LSP
type LSPGetSymbolsTool struct {
	lspManager *lsp.Manager
}

// Metadata returns tool metadata
func (t *LSPGetSymbolsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get_symbols",
		Description: "Get all functions, types, variables in a file. Supports Go, Python, JavaScript, TypeScript, Rust. Fast alternative to parse_file for understanding file structure.",
		Category:    CategoryAI,
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Description: "File to analyze",
				Example:     "agent/code_agent.go",
			},
			{
				Name:        "filter",
				Type:        "string",
				Required:    false,
				Description: "Filter by symbol type: functions, types, variables, all (default: all)",
				Example:     "functions",
			},
		},
		Examples: []string{
			`{"tool": "get_symbols", "arguments": {"file": "agent/code_agent.go"}}`,
			`{"tool": "get_symbols", "arguments": {"file": "main.go", "filter": "functions"}}`,
		},
	}
}

// Validate validates the arguments
func (t *LSPGetSymbolsTool) Validate(args map[string]interface{}) error {
	if _, ok := args["file"]; !ok {
		return fmt.Errorf("file is required")
	}
	return nil
}

// Execute gets all symbols in a document
func (t *LSPGetSymbolsTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
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

	// Get filter (default: all)
	filter := "all"
	if filterVal, ok := args["filter"]; ok {
		filter = filterVal.(string)
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

	// Call LSP document symbols
	symbols, err := client.GetDocumentSymbols(ctx, fileURI)
	if err != nil {
		return "", fmt.Errorf("get-symbols failed: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"file":         filePath,
		"filter":       filter,
		"symbol_count": 0,
	}

	// Categorize symbols
	functions := []map[string]interface{}{}
	types := []map[string]interface{}{}
	variables := []map[string]interface{}{}
	other := []map[string]interface{}{}

	var processSymbol func(sym lsp.DocumentSymbol)
	processSymbol = func(sym lsp.DocumentSymbol) {
		symbolInfo := map[string]interface{}{
			"name":   sym.Name,
			"kind":   symbolKindToString(sym.Kind),
			"line":   sym.Range.Start.Line + 1,
			"detail": sym.Detail,
		}

		// Categorize by kind
		switch sym.Kind {
		case lsp.SymbolKindFunction, lsp.SymbolKindMethod, lsp.SymbolKindConstructor:
			functions = append(functions, symbolInfo)
		case lsp.SymbolKindClass, lsp.SymbolKindInterface, lsp.SymbolKindStruct, lsp.SymbolKindEnum:
			types = append(types, symbolInfo)
		case lsp.SymbolKindVariable, lsp.SymbolKindConstant, lsp.SymbolKindField, lsp.SymbolKindProperty:
			variables = append(variables, symbolInfo)
		default:
			other = append(other, symbolInfo)
		}

		// Process children recursively
		for _, child := range sym.Children {
			processSymbol(child)
		}
	}

	for _, sym := range symbols {
		processSymbol(sym)
	}

	// Build result based on filter
	switch filter {
	case "functions":
		result["symbols"] = functions
		result["symbol_count"] = len(functions)
	case "types":
		result["symbols"] = types
		result["symbol_count"] = len(types)
	case "variables":
		result["symbols"] = variables
		result["symbol_count"] = len(variables)
	default: // "all"
		result["functions"] = functions
		result["types"] = types
		result["variables"] = variables
		result["other"] = other
		result["symbol_count"] = len(functions) + len(types) + len(variables) + len(other)
		result["counts"] = map[string]int{
			"functions": len(functions),
			"types":     len(types),
			"variables": len(variables),
			"other":     len(other),
		}
	}

	// Add summary
	if result["symbol_count"].(int) == 0 {
		result["message"] = "No symbols found in file"
	} else {
		if filter == "all" {
			result["message"] = fmt.Sprintf("Found %d total symbols (%d functions, %d types, %d variables)",
				result["symbol_count"], len(functions), len(types), len(variables))
		} else {
			result["message"] = fmt.Sprintf("Found %d %s", result["symbol_count"], filter)
		}
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func symbolKindToString(kind lsp.SymbolKind) string {
	switch kind {
	case lsp.SymbolKindFile:
		return "file"
	case lsp.SymbolKindModule:
		return "module"
	case lsp.SymbolKindNamespace:
		return "namespace"
	case lsp.SymbolKindPackage:
		return "package"
	case lsp.SymbolKindClass:
		return "class"
	case lsp.SymbolKindMethod:
		return "method"
	case lsp.SymbolKindProperty:
		return "property"
	case lsp.SymbolKindField:
		return "field"
	case lsp.SymbolKindConstructor:
		return "constructor"
	case lsp.SymbolKindEnum:
		return "enum"
	case lsp.SymbolKindInterface:
		return "interface"
	case lsp.SymbolKindFunction:
		return "function"
	case lsp.SymbolKindVariable:
		return "variable"
	case lsp.SymbolKindConstant:
		return "constant"
	case lsp.SymbolKindStruct:
		return "struct"
	default:
		return "unknown"
	}
}

func init() {
	tool := &LSPGetSymbolsTool{}
	registry.Register(tool)
}
