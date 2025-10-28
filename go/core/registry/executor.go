package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"wilson/config"
	"wilson/core/audit"

	. "wilson/core/types"
)

// Executor handles tool execution with confirmation logic
type Executor struct {
	ConfirmationHandler ConfirmationHandler                 // Handler for confirmation prompts (required)
	UserQuery           string                              // Track the original user query for audit logging
	StatusHandler       func(toolName string, phase string) // Optional callback for status updates
}

// NewExecutor creates a new tool executor with the specified confirmation handler
// For backward compatibility with tests that used AutoConfirm, use AlwaysConfirm handler
func NewExecutor() *Executor {
	return &Executor{
		ConfirmationHandler: &AlwaysConfirm{}, // Default: auto-confirm for backward compatibility
	}
}

// NewExecutorWithConfirmation creates an executor with a custom confirmation handler
// This is the recommended constructor for production code
func NewExecutorWithConfirmation(handler ConfirmationHandler) *Executor {
	return &Executor{
		ConfirmationHandler: handler,
	}
}

// SetUserQuery sets the user query for audit logging
func (e *Executor) SetUserQuery(query string) {
	e.UserQuery = query
}

// Execute runs a tool with proper validation and confirmation
func (e *Executor) Execute(ctx context.Context, call ToolCall) (string, error) {
	startTime := time.Now()

	// Get the tool from registry
	tool, err := GetTool(call.Tool)
	if err != nil {
		// Check if tool name looks similar to existing tools
		suggestions := e.findSimilarTools(call.Tool)
		if len(suggestions) > 0 {
			return "", fmt.Errorf("tool '%s' not found. Did you mean: %s?", call.Tool, strings.Join(suggestions, ", "))
		}
		return "", err
	}

	metadata := tool.Metadata()

	// Validate arguments
	if err := tool.Validate(call.Arguments); err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// Check if confirmation is required (config can override)
	requiresConfirm := metadata.RequiresConfirm
	if configConfirm := config.ShouldConfirm(metadata.Name); configConfirm != nil {
		requiresConfirm = *configConfirm
	}

	confirmed := true
	userDeclined := false

	if requiresConfirm {
		// Use injected confirmation handler
		if e.ConfirmationHandler == nil {
			return "", fmt.Errorf("confirmation required but no handler configured for tool '%s'", metadata.Name)
		}

		req := ConfirmationRequest{
			ToolName:    metadata.Name,
			Description: metadata.Description,
			RiskLevel:   string(metadata.RiskLevel),
			Arguments:   call.Arguments,
		}

		if !e.ConfirmationHandler.RequestConfirmation(req) {
			userDeclined = true
			duration := time.Since(startTime)

			// Log declined execution
			audit.LogExecution(audit.AuditLog{
				Timestamp:    startTime,
				ToolName:     metadata.Name,
				Category:     metadata.Category,
				Arguments:    call.Arguments,
				Duration:     duration,
				Confirmed:    false,
				UserDeclined: true,
				UserQuery:    e.UserQuery,
			})

			return "", fmt.Errorf("user declined to execute '%s'", metadata.Name)
		}
	}

	// Show execution status (after confirmation)
	if e.StatusHandler != nil {
		e.StatusHandler(metadata.Name, "executing")
	}

	// Execute the tool (with progress updates if supported)
	var result string
	var execErr error

	if toolWithProgress, ok := tool.(ToolWithProgress); ok {
		// Tool supports progress updates
		progressCallback := func(msg string) {
			if e.StatusHandler != nil {
				e.StatusHandler(msg, "progress")
			}
		}
		result, execErr = toolWithProgress.ExecuteWithProgress(ctx, call.Arguments, progressCallback)
	} else {
		// Tool doesn't support progress
		result, execErr = tool.Execute(ctx, call.Arguments)
	}

	duration := time.Since(startTime)

	// Notify completion
	if e.StatusHandler != nil {
		if execErr != nil {
			e.StatusHandler(metadata.Name, "error")
		} else {
			e.StatusHandler(metadata.Name, "completed")
		}
	}

	// Log execution
	auditLog := audit.AuditLog{
		Timestamp:    startTime,
		ToolName:     metadata.Name,
		Category:     metadata.Category,
		Arguments:    call.Arguments,
		Duration:     duration,
		Confirmed:    confirmed,
		UserDeclined: userDeclined,
		UserQuery:    e.UserQuery,
	}

	if execErr != nil {
		auditLog.Error = execErr.Error()
	} else {
		// Truncate result if too long
		if len(result) > 500 {
			auditLog.Result = result[:500] + "... (truncated)"
		} else {
			auditLog.Result = result
		}
	}

	if logErr := audit.LogExecution(auditLog); logErr != nil {
		// Don't fail execution if logging fails, just print warning
		fmt.Fprintf(os.Stderr, "Warning: failed to log audit entry: %v\n", logErr)
	}

	return result, execErr
}

