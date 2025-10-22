package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/llm"
)

// ANSI color codes and control sequences
const (
	colorReset     = "\033[0m"
	colorLightGrey = "\033[37m"
	colorGreen     = "\033[32m"
	clearLine      = "\r\033[K" // Return to start and clear line
)

// printStatus prints a status message in light grey, clearing current line first
func printStatus(message string) {
	fmt.Printf("%s%s%s%s\n", clearLine, colorLightGrey, message, colorReset)
}

// printSuccess prints a success message in green, clearing current line first
func printSuccess(message string) {
	fmt.Printf("%s%s%s%s\n", clearLine, colorGreen, message, colorReset)
}

// AgentToolExecutor handles tool execution for specialized agents
// This ensures agents actually execute tools instead of just describing them
type AgentToolExecutor struct {
	executor      *registry.Executor
	llmManager    *llm.Manager
	maxIterations int
}

// NewAgentToolExecutor creates a new agent tool executor
func NewAgentToolExecutor(executor *registry.Executor, llmManager *llm.Manager) *AgentToolExecutor {
	return &AgentToolExecutor{
		executor:      executor,
		llmManager:    llmManager,
		maxIterations: 9, // Allows multi-file tasks with compile step
	}
}

// ExecutionResult contains the results of executing an agent's response
type ExecutionResult struct {
	Success               bool
	Output                string
	ToolsExecuted         []string
	ToolResults           []string
	Artifacts             []string
	HallucinationDetected bool
	Error                 string
}

