package ast

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// FindSymbolTool finds symbol definitions and usages in Go code
type FindSymbolTool struct{}

func init() {
	registry.Register(&FindSymbolTool{})
}

func (t *FindSymbolTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "find_symbol",
		Description:     "Find where a symbol (function, type, variable, constant) is defined and used. Returns definition location and all usages.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "symbol",
				Type:        "string",
				Required:    true,
				Description: "Symbol name to search for (function, type, variable, constant)",
				Example:     "CodeAgent",
			},
			{
				Name:        "search_path",
				Type:        "string",
				Required:    false,
				Description: "Directory to search in (default: current directory)",
				Example:     "agent",
			},
		},
		Examples: []string{
			`{"tool": "find_symbol", "arguments": {"symbol": "CodeAgent"}}`,
			`{"tool": "find_symbol", "arguments": {"symbol": "Execute", "search_path": "agent"}}`,
		},
	}
}

func (t *FindSymbolTool) Validate(args map[string]interface{}) error {
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return fmt.Errorf("symbol parameter is required")
	}
	return nil
}

func (t *FindSymbolTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get symbol
	symbol, ok := input["symbol"].(string)
	if !ok || symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}

	// Get search path
	searchPath := "."
	if sp, ok := input["search_path"].(string); ok && sp != "" {
		searchPath = sp
	}

	// Make absolute
	absSearchPath, err := filepath.Abs(searchPath)
	if err != nil {
		return "", fmt.Errorf("invalid search path: %w", err)
	}

	// Check path exists
	if _, err := os.Stat(absSearchPath); os.IsNotExist(err) {
		return "", fmt.Errorf("search path does not exist: %s", searchPath)
	}

	// Find all Go files
	goFiles, err := findGoFiles(absSearchPath)
	if err != nil {
		return "", fmt.Errorf("failed to find Go files: %w", err)
	}

	if len(goFiles) == 0 {
		return "", fmt.Errorf("no Go files found in %s", searchPath)
	}

	// Search for symbol
	results := &SymbolResults{
		Symbol:      symbol,
		SearchPath:  searchPath,
		Definitions: []Location{},
		Usages:      []Location{},
	}

	for _, file := range goFiles {
		if err := searchFileForSymbol(file, symbol, results); err != nil {
			// Log error but continue with other files
			continue
		}
	}

	// Build result
	result := map[string]interface{}{
		"symbol":           results.Symbol,
		"search_path":      results.SearchPath,
		"files_searched":   len(goFiles),
		"definitions":      results.Definitions,
		"definition_count": len(results.Definitions),
		"usages":           results.Usages,
		"usage_count":      len(results.Usages),
	}

	if len(results.Definitions) == 0 {
		result["message"] = fmt.Sprintf("Symbol '%s' not found in %s", symbol, searchPath)
	} else {
		result["message"] = fmt.Sprintf("Found %d definition(s) and %d usage(s) of '%s'",
			len(results.Definitions), len(results.Usages), symbol)
	}

	// Convert to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// SymbolResults holds search results
type SymbolResults struct {
	Symbol      string
	SearchPath  string
	Definitions []Location
	Usages      []Location
}

// Location represents a code location
type Location struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Context string `json:"context"`
	Type    string `json:"type"` // function, type, variable, constant, method, field
}

// findGoFiles recursively finds all .go files
func findGoFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only .go files, skip test files for now
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// searchFileForSymbol searches a single file for symbol
func searchFileForSymbol(filePath string, symbol string, results *SymbolResults) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// Make relative path
	relPath, _ := filepath.Rel(".", filePath)

	// Inspect AST
	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			// Function or method definition
			if decl.Name.Name == symbol {
				pos := fset.Position(decl.Pos())
				context := "func " + symbol
				if decl.Recv != nil {
					context = "method " + symbol
				}
				results.Definitions = append(results.Definitions, Location{
					File:    relPath,
					Line:    pos.Line,
					Column:  pos.Column,
					Context: context,
					Type:    "function",
				})
			}

		case *ast.TypeSpec:
			// Type definition
			if decl.Name.Name == symbol {
				pos := fset.Position(decl.Pos())
				context := "type " + symbol
				results.Definitions = append(results.Definitions, Location{
					File:    relPath,
					Line:    pos.Line,
					Column:  pos.Column,
					Context: context,
					Type:    "type",
				})
			}

		case *ast.ValueSpec:
			// Variable or constant
			for _, name := range decl.Names {
				if name.Name == symbol {
					pos := fset.Position(name.Pos())
					context := "var " + symbol
					results.Definitions = append(results.Definitions, Location{
						File:    relPath,
						Line:    pos.Line,
						Column:  pos.Column,
						Context: context,
						Type:    "variable",
					})
				}
			}

		case *ast.Ident:
			// Usage (reference to symbol)
			if decl.Name == symbol {
				pos := fset.Position(decl.Pos())
				// Only add if not a definition (definitions already added above)
				if !isDefinition(n) {
					results.Usages = append(results.Usages, Location{
						File:    relPath,
						Line:    pos.Line,
						Column:  pos.Column,
						Context: extractUsageContext(decl, fset),
						Type:    "usage",
					})
				}
			}
		}
		return true
	})

	return nil
}

// isDefinition checks if an identifier is a definition
func isDefinition(node ast.Node) bool {
	// This is simplified - in reality we'd check parent nodes
	// For now, we rely on separate definition detection above
	return false
}

// extractUsageContext gets context around a usage
func extractUsageContext(ident *ast.Ident, fset *token.FileSet) string {
	// Simplified - just return the identifier name
	// Could be enhanced to show more context
	return ident.Name
}
