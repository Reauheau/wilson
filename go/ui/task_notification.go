package ui

// TaskNotification represents a completed task notification
// This is a DTO (Data Transfer Object) for passing task completion info from orchestration to UI
// Placed in ui package to avoid import cycles (orchestration → ui is allowed, ui → orchestration is not)
type TaskNotification struct {
	TaskID    string
	Type      string
	AgentName string
	Success   bool
	Output    string
	Error     string
}
