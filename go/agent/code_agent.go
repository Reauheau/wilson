package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/core/registry"
	"wilson/llm"
)

// CodeAgent specializes in code generation, analysis, and refactoring
type CodeAgent struct {
	*BaseAgent
}

// NewCodeAgent creates a new code agent
func NewCodeAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *CodeAgent {
	// CRITICAL: Use chat model instead of code model
	// qwen2.5:7b is much better at structured output (tool calls) than qwen2.5-coder:14b
	// The code model tends to hallucinate descriptions instead of generating JSON
	base := NewBaseAgent("Code", llm.PurposeChat, llmManager, contextMgr)

	// Code-specific tools
	base.SetAllowedTools([]string{
		// File reading
		"read_file",
		"search_files",
		"list_files",
		// File writing (critical for code generation!)
		"write_file",     // Create new files
		"modify_file",    // Replace existing content
		"append_to_file", // Add new functions/content to existing files
		// Code generation (CRITICAL - use this instead of writing code yourself!)
		"generate_code", // Calls specialist code model to generate actual code
		// Code intelligence (Phase 1)
		"parse_file",        // Understand code structure via AST
		"find_symbol",       // Find definitions and usages
		"analyze_structure", // Analyze package/file structure
		"analyze_imports",   // Analyze and manage imports
		// Compilation & iteration (Phase 2)
		"compile",   // Run go build and capture errors
		"run_tests", // Execute tests and capture results
		// Cross-file awareness (Phase 3)
		"dependency_graph", // Map import relationships
		"find_related",     // Find related files
		"find_patterns",    // Discover code patterns
		// Quality gates (Phase 4)
		"format_code",      // Auto-format code
		"lint_code",        // Check style/best practices
		"security_scan",    // Scan for vulnerabilities
		"complexity_check", // Check code complexity
		"coverage_check",   // Verify test coverage
		"code_review",      // Comprehensive quality check
		// Review workflow (ENDGAME Phase 3)
		"request_review",    // Request review of completed work
		"get_review_status", // Check review status and feedback
		// Autonomous coordination (ENDGAME Phase 4)
		"poll_tasks",           // Poll for available tasks
		"claim_task",           // Claim a task to work on
		"update_task_progress", // Update task progress
		"unblock_tasks",        // Unblock dependent tasks
		"get_task_queue",       // View task queue status
		// Context and artifacts
		"search_artifacts",
		"retrieve_context",
		"store_artifact",
		"leave_note",
	})
	base.SetCanDelegate(false)

	return &CodeAgent{
		BaseAgent: base,
	}
}

// CanHandle checks if the code agent can handle a task
func (a *CodeAgent) CanHandle(task *Task) bool {
	return task.Type == TaskTypeCode
}

