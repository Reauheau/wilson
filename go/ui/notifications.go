package ui

import (
	"fmt"
	"strings"
)

// DisplayTaskCompletionNotifications displays notifications for completed background tasks
// This function is purely presentational - it receives data and displays it
// Business logic (determining what's newly completed) happens in orchestration layer
func DisplayTaskCompletionNotifications(notifications []TaskNotification) {
	for _, notif := range notifications {
		if notif.Success {
			displaySuccessNotification(notif)
		} else {
			displayFailureNotification(notif)
		}
	}
}

// displaySuccessNotification shows a success notification for a completed task
func displaySuccessNotification(notif TaskNotification) {
	fmt.Printf("\n🎉 Background task completed: %s\n", shortenTaskID(notif.TaskID))
	fmt.Printf("   Type: %s | Agent: %s\n", notif.Type, notif.AgentName)

	if len(notif.Output) > 0 {
		// Show first line of output
		firstLine := strings.Split(notif.Output, "\n")[0]
		if len(firstLine) > 80 {
			firstLine = firstLine[:80] + "..."
		}
		fmt.Printf("   Result: %s\n", firstLine)
	}

	fmt.Printf("   Use 'check_task_progress %s' for full details\n\n", shortenTaskID(notif.TaskID))
}

// displayFailureNotification shows a failure notification for a failed task
func displayFailureNotification(notif TaskNotification) {
	fmt.Printf("\n❌ Background task failed: %s\n", shortenTaskID(notif.TaskID))
	fmt.Printf("   Type: %s | Agent: %s\n", notif.Type, notif.AgentName)

	if notif.Error != "" {
		fmt.Printf("   Error: %s\n", notif.Error)
	}

	fmt.Printf("   Use 'check_task_progress %s' for full details\n\n", shortenTaskID(notif.TaskID))
}

// shortenTaskID returns the first 8 characters of a task ID for display
func shortenTaskID(taskID string) string {
	if len(taskID) > 8 {
		return taskID[:8]
	}
	return taskID
}