// ExecuteAgentResponse parses an LLM response and executes all tool calls
// This is the key function that makes agents work like Claude Code
func (ate *AgentToolExecutor) ExecuteAgentResponse(
	ctx context.Context,
	llmResponse string,
	systemPrompt string,
	userPrompt string,
	purpose llm.Purpose,
	taskID string, // Task ID for progress updates
) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Success:       false,
		ToolsExecuted: []string{},
		ToolResults:   []string{},
		Artifacts:     []string{},
	}

	currentResponse := llmResponse
	conversationHistory := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Execute tool chain (like Claude Code does)
	for i := 0; i < ate.maxIterations; i++ {
		// Check if response contains a tool call
		isTool, toolCall := registry.IsToolCall(currentResponse)

		if !isTool || toolCall == nil {
			// No more tools to execute
			// If we executed at least one tool, this is success
			// If we executed zero tools, this might be hallucination
			if len(result.ToolsExecuted) == 0 {
				result.HallucinationDetected = true
				result.Error = "Model provided description instead of tool calls"
				result.Output = currentResponse
				return result, fmt.Errorf("hallucination detected: no tools executed")
			}

			// Success - tools were executed and now we have final response
			// Only show "Task complete!" for code generation tasks (not for chat/delegation tasks)
			if len(result.ToolsExecuted) > 0 {
				hasCodeTask := false
				for _, tool := range result.ToolsExecuted {
					if tool == "generate_code" || tool == "write_file" || tool == "compile" {
						hasCodeTask = true
						break
					}
				}
				if hasCodeTask {
					printSuccess("Task complete!")
				}
			}
			result.Success = true
			result.Output = currentResponse
			return result, nil
		}

		// Update task progress - show what we're about to execute
		coordinator := GetGlobalCoordinator()
		if coordinator != nil && taskID != "" {
			coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Executing %s", toolCall.Tool), result.ToolsExecuted)
		}

		// Execute the tool
		toolResult, err := ate.executor.Execute(ctx, *toolCall)
		result.ToolsExecuted = append(result.ToolsExecuted, toolCall.Tool)

		// Update task progress - show what we completed
		if coordinator != nil && taskID != "" {
			coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Completed %s", toolCall.Tool), result.ToolsExecuted)
		}

		if err != nil {
			// Tool execution failed
			if coordinator != nil && taskID != "" {
				coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Failed: %s", toolCall.Tool), result.ToolsExecuted)
			}
			result.Error = fmt.Sprintf("Tool '%s' failed: %v", toolCall.Tool, err)
			result.Output = currentResponse
			return result, fmt.Errorf("tool execution failed: %w", err)
		}

		result.ToolResults = append(result.ToolResults, toolResult)

		// AUTO-INJECT: If generate_code succeeded, immediately call write_file
		// This removes LLM decision-making and makes workflow 100% reliable
		if toolCall.Tool == "generate_code" && err == nil {
			// Extract project path from user prompt (task.Input contains project_path)
			projectPath := extractPathFromPrompt(userPrompt)

			// Determine filename based on file type and language
			filename := "main.go" // default
			if lang, ok := toolCall.Arguments["language"].(string); ok {
				if desc, ok := toolCall.Arguments["description"].(string); ok {
					// Check if this is for a test file
					isTestFile := strings.Contains(strings.ToLower(desc), "test") ||
						strings.Contains(strings.ToLower(userPrompt), "file_type: test")

					// Also check requirements
					if reqs, ok := toolCall.Arguments["requirements"].([]interface{}); ok {
						for _, req := range reqs {
							if reqStr, ok := req.(string); ok {
								if strings.Contains(strings.ToLower(reqStr), "test") {
									isTestFile = true
									break
								}
							}
						}
					}

					// Determine filename
					if isTestFile {
						if lang == "go" {
							filename = "main_test.go"
						} else {
							filename = "test_main." + lang
						}
					} else {
						if lang == "go" {
							filename = "main.go"
						} else {
							filename = "main." + lang
						}
					}
				}
			}

			// Build full target path
			targetPath := filepath.Join(projectPath, filename)

			// Extract filename for display
			displayName := filename
			if idx := strings.LastIndex(targetPath, "/"); idx != -1 {
				displayName = targetPath[idx+1:]
			}
			printStatus(fmt.Sprintf("Generating code for %s...", displayName))

			// Auto-inject write_file call
			writeFileCall := ToolCall{
				Tool: "write_file",
				Arguments: map[string]interface{}{
					"path":    targetPath,
					"content": toolResult, // The generated code
				},
			}

			// Execute write_file
			writeResult, writeErr := ate.executor.Execute(ctx, writeFileCall)
			result.ToolsExecuted = append(result.ToolsExecuted, "write_file")

			if writeErr != nil {
				result.Error = fmt.Sprintf("Auto-injected write_file failed: %v", writeErr)
				return result, fmt.Errorf("auto-injected write_file failed: %w", writeErr)
			}

			result.ToolResults = append(result.ToolResults, writeResult)

			// Update conversation history with the write_file action
			conversationHistory = append(conversationHistory, llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf(`{"tool": "write_file", "arguments": {"path": "%s", "content": "..."}}`, targetPath),
			})
			conversationHistory = append(conversationHistory, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool 'write_file' executed successfully (auto-injected). Result:\n\n%s", writeResult),
			})

			// AUTO-INJECT: After write_file succeeds, immediately call compile
			// This completes the mandatory workflow: generate → write → compile
			// Extract project directory from target path
			compileTarget := targetPath
			if idx := strings.LastIndex(targetPath, "/"); idx != -1 {
				compileTarget = targetPath[:idx]
			}

			// Extract directory name for display
			dirName := compileTarget
			if idx := strings.LastIndex(compileTarget, "/"); idx != -1 {
				dirName = compileTarget[idx+1:]
			}
			printStatus(fmt.Sprintf("Compiling %s...", dirName))

			// Auto-inject compile call
			compileCall := ToolCall{
				Tool: "compile",
				Arguments: map[string]interface{}{
					"target": compileTarget,
				},
			}

			// Execute compile
			compileResult, compileErr := ate.executor.Execute(ctx, compileCall)
			result.ToolsExecuted = append(result.ToolsExecuted, "compile")

			if compileErr != nil {
				result.Error = fmt.Sprintf("Auto-injected compile failed: %v", compileErr)
				return result, fmt.Errorf("auto-injected compile failed: %w", compileErr)
			}

			result.ToolResults = append(result.ToolResults, compileResult)

			// Update conversation history with the compile action
			conversationHistory = append(conversationHistory, llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf(`{"tool": "compile", "arguments": {"target": "%s"}}`, compileTarget),
			})
			conversationHistory = append(conversationHistory, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool 'compile' executed successfully (auto-injected). Result:\n\n%s", compileResult),
			})

			// Check compile result
			isCompileSuccess := strings.Contains(compileResult, `"success": true`) || strings.Contains(compileResult, "Compilation successful")
			if isCompileSuccess {
				printStatus("Compilation successful")

				// ATOMIC TASK PRINCIPLE: Exit immediately after successful compilation
				// Each subtask should do ONE thing: generate 1 file, make 1 change
				// ManagerAgent orchestrates the sequence of subtasks
				result.Success = true
				result.Output = fmt.Sprintf("Code generated and compiled successfully.\n\nTools used: %v", result.ToolsExecuted)
				return result, nil
			} else {
				// Compile had errors - ATOMIC TASK PRINCIPLE: File is generated, task complete
				// Don't try to fix compile errors here - that would violate atomic task principle
				// If fixes needed, ManagerAgent should create a new "fix compile errors" subtask
				printStatus("Compilation failed - task complete (file generated)")
				result.Success = true // File was generated and written
				result.Output = fmt.Sprintf("Code file generated.\n\nCompilation errors detected:\n%s\n\nTools used: %v", compileResult, result.ToolsExecuted)
				return result, nil
			}
		}

		// Add this exchange to conversation history
		conversationHistory = append(conversationHistory, llm.Message{
			Role:    "assistant",
			Content: fmt.Sprintf(`{"tool": "%s", "arguments": %s}`, toolCall.Tool, formatArgsForAgent(toolCall.Arguments)),
		})

		// This code is now unreachable after successful compilation due to early return above
		// Keeping it for non-compile tool feedback
		var feedbackMsg string
		feedbackMsg = fmt.Sprintf("Tool '%s' executed successfully. Result:\n\n%s\n\nIf you need to call more tools to complete the task, do so now. Otherwise, provide a summary of what was accomplished.", toolCall.Tool, toolResult)

		conversationHistory = append(conversationHistory, llm.Message{
			Role:    "user",
			Content: feedbackMsg,
		})

		// Get next response from LLM (might call another tool or finish)
		// Update progress to show we're waiting for LLM
		coord := GetGlobalCoordinator()
		if coord != nil && taskID != "" {
			coord.UpdateTaskProgress(taskID, fmt.Sprintf("Planning next step after %s...", toolCall.Tool), result.ToolsExecuted)
		}

		req := llm.Request{Messages: conversationHistory}
		resp, err := ate.llmManager.Generate(ctx, purpose, req)
		if err != nil {
			result.Error = fmt.Sprintf("LLM error: %v", err)
			return result, err
		}

		currentResponse = resp.Content

		// Debug: Show what LLM returned after tool execution
	}

	// Hit max iterations - might be infinite loop
	result.Error = fmt.Sprintf("Reached maximum iterations (%d) without completion", ate.maxIterations)
	result.Output = currentResponse
	return result, fmt.Errorf("max iterations exceeded")
}

// formatArgsForAgent converts arguments map to JSON string for display
func formatArgsForAgent(args map[string]interface{}) string {
	parts := []string{}
	for k, v := range args {
		parts = append(parts, fmt.Sprintf(`"%s": %v`, k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// truncateString truncates a string to n characters
func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// extractPathFromPrompt extracts project_path from user prompt
// The prompt format includes "- project_path: /path/to/project"
func extractPathFromPrompt(userPrompt string) string {
	// Look for "project_path:" (with or without dash prefix)
	searchPattern := "project_path:"
	if idx := strings.Index(userPrompt, searchPattern); idx != -1 {
		pathStart := idx + len(searchPattern)
		// Skip any whitespace after the colon
		pathStart += len(userPrompt[pathStart:]) - len(strings.TrimLeft(userPrompt[pathStart:], " \t"))

		// Find end of path (newline or end of string)
		pathEnd := strings.IndexAny(userPrompt[pathStart:], "\n\r")
		if pathEnd == -1 {
			pathEnd = len(userPrompt) - pathStart
		}
		path := strings.TrimSpace(userPrompt[pathStart : pathStart+pathEnd])
		if path != "" && path != "." {
			return path
		}
	}

	// Default to current directory
	return "."
}
