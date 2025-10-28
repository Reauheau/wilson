package setup

import (
	"context"
	"fmt"

	"wilson/agent/agents"
	"wilson/config"
	contextpkg "wilson/context"
	"wilson/llm"
	"wilson/lsp"
	"wilson/mcp"

	code_intelligence "wilson/capabilities/code_intelligence"
)

// Bootstrap contains all initialized components
type Bootstrap struct {
	LLMManager     *llm.Manager
	LSPManager     *lsp.Manager
	ContextManager *contextpkg.Manager
	MCPClient      *mcp.Client
	ChatAgent      *agents.ChatAgent
}

// Initialize bootstraps the entire Wilson system
func Initialize(ctx context.Context, cfg *config.Config) (*Bootstrap, error) {
	b := &Bootstrap{}

	// Initialize LLM Manager
	b.LLMManager = InitializeLLMManager(ctx, cfg)
	if b.LLMManager == nil {
		fmt.Println("Warning: Failed to initialize LLM manager, some features may not work")
	}

	// Initialize LSP Manager for code intelligence
	b.LSPManager = lsp.NewManager()
	code_intelligence.SetLSPManager(b.LSPManager)
	lsp.SetGlobalManager(b.LSPManager) // Enable LSP restart on project change

	// Initialize Context Store
	b.ContextManager = InitializeContextManager(cfg)

	// Initialize MCP Client
	b.MCPClient = InitializeMCPClient(ctx, cfg)

	// Initialize Agent System
	b.ChatAgent = InitializeAgentSystem(b.LLMManager, b.ContextManager)

	return b, nil
}

// Cleanup gracefully shuts down all components
func (b *Bootstrap) Cleanup() {
	if b.LSPManager != nil {
		b.LSPManager.StopAll()
	}
	if b.ContextManager != nil {
		b.ContextManager.Close()
	}
	if b.MCPClient != nil {
		b.MCPClient.Close()
	}
}
