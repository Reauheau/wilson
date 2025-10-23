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
	name           string
	purpose        llm.Purpose
	llmManager     *llm.Manager
	contextMgr     *contextpkg.Manager
	allowedTools   []string
	canDelegate    bool
	currentTaskID  string       // For feedback
	currentContext *TaskContext // Full context for rich feedback
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

// CallLLM calls the agent's LLM with a prompt using validation and retry
// This ensures ALL agents get reliable JSON responses with automatic correction
func (a *BaseAgent) CallLLM(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if a.llmManager == nil {
		return "", fmt.Errorf("LLM manager not available")
	}

	// Use validation with retry for reliable JSON generation
	// taskID is empty here since we don't have access to it in BaseAgent
	return CallLLMWithValidation(ctx, a.llmManager, a.purpose, systemPrompt, userPrompt, 5, "")
}

// SetTaskContext stores the TaskContext for feedback access
// Called by concrete agents in their ExecuteWithContext implementations
func (a *BaseAgent) SetTaskContext(taskCtx *TaskContext) {
	a.currentTaskID = taskCtx.TaskID
	a.currentContext = taskCtx
}

// ConvertTaskContextToTask converts TaskContext to old Task format
// Helper for concrete agents during transition period
func (a *BaseAgent) ConvertTaskContextToTask(taskCtx *TaskContext) *Task {
	return &Task{
		ID:          taskCtx.TaskID,
		Type:        string(taskCtx.Type),
		Description: taskCtx.Description,
		Input:       taskCtx.Input,
		Priority:    taskCtx.Priority,
		Status:      TaskPending,
	}
}

// SendFeedback sends feedback via the feedback bus with full TaskContext
func (a *BaseAgent) SendFeedback(ctx context.Context, feedbackType FeedbackType,
	severity FeedbackSeverity, message string,
	context map[string]interface{}, suggestion string) error {

	bus := GetFeedbackBus()

	feedback := &AgentFeedback{
		TaskID:       a.currentTaskID,
		AgentName:    a.name,
		FeedbackType: feedbackType,
		Severity:     severity,
		Message:      message,
		Context:      context,
		Suggestion:   suggestion,
		TaskContext:  a.currentContext, // ✅ Full execution context!
	}

	return bus.Send(feedback)
}

// RequestDependency requests a missing dependency with error context
// This method sends feedback to create a dependency task and returns an error to block the current task
func (a *BaseAgent) RequestDependency(ctx context.Context, description string,
	taskType ManagedTaskType, reason string) error {

	// Include error history in context
	errorInfo := make(map[string]interface{})
	if a.currentContext != nil {
		errorInfo["previous_attempts"] = a.currentContext.PreviousAttempts
		errorInfo["error_patterns"] = a.currentContext.GetErrorPatterns()
		if lastErr := a.currentContext.GetLastError(); lastErr != nil {
			errorInfo["last_error_type"] = lastErr.ErrorType
			errorInfo["last_error_message"] = lastErr.Message
		}
	}

	errorInfo["dependency_description"] = description
	errorInfo["dependency_type"] = string(taskType)
	errorInfo["reason"] = reason

	// Send feedback to manager
	feedbackErr := a.SendFeedback(ctx,
		FeedbackTypeDependencyNeeded,
		FeedbackSeverityCritical,
		fmt.Sprintf("Cannot proceed: %s", reason),
		errorInfo,
		"Create and complete the missing dependency before retrying this task",
	)

	// If feedback failed to send, return that error
	if feedbackErr != nil {
		return feedbackErr
	}

	// Always return an error to block the current task
	// The task will be unblocked after the dependency completes
	return fmt.Errorf("dependency needed: %s", reason)
}

// RecordError records an error in TaskContext for learning
func (a *BaseAgent) RecordError(errorType, phase, message, filePath string,
	lineNumber int, suggestion string) {

	if a.currentContext == nil {
		return
	}

	execError := ExecutionError{
		Timestamp:  a.currentContext.CreatedAt,
		Agent:      a.name,
		Phase:      phase,
		ErrorType:  errorType,
		Message:    message,
		FilePath:   filePath,
		LineNumber: lineNumber,
		Suggestion: suggestion,
	}

	a.currentContext.AddError(execError)
}
