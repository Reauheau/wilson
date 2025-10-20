package agent

import (
	"encoding/json"
	"time"
)

// ManagedTaskStatus represents the current state of a managed task
type ManagedTaskStatus string

const (
	ManagedTaskStatusNew        ManagedTaskStatus = "new"
	ManagedTaskStatusReady      ManagedTaskStatus = "ready"
	ManagedTaskStatusAssigned   ManagedTaskStatus = "assigned"
	ManagedTaskStatusInProgress ManagedTaskStatus = "in_progress"
	ManagedTaskStatusInReview   ManagedTaskStatus = "in_review"
	ManagedTaskStatusBlocked    ManagedTaskStatus = "blocked"
	ManagedTaskStatusDone       ManagedTaskStatus = "done"
	ManagedTaskStatusFailed     ManagedTaskStatus = "failed"
)

// ManagedTaskType categorizes the kind of work to be done
type ManagedTaskType string

const (
	ManagedTaskTypeResearch ManagedTaskType = "research"
	ManagedTaskTypeCode     ManagedTaskType = "code"
	ManagedTaskTypeTest     ManagedTaskType = "test"
	ManagedTaskTypeReview   ManagedTaskType = "review"
	ManagedTaskTypeAnalysis ManagedTaskType = "analysis"
	ManagedTaskTypeGeneral  ManagedTaskType = "general"
)

// ReviewStatus tracks the review state
type ReviewStatus string

const (
	ReviewStatusPending      ReviewStatus = "pending"
	ReviewStatusApproved     ReviewStatus = "approved"
	ReviewStatusNeedsChanges ReviewStatus = "needs_changes"
	ReviewStatusRejected     ReviewStatus = "rejected"
)

