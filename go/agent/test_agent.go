package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/core/registry"
	"wilson/llm"
)

// TestAgent specializes in test design, validation, and quality assurance
type TestAgent struct {
	*BaseAgent
}

// NewTestAgent creates a new test agent
func NewTestAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *TestAgent {
	// Use Code LLM - better at structured JSON output for tool calls
	// Chat LLM struggles with JSON generation and often produces invalid JSON
	base := NewBaseAgent("Test", llm.PurposeCode, llmManager, contextMgr)

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
		BaseAgent: base,
	}
}

// CanHandle checks if the test agent can handle a task
func (a *TestAgent) CanHandle(task *Task) bool {
	return task.Type == "test"
}

// ExecuteWithContext executes a task with full TaskContext
func (a *TestAgent) ExecuteWithContext(ctx context.Context, taskCtx *TaskContext) (*Result, error) {
	a.SetTaskContext(taskCtx)
	task := a.ConvertTaskContextToTask(taskCtx)
	return a.Execute(ctx, task)
}

// Execute executes a testing task
func (a *TestAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
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
	executor := NewAgentToolExecutor(
		registry.NewExecutor(),
		a.llmManager,
	)

	execResult, err := executor.ExecuteAgentResponse(
		ctx,
		response,
		systemPrompt,
		userPrompt,
		a.purpose,
		task.ID,
		a.currentContext, // Pass TaskContext (Phase 2)
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

func (a *TestAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description))

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
func (a *TestAgent) checkPreconditions(ctx context.Context, task *Task) error {
	lowerDesc := strings.ToLower(task.Description)

	// Check: "run tests" requires test files to exist
	if strings.Contains(lowerDesc, "run test") || strings.Contains(lowerDesc, "execute test") {
		projectPath := "."

		// ✅ Use TaskContext if available (better than parsing task.Input)
		if a.currentContext != nil && a.currentContext.ProjectPath != "" {
			projectPath = a.currentContext.ProjectPath
		} else if pathVal, ok := task.Input["project_path"]; ok {
			if pathStr, ok := pathVal.(string); ok {
				projectPath = pathStr
			}
		}

		// ✅ SMART: Check DependencyFiles first (we know what was created!)
		if a.currentContext != nil && len(a.currentContext.DependencyFiles) > 0 {
			hasTestFiles := false
			for _, file := range a.currentContext.DependencyFiles {
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

		// Fallback: Check filesystem
		// Use simple check instead of filepath.Glob to avoid import
		// The actual test execution tool will handle the detailed check

		// ✅ SMART: Include context in dependency request
		reason := fmt.Sprintf("No test files found in %s", projectPath)

		// Check if we already tried this before (from error history)
		if a.currentContext != nil {
			if lastErr := a.currentContext.GetLastError(); lastErr != nil {
				if lastErr.ErrorType == "missing_test_files" {
					reason += " (previous attempt also failed - check if code files exist)"
				}
			}
		}

		return a.RequestDependency(ctx,
			fmt.Sprintf("Create test files in %s", projectPath),
			ManagedTaskTypeCode,
			reason,
		)
	}

	return nil
}
