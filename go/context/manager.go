package context

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Manager provides high-level context operations
type Manager struct {
	store         *Store
	activeContext string // Current active context key
	mu            sync.RWMutex
	autoStore     bool
}

// NewManager creates a new context manager
func NewManager(dbPath string, autoStore bool) (*Manager, error) {
	store, err := NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	return &Manager{
		store:     store,
		autoStore: autoStore,
	}, nil
}

// Close closes the context manager
func (m *Manager) Close() error {
	return m.store.Close()
}

// GetDB returns the underlying database connection for task queue
func (m *Manager) GetDB() *sql.DB {
	if m.store == nil {
		return nil
	}
	return m.store.db
}

// SetActiveContext sets the current active context
func (m *Manager) SetActiveContext(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify context exists
	_, err := m.store.GetContext(key)
	if err != nil {
		return fmt.Errorf("context not found: %w", err)
	}

	m.activeContext = key
	return nil
}

// GetActiveContext returns the current active context key
func (m *Manager) GetActiveContext() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeContext
}

// CreateContext creates a new context and optionally makes it active
func (m *Manager) CreateContext(req CreateContextRequest, makeActive bool) (*Context, error) {
	// Set defaults
	if req.Type == "" {
		req.Type = TypeSession
	}
	if req.Key == "" {
		req.Key = fmt.Sprintf("%s-%d", req.Type, time.Now().Unix())
	}

	ctx, err := m.store.CreateContext(req)
	if err != nil {
		return nil, err
	}

	if makeActive {
		m.SetActiveContext(ctx.Key)
	}

	return ctx, nil
}

// GetOrCreateContext gets an existing context or creates it if it doesn't exist
func (m *Manager) GetOrCreateContext(key, contextType, title string) (*Context, error) {
	ctx, err := m.store.GetContext(key)
	if err == nil {
		return ctx, nil
	}

	// Context doesn't exist, create it
	return m.CreateContext(CreateContextRequest{
		Key:   key,
		Type:  contextType,
		Title: title,
	}, false)
}

// GetContext retrieves a context by key
func (m *Manager) GetContext(key string) (*Context, error) {
	return m.store.GetContext(key)
}

// ListContexts lists contexts with optional filtering
func (m *Manager) ListContexts(status string, limit int) ([]Context, error) {
	return m.store.ListContexts(status, limit)
}

// CompleteContext marks a context as completed
func (m *Manager) CompleteContext(key string) error {
	return m.store.UpdateContextStatus(key, StatusCompleted)
}

// ArchiveContext marks a context as archived
func (m *Manager) ArchiveContext(key string) error {
	return m.store.UpdateContextStatus(key, StatusArchived)
}

// StoreArtifact stores an artifact in a context
// If contextKey is empty, uses the active context
func (m *Manager) StoreArtifact(req StoreArtifactRequest) (*Artifact, error) {
	if req.ContextKey == "" {
		req.ContextKey = m.GetActiveContext()
	}

	if req.ContextKey == "" {
		return nil, fmt.Errorf("no context specified and no active context set")
	}

	return m.store.StoreArtifact(req)
}

// StoreToolResult is a convenience method to store tool results
func (m *Manager) StoreToolResult(toolName, result, agent string) (*Artifact, error) {
	contextKey := m.GetActiveContext()
	if contextKey == "" {
		return nil, fmt.Errorf("no active context set")
	}

	artifactType := mapToolToArtifactType(toolName)

	return m.store.StoreArtifact(StoreArtifactRequest{
		ContextKey: contextKey,
		Type:       artifactType,
		Content:    result,
		Source:     toolName,
		Agent:      agent,
		Metadata: ArtifactMetadata{
			Tags: []string{toolName},
		},
	})
}

// SearchArtifacts searches for artifacts
func (m *Manager) SearchArtifacts(req SearchArtifactsRequest) ([]SearchResult, error) {
	return m.store.SearchArtifacts(req)
}

// AddNote adds an inter-agent note
func (m *Manager) AddNote(contextKey, fromAgent, toAgent, note string) (*AgentNote, error) {
	if contextKey == "" {
		contextKey = m.GetActiveContext()
	}

	if contextKey == "" {
		return nil, fmt.Errorf("no context specified and no active context set")
	}

	return m.store.AddNote(contextKey, fromAgent, toAgent, note)
}

// GetContextSummary returns a summary of a context
func (m *Manager) GetContextSummary(key string) (string, error) {
	ctx, err := m.store.GetContext(key)
	if err != nil {
		return "", err
	}

	summary := fmt.Sprintf("Context: %s\n", ctx.Title)
	summary += fmt.Sprintf("Type: %s | Status: %s\n", ctx.Type, ctx.Status)
	summary += fmt.Sprintf("Created: %s | Updated: %s\n", ctx.CreatedAt.Format("2006-01-02 15:04"), ctx.UpdatedAt.Format("2006-01-02 15:04"))

	if ctx.Description != "" {
		summary += fmt.Sprintf("Description: %s\n", ctx.Description)
	}

	summary += fmt.Sprintf("\nArtifacts: %d\n", len(ctx.Artifacts))

	// Group artifacts by type
	typeCount := make(map[string]int)
	for _, artifact := range ctx.Artifacts {
		typeCount[artifact.Type]++
	}

	for artifactType, count := range typeCount {
		summary += fmt.Sprintf("  - %s: %d\n", artifactType, count)
	}

	if len(ctx.Notes) > 0 {
		summary += fmt.Sprintf("\nAgent Notes: %d\n", len(ctx.Notes))
	}

	return summary, nil
}

// IsAutoStoreEnabled returns whether auto-store is enabled
func (m *Manager) IsAutoStoreEnabled() bool {
	return m.autoStore
}

// Helper function to map tool names to artifact types
func mapToolToArtifactType(toolName string) string {
	mapping := map[string]string{
		"search_web":      ArtifactWebSearch,
		"fetch_page":      ArtifactWebPage,
		"extract_content": ArtifactExtractedText,
		"analyze_content": ArtifactAnalysis,
	}

	if artifactType, ok := mapping[toolName]; ok {
		return artifactType
	}

	return "tool_result"
}

// Global context manager instance
var globalManager *Manager
var globalManagerMu sync.RWMutex

// SetGlobalManager sets the global context manager
func SetGlobalManager(manager *Manager) {
	globalManagerMu.Lock()
	defer globalManagerMu.Unlock()
	globalManager = manager
}

// GetGlobalManager returns the global context manager
func GetGlobalManager() *Manager {
	globalManagerMu.RLock()
	defer globalManagerMu.RUnlock()
	return globalManager
}
