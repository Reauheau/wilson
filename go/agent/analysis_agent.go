package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/llm"
)

// AnalysisAgent specializes in research, web searches, and content analysis
type AnalysisAgent struct {
	*BaseAgent
}

// NewAnalysisAgent creates a new analysis agent
func NewAnalysisAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *AnalysisAgent {
	base := NewBaseAgent("analysis", llm.PurposeAnalysis, llmManager, contextMgr)

	// Only allow web and analysis tools
	base.SetAllowedTools([]string{
		"search_web",
		"fetch_page",
		"extract_content",
		"analyze_content",
		"search_artifacts",
		"retrieve_context",
		"store_artifact",
		"leave_note",
	})
	base.SetCanDelegate(false) // Cannot delegate further

	return &AnalysisAgent{
		BaseAgent: base,
	}
}

// CanHandle checks if the analysis agent can handle a task
func (a *AnalysisAgent) CanHandle(task *Task) bool {
	return task.Type == TaskTypeResearch ||
		task.Type == TaskTypeAnalysis ||
		task.Type == TaskTypeSummary
}

// ExecuteWithContext executes a task with full TaskContext
func (a *AnalysisAgent) ExecuteWithContext(ctx context.Context, taskCtx *TaskContext) (*Result, error) {
	a.SetTaskContext(taskCtx)
	task := a.ConvertTaskContextToTask(taskCtx)
	return a.Execute(ctx, task)
}

// Execute executes a research or analysis task
func (a *AnalysisAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
	}

	// Get current context for background
	currentCtx, err := a.GetContext()
	if err != nil {
		currentCtx = nil
	}

	// Build prompt based on task type
	systemPrompt := a.buildSystemPrompt(task)
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Call LLM
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// Store response as artifact
	artifactType := contextpkg.ArtifactAnalysis
	if task.Type == TaskTypeResearch {
		artifactType = "research_result"
	} else if task.Type == TaskTypeSummary {
		artifactType = contextpkg.ArtifactSummary
	}

	artifact, err := a.StoreArtifact(
		artifactType,
		response,
		"analysis_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	// Leave note for other agents
	noteText := fmt.Sprintf("Completed %s: %s. Results stored as artifact #%d",
		task.Type, task.Description, artifact.ID)
	_ = a.LeaveNote("", noteText) // Broadcast to all

	result.Success = true
	result.Output = response
	result.Metadata = map[string]interface{}{
		"model":       "analysis",
		"task_type":   task.Type,
		"artifact_id": artifact.ID,
	}

	return result, nil
}

func (a *AnalysisAgent) buildSystemPrompt(task *Task) string {
	// Start with shared core principles
	prompt := BuildSharedPrompt("Analysis Agent")

	// Add Analysis Agent specific instructions based on task type
	switch task.Type {
	case TaskTypeResearch:
		prompt += `
You are the RESEARCH SPECIALIST. You find information, don't create it.

=== YOUR ROLE ===

You FIND facts using tools. You do NOT provide information from "knowledge".

**CRITICAL ANTI-HALLUCINATION RULE:**
❌ "Based on my knowledge, X is..."
✅ {"tool": "search_web", "arguments": {"query": "..."}}

=== RESEARCH WORKFLOW ===

1. **Search** - Use search_web to find information
2. **Fetch** - Use fetch_page to get full content
3. **Extract** - Use extract_content to get key info
4. **Analyze** - Use analyze_content to synthesize
5. **Store** - Save findings as artifacts

=== AVAILABLE TOOLS ===

- **search_web**: Find information online
- **fetch_page**: Get full page content
- **extract_content**: Extract key information
- **analyze_content**: Synthesize and summarize
- **search_artifacts**: Find previous research
- **store_artifact**: Save findings
- **leave_note**: Share results with other agents

=== DELIVERABLES ===

1. Factual findings (sourced from web searches)
2. Key insights and patterns
3. Source citations when possible
4. Clear, structured summary
5. Stored as artifact for future reference

Remember: You are a FINDER, not a creator. Use tools for every fact.`

	case TaskTypeAnalysis:
		prompt += `
You are the CONTENT ANALYST. You analyze existing information.

=== YOUR ROLE ===

You analyze content that EXISTS. You do NOT analyze imaginary content.

**CRITICAL:**
Before analysis: Retrieve the content with tools
During analysis: Extract patterns, insights, themes
After analysis: Store structured findings

=== ANALYSIS WORKFLOW ===

1. **Retrieve** - Get content via retrieve_context or fetch_page
2. **Parse** - Extract key elements
3. **Identify** - Find patterns, themes, connections
4. **Synthesize** - Create insights
5. **Store** - Save analysis as artifact

=== AVAILABLE TOOLS ===

- **retrieve_context**: Get stored context
- **search_artifacts**: Find previous analysis
- **analyze_content**: Perform analysis
- **extract_content**: Get key points
- **store_artifact**: Save analysis
- **leave_note**: Share insights

=== DELIVERABLES ===

1. Systematic analysis
2. Key patterns and themes
3. Actionable insights
4. Clear structure
5. Stored findings

Remember: Analyze what exists, don't imagine content.`

	case TaskTypeSummary:
		prompt += `
You are the SUMMARIZATION SPECIALIST.

=== YOUR ROLE ===

Create concise, accurate summaries of existing content.

=== SUMMARIZATION WORKFLOW ===

1. **Gather** - Retrieve all relevant artifacts
2. **Review** - Identify most important information
3. **Synthesize** - Create clear, brief summary
4. **Highlight** - Note key takeaways
5. **Store** - Save summary

=== DELIVERABLES ===

- Concise summary (3-5 key points)
- Essential information only
- Clear takeaways
- Gaps or next steps noted

Focus on clarity and brevity. Essence without detail.`

	default:
		prompt += `
You are the ANALYSIS SPECIALIST. Research, analyze, summarize.

Use tools to gather information, analyze content, provide insights.
Store findings as artifacts. Leave notes for other agents.

Always use tools. Never hallucinate information.`
	}

	return prompt
}

func (a *AnalysisAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description))

	// Add context if available
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("Available context:\n")
		for i, artifact := range currentCtx.Artifacts {
			if i >= 10 { // Limit to last 10
				break
			}
			summary := artifact.Content
			if len(summary) > 150 {
				summary = summary[:150] + "..."
			}
			prompt.WriteString(fmt.Sprintf("- Artifact #%d [%s]: %s\n", artifact.ID, artifact.Type, summary))
		}
		prompt.WriteString("\n")
	}

	// Add task input
	if len(task.Input) > 0 {
		prompt.WriteString("Input:\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
	}

	return prompt.String()
}
