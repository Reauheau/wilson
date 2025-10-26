package lsp

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Manager manages multiple language server clients
// Handles lifecycle, routing, and caching for LSP operations
type Manager struct {
	clients map[string]*Client // language -> client (e.g., "go" -> gopls client)
	mu      sync.RWMutex
	cache   *ResponseCache
}

// NewManager creates a new LSP manager
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		cache:   NewResponseCache(),
	}
}

// GetClient returns or creates a client for the given language
func (m *Manager) GetClient(ctx context.Context, language string) (*Client, error) {
	m.mu.RLock()
	client, exists := m.clients[language]
	m.mu.RUnlock()

	if exists && client.IsRunning() {
		return client, nil
	}

	// Need to create new client
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := m.clients[language]; exists && client.IsRunning() {
		return client, nil
	}

	// Create and start new client
	client, err := NewClient(language)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s client: %w", language, err)
	}

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start %s language server: %w", language, err)
	}

	// Initialize the client with current directory as root
	// TODO: Make root URI configurable
	if err := client.Initialize(ctx, "file://"+getCurrentDir()); err != nil {
		_ = client.Stop()
		return nil, fmt.Errorf("failed to initialize %s client: %w", language, err)
	}

	m.clients[language] = client
	return client, nil
}

// GetClientForFile returns a client based on file extension
func (m *Manager) GetClientForFile(ctx context.Context, filePath string) (*Client, error) {
	language := detectLanguage(filePath)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}
	return m.GetClient(ctx, language)
}

// StopClient stops a language server client
func (m *Manager) StopClient(language string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[language]
	if !exists {
		return nil // Already stopped
	}

	if err := client.Stop(); err != nil {
		return fmt.Errorf("failed to stop %s client: %w", language, err)
	}

	delete(m.clients, language)
	return nil
}

// StopAll stops all language server clients
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for lang, client := range m.clients {
		if err := client.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", lang, err))
		}
	}

	m.clients = make(map[string]*Client)

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping clients: %v", errs)
	}
	return nil
}

// RestartClient stops and restarts a client
func (m *Manager) RestartClient(ctx context.Context, language string) error {
	if err := m.StopClient(language); err != nil {
		return err
	}
	_, err := m.GetClient(ctx, language)
	return err
}

// detectLanguage returns the language name based on file extension
func detectLanguage(filePath string) string {
	// Simple extension-based detection
	// Will expand this as we add more language support
	switch {
	case len(filePath) >= 3 && filePath[len(filePath)-3:] == ".go":
		return "go"
	case len(filePath) >= 3 && filePath[len(filePath)-3:] == ".py":
		return "python"
	case len(filePath) >= 3 && filePath[len(filePath)-3:] == ".js":
		return "javascript"
	case len(filePath) >= 3 && filePath[len(filePath)-3:] == ".ts":
		return "typescript"
	case len(filePath) >= 3 && filePath[len(filePath)-3:] == ".rs":
		return "rust"
	default:
		return ""
	}
}

// getCurrentDir returns the current working directory
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return dir
}
