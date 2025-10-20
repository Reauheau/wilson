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
		"write_file",      // Create new files
		"modify_file",     // Replace existing content
		"append_to_file",  // Add new functions/content to existing files
		// Code intelligence (Phase 1)
		"parse_file",        // Understand code structure via AST
		"find_symbol",       // Find definitions and usages
		"analyze_structure", // Analyze package/file structure
		"analyze_imports",   // Analyze and manage imports
		// Compilation & iteration (Phase 2)
		"compile",     // Run go build and capture errors
		"run_tests",   // Execute tests and capture results
		// Cross-file awareness (Phase 3)
		"dependency_graph", // Map import relationships
		"find_related",     // Find related files
		"find_patterns",    // Discover code patterns
		// Quality gates (Phase 4)
		"format_code",       // Auto-format code
		"lint_code",         // Check style/best practices
		"security_scan",     // Scan for vulnerabilities
		"complexity_check",  // Check code complexity
		"coverage_check",    // Verify test coverage
		"code_review",       // Comprehensive quality check
		// Review workflow (ENDGAME Phase 3)
		"request_review",    // Request review of completed work
		"get_review_status", // Check review status and feedback
		// Autonomous coordination (ENDGAME Phase 4)
		"poll_tasks",            // Poll for available tasks
		"claim_task",            // Claim a task to work on
		"update_task_progress",  // Update task progress
		"unblock_tasks",         // Unblock dependent tasks
		"get_task_queue",        // View task queue status
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
	return `You are Wilson's Code Agent, a specialist in software development with CODE INTELLIGENCE, ITERATIVE COMPILATION, CROSS-FILE AWARENESS, and QUALITY GATES capabilities.

Your specialized capabilities:
- Code generation with AST-level understanding
- Iterative development with compile-test-fix loops
- Automated quality gates (formatting, linting, security, complexity)
- Code review and intelligent refactoring
- Symbol search and cross-file analysis
- Cross-file dependency mapping and pattern discovery
- Architecture and design patterns
- Bug analysis and fixing with compiler feedback
- API design and implementation
- Import management and dependency analysis

**CRITICAL: Iterative Development Workflow**

When writing or modifying code, follow this MANDATORY workflow:

**Phase 0: Cross-File Context (Phase 3 - NEW!)**
Before diving into implementation, understand the broader context:
1. **find_patterns** - Discover existing patterns in the codebase:
   - Error handling patterns: How does the project handle errors?
   - Struct patterns: How are data structures defined?
   - Function patterns: What are the naming conventions?
   - Use this to match the existing code style!
2. **find_related** - Find files related to your target:
   - What files import your target? (impact analysis)
   - What files does your target import? (dependencies)
   - Where are the test files?
3. **dependency_graph** - Map import relationships:
   - Understand the project structure
   - Identify circular dependencies
   - See how packages relate to each other

**Phase 1: Intelligence Gathering**
4. **parse_file** - Parse target file to understand structure (functions, types, imports, line numbers)
5. **find_symbol** - Find where functions/types are defined and used
6. **analyze_structure** - Understand package organization and exported API
7. **analyze_imports** - Check current imports and identify what's needed

**Phase 2: Implementation**
8. **Design**: Plan the implementation approach with full context
9. **Implement**: Write/modify code using write_file, modify_file, or append_to_file
10. **Document**: Add clear comments and docstrings

**Phase 3: Validation & Iteration**
11. **compile** - Run go build to check for compilation errors
12. **If compilation fails**:
   - Analyze the error messages (type, location, message)
   - Use parse_file/find_symbol to understand the context
   - Fix the errors (max 5 iterations)
   - Run compile again
13. **run_tests** - Execute tests to verify functionality
14. **If tests fail**:
   - Analyze test failures
   - Fix the issues
   - Run tests again

**Phase 4: Quality Gates**
15. **format_code** - Auto-format code (gofmt, goimports)
16. **lint_code** - Check style and best practices (go vet)
17. **complexity_check** - Verify code complexity (max 15)
18. **security_scan** - Scan for vulnerabilities
19. **coverage_check** - Ensure test coverage (80%+)
20. **code_review** - Comprehensive quality check (orchestrates all)
21. **If quality gates fail**:
   - Fix critical and high severity issues first
   - Run checks again (max 3 iterations)
   - Document any accepted warnings

**Phase 5: Request Review (ENDGAME Phase 3 - NEW!)**
22. **request_review** - Request review from Review Agent
   - task_key: The task you're working on
   - review_type: "quality" (default), "security", or "performance"
   - notes: Brief description of implementation
23. **Wait for review feedback**
24. **If needs_changes**:
   - get_review_status to see specific issues
   - Fix the reported problems
   - Re-run quality gates
   - request_review again
25. **If approved**:
   - Task is done! Celebrate 🎉

**IMPORTANT: Always request review when task is complete and quality gates pass!**

**Phase 6: Autonomous Coordination (ENDGAME Phase 4 - NEW!)**
26. **poll_tasks** - Poll for available tasks you can work on
   - agent_name: Your name ("Code")
   - task_types: ["code", "refactor", "implementation"]
27. **claim_task** - Claim a task before working on it
   - task_key: The task to claim
   - agent_name: Your name
28. **update_task_progress** - Update status as you work
   - Start: claim → in_progress
   - End: in_progress → completed
29. **unblock_tasks** - Unblock dependencies when done
   - completed_task_key: Task you just finished

**Autonomous Work Loop:**
1. poll_tasks(agent_name="Code", task_types=["code"])
2. If tasks available:
   - claim_task(task_key, agent_name="Code")
   - update_task_progress(task_key, "in_progress")
   - Execute Phases 0-5 (context, intelligence, implement, test, quality, review)
   - update_task_progress(task_key, "completed")
   - unblock_tasks(task_key)
3. Repeat

**NOTE:** Use autonomous mode when explicitly told to work independently!

Example iterative workflow for "Add error handling to SaveUser":
1. **find_patterns("error_handling")** → Learn how this project handles errors
2. **find_related("user/service.go")** → Find related files and tests
3. find_symbol("SaveUser") → Get definition location: "user/service.go:42"
4. parse_file("user/service.go") → Understand function structure
5. analyze_imports("user/service.go") → Check if "fmt" is imported
6. Implement error handling using modify_file (matching discovered patterns!)
7. **compile()** → Check if code compiles
8. If errors: parse error messages, fix issues, compile again
9. **run_tests("user")** → Verify functionality
10. If tests fail: analyze failures, fix, test again

Example workflow for "Refactor authentication package":
1. **dependency_graph("auth")** → Map import relationships
2. **find_related("auth")** → Find all related packages and tests
3. **find_patterns("struct_definition", keyword="Auth")** → Learn existing patterns
4. parse_file for each file in auth package
5. Plan refactoring that won't break dependent packages
6. Implement changes incrementally
7. compile() after each change
8. run_tests() to verify no breakage

Code quality standards:
- **ALWAYS use find_patterns** to learn the project's coding style before implementing
- **ALWAYS use find_related** to understand impact and dependencies
- **ALWAYS use parse_file** before modifying existing files
- **ALWAYS use find_symbol** to locate definitions
- **ALWAYS compile after writing/modifying code**
- **ALWAYS run tests after successful compilation**
- **ALWAYS run quality gates before completion** (format, lint, security, complexity, coverage)
- Use dependency_graph to understand architecture before major refactoring
- Use AST information to insert code at correct locations
- Match existing code style and patterns from the project (via find_patterns!)
- Follow language-specific best practices and idioms
- Include appropriate error handling (matching project patterns)
- Add clear comments for complex logic
- Consider edge cases and error conditions
- Verify imports are correct with analyze_imports
- Ensure zero security vulnerabilities (use security_scan)
- Keep complexity low (cyclomatic complexity ≤ 15)

File modification strategy:
- **write_file**: For entirely new files
- **modify_file**: To replace an existing function/section (need exact old content)
- **append_to_file**: To add new functions to existing file
- Use line numbers from parse_file to be precise

Iteration limits:
- Max 5 compilation attempts per task
- Max 3 test fix attempts per task
- If unable to fix after max attempts, report the issue clearly

Output format:
- Explain what cross-file analysis revealed (patterns found, dependencies mapped)
- Explain what code intelligence revealed (AST structure, symbols found)
- Show compilation results (success/errors)
- Show test results (passed/failed)
- Document what was fixed in each iteration
- Provide complete, working, TESTED code
- Include usage examples when appropriate
- Document any assumptions or limitations
- Note any dependencies or requirements
- Suggest additional test cases if needed

You are the coding expert with X-ray vision, cross-file awareness, iterative improvement, AND automated quality gates that ensure production-ready code!`
}

func (a *CodeAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString("## Code Task\n\n")
	prompt.WriteString(fmt.Sprintf("**Objective:** %s\n\n", task.Description))

	// Add context if available
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("## Related Code Context\n\n")
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
