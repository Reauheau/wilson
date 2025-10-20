package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// ServerConfig represents configuration for an MCP server
type ServerConfig struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
	Enabled bool              `yaml:"enabled"`
}

// MCPConfig represents the MCP configuration section
type MCPConfig struct {
	Enabled bool                    `yaml:"enabled"`
	Servers map[string]ServerConfig `yaml:"servers"`
}

// MCPTool represents a tool exposed by an MCP server
type MCPTool struct {
	ServerName  string
	Name        string
	Description string
	InputSchema mcp.ToolInputSchema
}