// findSimilarTools finds tool names that are similar to the requested name
func (e *Executor) findSimilarTools(requested string) []string {
	allTools := GetAllToolNames()
	similar := make([]string, 0)

	requested = strings.ToLower(requested)

	for _, name := range allTools {
		nameLower := strings.ToLower(name)

		// Exact substring match
		if strings.Contains(nameLower, requested) || strings.Contains(requested, nameLower) {
			similar = append(similar, name)
			continue
		}

		// Simple similarity check (could be improved with Levenshtein distance)
		if levenshteinDistance(requested, nameLower) <= 2 {
			similar = append(similar, name)
		}
	}

	return similar
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// IsToolCall checks if the response is a tool call request
func IsToolCall(response string) (bool, *ToolCall) {
	response = strings.TrimSpace(response)

	// Try to repair common JSON issues (unescaped quotes in content)
	repairedResponse := repairToolCallJSON(response)

	// First, try to parse entire response as JSON (if model was perfect)
	var call ToolCall
	err := json.Unmarshal([]byte(repairedResponse), &call)
	if err == nil {
		if call.Tool != "" && call.Arguments != nil {
			return true, &call
		}
	}

	// Try to find JSON in the response (model might add text before/after)
	startIdx := strings.Index(response, `{"tool"`)
	if startIdx == -1 {
		// Try alternate format
		startIdx = strings.Index(response, `{ "tool"`)
	}

	if startIdx != -1 {
		// Find the matching closing brace by counting braces
		braceCount := 0
		endIdx := -1
		for i := startIdx; i < len(response); i++ {
			if response[i] == '{' {
				braceCount++
			} else if response[i] == '}' {
				braceCount--
				if braceCount == 0 {
					endIdx = i
					break
				}
			}
		}

		if endIdx != -1 {
			jsonStr := response[startIdx : endIdx+1]

			var call ToolCall
			if err := json.Unmarshal([]byte(jsonStr), &call); err == nil {
				// Check if it has required fields
				if call.Tool != "" && call.Arguments != nil {
					return true, &call
				}
			}
		}
	}

	return false, nil
}

// repairToolCallJSON attempts to fix common JSON issues in tool call responses
// Specifically handles unescaped quotes in the "content" field
func repairToolCallJSON(response string) string {
	// Look for the "content" field
	contentIdx := strings.Index(response, `"content":`)
	if contentIdx == -1 {
		return response // No content field
	}

	// Find the start of the content value (after the colon and optional whitespace)
	valueStart := contentIdx + len(`"content":`)
	for valueStart < len(response) && (response[valueStart] == ' ' || response[valueStart] == '\t') {
		valueStart++
	}

	// Make sure it starts with a quote
	if valueStart >= len(response) || response[valueStart] != '"' {
		return response // Not a string value
	}

	// Find the end of the content string by looking for the closing quote
	// We need to be careful - skip already escaped quotes
	valueStart++ // Skip opening quote
	var result strings.Builder
	result.WriteString(response[:valueStart])

	inEscape := false
	foundEnd := false
	i := valueStart

	for i < len(response) {
		ch := response[i]

		if inEscape {
			// Previous char was backslash, keep this char as-is
			result.WriteByte(ch)
			inEscape = false
			i++
			continue
		}

		if ch == '\\' {
			// Start of escape sequence
			result.WriteByte(ch)
			inEscape = true
			i++
			continue
		}

		if ch == '"' {
			// Unescaped quote - need to check if it's the closing quote or an embedded quote
			// Look ahead to see what's after
			nextIdx := i + 1
			// Skip whitespace
			for nextIdx < len(response) && (response[nextIdx] == ' ' || response[nextIdx] == '\t' || response[nextIdx] == '\n') {
				nextIdx++
			}

			// If next char is , or }, this is the closing quote
			if nextIdx < len(response) && (response[nextIdx] == ',' || response[nextIdx] == '}') {
				// This is the closing quote
				result.WriteByte('"')
				result.WriteString(response[i+1:])
				foundEnd = true
				break
			} else {
				// This is an embedded quote that should be escaped
				result.WriteString(`\"`)
				i++
				continue
			}
		}

		// Regular character
		result.WriteByte(ch)
		i++
	}

	if !foundEnd {
		return response // Couldn't find proper end
	}

	return result.String()
}

// truncateDebug truncates string for debug output
func truncateDebug(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
