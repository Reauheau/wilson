package analysis

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

// FindPatternsTool discovers code patterns in the codebase
type FindPatternsTool struct{}

func init() {
	registry.Register(&FindPatternsTool{})
}

func (t *FindPatternsTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "find_patterns",
		Description:     "Discover code patterns in the codebase. Search for error handling patterns, struct patterns, interface implementations, function patterns, and more. Learn project conventions from existing code.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "pattern_type",
				Type:        "string",
				Required:    true,
				Description: "Type of pattern to find: 'error_handling', 'struct_definition', 'interface_impl', 'function_pattern', 'import_pattern'",
				Example:     "error_handling",
			},
			{
				Name:        "search_path",
				Type:        "string",
				Required:    false,
				Description: "Directory to search in (default: current directory)",
				Example:     "agent",
			},
			{
				Name:        "keyword",
				Type:        "string",
				Required:    false,
				Description: "Optional keyword to filter results (e.g., specific function name, struct name)",
				Example:     "Agent",
			},
			{
				Name:        "limit",
				Type:        "number",
				Required:    false,
				Description: "Maximum number of examples to return (default: 10)",
				Example:     "5",
			},
		},
		Examples: []string{
			`{"tool": "find_patterns", "arguments": {"pattern_type": "error_handling"}}`,
			`{"tool": "find_patterns", "arguments": {"pattern_type": "struct_definition", "keyword": "Agent"}}`,
			`{"tool": "find_patterns", "arguments": {"pattern_type": "interface_impl", "search_path": "agent"}}`,
		},
	}
}

func (t *FindPatternsTool) Validate(args map[string]interface{}) error {
	patternType, ok := args["pattern_type"].(string)
	if !ok || patternType == "" {
		return fmt.Errorf("pattern_type parameter is required")
	}

	validTypes := []string{"error_handling", "struct_definition", "interface_impl", "function_pattern", "import_pattern"}
	valid := false
	for _, vt := range validTypes {
		if patternType == vt {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("pattern_type must be one of: %s", strings.Join(validTypes, ", "))
	}

	return nil
}

func (t *FindPatternsTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	patternType, _ := input["pattern_type"].(string)

	searchPath := "."
	if sp, ok := input["search_path"].(string); ok && sp != "" {
		searchPath = sp
	}

	keyword := ""
	if kw, ok := input["keyword"].(string); ok {
		keyword = kw
	}

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	// Make absolute
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", searchPath)
	}

	// Find patterns
	finder := &PatternFinder{
		PatternType: patternType,
		Keyword:     keyword,
		Limit:       limit,
		Examples:    []PatternExample{},
	}

	err = finder.search(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to search for patterns: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"pattern_type":   patternType,
		"search_path":    searchPath,
		"keyword":        keyword,
		"examples_found": len(finder.Examples),
		"examples":       finder.Examples,
		"summary":        finder.getSummary(),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// PatternFinder searches for code patterns
type PatternFinder struct {
	PatternType string
	Keyword     string
	Limit       int
	Examples    []PatternExample
}

// PatternExample represents a found pattern example
type PatternExample struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

// search recursively searches for patterns
func (f *PatternFinder) search(path string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files (not test files for cleaner examples)
		if !strings.HasSuffix(filePath, ".go") || strings.HasSuffix(filePath, "_test.go") {
			return nil
		}

		// Check if we've reached the limit
		if len(f.Examples) >= f.Limit {
			return filepath.SkipAll
		}

		// Parse and search file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip files with parse errors
		}

		// Search based on pattern type
		switch f.PatternType {
		case "error_handling":
			f.findErrorHandling(node, fset, filePath)
		case "struct_definition":
			f.findStructDefinitions(node, fset, filePath)
		case "interface_impl":
			f.findInterfaceImplementations(node, fset, filePath)
		case "function_pattern":
			f.findFunctionPatterns(node, fset, filePath)
		case "import_pattern":
			f.findImportPatterns(node, fset, filePath)
		}

		return nil
	})
}

// findErrorHandling finds error handling patterns
func (f *PatternFinder) findErrorHandling(node *ast.File, fset *token.FileSet, filePath string) {
	ast.Inspect(node, func(n ast.Node) bool {
		if len(f.Examples) >= f.Limit {
			return false
		}

		// Look for if err != nil patterns
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}

		// Check if it's an error check
		bin, ok := ifStmt.Cond.(*ast.BinaryExpr)
		if !ok || bin.Op.String() != "!=" {
			return true
		}

		// Check if comparing to nil
		ident, ok := bin.X.(*ast.Ident)
		if !ok || ident.Name != "err" {
			return true
		}

		// Extract the error handling code
		pos := fset.Position(ifStmt.Pos())
		endPos := fset.Position(ifStmt.End())

		// Read the source to get the actual code
		content, err := os.ReadFile(filePath)
		if err != nil {
			return true
		}

		lines := strings.Split(string(content), "\n")
		if pos.Line > 0 && endPos.Line <= len(lines) {
			codeLines := lines[pos.Line-1 : endPos.Line]
			code := strings.Join(codeLines, "\n")

			// Apply keyword filter if specified
			if f.Keyword != "" && !strings.Contains(code, f.Keyword) {
				return true
			}

			f.Examples = append(f.Examples, PatternExample{
				File:        filePath,
				Line:        pos.Line,
				Name:        "Error handling",
				Code:        strings.TrimSpace(code),
				Description: "Error check and handling pattern",
			})
		}

		return true
	})
}

