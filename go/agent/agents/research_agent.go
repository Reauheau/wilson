package agents

import (
	"context"
	"fmt"
	"strings"
	"wilson/agent"
	"wilson/agent/base"

	contextpkg "wilson/context"
	"wilson/llm"
)

// ResearchAgent specializes in deep research with multi-source analysis
type ResearchAgent struct {
	*base.BaseAgent
}

// NewResearchAgent creates a new research agent
func NewResearchAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *ResearchAgent {
	base := base.NewBaseAgent("Research", llm.PurposeAnalysis, llmManager, contextMgr)

	// Research-specific tools
	base.SetAllowedTools([]string{
		"search_web",
		"fetch_page",
		"extract_content",
		"analyze_content",
		"research_topic", // Multi-site orchestrator
		"search_artifacts",
		"retrieve_context",
		"store_artifact",
		"leave_note",
	})
	base.SetCanDelegate(false)

	return &ResearchAgent{
		BaseAgent: base,
	}
}

// CanHandle checks if the research agent can handle a task
func (a *ResearchAgent) CanHandle(task *agent.Task) bool {
	return task.Type == agent.TaskTypeResearch
}

// ExecuteWithContext executes a task with full TaskContext
func (a *ResearchAgent) ExecuteWithContext(ctx context.Context, taskCtx *base.TaskContext) (*agent.Result, error) {
	a.SetTaskContext(taskCtx)
	task := a.ConvertTaskContextToTask(taskCtx)
	return a.Execute(ctx, task)
}

// Execute executes a research task using multi-source analysis
func (a *ResearchAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	result := &agent.Result{
		TaskID: task.ID,
		Agent:  a.Name(),
	}

	// Get current context for background
	currentCtx, err := a.GetContext()
	if err != nil {
		currentCtx = nil
	}

	// Build research-specific prompts
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Call LLM
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// Store response as research artifact
	artifact, err := a.StoreArtifact(
		"research_result",
		response,
		"research_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	// Leave note for other agents
	noteText := fmt.Sprintf("Completed research: %s. Findings stored as artifact #%d",
		task.Description, artifact.ID)
	_ = a.LeaveNote("", noteText) // Broadcast to all

	result.Success = true
	result.Output = response
	result.Metadata = map[string]interface{}{
		"model":       "analysis",
		"agent_type":  "research",
		"artifact_id": artifact.ID,
	}

	return result, nil
}

func (a *ResearchAgent) buildSystemPrompt() string {
	return `You are Wilson's Research Agent - a specialist in deep research and multi-source information gathering.

=== CRITICAL: ANTI-HALLUCINATION RULES ===
YOU MUST ACTUALLY USE TOOLS - NEVER JUST DESCRIBE RESEARCH!

❌ NEVER DO THIS (HALLUCINATION):
"I'll search for information about..."
"According to my research, X is..." (without actually searching)
"I would recommend searching these sites..."
"Here's what I found: [made up information]"

✅ ALWAYS DO THIS (ACTUAL EXECUTION):
{"tool": "research_topic", "arguments": {"topic": "...", "num_sources": 3}}
{"tool": "search_web", "arguments": {"query": "..."}}
{"tool": "fetch_page", "arguments": {"url": "https://..."}}
{"tool": "analyze_content", "arguments": {"content": "...", "mode": "summarize"}}

RULE: Never provide information without using a tool to actually research it first!
RULE: Always cite actual URLs from your tool results!

=== CAPABILITIES ===
- Multi-site web research (research_topic tool for comprehensive analysis)
- Individual web searches (search_web)
- Deep content analysis (fetch_page, extract_content, analyze_content)
- Cross-referencing previous findings (search_artifacts)

Your research methodology:
1. **Scope Definition**: Understand the research question thoroughly
2. **Multi-Source Gathering**: Use research_topic for comprehensive multi-site analysis
3. **Cross-Validation**: Compare information across sources for accuracy
4. **Synthesis**: Combine findings into coherent insights
5. **Documentation**: Store results with clear citations and sources
6. **Communication**: Leave notes about key findings for other agents

Quality standards:
- Always cite sources and URLs
- Note confidence levels and contradictions
- Highlight gaps in knowledge
- Provide actionable insights, not just data dumps
- Store comprehensive findings as artifacts

Focus on depth, accuracy, and actionable insights. You are the research expert in the team.`
}

func (a *ResearchAgent) buildUserPrompt(task *agent.Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	prompt.WriteString("## Research Task\n\n")
	prompt.WriteString(fmt.Sprintf("**Objective:** %s\n\n", task.Description))

	// Add context if available
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("## Previous Research Context\n\n")
		prompt.WriteString("Build upon these existing findings:\n\n")
		for i, artifact := range currentCtx.Artifacts {
			if i >= 5 { // Limit to last 5 for relevance
				break
			}
			summary := artifact.Content
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			prompt.WriteString(fmt.Sprintf("- **Artifact #%d** [%s]: %s\n", artifact.ID, artifact.Type, summary))
		}
		prompt.WriteString("\n")
	}

	// Add specific research parameters
	if len(task.Input) > 0 {
		prompt.WriteString("## Research Parameters\n\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("## Instructions\n\n")
	prompt.WriteString("1. Use research_topic tool for comprehensive multi-site analysis\n")
	prompt.WriteString("2. Validate information across multiple sources\n")
	prompt.WriteString("3. Provide a clear, well-structured report with:\n")
	prompt.WriteString("   - Executive summary\n")
	prompt.WriteString("   - Key findings\n")
	prompt.WriteString("   - Sources and citations\n")
	prompt.WriteString("   - Confidence assessments\n")
	prompt.WriteString("   - Gaps and recommendations\n")
	prompt.WriteString("4. Store the complete research in an artifact\n")
	prompt.WriteString("5. Leave a note summarizing key insights\n")

	return prompt.String()
}
