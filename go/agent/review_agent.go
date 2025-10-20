package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/llm"
)

// ReviewAgent specializes in code review, quality assessment, and approval workflow
type ReviewAgent struct {
	*BaseAgent
}

// NewReviewAgent creates a new review agent
func NewReviewAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *ReviewAgent {
	base := NewBaseAgent("Review", llm.PurposeAnalysis, llmManager, contextMgr) // Use analysis model

	// Review-specific tools (ENDGAME Phase 3)
	base.SetAllowedTools([]string{
		// File operations
		"read_file",
		"search_files",
		"list_files",
		// Context operations
		"search_artifacts",
		"retrieve_context",
		"store_artifact",
		"leave_note",
		// Review workflow tools (Phase 3)
		"get_review_status",
		"submit_review",
		// Quality gate tools (for automated checks)
		"compile",
		"format_code",
		"lint_code",
		"security_scan",
		"complexity_check",
		"coverage_check",
		"code_review",
		// Autonomous coordination (ENDGAME Phase 4)
		"poll_tasks",
		"claim_task",
		"update_task_progress",
		"unblock_tasks",
		"get_task_queue",
	})
	base.SetCanDelegate(false)

	return &ReviewAgent{
		BaseAgent: base,
	}
}

// CanHandle checks if the review agent can handle a task
func (a *ReviewAgent) CanHandle(task *Task) bool {
	return task.Type == "review"
}

// Execute executes a review task
func (a *ReviewAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
	}

	// Get current context for artifacts to review
	currentCtx, err := a.GetContext()
	if err != nil {
		currentCtx = nil
	}

	// Build review-specific prompts
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Call LLM
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// Store response as review artifact
	artifact, err := a.StoreArtifact(
		"review",
		response,
		"review_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	// Leave note for Manager Agent
	noteText := fmt.Sprintf("Completed review: %s. Review report stored as artifact #%d.",
		task.Description, artifact.ID)
	_ = a.LeaveNote("Manager", noteText)

	result.Success = true
	result.Output = response
	result.Metadata = map[string]interface{}{
		"model":       "analysis",
		"agent_type":  "review",
		"artifact_id": artifact.ID,
	}

	return result, nil
}

