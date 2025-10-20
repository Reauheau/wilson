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

// AnalyzeImportsTool analyzes imports in a Go file
type AnalyzeImportsTool struct{}

func init() {
	registry.Register(&AnalyzeImportsTool{})
}

func (t *AnalyzeImportsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "analyze_imports",
		Description:     "Analyze imports in a Go file: current imports, unused imports, missing imports, suggestions.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Path to Go file",
				Example:     "main.go",
			},
		},
		Examples: []string{
			`{"tool": "analyze_imports", "arguments": {"path": "main.go"}}`,
		},
	}
}

func (t *AnalyzeImportsTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	if !strings.HasSuffix(path, ".go") {
		return fmt.Errorf("path must be a .go file")
	}
	return nil
}

func (t *AnalyzeImportsTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Get current imports
	imports := extractImportDetails(node)

	// Find which imports are used
	usedImports := findUsedImports(node, imports)

	// Determine unused
	unusedImports := []string{}
	for _, imp := range imports {
		if !usedImports[imp["path"].(string)] {
			unusedImports = append(unusedImports, imp["path"].(string))
		}
	}

	result := map[string]interface{}{
		"file":            path,
		"imports":         imports,
		"import_count":    len(imports),
		"unused_imports":  unusedImports,
		"unused_count":    len(unusedImports),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

func extractImportDetails(node *ast.File) []map[string]interface{} {
	var imports []map[string]interface{}

	for _, imp := range node.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		impInfo := map[string]interface{}{
			"path": impPath,
		}

		if imp.Name != nil {
			impInfo["alias"] = imp.Name.Name
		}

		// Get package name from path
		parts := strings.Split(impPath, "/")
		impInfo["package_name"] = parts[len(parts)-1]

		imports = append(imports, impInfo)
	}

	return imports
}

func findUsedImports(node *ast.File, imports []map[string]interface{}) map[string]bool {
	used := make(map[string]bool)

	// Build map of package names to import paths
	pkgToPath := make(map[string]string)
	for _, imp := range imports {
		pkgName := imp["package_name"].(string)
		if alias, ok := imp["alias"]; ok {
			pkgName = alias.(string)
		}
		pkgToPath[pkgName] = imp["path"].(string)
	}

	// Look for selectors (package.Symbol)
	ast.Inspect(node, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if path, exists := pkgToPath[ident.Name]; exists {
					used[path] = true
				}
			}
		}
		return true
	})

	return used
}
