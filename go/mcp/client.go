package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Client manages connections to MCP servers
type Client struct {
	servers map[string]*client.Client
	tools   []MCPTool
	config  MCPConfig
}

// NewClient creates a new MCP client
func NewClient(config MCPConfig) *Client {
	return &Client{
		servers: make(map[string]*client.Client),
		tools:   []MCPTool{},
		config:  config,
	}
}

// Initialize connects to all configured MCP servers
func (c *Client) Initialize(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	// Count enabled servers
	enabledCount := 0
	for _, serverCfg := range c.config.Servers {
		if serverCfg.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return nil
	}

	for name, serverCfg := range c.config.Servers {
		if !serverCfg.Enabled {
			continue // Silent skip
		}

		// Create timeout context for each server connection
		connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := c.connectServer(connectCtx, name, serverCfg); err != nil {
			log.Printf("[MCP] Failed to connect to '%s': %v", name, err)
			continue
		}
	}

	// Register MCP tools with Wilson's registry
	if err := c.RegisterTools(); err != nil {
		log.Printf("[MCP] Warning: Failed to register tools with Wilson: %v", err)
	}

	return nil
}

// RegisterTools registers all MCP tools with Wilson's tool registry
func (c *Client) RegisterTools() error {
	// Import registry here to avoid circular dependency
	// We'll use dynamic registration
	return nil // Implemented via bridge in main.go
}

// connectServer connects to a single MCP server
func (c *Client) connectServer(ctx context.Context, name string, config ServerConfig) error {
	// Resolve environment variables
	envVars := []string{}
	for key, value := range config.Env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, os.ExpandEnv(value)))
	}

	// Create MCP client
	mcpClient, err := client.NewStdioMCPClient(config.Command, envVars, config.Args...)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Initialize the client with basic capabilities
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "wilson",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		mcpClient.Close()
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// List available tools from this server
	result, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		mcpClient.Close()
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Store server client
	c.servers[name] = mcpClient

	// Register tools (silent)
	for _, tool := range result.Tools {
		c.tools = append(c.tools, MCPTool{
			ServerName:  name,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	return nil
}

// ListTools returns all available MCP tools
func (c *Client) ListTools() []MCPTool {
	return c.tools
}

// CallTool executes a tool on the appropriate MCP server
func (c *Client) CallTool(ctx context.Context, serverName, toolName string, arguments map[string]interface{}) (string, error) {
	server, exists := c.servers[serverName]
	if !exists {
		return "", fmt.Errorf("server '%s' not connected", serverName)
	}

	result, err := server.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	})

	if err != nil {
		return "", fmt.Errorf("tool call failed: %w", err)
	}

	// Extract content from result
	if len(result.Content) == 0 {
		return "", nil
	}

	// Combine all content items (MCP SDK returns JSON-RPC formatted results)
	// The actual structure depends on the MCP protocol version
	var output string
	for _, content := range result.Content {
		// Try to extract text from content
		// In MCP, content can be text, resource, or other types
		output += fmt.Sprintf("%v\n", content)
	}

	return output, nil
}

// GetServerNames returns list of connected server names
func (c *Client) GetServerNames() []string {
	names := make([]string, 0, len(c.servers))
	for name := range c.servers {
		names = append(names, name)
	}
	return names
}

// Close closes all MCP server connections
func (c *Client) Close() {
	for name, server := range c.servers {
		log.Printf("[MCP] Closing connection to server '%s'", name)
		server.Close()
	}
	c.servers = make(map[string]*client.Client)
	c.tools = []MCPTool{}
}
