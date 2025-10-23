package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FeedbackType categorizes agent feedback
type FeedbackType string

const (
	FeedbackTypeDependencyNeeded FeedbackType = "dependency_needed"
	FeedbackTypeBlocker          FeedbackType = "blocker"
	FeedbackTypeContextNeeded    FeedbackType = "context_needed"
	FeedbackTypeRetryRequest     FeedbackType = "retry_request"
	FeedbackTypeSuccess          FeedbackType = "success"
)

// FeedbackSeverity indicates urgency
type FeedbackSeverity string

const (
	FeedbackSeverityInfo     FeedbackSeverity = "info"
	FeedbackSeverityWarning  FeedbackSeverity = "warning"
	FeedbackSeverityCritical FeedbackSeverity = "critical"
)

// AgentFeedback represents feedback from an agent with full TaskContext
type AgentFeedback struct {
	TaskID       string                 `json:"task_id"`
	AgentName    string                 `json:"agent_name"`
	FeedbackType FeedbackType           `json:"feedback_type"`
	Severity     FeedbackSeverity       `json:"severity"`
	Message      string                 `json:"message"`
	Context      map[string]interface{} `json:"context"` // Additional context
	Suggestion   string                 `json:"suggestion"`
	TaskContext  *TaskContext           `json:"task_context"` // ✅ NEW: Full execution context
	CreatedAt    time.Time              `json:"created_at"`
}

// FeedbackBus manages event-driven feedback with TaskContext awareness
type FeedbackBus struct {
	feedbackChan chan *AgentFeedback
	mu           sync.RWMutex
	handlers     map[FeedbackType]FeedbackHandler
}

// FeedbackHandler processes specific feedback types
type FeedbackHandler func(context.Context, *AgentFeedback) error

// Global feedback bus (singleton)
var (
	globalFeedbackBus     *FeedbackBus
	globalFeedbackBusOnce sync.Once
)

// GetFeedbackBus returns the global feedback bus
func GetFeedbackBus() *FeedbackBus {
	globalFeedbackBusOnce.Do(func() {
		globalFeedbackBus = &FeedbackBus{
			feedbackChan: make(chan *AgentFeedback, 100), // Buffered
			handlers:     make(map[FeedbackType]FeedbackHandler),
		}
	})
	return globalFeedbackBus
}

// Send sends feedback (non-blocking with timeout)
func (fb *FeedbackBus) Send(feedback *AgentFeedback) error {
	feedback.CreatedAt = time.Now()

	select {
	case fb.feedbackChan <- feedback:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("feedback bus timeout")
	}
}

// RegisterHandler registers a handler for a feedback type
func (fb *FeedbackBus) RegisterHandler(feedbackType FeedbackType, handler FeedbackHandler) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.handlers[feedbackType] = handler
}

// Start begins processing feedback (runs in goroutine)
func (fb *FeedbackBus) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case feedback := <-fb.feedbackChan:
				fb.processFeedback(ctx, feedback)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// processFeedback routes feedback to appropriate handler
func (fb *FeedbackBus) processFeedback(ctx context.Context, feedback *AgentFeedback) {
	fb.mu.RLock()
	handler, exists := fb.handlers[feedback.FeedbackType]
	fb.mu.RUnlock()

	if !exists {
		fmt.Printf("[FeedbackBus] No handler for type: %s\n", feedback.FeedbackType)
		return
	}

	// Process async to avoid blocking channel
	go func() {
		if err := handler(ctx, feedback); err != nil {
			fmt.Printf("[FeedbackBus] Handler error for %s: %v\n", feedback.FeedbackType, err)
		}
	}()
}
