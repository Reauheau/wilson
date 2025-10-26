package base

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"wilson/agent"
	"wilson/agent/feedback"
	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/llm"
	"wilson/ui"
)

// ANSI color codes and control sequences
const (
	colorReset     = "\033[0m"
	colorLightGrey = "\033[37m"
	colorGreen     = "\033[32m"
	clearLine      = "\r\033[K" // Return to start and clear line
)

// printStatus prints a status message in light grey, clearing spinner first
func printStatus(message string) {
	ui.Printf("%s%s%s%s\n", clearLine, colorLightGrey, message, colorReset)
}

// printSuccess prints a success message in green, clearing spinner first
func printSuccess(message string) {
	ui.Printf("%s%s%s%s\n", clearLine, colorGreen, message, colorReset)
}

// AgentToolExecutor handles tool execution for specialized agents
// This ensures agents actually execute tools instead of just describing them
type AgentToolExecutor struct {
	executor      *registry.Executor
	llmManager    *llm.Manager
	maxIterations int
	taskContext   *TaskContext // Rich execution context (Phase 2)
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
	taskContext *TaskContext, // Rich execution context (optional for backward compatibility)
) (*ExecutionResult, error) {
	// Store context for use during execution
	ate.taskContext = taskContext
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
		//
		// 		// Update task progress - show what we're about to execute
		// 		coordinator := GetGlobalCoordinator()
		// 		if coordinator != nil && taskID != "" {
		// 			coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Executing %s", toolCall.Tool), result.ToolsExecuted)
		// 		}

		// Auto-inject source files for test generation
		if toolCall.Tool == "generate_code" {
			if desc, ok := toolCall.Arguments["description"].(string); ok {
				isTestFile := strings.Contains(strings.ToLower(desc), "test")
				if isTestFile && ate.taskContext != nil && len(ate.taskContext.DependencyFiles) > 0 {
					var contextBuilder strings.Builder
					contextBuilder.WriteString("Source files to test:\n\n")

					for _, file := range ate.taskContext.DependencyFiles {
						if !strings.HasSuffix(file, "_test.go") {
							if content, err := os.ReadFile(file); err == nil {
								contextBuilder.WriteString(fmt.Sprintf("=== %s ===\n```go\n%s\n```\n\n", filepath.Base(file), string(content)))
							}
						}
					}

					if existingContext, ok := toolCall.Arguments["context"].(string); ok && existingContext != "" {
						contextBuilder.WriteString(existingContext)
					}

					toolCall.Arguments["context"] = contextBuilder.String()
				}
			}
		}

		// Execute the tool
		toolResult, err := ate.executor.Execute(ctx, *toolCall)
		result.ToolsExecuted = append(result.ToolsExecuted, toolCall.Tool)

		// Update task progress - show what we completed
		// TODO: Add progress updates through a callback to avoid import cycle
		// if coordinator != nil && taskID != "" {
		// 	coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Completed %s", toolCall.Tool), result.ToolsExecuted)
		// }

		if err != nil {
			// Tool execution failed
			// if coordinator != nil && taskID != "" {
			// 	coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Failed: %s", toolCall.Tool), result.ToolsExecuted)
			// }
			result.Error = fmt.Sprintf("Tool '%s' failed: %v", toolCall.Tool, err)
			result.Output = currentResponse
			return result, fmt.Errorf("tool execution failed: %w", err)
		}

		result.ToolResults = append(result.ToolResults, toolResult)

		// AUTO-INJECT: If generate_code succeeded, immediately call write_file
		// This removes LLM decision-making and makes workflow 100% reliable
		if toolCall.Tool == "generate_code" && err == nil {
			// Get project path from TaskContext (or fallback to extraction for backward compat)
			projectPath := "."
			if ate.taskContext != nil && ate.taskContext.ProjectPath != "" {
				projectPath = ate.taskContext.ProjectPath
			} else {
				// Fallback for old code paths without TaskContext
				projectPath = extractPathFromPrompt(userPrompt)
			}

			// Determine filename based on file type and language
			filename := "main.go" // default
			if lang, ok := toolCall.Arguments["language"].(string); ok {
				if desc, ok := toolCall.Arguments["description"].(string); ok {
					// ✅ ROBUST FIX: Check explicit file_type first (most reliable)
					isTestFile := strings.Contains(strings.ToLower(userPrompt), "file_type: test") ||
						strings.Contains(strings.ToLower(userPrompt), "\"file_type\": \"test\"")

					// If explicitly marked as implementation, force NOT test
					isImplementation := strings.Contains(strings.ToLower(userPrompt), "file_type: implementation") ||
						strings.Contains(strings.ToLower(userPrompt), "\"file_type\": \"implementation\"")

					if isImplementation {
						isTestFile = false // Explicit override
					} else if !isTestFile {
						// Only use heuristics if not explicitly set
						isTestFile = strings.Contains(strings.ToLower(desc), "test")

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

			// ✅ AUTO-INIT: Check if go.mod exists in target directory, if not, initialize it
			// This ensures Go projects can compile without manual setup
			goModPath := filepath.Join(compileTarget, "go.mod")
			if _, err := os.Stat(goModPath); os.IsNotExist(err) {
				// Extract module name from directory name
				moduleName := filepath.Base(compileTarget)
				if moduleName == "." || moduleName == "/" {
					moduleName = "main"
				}

				// Create minimal go.mod
				goModContent := fmt.Sprintf("module %s\n\ngo 1.21\n", moduleName)
				if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err == nil {
					ui.Printf("[AgentExecutor] Initialized go.mod in %s\n", compileTarget)
				} else {
					ui.Printf("[AgentExecutor] Warning: Could not create go.mod: %v\n", err)
				}
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
					"path": compileTarget, // ✅ FIX: Use "path" not "target"
				},
			}

			// Execute compile
			compileResult, compileErr := ate.executor.Execute(ctx, compileCall)
			result.ToolsExecuted = append(result.ToolsExecuted, "compile")

			// Check compilation result
			compileSuccess := strings.Contains(compileResult, `"success": true`) || strings.Contains(compileResult, "Compilation successful")

			if compileErr != nil || !compileSuccess {
				errorMsg := ""
				targetFile := compileTarget

				if compileErr != nil {
					errorMsg = compileErr.Error()
				} else {
					// ✅ ROBUST FIX: Parse structured errors from compile tool JSON
					// The compile tool provides structured error objects with file/line/message
					var result map[string]interface{}
					if err := json.Unmarshal([]byte(compileResult), &result); err == nil {
						// Extract structured errors array
						if errorsArray, ok := result["errors"].([]interface{}); ok && len(errorsArray) > 0 {
							// Build clean error message from structured data
							var errorLines []string
							for _, errObj := range errorsArray {
								if errMap, ok := errObj.(map[string]interface{}); ok {
									file := errMap["file"].(string)
									line := int(errMap["line"].(float64))
									message := errMap["message"].(string)

									errorLines = append(errorLines, fmt.Sprintf("%s:%d: %s", file, line, message))

									// Use the FIRST error's file as the target to fix
									if targetFile == compileTarget {
										// Make absolute path
										if !filepath.IsAbs(file) {
											targetFile = filepath.Join(compileTarget, file)
										} else {
											targetFile = file
										}
									}
								}
							}
							errorMsg = strings.Join(errorLines, "\n")
							fmt.Printf("[AgentExecutor] ✓ Extracted %d structured errors, target file: %s\n",
								len(errorsArray), targetFile)
						} else {
							// Fallback: use raw output field
							if output, ok := result["output"].(string); ok && output != "" {
								errorMsg = output
								fmt.Printf("[AgentExecutor] Using raw output field from JSON\n")
							} else {
								errorMsg = compileResult
								fmt.Printf("[AgentExecutor] No structured errors, using full result\n")
							}
						}
					} else {
						errorMsg = compileResult
						fmt.Printf("[AgentExecutor] Failed to parse JSON: %v\n", err)
					}
				}

				// ✅ RECORD ERROR IN TASKCONTEXT for learning
				if ate.taskContext != nil {
					// Try to extract file and line number from compile error
					// Go format: "file.go:10:5: error message"
					filePath := compileTarget
					lineNumber := 0

					parts := strings.Split(errorMsg, ":")
					if len(parts) >= 3 {
						if num, err := strconv.Atoi(parts[1]); err == nil {
							lineNumber = num
						}
					}

					ate.taskContext.AddError(ExecutionError{
						Timestamp:   ate.taskContext.CreatedAt,
						Agent:       "AgentExecutor",
						Phase:       "compilation",
						ErrorType:   "compile_error",
						Message:     errorMsg,
						FilePath:    filePath,
						LineNumber:  lineNumber,
						CodeSnippet: "", // Could extract from file
						Suggestion:  "Fix compilation errors in generated code",
					})
				}

				// ✅ HYBRID APPROACH: Analyze error and decide action
				analysis := feedback.AnalyzeCompileError(errorMsg)
				fmt.Printf("[AgentExecutor] Compile error detected: %s (severity: %s, files: %d, errors: %d)\n",
					analysis.ErrorType, analysis.Severity, analysis.FilesCount, analysis.ErrorCount)

				// SIMPLE error + haven't exceeded max attempts → iterative fix
				const maxSimpleFixAttempts = 3
				if analysis.Severity == feedback.ErrorSeveritySimple && i < maxSimpleFixAttempts {
					fmt.Printf("[AgentExecutor] Attempting iterative fix (attempt %d/%d)\n",
						i+1, maxSimpleFixAttempts)

					// Add error context to conversation for LLM to fix
					conversationHistory = append(conversationHistory, llm.Message{
						Role:    "assistant",
						Content: fmt.Sprintf(`{"tool": "compile", "arguments": {"path": "%s"}}`, compileTarget), // ✅ FIX: Use "path" not "target"
					})
					// ✅ INJECT FILE CONTENT: Read the target file and inject into prompt
					fixPrompt := analysis.FormatFixPrompt(errorMsg)
					if targetFile != "" && targetFile != compileTarget {
						if content, err := os.ReadFile(targetFile); err == nil {
							fixPrompt += fmt.Sprintf("\n\n**Current File Content** (%s):\n```go\n%s\n```\n\n", targetFile, string(content))
							fixPrompt += "**CRITICAL: Use edit_line tool ONLY**\n"
							fixPrompt += "Extract line number from error, then call: {\"tool\": \"edit_line\", \"arguments\": {\"path\": \"...\", \"line\": N, \"new_content\": \"fixed line\"}}\n"
						} else {
							fmt.Printf("[AgentExecutor] Warning: Could not read %s for fix context: %v\n", targetFile, err)
						}
					}

					conversationHistory = append(conversationHistory, llm.Message{
						Role:    "user",
						Content: fixPrompt,
					})

					// Continue to next iteration - LLM will attempt to fix
					continue
				}

				// COMPLEX error OR max simple attempts exceeded → send feedback
				if ate.taskContext == nil {
					// No task context - can't use feedback loop, return error directly
					fmt.Printf("[AgentExecutor] %s but no task context available - cannot create fix task\n",
						func() string {
							if analysis.Severity == feedback.ErrorSeverityComplex {
								return "Complex error detected"
							}
							return "Max iterative fix attempts exceeded"
						}())
					result.Error = fmt.Sprintf("Compilation failed with %s: %s", analysis.ErrorType, errorMsg)
					return result, fmt.Errorf("compilation failed: %s", analysis.ErrorType)
				}

				fmt.Printf("[AgentExecutor] %s - sending feedback for separate fix task\n",
					func() string {
						if analysis.Severity == feedback.ErrorSeverityComplex {
							return "Complex error detected"
						}
						return "Max iterative fix attempts exceeded"
					}())

				if true { // Always enter this block now that we've checked taskContext above
					// Create base agent to send feedback
					baseAgent := &BaseAgent{
						name:           "AgentExecutor",
						currentTaskID:  ate.taskContext.TaskID,
						currentContext: ate.taskContext,
					}

					// Send feedback requesting fix task

					feedbackCtx := map[string]interface{}{
						// Required fields for dependency handler
						"dependency_description": fmt.Sprintf("Fix %s in %s", analysis.ErrorType, filepath.Base(targetFile)),
						"dependency_type":        "code",
						// Additional context
						"error_message":  errorMsg, // Clean, structured error message
						"error_type":     analysis.ErrorType,
						"severity":       string(analysis.Severity),
						"affected_files": analysis.FilesCount,
						"error_count":    analysis.ErrorCount,
						"target_path":    compileTarget, // Directory
						"target_file":    targetFile,    // ✅ CRITICAL: Specific file to fix
						"suggestion":     analysis.Suggestion,
					}

					// ✅ Use SendAndWait to block until fix task completes
					fmt.Printf("[AgentExecutor] Sending feedback and waiting for fix task...\n")
					err := baseAgent.SendFeedbackAndWait(ctx,
						agent.FeedbackTypeDependencyNeeded,
						agent.FeedbackSeverityCritical,
						fmt.Sprintf("Compilation errors need fixing: %s", analysis.ErrorType),
						feedbackCtx,
						"Create a fix task to resolve compilation errors")

					if err != nil {
						// Fix task failed
						fmt.Printf("[AgentExecutor] Fix task failed: %v\n", err)
						result.Error = fmt.Sprintf("Compilation failed and fix task failed: %v", err)
						return result, fmt.Errorf("compilation and fix both failed: %w", err)
					}

					// ✅ Fix task succeeded! Compile again to verify
					fmt.Printf("[AgentExecutor] Fix task completed - recompiling to verify...\n")
					recompileCall := ToolCall{
						Tool: "compile",
						Arguments: map[string]interface{}{
							"path": compileTarget,
						},
					}

					recompileResult, recompileErr := ate.executor.Execute(ctx, recompileCall)
					result.ToolsExecuted = append(result.ToolsExecuted, "compile")

					// Debug logging
					fmt.Printf("[AgentExecutor] Recompile result: err=%v, success_in_json=%v\n",
						recompileErr, strings.Contains(recompileResult, `"success": true`))
					if !strings.Contains(recompileResult, `"success": true`) {
						fmt.Printf("[AgentExecutor] Recompile output: %s\n", recompileResult)
					}

					if recompileErr != nil || !strings.Contains(recompileResult, `"success": true`) {
						// Still failing after fix
						result.Error = fmt.Sprintf("Fix task completed but compilation still fails: %v", recompileErr)
						return result, fmt.Errorf("post-fix compilation failed")
					}

					// ✅ SUCCESS! Fixed and recompiled successfully
					result.ToolResults = append(result.ToolResults, recompileResult)
					printSuccess("Code fixed and compiled successfully!")
					result.Success = true
					result.Output = fmt.Sprintf("Code generated, fixed, and compiled successfully.\n\nTools used: %v", result.ToolsExecuted)
					return result, nil
				}

				// Compilation failed but no fix task (shouldn't happen)
				result.Error = fmt.Sprintf("Compilation failed: %v", compileErr)
				return result, fmt.Errorf("compilation failed: %w", compileErr)
			}

			result.ToolResults = append(result.ToolResults, compileResult)

			// Update conversation history with the compile action
			conversationHistory = append(conversationHistory, llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf(`{"tool": "compile", "arguments": {"path": "%s"}}`, compileTarget),
			})
			conversationHistory = append(conversationHistory, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool 'compile' executed successfully (auto-injected). Result:\n\n%s", compileResult),
			})

			// ✅ SUCCESS: Compilation worked!
			// ATOMIC TASK PRINCIPLE: Exit immediately after successful compilation
			// Each subtask should do ONE thing: generate 1 file, make 1 change
			// ManagerAgent orchestrates the sequence of subtasks
			printStatus("Compilation successful")
			result.Success = true
			result.Output = fmt.Sprintf("Code generated and compiled successfully.\n\nTools used: %v", result.ToolsExecuted)
			return result, nil
		}

		// Add this exchange to conversation history
		conversationHistory = append(conversationHistory, llm.Message{
			Role:    "assistant",
			Content: fmt.Sprintf(`{"tool": "%s", "arguments": %s}`, toolCall.Tool, formatArgsForAgent(toolCall.Arguments)),
		})

		// ✅ CRITICAL FIX: Terminal tools should exit immediately, not ask LLM "what's next?"
		// Delegation tools like orchestrate_code_task handle the entire task internally
		// Asking the LLM for next steps causes it to hallucinate and call the tool again
		terminalTools := map[string]bool{
			"orchestrate_code_task": true,
			"delegate_task":         true,
		}

		if terminalTools[toolCall.Tool] {
			// Terminal tool completed - return immediately without asking LLM for next step
			result.Success = true
			result.Output = toolResult
			return result, nil
		}

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
		// 		coord := GetGlobalCoordinator()
		// 		if coord != nil && taskID != "" {
		// 			coord.UpdateTaskProgress(taskID, fmt.Sprintf("Planning next step after %s...", toolCall.Tool), result.ToolsExecuted)
		// 		}

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
