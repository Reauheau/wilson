package system

import "wilson/config"

// GetSafeWorkspace returns the configured safe workspace directory
func GetSafeWorkspace() string {
	return config.GetWorkspacePath()
}

// SafeWorkspace is kept for backwards compatibility
var SafeWorkspace = config.GetWorkspacePath()
