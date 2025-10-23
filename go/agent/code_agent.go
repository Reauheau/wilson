package agent

import (
	"context"
	"fmt"
	"os"
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

// ExecuteWithContext executes a task with full TaskContext
// This is the preferred method - provides rich context for feedback and learning
func (a *CodeAgent) ExecuteWithContext(ctx context.Context, taskCtx *TaskContext) (*Result, error) {
	// Store context for feedback access
	a.SetTaskContext(taskCtx)

	// Convert to Task and execute
	task := a.ConvertTaskContextToTask(taskCtx)
	return a.Execute(ctx, task)
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

	// ✅ PRECONDITION CHECK - Validate prerequisites before execution
	if err := a.checkPreconditions(ctx, task); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Precondition failed: %v", err)
		a.RecordError("precondition_failed", "precondition", err.Error(), "", 0,
			"Ensure prerequisites are met")
		return result, err
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
	// No retries needed - atomic task principle, workflow validation in agent_executor
	maxWorkflowRetries := 1

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
			task.ID,          // Pass task ID for progress updates
			a.currentContext, // Pass TaskContext for path extraction (Phase 2)
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
		// Note: Workflow validation is now handled by agent_executor.go auto-injection
		// No retry logic needed here - atomic task principle means each task does one thing

		// Workflow validation passed - continue to verification
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

// checkPreconditions validates prerequisites with TaskContext awareness
func (a *CodeAgent) checkPreconditions(ctx context.Context, task *Task) error {
	// Check 1: Target directory exists
	projectPath := "."

	// ✅ Use TaskContext if available (better than parsing task.Input)
	if a.currentContext != nil && a.currentContext.ProjectPath != "" {
		projectPath = a.currentContext.ProjectPath
	} else if pathVal, ok := task.Input["project_path"]; ok {
		if pathStr, ok := pathVal.(string); ok && pathStr != "" {
			projectPath = pathStr
		}
	}

	// If not current directory, verify it exists
	if projectPath != "." {
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			return a.RequestDependency(ctx,
				fmt.Sprintf("Create directory %s", projectPath),
				ManagedTaskTypeCode,
				fmt.Sprintf("Target directory does not exist: %s", projectPath))
		}
	}

	// Check 2: For "fix" tasks, verify file exists
	if fixMode, ok := task.Input["fix_mode"].(bool); ok && fixMode {
		if targetFile, ok := task.Input["target_file"].(string); ok {
			if _, err := os.Stat(targetFile); os.IsNotExist(err) {
				// Don't request dependency for fix tasks on missing files - this is a real error
				return fmt.Errorf("cannot fix non-existent file: %s", targetFile)
			}
		}
	}

	// Check 3: For tasks with compile errors, verify the file being fixed exists
	if compileError, ok := task.Input["compile_error"].(string); ok && compileError != "" {
		if targetFile, ok := task.Input["target_file"].(string); ok {
			if _, err := os.Stat(targetFile); os.IsNotExist(err) {
				return fmt.Errorf("cannot fix compile errors in non-existent file: %s", targetFile)
			}
		}
	}

	return nil
}

func (a *CodeAgent) buildSystemPrompt() string {
	// Start with shared core principles
	prompt := BuildSharedPrompt("Code Agent")

	// Add Code Agent specific instructions
	prompt += `
You orchestrate code generation. You delegate to specialized code models via tools.

=== YOUR ROLE ===

Generate ONE file per task. You are part of a multi-task workflow managed by the system.

**Atomic Task Principle:**
- Each task = ONE file or ONE code change
- ManagerAgent coordinates multiple file workflows
- You focus on executing YOUR assigned task only

=== WORKFLOW (AUTOMATIC) ===

1. **generate_code** - You call this with requirements
   → Code generation model creates code

2. **[AUTO]** write_file - System saves to filesystem
   → Automatic after generate_code

3. **[AUTO]** compile - System validates compilation
   → Automatic after write_file
   → If errors: You get another chance to fix

4. **EXIT** - Task complete after successful compilation
   → Next task (if any) handled by ManagerAgent

=== TOOL USAGE ===

**generate_code** - Primary tool for code creation
{"tool": "generate_code", "arguments": {
  "language": "go",
  "description": "What this file should do",
  "requirements": ["Requirement 1", "Requirement 2"]
}}

**read_file** - Understand existing code before modifying
{"tool": "read_file", "arguments": {"path": "existing.go"}}

**modify_file** - Change existing code
{"tool": "modify_file", "arguments": {"path": "file.go", "old_content": "...", "new_content": "..."}}

=== EXAMPLE TASKS ===

Task: "Implement main.go for app opener"
→ {"tool": "generate_code", "arguments": {
    "language": "go",
    "description": "Main program that opens macOS applications",
    "requirements": ["CLI interface", "Error handling", "Uses exec.Command"]
}}
→ System auto-saves and compiles
→ Done (1 file created)

Task: "Write tests for code in /project"
→ **CRITICAL: Discover and read source files first!**
→ {"tool": "list_files", "arguments": {"directory": "/project"}}
→ Find source files (e.g., main.go, handler.js, app.py)
→ {"tool": "read_file", "arguments": {"path": "/project/[discovered_file]"}}
→ Analyze: What functions? What logic? What needs testing?
→ {"tool": "generate_code", "arguments": {
    "language": "[same as source]",
    "description": "Test file for [discovered_file] functionality",
    "requirements": ["Test function X", "Test error case Y", "Test edge case Z"]
}}
→ System auto-saves test file with proper naming convention
→ Done (context-aware tests that actually test the real code)

Task: "Add error logging to auth.go"
→ Read auth.go first to understand structure
→ Use modify_file to add logging
→ Done (1 file modified)

Task: "Fix compilation error in validator.go"
→ Read error message
→ Use generate_code with error context to create fixed version
→ Done (1 file fixed)

=== ERROR HANDLING ===

Compilation errors (from auto-compile):
1. Read error carefully - what's actually wrong?
2. Call generate_code again with error as context
3. System re-saves and re-compiles
4. Repeat if needed (up to system limit)

Fix by providing better instructions to generate_code, not by manually editing.

=== QUALITY STANDARDS ===

Code you generate should:
- Match project conventions (read existing code first if unsure)
- Handle errors appropriately
- Include necessary imports
- Be well-structured and readable
- Work on first compile (test requirements in your mind before generating)

Remember: You orchestrate. The code model generates. The system handles saving and compiling.`

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

		// SPECIAL HANDLING: If this is a test file creation task with dependency files
		if fileType, ok := task.Input["file_type"].(string); ok && fileType == "test" {
			if depFiles, ok := task.Input["dependency_files"].([]string); ok && len(depFiles) > 0 {
				prompt.WriteString("⚠️  IMPORTANT: This is a TEST FILE task.\n")
				prompt.WriteString("Files created by previous task:\n")
				for _, file := range depFiles {
					prompt.WriteString(fmt.Sprintf("  - %s\n", file))
				}
				prompt.WriteString("\n→ Read those files using read_file to understand what to test\n")
				prompt.WriteString("→ Generate tests that actually test the real code functionality\n")
				prompt.WriteString("→ Do NOT generate generic template tests\n\n")
			} else if projectPath, ok := task.Input["project_path"].(string); ok {
				// Fallback: no dependency files provided, need to discover
				prompt.WriteString("⚠️  IMPORTANT: This is a TEST FILE task.\n")
				prompt.WriteString(fmt.Sprintf("→ First: Use list_files to find source files in %s\n", projectPath))
				prompt.WriteString("→ Then: Read those files using read_file\n")
				prompt.WriteString("→ Finally: Generate appropriate tests\n\n")
			}
		}
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
