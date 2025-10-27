package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client represents a Language Server Protocol client
// Manages communication with a language server process (e.g., gopls)
type Client struct {
	language    string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	running     atomic.Bool
	nextID      atomic.Int64
	mu          sync.RWMutex
	pending     map[int64]chan *jsonrpcResponse
	initialized bool
	rootURI     string
	// Diagnostic storage
	diagnosticsMu sync.RWMutex
	diagnostics   map[string][]Diagnostic // uri -> diagnostics
}

// jsonrpcRequest represents a JSON-RPC 2.0 request
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcError represents a JSON-RPC 2.0 error
type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewClient creates a new LSP client for the given language
func NewClient(language string) (*Client, error) {
	// Validate language and get executable path
	executable := getLanguageServerExecutable(language)
	if executable == "" {
		return nil, fmt.Errorf("no language server configured for %s", language)
	}

	return &Client{
		language:    language,
		pending:     make(map[int64]chan *jsonrpcResponse),
		diagnostics: make(map[string][]Diagnostic),
	}, nil
}

// Start launches the language server process
func (c *Client) Start(ctx context.Context) error {
	// Get language server executable and args
	executable := getLanguageServerExecutable(c.language)
	args := getLanguageServerArgs(c.language)

	// Create command
	c.cmd = exec.CommandContext(ctx, executable, args...)

	// Setup pipes
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start language server process
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start language server: %w", err)
	}

	c.running.Store(true)

	// Start message listener goroutine
	go c.listen()

	return nil
}

// Stop gracefully shuts down the language server
func (c *Client) Stop() error {
	if !c.running.Load() {
		return nil
	}

	// Send shutdown request
	_, err := c.SendRequest(context.Background(), "shutdown", nil)
	if err != nil {
		// Best effort - continue with exit
	}

	// Send exit notification
	_ = c.sendNotification("exit", nil)

	// Wait for process to exit (with timeout)
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}

	c.running.Store(false)
	return nil
}

// IsRunning returns whether the language server is running
func (c *Client) IsRunning() bool {
	return c.running.Load()
}

// Initialize sends the initialize request to the language server
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	if c.initialized {
		return nil // Already initialized
	}

	c.rootURI = rootURI

	// Build initialize params with language-specific options
	params := InitializeParams{
		ProcessID:             -1, // Use -1 for no parent process
		RootURI:               rootURI,
		Capabilities:          c.getClientCapabilities(),
		InitializationOptions: c.getInitializationOptions(),
	}

	// Send initialize request
	result, err := c.SendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// Parse server capabilities
	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Send initialized notification
	if err := c.sendNotification("initialized", struct{}{}); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	c.initialized = true
	return nil
}

// getClientCapabilities returns language-specific client capabilities
func (c *Client) getClientCapabilities() ClientCapabilities {
	// Base capabilities that all clients support
	return ClientCapabilities{
		TextDocument: TextDocumentClientCapabilities{
			PublishDiagnostics: PublishDiagnosticsClientCapabilities{
				RelatedInformation: true,
			},
		},
	}
}

// getInitializationOptions returns language-specific initialization options
// These customize the language server's behavior for optimal performance
func (c *Client) getInitializationOptions() interface{} {
	switch c.language {
	case "python":
		// Pyright/Pylance configuration
		return map[string]interface{}{
			"python": map[string]interface{}{
				"analysis": map[string]interface{}{
					"typeCheckingMode": "basic", // basic, standard, or strict
					"diagnosticMode":   "openFilesOnly",
					"autoSearchPaths":  true,
				},
			},
		}

	case "javascript", "typescript":
		// TypeScript language server configuration
		return map[string]interface{}{
			"preferences": map[string]interface{}{
				"includeInlayParameterNameHints":         "all",
				"includeInlayFunctionParameterTypeHints": true,
			},
		}

	case "rust":
		// rust-analyzer configuration
		return map[string]interface{}{
			"checkOnSave": map[string]interface{}{
				"command": "clippy", // Use clippy for additional lints
			},
			"cargo": map[string]interface{}{
				"features": "all", // Enable all cargo features
			},
		}

	case "go":
		// gopls configuration (use defaults, they're already good)
		return nil

	default:
		return nil
	}
}

// OpenDocument notifies the server that a document has been opened
func (c *Client) OpenDocument(ctx context.Context, uri, languageID, text string) error {
	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}

	return c.sendNotification("textDocument/didOpen", params)
}

