package agent

import (
	"context"
	"time"

	"wilson/llm"
)

// Agent represents an autonomous agent that can execute tasks
type Agent interface {
	// Name returns the agent's unique name
	Name() string

	// Purpose returns the LLM purpose this agent uses
	Purpose() llm.Purpose

	// CanHandle checks if the agent can handle a specific task
	CanHandle(task *Task) bool

	// Execute executes a task and returns the result
	Execute(ctx context.Context, task *Task) (*Result, error)

	// AllowedTools returns the tools this agent can use (empty = all tools)
	AllowedTools() []string
}

// Task represents a task to be executed by an agent
type Task struct {
	ID           string                 `json:"id"`
	ContextKey   string                 `json:"context_key"`
	Type         string                 `json:"type"` // "research", "analysis", "code", "general"
	Description  string                 `json:"description"`
	Input        map[string]interface{} `json:"input"`
	RequestedBy  string                 `json:"requested_by"` // Which agent delegated this
	CreatedAt    time.Time              `json:"created_at"`
	Status       TaskStatus             `json:"status"`
	Priority     int                    `json:"priority"` // 1-5, 5 is highest
	ModelUsed    string                 `json:"model_used"`    // Phase 3: Track which model is executing this task
	AgentName    string                 `json:"agent_name"`    // Phase 3: Track which agent is executing
	UsedFallback bool                   `json:"used_fallback"` // Phase 5: Track if fallback model was used
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
)

// TaskType constants
const (
	TaskTypeGeneral  = "general"
	TaskTypeResearch = "research"
	TaskTypeAnalysis = "analysis"
	TaskTypeCode     = "code"
	TaskTypeSummary  = "summary"
)

// Result represents the result of task execution
type Result struct {
	TaskID      string                 `json:"task_id"`
	Success     bool                   `json:"success"`
	Output      string                 `json:"output"`
	Artifacts   []string               `json:"artifacts"`   // IDs of created artifacts
	Error       string                 `json:"error"`
	Metadata    map[string]interface{} `json:"metadata"`
	CompletedAt time.Time              `json:"completed_at"`
	Agent       string                 `json:"agent"`
}

// DelegationRequest represents a request to delegate a task
type DelegationRequest struct {
	ToAgent     string                 `json:"to_agent"`     // Target agent name
	ContextKey  string                 `json:"context_key"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Priority    int                    `json:"priority"`
}

// AgentInfo provides information about an agent
type AgentInfo struct {
	Name         string   `json:"name"`
	Purpose      string   `json:"purpose"`
	Description  string   `json:"description"`
	CanDelegate  bool     `json:"can_delegate"`
	AllowedTools []string `json:"allowed_tools"`
	Status       string   `json:"status"` // "available", "busy", "offline"
}
