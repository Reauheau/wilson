package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TaskQueue manages the queue of tasks and provides operations for task management
type TaskQueue struct {
	db *sql.DB
}

// NewTaskQueue creates a new task queue manager
func NewTaskQueue(db *sql.DB) *TaskQueue {
	return &TaskQueue{db: db}
}

// CreateTask creates a new task in the database
func (q *TaskQueue) CreateTask(task *ManagedTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Generate task key if not provided
	if task.TaskKey == "" {
		task.TaskKey = q.generateTaskKey()
	}

	// Set defaults if not provided
	if task.Status == "" {
		task.Status = ManagedTaskStatusNew
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	// Marshal JSON fields
	dorCriteria, _ := json.Marshal(task.DORCriteria)
	dodCriteria, _ := json.Marshal(task.DODCriteria)
	dependsOn, _ := json.Marshal(task.DependsOn)
	blocks, _ := json.Marshal(task.Blocks)
	input, _ := json.Marshal(task.Input)
	artifactIDs, _ := json.Marshal(task.ArtifactIDs)
	metadata, _ := json.Marshal(task.Metadata)

	query := `
		INSERT INTO tasks (
			parent_task_id, task_key, title, description, type,
			assigned_to, assigned_at, status, priority,
			dor_criteria, dor_met, dod_criteria, dod_met,
			depends_on, blocks, input, result, artifact_ids,
			created_at, started_at, completed_at,
			review_status, review_comments, reviewer, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := q.db.Exec(query,
		task.ParentTaskID, task.TaskKey, task.Title, task.Description, task.Type,
		task.AssignedTo, task.AssignedAt, task.Status, task.Priority,
		dorCriteria, task.DORMet, dodCriteria, task.DODMet,
		dependsOn, blocks, input, task.Result, artifactIDs,
		task.CreatedAt, task.StartedAt, task.CompletedAt,
		task.ReviewStatus, task.ReviewComments, task.Reviewer, metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get task ID: %w", err)
	}

	task.ID = int(id)
	return nil
}

// GetTask retrieves a task by ID
func (q *TaskQueue) GetTask(id int) (*ManagedTask, error) {
	query := `
		SELECT id, parent_task_id, task_key, title, description, type,
			assigned_to, assigned_at, status, priority,
			dor_criteria, dor_met, dod_criteria, dod_met,
			depends_on, blocks, input, result, artifact_ids,
			created_at, started_at, completed_at,
			review_status, review_comments, reviewer, metadata
		FROM tasks WHERE id = ?
	`

	return q.scanTask(q.db.QueryRow(query, id))
}

// GetTaskByKey retrieves a task by its task key
func (q *TaskQueue) GetTaskByKey(taskKey string) (*ManagedTask, error) {
	query := `
		SELECT id, parent_task_id, task_key, title, description, type,
			assigned_to, assigned_at, status, priority,
			dor_criteria, dor_met, dod_criteria, dod_met,
			depends_on, blocks, input, result, artifact_ids,
			created_at, started_at, completed_at,
			review_status, review_comments, reviewer, metadata
		FROM tasks WHERE task_key = ?
	`

	return q.scanTask(q.db.QueryRow(query, taskKey))
}

// UpdateTask updates an existing task
func (q *TaskQueue) UpdateTask(task *ManagedTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Marshal JSON fields
	dorCriteria, _ := json.Marshal(task.DORCriteria)
	dodCriteria, _ := json.Marshal(task.DODCriteria)
	dependsOn, _ := json.Marshal(task.DependsOn)
	blocks, _ := json.Marshal(task.Blocks)
	input, _ := json.Marshal(task.Input)
	artifactIDs, _ := json.Marshal(task.ArtifactIDs)
	metadata, _ := json.Marshal(task.Metadata)

	query := `
		UPDATE tasks SET
			parent_task_id = ?, task_key = ?, title = ?, description = ?, type = ?,
			assigned_to = ?, assigned_at = ?, status = ?, priority = ?,
			dor_criteria = ?, dor_met = ?, dod_criteria = ?, dod_met = ?,
			depends_on = ?, blocks = ?, input = ?, result = ?, artifact_ids = ?,
			started_at = ?, completed_at = ?,
			review_status = ?, review_comments = ?, reviewer = ?, metadata = ?
		WHERE id = ?
	`

	_, err := q.db.Exec(query,
		task.ParentTaskID, task.TaskKey, task.Title, task.Description, task.Type,
		task.AssignedTo, task.AssignedAt, task.Status, task.Priority,
		dorCriteria, task.DORMet, dodCriteria, task.DODMet,
		dependsOn, blocks, input, task.Result, artifactIDs,
		task.StartedAt, task.CompletedAt,
		task.ReviewStatus, task.ReviewComments, task.Reviewer, metadata,
		task.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// DeleteTask deletes a task by ID
func (q *TaskQueue) DeleteTask(id int) error {
	query := `DELETE FROM tasks WHERE id = ?`
	_, err := q.db.Exec(query, id)
	return err
}

// ListTasks lists tasks with optional filters
func (q *TaskQueue) ListTasks(filters TaskFilters) ([]*ManagedTask, error) {
	query := `
		SELECT id, parent_task_id, task_key, title, description, type,
			assigned_to, assigned_at, status, priority,
			dor_criteria, dor_met, dod_criteria, dod_met,
			depends_on, blocks, input, result, artifact_ids,
			created_at, started_at, completed_at,
			review_status, review_comments, reviewer, metadata
		FROM tasks WHERE 1=1
	`

	var args []interface{}
	var conditions []string

	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}

	if filters.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filters.AssignedTo)
	}

	if filters.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, filters.Type)
	}

	if filters.ParentTaskID != nil {
		conditions = append(conditions, "parent_task_id = ?")
		args = append(args, *filters.ParentTaskID)
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ordering
	query += " ORDER BY priority DESC, created_at ASC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}

	rows, err := q.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*ManagedTask
	for rows.Next() {
		task, err := q.scanTaskFromRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetReadyTasks returns all tasks that are ready to be picked up
func (q *TaskQueue) GetReadyTasks() ([]*ManagedTask, error) {
	return q.ListTasks(TaskFilters{
		Status: ManagedTaskStatusReady,
	})
}

// GetTasksForAgent returns all tasks assigned to a specific agent
func (q *TaskQueue) GetTasksForAgent(agentName string) ([]*ManagedTask, error) {
	return q.ListTasks(TaskFilters{
		AssignedTo: agentName,
	})
}

// GetInProgressTasks returns all tasks currently in progress
func (q *TaskQueue) GetInProgressTasks() ([]*ManagedTask, error) {
	return q.ListTasks(TaskFilters{
		Status: ManagedTaskStatusInProgress,
	})
}

// GetBlockedTasks returns all blocked tasks
func (q *TaskQueue) GetBlockedTasks() ([]*ManagedTask, error) {
	return q.ListTasks(TaskFilters{
		Status: ManagedTaskStatusBlocked,
	})
}

// GetSubtasks returns all subtasks of a parent task
func (q *TaskQueue) GetSubtasks(parentTaskID int) ([]*ManagedTask, error) {
	return q.ListTasks(TaskFilters{
		ParentTaskID: &parentTaskID,
	})
}

// AssignTask assigns a task to an agent
func (q *TaskQueue) AssignTask(taskID int, agentName string) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	task.Assign(agentName)
	return q.UpdateTask(task)
}

// StartTask marks a task as in progress
func (q *TaskQueue) StartTask(taskID int) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := task.Start(); err != nil {
		return err
	}

	return q.UpdateTask(task)
}

// CompleteTask marks a task as done
func (q *TaskQueue) CompleteTask(taskID int, result string, artifactIDs []int) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := task.Complete(result, artifactIDs); err != nil {
		return err
	}

	// Check if this task blocks other tasks and unblock them
	if err := q.UnblockDependentTasks(task.TaskKey); err != nil {
		return fmt.Errorf("failed to unblock dependent tasks: %w", err)
	}

	return q.UpdateTask(task)
}

// BlockTask marks a task as blocked
func (q *TaskQueue) BlockTask(taskID int, reason string) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	task.Block(reason)
	return q.UpdateTask(task)
}

// UnblockTask removes the blocked status from a task
func (q *TaskQueue) UnblockTask(taskID int) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	task.Unblock()
	return q.UpdateTask(task)
}

// UnblockDependentTasks unblocks tasks that depend on the completed task
func (q *TaskQueue) UnblockDependentTasks(completedTaskKey string) error {
	// Find all tasks that depend on the completed task
	allTasks, err := q.ListTasks(TaskFilters{})
	if err != nil {
		return err
	}

	for _, task := range allTasks {
		// Check if this task depends on the completed task
		for i, dep := range task.DependsOn {
			if dep == completedTaskKey {
				// Remove the dependency
				task.DependsOn = append(task.DependsOn[:i], task.DependsOn[i+1:]...)

				// If no more dependencies and task is blocked, unblock it
				if len(task.DependsOn) == 0 && task.Status == ManagedTaskStatusBlocked {
					task.Unblock()
				}

				if err := q.UpdateTask(task); err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

// RequestReview moves a task to review status
func (q *TaskQueue) RequestReview(taskID int, reviewer string) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	task.RequestReview(reviewer)
	return q.UpdateTask(task)
}

// GetTaskStatistics returns statistics about tasks in the queue
func (q *TaskQueue) GetTaskStatistics() (*TaskStatistics, error) {
	stats := &TaskStatistics{}

	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'new' THEN 1 ELSE 0 END) as new,
			SUM(CASE WHEN status = 'ready' THEN 1 ELSE 0 END) as ready,
			SUM(CASE WHEN status = 'assigned' THEN 1 ELSE 0 END) as assigned,
			SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END) as in_progress,
			SUM(CASE WHEN status = 'in_review' THEN 1 ELSE 0 END) as in_review,
			SUM(CASE WHEN status = 'blocked' THEN 1 ELSE 0 END) as blocked,
			SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END) as done,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed
		FROM tasks
	`

	err := q.db.QueryRow(query).Scan(
		&stats.Total,
		&stats.New,
		&stats.Ready,
		&stats.Assigned,
		&stats.InProgress,
		&stats.InReview,
		&stats.Blocked,
		&stats.Done,
		&stats.Failed,
	)

	return stats, err
}

// generateTaskKey generates a unique task key
func (q *TaskQueue) generateTaskKey() string {
	// Query for the highest task number
	var maxNum int
	query := `
		SELECT COALESCE(MAX(CAST(SUBSTR(task_key, 6) AS INTEGER)), 0)
		FROM tasks
		WHERE task_key LIKE 'TASK-%'
	`
	_ = q.db.QueryRow(query).Scan(&maxNum)

	return fmt.Sprintf("TASK-%03d", maxNum+1)
}

// scanTask scans a single row into a ManagedTask
func (q *TaskQueue) scanTask(row *sql.Row) (*ManagedTask, error) {
	task := &ManagedTask{}

	var dorCriteria, dodCriteria, dependsOn, blocks, input, artifactIDs, metadata []byte
	var parentTaskID sql.NullInt64
	var assignedTo, result, reviewComments, reviewer sql.NullString
	var assignedAt, startedAt, completedAt sql.NullTime
	var reviewStatus sql.NullString

	err := row.Scan(
		&task.ID, &parentTaskID, &task.TaskKey, &task.Title, &task.Description, &task.Type,
		&assignedTo, &assignedAt, &task.Status, &task.Priority,
		&dorCriteria, &task.DORMet, &dodCriteria, &task.DODMet,
		&dependsOn, &blocks, &input, &result, &artifactIDs,
		&task.CreatedAt, &startedAt, &completedAt,
		&reviewStatus, &reviewComments, &reviewer, &metadata,
	)

	if err != nil {
		return nil, err
	}

	return q.populateTask(task, parentTaskID, assignedTo, result, reviewComments, reviewer,
		assignedAt, startedAt, completedAt, reviewStatus, dorCriteria, dodCriteria, dependsOn, blocks, input, artifactIDs, metadata)
}

// scanTaskFromRows scans a row from a result set into a ManagedTask
func (q *TaskQueue) scanTaskFromRows(rows *sql.Rows) (*ManagedTask, error) {
	task := &ManagedTask{}

	var dorCriteria, dodCriteria, dependsOn, blocks, input, artifactIDs, metadata []byte
	var parentTaskID sql.NullInt64
	var assignedTo, result, reviewComments, reviewer sql.NullString
	var assignedAt, startedAt, completedAt sql.NullTime
	var reviewStatus sql.NullString

	err := rows.Scan(
		&task.ID, &parentTaskID, &task.TaskKey, &task.Title, &task.Description, &task.Type,
		&assignedTo, &assignedAt, &task.Status, &task.Priority,
		&dorCriteria, &task.DORMet, &dodCriteria, &task.DODMet,
		&dependsOn, &blocks, &input, &result, &artifactIDs,
		&task.CreatedAt, &startedAt, &completedAt,
		&reviewStatus, &reviewComments, &reviewer, &metadata,
	)

	if err != nil {
		return nil, err
	}

	return q.populateTask(task, parentTaskID, assignedTo, result, reviewComments, reviewer,
		assignedAt, startedAt, completedAt, reviewStatus, dorCriteria, dodCriteria, dependsOn, blocks, input, artifactIDs, metadata)
}

// populateTask populates a task with nullable and JSON fields
func (q *TaskQueue) populateTask(task *ManagedTask, parentTaskID sql.NullInt64, assignedTo, result, reviewComments, reviewer sql.NullString,
	assignedAt, startedAt, completedAt sql.NullTime, reviewStatus sql.NullString,
	dorCriteria, dodCriteria, dependsOn, blocks, input, artifactIDs, metadata []byte) (*ManagedTask, error) {

	// Handle nullable fields
	if parentTaskID.Valid {
		pid := int(parentTaskID.Int64)
		task.ParentTaskID = &pid
	}
	if assignedTo.Valid {
		task.AssignedTo = assignedTo.String
	}
	if assignedAt.Valid {
		task.AssignedAt = &assignedAt.Time
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if result.Valid {
		task.Result = result.String
	}
	if reviewStatus.Valid {
		task.ReviewStatus = ReviewStatus(reviewStatus.String)
	}
	if reviewComments.Valid {
		task.ReviewComments = reviewComments.String
	}
	if reviewer.Valid {
		task.Reviewer = reviewer.String
	}

	// Unmarshal JSON fields
	if len(dorCriteria) > 0 {
		json.Unmarshal(dorCriteria, &task.DORCriteria)
	}
	if len(dodCriteria) > 0 {
		json.Unmarshal(dodCriteria, &task.DODCriteria)
	}
	if len(dependsOn) > 0 {
		json.Unmarshal(dependsOn, &task.DependsOn)
	}
	if len(blocks) > 0 {
		json.Unmarshal(blocks, &task.Blocks)
	}
	if len(input) > 0 {
		json.Unmarshal(input, &task.Input)
	}
	if len(artifactIDs) > 0 {
		json.Unmarshal(artifactIDs, &task.ArtifactIDs)
	}
	if len(metadata) > 0 {
		json.Unmarshal(metadata, &task.Metadata)
	}

	return task, nil
}

// TaskFilters defines filters for listing tasks
type TaskFilters struct {
	Status       ManagedTaskStatus
	AssignedTo   string
	Type         ManagedTaskType
	ParentTaskID *int
	Limit        int
}

// TaskStatistics contains statistics about tasks
type TaskStatistics struct {
	Total      int
	New        int
	Ready      int
	Assigned   int
	InProgress int
	InReview   int
	Blocked    int
	Done       int
	Failed     int
}
