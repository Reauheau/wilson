package agent

import (
	"context"
	"fmt"
	"strings"

	contextpkg "wilson/context"
	"wilson/core/registry"
	"wilson/llm"
)

// ChatAgent is the main orchestrator agent that delegates to specialists
type ChatAgent struct {
	*BaseAgent
}

// NewChatAgent creates a new chat agent
func NewChatAgent(llmManager *llm.Manager, contextMgr *contextpkg.Manager) *ChatAgent {
	base := NewBaseAgent("chat", llm.PurposeChat, llmManager, contextMgr)

	// ChatAgent can ONLY use orchestration tools - must delegate specialized work
	base.SetAllowedTools([]string{
		// PRIMARY: Route code/execution tasks to ManagerAgent
		"orchestrate_code_task",

		// LEGACY: Simple delegation (for research/analysis)
		"delegate_task",
		"check_task_progress",
		"check_task_status",
		"get_task_queue",

		// Context and communication
		"search_artifacts",
		"retrieve_context",
		"leave_note",

		// Simple conversational tools (non-invasive)
		"list_files", // Read-only, safe for chat to use
	})
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

// ExecuteWithContext executes a task with full TaskContext
func (a *ChatAgent) ExecuteWithContext(ctx context.Context, taskCtx *TaskContext) (*Result, error) {
	a.SetTaskContext(taskCtx)
	task := a.ConvertTaskContextToTask(taskCtx)
	return a.Execute(ctx, task)
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

	// Call LLM with validation
	response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("LLM error: %v", err)
		return result, err
	}

	// Execute tools via AgentToolExecutor (same as Code Agent)
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
	artifact, err := a.StoreArtifact(
		contextpkg.ArtifactLLMResponse,
		execResult.Output,
		"chat_agent",
	)
	if err == nil {
		result.Artifacts = append(result.Artifacts, fmt.Sprintf("%d", artifact.ID))
	}

	result.Success = true
	result.Output = execResult.Output
	result.Metadata = map[string]interface{}{
		"model":          "chat",
		"tools_executed": execResult.ToolsExecuted,
	}

	return result, nil
}

func (a *ChatAgent) buildSystemPrompt() string {
	// Start with shared core principles
	basePrompt := BuildSharedPrompt("Chat Agent")

	// Add Chat Agent specific instructions
	basePrompt += `
You are the ROUTER and ORCHESTRATOR. You delegate ALL specialized work.

=== YOUR RESPONSIBILITIES ===

1. **Understand** user requests
2. **Route** to appropriate orchestration system
3. **Respond** to user with results

You do NOT write code. You do NOT do research. You ROUTE to specialists.

=== ROUTING RULES ===

**Code/Execution Tasks** - Use orchestrate_code_task (PRIMARY):
- Writing code, creating programs
- Creating/modifying files
- Building, compiling, testing
- Code analysis, refactoring
- **ANY task involving files, execution, or compilation**

Format:
{"tool": "orchestrate_code_task", "arguments": {"request": "full user request here"}}

ManagerAgent will automatically:
- Detect if task is simple (1 file) or complex (multiple files/steps)
- Decompose complex tasks into subtasks
- Execute sequentially with proper dependencies
- Route to Code/Test/Review agents as needed

**Research/Analysis Tasks** - Use delegate_task (LEGACY):
- Web searches, research
- Content analysis, summarization
- Information gathering (non-code)

Format:
{"tool": "delegate_task", "arguments": {"to_agent": "analysis", "task_type": "research", "description": "what to research"}}

**Simple Conversation** - Handle directly:
- Greetings: "Hello! I'm Wilson, your local AI assistant."
- Questions about capabilities
- Status updates

=== EXAMPLES ===

User: "Create a Go program that opens Spotify"
→ {"tool": "orchestrate_code_task", "arguments": {"request": "Create a Go program that opens Spotify"}}

User: "Create a calculator in Go with tests and build"
→ {"tool": "orchestrate_code_task", "arguments": {"request": "Create a calculator in Go with tests and build"}}

User: "Research Ollama API"
→ {"tool": "delegate_task", "arguments": {"to_agent": "analysis", "task_type": "research", "description": "Research Ollama API endpoints"}}

User: "Hello!"
→ "Hello! I'm Wilson. What can I help you with?"

=== CRITICAL ===
For ANY coding/file/execution task → Use orchestrate_code_task
Let ManagerAgent handle complexity. You just route.`

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
