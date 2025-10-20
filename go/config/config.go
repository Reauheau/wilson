package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var globalConfig *Config

// Load reads the configuration file
func Load(configPath string) (*Config, error) {
	// If path is empty, use default
	if configPath == "" {
		configPath = "config/tools.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Workspace.Path == "" {
		cfg.Workspace.Path = getDefaultWorkspacePath()
	} else {
		// Expand ~ to user home directory if present
		cfg.Workspace.Path = expandHomePath(cfg.Workspace.Path)
	}

	if cfg.Ollama.Model == "" {
		cfg.Ollama.Model = "llama3:latest"
	}

	if cfg.Ollama.URL == "" {
		cfg.Ollama.URL = "http://localhost:11434"
	}

	if cfg.Audit.LogPath == "" {
		cfg.Audit.LogPath = ".wilson/audit.log"
	}

	if cfg.Audit.LogLevel == "" {
		cfg.Audit.LogLevel = "info"
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get returns the global configuration
func Get() *Config {
	if globalConfig == nil {
		// Return default config if not loaded
		return &Config{
			Workspace: WorkspaceConfig{
				Path: getDefaultWorkspacePath(),
			},
			Ollama: OllamaConfig{
				Model: "llama3:latest",
				URL:   "http://localhost:11434",
			},
			Audit: AuditConfig{
				Enabled:  true,
				LogPath:  ".wilson/audit.log",
				LogLevel: "info",
			},
		}
	}
	return globalConfig
}

// GetToolConfig returns configuration for a specific tool
func GetToolConfig(toolName string) *ToolConfig {
	cfg := Get()

	if toolCfg, exists := cfg.Tools.Tools[toolName]; exists {
		return &toolCfg
	}

	// Return default config
	return &ToolConfig{
		Enabled: true,
	}
}

// IsToolEnabled checks if a tool is enabled in config
func IsToolEnabled(toolName string) bool {
	toolCfg := GetToolConfig(toolName)
	return toolCfg.Enabled
}

// ShouldConfirm checks if a tool requires confirmation
// Returns nil if config doesn't override, otherwise returns the configured value
func ShouldConfirm(toolName string) *bool {
	toolCfg := GetToolConfig(toolName)
	return toolCfg.RequiresConfirm
}

// GetWorkspacePath returns the configured workspace path
func GetWorkspacePath() string {
	return Get().Workspace.Path
}

// GetAuditLogPath returns the full path to audit log
func GetAuditLogPath() string {
	cfg := Get()
	logPath := cfg.Audit.LogPath

	// If relative path, make it relative to workspace
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(cfg.Workspace.Path, logPath)
	}

	return logPath
}

// IsAuditEnabled checks if audit logging is enabled
func IsAuditEnabled() bool {
	return Get().Audit.Enabled
}

// getDefaultWorkspacePath returns the default workspace path
// Priority: WILSON_WORKSPACE env var > user home directory
func getDefaultWorkspacePath() string {
	// Check for environment variable first
	if workspacePath := os.Getenv("WILSON_WORKSPACE"); workspacePath != "" {
		return expandHomePath(workspacePath)
	}

	// Fall back to user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, use current working directory
		cwd, err := os.Getwd()
		if err != nil {
			// Last resort fallback
			return "."
		}
		return cwd
	}

	return homeDir
}

// expandHomePath expands ~ to the user's home directory
func expandHomePath(path string) string {
	if len(path) == 0 {
		return path
	}

	// Handle ~ at the beginning of the path
	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path // Return unchanged if we can't get home dir
		}

		if len(path) == 1 {
			return homeDir
		}

		// Handle ~/something
		if path[1] == '/' || path[1] == filepath.Separator {
			return filepath.Join(homeDir, path[2:])
		}
	}

	return path
}
