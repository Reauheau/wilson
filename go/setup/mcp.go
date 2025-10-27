package setup

import (
	"context"
	"fmt"
	"strings"

	"wilson/config"
	"wilson/core/registry"
	"wilson/mcp"
)

// InitializeMCPClient creates and initializes the MCP client
func InitializeMCPClient(ctx context.Context, cfg *config.Config) *mcp.Client {
	if !cfg.MCP.Enabled {
		return nil
	}

	client := mcp.NewClient(cfg.MCP)
	if err := client.Initialize(ctx); err != nil {
		fmt.Printf("Warning: Failed to initialize MCP client: %v\n", err)
		return nil
	}

	// List available tools
	tools := client.ListTools()
	if len(tools) > 0 {
		// Register MCP tools with Wilson's tool registry
		registerMCPTools(client)

		// Show clean summary
		serverNames := client.GetServerNames()
		if len(serverNames) > 0 {
			fmt.Printf("[MCP] Active: %s (%d tools)\n", strings.Join(serverNames, ", "), len(tools))
		}
	}

	return client
}

// registerMCPTools registers all MCP tools with Wilson's registry
func registerMCPTools(client *mcp.Client) {
	tools := client.ListTools()

	for _, mcpTool := range tools {
		// Create a bridge for each MCP tool
		bridge := mcp.NewMCPToolBridge(client, mcpTool.ServerName, mcpTool)

		// Register with Wilson's tool registry (silent)
		registry.Register(bridge)
	}
}
