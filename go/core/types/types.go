package types

import (
	"context"
)

// ToolCategory represents the category of a tool
type ToolCategory string

const (
	CategoryFileSystem    ToolCategory = "filesystem"
	CategoryNetwork       ToolCategory = "network"
	CategorySystem        ToolCategory = "system"
	CategoryAI            ToolCategory = "ai"
	CategoryWeb           ToolCategory = "web"
	CategoryOrchestration ToolCategory = "orchestration"
)

// RiskLevel indicates how dangerous a tool operation is
type RiskLevel string

const (
	RiskSafe      RiskLevel = "safe"      // Read-only, no side effects
	RiskModerate  RiskLevel = "moderate"  // Can modify files, needs confirmation
	RiskDangerous RiskLevel = "dangerous" // System commands, always confirm
)

// Parameter defines a tool parameter with validation rules
type Parameter struct {
	Name        string
	Type        string // string, int, bool, array
	Required    bool
	Description string
	Default     interface{}
	Example     string
}

// ToolMetadata contains information about a tool
type ToolMetadata struct {
	Name            string
	Description     string
	Category        ToolCategory
	RiskLevel       RiskLevel
	RequiresConfirm bool
	Parameters      []Parameter
	Examples        []string
	Enabled         bool
}

// ProgressCallback is called during tool execution to report progress
type ProgressCallback func(message string)

// Tool interface that all tools must implement
type Tool interface {
	Metadata() ToolMetadata
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
	Validate(args map[string]interface{}) error
}

// ToolWithProgress is an optional interface for tools that support progress updates
type ToolWithProgress interface {
	Tool
	ExecuteWithProgress(ctx context.Context, args map[string]interface{}, progress ProgressCallback) (string, error)
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}
