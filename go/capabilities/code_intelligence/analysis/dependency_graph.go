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

// DependencyGraphTool analyzes import relationships between Go files/packages
type DependencyGraphTool struct{}

func init() {
	registry.Register(&DependencyGraphTool{})
}

func (t *DependencyGraphTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "dependency_graph",
		Description:     "Analyze import relationships and dependencies between Go packages/files. Shows import graph, circular dependencies, and transitive dependencies.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Path to package or file to analyze (default: current directory)",
				Example:     "agent",
			},
			{
				Name:        "depth",
				Type:        "number",
				Required:    false,
				Description: "How many levels deep to analyze (default: 2, max: 5)",
				Example:     "3",
			},
			{
				Name:        "include_external",
				Type:        "boolean",
				Required:    false,
				Description: "Include external (non-project) dependencies (default: false)",
				Example:     "true",
			},
		},
		Examples: []string{
			`{"tool": "dependency_graph", "arguments": {}}`,
			`{"tool": "dependency_graph", "arguments": {"path": "agent", "depth": 3}}`,
			`{"tool": "dependency_graph", "arguments": {"path": ".", "include_external": true}}`,
		},
	}
}

func (t *DependencyGraphTool) Validate(args map[string]interface{}) error {
	if depth, ok := args["depth"].(float64); ok {
		if depth < 1 || depth > 5 {
			return fmt.Errorf("depth must be between 1 and 5")
		}
	}
	return nil
}

func (t *DependencyGraphTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	// Get parameters
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	depth := 2
	if d, ok := input["depth"].(float64); ok {
		depth = int(d)
	}

	includeExternal := false
	if ie, ok := input["include_external"].(bool); ok {
		includeExternal = ie
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	// Build dependency graph
	graph := &DependencyGraph{
		Nodes:        make(map[string]*PackageNode),
		ProjectRoot:  getProjectRoot(absPath),
		IsFile:       !info.IsDir(),
	}

	if graph.IsFile {
		err = graph.analyzeFile(absPath, depth, includeExternal)
	} else {
		err = graph.analyzePackage(absPath, depth, includeExternal)
	}

	if err != nil {
		return "", fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	// Detect circular dependencies
	circularDeps := graph.detectCircularDependencies()

	// Build result
	result := map[string]interface{}{
		"path":              path,
		"project_root":      graph.ProjectRoot,
		"total_packages":    len(graph.Nodes),
		"direct_imports":    len(graph.getRootNode().Imports),
		"transitive_deps":   graph.countTransitiveDeps(),
		"circular_deps":     circularDeps,
		"has_circular":      len(circularDeps) > 0,
		"dependency_tree":   graph.buildTree(includeExternal),
		"import_graph":      graph.buildImportGraph(includeExternal),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// DependencyGraph represents the import relationship graph
type DependencyGraph struct {
	Nodes       map[string]*PackageNode
	ProjectRoot string
	IsFile      bool
	RootPath    string
}

// PackageNode represents a package in the dependency graph
type PackageNode struct {
	Path       string
	Name       string
	IsExternal bool
	Imports    []string
	ImportedBy []string
	Depth      int
}

// analyzeFile analyzes dependencies for a single file
func (g *DependencyGraph) analyzeFile(filePath string, maxDepth int, includeExternal bool) error {
	g.RootPath = filePath

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Create root node
	pkgPath := filepath.Dir(filePath)
	relPath, _ := filepath.Rel(g.ProjectRoot, pkgPath)
	g.Nodes[relPath] = &PackageNode{
		Path:    relPath,
		Name:    node.Name.Name,
		Depth:   0,
		Imports: []string{},
	}

	// Analyze imports
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		g.addDependency(relPath, importPath, 1, maxDepth, includeExternal)
	}

	return nil
}

// analyzePackage analyzes dependencies for an entire package
func (g *DependencyGraph) analyzePackage(pkgPath string, maxDepth int, includeExternal bool) error {
	g.RootPath = pkgPath

	relPath, _ := filepath.Rel(g.ProjectRoot, pkgPath)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("failed to parse package: %w", err)
	}

	// Get primary package name
	pkgName := ""
	allImports := make(map[string]bool)

	for name, pkg := range pkgs {
		if !strings.HasSuffix(name, "_test") {
			pkgName = name
		}

		// Collect all imports from all files
		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				allImports[importPath] = true
			}
		}
	}

	// Create root node
	g.Nodes[relPath] = &PackageNode{
		Path:    relPath,
		Name:    pkgName,
		Depth:   0,
		Imports: []string{},
	}

	// Analyze each import
	for importPath := range allImports {
		g.addDependency(relPath, importPath, 1, maxDepth, includeExternal)
	}

	return nil
}

