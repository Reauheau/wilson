package registry

// ConfirmationRequest contains all data needed to ask for confirmation
type ConfirmationRequest struct {
	ToolName    string
	Description string
	RiskLevel   string
	Arguments   map[string]interface{}
}

// ConfirmationHandler is an interface for handling tool execution confirmations
// Different implementations can be provided for different environments:
// - TerminalConfirmation (interactive CLI) - default for Wilson
// - AlwaysConfirm (testing/automation) - skips confirmation prompts
// - AlwaysDeny (testing) - always rejects execution
// - Future: WebConfirmation (web UI), SlackConfirmation (bot approval), etc.
//
// This design enables:
// 1. Testability: Core logic can be tested without user interaction
// 2. Flexibility: Different UIs can implement their own confirmation flows
// 3. Separation of concerns: Core business logic doesn't depend on UI
type ConfirmationHandler interface {
	// RequestConfirmation asks for user approval
	// Returns true if approved, false if declined
	RequestConfirmation(req ConfirmationRequest) bool
}

// AlwaysConfirm is a confirmation handler that always approves (for testing/automation)
type AlwaysConfirm struct{}

// RequestConfirmation always returns true (no-op confirmation)
func (a *AlwaysConfirm) RequestConfirmation(req ConfirmationRequest) bool {
	return true
}

// AlwaysDeny is a confirmation handler that always denies (for testing)
type AlwaysDeny struct{}

// RequestConfirmation always returns false (denies all execution)
func (a *AlwaysDeny) RequestConfirmation(req ConfirmationRequest) bool {
	return false
}
