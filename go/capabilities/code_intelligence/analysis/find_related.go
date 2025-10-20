package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// FindRelatedTool finds files related to a given file or symbol
type FindRelatedTool struct{}

func init() {
	registry.Register(&FindRelatedTool{})
}

func (t *FindRelatedTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "find_related",
		Description:     "Find files related to a given file or symbol. Shows files that import this file, files that this file imports, test files, and files with related functionality.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "File or package path to find related files for",
				Example:     "agent/code_agent.go",
			},
			{
				Name:        "include_tests",
				Type:        "boolean",
				Required:    false,
				Description: "Include test files in results (default: true)",
				Example:     "true",
			},
			{
				Name:        "search_depth",
				Type:        "number",
				Required:    false,
				Description: "How far to search in the project tree (default: all)",
				Example:     "3",
			},
		},
		Examples: []string{
			`{"tool": "find_related", "arguments": {"path": "agent/code_agent.go"}}`,
			`{"tool": "find_related", "arguments": {"path": "agent", "include_tests": false}}`,
		},
	}
}

func (t *FindRelatedTool) Validate(args map[string]interface{}) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("path parameter is required")
	}
	return nil
}

func (t *FindRelatedTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)

	includeTests := true
	if it, ok := input["include_tests"].(bool); ok {
		includeTests = it
	}

	searchDepth := -1 // unlimited
	if sd, ok := input["search_depth"].(float64); ok {
		searchDepth = int(sd)
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	// Get project root
	projectRoot := getProjectRoot(absPath)

	// Initialize result
	related := &RelatedFiles{
		TargetPath:  path,
		IsDirectory: info.IsDir(),
		ProjectRoot: projectRoot,
	}

	// Find different types of related files
	if info.IsDir() {
		err = related.findRelatedToPackage(absPath, projectRoot, includeTests, searchDepth)
	} else {
		err = related.findRelatedToFile(absPath, projectRoot, includeTests, searchDepth)
	}

	if err != nil {
		return "", fmt.Errorf("failed to find related files: %w", err)
	}

	// Build result
	result := map[string]interface{}{
		"target":           path,
		"is_directory":     info.IsDir(),
		"imported_by":      related.ImportedBy,
		"imports":          related.Imports,
		"test_files":       related.TestFiles,
		"same_package":     related.SamePackage,
		"related_packages": related.RelatedPackages,
		"total_related":    len(related.ImportedBy) + len(related.Imports) + len(related.TestFiles) + len(related.SamePackage),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// RelatedFiles holds information about files related to a target
type RelatedFiles struct {
	TargetPath      string
	IsDirectory     bool
	ProjectRoot     string
	ImportedBy      []string // Files that import this file/package
	Imports         []string // Files that this file/package imports
	TestFiles       []string // Test files for this file/package
	SamePackage     []string // Other files in the same package
	RelatedPackages []string // Packages with similar names or functionality
}

// findRelatedToFile finds files related to a specific file
func (r *RelatedFiles) findRelatedToFile(filePath, projectRoot string, includeTests bool, searchDepth int) error {
	// Parse the target file to get its package and imports
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	targetDir := filepath.Dir(filePath)

	// Get target's imports
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Resolve to file path
		if strings.HasPrefix(importPath, "wilson/") {
			localPath := strings.TrimPrefix(importPath, "wilson/")
			fullPath := filepath.Join(projectRoot, localPath)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				relPath, _ := filepath.Rel(projectRoot, fullPath)
				r.Imports = append(r.Imports, relPath)
			}
		}
	}

	// Find test file for this file
	if includeTests {
		testFile := strings.TrimSuffix(filePath, ".go") + "_test.go"
		if _, err := os.Stat(testFile); err == nil {
			relPath, _ := filepath.Rel(projectRoot, testFile)
			r.TestFiles = append(r.TestFiles, relPath)
		}
	}

	// Find other files in the same package
	files, err := filepath.Glob(filepath.Join(targetDir, "*.go"))
	if err == nil {
		for _, file := range files {
			if file != filePath && (!strings.HasSuffix(file, "_test.go") || includeTests) {
				relPath, _ := filepath.Rel(projectRoot, file)
				r.SamePackage = append(r.SamePackage, relPath)
			}
		}
	}

	// Search the entire project for files that import this file's package
	relTargetPath, _ := filepath.Rel(projectRoot, filePath)
	targetPkgPath := filepath.Dir(relTargetPath)
	projectImportPath := "wilson/" + targetPkgPath

	err = r.searchForImporters(projectRoot, projectImportPath, projectRoot, 0, searchDepth)
	if err != nil {
		return err
	}

	return nil
}

