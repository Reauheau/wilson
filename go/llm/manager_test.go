package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient is a mock LLM client for testing
type mockClient struct {
	model        string
	provider     string
	available    bool
	generateFunc func(ctx context.Context, req Request) (*Response, error)
}

func (m *mockClient) Generate(ctx context.Context, req Request) (*Response, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}
	return &Response{
		Content:    "mock response from " + m.model,
		Model:      m.model,
		TokensUsed: 10,
	}, nil
}

func (m *mockClient) GetModel() string {
	return m.model
}

func (m *mockClient) GetProvider() string {
	return m.provider
}

func (m *mockClient) IsAvailable(ctx context.Context) bool {
	return m.available
}

func TestNewManager(t *testing.T) {
	manager := NewManager()

	require.NotNil(t, manager)
	assert.NotNil(t, manager.clients)
	assert.NotNil(t, manager.configs)
}

func TestRegisterLLMInvalidProvider(t *testing.T) {
	manager := NewManager()

	config := Config{
		Provider: "nonexistent",
		Model:    "test-model",
	}

	err := manager.RegisterLLM(PurposeChat, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestGetClient(t *testing.T) {
	manager := NewManager()

	// Manually add a mock client
	mockChatClient := &mockClient{
		model:     "chat-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeChat] = mockChatClient

	// Get the client
	client, err := manager.GetClient(PurposeChat)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "chat-model", client.GetModel())
}

func TestGetClientNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetClient(PurposeAnalysis)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM available")
}

func TestGetClientWithFallback(t *testing.T) {
	manager := NewManager()

	// Manually add chat client
	mockChatClient := &mockClient{
		model:     "chat-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeChat] = mockChatClient

	// Try to get analysis client (not registered)
	// Should fall back to chat
	client, err := manager.GetClient(PurposeAnalysis)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "chat-model", client.GetModel(), "Should fallback to chat LLM")
}

func TestGenerate(t *testing.T) {
	manager := NewManager()

	// Add mock client with custom generate function
	mockChatClient := &mockClient{
		model:     "chat-model",
		provider:  "mock",
		available: true,
		generateFunc: func(ctx context.Context, req Request) (*Response, error) {
			return &Response{
				Content:    "Test response",
				Model:      "chat-model",
				TokensUsed: 15,
			}, nil
		},
	}
	manager.clients[PurposeChat] = mockChatClient

	// Call Generate
	ctx := context.Background()
	req := Request{
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := manager.Generate(ctx, PurposeChat, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Test response", resp.Content)
	assert.Equal(t, "chat-model", resp.Model)
	assert.Equal(t, 15, resp.TokensUsed)
}

func TestGenerateNotRegistered(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	req := Request{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := manager.Generate(ctx, PurposeAnalysis, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM available")
}

func TestGenerateWithFallback(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Add chat client
	mockChatClient := &mockClient{
		model:     "chat-model",
		provider:  "mock",
		available: true,
		generateFunc: func(ctx context.Context, req Request) (*Response, error) {
			return &Response{
				Content:    "Fallback response",
				Model:      "chat-model",
				TokensUsed: 10,
			}, nil
		},
	}
	manager.clients[PurposeChat] = mockChatClient

	// Try to generate with analysis (not registered)
	// Should fall back to chat
	req := Request{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := manager.Generate(ctx, PurposeAnalysis, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Fallback response", resp.Content)
}

func TestGenerateUnavailableClient(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Add unavailable client
	mockClient := &mockClient{
		model:     "analysis-model",
		provider:  "mock",
		available: false, // Not available
	}
	manager.clients[PurposeAnalysis] = mockClient

	req := Request{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Should fail - no fallback available
	_, err := manager.Generate(ctx, PurposeAnalysis, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestListAvailable(t *testing.T) {
	manager := NewManager()

	// Add multiple clients
	manager.clients[PurposeChat] = &mockClient{
		model:     "chat-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeAnalysis] = &mockClient{
		model:     "analysis-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeCode] = &mockClient{
		model:     "code-model",
		provider:  "mock",
		available: false, // Not available
	}

	// List all available
	available := manager.ListAvailable()
	assert.Len(t, available, 2, "Should only list available clients")

	// Verify chat and analysis are present, code is not
	hasChat := false
	hasAnalysis := false
	hasCode := false
	for _, purpose := range available {
		if purpose == PurposeChat {
			hasChat = true
		}
		if purpose == PurposeAnalysis {
			hasAnalysis = true
		}
		if purpose == PurposeCode {
			hasCode = true
		}
	}

	assert.True(t, hasChat, "Chat should be available")
	assert.True(t, hasAnalysis, "Analysis should be available")
	assert.False(t, hasCode, "Code should not be available")
}

func TestListAvailableEmpty(t *testing.T) {
	manager := NewManager()

	available := manager.ListAvailable()
	assert.Len(t, available, 0)
}

func TestIsAvailable(t *testing.T) {
	manager := NewManager()

	// Add available client
	manager.clients[PurposeChat] = &mockClient{
		model:     "chat-model",
		provider:  "mock",
		available: true,
	}

	// Add unavailable client
	manager.clients[PurposeAnalysis] = &mockClient{
		model:     "analysis-model",
		provider:  "mock",
		available: false,
	}

	assert.True(t, manager.IsAvailable(PurposeChat))
	assert.False(t, manager.IsAvailable(PurposeAnalysis))
	assert.False(t, manager.IsAvailable(PurposeCode)) // Not registered
}

func TestGetConfig(t *testing.T) {
	manager := NewManager()

	// Add config
	testConfig := Config{
		Provider:    "ollama",
		Model:       "test-model",
		Temperature: 0.7,
	}
	manager.configs[PurposeChat] = testConfig

	// Get config
	config, ok := manager.GetConfig(PurposeChat)
	assert.True(t, ok)
	assert.Equal(t, "ollama", config.Provider)
	assert.Equal(t, "test-model", config.Model)
	assert.Equal(t, 0.7, config.Temperature)

	// Get non-existent config
	_, ok = manager.GetConfig(PurposeAnalysis)
	assert.False(t, ok)
}

func TestConcurrentGetClient(t *testing.T) {
	manager := NewManager()

	// Add mock client
	manager.clients[PurposeChat] = &mockClient{
		model:     "test-model",
		provider:  "mock",
		available: true,
	}

	// Get client concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			client, err := manager.GetClient(PurposeChat)
			assert.NoError(t, err)
			assert.NotNil(t, client)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrentGenerate(t *testing.T) {
	manager := NewManager()

	// Add mock client
	manager.clients[PurposeChat] = &mockClient{
		model:     "test-model",
		provider:  "mock",
		available: true,
		generateFunc: func(ctx context.Context, req Request) (*Response, error) {
			return &Response{
				Content:    "Response",
				Model:      "test-model",
				TokensUsed: 10,
			}, nil
		},
	}

	// Call concurrently
	ctx := context.Background()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			req := Request{
				Messages: []Message{
					{Role: "user", Content: "Test"},
				},
			}
			resp, err := manager.Generate(ctx, PurposeChat, req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPurposeConstants(t *testing.T) {
	assert.Equal(t, "chat", string(PurposeChat))
	assert.Equal(t, "analysis", string(PurposeAnalysis))
	assert.Equal(t, "code", string(PurposeCode))
	assert.Equal(t, "vision", string(PurposeVision))
}

// TestModelLifecycleBasic tests basic model instance lifecycle with reference counting
func TestModelLifecycleBasic(t *testing.T) {
	manager := NewManager()
	defer manager.Stop()

	// Manually add a mock client and config
	mockClient := &mockClient{
		model:     "test-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeCode] = mockClient
	manager.configs[PurposeCode] = Config{
		Provider:    "mock",
		Model:       "test-model",
		KeepAlive:   false,
		IdleTimeout: 0, // Immediate unload
	}

	// Initially, model should not be loaded
	assert.False(t, manager.IsLoaded(PurposeCode), "Model should not be loaded initially")

	// Acquire model - this should create instance
	client, release, _, err := manager.AcquireModel(PurposeCode)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Model should now be loaded
	assert.True(t, manager.IsLoaded(PurposeCode), "Model should be loaded after acquisition")

	// RefCount should be 1
	assert.Equal(t, 1, manager.GetRefCount(PurposeCode), "RefCount should be 1")

	// Release the model
	release()

	// RefCount should be 0
	assert.Equal(t, 0, manager.GetRefCount(PurposeCode), "RefCount should be 0 after release")

	// With IdleTimeout=0, model should be immediately unloaded
	assert.False(t, manager.IsLoaded(PurposeCode), "Model should be unloaded immediately when IdleTimeout=0")
}

// TestModelReferenceCountingMultiple tests multiple concurrent acquisitions
func TestModelReferenceCountingMultiple(t *testing.T) {
	manager := NewManager()
	defer manager.Stop()

	// Manually add mock client and config
	mockClient := &mockClient{
		model:     "test-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeCode] = mockClient
	manager.configs[PurposeCode] = Config{
		Provider:    "mock",
		Model:       "test-model",
		KeepAlive:   false,
		IdleTimeout: 2, // 2 second timeout for testing
	}

	// Acquire model twice
	client1, release1, _, err := manager.AcquireModel(PurposeCode)
	require.NoError(t, err)
	require.NotNil(t, client1)

	client2, release2, _, err := manager.AcquireModel(PurposeCode)
	require.NoError(t, err)
	require.NotNil(t, client2)

	// Both clients should be the same instance
	assert.Equal(t, client1, client2, "Both acquisitions should return same client instance")

	// RefCount should be 2
	assert.Equal(t, 2, manager.GetRefCount(PurposeCode), "RefCount should be 2")

	// Release first
	release1()

	// RefCount should be 1
	assert.Equal(t, 1, manager.GetRefCount(PurposeCode), "RefCount should be 1 after first release")

	// Model should still be loaded
	assert.True(t, manager.IsLoaded(PurposeCode), "Model should still be loaded with refCount=1")

	// Release second
	release2()

	// RefCount should be 0
	assert.Equal(t, 0, manager.GetRefCount(PurposeCode), "RefCount should be 0 after all releases")

	// Model should still be loaded (idle timeout not elapsed)
	assert.True(t, manager.IsLoaded(PurposeCode), "Model should still be loaded before idle timeout")
}

// TestModelKeepAlive tests that KeepAlive prevents unloading
func TestModelKeepAlive(t *testing.T) {
	manager := NewManager()
	defer manager.Stop()

	// Manually add mock client and config with KeepAlive
	mockClient := &mockClient{
		model:     "test-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeChat] = mockClient
	manager.configs[PurposeChat] = Config{
		Provider:    "mock",
		Model:       "test-model",
		KeepAlive:   true, // Never unload
		IdleTimeout: 1,    // Short timeout (should be ignored)
	}

	// Acquire and release
	client, release, _, err := manager.AcquireModel(PurposeChat)
	require.NoError(t, err)
	require.NotNil(t, client)
	release()

	// RefCount should be 0
	assert.Equal(t, 0, manager.GetRefCount(PurposeChat), "RefCount should be 0")

	// Model should still be loaded (KeepAlive=true)
	assert.True(t, manager.IsLoaded(PurposeChat), "Model with KeepAlive=true should never be unloaded")
}

// TestManagerStop tests graceful shutdown
func TestManagerStop(t *testing.T) {
	manager := NewManager()

	// Manually add mock client and config
	mockClient := &mockClient{
		model:     "test-model",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeCode] = mockClient
	manager.configs[PurposeCode] = Config{
		Provider:    "mock",
		Model:       "test-model",
		KeepAlive:   false,
		IdleTimeout: 10,
	}

	// Acquire model with timeout
	_, release, _, err := manager.AcquireModel(PurposeCode)
	require.NoError(t, err)
	release()

	// Stop manager
	manager.Stop()

	// Context should be cancelled
	select {
	case <-manager.ctx.Done():
		// Expected
	default:
		t.Error("Manager context should be cancelled after Stop()")
	}
}

// TestAcquireModelFallback tests fallback to chat model
func TestAcquireModelFallback(t *testing.T) {
	manager := NewManager()
	defer manager.Stop()

	// Register only chat model
	mockChatClient := &mockClient{
		model:     "llama3",
		provider:  "mock",
		available: true,
	}
	manager.clients[PurposeChat] = mockChatClient
	manager.configs[PurposeChat] = Config{
		Provider:  "mock",
		Model:     "llama3",
		KeepAlive: true,
	}

	// Try to acquire code model (not registered)
	// Should fallback to chat
	client, release, _, err := manager.AcquireModel(PurposeCode)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer release()

	// Verify it's the chat client
	assert.Equal(t, "llama3", client.GetModel(), "Should fallback to chat model")
}