func (a *ReviewAgent) buildSystemPrompt() string {
	return `You are Wilson's Review Agent, a specialist in code review, quality assessment, and approval workflows with AUTOMATED QUALITY GATES.

Your specialized capabilities:
- Code review and quality assessment
- Automated quality gate execution
- Architecture review
- Security vulnerability identification
- Performance analysis
- Best practices validation
- Documentation review
- Test coverage evaluation
- Approval workflow management

**CRITICAL: Automated Review Workflow (ENDGAME Phase 3)**

When reviewing a task, ALWAYS follow this workflow:

**Step 1: Get Review Context**
Use get_review_status with the task_key or review_id to understand:
- What needs to be reviewed
- Review type (quality, security, performance)
- Previous review attempts (if any)

**Step 2: Run Automated Quality Gates**
Execute these tools ON THE CODE PATH to check quality:
1. **compile** - Check if code compiles (CRITICAL - fail if doesn't compile)
2. **format_code** - Check code formatting (INFO - can auto-fix)
3. **lint_code** - Check style and best practices (WARNING)
4. **security_scan** - Find vulnerabilities (CRITICAL if high/critical found)
5. **complexity_check** - Check code complexity (WARNING if >15)
6. **coverage_check** - Verify test coverage (WARNING if <80%)

**Step 3: Analyze Automated Results**
- If CRITICAL issues found (compilation fails, security vulnerabilities):
  → Immediately REJECT or REQUEST_CHANGES
  → Don't waste time on manual review
- If only WARNINGS:
  → Continue with manual review
  → Include automated findings in review
- If all pass:
  → Do quick manual review
  → Likely APPROVE

**Step 4: Manual Review (if automated checks pass)**
Examine:
- Correctness and logic
- Design and architecture
- Requirements fulfillment
- Edge cases and error handling
- Documentation quality

**Step 5: Submit Review Decision**
Use submit_review with:
- review_id: From get_review_status
- status: "approved" | "needs_changes" | "rejected"
- findings: Array of issues from automated + manual review
- comments: Clear summary of review
- required_changes: List of specific changes needed (if needs_changes)

Your review methodology:
1. **Understanding**: Get review context, comprehend purpose
2. **Automated Checks**: Run quality gates, analyze results
3. **Manual Analysis**: Systematic examination (if automated pass)
4. **Evaluation**: Assess against quality criteria
5. **Findings**: Categorize issues by severity (combine automated + manual)
6. **Decision**: Submit review with clear reasoning

Review dimensions:
- **Correctness**: Does it work as intended? Are there bugs?
- **Quality**: Is the code clean, readable, and maintainable?
- **Design**: Is the architecture sound? Any design issues?
- **Performance**: Are there performance concerns?
- **Security**: Any security vulnerabilities or risks?
- **Testing**: Adequate test coverage and quality?
- **Documentation**: Clear and sufficient documentation?
- **Best Practices**: Follows language/framework conventions?

Finding severity levels:
- **Critical**: Must fix - blocks approval (security, major bugs, data loss)
- **Major**: Should fix - significant impact (performance, maintainability)
- **Minor**: Nice to fix - small improvements (style, naming, minor optimization)
- **Info**: FYI - suggestions and observations (alternative approaches)

Output format:
- **Summary**: Brief overview of the review
- **Decision**: APPROVED / REQUEST_CHANGES / REJECTED
- **Findings**: Categorized list of issues with severity
- **Strengths**: Positive aspects worth highlighting
- **Recommendations**: Prioritized action items
- **Risk Assessment**: Potential risks if deployed as-is

Review principles:
- Be objective and constructive
- Focus on significant issues, not nitpicks (unless asked for detailed review)
- Provide clear rationale for each finding
- Suggest solutions, not just problems
- Consider the context and constraints
- Balance perfection with pragmatism

You are the quality gatekeeper. Ensure high standards while being pragmatic.`
}

func (a *ReviewAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString("## Review Task\n\n")
	prompt.WriteString(fmt.Sprintf("**Objective:** %s\n\n", task.Description))

	// Add context - review all relevant artifacts
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("## Artifacts to Review\n\n")
		hasArtifacts := false
		for i, artifact := range currentCtx.Artifacts {
			if i >= 10 { // Review last 10 artifacts
				break
			}
			// Show code, tests, and other relevant artifacts
			if artifact.Type == "code" || artifact.Type == "test" || artifact.Type == "analysis" {
				hasArtifacts = true
				content := artifact.Content
				if len(content) > 600 {
					content = content[:600] + "\n... (truncated) ..."
				}
				prompt.WriteString(fmt.Sprintf("### Artifact #%d [%s]\n```\n%s\n```\n\n", artifact.ID, artifact.Type, content))
			}
		}
		if !hasArtifacts {
			prompt.WriteString("*No artifacts found for review. Will provide general review guidelines.*\n\n")
		}
	}

	// Add review criteria
	if len(task.Input) > 0 {
		prompt.WriteString("## Review Criteria\n\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("## Required Deliverables\n\n")
	prompt.WriteString("Provide a comprehensive review report with:\n\n")
	prompt.WriteString("1. **Executive Summary** (2-3 sentences)\n")
	prompt.WriteString("2. **Review Decision**: APPROVED / REQUEST_CHANGES / REJECTED\n")
	prompt.WriteString("3. **Findings** (grouped by severity):\n")
	prompt.WriteString("   - Critical issues (if any)\n")
	prompt.WriteString("   - Major issues (if any)\n")
	prompt.WriteString("   - Minor issues (if any)\n")
	prompt.WriteString("   - Informational notes (if any)\n")
	prompt.WriteString("4. **Strengths**: What was done well\n")
	prompt.WriteString("5. **Recommendations**: Prioritized action items\n")
	prompt.WriteString("6. **Risk Assessment**: Deployment risks (if applicable)\n")
	prompt.WriteString("\n")
	prompt.WriteString("Be thorough but concise. Focus on actionable feedback.\n")

	return prompt.String()
}
