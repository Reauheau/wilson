package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wilson/config"

	. "wilson/core/types"
)

// AuditLog represents a single tool execution event
type AuditLog struct {
	Timestamp    time.Time              `json:"timestamp"`
	ToolName     string                 `json:"tool_name"`
	Category     ToolCategory           `json:"category"`
	Arguments    map[string]interface{} `json:"arguments"`
	Result       string                 `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     time.Duration          `json:"duration_ms"`
	Confirmed    bool                   `json:"confirmed"`
	UserDeclined bool                   `json:"user_declined"`
	UserQuery    string                 `json:"user_query"`
}

// LogExecution logs a tool execution to the audit log
func LogExecution(log AuditLog) error {
	if !config.IsAuditEnabled() {
		return nil // Audit logging is disabled
	}

	logPath := config.GetAuditLogPath()

	// Create directory if it doesn't exist
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	// Marshal to JSON
	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	// Write log entry
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// GetAuditLogs reads all audit logs from the file
func GetAuditLogs() ([]AuditLog, error) {
	logPath := config.GetAuditLogPath()

	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditLog{}, nil // No logs yet
		}
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	var logs []AuditLog
	lines := splitLines(string(data))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var log AuditLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			// Skip malformed lines
			continue
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetRecentAuditLogs returns the N most recent audit logs
func GetRecentAuditLogs(n int) ([]AuditLog, error) {
	logs, err := GetAuditLogs()
	if err != nil {
		return nil, err
	}

	if len(logs) <= n {
		return logs, nil
	}

	return logs[len(logs)-n:], nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}

	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}