// findStructDefinitions finds struct definition patterns
func (f *PatternFinder) findStructDefinitions(node *ast.File, fset *token.FileSet, filePath string) {
	ast.Inspect(node, func(n ast.Node) bool {
		if len(f.Examples) >= f.Limit {
			return false
		}

		// Look for type declarations
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok.String() != "type" {
			return true
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Check if it's a struct
			_, ok = typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Apply keyword filter
			if f.Keyword != "" && !strings.Contains(typeSpec.Name.Name, f.Keyword) {
				continue
			}

			pos := fset.Position(genDecl.Pos())
			endPos := fset.Position(genDecl.End())

			// Read source
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			lines := strings.Split(string(content), "\n")
			if pos.Line > 0 && endPos.Line <= len(lines) {
				codeLines := lines[pos.Line-1 : endPos.Line]
				code := strings.Join(codeLines, "\n")

				f.Examples = append(f.Examples, PatternExample{
					File:        filePath,
					Line:        pos.Line,
					Name:        typeSpec.Name.Name,
					Code:        strings.TrimSpace(code),
					Description: fmt.Sprintf("Struct definition for %s", typeSpec.Name.Name),
				})
			}
		}

		return true
	})
}

// findInterfaceImplementations finds interface implementation patterns
func (f *PatternFinder) findInterfaceImplementations(node *ast.File, fset *token.FileSet, filePath string) {
	// Find interface definitions
	interfaces := make(map[string][]string)

	ast.Inspect(node, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok.String() != "type" {
			return true
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Check if it's an interface
			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			// Extract method names
			methods := []string{}
			for _, method := range interfaceType.Methods.List {
				if len(method.Names) > 0 {
					methods = append(methods, method.Names[0].Name)
				}
			}

			interfaces[typeSpec.Name.Name] = methods
		}

		return true
	})

	// Show interface definitions as examples
	for name, methods := range interfaces {
		if len(f.Examples) >= f.Limit {
			break
		}

		// Apply keyword filter
		if f.Keyword != "" && !strings.Contains(name, f.Keyword) {
			continue
		}

		f.Examples = append(f.Examples, PatternExample{
			File:        filePath,
			Line:        0,
			Name:        name,
			Code:        fmt.Sprintf("interface %s with methods: %s", name, strings.Join(methods, ", ")),
			Description: fmt.Sprintf("Interface %s requires %d methods", name, len(methods)),
		})
	}
}

// findFunctionPatterns finds function definition patterns
func (f *PatternFinder) findFunctionPatterns(node *ast.File, fset *token.FileSet, filePath string) {
	ast.Inspect(node, func(n ast.Node) bool {
		if len(f.Examples) >= f.Limit {
			return false
		}

		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Apply keyword filter
		if f.Keyword != "" && !strings.Contains(funcDecl.Name.Name, f.Keyword) {
			return true
		}

		pos := fset.Position(funcDecl.Pos())

		// Extract function signature
		signature := funcDecl.Name.Name + "("
		if funcDecl.Type.Params != nil {
			params := []string{}
			for _, param := range funcDecl.Type.Params.List {
				for range param.Names {
					params = append(params, fmt.Sprintf("%v", param.Type))
				}
			}
			signature += strings.Join(params, ", ")
		}
		signature += ")"

		if funcDecl.Type.Results != nil {
			results := []string{}
			for _, result := range funcDecl.Type.Results.List {
				results = append(results, fmt.Sprintf("%v", result.Type))
			}
			if len(results) > 0 {
				signature += " (" + strings.Join(results, ", ") + ")"
			}
		}

		description := "Function pattern"
		if funcDecl.Recv != nil {
			description = "Method pattern"
		}

		f.Examples = append(f.Examples, PatternExample{
			File:        filePath,
			Line:        pos.Line,
			Name:        funcDecl.Name.Name,
			Code:        signature,
			Description: description,
		})

		return true
	})
}

// findImportPatterns finds common import patterns
func (f *PatternFinder) findImportPatterns(node *ast.File, fset *token.FileSet, filePath string) {
	if len(node.Imports) == 0 {
		return
	}

	imports := []string{}
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Apply keyword filter
		if f.Keyword != "" && !strings.Contains(importPath, f.Keyword) {
			continue
		}

		if imp.Name != nil {
			imports = append(imports, fmt.Sprintf("%s %s", imp.Name.Name, importPath))
		} else {
			imports = append(imports, importPath)
		}
	}

	if len(imports) > 0 && len(f.Examples) < f.Limit {
		f.Examples = append(f.Examples, PatternExample{
			File:        filePath,
			Line:        1,
			Name:        "Import pattern",
			Code:        strings.Join(imports, "\n"),
			Description: fmt.Sprintf("Common imports in %s package", node.Name.Name),
		})
	}
}

// getSummary provides a summary of found patterns
func (f *PatternFinder) getSummary() string {
	if len(f.Examples) == 0 {
		return "No patterns found matching the criteria"
	}

	switch f.PatternType {
	case "error_handling":
		return fmt.Sprintf("Found %d error handling patterns. Project uses standard if err != nil checks.", len(f.Examples))
	case "struct_definition":
		return fmt.Sprintf("Found %d struct definitions. These show the data modeling patterns used in the project.", len(f.Examples))
	case "interface_impl":
		return fmt.Sprintf("Found %d interfaces. These define the contracts used in the project.", len(f.Examples))
	case "function_pattern":
		return fmt.Sprintf("Found %d function patterns. These show common function signatures and naming conventions.", len(f.Examples))
	case "import_pattern":
		return fmt.Sprintf("Found %d import patterns. These show commonly used packages in the project.", len(f.Examples))
	default:
		return fmt.Sprintf("Found %d examples", len(f.Examples))
	}
}