// findRelatedToPackage finds files related to a package
func (r *RelatedFiles) findRelatedToPackage(pkgPath, projectRoot string, includeTests bool, searchDepth int) error {
	// Get package's imports
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("failed to parse package: %w", err)
	}

	allImports := make(map[string]bool)
	pkgName := ""

	for name, pkg := range pkgs {
		if !strings.HasSuffix(name, "_test") {
			pkgName = name
		}

		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				allImports[importPath] = true
			}
		}
	}

	// Resolve imports to file paths
	for importPath := range allImports {
		if strings.HasPrefix(importPath, "wilson/") {
			localPath := strings.TrimPrefix(importPath, "wilson/")
			fullPath := filepath.Join(projectRoot, localPath)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				relPath, _ := filepath.Rel(projectRoot, fullPath)
				r.Imports = append(r.Imports, relPath)
			}
		}
	}

	// Find test files in this package
	if includeTests {
		files, err := filepath.Glob(filepath.Join(pkgPath, "*_test.go"))
		if err == nil {
			for _, file := range files {
				relPath, _ := filepath.Rel(projectRoot, file)
				r.TestFiles = append(r.TestFiles, relPath)
			}
		}
	}

	// Find all files in the package
	files, err := filepath.Glob(filepath.Join(pkgPath, "*.go"))
	if err == nil {
		for _, file := range files {
			if !strings.HasSuffix(file, "_test.go") || includeTests {
				relPath, _ := filepath.Rel(projectRoot, file)
				r.SamePackage = append(r.SamePackage, relPath)
			}
		}
	}

	// Search for packages that import this package
	relPkgPath, _ := filepath.Rel(projectRoot, pkgPath)
	projectImportPath := "wilson/" + relPkgPath

	err = r.searchForImporters(projectRoot, projectImportPath, projectRoot, 0, searchDepth)
	if err != nil {
		return err
	}

	// Find related packages (packages with similar names)
	r.findRelatedPackages(pkgPath, pkgName, projectRoot)

	return nil
}

// searchForImporters recursively searches for files that import the target
func (r *RelatedFiles) searchForImporters(searchPath, targetImport, projectRoot string, currentDepth, maxDepth int) error {
	if maxDepth >= 0 && currentDepth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(searchPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(searchPath, entry.Name())

		// Skip hidden directories and vendor
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "node_modules" {
				continue
			}
			// Recurse into directory
			r.searchForImporters(fullPath, targetImport, projectRoot, currentDepth+1, maxDepth)
			continue
		}

		// Only check .go files
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		// Parse file and check imports
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
		if err != nil {
			continue
		}

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if importPath == targetImport || strings.HasPrefix(targetImport, importPath) {
				relPath, _ := filepath.Rel(projectRoot, fullPath)
				r.ImportedBy = append(r.ImportedBy, relPath)
				break
			}
		}
	}

	return nil
}

// findRelatedPackages finds packages with similar names or in related directories
func (r *RelatedFiles) findRelatedPackages(pkgPath, pkgName, projectRoot string) {
	parentDir := filepath.Dir(pkgPath)

	// Find sibling packages
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(parentDir, entry.Name())
		if fullPath == pkgPath {
			continue
		}

		// Check if it's a Go package
		files, err := filepath.Glob(filepath.Join(fullPath, "*.go"))
		if err != nil || len(files) == 0 {
			continue
		}

		// Check if the names are similar
		if strings.Contains(entry.Name(), pkgName) || strings.Contains(pkgName, entry.Name()) {
			relPath, _ := filepath.Rel(projectRoot, fullPath)
			r.RelatedPackages = append(r.RelatedPackages, relPath)
		}
	}
}
