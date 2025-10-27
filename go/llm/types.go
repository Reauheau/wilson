package llm

import "context"

// Purpose defines the intended use case for an LLM
type Purpose string

const (
	PurposeChat          Purpose = "chat"
	PurposeOrchestration Purpose = "orchestration" // Tool calling and task orchestration
	PurposePlanning      Purpose = "planning"      // Task decomposition and strategic planning
	PurposeAnalysis      Purpose = "analysis"
	PurposeCode          Purpose = "code"
	PurposeVision        Purpose = "vision"
)

// Message represents a single message in a conversation
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // The message content
}

// Request represents a request to an LLM
type Request struct {
	Messages    []Message      `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
}

// Response represents a response from an LLM
type Response struct {
	Content    string         `json:"content"`
	Model      string         `json:"model"`
	TokensUsed int            `json:"tokens_used,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Client defines the interface for interacting with LLM providers
type Client interface {
	// Generate sends a request to the LLM and returns a response
	Generate(ctx context.Context, req Request) (*Response, error)

	// GetModel returns the model name this client is using
	GetModel() string

	// GetProvider returns the provider name (e.g., "ollama", "openai")
	GetProvider() string

	// IsAvailable checks if the LLM is available and responding
	IsAvailable(ctx context.Context) bool
}

// Config represents configuration for a specific LLM instance
type Config struct {
	Provider    string         `yaml:"provider"`
	Model       string         `yaml:"model"`
	Temperature float64        `yaml:"temperature"`
	BaseURL     string         `yaml:"base_url,omitempty"`
	APIKey      string         `yaml:"api_key,omitempty"`
	Fallback    string         `yaml:"fallback,omitempty"` // Fallback model name
	Options     map[string]any `yaml:"options,omitempty"`
	KeepAlive   bool           `yaml:"keep_alive"`   // Never unload model (for Wilson's chat model)
	IdleTimeout int            `yaml:"idle_timeout"` // Seconds before unloading (0 = immediate)
}
