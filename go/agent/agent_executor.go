package agent

import (
	"context"
	"fmt"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/llm"
)

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
		maxIterations: 7, // Balanced: allows multi-file tasks but prevents runaway
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
			// Extract target path from the generate_code arguments
			targetPath := "/tmp/generated_code.go"
			if lang, ok := toolCall.Arguments["language"].(string); ok {
				if desc, ok := toolCall.Arguments["description"].(string); ok {
					// Try to infer filename from description
					if strings.Contains(strings.ToLower(desc), "test") {
						if lang == "go" {
							targetPath = "main_test.go"
						} else {
							targetPath = "test_main." + lang
						}
					} else {
						if lang == "go" {
							targetPath = "main.go"
						} else {
							targetPath = "main." + lang
						}
					}
				}
			}

			// Check if path hint exists in original user prompt or system prompt
			if strings.Contains(userPrompt, "wilsontestdir") {
				targetPath = "/Users/roderick.vannievelt/IdeaProjects/wilsontestdir/" + targetPath
			}

			// Auto-inject write_file call
			writeFileCall := ToolCall{
				Tool: "write_file",
				Arguments: map[string]interface{}{
					"path":    targetPath,
					"content": toolResult, // The generated code
				},
			}

			fmt.Printf("→ Auto-injecting write_file to save generated code to %s\n", targetPath)

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
		}

		// Check if we should stop after successful compile
		// For code tasks: generate → write → compile should be enough
		if toolCall.Tool == "compile" && !strings.Contains(toolResult, "error") && !strings.Contains(toolResult, "failed") {
			// Compile succeeded - check if we have the required sequence
			hasGenerate := false
			hasWrite := false
			for _, t := range result.ToolsExecuted {
				if t == "generate_code" {
					hasGenerate = true
				}
				if t == "write_file" || t == "modify_file" || t == "append_to_file" {
					hasWrite = true
				}
			}

			// If we have generate → write → compile, we're done
			if hasGenerate && hasWrite {
				result.Success = true
				result.Output = fmt.Sprintf("Code generation completed successfully:\n\n%s", toolResult)
				return result, nil
			}
		}

		// Add this exchange to conversation history
		conversationHistory = append(conversationHistory, llm.Message{
			Role:    "assistant",
			Content: fmt.Sprintf(`{"tool": "%s", "arguments": %s}`, toolCall.Tool, formatArgsForAgent(toolCall.Arguments)),
		})
		conversationHistory = append(conversationHistory, llm.Message{
			Role:    "user",
			Content: fmt.Sprintf("Tool '%s' executed successfully. Result:\n\n%s\n\nIf you need to call more tools to complete the task, do so now. Otherwise, provide a summary of what was accomplished.", toolCall.Tool, toolResult),
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
