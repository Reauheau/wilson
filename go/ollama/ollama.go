package ollama

import (
	"context"
	"fmt"
)

// Package-level client instance
var ollamaClient *Client

func Init(ctx context.Context, model string) error {
	var err error
	ollamaClient, err = NewClient(model)
	if err != nil {
		return fmt.Errorf("Ollama Init failed: %w", err)
	}
	return ollamaClient.Ping(ctx)
}

func Shutdown() {
	// No persistent resources, but future-proof for background workers, etc.
	if ollamaClient != nil {
		ollamaClient.Close()
	}
}

func AskOllama(ctx context.Context, prompt string, handler func(string)) error {
	if ollamaClient == nil {
		return fmt.Errorf("Ollama client not initialized")
	}
	return ollamaClient.Ask(ctx, prompt, handler)
}

func AskOllamaWithSystem(ctx context.Context, prompt string, system string, handler func(string)) error {
	if ollamaClient == nil {
		return fmt.Errorf("Ollama client not initialized")
	}
	return ollamaClient.AskWithSystem(ctx, prompt, system, handler)
}

func AskOllamaWithMessages(ctx context.Context, messages []Message, handler func(string)) error {
	if ollamaClient == nil {
		return fmt.Errorf("Ollama client not initialized")
	}
	return ollamaClient.AskWithMessages(ctx, messages, handler)
}
