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
		language: language,
		pending:  make(map[int64]chan *jsonrpcResponse),
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

	// Build initialize params
	params := InitializeParams{
		ProcessID: -1, // Use -1 for no parent process
		RootURI:   rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				PublishDiagnostics: PublishDiagnosticsClientCapabilities{
					RelatedInformation: true,
				},
			},
		},
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

		// Parse as JSON-RPC response
		var resp jsonrpcResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			fmt.Printf("[LSP] Failed to parse response: %v\n", err)
			continue
		}

		// Deliver to pending request
		c.mu.RLock()
		ch, exists := c.pending[resp.ID]
		c.mu.RUnlock()

		if exists {
			ch <- &resp
		}
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

// getLanguageServerExecutable returns the executable path for a language server
func getLanguageServerExecutable(language string) string {
	switch language {
	case "go":
		return "gopls"
	case "python":
		return "pyright-langserver"
	case "javascript", "typescript":
		return "typescript-language-server"
	case "rust":
		return "rust-analyzer"
	default:
		return ""
	}
}

// getLanguageServerArgs returns command-line arguments for a language server
func getLanguageServerArgs(language string) []string {
	switch language {
	case "go":
		return []string{} // gopls runs in stdio mode by default
	case "python":
		return []string{"--stdio"}
	case "javascript", "typescript":
		return []string{"--stdio"}
	case "rust":
		return []string{}
	default:
		return []string{}
	}
}
