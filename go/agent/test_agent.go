package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/llm"
)

// TestAgent specializes in test design, validation, and quality assurance
type TestAgent struct {
	*BaseAgent
}

// NewTestAgent creates a new test agent
func NewTestAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *TestAgent {
	base := NewBaseAgent("Test", llm.PurposeChat, llmManager, contextMgr) // Use chat model for now

	// Test-specific tools
	base.SetAllowedTools([]string{
		// File reading
		"read_file",
		"search_files",
		"list_files",
		// File writing (for creating test files)
		"write_file",      // Create new test files
		"modify_file",     // Update existing tests
		"append_to_file",  // Add new test cases
		// Context and artifacts
		"search_artifacts",
		"retrieve_context",
		"store_artifact",
		"leave_note",
		// Future: run_tests, coverage_report, lint, etc.
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

// Execute executes a testing task
func (a *TestAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
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

	// Store response as test artifact
	artifact, err := a.StoreArtifact(
		"test",
		response,
		"test_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	// Leave note for Review Agent
	noteText := fmt.Sprintf("Completed test design: %s. Tests stored as artifact #%d. Ready for review.",
		task.Description, artifact.ID)
	_ = a.LeaveNote("Review", noteText)

	result.Success = true
	result.Output = response
	result.Metadata = map[string]interface{}{
		"model":       "chat",
		"agent_type":  "test",
		"artifact_id": artifact.ID,
	}

	return result, nil
}

func (a *TestAgent) buildSystemPrompt() string {
	return `You are Wilson's Test Agent, a specialist in quality assurance, test design, and validation.

Your specialized capabilities:
- Test case design (unit, integration, end-to-end)
- Test data generation
- Edge case identification
- Test coverage analysis
- Bug reproduction scenarios
- Quality metrics assessment

Your testing methodology:
1. **Code Understanding**: Analyze what needs to be tested
2. **Test Strategy**: Determine appropriate test types and coverage
3. **Test Design**: Create comprehensive test cases
4. **Edge Cases**: Identify boundary conditions and error scenarios
5. **Test Data**: Generate appropriate test data
6. **Documentation**: Write clear test descriptions and expected outcomes

Testing principles:
- Test normal cases, edge cases, and error conditions
- Aim for comprehensive coverage without redundancy
- Write clear, maintainable tests
- Include both positive and negative test cases
- Consider performance and security aspects
- Make tests independent and repeatable
- Document test assumptions and prerequisites

Test quality standards:
- Each test should verify one specific behavior
- Tests should be deterministic (same input = same output)
- Include clear assertions and expected results
- Provide helpful failure messages
- Consider test execution time
- Think about test maintainability

Output format:
- Provide complete test suite or test cases
- Include test data and fixtures
- Document test dependencies
- Specify expected outcomes
- Note any testing limitations or assumptions
- Suggest integration with CI/CD if applicable

You are the quality expert in the team. Ensure thorough validation.`
}

func (a *TestAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString("## Testing Task\n\n")
	prompt.WriteString(fmt.Sprintf("**Objective:** %s\n\n", task.Description))

	// Add context - especially code artifacts
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("## Code to Test\n\n")
		hasCode := false
		for i, artifact := range currentCtx.Artifacts {
			if i >= 10 { // Check more artifacts for code
				break
			}
			// Prioritize code artifacts
			if artifact.Type == "code" {
				hasCode = true
				content := artifact.Content
				if len(content) > 500 {
					content = content[:500] + "\n... (truncated) ..."
				}
				prompt.WriteString(fmt.Sprintf("**Artifact #%d** - Code to test:\n```\n%s\n```\n\n", artifact.ID, content))
			}
		}
		if !hasCode {
			prompt.WriteString("*No code artifacts found in context. Will design tests based on task description.*\n\n")
		}
	}

	// Add task specifications
	if len(task.Input) > 0 {
		prompt.WriteString("## Test Requirements\n\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("## Deliverables\n\n")
	prompt.WriteString("Provide:\n")
	prompt.WriteString("1. Comprehensive test cases covering:\n")
	prompt.WriteString("   - Normal/happy path scenarios\n")
	prompt.WriteString("   - Edge cases and boundary conditions\n")
	prompt.WriteString("   - Error conditions and exception handling\n")
	prompt.WriteString("   - Performance considerations (if applicable)\n")
	prompt.WriteString("2. Test data and fixtures needed\n")
	prompt.WriteString("3. Expected outcomes for each test\n")
	prompt.WriteString("4. Test coverage assessment\n")
	prompt.WriteString("5. Any testing limitations or assumptions\n")

	return prompt.String()
}