// CloseDocument notifies the server that a document has been closed
func (c *Client) CloseDocument(ctx context.Context, uri string) error {
	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	return c.sendNotification("textDocument/didClose", params)
}

// UpdateDocument notifies the server of document changes
func (c *Client) UpdateDocument(ctx context.Context, uri string, version int, text string) error {
	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
			Version:                version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: text},
		},
	}

	return c.sendNotification("textDocument/didChange", params)
}

// GoToDefinition requests the definition location of a symbol
func (c *Client) GoToDefinition(ctx context.Context, uri string, line, character int) ([]Location, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.SendRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse definition result: %w", err)
	}

	return locations, nil
}

// FindReferences requests all references to a symbol
func (c *Client) FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]Location, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
		Context: ReferenceContext{
			IncludeDeclaration: includeDeclaration,
		},
	}

	result, err := c.SendRequest(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse references result: %w", err)
	}

	return locations, nil
}

// GetHover requests hover information at a position
func (c *Client) GetHover(ctx context.Context, uri string, line, character int) (*Hover, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.SendRequest(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, fmt.Errorf("failed to parse hover result: %w", err)
	}

	return &hover, nil
}

// GetDocumentSymbols requests all symbols in a document
func (c *Client) GetDocumentSymbols(ctx context.Context, uri string) ([]DocumentSymbol, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	result, err := c.SendRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []DocumentSymbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse document symbols: %w", err)
	}

	return symbols, nil
}

// === Phase 2: Advanced LSP Methods ===

// FindImplementations requests all implementations of an interface/type
func (c *Client) FindImplementations(ctx context.Context, uri string, line, character int) ([]Location, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := ImplementationParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.SendRequest(ctx, "textDocument/implementation", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse implementation result: %w", err)
	}

	return locations, nil
}

// GetTypeDefinition requests the type definition of a variable
func (c *Client) GetTypeDefinition(ctx context.Context, uri string, line, character int) ([]Location, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := TypeDefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}

	result, err := c.SendRequest(ctx, "textDocument/typeDefinition", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse type definition result: %w", err)
	}

	return locations, nil
}

// GetWorkspaceSymbols requests symbols across the entire workspace
func (c *Client) GetWorkspaceSymbols(ctx context.Context, query string) ([]SymbolInformation, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := WorkspaceSymbolParams{
		Query: query,
	}

	result, err := c.SendRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []SymbolInformation
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse workspace symbols: %w", err)
	}

	return symbols, nil
}

// GetDiagnostics returns stored diagnostics for a file
func (c *Client) GetDiagnostics(uri string) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	// Return copy to avoid race conditions
	diagnostics := c.diagnostics[uri]
	result := make([]Diagnostic, len(diagnostics))
	copy(result, diagnostics)
	return result
}

// ClearDiagnostics clears diagnostics for a file
func (c *Client) ClearDiagnostics(uri string) {
	c.diagnosticsMu.Lock()
	defer c.diagnosticsMu.Unlock()
	delete(c.diagnostics, uri)
}

// SendRequest sends a request and waits for the response
func (c *Client) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if !c.running.Load() {
		return nil, fmt.Errorf("language server not running")
	}

	// Generate request ID
	id := c.nextID.Add(1)

	// Create response channel
	respChan := make(chan *jsonrpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = respChan
	c.mu.Unlock()

	// Cleanup on exit
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	// Build request
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Send request
	if err := c.sendMessage(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error [%d]: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendNotification sends a notification (no response expected)
func (c *Client) sendNotification(method string, params interface{}) error {
	if !c.running.Load() {
		return fmt.Errorf("language server not running")
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	return c.sendMessage(req)
}

// sendMessage sends a JSON-RPC message to the language server
func (c *Client) sendMessage(message interface{}) error {
	// Marshal to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// LSP uses Content-Length header
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	// Write header + body
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}

	return nil
}

// listen reads messages from the language server
func (c *Client) listen() {
	scanner := newLSPScanner(c.stdout)

	for c.running.Load() {
		// Read next message
		msg, err := scanner.ReadMessage()
		if err != nil {
			if err != io.EOF {
				fmt.Printf("[LSP] Read error: %v\n", err)
			}
			break
		}

		// Try to parse as response first (has ID)
		var resp jsonrpcResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			fmt.Printf("[LSP] Failed to parse message: %v\n", err)
			continue
		}

		// Check if this is a notification (no ID) or a response (has ID)
		if resp.ID == 0 {
			// This is a notification - check for method field
			var notification struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(msg, &notification); err == nil {
				c.handleNotification(notification.Method, notification.Params)
			}
		} else {
			// This is a response - deliver to pending request
			c.mu.RLock()
			ch, exists := c.pending[resp.ID]
			c.mu.RUnlock()

			if exists {
				ch <- &resp
			}
		}
	}
}

// handleNotification processes LSP notifications
func (c *Client) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		var diagParams PublishDiagnosticsParams
		if err := json.Unmarshal(params, &diagParams); err != nil {
			fmt.Printf("[LSP] Failed to parse diagnostics: %v\n", err)
			return
		}

		// Store diagnostics
		c.diagnosticsMu.Lock()
		c.diagnostics[diagParams.URI] = diagParams.Diagnostics
		c.diagnosticsMu.Unlock()

		// Log diagnostic count
		errorCount := 0
		warningCount := 0
		for _, diag := range diagParams.Diagnostics {
			if diag.Severity == SeverityError {
				errorCount++
			} else if diag.Severity == SeverityWarning {
				warningCount++
			}
		}
		if errorCount > 0 || warningCount > 0 {
			fmt.Printf("[LSP] Diagnostics for %s: %d errors, %d warnings\n",
				diagParams.URI, errorCount, warningCount)
		}

	default:
		// Ignore other notifications for now
	}
}

