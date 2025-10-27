package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wilson/agent"
	"wilson/agent/base"
	"wilson/agent/feedback"
	"wilson/agent/orchestration"
	"wilson/agent/validation"

	contextpkg "wilson/context"
	"wilson/core/registry"
	"wilson/llm"
	"wilson/ui"
)

// CodeAgent specializes in code generation, analysis, and refactoring
type CodeAgent struct {
	*base.BaseAgent
	llmManager *llm.Manager
}

// NewCodeAgent creates a new code agent
func NewCodeAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *CodeAgent {
	// Use orchestration model for tool calling (hermes3:8b)
	// Specialized for reliable JSON generation and precise tool selection
	base := base.NewBaseAgent("Code", llm.PurposeOrchestration, llmManager, contextMgr)

	// Code-specific tools
	base.SetAllowedTools([]string{
		// File reading
		"read_file",
		"search_files",
		"list_files",
		// File writing (critical for code generation!)
		"write_file",     // Create new files
		"modify_file",    // Replace existing content (use for multi-line changes)
		"edit_line",      // Edit specific line by line number (use for single-line fixes)
		"append_to_file", // Add new functions/content to existing files
		// Code generation (CRITICAL - use this instead of writing code yourself!)
		"generate_code", // Calls specialist code model to generate actual code
		// ===== LSP Code Intelligence (Phase 1) - PREFERRED =====
		"get_diagnostics",      // Real-time errors/warnings from language server (CRITICAL)
		"go_to_definition",     // Find where symbol is defined (use instead of grep)
		"find_references",      // Find all usages of symbol (use for impact analysis)
		"get_hover_info",       // Get signature and documentation (fast lookup)
		"get_symbols",          // List functions/types in file or search workspace
		"find_implementations", // Find types implementing interface (Phase 2)
		"get_type_definition",  // Jump to type definition of variable (Phase 2)
		"rename_symbol",        // Safely rename symbols across workspace (Phase 2 Extended)
		// ===== Legacy AST Tools (use only when LSP not available) =====
		"parse_file",      // Deep AST analysis (use only for advanced cases)
		"analyze_imports", // Analyze and manage imports (no LSP equivalent yet)
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
		BaseAgent:  base,
		llmManager: llmManager,
	}
}

// CanHandle checks if the code agent can handle a task
func (a *CodeAgent) CanHandle(task *agent.Task) bool {
	return task.Type == agent.TaskTypeCode
}

// ExecuteWithContext executes a task with full TaskContext
// This is the preferred method - provides rich context for feedback and learning
func (a *CodeAgent) ExecuteWithContext(ctx context.Context, taskCtx *base.TaskContext) (*agent.Result, error) {
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
func (a *CodeAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	result := &agent.Result{
		TaskID: task.ID,
		Agent:  a.Name(),
	}

	// ✅ PRECONDITION CHECK - Validate prerequisites before execution
	if err := a.checkPreconditions(ctx, task); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Precondition failed: %v", err)
		a.RecordError("precondition_failed", "precondition", err.Error(), "", 0,
			"Ensure prerequisites are met")
		return result, err
	}

	// ✅ CRITICAL FIX: For ALL fix tasks, remove generate_code - force surgical editing only
	if fixMode, ok := task.Input["fix_mode"].(bool); ok && fixMode {
		// Save original tools
		originalTools := a.AllowedTools()

		// Create filtered list WITHOUT generate_code
		filteredTools := make([]string, 0)
		for _, tool := range originalTools {
			if tool != "generate_code" {
				filteredTools = append(filteredTools, tool)
			}
		}
		a.SetAllowedTools(filteredTools)
		fmt.Printf("[CodeAgent] Fix mode: removed generate_code, enforcing surgical edits (edit_line/modify_file)\n")

		// Restore tools after execution
		defer func() {
			a.SetAllowedTools(originalTools)
		}()
	}

	// Get current context for background
	currentCtx, err := a.GetContext()
	if err != nil {
		currentCtx = nil
	}

	// === LAYER 1: INTENT - Get LLM plan ===
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)
	// ✅ ROBUST FIX: Auto-inject source code for test tasks
	// Don't rely on LLM to figure out it needs to read files - just give it the content
	if fileType, ok := task.Input["file_type"].(string); ok && fileType == "test" {
		if depFiles, ok := task.Input["dependency_files"].([]string); ok && len(depFiles) > 0 {
			userPrompt += "\n\n=== SOURCE CODE TO TEST ===\n"
			for _, file := range depFiles {
				content, err := os.ReadFile(file)
				if err == nil {
					userPrompt += fmt.Sprintf("\n**File: %s**\n```go\n%s\n```\n", file, string(content))
				} else {
					userPrompt += fmt.Sprintf("\n**File: %s** (could not read: %v)\n", file, err)
				}
			}
			userPrompt += "\n→ Generate unit tests for the functions/methods in the above code.\n"
			userPrompt += "→ Do NOT try to test main() or CLI I/O. Focus on testable functions.\n"
			userPrompt += "→ IMPORTANT: Do NOT redeclare or redefine the functions - they already exist in the source files above!\n"
			userPrompt += "→ Just write test functions (TestXxx) that CALL the existing functions.\n"
		}
	}

	// Update progress
	// TODO: Add callback interface to avoid import cycle
	// coordinator := GetGlobalCoordinator()
	// if coordinator != nil {
	// 	coordinator.UpdateTaskProgress(task.ID, "Waiting for LLM response...", nil)
	// }

	// ✅ REMOVED: Redundant maxWorkflowRetries loop that only ran once
	// Retries are now handled by:
	// 1. agent_executor iterative fix (up to 3 attempts for simple errors)
	// 2. Feedback loop (for complex errors)
	// This separation is cleaner and follows Wilson's feedback-driven architecture

	// Use validated LLM call with automatic retry
	response, err := base.CallLLMWithValidation(ctx, a.llmManager, a.Purpose(), systemPrompt, userPrompt, 5, task.ID)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM validation error: %v", err)
		return result, err
	}

	// === LAYER 2: EXECUTION - Actually run the tools ===
	executor := base.NewAgentToolExecutor(
		registry.NewExecutor(),
		a.llmManager,
	)

	// ✅ FIX: Get TaskContext from agent if available (set by ExecuteWithContext)
	// This enables the feedback loop to work properly for compile error fixes
	// Access currentContext field directly (no getter method exists)
	taskCtx := a.GetCurrentContext()

	execResult, err := executor.ExecuteAgentResponse(
		ctx,
		response,
		systemPrompt,
		userPrompt,
		a.Purpose(),
		task.ID, // Pass task ID for progress updates
		taskCtx, // Pass TaskContext for feedback loop support
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
	// Each task does one thing cleanly - ManagerAgent coordinates the sequence

	// === LAYER 3: VERIFICATION - Check actual results ===
	verifier := validation.GetVerifier(string(agent.TaskTypeCode))
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

	// Extract created files for dependency tracking
	createdFiles := []string{}
	if verifier != nil {
		if codeVerifier, ok := verifier.(*validation.CodeTaskVerifier); ok {
			createdFiles = codeVerifier.ExtractCreatedFiles(execResult)
			if len(createdFiles) > 0 {
				ui.Printf("[CodeAgent] Extracted created files: %v\n", createdFiles)
			} else {
				ui.Printf("[CodeAgent] Warning: No files extracted from tools: %v\n", execResult.ToolsExecuted)
			}
		}
	}

	// Store execution summary as artifact
	artifactContent := fmt.Sprintf("Code Generation Task Completed\n\n")
	artifactContent += fmt.Sprintf("Tools Executed: %s\n\n", strings.Join(execResult.ToolsExecuted, ", "))
	if len(createdFiles) > 0 {
		artifactContent += fmt.Sprintf("Created Files: %s\n\n", strings.Join(createdFiles, ", "))
	}
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
		"created_files":  createdFiles, // ✅ CRITICAL: Track created files for dependency injection
	}

	return result, nil
}

