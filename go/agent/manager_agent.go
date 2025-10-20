package agent

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ManagerAgent is responsible for task orchestration and coordination
type ManagerAgent struct {
	name      string
	queue     *TaskQueue
	db        *sql.DB
	agentPool map[string]ManagedAgentInfo
}

// ManagedAgentInfo contains information about available agents in the task queue
type ManagedAgentInfo struct {
	Name         string
	Type         string // research, code, test, review
	Available    bool
	CurrentTasks []int // Task IDs currently assigned
	Capacity     int   // Max concurrent tasks
}

// NewManagerAgent creates a new Manager Agent
func NewManagerAgent(db *sql.DB) *ManagerAgent {
	return &ManagerAgent{
		name:      "Manager",
		queue:     NewTaskQueue(db),
		db:        db,
		agentPool: make(map[string]ManagedAgentInfo),
	}
}

// RegisterAgent registers an agent in the pool
func (m *ManagerAgent) RegisterAgent(info ManagedAgentInfo) {
	m.agentPool[info.Name] = info
}

// CreateTask creates a new task with proper validation
func (m *ManagerAgent) CreateTask(ctx context.Context, title, description string, taskType ManagedTaskType) (*ManagedTask, error) {
	// Create task with default values
	task := NewManagedTask(title, description, taskType)

	// Set default DoR/DoD criteria based on type
	SetDefaultDORCriteria(task)
	SetDefaultDODCriteria(task)

	// Create in database
	if err := m.queue.CreateTask(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Log the creation
	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Created task %s: %s", task.TaskKey, task.Title), task.TaskKey)

	return task, nil
}

// CreateSubtask creates a subtask under a parent task
func (m *ManagerAgent) CreateSubtask(ctx context.Context, parentTaskID int, title, description string, taskType ManagedTaskType) (*ManagedTask, error) {
	// Verify parent exists
	parent, err := m.queue.GetTask(parentTaskID)
	if err != nil {
		return nil, fmt.Errorf("parent task not found: %w", err)
	}

	// Create subtask
	task := NewManagedTask(title, description, taskType)
	task.ParentTaskID = &parentTaskID

	// Inherit priority from parent
	task.Priority = parent.Priority

	// Set default criteria
	SetDefaultDORCriteria(task)
	SetDefaultDODCriteria(task)

	if err := m.queue.CreateTask(task); err != nil {
		return nil, fmt.Errorf("failed to create subtask: %w", err)
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Created subtask %s under %s", task.TaskKey, parent.TaskKey), task.TaskKey)

	return task, nil
}

// ValidateAndMarkReady validates DoR and marks task as ready
func (m *ManagerAgent) ValidateAndMarkReady(ctx context.Context, taskID int) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	// Run DoR validation
	validator := NewDORValidator(task)
	if err := validator.MarkReady(); err != nil {
		m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s failed DoR validation: %s", task.TaskKey, err.Error()), task.TaskKey)
		return err
	}

	// Update in database
	if err := m.queue.UpdateTask(task); err != nil {
		return err
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s marked as ready", task.TaskKey), task.TaskKey)

	return nil
}

// AssignTaskToAgent assigns a task to the most appropriate agent
func (m *ManagerAgent) AssignTaskToAgent(ctx context.Context, taskID int, preferredAgent string) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	// Check if task is ready
	if !task.IsReady() {
		return fmt.Errorf("task %s is not ready for assignment", task.TaskKey)
	}

	// Determine which agent to assign
	agentName := preferredAgent
	if agentName == "" {
		agentName = m.selectBestAgent(task)
	}

	// Assign the task
	if err := m.queue.AssignTask(taskID, agentName); err != nil {
		return err
	}

	m.logCommunication(ctx, agentName, "handoff", fmt.Sprintf("Assigned task %s: %s", task.TaskKey, task.Title), task.TaskKey)

	return nil
}

// StartTask marks a task as in progress
func (m *ManagerAgent) StartTask(ctx context.Context, taskID int, agentName string) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	// Verify agent is assigned to this task
	if task.AssignedTo != agentName {
		return fmt.Errorf("task %s is not assigned to %s", task.TaskKey, agentName)
	}

	if err := m.queue.StartTask(taskID); err != nil {
		return err
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s started by %s", task.TaskKey, agentName), task.TaskKey)

	return nil
}

// CompleteTask validates DoD and marks task as complete
func (m *ManagerAgent) CompleteTask(ctx context.Context, taskID int, result string, artifactIDs []int) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	// Set the result
	task.Result = result
	task.ArtifactIDs = artifactIDs

	// Validate DoD
	validator := NewDODValidator(task)
	if err := validator.MarkDone(); err != nil {
		m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s failed DoD validation: %s", task.TaskKey, err.Error()), task.TaskKey)
		return err
	}

	// Complete the task
	if err := m.queue.CompleteTask(taskID, result, artifactIDs); err != nil {
		return err
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s completed successfully", task.TaskKey), task.TaskKey)

	// Check if parent task can be completed
	if task.ParentTaskID != nil {
		m.checkParentCompletion(ctx, *task.ParentTaskID)
	}

	return nil
}

// BlockTask marks a task as blocked
func (m *ManagerAgent) BlockTask(ctx context.Context, taskID int, reason string) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := m.queue.BlockTask(taskID, reason); err != nil {
		return err
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s blocked: %s", task.TaskKey, reason), task.TaskKey)

	return nil
}

// UnblockTask removes blocked status
func (m *ManagerAgent) UnblockTask(ctx context.Context, taskID int) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := m.queue.UnblockTask(taskID); err != nil {
		return err
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s unblocked", task.TaskKey), task.TaskKey)

	return nil
}

// RequestReview requests a review for a task
func (m *ManagerAgent) RequestReview(ctx context.Context, taskID int, reviewerAgent string) error {
	task, err := m.queue.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := m.queue.RequestReview(taskID, reviewerAgent); err != nil {
		return err
	}

	m.logCommunication(ctx, reviewerAgent, "handoff", fmt.Sprintf("Review requested for task %s", task.TaskKey), task.TaskKey)

	return nil
}

// GetTaskStatus returns the current status of a task
func (m *ManagerAgent) GetTaskStatus(taskID int) (*ManagedTask, error) {
	return m.queue.GetTask(taskID)
}

// ListAllTasks lists all tasks with optional filters
func (m *ManagerAgent) ListAllTasks(filters TaskFilters) ([]*ManagedTask, error) {
	return m.queue.ListTasks(filters)
}

// GetQueueStatistics returns statistics about the task queue
func (m *ManagerAgent) GetQueueStatistics() (*TaskStatistics, error) {
	return m.queue.GetTaskStatistics()
}

// GetReadyTasksForAssignment returns tasks that are ready and unassigned
func (m *ManagerAgent) GetReadyTasksForAssignment() ([]*ManagedTask, error) {
	readyTasks, err := m.queue.GetReadyTasks()
	if err != nil {
		return nil, err
	}

	// Filter for unassigned tasks
	var unassignedTasks []*ManagedTask
	for _, task := range readyTasks {
		if task.AssignedTo == "" {
			unassignedTasks = append(unassignedTasks, task)
		}
	}

	return unassignedTasks, nil
}

// AutoAssignReadyTasks automatically assigns ready tasks to available agents
func (m *ManagerAgent) AutoAssignReadyTasks(ctx context.Context) (int, error) {
	readyTasks, err := m.GetReadyTasksForAssignment()
	if err != nil {
		return 0, err
	}

	assigned := 0
	for _, task := range readyTasks {
		agent := m.selectBestAgent(task)
		if agent != "" {
			if err := m.AssignTaskToAgent(ctx, task.ID, agent); err != nil {
				// Log error but continue with other tasks
				m.logCommunication(ctx, "", "notification", fmt.Sprintf("Failed to assign task %s: %s", task.TaskKey, err.Error()), task.TaskKey)
				continue
			}
			assigned++
		}
	}

	return assigned, nil
}

// selectBestAgent selects the best agent for a task based on type and availability
func (m *ManagerAgent) selectBestAgent(task *ManagedTask) string {
	var bestAgent string
	minLoad := 999999

	for name, info := range m.agentPool {
		// Check if agent handles this task type
		if !m.agentCanHandleTaskType(info, task.Type) {
			continue
		}

		// Check if agent is available
		if !info.Available {
			continue
		}

		// Check if agent is at capacity
		if len(info.CurrentTasks) >= info.Capacity {
			continue
		}

		// Select agent with least load
		if len(info.CurrentTasks) < minLoad {
			minLoad = len(info.CurrentTasks)
			bestAgent = name
		}
	}

	return bestAgent
}

// agentCanHandleTaskType checks if an agent can handle a specific task type
func (m *ManagerAgent) agentCanHandleTaskType(agent ManagedAgentInfo, taskType ManagedTaskType) bool {
	switch agent.Type {
	case "research":
		return taskType == ManagedTaskTypeResearch || taskType == ManagedTaskTypeAnalysis
	case "code":
		return taskType == ManagedTaskTypeCode
	case "test":
		return taskType == ManagedTaskTypeTest
	case "review":
		return taskType == ManagedTaskTypeReview
	case "general":
		return true // General agents can handle any task
	default:
		return false
	}
}

// checkParentCompletion checks if all subtasks are complete and completes parent if so
func (m *ManagerAgent) checkParentCompletion(ctx context.Context, parentTaskID int) {
	parent, err := m.queue.GetTask(parentTaskID)
	if err != nil {
		return
	}

	// Get all subtasks
	subtasks, err := m.queue.GetSubtasks(parentTaskID)
	if err != nil {
		return
	}

	// Check if all subtasks are done
	allDone := true
	for _, subtask := range subtasks {
		if !subtask.IsDone() {
			allDone = false
			break
		}
	}

	if allDone && parent.Status != ManagedTaskStatusDone {
		// Mark parent DoD as met
		parent.DODMet = true

		// Collect all subtask results
		var results []string
		var allArtifacts []int
		for _, subtask := range subtasks {
			if subtask.Result != "" {
				results = append(results, fmt.Sprintf("%s: %s", subtask.TaskKey, subtask.Result))
			}
			allArtifacts = append(allArtifacts, subtask.ArtifactIDs...)
		}

		// Complete parent task
		combinedResult := fmt.Sprintf("All %d subtasks completed", len(subtasks))
		if len(results) > 0 {
			combinedResult += "\n\nSubtask results:\n" + joinStrings(results, "\n")
		}

		if err := m.queue.CompleteTask(parentTaskID, combinedResult, allArtifacts); err == nil {
			m.logCommunication(ctx, "", "notification", fmt.Sprintf("Parent task %s auto-completed (all subtasks done)", parent.TaskKey), parent.TaskKey)
		}
	}
}

// logCommunication logs inter-agent communication
func (m *ManagerAgent) logCommunication(ctx context.Context, toAgent, messageType, content, contextRef string) {
	query := `
		INSERT INTO agent_communications (from_agent, to_agent, message_type, content, context_ref, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := m.db.Exec(query, m.name, toAgent, messageType, content, contextRef, time.Now())
	if err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: Failed to log communication: %v\n", err)
	}
}

// GetCommunicationsForAgent retrieves communications for a specific agent
func (m *ManagerAgent) GetCommunicationsForAgent(agentName string, limit int) ([]AgentCommunication, error) {
	query := `
		SELECT id, from_agent, to_agent, message_type, content, context_ref, created_at
		FROM agent_communications
		WHERE to_agent = ? OR to_agent = ''
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := m.db.Query(query, agentName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comms []AgentCommunication
	for rows.Next() {
		var comm AgentCommunication
		var toAgent sql.NullString

		err := rows.Scan(&comm.ID, &comm.FromAgent, &toAgent, &comm.MessageType, &comm.Content, &comm.ContextRef, &comm.CreatedAt)
		if err != nil {
			continue
		}

		if toAgent.Valid {
			comm.ToAgent = toAgent.String
		}

		comms = append(comms, comm)
	}

	return comms, nil
}

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
