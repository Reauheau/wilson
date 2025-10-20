package ctxtools

import (
	"context"
	"fmt"

	ctx "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type LeaveNoteTool struct{}

func (t *LeaveNoteTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "leave_note",
		Description:     "Leave a note for another agent in a context (inter-agent communication)",
		Category:        CategoryContext,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "note",
				Type:        "string",
				Required:    true,
				Description: "The note to leave",
			},
			{
				Name:        "to_agent",
				Type:        "string",
				Required:    false,
				Description: "Target agent (empty = broadcast to all)",
				Example:     "analysis",
			},
			{
				Name:        "context_key",
				Type:        "string",
				Required:    false,
				Description: "Context to leave note in (uses active context if not specified)",
			},
		},
		Examples: []string{
			`{"tool": "leave_note", "arguments": {"note": "API research complete, ready for code generation", "to_agent": "code"}}`,
		},
	}
}

func (t *LeaveNoteTool) Validate(args map[string]interface{}) error {
	note, ok := args["note"].(string)
	if !ok || note == "" {
		return fmt.Errorf("note parameter is required")
	}
	return nil
}

func (t *LeaveNoteTool) Execute(ctxArg context.Context, args map[string]interface{}) (string, error) {
	manager := ctx.GetGlobalManager()
	if manager == nil {
		return "", fmt.Errorf("context manager not initialized")
	}

	note, _ := args["note"].(string)
	toAgent, _ := args["to_agent"].(string)
	contextKey, _ := args["context_key"].(string)

	agentNote, err := manager.AddNote(contextKey, "user", toAgent, note)
	if err != nil {
		return "", fmt.Errorf("failed to leave note: %w", err)
	}

	result := fmt.Sprintf("✓ Note left (ID: %d)\n", agentNote.ID)
	result += fmt.Sprintf("  From: %s\n", agentNote.FromAgent)
	if toAgent != "" {
		result += fmt.Sprintf("  To: %s\n", toAgent)
	} else {
		result += "  To: [all agents]\n"
	}
	result += fmt.Sprintf("  Note: %s\n", note)

	return result, nil
}

func init() {
	registry.Register(&LeaveNoteTool{})
}
