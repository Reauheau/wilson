package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"wilson/config"
)

// currentProjectPath holds the active project path for this execution
// Set by agents before tool execution to override default workspace
var (
	currentProjectPath string
	projectPathMu      sync.RWMutex
)

// SetProjectPath sets the project path for current execution
// This overrides the default workspace for file operations
func SetProjectPath(path string) {
	projectPathMu.Lock()
	defer projectPathMu.Unlock()
	currentProjectPath = path
}

// GetProjectPath returns the current project path override (if set)
func GetProjectPath() string {
	projectPathMu.RLock()
	defer projectPathMu.RUnlock()
	return currentProjectPath
}

// ClearProjectPath clears the project path override
func ClearProjectPath() {
	projectPathMu.Lock()
	defer projectPathMu.Unlock()
	currentProjectPath = ""
}

// GetSafeWorkspace returns the configured safe workspace directory
// If a project path override is set, returns that instead
func GetSafeWorkspace() string {
	projectPathMu.RLock()
	override := currentProjectPath
	projectPathMu.RUnlock()

	if override != "" {
		return override
	}
	return config.GetWorkspacePath()
}

// SafeWorkspace is kept for backwards compatibility
var SafeWorkspace = config.GetWorkspacePath()

// ResolvePath resolves a path relative to workspace, handling tilde and absolute paths
// Returns the resolved full path
func ResolvePath(pathArg string, workspace string) string {
	// Handle tilde expansion
	if strings.HasPrefix(pathArg, "~") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			if pathArg == "~" {
				pathArg = homeDir
			} else if strings.HasPrefix(pathArg, "~/") {
				pathArg = filepath.Join(homeDir, pathArg[2:])
			}
		}
	}

	// Determine full path: absolute paths used as-is, relative paths joined with workspace
	var fullPath string
	if filepath.IsAbs(pathArg) {
		fullPath = pathArg
	} else {
		fullPath = filepath.Join(workspace, pathArg)
	}

	// Clean the path
	return filepath.Clean(fullPath)
}