// Execute executes a code-related task using the 3-layer architecture
// Layer 1: INTENT (LLM Planning)
// Layer 2: EXECUTION (Actual Tool Calls)
// Layer 3: VERIFICATION (Result Validation)
func (a *CodeAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
	}

	// Get current context for background
	currentCtx, err := a.GetContext()
	if err != nil {
		currentCtx = nil
	}

	// === LAYER 1: INTENT - Get LLM plan ===
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Update progress
	coordinator := GetGlobalCoordinator()
	if coordinator != nil {
		coordinator.UpdateTaskProgress(task.ID, "Waiting for LLM response...", nil)
	}

	var execResult *ExecutionResult
	maxWorkflowRetries := 2 // Allow one retry if workflow validation fails

	for attempt := 1; attempt <= maxWorkflowRetries; attempt++ {
		// Use validated LLM call with automatic retry
		response, err := CallLLMWithValidation(ctx, a.llmManager, a.purpose, systemPrompt, userPrompt, 5, task.ID)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("LLM validation error: %v", err)
			return result, err
		}

		// === LAYER 2: EXECUTION - Actually run the tools ===
		executor := NewAgentToolExecutor(
			registry.NewExecutor(),
			a.llmManager,
		)

		execResult, err = executor.ExecuteAgentResponse(
			ctx,
			response,
			systemPrompt,
			userPrompt,
			a.purpose,
			task.ID, // Pass task ID for progress updates
		)

		if err != nil {
			// Execution failed or hallucination detected
			result.Success = false
			result.Error = fmt.Sprintf("Execution failed: %v", err)
			result.Output = execResult.Output

			if execResult.HallucinationDetected {
				result.Error = "Code Agent hallucinated: provided description instead of using tools"
			}

			return result, err
		}

		// === LAYER 2.5: WORKFLOW VALIDATION - Check mandatory sequences ===
		if err := validateCodeWorkflow(execResult.ToolsExecuted); err != nil {
			if attempt < maxWorkflowRetries {
				// Retry with feedback
				fmt.Printf("\n❌ [Workflow Validation] Attempt %d/%d failed: %v\n", attempt, maxWorkflowRetries, err)
				fmt.Printf("   Tools executed: %v\n", execResult.ToolsExecuted)
				fmt.Printf("   Retrying with corrective feedback...\n\n")

				// Extract the path from task description
				targetPath := "/Users/roderick.vannievelt/IdeaProjects/wilsontestdir"
				if strings.Contains(task.Description, "wilsontestdir") {
					// Keep the path
				} else if strings.Contains(strings.ToLower(task.Description), "test") {
					// Likely a test file
					targetPath = targetPath + "/main_test.go"
				} else {
					targetPath = targetPath + "/main.go"
				}

				// Simplified retry - just tell it what's missing
				userPrompt = fmt.Sprintf(`Task incomplete. You executed: %v

Missing step: %s

Call the missing tool now.`,
					execResult.ToolsExecuted, err.Error())

				if coordinator != nil {
					coordinator.UpdateTaskProgress(task.ID, "Retrying with workflow feedback...", nil)
				}
				continue // Retry
			}

			// Last attempt failed
			result.Success = false
			result.Error = fmt.Sprintf("Workflow validation failed after %d attempts: %v", maxWorkflowRetries, err)
			result.Output = execResult.Output
			result.Metadata = map[string]interface{}{
				"tools_executed": execResult.ToolsExecuted,
				"workflow_error": err.Error(),
			}
			return result, fmt.Errorf("workflow validation failed: %w", err)
		}

		// Workflow validation passed!
		fmt.Printf("✓ [Workflow Validation] Passed - correct tool sequence followed\n")
		break
	}

	// === LAYER 3: VERIFICATION - Check actual results ===
	verifier := GetVerifier(string(TaskTypeCode))
	if verifier != nil {
		if err := verifier.Verify(ctx, execResult, task); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Verification failed: %v", err)
			result.Output = execResult.Output
			result.Metadata = map[string]interface{}{
				"tools_executed":     execResult.ToolsExecuted,
				"verification_error": err.Error(),
			}
			return result, err
		}
	}

	// === SUCCESS - All three layers passed ===

	// Store execution summary as artifact
	artifactContent := fmt.Sprintf("Code Generation Task Completed\n\n")
	artifactContent += fmt.Sprintf("Tools Executed: %s\n\n", strings.Join(execResult.ToolsExecuted, ", "))
	artifactContent += fmt.Sprintf("Results:\n%s", execResult.Output)

	artifact, err := a.StoreArtifact(
		"code",
		artifactContent,
		"code_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	// Leave note for other agents
	noteText := fmt.Sprintf("✓ Completed code task: %s. Created %d file(s) using tools: %s. Ready for testing.",
		task.Description, len(execResult.ToolsExecuted), strings.Join(execResult.ToolsExecuted, ", "))
	_ = a.LeaveNote("Test", noteText)

	result.Success = true
	result.Output = execResult.Output
	result.Metadata = map[string]interface{}{
		"model":          "code",
		"agent_type":     "code",
		"artifact_id":    artifact.ID,
		"tools_executed": execResult.ToolsExecuted,
		"verified":       true,
	}

	return result, nil
}

