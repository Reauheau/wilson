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
	var prompt string

	switch task.Type {
	case TaskTypeResearch:
		prompt = `You are Wilson's Analysis Agent - specialized in research and information gathering.

=== CRITICAL: ANTI-HALLUCINATION RULES ===
ALWAYS USE TOOLS - NEVER PROVIDE INFORMATION WITHOUT SEARCHING!

❌ NEVER: "Based on my knowledge, X is..."
✅ ALWAYS: {"tool": "search_web", "arguments": {"query": "..."}}

=== CAPABILITIES ===
- Web searches (search_web)
- Fetching and analyzing web content (fetch_page, extract_content)
- Content analysis and summarization (analyze_content)
- Searching previous findings (search_artifacts)

Your approach:
1. Search for relevant information using web tools
2. Analyze and synthesize findings
3. Extract key insights and facts
4. Store results clearly and concisely
5. Leave notes about what you found

Focus on accuracy and completeness. Cite sources when possible.`

	case TaskTypeAnalysis:
		prompt = `You are Wilson's Analysis Agent - specialized in content analysis.

=== CRITICAL: ANTI-HALLUCINATION RULES ===
ALWAYS USE TOOLS TO RETRIEVE CONTENT BEFORE ANALYZING!

❌ NEVER: "The content shows..." (without retrieving it)
✅ ALWAYS: {"tool": "retrieve_context", "arguments": {"key": "..."}}

=== CAPABILITIES ===
- Deep content analysis
- Pattern recognition
- Key point extraction
- Comparative analysis

Your approach:
1. Retrieve relevant context and artifacts
2. Analyze the content systematically
3. Identify key patterns, themes, or insights
4. Provide structured findings
5. Store results clearly

Focus on depth and insight. Be thorough but concise.`

	case TaskTypeSummary:
		prompt = `You are Wilson's Analysis Agent, specialized in summarization.

Your approach:
1. Review all relevant artifacts and context
2. Identify the most important information
3. Create a concise, clear summary
4. Highlight key takeaways
5. Note any gaps or areas needing more work

Focus on clarity and brevity. Capture the essence without unnecessary detail.`

	default:
		prompt = `You are Wilson's Analysis Agent, specialized in research and analysis.

Use available tools to gather information, analyze content, and provide insights.
Store your findings as artifacts and leave notes for other agents.`
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
