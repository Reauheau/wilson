package agent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"wilson/agent"
	"wilson/agent/base"
	"wilson/llm"
)

// mockAgent is a simple mock agent for testing
type mockAgent struct {
	name         string
	purpose      llm.Purpose
	allowedTools []string
	canHandle    func(*agent.Task) bool
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Purpose() llm.Purpose {
	return m.purpose
}

func (m *mockAgent) CanHandle(task *agent.Task) bool {
	if m.canHandle != nil {
		return m.canHandle(task)
	}
	return true
}

func (m *mockAgent) Execute(ctx context.Context, task *agent.Task) (*agent.Result, error) {
	return &agent.Result{
		TaskID:  task.ID,
		Success: true,
		Output:  "mock result",
		Agent:   m.name,
	}, nil
}

func (m *mockAgent) ExecuteWithContext(ctx context.Context, taskCtx *base.TaskContext) (*agent.Result, error) {
	// Convert TaskContext to Task for mock execution
	task := &agent.Task{
		ID:          taskCtx.TaskID,
		Type:        taskCtx.Type,
		Description: taskCtx.Description,
		Input:       taskCtx.Input,
		Priority:    taskCtx.Priority,
		Status:      agent.TaskPending,
	}
	return m.Execute(ctx, task)
}

func (m *mockAgent) AllowedTools() []string {
	return m.allowedTools
}

func TestNewRegistry(t *testing.T) {
	registry := agent.NewRegistry()

	require.NotNil(t, registry)
	// Verify registry is empty by listing agents
	agents := registry.List()
	assert.Len(t, agents, 0)
}

func TestRegisterAgent(t *testing.T) {
	registry := agent.NewRegistry()

	mockAg := &mockAgent{
		name:    "test-agent",
		purpose: llm.PurposeChat,
	}

	err := registry.Register(mockAg)
	require.NoError(t, err)

	// Verify agent was registered
	agents := registry.List()
	assert.Len(t, agents, 1)
	assert.Equal(t, "test-agent", agents[0].Name())
}

func TestRegisterAgentDuplicate(t *testing.T) {
	registry := agent.NewRegistry()

	agent1 := &mockAgent{name: "duplicate"}
	agent2 := &mockAgent{name: "duplicate"}

	// First registration should succeed
	err := registry.Register(agent1)
	require.NoError(t, err)

	// Second registration with same name should fail
	err = registry.Register(agent2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestGetAgent(t *testing.T) {
	registry := agent.NewRegistry()

	mockAg := &mockAgent{
		name:    "get-test",
		purpose: llm.PurposeAnalysis,
	}

	err := registry.Register(mockAg)
	require.NoError(t, err)

	// Get the agent
	retrieved, err := registry.Get("get-test")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "get-test", retrieved.Name())
	assert.Equal(t, llm.PurposeAnalysis, retrieved.Purpose())
}

func TestGetAgentNotFound(t *testing.T) {
	registry := agent.NewRegistry()

	_, err := registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListAgents(t *testing.T) {
	registry := agent.NewRegistry()

	// Register multiple agents
	agents := []*mockAgent{
		{name: "agent-1", purpose: llm.PurposeChat},
		{name: "agent-2", purpose: llm.PurposeAnalysis},
		{name: "agent-3", purpose: llm.PurposeCode},
	}

	for _, mockAg := range agents {
		err := registry.Register(mockAg)
		require.NoError(t, err)
	}

	// List all agents
	listed := registry.List()
	assert.Len(t, listed, 3)

	// Verify all names are present
	names := make(map[string]bool)
	for _, agent := range listed {
		names[agent.Name()] = true
	}

	assert.True(t, names["agent-1"])
	assert.True(t, names["agent-2"])
	assert.True(t, names["agent-3"])
}

func TestListInfo(t *testing.T) {
	registry := agent.NewRegistry()

	mockAg := &mockAgent{
		name:         "info-test",
		purpose:      llm.PurposeChat,
		allowedTools: []string{"tool1", "tool2"},
	}

	err := registry.Register(mockAg)
	require.NoError(t, err)

	// Get info
	infos := registry.ListInfo()
	require.Len(t, infos, 1)

	info := infos[0]
	assert.Equal(t, "info-test", info.Name)
	assert.Equal(t, string(llm.PurposeChat), info.Purpose)
	assert.Equal(t, "available", info.Status)
	assert.Equal(t, []string{"tool1", "tool2"}, info.AllowedTools)
}

func TestFindCapable(t *testing.T) {
	registry := agent.NewRegistry()

	// Register agents with different capabilities
	chatAgent := &mockAgent{
		name:    "chat",
		purpose: llm.PurposeChat,
		canHandle: func(task *agent.Task) bool {
			return task.Type == agent.TaskTypeGeneral
		},
	}

	analysisAgent := &mockAgent{
		name:    "analysis",
		purpose: llm.PurposeAnalysis,
		canHandle: func(task *agent.Task) bool {
			return task.Type == agent.TaskTypeResearch || task.Type == agent.TaskTypeAnalysis
		},
	}

	codeAgent := &mockAgent{
		name:    "code",
		purpose: llm.PurposeCode,
		canHandle: func(task *agent.Task) bool {
			return task.Type == agent.TaskTypeCode
		},
	}

	registry.Register(chatAgent)
	registry.Register(analysisAgent)
	registry.Register(codeAgent)

	// Test finding agents for research task
	researchTask := &agent.Task{
		Type:        agent.TaskTypeResearch,
		Description: "Research something",
	}

	capable := registry.FindCapable(researchTask)
	require.Len(t, capable, 1)
	assert.Equal(t, "analysis", capable[0].Name())

	// Test finding agents for code task
	codeTask := &agent.Task{
		Type:        agent.TaskTypeCode,
		Description: "Write code",
	}

	capable = registry.FindCapable(codeTask)
	require.Len(t, capable, 1)
	assert.Equal(t, "code", capable[0].Name())

	// Test finding agents for general task
	generalTask := &agent.Task{
		Type:        agent.TaskTypeGeneral,
		Description: "Do something",
	}

	capable = registry.FindCapable(generalTask)
	require.Len(t, capable, 1)
	assert.Equal(t, "chat", capable[0].Name())
}

func TestFindCapableMultiple(t *testing.T) {
	registry := agent.NewRegistry()

	// Register two agents that can both handle the same task type
	agent1 := &mockAgent{
		name:    "agent-1",
		purpose: llm.PurposeAnalysis,
		canHandle: func(task *agent.Task) bool {
			return task.Type == agent.TaskTypeAnalysis
		},
	}

	agent2 := &mockAgent{
		name:    "agent-2",
		purpose: llm.PurposeAnalysis,
		canHandle: func(task *agent.Task) bool {
			return task.Type == agent.TaskTypeAnalysis
		},
	}

	registry.Register(agent1)
	registry.Register(agent2)

	task := &agent.Task{
		Type:        agent.TaskTypeAnalysis,
		Description: "Analyze data",
	}

	capable := registry.FindCapable(task)
	assert.Len(t, capable, 2, "Both agents should be capable")
}

func TestFindCapableNone(t *testing.T) {
	registry := agent.NewRegistry()

	// Register agent that can't handle code tasks
	mockAg := &mockAgent{
		name:    "chat-only",
		purpose: llm.PurposeChat,
		canHandle: func(task *agent.Task) bool {
			return task.Type == agent.TaskTypeGeneral
		},
	}

	registry.Register(mockAg)

	// Try to find agent for code task
	codeTask := &agent.Task{
		Type:        agent.TaskTypeCode,
		Description: "Write code",
	}

	capable := registry.FindCapable(codeTask)
	assert.Len(t, capable, 0, "No agents should be capable")
}

func TestGlobalRegistry(t *testing.T) {
	// Save original global registry
	originalRegistry := agent.GetGlobalRegistry()
	defer agent.SetGlobalRegistry(originalRegistry)

	// Create and set new registry
	registry := agent.NewRegistry()
	agent.SetGlobalRegistry(registry)

	// Verify it was set
	retrieved := agent.GetGlobalRegistry()
	assert.Equal(t, registry, retrieved)

	// Register an agent
	mockAg := &mockAgent{name: "global-test"}
	registry.Register(mockAg)

	// Verify we can get it through global registry
	globalAgent, err := agent.GetGlobalRegistry().Get("global-test")
	require.NoError(t, err)
	assert.Equal(t, "global-test", globalAgent.Name())
}

func TestConcurrentRegistration(t *testing.T) {
	registry := agent.NewRegistry()

	// Register agents concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(index int) {
			mockAg := &mockAgent{
				name:    "concurrent-" + string(rune('0'+index)),
				purpose: llm.PurposeChat,
			}
			registry.Register(mockAg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all agents were registered
	agents := registry.List()
	assert.Len(t, agents, 10)
}

func TestConcurrentListAndRegister(t *testing.T) {
	registry := agent.NewRegistry()

	// Initial agent
	registry.Register(&mockAgent{name: "initial"})

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			registry.List()
			registry.ListInfo()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(index int) {
			mockAg := &mockAgent{
				name: "writer-" + string(rune('0'+index)),
			}
			registry.Register(mockAg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 6 agents total (1 initial + 5 writers)
	agents := registry.List()
	assert.Len(t, agents, 6)
}
