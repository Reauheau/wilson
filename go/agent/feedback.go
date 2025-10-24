package agent

import (
	"context"
	"database/sql"
	"encoding/json"
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
	db           *sql.DB // For persistence (optional)
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

// SendAndWait sends feedback and waits for it to be processed (synchronous)
// Returns the handler's error result
func (fb *FeedbackBus) SendAndWait(ctx context.Context, feedback *AgentFeedback) error {
	feedback.CreatedAt = time.Now()

	// Get handler immediately (before sending to channel)
	fb.mu.RLock()
	handler, exists := fb.handlers[feedback.FeedbackType]
	fb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler for feedback type: %s", feedback.FeedbackType)
	}

	// Persist feedback (still async)
	if fb.db != nil {
		go func() {
			if err := fb.persistFeedback(feedback); err != nil {
				fmt.Printf("[FeedbackBus] Failed to persist feedback: %v\n", err)
			}
		}()
	}

	// Execute handler synchronously
	handlerErr := handler(ctx, feedback)

	// Update processed status
	if fb.db != nil {
		fb.updateFeedbackProcessed(feedback, handlerErr)
	}

	return handlerErr
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

// SetDatabase sets the database for persistence (optional)
func (fb *FeedbackBus) SetDatabase(db *sql.DB) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.db = db
}

// processFeedback routes feedback to appropriate handler
func (fb *FeedbackBus) processFeedback(ctx context.Context, feedback *AgentFeedback) {
	// Persist feedback first (async, non-blocking)
	if fb.db != nil {
		go func() {
			if err := fb.persistFeedback(feedback); err != nil {
				fmt.Printf("[FeedbackBus] Failed to persist feedback: %v\n", err)
			}
		}()
	}

	fb.mu.RLock()
	handler, exists := fb.handlers[feedback.FeedbackType]
	fb.mu.RUnlock()

	if !exists {
		fmt.Printf("[FeedbackBus] No handler for type: %s\n", feedback.FeedbackType)
		return
	}

	// Process async to avoid blocking channel
	go func() {
		handlerErr := handler(ctx, feedback)

		// Update processed_at and result in database
		if fb.db != nil {
			fb.updateFeedbackProcessed(feedback, handlerErr)
		}

		if handlerErr != nil {
			fmt.Printf("[FeedbackBus] Handler error for %s: %v\n", feedback.FeedbackType, handlerErr)
		}
	}()
}

