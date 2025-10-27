package agents

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"wilson/agent"
	"wilson/agent/base"

	contextpkg "wilson/context"
	"wilson/core/registry"
	"wilson/llm"
)

// TestAgent specializes in test design, validation, and quality assurance
type TestAgent struct {
	*base.BaseAgent
	llmManager *llm.Manager
}

// NewTestAgent creates a new test agent
func NewTestAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *TestAgent {
	// Use orchestration model for tool calling (hermes3:8b)
	// Specialized for reliable JSON generation and consistent tool selection
	base := base.NewBaseAgent("Test", llm.PurposeOrchestration, llmManager, contextMgr)

	// Test-specific tools
	base.SetAllowedTools([]string{
		// File reading
		"read_file",
		"search_files",
		"list_files",
		// File writing (for creating test files)
		"write_file",     // Create new test files
		"modify_file",    // Update existing tests
		"append_to_file", // Add new test cases
		// Git tools (see what code changed)
		"git_status", // Detect test files
		"git_diff",   // See what code changed
		"git_log",    // Understand test history
		// Test execution
		"run_tests", // Execute go test
		// Context and artifacts
		"search_artifacts",
		"retrieve_context",
		"store_artifact",
		"leave_note",
	})
	base.SetCanDelegate(false)

	return &TestAgent{
		BaseAgent:  base,
		llmManager: llmManager,
	}
}

// CanHandle checks if the test agent can handle a task
func (a *TestAgent) CanHandle(task *agent.Task) bool {
	return task.Type == "test"
}

// ExecuteWithContext executes a task with full TaskContext
func (a *TestAgent) ExecuteWithContext(ctx context.Context, taskCtx *base.TaskContext) (*agent.Result, error) {
	a.SetTaskContext(taskCtx)
	task := a.ConvertTaskContextToTask(taskCtx)
	return a.Execute(ctx, task)
}

// Execute executes a testing task
func (a *TestAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	result := &agent.Result{
		TaskID: task.ID,
		Agent:  a.Name(),
	}

	// ✅ CONTEXT-AWARE PRECONDITION CHECK
	if err := a.checkPreconditions(ctx, task); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Precondition failed: %v", err)

		// Record error in TaskContext for learning
		a.RecordError("precondition_failed", "precondition", err.Error(), "", 0,
			"Ensure prerequisites are met before running tests")

		return result, err
	}

	// Get current context for code to test
	currentCtx, err := a.GetContext()
	if err != nil {
		currentCtx = nil
	}

	// Build test-specific prompts
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Call LLM
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// Execute tools via AgentToolExecutor (same pattern as CodeAgent)
	executor := base.NewAgentToolExecutor(
		registry.NewExecutor(),
		a.llmManager,
	)

	execResult, err := executor.ExecuteAgentResponse(
		ctx,
		response,
		systemPrompt,
		userPrompt,
		a.Purpose(),
		task.ID,
		nil, // TaskContext not available in old Execute method
	)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Execution failed: %v", err)
		result.Output = execResult.Output
		return result, err
	}

	// Store response as artifact
	artifact, artifactErr := a.StoreArtifact(
		"test_execution",
		execResult.Output,
		"test_agent",
	)
	if artifactErr == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	result.Success = true
	result.Output = execResult.Output
	result.Metadata = map[string]interface{}{
		"model":      "chat",
		"agent_type": "test",
		"tools_used": execResult.ToolsExecuted,
	}

	return result, nil
}