// lspScanner reads LSP messages with Content-Length headers
type lspScanner struct {
	reader *bufio.Reader
}

func newLSPScanner(r io.Reader) *lspScanner {
	return &lspScanner{reader: bufio.NewReader(r)}
}

func (s *lspScanner) ReadMessage() ([]byte, error) {
	// Read headers until blank line
	var contentLength int
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		// Trim \r\n
		line = line[:len(line)-1]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		// Blank line means end of headers
		if line == "" {
			break
		}

		// Parse Content-Length header
		if len(line) > 16 && line[:16] == "Content-Length: " {
			fmt.Sscanf(line[16:], "%d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, err
	}

	return body, nil
}

// ServerConfig represents language server configuration with fallback support
type ServerConfig struct {
	Primary   string   // Primary executable to try
	Fallbacks []string // Fallback executables if primary not found
	Args      []string // Command-line arguments
}

// languageServers maps languages to their server configurations
var languageServers = map[string]ServerConfig{
	"go": {
		Primary:   "gopls",
		Fallbacks: []string{},
		Args:      []string{},
	},
	"python": {
		Primary:   "pyright-langserver",
		Fallbacks: []string{"pylsp"}, // Python LSP Server as fallback
		Args:      []string{"--stdio"},
	},
	"javascript": {
		Primary:   "typescript-language-server",
		Fallbacks: []string{},
		Args:      []string{"--stdio"},
	},
	"typescript": {
		Primary:   "typescript-language-server",
		Fallbacks: []string{},
		Args:      []string{"--stdio"},
	},
	"rust": {
		Primary:   "rust-analyzer",
		Fallbacks: []string{},
		Args:      []string{},
	},
}

// getLanguageServerExecutable finds the first available language server executable
// Tries primary first, then falls back to alternatives
func getLanguageServerExecutable(language string) string {
	config, ok := languageServers[language]
	if !ok {
		return "" // Unsupported language
	}

	// Try primary first
	if execPath, err := exec.LookPath(config.Primary); err == nil {
		return execPath
	}

	// Try fallbacks
	for _, fallback := range config.Fallbacks {
		if execPath, err := exec.LookPath(fallback); err == nil {
			fmt.Printf("[LSP] Using fallback %s for %s (primary %s not found)\n",
				fallback, language, config.Primary)
			return execPath
		}
	}

	// None found
	return ""
}

// getLanguageServerArgs returns command-line arguments for a language server
func getLanguageServerArgs(language string) []string {
	if config, ok := languageServers[language]; ok {
		return config.Args
	}
	return []string{}
}

// ValidateLanguageServer checks if a language server is available for the given language
func ValidateLanguageServer(language string) error {
	executable := getLanguageServerExecutable(language)
	if executable == "" {
		config, ok := languageServers[language]
		if !ok {
			return fmt.Errorf("language %s is not supported", language)
		}

		// Build helpful error message listing what was tried
		tried := []string{config.Primary}
		tried = append(tried, config.Fallbacks...)

		return fmt.Errorf("language server for %s not found (tried: %v)", language, tried)
	}
	return nil
}