// persistFeedback stores feedback in the database
func (fb *FeedbackBus) persistFeedback(feedback *AgentFeedback) error {
	if fb.db == nil {
		return nil // No database configured
	}

	// Serialize Context map to JSON
	contextJSON, err := json.Marshal(feedback.Context)
	if err != nil {
		contextJSON = []byte("{}")
	}

	// Serialize TaskContext to JSON (optional, can be large)
	var taskContextJSON []byte
	if feedback.TaskContext != nil {
		taskContextJSON, err = json.Marshal(feedback.TaskContext)
		if err != nil {
			taskContextJSON = nil // Skip if serialization fails
		}
	}

	query := `
		INSERT INTO agent_feedback (
			task_id, agent_name, feedback_type, severity,
			message, context, suggestion, task_context, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = fb.db.Exec(
		query,
		feedback.TaskID,
		feedback.AgentName,
		string(feedback.FeedbackType),
		string(feedback.Severity),
		feedback.Message,
		string(contextJSON),
		feedback.Suggestion,
		string(taskContextJSON),
		feedback.CreatedAt,
	)

	return err
}

// updateFeedbackProcessed updates the feedback record with processing results
func (fb *FeedbackBus) updateFeedbackProcessed(feedback *AgentFeedback, handlerErr error) {
	if fb.db == nil {
		return
	}

	handlerResult := "success"
	handlerError := ""
	if handlerErr != nil {
		handlerResult = "error"
		handlerError = handlerErr.Error()
	}

	query := `
		UPDATE agent_feedback
		SET processed_at = ?, handler_result = ?, handler_error = ?
		WHERE task_id = ? AND agent_name = ? AND created_at = ?
		AND processed_at IS NULL
	`

	_, err := fb.db.Exec(
		query,
		time.Now(),
		handlerResult,
		handlerError,
		feedback.TaskID,
		feedback.AgentName,
		feedback.CreatedAt,
	)

	if err != nil {
		fmt.Printf("[FeedbackBus] Failed to update feedback processing status: %v\n", err)
	}
}

// GetFeedbackForTask retrieves all feedback for a specific task
func (fb *FeedbackBus) GetFeedbackForTask(taskID string) ([]*AgentFeedback, error) {
	if fb.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	query := `
		SELECT id, task_id, agent_name, feedback_type, severity,
		       message, context, suggestion, created_at, processed_at,
		       handler_result, handler_error
		FROM agent_feedback
		WHERE task_id = ?
		ORDER BY created_at ASC
	`

	rows, err := fb.db.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedbacks []*AgentFeedback
	for rows.Next() {
		var (
			id            int
			taskID        string
			agentName     string
			feedbackType  string
			severity      string
			message       sql.NullString
			contextJSON   sql.NullString
			suggestion    sql.NullString
			createdAt     time.Time
			processedAt   sql.NullTime
			handlerResult sql.NullString
			handlerError  sql.NullString
		)

		err := rows.Scan(
			&id, &taskID, &agentName, &feedbackType, &severity,
			&message, &contextJSON, &suggestion, &createdAt, &processedAt,
			&handlerResult, &handlerError,
		)
		if err != nil {
			continue
		}

		feedback := &AgentFeedback{
			TaskID:       taskID,
			AgentName:    agentName,
			FeedbackType: FeedbackType(feedbackType),
			Severity:     FeedbackSeverity(severity),
			Message:      message.String,
			Suggestion:   suggestion.String,
			CreatedAt:    createdAt,
		}

		// Deserialize context if available
		if contextJSON.Valid && contextJSON.String != "" {
			var ctx map[string]interface{}
			if err := json.Unmarshal([]byte(contextJSON.String), &ctx); err == nil {
				feedback.Context = ctx
			}
		}

		feedbacks = append(feedbacks, feedback)
	}

	return feedbacks, nil
}

// FeedbackStats contains analytics about feedback
type FeedbackStats struct {
	TotalFeedback     int
	ByType            map[string]int
	BySeverity        map[string]int
	ProcessedCount    int
	ErrorCount        int
	AverageProcessing time.Duration // Average time from created to processed
}

// GetFeedbackStats retrieves statistics about feedback
func (fb *FeedbackBus) GetFeedbackStats(since time.Time) (*FeedbackStats, error) {
	if fb.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	stats := &FeedbackStats{
		ByType:     make(map[string]int),
		BySeverity: make(map[string]int),
	}

	// Get total count and counts by type/severity
	query := `
		SELECT
			COUNT(*) as total,
			feedback_type,
			severity,
			COUNT(CASE WHEN processed_at IS NOT NULL THEN 1 END) as processed,
			COUNT(CASE WHEN handler_result = 'error' THEN 1 END) as errors,
			AVG(CASE WHEN processed_at IS NOT NULL
				THEN (julianday(processed_at) - julianday(created_at)) * 86400
				END) as avg_processing_seconds
		FROM agent_feedback
		WHERE created_at >= ?
		GROUP BY feedback_type, severity
	`

	rows, err := fb.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalProcessingTime float64
	var processedCount int

	for rows.Next() {
		var (
			count                int
			feedbackType         string
			severity             string
			processed            int
			errors               int
			avgProcessingSeconds sql.NullFloat64
		)

		if err := rows.Scan(&count, &feedbackType, &severity, &processed, &errors, &avgProcessingSeconds); err != nil {
			continue
		}

		stats.TotalFeedback += count
		stats.ByType[feedbackType] += count
		stats.BySeverity[severity] += count
		stats.ProcessedCount += processed
		stats.ErrorCount += errors

		if avgProcessingSeconds.Valid && avgProcessingSeconds.Float64 > 0 {
			totalProcessingTime += avgProcessingSeconds.Float64 * float64(processed)
			processedCount += processed
		}
	}

	// Calculate average processing time
	if processedCount > 0 {
		stats.AverageProcessing = time.Duration(totalProcessingTime/float64(processedCount)) * time.Second
	}

	return stats, nil
}

// GetRecentFeedback retrieves the most recent feedback entries
func (fb *FeedbackBus) GetRecentFeedback(limit int) ([]*AgentFeedback, error) {
	if fb.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	query := `
		SELECT task_id, agent_name, feedback_type, severity,
		       message, suggestion, created_at
		FROM agent_feedback
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := fb.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedbacks []*AgentFeedback
	for rows.Next() {
		var (
			taskID       string
			agentName    string
			feedbackType string
			severity     string
			message      sql.NullString
			suggestion   sql.NullString
			createdAt    time.Time
		)

		if err := rows.Scan(&taskID, &agentName, &feedbackType, &severity, &message, &suggestion, &createdAt); err != nil {
			continue
		}

		feedbacks = append(feedbacks, &AgentFeedback{
			TaskID:       taskID,
			AgentName:    agentName,
			FeedbackType: FeedbackType(feedbackType),
			Severity:     FeedbackSeverity(severity),
			Message:      message.String,
			Suggestion:   suggestion.String,
			CreatedAt:    createdAt,
		})
	}

	return feedbacks, nil
}
