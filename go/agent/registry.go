package agent

import (
	"fmt"
	"sync"
)

// Registry manages all available agents
type Registry struct {
	agents map[string]Agent
	mu     sync.RWMutex
}

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]Agent),
	}
}

// Register registers an agent
func (r *Registry) Register(agent Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := agent.Name()
	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent '%s' already registered", name)
	}

	r.agents[name] = agent
	return nil
}

// Get retrieves an agent by name
func (r *Registry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent '%s' not found", name)
	}

	return agent, nil
}

// List returns all registered agents
func (r *Registry) List() []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}

	return agents
}

// ListInfo returns information about all agents
func (r *Registry) ListInfo() []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		info := AgentInfo{
			Name:         agent.Name(),
			Purpose:      string(agent.Purpose()),
			AllowedTools: agent.AllowedTools(),
			Status:       "available",
		}
		infos = append(infos, info)
	}

	return infos
}

// FindCapable finds agents capable of handling a task
func (r *Registry) FindCapable(task *Task) []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capable := make([]Agent, 0)
	for _, agent := range r.agents {
		if agent.CanHandle(task) {
			capable = append(capable, agent)
		}
	}

	return capable
}

// Global registry instance
var globalRegistry *Registry
var globalRegistryMu sync.RWMutex

// SetGlobalRegistry sets the global agent registry
func SetGlobalRegistry(registry *Registry) {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	globalRegistry = registry
}

// GetGlobalRegistry returns the global agent registry
func GetGlobalRegistry() *Registry {
	globalRegistryMu.RLock()
	defer globalRegistryMu.RUnlock()
	return globalRegistry
}
