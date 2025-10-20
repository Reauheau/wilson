package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/llm"
)

// ChatAgent is the main orchestrator agent that delegates to specialists
type ChatAgent struct {
	*BaseAgent
}

// NewChatAgent creates a new chat agent
func NewChatAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *ChatAgent {
	base := NewBaseAgent("chat", llm.PurposeChat, llmManager, contextMgr)
	base.SetAllowedTools([]string{"*"}) // Can use all tools
	base.SetCanDelegate(true)

	return &ChatAgent{
		BaseAgent: base,
	}
}

// CanHandle checks if the chat agent can handle a task
func (a *ChatAgent) CanHandle(task *Task) bool {
	// Chat agent can handle any general task
	return task.Type == TaskTypeGeneral || task.Type == ""
}

// Execute executes a task
func (a *ChatAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
	}

	// Get current context for background
	currentCtx, err := a.GetContext()
	if err != nil {
		// No context available, that's okay
		currentCtx = nil
	}

	// Build prompt
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(task, currentCtx)

	// Call LLM
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// Store response as artifact
	artifact, err := a.StoreArtifact(
		contextpkg.ArtifactLLMResponse,
		response,
		"chat_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	result.Success = true
	result.Output = response
	result.Metadata = map[string]interface{}{
		"model": "chat",
	}

	return result, nil
}

func (a *ChatAgent) buildSystemPrompt() string {
	basePrompt := `You are Wilson's Chat Agent, the main orchestrator of a multi-agent system.

Your responsibilities:
- Understand user requests and break them into subtasks
- Delegate specialized work to other agents (analysis, code, research)
- Synthesize results from multiple agents
- Provide natural, helpful responses to users

You can delegate tasks to:
- analysis agent: For research, web searches, content analysis, summarization
- code agent: For code generation, code review, technical implementation

When you need specialized help, explain what you need clearly and provide context.`

	// Phase 4: Add active task awareness
	coordinator := GetGlobalCoordinator()
	if coordinator != nil {
		activeTasks := coordinator.GetActiveTasks()
		if len(activeTasks) > 0 {
			basePrompt += "\n\nActive background tasks you're coordinating:\n"
			for _, task := range activeTasks {
				basePrompt += fmt.Sprintf("- Task %s (%s): %s",
					task.ID[:8], task.Type, task.Description)
				if task.ModelUsed != "" {
					basePrompt += fmt.Sprintf(" [using %s model]", task.ModelUsed)
				}
				basePrompt += "\n"
			}
			basePrompt += "\nYou can reference these tasks when answering questions. They're running in background while you chat."
		}
	}

	return basePrompt
}

func (a *ChatAgent) buildUserPrompt(task *Task, currentCtx *contextpkg.Context) string {
	var prompt strings.Builder

	// Add task description
	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description))

	// Add context if available
	if currentCtx != nil && len(currentCtx.Artifacts) > 0 {
		prompt.WriteString("Previous work in this context:\n")
		for i, artifact := range currentCtx.Artifacts {
			if i >= 5 { // Limit to last 5 artifacts
				break
			}
			summary := artifact.Content
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			prompt.WriteString(fmt.Sprintf("- [%s by %s]: %s\n", artifact.Type, artifact.Agent, summary))
		}
		prompt.WriteString("\n")
	}

	// Add task input if any
	if len(task.Input) > 0 {
		prompt.WriteString("Additional input:\n")
		for key, value := range task.Input {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
	}

	return prompt.String()
}
