package setup

import (
	"context"

	"wilson/agent"
	"wilson/agent/agents"
	"wilson/agent/orchestration"
	contextpkg "wilson/context"
	"wilson/llm"
)

// InitializeAgentSystem creates and configures the agent system (silent)
func InitializeAgentSystem(llmMgr *llm.Manager, contextMgr *contextpkg.Manager) *agents.ChatAgent {
	if llmMgr == nil || contextMgr == nil {
		return nil
	}

	// Create agent registry
	agentRegistry := agent.NewRegistry()

	// Register agents
	chatAgent := agents.NewChatAgent(llmMgr, contextMgr)
	analysisAgent := agents.NewAnalysisAgent(llmMgr, contextMgr)
	codeAgent := agents.NewCodeAgent(llmMgr, contextMgr)
	testAgent := agents.NewTestAgent(llmMgr, contextMgr)
	reviewAgent := agents.NewReviewAgent(llmMgr, contextMgr)

	_ = agentRegistry.Register(chatAgent)
	_ = agentRegistry.Register(analysisAgent)
	_ = agentRegistry.Register(codeAgent)
	_ = agentRegistry.Register(testAgent)
	_ = agentRegistry.Register(reviewAgent)

	// Create coordinator
	coordinator := orchestration.NewCoordinator(agentRegistry)

	// Set LLM manager for model lifecycle (Phase 2)
	coordinator.SetLLMManager(llmMgr)

	// Initialize Manager Agent with task queue
	// Use same database as context store for tasks
	db := contextMgr.GetDB()
	if db != nil {
		managerAgent := orchestration.NewManagerAgent(db)
		managerAgent.SetLLMManager(llmMgr)
		managerAgent.SetRegistry(agentRegistry)
		coordinator.SetManager(managerAgent)

		// ✅ START FEEDBACK PROCESSING (Phase 1)
		managerAgent.StartFeedbackProcessing(context.Background())
	}

	// Configure max concurrent workers (default: 2 for 16GB RAM)
	// coordinator.SetMaxConcurrent(2) // Can be configured via config.yaml

	// Set globals
	agent.SetGlobalRegistry(agentRegistry)
	orchestration.SetGlobalCoordinator(coordinator)

	return chatAgent
}
