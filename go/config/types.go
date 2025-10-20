package config

import (
	"time"
)

// Config represents the application configuration
type Config struct {
	Workspace WorkspaceConfig        `yaml:"workspace"`
	Ollama    OllamaConfig           `yaml:"ollama"` // Deprecated: Use LLMs instead
	LLMs      map[string]LLMConfig   `yaml:"llms"`
	Tools     ToolsConfig            `yaml:"tools"`
	Audit     AuditConfig            `yaml:"audit"`
	Context   ContextConfig          `yaml:"context"`
}

// WorkspaceConfig defines the safe workspace directory
type WorkspaceConfig struct {
	Path string `yaml:"path"`
}

// OllamaConfig defines Ollama-specific settings (deprecated)
type OllamaConfig struct {
	Model string `yaml:"model"`
	URL   string `yaml:"url"`
}

// LLMConfig defines settings for a specific LLM instance
type LLMConfig struct {
	Provider     string         `yaml:"provider"`
	Model        string         `yaml:"model"`
	Temperature  float64        `yaml:"temperature"`
	BaseURL      string         `yaml:"base_url,omitempty"`
	APIKey       string         `yaml:"api_key,omitempty"`
	Fallback     string         `yaml:"fallback,omitempty"`
	Options      map[string]any `yaml:"options,omitempty"`
	KeepAlive    bool           `yaml:"keep_alive"`     // Never unload model (for Wilson's chat model)
	IdleTimeout  int            `yaml:"idle_timeout"`   // Seconds before unloading (0 = immediate)
}

// ToolsConfig contains settings for all tools
type ToolsConfig struct {
	Tools map[string]ToolConfig `yaml:"tools"`
}

// ToolConfig represents configuration for a single tool
type ToolConfig struct {
	Enabled         bool              `yaml:"enabled"`
	RequiresConfirm *bool             `yaml:"requires_confirm,omitempty"` // Override default
	LLM             string            `yaml:"llm,omitempty"`              // Which LLM to use (chat/analysis/code)
	MaxFileSize     *int              `yaml:"max_file_size,omitempty"`
	MaxResults      *int              `yaml:"max_results,omitempty"`
	MaxContentLen   *int              `yaml:"max_content_length,omitempty"`
	Timeout         *time.Duration    `yaml:"timeout,omitempty"`
	BlockedPatterns []string          `yaml:"blocked_patterns,omitempty"`
	AllowedDomains  []string          `yaml:"allowed_domains,omitempty"`
	Prompts         map[string]string `yaml:"prompts,omitempty"`          // LLM prompts for different modes
	Extra           map[string]string `yaml:"extra,omitempty"`            // Tool-specific settings
}

// AuditConfig defines audit logging settings
type AuditConfig struct {
	Enabled  bool   `yaml:"enabled"`
	LogPath  string `yaml:"log_path"`
	LogLevel string `yaml:"log_level"` // info, warning, error
}

// ContextConfig defines context store settings
type ContextConfig struct {
	Enabled        bool   `yaml:"enabled"`
	DBPath         string `yaml:"db_path"`
	AutoStore      bool   `yaml:"auto_store"`       // Automatically store tool results
	DefaultContext string `yaml:"default_context"`  // Default context key
}
