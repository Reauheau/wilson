package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"wilson/config"
)

// GetSafeWorkspace returns the configured safe workspace directory
func GetSafeWorkspace() string {
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