// ManagedTask represents a unit of work with full ENDGAME task management
// This is the enhanced version with DoR/DoD, dependencies, and review workflow
// The simple Task type in types.go is used for basic agent delegation
type ManagedTask struct {
	ID            int            `json:"id"`
	ParentTaskID  *int           `json:"parent_task_id,omitempty"`
	TaskKey       string         `json:"task_key"`        // e.g., TASK-001
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Type          ManagedTaskType   `json:"type"`

	// Assignment
	AssignedTo    string            `json:"assigned_to,omitempty"`
	AssignedAt    *time.Time        `json:"assigned_at,omitempty"`

	// Status
	Status        ManagedTaskStatus `json:"status"`
	Priority      int            `json:"priority"`        // Higher = more important

	// Definition of Ready/Done
	DORCriteria   []string       `json:"dor_criteria"`
	DORMet        bool           `json:"dor_met"`
	DODCriteria   []string       `json:"dod_criteria"`
	DODMet        bool           `json:"dod_met"`

	// Dependencies
	DependsOn     []string       `json:"depends_on"`      // Task keys this depends on
	Blocks        []string       `json:"blocks"`          // Task keys this blocks

	// Results
	Result        string         `json:"result,omitempty"`
	ArtifactIDs   []int          `json:"artifact_ids,omitempty"`

	// Timestamps
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`

	// Review
	ReviewStatus  ReviewStatus   `json:"review_status,omitempty"`
	ReviewComments string        `json:"review_comments,omitempty"`
	Reviewer      string         `json:"reviewer,omitempty"`

	// Metadata
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Subtask is an alias for ManagedTask but semantically represents a child task
type Subtask = ManagedTask

// TaskReview represents a review of a task's completion
type TaskReview struct {
	ID           int            `json:"id"`
	TaskID       int            `json:"task_id"`
	ReviewerAgent string        `json:"reviewer_agent"`
	ReviewType   string         `json:"review_type"` // code_review, security_review, quality_review
	Status       ReviewStatus   `json:"status"`
	Findings     []Finding      `json:"findings"`
	Comments     string         `json:"comments"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Finding represents an issue found during review
type Finding struct {
	Severity    string `json:"severity"` // critical, major, minor, info
	Category    string `json:"category"` // security, performance, style, etc.
	Description string `json:"description"`
	Location    string `json:"location,omitempty"` // file:line or artifact reference
}

// AgentCommunication represents a message between agents
type AgentCommunication struct {
	ID          int       `json:"id"`
	FromAgent   string    `json:"from_agent"`
	ToAgent     string    `json:"to_agent,omitempty"` // empty = broadcast
	MessageType string    `json:"message_type"`       // question, response, notification, handoff
	Content     string    `json:"content"`
	ContextRef  string    `json:"context_ref,omitempty"` // Task key or artifact ID
	CreatedAt   time.Time `json:"created_at"`
}

// NewManagedTask creates a new managed task with default values
func NewManagedTask(title, description string, taskType ManagedTaskType) *ManagedTask {
	return &ManagedTask{
		Title:       title,
		Description: description,
		Type:        taskType,
		Status:      ManagedTaskStatusNew,
		Priority:    0,
		DORCriteria: []string{},
		DODCriteria: []string{},
		DORMet:      false,
		DODMet:      false,
		DependsOn:   []string{},
		Blocks:      []string{},
		ArtifactIDs: []int{},
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
}

// IsReady checks if the task meets Definition of Ready
func (t *ManagedTask) IsReady() bool {
	return t.DORMet && len(t.DependsOn) == 0
}

// IsDone checks if the task meets Definition of Done
func (t *ManagedTask) IsDone() bool {
	return t.DODMet && t.Status == ManagedTaskStatusDone
}

// IsBlocked checks if the task is blocked by dependencies or issues
func (t *ManagedTask) IsBlocked() bool {
	return t.Status == ManagedTaskStatusBlocked
}

// CanStart checks if a task can be started (ready and not blocked)
func (t *ManagedTask) CanStart() bool {
	return t.IsReady() && !t.IsBlocked() && t.Status == ManagedTaskStatusReady
}

// Assign assigns the task to an agent
func (t *ManagedTask) Assign(agentName string) {
	t.AssignedTo = agentName
	now := time.Now()
	t.AssignedAt = &now
	if t.Status == ManagedTaskStatusNew || t.Status == ManagedTaskStatusReady {
		t.Status = ManagedTaskStatusAssigned
	}
}

// Start marks the task as in progress
func (t *ManagedTask) Start() error {
	if !t.CanStart() && t.Status != ManagedTaskStatusAssigned {
		return &TaskError{
			TaskKey: t.TaskKey,
			Message: "task cannot be started: not ready or blocked",
		}
	}

	now := time.Now()
	t.StartedAt = &now
	t.Status = ManagedTaskStatusInProgress
	return nil
}

// Complete marks the task as done
func (t *ManagedTask) Complete(result string, artifactIDs []int) error {
	if !t.DODMet {
		return &TaskError{
			TaskKey: t.TaskKey,
			Message: "task cannot be completed: Definition of Done not met",
		}
	}

	now := time.Now()
	t.CompletedAt = &now
	t.Result = result
	t.ArtifactIDs = artifactIDs
	t.Status = ManagedTaskStatusDone
	return nil
}

// Block marks the task as blocked
func (t *ManagedTask) Block(reason string) {
	t.Status = ManagedTaskStatusBlocked
	if t.Metadata == nil {
		t.Metadata = make(map[string]interface{})
	}
	t.Metadata["block_reason"] = reason
	t.Metadata["blocked_at"] = time.Now()
}

// Unblock removes the blocked status
func (t *ManagedTask) Unblock() {
	if t.Status == ManagedTaskStatusBlocked {
		t.Status = ManagedTaskStatusReady
		if t.Metadata != nil {
			delete(t.Metadata, "block_reason")
			delete(t.Metadata, "blocked_at")
		}
	}
}

// RequestReview moves task to review status
func (t *ManagedTask) RequestReview(reviewer string) {
	t.Status = ManagedTaskStatusInReview
	t.Reviewer = reviewer
	t.ReviewStatus = ReviewStatusPending
}

// ToJSON converts task to JSON string
func (t *ManagedTask) ToJSON() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON creates a managed task from JSON string
func FromJSON(jsonStr string) (*ManagedTask, error) {
	var task ManagedTask
	err := json.Unmarshal([]byte(jsonStr), &task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// TaskError represents an error related to task operations
type TaskError struct {
	TaskKey string
	Message string
}

func (e *TaskError) Error() string {
	return "task " + e.TaskKey + ": " + e.Message
}
