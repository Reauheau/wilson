package llm

import (
	"context"
	"fmt"
	"strings"

	"wilson/ollama"
)

// OllamaClient implements the Client interface for Ollama
type OllamaClient struct {
	client      *ollama.Client
	model       string
	temperature float64
	baseURL     string
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(config Config) (*OllamaClient, error) {
	// Create underlying ollama client
	client, err := ollama.NewClient(config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}

	return &OllamaClient{
		client:      client,
		model:       config.Model,
		temperature: config.Temperature,
		baseURL:     config.BaseURL,
	}, nil
}

// Generate sends a request to Ollama and returns the response
func (c *OllamaClient) Generate(ctx context.Context, req Request) (*Response, error) {
	// Build the prompt from messages
	var promptBuilder strings.Builder
	var systemPrompt string

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			systemPrompt = msg.Content
		case "user":
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		case "assistant":
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		}
	}

	prompt := strings.TrimSpace(promptBuilder.String())

	// Collect the response
	var responseContent strings.Builder
	handler := func(text string) {
		responseContent.WriteString(text)
	}

	// Call the existing ollama client
	err := c.client.AskWithSystem(ctx, prompt, systemPrompt, handler)
	if err != nil {
		return nil, fmt.Errorf("ollama generation error: %w", err)
	}

	// Create response
	response := &Response{
		Content: responseContent.String(),
		Model:   c.model,
		Metadata: map[string]any{
			"temperature": c.temperature,
		},
	}

	return response, nil
}

// GetModel returns the model name
func (c *OllamaClient) GetModel() string {
	return c.model
}

// GetProvider returns the provider name
func (c *OllamaClient) GetProvider() string {
	return "ollama"
}

// IsAvailable checks if Ollama is responding
func (c *OllamaClient) IsAvailable(ctx context.Context) bool {
	err := c.client.Ping(ctx)
	return err == nil
}
