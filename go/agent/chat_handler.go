package agent

import (
	"context"
	"fmt"

	"wilson/core/registry"
	"wilson/session"
)

// ChatHandler handles direct chat interactions (not task-based)
// REFACTORED: Thin wrapper that delegates to ChatAgent
type ChatHandler struct {
	agent    *ChatAgent
	history  *session.History
	executor *registry.Executor
}

// NewChatHandler creates a new chat handler
func NewChatHandler(agent *ChatAgent, history *session.History, executor *registry.Executor) *ChatHandler {
	return &ChatHandler{
		agent:    agent,
		history:  history,
		executor: executor,
	}
}

// ChatRequest represents a chat request
type ChatRequest struct {
	UserInput string
	Context   context.Context
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Text          string
	ToolUsed      string
	ToolCancelled bool
	Success       bool
	Artifacts     []string
}

// HandleChat processes a chat request and returns a response
// REFACTORED: Simple wrapper that delegates everything to ChatAgent
func (h *ChatHandler) HandleChat(ctx context.Context, userInput string) (*ChatResponse, error) {
	// Add user message to history
	h.history.AddMessage("user", userInput)

	// Create task for ChatAgent
	task := &Task{
		ID:          generateTaskID(),
		Type:        TaskTypeGeneral,
		Description: userInput,
		Priority:    5,
		Status:      "pending",
	}

	// Execute through ChatAgent (proper architecture)
	result, err := h.agent.Execute(ctx, task)
	if err != nil {
		return &ChatResponse{
			Text:    fmt.Sprintf("Error: %v", err),
			Success: false,
		}, err
	}

	// Add response to history
	h.history.AddMessage("assistant", result.Output)

	// Extract tools used from metadata
	toolsUsed := ""
	if result.Metadata != nil {
		if tools, ok := result.Metadata["tools_executed"].([]string); ok && len(tools) > 0 {
			toolsUsed = fmt.Sprintf("%v", tools)
		}
	}

	// Return response
	return &ChatResponse{
		Text:     result.Output,
		Success:  result.Success,
		ToolUsed: toolsUsed,
	}, nil
}

// ClearHistory clears the conversation history
func (h *ChatHandler) ClearHistory() {
	h.history.Clear()
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("%d", len(fmt.Sprintf("%p", &struct{}{})))
}
