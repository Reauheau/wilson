package context

import (
	"time"
)

// Context represents a task or project container
type Context struct {
	ID          int                    `json:"id"`
	Key         string                 `json:"key"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by"`
	Metadata    map[string]interface{} `json:"metadata"`
	Artifacts   []Artifact             `json:"artifacts,omitempty"`
	Notes       []AgentNote            `json:"notes,omitempty"`
}

// Artifact represents an output or finding from an agent
type Artifact struct {
	ID        int              `json:"id"`
	ContextID int              `json:"context_id"`
	Type      string           `json:"type"`
	Content   string           `json:"content"`
	Source    string           `json:"source"`
	Agent     string           `json:"agent"`
	CreatedAt time.Time        `json:"created_at"`
	Metadata  ArtifactMetadata `json:"metadata"`
}

// ArtifactMetadata contains metadata about an artifact
type ArtifactMetadata struct {
	Model      string   `json:"model,omitempty"`
	TokensUsed int      `json:"tokens_used,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	References []int    `json:"references,omitempty"` // IDs of related artifacts
	Tags       []string `json:"tags,omitempty"`
	Quality    string   `json:"quality,omitempty"` // "draft", "reviewed", "final"
}

// AgentNote represents inter-agent communication
type AgentNote struct {
	ID        int       `json:"id"`
	ContextID int       `json:"context_id"`
	FromAgent string    `json:"from_agent"`
	ToAgent   string    `json:"to_agent"` // empty = broadcast
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// ContextStatus constants
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusArchived  = "archived"
	StatusFailed    = "failed"
)

// ContextType constants
const (
	TypeTask     = "task"
	TypeResearch = "research"
	TypeAnalysis = "analysis"
	TypeCode     = "code"
	TypeSession  = "session"
)

// ArtifactType constants
const (
	ArtifactWebSearch    = "web_search"
	ArtifactWebPage      = "web_page"
	ArtifactAnalysis     = "analysis"
	ArtifactSummary      = "summary"
	ArtifactCode         = "code"
	ArtifactLLMResponse  = "llm_response"
	ArtifactExtractedText = "extracted_text"
)

// SearchResult represents a search result
type SearchResult struct {
	Artifact Artifact  `json:"artifact"`
	Context  Context   `json:"context"`
	Rank     float64   `json:"rank"`
}

// CreateContextRequest is the request to create a new context
type CreateContextRequest struct {
	Key         string                 `json:"key"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	CreatedBy   string                 `json:"created_by"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// StoreArtifactRequest is the request to store an artifact
type StoreArtifactRequest struct {
	ContextKey string           `json:"context_key"`
	Type       string           `json:"type"`
	Content    string           `json:"content"`
	Source     string           `json:"source"`
	Agent      string           `json:"agent"`
	Metadata   ArtifactMetadata `json:"metadata,omitempty"`
}

// SearchArtifactsRequest is the request to search artifacts
type SearchArtifactsRequest struct {
	Query      string   `json:"query"`
	ContextKey string   `json:"context_key,omitempty"` // optional: limit to context
	Types      []string `json:"types,omitempty"`       // optional: filter by type
	Agent      string   `json:"agent,omitempty"`       // optional: filter by agent
	Limit      int      `json:"limit,omitempty"`       // default: 10
}