func (a *TestAgent) buildSystemPrompt() string {
	// Start with shared core principles
	prompt := BuildSharedPrompt("Test Agent")

	// Add Test Agent specific instructions
	prompt += `
You execute and create tests. You work with test files and test runners.

=== AVAILABLE TOOLS ===

**run_tests** - Execute test suite
{"tool": "run_tests", "arguments": {"path": "/project/path"}}

**write_file** - Create new test file
{"tool": "write_file", "arguments": {"path": "feature_test.go", "content": "package..."}}

**read_file** - Read existing tests or code
{"tool": "read_file", "arguments": {"path": "main.go"}}

**modify_file** - Update existing tests
{"tool": "modify_file", "arguments": {"path": "test.go", "old_content": "...", "new_content": "..."}}

=== COMMON PATTERNS ===

Task: "Run tests" or "Execute test suite"
→ Check context for project path
→ Call run_tests with that path
→ Report results

Task: "Write tests for X"
→ Read code if not in context
→ Design test cases (happy path, edge cases, errors)
→ Use write_file to create test file
→ Include package declaration, imports, test functions

Task: "Fix failing test Y"
→ Read test file
→ Analyze failure
→ Use modify_file to fix

=== QUALITY STANDARDS ===

When creating tests:
- Cover normal cases, edge cases, error handling
- Make tests independent and deterministic
- Use clear test names (TestFunctionName_Scenario_ExpectedBehavior)
- Include meaningful assertions
- Add comments for complex test logic

=== EXECUTION ===

Read task description → Identify needed tools → Call them → Done.
No planning discussions. Just execute.
`

	return prompt
}

func (a *TestAgent) buildUserPrompt(task *agent.Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description))

	// Add git context if available
	if taskCtx := a.GetCurrentContext(); taskCtx != nil && len(taskCtx.GitModifiedFiles) > 0 {
		prompt.WriteString("📝 **Git Context - Modified Files**:\n")
		prompt.WriteString("Focus testing on these recently changed files:\n")
		for _, file := range taskCtx.GitModifiedFiles {
			if !strings.HasSuffix(file, "_test.go") {
				prompt.WriteString(fmt.Sprintf("  - %s\n", file))
			}
		}
		prompt.WriteString("\n")
	}

	// Add context - code artifacts
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("Available context:\n")
		for i, artifact := range currentCtx.Artifacts {
			if i >= 5 { // Limit context
				break
			}
			if artifact.Type == "code" {
				content := artifact.Content
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				prompt.WriteString(fmt.Sprintf("- %s artifact:\n```\n%s\n```\n\n", artifact.Type, content))
			}
		}
	}

	// Add task input if provided
	if len(task.Input) > 0 {
		prompt.WriteString("Additional parameters:\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
	}

	return prompt.String()
}

// checkPreconditions validates prerequisites with TaskContext awareness
func (a *TestAgent) checkPreconditions(ctx context.Context, task *agent.Task) error {
	lowerDesc := strings.ToLower(task.Description)

	// Check: "run tests" requires test files to exist
	if strings.Contains(lowerDesc, "run test") || strings.Contains(lowerDesc, "execute test") {
		projectPath := "."

		// Get project path from task input
		if pathVal, ok := task.Input["project_path"]; ok {
			if pathStr, ok := pathVal.(string); ok {
				projectPath = pathStr
			}
		}

		// Check if dependency_files contains test files
		if depFiles, ok := task.Input["dependency_files"].([]string); ok && len(depFiles) > 0 {
			hasTestFiles := false
			for _, file := range depFiles {
				if strings.Contains(file, "_test.go") || strings.Contains(file, "_test.") {
					hasTestFiles = true
					break
				}
			}

			if hasTestFiles {
				// Dependencies created test files - we're good!
				return nil
			}
		}

		// ✅ CRITICAL FIX: Test Agent should NEVER create new tasks
		// If test files don't exist, that's a FATAL ERROR - the task decomposition is wrong
		// Test Agent is the LAST step in a workflow, not the orchestrator

		// Check filesystem for test files
		testFiles, _ := filepath.Glob(filepath.Join(projectPath, "*_test.go"))
		if len(testFiles) > 0 {
			// Test files exist! Proceed with execution
			fmt.Printf("[TestAgent] Found %d existing test files, proceeding with execution\n", len(testFiles))
			return nil
		}

		// Test files missing - this is a fatal error
		return fmt.Errorf("no test files found in %s - test creation should happen BEFORE test execution", projectPath)
	}

	return nil
}
