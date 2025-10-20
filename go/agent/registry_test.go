package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"wilson/llm"
)

// mockAgent is a simple mock agent for testing
type mockAgent struct {
	name         string
	purpose      llm.Purpose
	allowedTools []string
	canHandle    func(*Task) bool
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Purpose() llm.Purpose {
	return m.purpose
}

func (m *mockAgent) CanHandle(task *Task) bool {
	if m.canHandle != nil {
		return m.canHandle(task)
	}
	return true
}

func (m *mockAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	return &Result{
		TaskID:  task.ID,
		Success: true,
		Output:  "mock result",
		Agent:   m.name,
	}, nil
}

func (m *mockAgent) AllowedTools() []string {
	return m.allowedTools
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	require.NotNil(t, registry)
	assert.NotNil(t, registry.agents)
	assert.Len(t, registry.agents, 0)
}

func TestRegisterAgent(t *testing.T) {
	registry := NewRegistry()

	agent := &mockAgent{
		name:    "test-agent",
		purpose: llm.PurposeChat,
	}

	err := registry.Register(agent)
	require.NoError(t, err)

	// Verify agent was registered
	agents := registry.List()
	assert.Len(t, agents, 1)
	assert.Equal(t, "test-agent", agents[0].Name())
}

func TestRegisterAgentDuplicate(t *testing.T) {
	registry := NewRegistry()

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
	registry := NewRegistry()

	agent := &mockAgent{
		name:    "get-test",
		purpose: llm.PurposeAnalysis,
	}

	err := registry.Register(agent)
	require.NoError(t, err)

	// Get the agent
	retrieved, err := registry.Get("get-test")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "get-test", retrieved.Name())
	assert.Equal(t, llm.PurposeAnalysis, retrieved.Purpose())
}

func TestGetAgentNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListAgents(t *testing.T) {
	registry := NewRegistry()

	// Register multiple agents
	agents := []*mockAgent{
		{name: "agent-1", purpose: llm.PurposeChat},
		{name: "agent-2", purpose: llm.PurposeAnalysis},
		{name: "agent-3", purpose: llm.PurposeCode},
	}

	for _, agent := range agents {
		err := registry.Register(agent)
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
	registry := NewRegistry()

	agent := &mockAgent{
		name:         "info-test",
		purpose:      llm.PurposeChat,
		allowedTools: []string{"tool1", "tool2"},
	}

	err := registry.Register(agent)
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
	registry := NewRegistry()

	// Register agents with different capabilities
	chatAgent := &mockAgent{
		name:    "chat",
		purpose: llm.PurposeChat,
		canHandle: func(task *Task) bool {
			return task.Type == TaskTypeGeneral
		},
	}

	analysisAgent := &mockAgent{
		name:    "analysis",
		purpose: llm.PurposeAnalysis,
		canHandle: func(task *Task) bool {
			return task.Type == TaskTypeResearch || task.Type == TaskTypeAnalysis
		},
	}

	codeAgent := &mockAgent{
		name:    "code",
		purpose: llm.PurposeCode,
		canHandle: func(task *Task) bool {
			return task.Type == TaskTypeCode
		},
	}

	registry.Register(chatAgent)
	registry.Register(analysisAgent)
	registry.Register(codeAgent)

	// Test finding agents for research task
	researchTask := &Task{
		Type:        TaskTypeResearch,
		Description: "Research something",
	}

	capable := registry.FindCapable(researchTask)
	require.Len(t, capable, 1)
	assert.Equal(t, "analysis", capable[0].Name())

	// Test finding agents for code task
	codeTask := &Task{
		Type:        TaskTypeCode,
		Description: "Write code",
	}

	capable = registry.FindCapable(codeTask)
	require.Len(t, capable, 1)
	assert.Equal(t, "code", capable[0].Name())

	// Test finding agents for general task
	generalTask := &Task{
		Type:        TaskTypeGeneral,
		Description: "Do something",
	}

	capable = registry.FindCapable(generalTask)
	require.Len(t, capable, 1)
	assert.Equal(t, "chat", capable[0].Name())
}

func TestFindCapableMultiple(t *testing.T) {
	registry := NewRegistry()

	// Register two agents that can both handle the same task type
	agent1 := &mockAgent{
		name:    "agent-1",
		purpose: llm.PurposeAnalysis,
		canHandle: func(task *Task) bool {
			return task.Type == TaskTypeAnalysis
		},
	}

	agent2 := &mockAgent{
		name:    "agent-2",
		purpose: llm.PurposeAnalysis,
		canHandle: func(task *Task) bool {
			return task.Type == TaskTypeAnalysis
		},
	}

	registry.Register(agent1)
	registry.Register(agent2)

	task := &Task{
		Type:        TaskTypeAnalysis,
		Description: "Analyze data",
	}

	capable := registry.FindCapable(task)
	assert.Len(t, capable, 2, "Both agents should be capable")
}

func TestFindCapableNone(t *testing.T) {
	registry := NewRegistry()

	// Register agent that can't handle code tasks
	agent := &mockAgent{
		name:    "chat-only",
		purpose: llm.PurposeChat,
		canHandle: func(task *Task) bool {
			return task.Type == TaskTypeGeneral
		},
	}

	registry.Register(agent)

	// Try to find agent for code task
	codeTask := &Task{
		Type:        TaskTypeCode,
		Description: "Write code",
	}

	capable := registry.FindCapable(codeTask)
	assert.Len(t, capable, 0, "No agents should be capable")
}

func TestGlobalRegistry(t *testing.T) {
	// Save original global registry
	originalRegistry := GetGlobalRegistry()
	defer SetGlobalRegistry(originalRegistry)

	// Create and set new registry
	registry := NewRegistry()
	SetGlobalRegistry(registry)

	// Verify it was set
	retrieved := GetGlobalRegistry()
	assert.Equal(t, registry, retrieved)

	// Register an agent
	agent := &mockAgent{name: "global-test"}
	registry.Register(agent)

	// Verify we can get it through global registry
	globalAgent, err := GetGlobalRegistry().Get("global-test")
	require.NoError(t, err)
	assert.Equal(t, "global-test", globalAgent.Name())
}

func TestConcurrentRegistration(t *testing.T) {
	registry := NewRegistry()

	// Register agents concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(index int) {
			agent := &mockAgent{
				name:    "concurrent-" + string(rune('0'+index)),
				purpose: llm.PurposeChat,
			}
			registry.Register(agent)
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
	registry := NewRegistry()

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
			agent := &mockAgent{
				name: "writer-" + string(rune('0'+index)),
			}
			registry.Register(agent)
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
