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

// ParseFileTool parses a Go file and returns its AST structure
type ParseFileTool struct{}

func init() {
	registry.Register(&ParseFileTool{})
}

func (t *ParseFileTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "parse_file",
		Description:     "Parse a Go file and extract its structure: functions, types, imports, variables. Returns AST information for code understanding.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Relative path to the Go file to parse",
				Example:     "agent/code_agent.go",
			},
			{
				Name:        "detail_level",
				Type:        "string",
				Required:    false,
				Description: "Level of detail: 'summary' (default) or 'full' (includes function bodies)",
				Example:     "summary",
			},
		},
		Examples: []string{
			`{"tool": "parse_file", "arguments": {"path": "main.go"}}`,
		},
	}
}

func (t *ParseFileTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	if !strings.HasSuffix(path, ".go") {
		return fmt.Errorf("path must be a .go file")
	}
	return nil
}

func (t *ParseFileTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get path
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Get detail level
	detailLevel := "summary"
	if dl, ok := input["detail_level"].(string); ok {
		detailLevel = dl
	}

	// Validate path
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths not allowed, use relative paths")
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}

	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Extract structure
	structure := extractFileStructure(node, fset, detailLevel)
	structure["file_path"] = path

	// Convert to JSON
	resultJSON, err := json.MarshalIndent(structure, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// extractFileStructure extracts structured information from AST
func extractFileStructure(node *ast.File, fset *token.FileSet, detailLevel string) map[string]interface{} {
	result := map[string]interface{}{
		"package":   node.Name.Name,
		"imports":   extractImports(node),
		"functions": extractFunctions(node, fset, detailLevel),
		"types":     extractTypes(node, fset),
		"constants": extractConstants(node, fset),
		"variables": extractVariables(node, fset),
	}

	if node.Doc != nil {
		result["package_doc"] = node.Doc.Text()
	}

	return result
}

// extractImports extracts import declarations
func extractImports(node *ast.File) []map[string]interface{} {
	var imports []map[string]interface{}

	for _, imp := range node.Imports {
		impInfo := map[string]interface{}{
			"path": strings.Trim(imp.Path.Value, `"`),
		}
		if imp.Name != nil {
			impInfo["alias"] = imp.Name.Name
		}
		imports = append(imports, impInfo)
	}

	return imports
}

// extractFunctions extracts function declarations
func extractFunctions(node *ast.File, fset *token.FileSet, detailLevel string) []map[string]interface{} {
	var functions []map[string]interface{}

	ast.Inspect(node, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			funcInfo := map[string]interface{}{
				"name":     fn.Name.Name,
				"line":     fset.Position(fn.Pos()).Line,
				"exported": ast.IsExported(fn.Name.Name),
			}

			// Receiver (method)
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				funcInfo["receiver"] = extractType(fn.Recv.List[0].Type)
				funcInfo["is_method"] = true
			} else {
				funcInfo["is_method"] = false
			}

			// Parameters
			if fn.Type.Params != nil {
				funcInfo["parameters"] = extractFieldList(fn.Type.Params)
			}

			// Return values
			if fn.Type.Results != nil {
				funcInfo["returns"] = extractFieldList(fn.Type.Results)
			}

			// Documentation
			if fn.Doc != nil {
				funcInfo["doc"] = strings.TrimSpace(fn.Doc.Text())
			}

			// Body (if full detail)
			if detailLevel == "full" && fn.Body != nil {
				funcInfo["body_lines"] = fset.Position(fn.Body.End()).Line - fset.Position(fn.Body.Pos()).Line
			}

			functions = append(functions, funcInfo)
		}
		return true
	})

	return functions
}

// extractTypes extracts type declarations
func extractTypes(node *ast.File, fset *token.FileSet) []map[string]interface{} {
	var types []map[string]interface{}

	ast.Inspect(node, func(n ast.Node) bool {
		switch typ := n.(type) {
		case *ast.TypeSpec:
			typeInfo := map[string]interface{}{
				"name":     typ.Name.Name,
				"line":     fset.Position(typ.Pos()).Line,
				"exported": ast.IsExported(typ.Name.Name),
			}

			// Documentation
			if typ.Doc != nil {
				typeInfo["doc"] = strings.TrimSpace(typ.Doc.Text())
			}

			// Type kind
			switch t := typ.Type.(type) {
			case *ast.StructType:
				typeInfo["kind"] = "struct"
				typeInfo["fields"] = extractFieldList(t.Fields)
			case *ast.InterfaceType:
				typeInfo["kind"] = "interface"
				typeInfo["methods"] = extractFieldList(t.Methods)
			case *ast.ArrayType:
				typeInfo["kind"] = "array"
				typeInfo["element_type"] = extractType(t.Elt)
			case *ast.MapType:
				typeInfo["kind"] = "map"
				typeInfo["key_type"] = extractType(t.Key)
				typeInfo["value_type"] = extractType(t.Value)
			case *ast.Ident:
				typeInfo["kind"] = "alias"
				typeInfo["underlying"] = t.Name
			default:
				typeInfo["kind"] = "other"
			}

			types = append(types, typeInfo)
		}
		return true
	})

	return types
}

// extractConstants extracts constant declarations
func extractConstants(node *ast.File, fset *token.FileSet) []map[string]interface{} {
	var constants []map[string]interface{}

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range valueSpec.Names {
				constInfo := map[string]interface{}{
					"name":     name.Name,
					"line":     fset.Position(name.Pos()).Line,
					"exported": ast.IsExported(name.Name),
				}

				if valueSpec.Type != nil {
					constInfo["type"] = extractType(valueSpec.Type)
				}

				if i < len(valueSpec.Values) {
					constInfo["has_value"] = true
				}

				constants = append(constants, constInfo)
			}
		}
	}

	return constants
}

// extractVariables extracts variable declarations
func extractVariables(node *ast.File, fset *token.FileSet) []map[string]interface{} {
	var variables []map[string]interface{}

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for _, name := range valueSpec.Names {
				varInfo := map[string]interface{}{
					"name":     name.Name,
					"line":     fset.Position(name.Pos()).Line,
					"exported": ast.IsExported(name.Name),
				}

				if valueSpec.Type != nil {
					varInfo["type"] = extractType(valueSpec.Type)
				}

				variables = append(variables, varInfo)
			}
		}
	}

	return variables
}

// extractFieldList extracts field information from a field list
func extractFieldList(fields *ast.FieldList) []map[string]interface{} {
	if fields == nil {
		return nil
	}

	var result []map[string]interface{}
	for _, field := range fields.List {
		fieldInfo := map[string]interface{}{
			"type": extractType(field.Type),
		}

		// Field names (can be multiple: a, b int)
		if len(field.Names) > 0 {
			names := make([]string, len(field.Names))
			for i, name := range field.Names {
				names[i] = name.Name
			}
			fieldInfo["names"] = names
		}

		// Tags for struct fields
		if field.Tag != nil {
			fieldInfo["tag"] = field.Tag.Value
		}

		result = append(result, fieldInfo)
	}

	return result
}

// extractType extracts type information as a string
func extractType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + extractType(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + extractType(t.Elt)
		}
		return "[...]" + extractType(t.Elt)
	case *ast.MapType:
		return "map[" + extractType(t.Key) + "]" + extractType(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.SelectorExpr:
		return extractType(t.X) + "." + t.Sel.Name
	case *ast.ChanType:
		return "chan " + extractType(t.Value)
	default:
		return "unknown"
	}
}
