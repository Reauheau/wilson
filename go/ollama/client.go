package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	model string
	url   string
	http  *http.Client
}

func NewClient(model string) (*Client, error) {
	return &Client{
		model: model,
		url:   "http://localhost:11434",
		http:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("could not connect to Ollama server")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama server returned %d", resp.StatusCode)
	}
	var tags struct{ Models []struct{ Name string } }
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return err
	}
	for _, m := range tags.Models {
		if m.Name == c.model {
			return nil
		}
	}
	return fmt.Errorf("model '%s' not found locally; run 'ollama pull %s'", c.model, c.model)
}

func (c *Client) Ask(ctx context.Context, prompt string, handler func(string)) error {
	return c.AskWithSystem(ctx, prompt, "", handler)
}

// Message represents a chat message with role and content
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AskWithMessages sends a chat request with message history
func (c *Client) AskWithMessages(ctx context.Context, messages []Message, handler func(string)) error {
	reqBody := map[string]interface{}{
		"model":    c.model,
		"messages": messages,
		"stream":   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a new client with no timeout for streaming
	httpClient := &http.Client{Timeout: 0}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned %d: %s", resp.StatusCode, b)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read stream: %w", err)
		}

		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}

		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			handler(chunk.Message.Content)
		}

		if chunk.Done {
			break
		}
	}

	return nil
}

func (c *Client) AskWithSystem(ctx context.Context, prompt string, system string, handler func(string)) error {
	reqBody := map[string]interface{}{
		"model":  c.model,
		"prompt": prompt,
		"stream": true,
	}

	// Add system prompt if provided
	if system != "" {
		reqBody["system"] = system
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a new client with no timeout for streaming
	httpClient := &http.Client{Timeout: 0}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned %d: %s", resp.StatusCode, b)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		var chunk struct {
			Response string `json:"response"`
		}
		if json.Unmarshal(line, &chunk) == nil {
			handler(chunk.Response)
		}
	}
	return nil
}

func (c *Client) Close() {
	// If background goroutines or websockets, clean up here
	// For stateless HTTP, nothing required
}
