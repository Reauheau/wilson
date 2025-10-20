package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ModelInstance tracks an active model with reference counting
type ModelInstance struct {
	client       Client
	lastUsed     time.Time
	refCount     int
	keepAlive    bool
	idleTimeout  time.Duration
	unloadTimer  *time.Timer
	mu           sync.Mutex
}

// Manager manages multiple LLM clients for different purposes
type Manager struct {
	clients   map[Purpose]Client          // Registered clients (may not be loaded)
	configs   map[Purpose]Config          // Configurations
	instances map[Purpose]*ModelInstance  // Active instances with ref counting
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new LLM manager
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		clients:   make(map[Purpose]Client),
		configs:   make(map[Purpose]Config),
		instances: make(map[Purpose]*ModelInstance),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start cleanup routine
	go m.cleanupRoutine()

	return m
}

// Stop gracefully shuts down the manager
func (m *Manager) Stop() {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel all unload timers
	for _, instance := range m.instances {
		instance.mu.Lock()
		if instance.unloadTimer != nil {
			instance.unloadTimer.Stop()
		}
		instance.mu.Unlock()
	}
}

// RegisterLLM registers an LLM for a specific purpose
func (m *Manager) RegisterLLM(purpose Purpose, config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store config
	m.configs[purpose] = config

	// Create client based on provider
	var client Client
	var err error

	switch config.Provider {
	case "ollama":
		client, err = NewOllamaClient(config)
	default:
		return fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	if err != nil {
		return fmt.Errorf("failed to create %s client: %w", config.Provider, err)
	}

	m.clients[purpose] = client
	return nil
}

// GetClient returns the LLM client for a specific purpose
// If the requested client is not available, it falls back to the chat client
func (m *Manager) GetClient(purpose Purpose) (Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try to get the requested client
	if client, ok := m.clients[purpose]; ok {
		return client, nil
	}

	// Fallback to chat client if available
	if purpose != PurposeChat {
		if chatClient, ok := m.clients[PurposeChat]; ok {
			return chatClient, nil
		}
	}

	return nil, fmt.Errorf("no LLM available for purpose: %s", purpose)
}

// Generate sends a request to the appropriate LLM based on purpose
// If the requested LLM is not available, it will try fallback options
func (m *Manager) Generate(ctx context.Context, purpose Purpose, req Request) (*Response, error) {
	client, err := m.GetClient(purpose)
	if err != nil {
		return nil, err
	}

	// Check if client is available
	if !client.IsAvailable(ctx) {
		// Try fallback if not the chat client
		if purpose != PurposeChat {
			chatClient, err := m.GetClient(PurposeChat)
			if err == nil && chatClient.IsAvailable(ctx) {
				client = chatClient
			} else {
				return nil, fmt.Errorf("LLM for %s is not available and no fallback found", purpose)
			}
		} else {
			return nil, fmt.Errorf("chat LLM is not available")
		}
	}

	return client.Generate(ctx, req)
}

// IsAvailable checks if an LLM for the given purpose is available
func (m *Manager) IsAvailable(purpose Purpose) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[purpose]
	if !ok {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.IsAvailable(ctx)
}

// ListAvailable returns a list of purposes that have available LLMs
func (m *Manager) ListAvailable() []Purpose {
	m.mu.RLock()
	defer m.mu.RUnlock()

	available := make([]Purpose, 0, len(m.clients))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for purpose, client := range m.clients {
		if client.IsAvailable(ctx) {
			available = append(available, purpose)
		}
	}

	return available
}

// GetConfig returns the configuration for a specific purpose
func (m *Manager) GetConfig(purpose Purpose) (Config, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.configs[purpose]
	return config, ok
}

// AcquireModel acquires a model for use with reference counting
// Returns the client, a release function that MUST be called when done, whether fallback was used, and error
// The release function should be deferred immediately after acquisition
func (m *Manager) AcquireModel(purpose Purpose) (Client, func(), bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	usedFallback := false

	// Check if client is registered
	client, ok := m.clients[purpose]
	if !ok {
		// Try fallback to chat if available
		if purpose != PurposeChat {
			client, ok = m.clients[PurposeChat]
			if ok {
				purpose = PurposeChat
				usedFallback = true // Phase 5: Track that we used fallback
			}
		}
		if !ok {
			return nil, nil, false, fmt.Errorf("no model registered for purpose: %s", purpose)
		}
	}

	// Get or create instance
	instance, ok := m.instances[purpose]
	if !ok {
		config := m.configs[purpose]
		idleTimeout := time.Duration(config.IdleTimeout) * time.Second

		instance = &ModelInstance{
			client:      client,
			lastUsed:    time.Now(),
			refCount:    0,
			keepAlive:   config.KeepAlive,
			idleTimeout: idleTimeout,
		}
		m.instances[purpose] = instance
	}

	// Increment reference count
	instance.mu.Lock()
	instance.refCount++
	instance.lastUsed = time.Now()

	// Cancel unload timer if exists (model is being used)
	if instance.unloadTimer != nil {
		instance.unloadTimer.Stop()
		instance.unloadTimer = nil
	}
	instance.mu.Unlock()

	// Create release function
	releaseFunc := func() {
		m.releaseModel(purpose)
	}

	return client, releaseFunc, usedFallback, nil
}

// releaseModel decrements the reference count and starts idle timeout if needed
func (m *Manager) releaseModel(purpose Purpose) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.instances[purpose]
	if !ok {
		return
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	// Decrement reference count
	instance.refCount--
	if instance.refCount < 0 {
		instance.refCount = 0 // Safety check
	}

	// If no more references and not keep-alive, start unload timer
	if instance.refCount == 0 && !instance.keepAlive {
		if instance.idleTimeout == 0 {
			// Immediate unload
			delete(m.instances, purpose)
		} else if instance.idleTimeout > 0 {
			// Schedule unload after timeout
			instance.unloadTimer = time.AfterFunc(instance.idleTimeout, func() {
				m.unloadModel(purpose)
			})
		}
	}
}

// unloadModel removes an idle model instance
func (m *Manager) unloadModel(purpose Purpose) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.instances[purpose]
	if !ok {
		return
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	// Only unload if refCount is still 0 (not re-acquired during timeout)
	if instance.refCount == 0 && !instance.keepAlive {
		delete(m.instances, purpose)
	}
}

// cleanupRoutine periodically checks for stale instances
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupStaleInstances()
		}
	}
}

// cleanupStaleInstances force-unloads any instances with refCount=0 (safety net)
func (m *Manager) cleanupStaleInstances() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for purpose, instance := range m.instances {
		instance.mu.Lock()
		shouldCleanup := instance.refCount == 0 && !instance.keepAlive
		instance.mu.Unlock()

		if shouldCleanup {
			delete(m.instances, purpose)
		}
	}
}

// GetRefCount returns the current reference count for a purpose (for testing)
func (m *Manager) GetRefCount(purpose Purpose) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, ok := m.instances[purpose]
	if !ok {
		return 0
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()

	return instance.refCount
}

// IsLoaded returns true if a model is currently loaded (for testing)
func (m *Manager) IsLoaded(purpose Purpose) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.instances[purpose]
	return ok
}