// checkPreconditions validates prerequisites with TaskContext awareness
func (a *CodeAgent) checkPreconditions(ctx context.Context, task *agent.Task) error {
	// Check 1: Target directory exists
	projectPath := "."

	// ✅ Use TaskContext if available (better than parsing task.Input)
	// Note: currentContext is only available in ExecuteWithContext, not Execute
	if pathVal, ok := task.Input["project_path"]; ok {
		if pathStr, ok := pathVal.(string); ok && pathStr != "" {
			projectPath = pathStr
		}
	}

	// If not current directory, verify it exists
	if projectPath != "." {
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			return a.RequestDependency(ctx,
				fmt.Sprintf("Create directory %s", projectPath),
				string(orchestration.ManagedTaskTypeCode),
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

	// ✅ Check 4: STALE FILE DETECTION
	// If creating a test file, check for conflicting test files from previous runs
	if fileType, ok := task.Input["file_type"].(string); ok && fileType == "test" {
		// Check if test files already exist in the target directory
		if projectPath != "." {
			testFiles, err := filepath.Glob(filepath.Join(projectPath, "*_test.go"))
			if err == nil && len(testFiles) > 0 {
				// Stale test files detected - log warning
				fmt.Printf("[CodeAgent] ⚠️  STALE FILES DETECTED: Found %d existing test files in %s\n",
					len(testFiles), projectPath)
				fmt.Printf("[CodeAgent] These may conflict with new test generation. Files: %v\n", testFiles)
				// For now, just log - future: could auto-delete or merge
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

**FOR NEW FILES:**
1. **generate_code** - Create new code
   → System auto-saves to file
   → System auto-compiles

**FOR FIXING ERRORS:**
1. **ALWAYS** see the file content (auto-provided in fix tasks)
2. **edit_line** - For single-line fixes (PREFERRED for simple errors)
   → Specify line number from error message
   → System auto-compiles after edit
3. **modify_file** - For multi-line changes
   → Specify exact old/new content
   → System auto-compiles after modification
4. **generate_code** - ONLY for complex logic rewrites
   → Use as last resort when simple edits won't work

=== TOOL SELECTION GUIDE ===

**edit_line** - BEST for fixing compilation errors ⭐
- Error gives line number → use that line number
- Change only what's broken
- Preserves formatting automatically
{"tool": "edit_line", "arguments": {"path": "file.go", "line": 9, "new_content": "fixed code here"}}

**modify_file** - For multi-line surgical changes
- When you need to change multiple consecutive lines
- When exact old_content match is reliable
{"tool": "modify_file", "arguments": {"path": "file.go", "old_content": "...", "new_content": "..."}}

**generate_code** - ONLY for creating NEW files
- Initial file creation
- DO NOT use for fixes (too risky, loses context)
{"tool": "generate_code", "arguments": {"language": "go", "description": "what to create"}}

=== CODE INTELLIGENCE (LSP TOOLS) ===

**Use LSP for understanding code - faster and more accurate than grep/parse:**

**get_diagnostics** - Check for errors after making changes ⚠️
- Call after EVERY write_file, modify_file, or edit_line
- Returns real-time compiler errors and warnings from language server
- CRITICAL: Prevents broken code from reaching user
{"tool": "get_diagnostics", "arguments": {"path": "main.go"}}

**go_to_definition** - Find where something is defined 🔍
- User asks "where is Execute defined?"
- Need to understand a function before modifying it
- Following code references during analysis
- USE THIS instead of grep/search for finding definitions
{"tool": "go_to_definition", "arguments": {"file": "agent/base.go", "line": 89}}

**find_references** - Find all places a symbol is used 🔎
- Before making changes (understand impact)
- Finding all call sites of a function
- Safety check before refactoring
{"tool": "find_references", "arguments": {"file": "agent/base.go", "line": 89}}

**get_symbols** - Understand file structure 📋
- "What functions are in this file?"
- Quick overview before modifications
- FASTER than reading entire file or using parse_file
{"tool": "get_symbols", "arguments": {"file": "agent/code_agent.go"}}

**get_hover_info** - Quick documentation lookup 📖
- See function signature without reading file
- Understand parameter types and return values
{"tool": "get_hover_info", "arguments": {"file": "main.go", "line": 42}}

**find_implementations** - Find all types implementing an interface
{"tool": "find_implementations", "arguments": {"file": "agent/base.go", "line": 15}}

**get_type_definition** - Jump to type definition (see struct fields/methods)
{"tool": "get_type_definition", "arguments": {"file": "agent/executor.go", "line": 105}}

**rename_symbol** - Rename symbol across entire workspace (use find_references first!)
{"tool": "rename_symbol", "arguments": {"file": "agent/executor.go", "line": 42, "new_name": "ExecuteTask"}}

**LSP Best Practices:**
→ Use get_diagnostics after EVERY code change
→ Use go_to_definition instead of grep for finding definitions
→ Use find_references before making changes to understand impact
→ Use find_implementations when working with interfaces
→ Use get_type_definition to understand struct/type layouts
→ Use rename_symbol for safe workspace-wide renaming (check find_references first!)
→ Use get_symbols for quick file overview (faster than parse_file)
→ LSP tools work for Go, Python, JavaScript/TypeScript, and Rust
→ Note: Rust LSP is slow on first check (5-30s), then fast

=== DEPENDENCY MANAGEMENT ===

**CRITICAL: For new projects, use ONLY Go standard library**

When creating new code in an empty directory or for test scenarios:
→ Use ONLY standard library packages
→ DO NOT use external dependencies (github.com/*, gopkg.in/*, etc.)
→ This ensures code compiles without complex go.mod setup

**Standard library packages you can use:**
✅ net/http - HTTP servers and clients
✅ encoding/json - JSON encoding/decoding
✅ flag - Command-line flag parsing
✅ os - Operating system functionality
✅ io - I/O primitives
✅ fmt - Formatted I/O
✅ strings - String manipulation
✅ time - Time and date functions
✅ testing - Unit tests

**External packages to AVOID in new projects:**
❌ github.com/gorilla/mux → Use http.HandleFunc from stdlib instead
❌ github.com/gin-gonic/gin → Use net/http from stdlib instead
❌ github.com/sirupsen/logrus → Use log or fmt from stdlib instead
❌ github.com/spf13/cobra → Use flag from stdlib instead

**Example: HTTP routing WITHOUT external deps**
❌ WRONG (external dependency): github.com/gorilla/mux
✅ CORRECT (stdlib only): net/http with http.HandleFunc

**Exception:** If project already has go.mod with dependencies OR user explicitly requests a specific package, external deps are allowed.

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
→ Error shows: "./validator.go:15:20: undefined: ValidateEmail"
→ Use edit_line to fix line 15
→ System auto-compiles
→ Done (1 file fixed, 1 line changed)

=== ERROR HANDLING ===

Compilation errors contain line numbers like "./file.go:LINE:COL: message"

**Fix Strategy:**
1. Extract line number from error (e.g., "./main.go:42:5:" → line 42)
2. See the file content (automatically provided in fix tasks)
3. Use **edit_line** to fix that specific line:
   {"tool": "edit_line", "arguments": {"path": "main.go", "line": 42, "new_content": "corrected code"}}
4. System auto-compiles
5. Repeat if more errors

**Multiple errors:** Call edit_line multiple times (once per line)

**DO NOT use generate_code for fixes** - it regenerates the entire file and loses context.

=== DIAGNOSTIC-DRIVEN DEVELOPMENT ===

**CRITICAL: System automatically checks diagnostics after EVERY code change**

After you call write_file, modify_file, or edit_line, the system will:
1. Automatically call get_diagnostics (you don't need to!)
2. Show you any errors found by the language server
3. Give you a chance to fix errors before compilation

**Your responsibility:**
→ If system shows LSP errors, fix them immediately with edit_line
→ Don't ignore errors - they will cause compilation to fail
→ Use the line numbers from diagnostic messages
→ Fix errors surgically (edit_line) - don't regenerate entire file

**Example workflow (automatic):**
1. You: {"tool": "write_file", ...} → creates main.go
2. System: Automatically checks LSP diagnostics
3. System: "LSP detected 1 error: line 42: undefined variable"
4. You: {"tool": "edit_line", "arguments": {"path": "main.go", "line": 42, "new_content": "fixed"}}
5. System: Automatically checks diagnostics again
6. System: "✓ No errors" → proceeds to compilation

**Benefits:**
→ Catch errors in <500ms (vs 2-5s for compilation)
→ More accurate error messages from language server
→ Fix errors before they reach compilation
→ Better user experience (fewer failed builds)

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

func (a *CodeAgent) buildUserPrompt(task *agent.Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description))

	// ✅ FIX 2 & 3: Special handling for FIX MODE tasks
	if fixMode, ok := task.Input["fix_mode"].(bool); ok && fixMode {
		prompt.WriteString("⚠️  **FIX MODE ACTIVATED** ⚠️\n\n")
		prompt.WriteString("This is a compilation error fix task. Your goal is to fix the error by generating corrected code.\n\n")

		// ✅ CRITICAL: Show original task goal to prevent context loss
		if originalGoal, ok := task.Input["original_task_description"].(string); ok && originalGoal != "" {
			prompt.WriteString(fmt.Sprintf("🎯 **REMEMBER THE ORIGINAL GOAL**: %s\n\n", originalGoal))
			prompt.WriteString("You must fix the compilation error while staying true to this original requirement.\n\n")
		}

		// ✅ SHOW RETRY CONTEXT: If this is a repeated error, tell the LLM
		// 		if a.currentContext != nil && a.currentContext.PreviousAttempts > 0 {
		// 			prompt.WriteString(fmt.Sprintf("⚠️  **RETRY #%d**: Previous attempts failed with similar errors.\n", a.currentContext.PreviousAttempts))
		// 			prompt.WriteString("Analyze what went wrong before and try a DIFFERENT approach.\n\n")
		// 		}

		// Show error type
		if errorType, ok := task.Input["error_type"].(string); ok {
			prompt.WriteString(fmt.Sprintf("**Error Type**: %s\n\n", errorType))
		}

		// Show the compilation error
		if compileError, ok := task.Input["compile_error"].(string); ok {
			prompt.WriteString("**Compilation Error**:\n```\n")
			prompt.WriteString(compileError)
			prompt.WriteString("\n```\n\n")

			// ✅ FIX 3: Include error classifier guidance
			analysis := feedback.AnalyzeCompileError(compileError)
			prompt.WriteString(analysis.FormatFixPrompt(compileError))
			prompt.WriteString("\n")
		}

		// ✅ FIX 2: Auto-load and inject file content
		if targetFile, ok := task.Input["target_file"].(string); ok {
			prompt.WriteString(fmt.Sprintf("**Target File**: %s\n\n", targetFile))

			if content, err := os.ReadFile(targetFile); err == nil {
				prompt.WriteString("**Current Code**:\n```go\n")
				prompt.WriteString(string(content))
				prompt.WriteString("\n```\n\n")

				prompt.WriteString("**Fix Strategy: Use edit_line for ALL fixes**\n")
				prompt.WriteString("1. Extract line number from error (format: './file.go:LINE:COL: message')\n")
				prompt.WriteString("2. Call edit_line with that line number and corrected content\n")
				prompt.WriteString("3. For multiple errors, call edit_line multiple times\n")
				prompt.WriteString("4. Example: {\"tool\": \"edit_line\", \"arguments\": {\"path\": \"file.go\", \"line\": 42, \"new_content\": \"fixed line\"}}\n\n")
				prompt.WriteString("⚠️  CRITICAL: generate_code is NOT available in fix mode - you can only use edit_line\n\n")

				// ✅ For test file fixes with undefined functions, inject source code
				if strings.HasSuffix(targetFile, "_test.go") {
					if depFiles, ok := task.Input["dependency_files"].([]string); ok && len(depFiles) > 0 {
						prompt.WriteString("**Source files that tests reference:**\n")
						for _, file := range depFiles {
							if !strings.HasSuffix(file, "_test.go") {
								if content, err := os.ReadFile(file); err == nil {
									prompt.WriteString(fmt.Sprintf("\n**File: %s**\n```go\n%s\n```\n", file, string(content)))
								}
							}
						}
						prompt.WriteString("\n→ Use the ACTUAL function names from the source files above\n")
						prompt.WriteString("→ Fix test code to call the correct functions\n\n")
					}
				}
			} else {
				prompt.WriteString(fmt.Sprintf("⚠️  Warning: Could not read file: %v\n\n", err))
				prompt.WriteString("You will need to use `read_file` tool first to see the current code.\n\n")
			}
		}

		return prompt.String()
	}

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
				// NOTE: Source code will be injected directly after this prompt (line 156-172 in Execute)
				// So we don't need to tell LLM to read files - it will already have the content
				prompt.WriteString("⚠️  IMPORTANT: This is a TEST FILE task.\n")
				prompt.WriteString("The source code to test will be provided below.\n\n")
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