// addDependency adds a dependency to the graph and recursively analyzes it
func (g *DependencyGraph) addDependency(fromPath, importPath string, currentDepth, maxDepth int, includeExternal bool) {
	if currentDepth > maxDepth {
		return
	}

	// Check if external
	isExternal := !strings.HasPrefix(importPath, "wilson/") && !strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "../")

	if isExternal && !includeExternal {
		return
	}

	// Resolve import path to file system path
	var targetPath string
	if strings.HasPrefix(importPath, "wilson/") {
		// Internal import
		targetPath = strings.TrimPrefix(importPath, "wilson/")
	} else {
		// External or relative
		targetPath = importPath
	}

	// Add to graph
	if _, exists := g.Nodes[targetPath]; !exists {
		g.Nodes[targetPath] = &PackageNode{
			Path:       targetPath,
			Name:       filepath.Base(targetPath),
			IsExternal: isExternal,
			Depth:      currentDepth,
			Imports:    []string{},
			ImportedBy: []string{},
		}
	}

	// Add edge
	fromNode := g.Nodes[fromPath]
	toNode := g.Nodes[targetPath]

	fromNode.Imports = append(fromNode.Imports, targetPath)
	toNode.ImportedBy = append(toNode.ImportedBy, fromPath)

	// Recursively analyze if internal and not at max depth
	if !isExternal && currentDepth < maxDepth {
		absTargetPath := filepath.Join(g.ProjectRoot, targetPath)
		if info, err := os.Stat(absTargetPath); err == nil && info.IsDir() {
			fset := token.NewFileSet()
			pkgs, err := parser.ParseDir(fset, absTargetPath, nil, parser.ImportsOnly)
			if err == nil {
				for _, pkg := range pkgs {
					for _, file := range pkg.Files {
						for _, imp := range file.Imports {
							nextImport := strings.Trim(imp.Path.Value, `"`)
							g.addDependency(targetPath, nextImport, currentDepth+1, maxDepth, includeExternal)
						}
					}
				}
			}
		}
	}
}

// getRootNode returns the root node of the graph
func (g *DependencyGraph) getRootNode() *PackageNode {
	relPath, _ := filepath.Rel(g.ProjectRoot, g.RootPath)
	if g.IsFile {
		relPath = filepath.Dir(relPath)
	}
	return g.Nodes[relPath]
}

// countTransitiveDeps counts total transitive dependencies
func (g *DependencyGraph) countTransitiveDeps() int {
	visited := make(map[string]bool)
	g.countTransitiveHelper(g.getRootNode().Path, visited)
	return len(visited) - 1 // Exclude root itself
}

func (g *DependencyGraph) countTransitiveHelper(path string, visited map[string]bool) {
	if visited[path] {
		return
	}
	visited[path] = true

	node := g.Nodes[path]
	for _, imp := range node.Imports {
		g.countTransitiveHelper(imp, visited)
	}
}

// detectCircularDependencies finds circular import cycles
func (g *DependencyGraph) detectCircularDependencies() [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for path := range g.Nodes {
		if !visited[path] {
			g.detectCyclesHelper(path, visited, recStack, []string{}, &cycles)
		}
	}

	return cycles
}

func (g *DependencyGraph) detectCyclesHelper(path string, visited, recStack map[string]bool, currentPath []string, cycles *[][]string) {
	visited[path] = true
	recStack[path] = true
	currentPath = append(currentPath, path)

	node := g.Nodes[path]
	for _, imp := range node.Imports {
		if !visited[imp] {
			g.detectCyclesHelper(imp, visited, recStack, currentPath, cycles)
		} else if recStack[imp] {
			// Found cycle
			cycleStart := -1
			for i, p := range currentPath {
				if p == imp {
					cycleStart = i
					break
				}
			}
			if cycleStart != -1 {
				cycle := append([]string{}, currentPath[cycleStart:]...)
				cycle = append(cycle, imp)
				*cycles = append(*cycles, cycle)
			}
		}
	}

	recStack[path] = false
}

// buildTree builds a hierarchical tree view of dependencies
func (g *DependencyGraph) buildTree(includeExternal bool) map[string]interface{} {
	root := g.getRootNode()
	visited := make(map[string]bool)
	return g.buildTreeHelper(root.Path, 0, visited, includeExternal)
}

func (g *DependencyGraph) buildTreeHelper(path string, level int, visited map[string]bool, includeExternal bool) map[string]interface{} {
	node := g.Nodes[path]

	tree := map[string]interface{}{
		"package":     node.Name,
		"path":        node.Path,
		"is_external": node.IsExternal,
		"level":       level,
	}

	if visited[path] {
		tree["circular"] = true
		return tree
	}

	visited[path] = true

	if len(node.Imports) > 0 {
		children := []map[string]interface{}{}
		for _, imp := range node.Imports {
			if impNode, exists := g.Nodes[imp]; exists {
				if !impNode.IsExternal || includeExternal {
					child := g.buildTreeHelper(imp, level+1, visited, includeExternal)
					children = append(children, child)
				}
			}
		}
		if len(children) > 0 {
			tree["imports"] = children
		}
	}

	delete(visited, path)
	return tree
}

// buildImportGraph builds a flat import graph (for visualization)
func (g *DependencyGraph) buildImportGraph(includeExternal bool) map[string]interface{} {
	nodes := []map[string]interface{}{}
	edges := []map[string]interface{}{}

	for path, node := range g.Nodes {
		if !node.IsExternal || includeExternal {
			nodes = append(nodes, map[string]interface{}{
				"id":          path,
				"label":       node.Name,
				"is_external": node.IsExternal,
				"depth":       node.Depth,
			})

			for _, imp := range node.Imports {
				if impNode, exists := g.Nodes[imp]; exists {
					if !impNode.IsExternal || includeExternal {
						edges = append(edges, map[string]interface{}{
							"from": path,
							"to":   imp,
						})
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
	}
}

// getProjectRoot finds the project root by looking for go.mod
func getProjectRoot(startPath string) string {
	dir := startPath
	if info, err := os.Stat(startPath); err == nil && !info.IsDir() {
		dir = filepath.Dir(startPath)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	// Fallback to current directory
	return startPath
}
