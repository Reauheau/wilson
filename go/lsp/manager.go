package lsp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
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

// detectLanguage returns the language name and ID based on file extension
// Returns (language, languageID) where language is used for server selection
// and languageID is passed to the LSP server for syntax detection
func detectLanguage(filePath string) string {
	lang, _ := detectLanguageAndID(filePath)
	return lang
}

// detectLanguageAndID performs enhanced language detection with full extension support
func detectLanguageAndID(filePath string) (language string, languageID string) {
	// Get file extension
	ext := ""
	for i := len(filePath) - 1; i >= 0 && i > len(filePath)-10; i-- {
		if filePath[i] == '.' {
			ext = filePath[i:]
			break
		}
		if filePath[i] == '/' || filePath[i] == '\\' {
			break
		}
	}

	// Extension-based detection with full multi-language support
	switch ext {
	// Go
	case ".go":
		return "go", "go"

	// Python
	case ".py":
		return "python", "python"
	case ".pyi": // Python type stubs
		return "python", "python"

	// JavaScript variants
	case ".js":
		return "javascript", "javascript"
	case ".mjs": // ES modules
		return "javascript", "javascript"
	case ".cjs": // CommonJS
		return "javascript", "javascript"
	case ".jsx": // React JSX
		return "javascript", "javascriptreact"

	// TypeScript variants
	case ".ts":
		return "typescript", "typescript"
	case ".tsx": // React TSX
		return "typescript", "typescriptreact"
	case ".mts": // ES module TypeScript
		return "typescript", "typescript"
	case ".cts": // CommonJS TypeScript
		return "typescript", "typescript"

	// Rust
	case ".rs":
		return "rust", "rust"

	default:
		// Try shebang detection for scripts without extension
		if ext == "" {
			if lang := detectShebang(filePath); lang != "" {
				return lang, lang
			}
		}
		return "", ""
	}
}

// detectShebang reads the first line to detect interpreter for extensionless scripts
func detectShebang(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read first line only
	buf := make([]byte, 256)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return ""
	}

	firstLine := string(buf[:n])
	// Find newline
	if idx := strings.Index(firstLine, "\n"); idx != -1 {
		firstLine = firstLine[:idx]
	}

	// Check for shebang
	if !strings.HasPrefix(firstLine, "#!") {
		return ""
	}

	// Detect language from shebang
	shebang := strings.ToLower(firstLine)
	if strings.Contains(shebang, "python") {
		return "python"
	}
	if strings.Contains(shebang, "node") || strings.Contains(shebang, "bun") || strings.Contains(shebang, "deno") {
		return "javascript"
	}

	return ""
}

// getCurrentDir returns the current working directory
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return dir
}