// truncate returns first n characters of string
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (a *CodeAgent) buildSystemPrompt() string {
	// Start with shared core principles
	prompt := BuildSharedPrompt("Code Agent")

	// Add Code Agent specific instructions
	prompt += `
You are the CODE ORCHESTRATOR. You coordinate code development using tools.

=== YOUR ROLE ===

You do NOT write code yourself. You ORCHESTRATE code generation.
You delegate to the code model via generate_code tool, then save and validate.

=== WORKFLOW (MANDATORY SEQUENCE) ===

1. **generate_code** - Get code from specialist model
   → Returns actual working code

2. **[AUTO-INJECTED]** - System saves code automatically
   → write_file happens automatically after generate_code

3. **compile** - REQUIRED - Validate code works
   → ALWAYS call compile after code is saved
   → Check for errors, fix if needed

4. **STOP** - Task complete once compile succeeds

CRITICAL: After generate_code completes, you MUST call compile. Don't stop without compiling.

=== TOOL USAGE RULES ===

**generate_code** - For ALL code creation
{"tool": "generate_code", "arguments": {
  "language": "go",
  "description": "What the code should do",
  "requirements": ["List", "of", "requirements"]
}}

**compile** - After code is saved
{"tool": "compile", "arguments": {"target": "/path/to/project"}}

**run_tests** - If tests exist
{"tool": "run_tests", "arguments": {"package": "/path"}}

=== CODE UNDERSTANDING TOOLS ===

Before implementing, understand context:
- **parse_file**: Read code structure (AST)
- **find_symbol**: Locate definitions/usages
- **find_patterns**: Learn existing style
- **find_related**: Find dependencies

=== EXAMPLE WORKFLOW ===

Task: "Create Go program that opens Spotify"

Step 1: Generate
{"tool": "generate_code", "arguments": {
  "language": "go",
  "description": "CLI that opens applications using exec.Command",
  "requirements": ["macOS support", "Error handling"]
}}

Step 2: [System auto-saves to main.go]

Step 3: Compile
{"tool": "compile", "arguments": {"target": "/path"}}

Step 4: Done (if compile succeeds)

=== ERROR RECOVERY ===

If compile fails:
1. Read error message carefully
2. Call generate_code with error feedback
3. Let system save new code automatically
4. Compile again
5. Repeat until success

=== SECURITY & QUALITY ===

**Security Checks:**
- Never log credentials
- Validate user inputs
- Check for SQL injection risks
- Scan for vulnerabilities

**Quality Standards:**
- Match existing code style
- Include error handling
- Add clear comments
- Keep functions focused
- Test coverage 80%+

=== AVAILABLE TOOLS ===

**Code Generation:**
- generate_code: Get code from specialist model

**File Operations:**
- read_file, list_files, search_files
- modify_file, append_to_file (for existing files)

**Validation:**
- compile, run_tests
- format_code, lint_code
- security_scan, complexity_check

**Code Intelligence:**
- parse_file, find_symbol, find_patterns
- analyze_structure, analyze_imports
- dependency_graph, find_related

**Context Management:**
- search_artifacts, retrieve_context
- store_artifact, leave_note

Remember: You orchestrate. The code model generates. The system saves.`

	return prompt
}

func (a *CodeAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description))

	// Include input parameters if any
	if len(task.Input) > 0 {
		prompt.WriteString("Input parameters:\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	// Include relevant context artifacts if available
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("Consider these existing artifacts:\n\n")
		for i, artifact := range currentCtx.Artifacts {
			if i >= 5 { // Limit to last 5
				break
			}
			// Show code artifacts in more detail
			if artifact.Type == "code" || artifact.Type == "analysis" {
				summary := artifact.Content
				if len(summary) > 300 {
					summary = summary[:300] + "..."
				}
				prompt.WriteString(fmt.Sprintf("- **Artifact #%d** [%s]: %s\n\n", artifact.ID, artifact.Type, summary))
			}
		}
	}

	// Add task input parameters
	if len(task.Input) > 0 {
		prompt.WriteString("## Specifications\n\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("## Deliverables\n\n")
	prompt.WriteString("Provide:\n")
	prompt.WriteString("1. Complete, working code implementation\n")
	prompt.WriteString("2. Clear documentation and comments\n")
	prompt.WriteString("3. Usage examples\n")
	prompt.WriteString("4. Any assumptions or design decisions\n")
	prompt.WriteString("5. Suggested test cases for validation\n")

	return prompt.String()
}

// validateCodeWorkflow checks if mandatory tool sequences were followed
// Required sequence: generate_code → write_file → compile
func validateCodeWorkflow(toolsExecuted []string) error {
	generateCodeIndex := -1
	writeFileIndex := -1
	compileIndex := -1

	// Find indices of key operations
	for i, tool := range toolsExecuted {
		if tool == "generate_code" {
			generateCodeIndex = i
		}
		if tool == "write_file" || tool == "modify_file" || tool == "append_to_file" {
			// Only count file operations AFTER generate_code
			if generateCodeIndex >= 0 && i > generateCodeIndex && writeFileIndex == -1 {
				writeFileIndex = i
			}
		}
		if tool == "compile" {
			// Only count compile AFTER write_file
			if writeFileIndex >= 0 && i > writeFileIndex && compileIndex == -1 {
				compileIndex = i
			}
		}
	}

	// If generate_code was called, verify the full sequence
	if generateCodeIndex >= 0 {
		if writeFileIndex == -1 {
			return fmt.Errorf("generate_code was called but code was never saved (missing write_file AFTER generate_code)")
		}
		if compileIndex == -1 {
			return fmt.Errorf("code was generated and saved but never compiled (missing compile AFTER write_file)")
		}
	}

	return nil
}
