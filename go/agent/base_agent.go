package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/llm"
)

// BaseAgent provides common agent functionality
type BaseAgent struct {
	name         string
	purpose      llm.Purpose
	llmManager   *llm.Manager
	contextMgr   *contextpkg.Manager
	allowedTools []string
	canDelegate  bool
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(name string, purpose llm.Purpose, llmManager *llm.Manager, contextMgr *contextpkg.Manager) *BaseAgent {
	return &BaseAgent{
		name:       name,
		purpose:    purpose,
		llmManager: llmManager,
		contextMgr: contextMgr,
	}
}

// Name returns the agent name
func (a *BaseAgent) Name() string {
	return a.name
}

// Purpose returns the agent's LLM purpose
func (a *BaseAgent) Purpose() llm.Purpose {
	return a.purpose
}

// AllowedTools returns the list of allowed tools
func (a *BaseAgent) AllowedTools() []string {
	return a.allowedTools
}

// SetAllowedTools sets which tools this agent can use
func (a *BaseAgent) SetAllowedTools(tools []string) {
	a.allowedTools = tools
}

// SetCanDelegate sets whether this agent can delegate tasks
func (a *BaseAgent) SetCanDelegate(can bool) {
	a.canDelegate = can
}

// CanDelegate returns whether this agent can delegate
func (a *BaseAgent) CanDelegate() bool {
	return a.canDelegate
}

// IsToolAllowed checks if the agent can use a specific tool
func (a *BaseAgent) IsToolAllowed(toolName string) bool {
	// Empty list means all tools allowed
	if len(a.allowedTools) == 0 {
		return true
	}

	// Check if tool is in allowed list
	for _, allowed := range a.allowedTools {
		if allowed == "*" {
			return true
		}
		if allowed == toolName {
			return true
		}
		// Support wildcards like "search_*"
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(toolName, prefix) {
				return true
			}
		}
	}

	return false
}

// StoreArtifact stores an artifact in the current context
func (a *BaseAgent) StoreArtifact(artifactType, content, source string) (*contextpkg.Artifact, error) {
	if a.contextMgr == nil {
		return nil, fmt.Errorf("context manager not available")
	}

	req := contextpkg.StoreArtifactRequest{
		Type:    artifactType,
		Content: content,
		Source:  source,
		Agent:   a.name,
	}

	return a.contextMgr.StoreArtifact(req)
}

// LeaveNote leaves a note for another agent
func (a *BaseAgent) LeaveNote(toAgent, note string) error {
	if a.contextMgr == nil {
		return fmt.Errorf("context manager not available")
	}

	_, err := a.contextMgr.AddNote("", a.name, toAgent, note)
	return err
}

// GetContext retrieves the current context
func (a *BaseAgent) GetContext() (*contextpkg.Context, error) {
	if a.contextMgr == nil {
		return nil, fmt.Errorf("context manager not available")
	}

	contextKey := a.contextMgr.GetActiveContext()
	if contextKey == "" {
		return nil, fmt.Errorf("no active context")
	}

	return a.contextMgr.GetContext(contextKey)
}

// CallLLM calls the agent's LLM with a prompt
func (a *BaseAgent) CallLLM(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if a.llmManager == nil {
		return "", fmt.Errorf("LLM manager not available")
	}

	req := llm.Request{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	resp, err := a.llmManager.Generate(ctx, a.purpose, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}
