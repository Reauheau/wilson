package registry

import (
	"bufio"
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
	AutoConfirm   bool                          // If true, skip confirmation (useful for testing)
	UserQuery     string                        // Track the original user query for audit logging
	StatusHandler func(toolName string, phase string) // Optional callback for status updates
}

// NewExecutor creates a new tool executor
func NewExecutor() *Executor {
	return &Executor{
		AutoConfirm: false,
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

	if requiresConfirm && !e.AutoConfirm {
		if !e.requestConfirmation(metadata, call.Arguments) {
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

// requestConfirmation asks the user to confirm a risky operation
func (e *Executor) requestConfirmation(metadata ToolMetadata, args map[string]interface{}) bool {
	fmt.Println()
	fmt.Printf("⚠️  Confirmation Required\n")
	fmt.Printf("Tool: %s\n", metadata.Name)
	fmt.Printf("Risk Level: %s\n", metadata.RiskLevel)
	fmt.Printf("Description: %s\n", metadata.Description)
	fmt.Printf("Arguments: %s\n", formatArgs(args))
	fmt.Println()
	fmt.Print("Allow execution? (y/n): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return response == "y" || response == "yes"
	}

	return false
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

// formatArgs formats arguments for display
func formatArgs(args map[string]interface{}) string {
	data, err := json.MarshalIndent(args, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(data)
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

	// First, try to parse entire response as JSON (if model was perfect)
	var call ToolCall
	if err := json.Unmarshal([]byte(response), &call); err == nil {
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
