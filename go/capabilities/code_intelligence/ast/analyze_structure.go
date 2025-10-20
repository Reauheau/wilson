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

	"wilson/core/registry"
	. "wilson/core/types"
)

// AnalyzeStructureTool analyzes code structure of a file or package
type AnalyzeStructureTool struct{}

func init() {
	registry.Register(&AnalyzeStructureTool{})
}

func (t *AnalyzeStructureTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "analyze_structure",
		Description:     "Analyze the structure of a Go file or package: exported API, dependencies, package organization.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Path to file or directory to analyze",
				Example:     "agent",
			},
		},
		Examples: []string{
			`{"tool": "analyze_structure", "arguments": {"path": "agent"}}`,
		},
	}
}

func (t *AnalyzeStructureTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	return nil
}

func (t *AnalyzeStructureTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	var result map[string]interface{}

	if info.IsDir() {
		result, err = analyzePackage(absPath)
	} else {
		result, err = analyzeSingleFile(absPath)
	}

	if err != nil {
		return "", err
	}

	result["path"] = path

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

func analyzePackage(dir string) (map[string]interface{}, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package: %w", err)
	}

	result := map[string]interface{}{
		"type":     "package",
		"packages": make([]map[string]interface{}, 0),
	}

	for pkgName, pkg := range pkgs {
		pkgInfo := map[string]interface{}{
			"name":              pkgName,
			"files":             len(pkg.Files),
			"exported_functions": 0,
			"exported_types":     0,
			"total_functions":    0,
			"total_types":        0,
		}

		exportedFuncs := []string{}
		exportedTypes := []string{}

		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch decl := n.(type) {
				case *ast.FuncDecl:
					pkgInfo["total_functions"] = pkgInfo["total_functions"].(int) + 1
					if ast.IsExported(decl.Name.Name) {
						pkgInfo["exported_functions"] = pkgInfo["exported_functions"].(int) + 1
						exportedFuncs = append(exportedFuncs, decl.Name.Name)
					}
				case *ast.TypeSpec:
					pkgInfo["total_types"] = pkgInfo["total_types"].(int) + 1
					if ast.IsExported(decl.Name.Name) {
						pkgInfo["exported_types"] = pkgInfo["exported_types"].(int) + 1
						exportedTypes = append(exportedTypes, decl.Name.Name)
					}
				}
				return true
			})
		}

		pkgInfo["exported_function_names"] = exportedFuncs
		pkgInfo["exported_type_names"] = exportedTypes

		result["packages"] = append(result["packages"].([]map[string]interface{}), pkgInfo)
	}

	return result, nil
}

func analyzeSingleFile(filePath string) (map[string]interface{}, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	result := map[string]interface{}{
		"type":               "file",
		"package":            node.Name.Name,
		"imports":            len(node.Imports),
		"functions":          0,
		"exported_functions": 0,
		"types":              0,
		"exported_types":     0,
	}

	exportedAPI := []string{}

	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			result["functions"] = result["functions"].(int) + 1
			if ast.IsExported(decl.Name.Name) {
				result["exported_functions"] = result["exported_functions"].(int) + 1
				funcSig := decl.Name.Name + "("
				if decl.Recv != nil {
					funcSig = "method " + funcSig
				} else {
					funcSig = "func " + funcSig
				}
				exportedAPI = append(exportedAPI, funcSig+")")
			}
		case *ast.TypeSpec:
			result["types"] = result["types"].(int) + 1
			if ast.IsExported(decl.Name.Name) {
				result["exported_types"] = result["exported_types"].(int) + 1
				exportedAPI = append(exportedAPI, "type "+decl.Name.Name)
			}
		}
		return true
	})

	result["exported_api"] = exportedAPI

	return result, nil
}
