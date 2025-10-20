package session

import (
	"fmt"
	"sync"
	"time"
)

// Message represents a single message in the conversation
type Message struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // When message was sent
}

// History maintains conversation history for a session
type History struct {
	messages []Message
	maxTurns int // Maximum number of turns to keep (0 = unlimited)
	mu       sync.RWMutex // Phase 4: Thread-safe for concurrent chat
}

// NewHistory creates a new conversation history
// maxTurns: maximum number of user-assistant exchanges to keep (e.g., 10 = 20 messages)
// Set to 0 for unlimited history (not recommended)
func NewHistory(maxTurns int) *History {
	return &History{
		messages: make([]Message, 0),
		maxTurns: maxTurns,
	}
}

// AddMessage adds a message to the conversation history
// role should be either "user" or "assistant"
func (h *History) AddMessage(role, content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})

	// Trim history if it exceeds max turns
	// Each turn = user + assistant message, so maxTurns * 2
	if h.maxTurns > 0 {
		maxMessages := h.maxTurns * 2
		if len(h.messages) > maxMessages {
			// Keep only the most recent messages
			h.messages = h.messages[len(h.messages)-maxMessages:]
		}
	}
}

// GetMessages returns all messages in the conversation history
func (h *History) GetMessages() []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.messages
}

// GetMessageCount returns the number of messages in history
func (h *History) GetMessageCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages)
}

// GetTurnCount returns the number of conversational turns (user+assistant pairs)
func (h *History) GetTurnCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages) / 2
}

// Clear removes all messages from the history
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = nil
}

// IsEmpty returns true if there are no messages in history
func (h *History) IsEmpty() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages) == 0
}

// GetLastUserMessage returns the most recent user message, or empty string if none
func (h *History) GetLastUserMessage() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.messages) - 1; i >= 0; i-- {
		if h.messages[i].Role == "user" {
			return h.messages[i].Content
		}
	}
	return ""
}

// GetLastAssistantMessage returns the most recent assistant message, or empty string if none
func (h *History) GetLastAssistantMessage() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.messages) - 1; i >= 0; i-- {
		if h.messages[i].Role == "assistant" {
			return h.messages[i].Content
		}
	}
	return ""
}

// GetSummary returns a brief summary of the conversation history
func (h *History) GetSummary() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.messages) == 0 {
		return "No conversation history"
	}

	turnCount := len(h.messages) / 2
	messageCount := len(h.messages)

	return fmt.Sprintf("%d turns (%d messages), started %s",
		turnCount, messageCount, h.messages[0].Timestamp.Format("15:04:05"))
}
