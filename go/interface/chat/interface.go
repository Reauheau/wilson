package chat

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"wilson/agent"
	"wilson/ui"
)

// Interface handles chat I/O operations
type Interface struct {
	scanner *bufio.Scanner
	status  *ui.StatusLine
}

// NewInterface creates a new chat interface
func NewInterface() *Interface {
	return &Interface{
		scanner: bufio.NewScanner(os.Stdin),
		status:  ui.GetGlobalStatus(),
	}
}

// ReadInput reads user input from stdin
func (i *Interface) ReadInput() (string, error) {
	// Always use simple "You:" prompt
	// Task status updates will appear on separate lines above via printStatus()
	prompt := "You: "
	fmt.Print(prompt)

	if !i.scanner.Scan() {
		if err := i.scanner.Err(); err != nil {
			return "", fmt.Errorf("error reading input: %w", err)
		}
		// EOF
		return "", nil
	}

	return strings.TrimSpace(i.scanner.Text()), nil
}

// pluralize returns "s" if count != 1, otherwise empty string
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// ShowThinking displays a thinking indicator
func (i *Interface) ShowThinking(message string) {
	i.status.ShowWithSpinner(message)
}

// ClearStatus clears the status line
func (i *Interface) ClearStatus() {
	i.status.Clear()
}

// DisplayResponse displays Wilson's response
func (i *Interface) DisplayResponse(response string) {
	fmt.Print("Wilson: ")
	fmt.Println(response)
}

// DisplayError displays an error message
func (i *Interface) DisplayError(err error) {
	fmt.Printf("\nError: %v\n", err)
}

// DisplayToolExecution displays tool execution status
func (i *Interface) DisplayToolExecution(toolName string, status string) {
	switch status {
	case "preparing":
		fmt.Printf("Wilson: Preparing to use tool: %s\n", toolName)
	case "executing":
		i.status.ShowWithSpinner(fmt.Sprintf("Executing: %s", toolName))
	case "completed":
		i.status.Clear()
		fmt.Printf("Wilson: [✓ Completed: %s]\n", toolName)
	case "error":
		i.status.Clear()
		fmt.Printf("Wilson: [✗ Error in %s]\n", toolName)
	case "cancelled":
		i.status.Clear()
		fmt.Printf("Wilson: [Tool execution cancelled by user]\n")
	}
}

// DisplayToolResult displays the result of tool execution
func (i *Interface) DisplayToolResult(result string) {
	fmt.Printf("[Tool result:]\n%s\n", result)
}

// StreamToken displays a token from streaming response
func (i *Interface) StreamToken(token string, firstToken bool) {
	if firstToken {
		i.ClearStatus()
		fmt.Print("Wilson: ")
	}
	fmt.Print(token)
}

// PrintSeparator prints a line separator for readability
func (i *Interface) PrintSeparator() {
	fmt.Println()
}

// PrintHelp prints help information
func (i *Interface) PrintHelp(helpText string) {
	fmt.Println(helpText)
}

// PrintWelcome prints welcome banner
func (i *Interface) PrintWelcome(banner string) {
	fmt.Println(banner)
}

// CheckAndNotifyCompletedTasks checks for newly completed tasks and notifies the user
func (i *Interface) CheckAndNotifyCompletedTasks(lastCheckedTasks map[string]bool) map[string]bool {
	coordinator := agent.GetGlobalCoordinator()
	if coordinator == nil {
		return lastCheckedTasks
	}

	// Get all tasks
	allTasks := coordinator.ListTasks()
	newCompleted := make(map[string]bool)

	// Check each task
	for _, task := range allTasks {
		if task.Status == "completed" || task.Status == "failed" {
			// This task is done
			newCompleted[task.ID] = true

			// If we haven't notified about this task yet, show notification
			if lastCheckedTasks == nil || !lastCheckedTasks[task.ID] {
				i.notifyTaskCompletion(task)
			}
		}
	}

	return newCompleted
}

// notifyTaskCompletion shows a notification that a background task completed
func (i *Interface) notifyTaskCompletion(task *agent.Task) {
	coordinator := agent.GetGlobalCoordinator()
	if coordinator == nil {
		return
	}

	// Get the task result
	_, result, _ := coordinator.GetTaskStatus(task.ID)

	if result != nil && result.Success {
		// Success
		fmt.Printf("\n🎉 Background task completed: %s\n", shortenTaskID(task.ID))
		fmt.Printf("   Type: %s | Agent: %s\n", task.Type, task.AgentName)
		if len(result.Output) > 0 {
			// Show first line of output
			firstLine := strings.Split(result.Output, "\n")[0]
			if len(firstLine) > 80 {
				firstLine = firstLine[:80] + "..."
			}
			fmt.Printf("   Result: %s\n", firstLine)
		}
		fmt.Printf("   Use 'check_task_progress %s' for full details\n\n", shortenTaskID(task.ID))
	} else if result != nil && !result.Success {
		// Failed
		fmt.Printf("\n❌ Background task failed: %s\n", shortenTaskID(task.ID))
		fmt.Printf("   Type: %s | Agent: %s\n", task.Type, task.AgentName)
		if result.Error != "" {
			fmt.Printf("   Error: %s\n", result.Error)
		}
		fmt.Printf("   Use 'check_task_progress %s' for full details\n\n", shortenTaskID(task.ID))
	}
}

// shortenTaskID returns the first 8 characters of a task ID for display
func shortenTaskID(taskID string) string {
	if len(taskID) > 8 {
		return taskID[:8]
	}
	return taskID
}
