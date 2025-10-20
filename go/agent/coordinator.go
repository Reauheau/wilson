package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"wilson/llm"
)

// Coordinator manages agent execution and task delegation
type Coordinator struct {
	registry      *Registry
	llmManager    *llm.Manager
	tasks         map[string]*Task
	results       map[string]*Result
	mu            sync.RWMutex
	maxDepth      int // Maximum delegation depth to prevent infinite loops
	maxConcurrent int // Maximum concurrent workers (default: 2)
	semaphore     chan struct{} // Semaphore for concurrency control
}

// NewCoordinator creates a new agent coordinator
func NewCoordinator(registry *Registry) *Coordinator {
	maxConcurrent := 2 // Default: 2 concurrent workers for 16GB RAM machines
	return &Coordinator{
		registry:      registry,
		tasks:         make(map[string]*Task),
		results:       make(map[string]*Result),
		maxDepth:      5, // Max 5 levels of delegation
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// SetLLMManager sets the LLM manager for model lifecycle management
func (c *Coordinator) SetLLMManager(manager *llm.Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.llmManager = manager
}

// SetMaxConcurrent sets the maximum concurrent workers
func (c *Coordinator) SetMaxConcurrent(max int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxConcurrent = max
	c.semaphore = make(chan struct{}, max)
}

// DelegateTask delegates a task to an appropriate agent (synchronous)
func (c *Coordinator) DelegateTask(ctx context.Context, req DelegationRequest) (*Result, error) {
	// Create task
	task := &Task{
		ID:          uuid.New().String(),
		ContextKey:  req.ContextKey,
		Type:        req.Type,
		Description: req.Description,
		Input:       req.Input,
		RequestedBy: "user",
		CreatedAt:   time.Now(),
		Status:      TaskPending,
		Priority:    req.Priority,
	}

	// Store task
	c.mu.Lock()
	c.tasks[task.ID] = task
	c.mu.Unlock()

	// Get agent
	agent, err := c.registry.Get(req.ToAgent)
	if err != nil {
		// Try to find a capable agent
		capable := c.registry.FindCapable(task)
		if len(capable) == 0 {
			return nil, fmt.Errorf("no agent found to handle task type: %s", task.Type)
		}
		agent = capable[0] // Use first capable agent
	}

	// Execute task
	return c.ExecuteTask(ctx, task, agent)
}

// DelegateTaskAsync delegates a task asynchronously (non-blocking)
// Returns task ID immediately, execution happens in background goroutine
// Wilson's chat model never blocks - agent uses its own model in background
func (c *Coordinator) DelegateTaskAsync(ctx context.Context, req DelegationRequest) (string, error) {
	// Create task
	task := &Task{
		ID:          uuid.New().String(),
		ContextKey:  req.ContextKey,
		Type:        req.Type,
		Description: req.Description,
		Input:       req.Input,
		RequestedBy: "user",
		CreatedAt:   time.Now(),
		Status:      TaskPending,
		Priority:    req.Priority,
	}

	// Store task immediately
	c.mu.Lock()
	c.tasks[task.ID] = task
	c.mu.Unlock()

	// Get agent
	agent, err := c.registry.Get(req.ToAgent)
	if err != nil {
		// Try to find a capable agent
		capable := c.registry.FindCapable(task)
		if len(capable) == 0 {
			// Mark task as failed
			c.mu.Lock()
			task.Status = TaskFailed
			c.results[task.ID] = &Result{
				TaskID:  task.ID,
				Success: false,
				Error:   fmt.Sprintf("no agent found to handle task type: %s", task.Type),
			}
			c.mu.Unlock()
			return task.ID, fmt.Errorf("no agent found to handle task type: %s", task.Type)
		}
		agent = capable[0] // Use first capable agent
	}

	// Spawn goroutine for execution - DOES NOT BLOCK
	go func() {
		// Acquire semaphore slot (blocks if at max_concurrent limit)
		c.semaphore <- struct{}{}
		defer func() { <-c.semaphore }() // Release slot when done

		// Acquire model for this agent (Phase 0 lifecycle + Phase 5 fallback)
		var modelName string
		var usedFallback bool
		if c.llmManager != nil {
			client, release, fallback, err := c.llmManager.AcquireModel(agent.Purpose())
			if err != nil {
				// Model unavailable (no fallback available either)
				c.mu.Lock()
				task.Status = TaskFailed
				c.results[task.ID] = &Result{
					TaskID:  task.ID,
					Success: false,
					Error:   fmt.Sprintf("model unavailable (no fallback): %v", err),
				}
				c.mu.Unlock()
				return
			}

			// Successfully got model (either preferred or fallback)
			modelName = client.GetModel()
			usedFallback = fallback // Phase 5: Track if we used fallback
			defer release()          // ALWAYS release model when done (kill-after-task)

			// Update task with model and agent info (Phase 3 + Phase 5)
			c.mu.Lock()
			task.ModelUsed = modelName
			task.AgentName = agent.Name()
			task.UsedFallback = usedFallback
			c.mu.Unlock()
		}

		// Create background context (don't use parent ctx as it may be cancelled)
		bgCtx := context.Background()

		// Execute task in background with agent's own model
		result, err := c.ExecuteTask(bgCtx, task, agent)

		// Store result
		c.mu.Lock()
		if err != nil {
			task.Status = TaskFailed
			if result == nil {
				result = &Result{
					TaskID:  task.ID,
					Success: false,
					Error:   err.Error(),
				}
			}
		}
		c.results[task.ID] = result
		c.mu.Unlock()

		// Model automatically released via defer above (kill-after-task!)
	}()

	// Return task ID immediately (Wilson can continue chatting)
	return task.ID, nil
}

// ExecuteTask executes a task with a specific agent
func (c *Coordinator) ExecuteTask(ctx context.Context, task *Task, agent Agent) (*Result, error) {
	// Update task status
	c.mu.Lock()
	task.Status = TaskInProgress
	c.mu.Unlock()

	// Execute
	result, err := agent.Execute(ctx, task)

	// Update task status and store result
	c.mu.Lock()
	if err != nil || !result.Success {
		task.Status = TaskFailed
	} else {
		task.Status = TaskCompleted
	}
	result.CompletedAt = time.Now()
	c.results[task.ID] = result
	c.mu.Unlock()

	return result, err
}

// GetTaskStatus returns the status of a task
func (c *Coordinator) GetTaskStatus(taskID string) (*Task, *Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	task, ok := c.tasks[taskID]
	if !ok {
		return nil, nil, fmt.Errorf("task not found: %s", taskID)
	}

	result, _ := c.results[taskID]

	return task, result, nil
}

// ListTasks returns all tasks
func (c *Coordinator) ListTasks() []*Task {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tasks := make([]*Task, 0, len(c.tasks))
	for _, task := range c.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// GetActiveTasks returns tasks that are pending or in progress
func (c *Coordinator) GetActiveTasks() []*Task {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range c.tasks {
		if task.Status == TaskPending || task.Status == TaskInProgress {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// GetResult retrieves a task result
func (c *Coordinator) GetResult(taskID string) (*Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, ok := c.results[taskID]
	if !ok {
		return nil, fmt.Errorf("result not found for task: %s", taskID)
	}

	return result, nil
}

// Global coordinator instance
var globalCoordinator *Coordinator
var globalCoordinatorMu sync.RWMutex

// SetGlobalCoordinator sets the global coordinator
func SetGlobalCoordinator(coordinator *Coordinator) {
	globalCoordinatorMu.Lock()
	defer globalCoordinatorMu.Unlock()
	globalCoordinator = coordinator
}

// GetGlobalCoordinator returns the global coordinator
func GetGlobalCoordinator() *Coordinator {
	globalCoordinatorMu.RLock()
	defer globalCoordinatorMu.RUnlock()
	return globalCoordinator
}
