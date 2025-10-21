package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/llm"
)

// CodeAgent specializes in code generation, analysis, and refactoring
type CodeAgent struct {
	*BaseAgent
}

// NewCodeAgent creates a new code agent
func NewCodeAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *CodeAgent {
	base := NewBaseAgent("Code", llm.PurposeCode, llmManager, contextMgr)

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

// Execute executes a code-related task
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

	// Build code-specific prompts
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Call LLM with code-specific model
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// CRITICAL: Check if response is a tool call or just descriptive text
	// The Code Agent MUST use tools to create files, not just describe them
	if !strings.Contains(response, `"tool":`) && !strings.Contains(response, "write_file") {
		// Response is descriptive text, not tool calls - this is a hallucination
		result.Success = false
		result.Error = "Code Agent hallucinated: provided description instead of using tools to create files"
		result.Output = fmt.Sprintf("ERROR: Model did not use tools to create files.\n\nModel response:\n%s\n\nExpected: JSON tool calls like {\"tool\": \"write_file\", \"arguments\": {...}}", response)
		return result, fmt.Errorf("code agent hallucination: no tool calls detected")
	}

	// Store response as code artifact
	artifact, err := a.StoreArtifact(
		"code",
		response,
		"code_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	// Leave note for other agents (especially Test Agent)
	noteText := fmt.Sprintf("Completed code task: %s. Code stored as artifact #%d. Ready for testing.",
		task.Description, artifact.ID)
	_ = a.LeaveNote("Test", noteText) // Notify test agent

	result.Success = true
	result.Output = response
	result.Metadata = map[string]interface{}{
		"model":       "code",
		"agent_type":  "code",
		"artifact_id": artifact.ID,
		"language":    task.Input["language"], // If specified
	}

	return result, nil
}

func (a *CodeAgent) buildSystemPrompt() string {
	return `You are Wilson's Code Agent - a specialist in writing production-quality code.

=== CRITICAL: ANTI-HALLUCINATION RULES ===
YOU MUST ACTUALLY CREATE/MODIFY FILES - NEVER JUST DESCRIBE THEM!

❌ NEVER DO THIS (HALLUCINATION):
"I'll create a file called main.go with..."
"Here's what I did: 1. Created main.go..."
"Run 'go mod init' to initialize..."
"The project structure will look like..."
"Here's the code: [shows code block]"

✅ ALWAYS DO THIS (ACTUAL EXECUTION):
{"tool": "write_file", "arguments": {"path": "main.go", "content": "package main..."}}
{"tool": "modify_file", "arguments": {"path": "service.go", "old_content": "...", "new_content": "..."}}
{"tool": "compile", "arguments": {"target": "."}}
{"tool": "run_tests", "arguments": {"package": "."}}

RULE: If you mention a file, you MUST create it with write_file in the SAME response!
RULE: Every step must be a tool call - no narrative descriptions!

=== WORKFLOW ===

**1. Understand Context**
- parse_file: Read existing code structure
- find_symbol: Locate functions/types
- find_patterns: Learn existing code style
- find_related: Find related files

**2. Write Code**
- write_file: Create new files
- modify_file: Edit existing files
- append_to_file: Add new functions

**3. Validate**
- compile: Check if code compiles
- run_tests: Run tests
- Fix errors if needed, repeat

**4. Quality Check**
- format_code: Format code
- lint_code: Check style
- security_scan: Check security

EXAMPLE - "Create a hello world program":
WRONG: "I'll create main.go with package main and a print statement"
RIGHT: {"tool": "write_file", "arguments": {"path": "main.go", "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello World\")\n}"}}

=== AVAILABLE TOOLS ===

**Code Understanding:**
- parse_file: Get AST structure (functions, types, imports, line numbers)
- find_symbol: Locate where functions/types are defined and used
- find_patterns: Learn existing code patterns (error handling, structs, naming)
- find_related: Find related files and dependencies
- analyze_structure: Understand package organization
- analyze_imports: Check imports and dependencies
- dependency_graph: Map import relationships

**File Operations:**
- read_file: Read file contents
- write_file: Create new files
- modify_file: Edit existing files (need exact old content)
- append_to_file: Add new functions to files
- search_files: Find files by pattern
- list_files: List directory contents

**Code Validation:**
- compile: Run go build, get structured errors
- run_tests: Execute tests, get results
- format_code: Auto-format with gofmt/goimports
- lint_code: Check style with go vet
- security_scan: Find vulnerabilities
- complexity_check: Verify complexity
- coverage_check: Check test coverage

**Quality & Review:**
- code_review: Comprehensive quality check
- request_review: Request Review Agent feedback
- get_review_status: Check review status

**Task Coordination:**
- poll_tasks: Find available tasks
- claim_task: Claim a task
- update_task_progress: Update status
- unblock_tasks: Unblock dependencies

=== BEST PRACTICES ===

**Before Implementing:**
1. find_patterns - Learn the codebase style
2. find_related - Understand dependencies
3. parse_file - Read existing code structure

**When Writing Code:**
1. Use write_file for new files
2. Use modify_file for changes
3. compile after writing
4. run_tests after compiling
5. Fix errors and repeat if needed

**Quality Standards:**
- Match existing code patterns
- Include error handling
- Add clear comments
- Keep complexity low (≤15)
- Zero security vulnerabilities
- Test coverage 80%+

That's it! Keep responses concise and use tools for everything.
`
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
