package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	. "wilson/core/types"
)

// MCPToolBridge adapts an MCP tool to Wilson's Tool interface
type MCPToolBridge struct {
	client     *Client
	serverName string
	tool       MCPTool
}

// NewMCPToolBridge creates a new bridge for an MCP tool
func NewMCPToolBridge(client *Client, serverName string, tool MCPTool) *MCPToolBridge {
	return &MCPToolBridge{
		client:     client,
		serverName: serverName,
		tool:       tool,
	}
}

// Metadata returns the tool metadata in Wilson's format
func (b *MCPToolBridge) Metadata() ToolMetadata {
	// Convert MCP tool schema to Wilson parameters
	// MCP uses JSON Schema, but for now we'll keep it simple
	// and just indicate the tool takes flexible arguments
	params := []Parameter{
		{
			Name:        "arguments",
			Type:        "object",
			Required:    false,
			Description: "Tool-specific arguments (see MCP server documentation)",
		},
	}

	return ToolMetadata{
		Name:            fmt.Sprintf("mcp_%s_%s", b.serverName, b.tool.Name),
		Description:     fmt.Sprintf("[MCP:%s] %s", b.serverName, b.tool.Description),
		Category:        "mcp",        // New category for MCP tools
		RiskLevel:       RiskModerate, // MCP tools can modify external state
		RequiresConfirm: false,
		Enabled:         true,
		Parameters:      params,
		Examples: []string{
			fmt.Sprintf(`{"tool": "mcp_%s_%s", "arguments": {...}}`, b.serverName, b.tool.Name),
		},
	}
}

// Validate validates the tool arguments
func (b *MCPToolBridge) Validate(_ map[string]interface{}) error {
	// MCP servers handle their own validation
	// We just do basic sanity checking
	return nil
}

// Execute executes the MCP tool
func (b *MCPToolBridge) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Call the MCP server
	result, err := b.client.CallTool(ctx, b.serverName, b.tool.Name, args)
	if err != nil {
		return "", fmt.Errorf("MCP tool execution failed: %w", err)
	}

	// Try to format as JSON for better display
	var jsonResult interface{}
	if err := json.Unmarshal([]byte(result), &jsonResult); err == nil {
		formatted, err := json.MarshalIndent(jsonResult, "", "  ")
		if err == nil {
			return string(formatted), nil
		}
	}

	return result, nil
}
