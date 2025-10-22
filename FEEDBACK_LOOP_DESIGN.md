# Wilson Feedback Loop Architecture - Implementation Specification

**Status:** Ready for Implementation
**Created:** 2025-10-22
**Last Updated:** 2025-10-22
**Purpose:** Self-healing and adaptive feedback system for Wilson's multi-agent architecture

---

## 🎯 Vision & Goals

Transform Wilson into a **self-improving, self-healing system** where:
- Agents detect failures and blockers automatically
- Agents request missing dependencies intelligently
- Manager dynamically adjusts task plans based on feedback
- System learns from errors (future phase)
- Success rate improves from ~75% to >95%

**Key Insight:** Like Claude Code - iteratively improving until success, not failing on first error.

---

## 📊 Current State (What Exists)

### ✅ Working Components

1. **Task Queue** (`agent/queue.go`)
   - SQLite persistence, full lifecycle: NEW → READY → ASSIGNED → IN_PROGRESS → DONE/FAILED
   - BLOCKED state exists but underutilized
   - Dependency tracking via `DependsOn` and `Blocks` fields
   - ⚠️ **Gap:** `UnblockDependentTasks()` exists but rarely called

2. **Manager Agent** (`agent/manager_agent.go`)
   - Heuristic task decomposition (keyword-based)
   - Sequential execution with dependency waiting
   - Recently added: `injectDependencyArtifacts()` for context passing
   - ⚠️ **Gap:** No feedback monitoring, no dynamic task insertion

3. **Agent Executor** (`agent/agent_executor.go`)
   - Max 9 iterations per task
   - Auto-injection: generate_code → write_file → compile (atomic)
   - ⚠️ **Gap:** Early return on success prevents feedback, brute force retries only

4. **Coordinator** (`agent/coordinator.go`)
   - Async execution (max 2 workers via semaphore)
   - Task progress tracking
   - ⚠️ **Gap:** No feedback aggregation

### ❌ Missing Components (To Be Implemented)

1. **Feedback Infrastructure** - No `feedback.go`, no `agent_feedback` table, no SendFeedback() methods
2. **Agent-to-Manager Communication** - Agents can't report "I need X before continuing"
3. **Precondition Checks** - No validation before execution starts
4. **Automatic Unblocking** - Blocked tasks are dead-ends

---

## 🏗️ Architecture Design

### Event-Driven Feedback Flow (NOT Polling)

```
┌─────────────────────────────────────────────────────────────┐
│                         USER                                 │
└────────────┬────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│                   MANAGER AGENT                              │
│  • Decomposes tasks                                          │
│  • Monitors feedback channel (event-driven)                  │
│  • Dynamically creates dependency tasks                      │
│  • Unblocks waiting tasks                                    │
└─────┬───────────────────────┬───────────────────────────────┘
      │                       │
      │ feedback chan         │ task assignment
      │ (non-blocking)        │
      ▼                       ▼
┌──────────────────┐    ┌─────────────────────────────────────┐
│ FEEDBACK BUS     │    │  WORKER AGENTS (Code, Test, Review) │
│ (Go channel)     │◄───┤  • Check preconditions              │
│                  │    │  • Execute with tools                │
│ Buffered: 100    │    │  • Send feedback via channel        │
└──────────────────┘    └─────────────────────────────────────┘
```

**Key Decision:** Event-driven (Go channels) NOT polling
**Rationale:** Tasks complete in <5 seconds. 2-second polling = 40% miss rate.

---

## 🔧 Implementation Specification

### Phase 0: Foundation Fixes (PREREQUISITE - 2 hours)

**Before implementing feedback, fix existing issues:**

#### Fix 1: Auto-Unblock on Task Completion

**File:** `agent/queue.go` line 287-303

```go
// CompleteTask marks a task as done
func (q *TaskQueue) CompleteTask(taskID int, result string, artifactIDs []int) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := task.Complete(result, artifactIDs); err != nil {
		return err
	}

	// ✅ ADD THIS: Always unblock dependents when task completes
	if err := q.UnblockDependentTasks(task.TaskKey); err != nil {
		// Log but don't fail - unblocking is non-critical
		fmt.Printf("Warning: Failed to unblock dependents of %s: %v\n", task.TaskKey, err)
	}

	return q.UpdateTask(task)
}
```

**Test:** Task A blocks on Task B → Task B completes → Task A automatically unblocked

#### Fix 2: Manager Calls UnblockDependentTasks

**File:** `agent/manager_agent.go` line 207

```go
func (m *ManagerAgent) CompleteTask(ctx context.Context, taskID int, result string, artifactIDs []int) error {
	// ... existing validation code ...

	if err := m.queue.CompleteTask(taskID, result, artifactIDs); err != nil {
		return err
	}

	m.logCommunication(ctx, "", "notification", fmt.Sprintf("Task %s completed successfully", task.TaskKey), task.TaskKey)

	// ✅ ADD THIS: Explicit unblock call (redundant but safe)
	if err := m.queue.UnblockDependentTasks(task.TaskKey); err != nil {
		fmt.Printf("[ManagerAgent] Warning: Unblock failed for %s: %v\n", task.TaskKey, err)
	}

	// Check if parent task can be completed
	if task.ParentTaskID != nil {
		m.checkParentCompletion(ctx, *task.ParentTaskID)
	}

	return nil
}
```

---

### Phase 1: Minimal Viable Feedback (MVP - 3-4 hours)

**Goal:** Prove concept with ONE feedback type working end-to-end

#### Component 1: Feedback Types & Bus

**File:** `agent/feedback.go` (NEW)

```go
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
	FeedbackTypeSuccess          FeedbackType = "success"
)

// FeedbackSeverity indicates urgency
type FeedbackSeverity string

const (
	FeedbackSeverityInfo     FeedbackSeverity = "info"
	FeedbackSeverityWarning  FeedbackSeverity = "warning"
	FeedbackSeverityCritical FeedbackSeverity = "critical"
)

// AgentFeedback represents feedback from an agent
type AgentFeedback struct {
	TaskID       string                 `json:"task_id"`
	AgentName    string                 `json:"agent_name"`
	FeedbackType FeedbackType           `json:"feedback_type"`
	Severity     FeedbackSeverity       `json:"severity"`
	Message      string                 `json:"message"`
	Context      map[string]interface{} `json:"context"`
	Suggestion   string                 `json:"suggestion"`
	CreatedAt    time.Time              `json:"created_at"`
}

// FeedbackBus manages event-driven feedback
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
```

#### Component 2: Agent Methods

**File:** `agent/base_agent.go` (ADD TO EXISTING)

```go
// SendFeedback sends feedback via the feedback bus
func (a *BaseAgent) SendFeedback(ctx context.Context, feedbackType FeedbackType, severity FeedbackSeverity, message string, context map[string]interface{}, suggestion string) error {
	bus := GetFeedbackBus()

	feedback := &AgentFeedback{
		TaskID:       a.currentTaskID,
		AgentName:    a.name,
		FeedbackType: feedbackType,
		Severity:     severity,
		Message:      message,
		Context:      context,
		Suggestion:   suggestion,
	}

	return bus.Send(feedback)
}

// RequestDependency requests a missing dependency
func (a *BaseAgent) RequestDependency(ctx context.Context, description string, taskType ManagedTaskType) error {
	return a.SendFeedback(ctx,
		FeedbackTypeDependencyNeeded,
		FeedbackSeverityCritical,
		fmt.Sprintf("Cannot proceed without: %s", description),
		map[string]interface{}{
			"dependency_description": description,
			"dependency_type":        string(taskType),
		},
		"Create and complete the missing dependency before retrying this task",
	)
}
```

#### Component 3: Manager Handler

**File:** `agent/manager_agent.go` (ADD TO EXISTING)

```go
// StartFeedbackProcessing registers handlers and starts the feedback bus
func (m *ManagerAgent) StartFeedbackProcessing(ctx context.Context) {
	bus := GetFeedbackBus()

	// Register handlers
	bus.RegisterHandler(FeedbackTypeDependencyNeeded, m.handleDependencyRequest)

	// Start processing
	bus.Start(ctx)

	fmt.Println("[ManagerAgent] Feedback processing started")
}

// handleDependencyRequest creates missing dependency task
func (m *ManagerAgent) handleDependencyRequest(ctx context.Context, feedback *AgentFeedback) error {
	fmt.Printf("[ManagerAgent] Processing dependency request from %s for task %s\n",
		feedback.AgentName, feedback.TaskID)

	// Extract dependency info
	depDesc, ok := feedback.Context["dependency_description"].(string)
	if !ok {
		return fmt.Errorf("missing dependency_description")
	}

	depTypeStr, ok := feedback.Context["dependency_type"].(string)
	if !ok {
		depTypeStr = "code" // default
	}
	depType := ManagedTaskType(depTypeStr)

	// Parse task ID (format: TASK-001 or SIMPLE-timestamp)
	var currentTaskID int
	fmt.Sscanf(feedback.TaskID, "TASK-%d", &currentTaskID)
	if currentTaskID == 0 {
		return fmt.Errorf("cannot insert dependency for simple task: %s", feedback.TaskID)
	}

	currentTask, err := m.queue.GetTask(currentTaskID)
	if err != nil {
		return fmt.Errorf("failed to get current task: %w", err)
	}

	// Create dependency task
	depTask := NewManagedTask(
		fmt.Sprintf("Prerequisite: %s", depDesc),
		depDesc,
		depType,
	)
	depTask.ParentTaskID = currentTask.ParentTaskID
	depTask.Priority = currentTask.Priority + 1 // Higher priority

	// Copy project_path from current task
	if projectPath, ok := currentTask.Input["project_path"]; ok {
		depTask.Input = map[string]interface{}{
			"project_path": projectPath,
		}
	}

	SetDefaultDORCriteria(depTask)
	SetDefaultDODCriteria(depTask)

	// ✅ ATOMIC OPERATION: Create dep + block current (use transaction if available)
	if err := m.queue.CreateTask(depTask); err != nil {
		return fmt.Errorf("failed to create dependency task: %w", err)
	}

	currentTask.DependsOn = append(currentTask.DependsOn, depTask.TaskKey)
	currentTask.Block(fmt.Sprintf("Waiting for prerequisite: %s", depTask.TaskKey))

	if err := m.queue.UpdateTask(currentTask); err != nil {
		return fmt.Errorf("failed to update current task: %w", err)
	}

	fmt.Printf("[ManagerAgent] ✓ Created dependency %s → blocks %s\n", depTask.TaskKey, currentTask.TaskKey)

	// Mark dependency as ready and schedule for execution
	if err := m.ValidateAndMarkReady(ctx, depTask.ID); err != nil {
		return fmt.Errorf("failed to mark dependency ready: %w", err)
	}

	// ⚠️ IMPORTANT: Do NOT execute immediately - let coordinator handle it
	// This prevents deadlock when both workers are blocked waiting for dependencies
	fmt.Printf("[ManagerAgent] Dependency task %s queued for execution\n", depTask.TaskKey)

	return nil
}
```

#### Component 4: TestAgent Precondition Check

**File:** `agent/test_agent.go` (MODIFY Execute method)

```go
func (a *TestAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
	}

	// ✅ PRECONDITION CHECK
	if err := a.checkPreconditions(ctx, task); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Precondition failed: %v", err)
		// Feedback already sent, return error so task can be blocked
		return result, err
	}

	// ... rest of existing Execute logic ...
}

// checkPreconditions validates prerequisites before execution
func (a *TestAgent) checkPreconditions(ctx context.Context, task *Task) error {
	lowerDesc := strings.ToLower(task.Description)

	// Check: "run tests" requires test files to exist
	if strings.Contains(lowerDesc, "run test") || strings.Contains(lowerDesc, "execute test") {
		projectPath := "."
		if pathVal, ok := task.Input["project_path"]; ok {
			if pathStr, ok := pathVal.(string); ok {
				projectPath = pathStr
			}
		}

		// Check for test files
		testFiles, err := filepath.Glob(filepath.Join(projectPath, "*_test.go"))
		if err != nil || len(testFiles) == 0 {
			// Send feedback requesting test creation
			return a.RequestDependency(ctx,
				fmt.Sprintf("Create test files in %s", projectPath),
				ManagedTaskTypeCode,
			)
		}
	}

	return nil
}
```

#### Component 5: Initialization

**File:** `main.go` or wherever agents are initialized

```go
// Start feedback processing when manager is created
manager := NewManagerAgent(db)
manager.StartFeedbackProcessing(context.Background())
```

#### MVP Test Scenario

```
User: "Run tests in /empty/directory"

Flow:
1. ManagerAgent creates task TASK-001 (type: test)
2. Assigns to TestAgent
3. TestAgent.Execute() → checkPreconditions()
4. Detects: No *_test.go files
5. TestAgent.RequestDependency("Create test files")
6. Feedback sent via channel (non-blocking)
7. ManagerAgent.handleDependencyRequest():
   - Creates TASK-002 (type: code, "Create test files")
   - Blocks TASK-001 (depends on TASK-002)
   - Marks TASK-002 as READY
8. Coordinator picks up TASK-002
9. CodeAgent executes → creates test files
10. TASK-002 completes → UnblockDependentTasks(TASK-002)
11. TASK-001 unblocked automatically
12. TestAgent retries TASK-001 → test files exist → runs tests ✓

Expected: No max iterations error, task succeeds
```

**Success Criteria:**
- ✅ Feedback sent and received within 100ms
- ✅ Dependency task created and executed
- ✅ Original task automatically unblocked and retried
- ✅ No deadlocks with 2-worker semaphore

---

### Phase 2: Production Ready (4-5 hours)

After MVP works:

1. **Add Database Persistence** (optional - channels work fine for now)
   - Store feedback in `agent_feedback` table for auditing
   - Query feedback history per task
   - Metrics: feedback count by type, resolution time

2. **Add More Feedback Types**
   - `FeedbackTypeBlocker`: Unrecoverable errors (notify user)
   - `FeedbackTypeContextNeeded`: Request file loading
   - `FeedbackTypeSuccess`: Positive signals for learning

3. **Extend to Other Agents**
   - CodeAgent: Check if target directory exists
   - CodeAgent: Request related files on compile error
   - ReviewAgent: Request code artifacts

4. **Add Metrics**
   ```go
   type FeedbackMetrics struct {
       TotalFeedbacks       int
       ByType              map[FeedbackType]int
       ByAgent             map[string]int
       AvgResolutionTime   time.Duration
       UnresolvedCount     int
   }
   ```

---

## 🎯 Critical Decisions Made

### Decision 1: Event-Driven vs. Polling
**Chosen:** Event-driven (Go channels)
**Rationale:** Immediate processing, no polling overhead, natural Go pattern

### Decision 2: Agent State Management
**Chosen:** Stateless agents, state in task metadata
**Rationale:** Simpler, leverage existing task persistence
**Implementation:** Task blocked → returns error → Manager unblocks later → Agent retries from scratch

### Decision 3: Dependency Execution
**Chosen:** Queue dependency, let coordinator pick up (not immediate execution)
**Rationale:** Prevents deadlock when both workers blocked
**Trade-off:** Slightly slower but safe

### Decision 4: Atomic Task Principle
**Chosen:** Preserve it - feedback only DURING execution, not after
**Rationale:** Keeps tasks simple, feedback is for preconditions and mid-execution issues
**Consequence:** Post-execution feedback (like "needs review") handled differently (Phase 3)

---

## 🧪 Testing Strategy

### Unit Tests
```go
// agent/feedback_test.go
func TestFeedbackBus_SendReceive(t *testing.T)
func TestFeedbackBus_Timeout(t *testing.T)
func TestFeedbackBus_MultipleHandlers(t *testing.T)

// agent/manager_agent_test.go
func TestHandleDependencyRequest_CreatesTask(t *testing.T)
func TestHandleDependencyRequest_BlocksOriginalTask(t *testing.T)
func TestHandleDependencyRequest_InvalidTaskID(t *testing.T)

// agent/test_agent_test.go
func TestCheckPreconditions_MissingTestFiles(t *testing.T)
func TestCheckPreconditions_TestFilesExist(t *testing.T)
```

### Integration Test (E2E)
```go
func TestE2E_MissingTestFiles(t *testing.T) {
	// Setup: Empty directory
	// User: "Run tests in /tmp/testdir"
	// Assert:
	//   - TASK-002 created (write tests)
	//   - TASK-001 blocked
	//   - TASK-002 completes
	//   - TASK-001 unblocked and succeeds
}
```

### Load Test
```go
func TestConcurrency_10AgentsSendFeedback(t *testing.T) {
	// 10 agents send 5 feedbacks each simultaneously
	// Assert: All 50 feedbacks processed, no lost messages
}
```

---

## ⚠️ Known Risks & Mitigations

### Risk 1: Deadlock with 2-Worker Semaphore
**Scenario:** Both workers blocked waiting for dependencies
**Mitigation:** Dependencies queued (not executed immediately), coordinator picks up when slot available
**Future:** Priority queue for dependencies OR reserve 1 slot for dependencies

### Risk 2: Circular Dependencies
**Scenario:** Task A requests Task B, Task B requests Task A
**Mitigation Phase 1:** Manual prevention (don't create circular deps)
**Mitigation Phase 2:** Add cycle detection in handleDependencyRequest

### Risk 3: Feedback Channel Overflow
**Scenario:** >100 feedbacks queued, channel full
**Mitigation:** Buffered channel (100), 100ms timeout on send
**Monitoring:** Log when Send() times out

### Risk 4: Agent Doesn't Retry After Unblock
**Scenario:** Task unblocked but agent doesn't know to retry
**Mitigation:** Coordinator polls for READY tasks continuously (existing behavior)
**How it works:** Task blocked → unblocked (status becomes READY) → coordinator assigns to agent → agent retries

---

## 📈 Success Metrics

**Phase 1 MVP:**
- ✅ TestAgent "run tests" scenario works end-to-end
- ✅ Zero "max iterations exceeded" errors for missing preconditions
- ✅ Feedback latency < 100ms
- ✅ No deadlocks in 100-task stress test

**Phase 2 Production:**
- ✅ Task success rate: 75% → 90%
- ✅ 80% of blocked tasks auto-unblock
- ✅ Avg retries per task: <2
- ✅ All agents have precondition checks

---

## 🔍 To Explore

### Concurrency & Performance
- **Semaphore strategy**: Reserve worker slot for dependencies? Priority queue? Dynamic worker pool?
- **Channel size**: 100 optimal? Measure actual usage
- **Handler parallelism**: Currently handlers run in goroutines - any risk of too many goroutines?

### State Management
- **Agent resume**: Currently agent retries from scratch - any cases where we need to preserve partial state?
- **Task metadata**: Use existing Metadata field or separate state table?
- **Context preservation**: How to pass "what agent learned" to retry?

### Error Recovery
- **Retry strategies**: Exponential backoff? Max retry count?
- **Context loading**: When compile fails, which files to load? All in project? Related files only?
- **Escalation**: After N retries, escalate to user? Different agent? Break into smaller tasks?

### Learning (Phase 4+)
- **Error patterns**: Store in separate table? Use LLM to match patterns vs. regex?
- **Pattern application**: Preemptive fixes vs. suggestions?
- **Cross-agent learning**: Share learned patterns? Artifact-based knowledge base?

### User Experience
- **Feedback visibility**: Show feedback to user? Internal only? Configurable?
- **Progress tracking**: How to communicate "waiting for dependency X"?
- **Manual intervention**: UI to manually unblock tasks? Provide context? Force retry?

### Scalability
- **Database transactions**: Add BEGIN/COMMIT for atomic operations?
- **Feedback persistence**: When to write to DB vs. memory only?
- **Large-scale**: 10 agents? 100 concurrent tasks? Channel bottleneck?

### Advanced Patterns
- **Proactive planning**: LLM predicts dependencies during decomposition?
- **Contract-based**: Tasks declare requires/produces upfront?
- **Checkpoint execution**: Explicit checkpoints vs. continuous loop?
- **Peer-to-peer**: Direct agent-agent communication vs. manager hub?

---

## 📚 Appendices

### Appendix A: Database Schema (Phase 2)

```sql
-- Optional: For feedback auditing and metrics
CREATE TABLE IF NOT EXISTS agent_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    feedback_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    context TEXT, -- JSON
    suggestion TEXT,
    created_at DATETIME NOT NULL,
    processed_at DATETIME,

    FOREIGN KEY (task_id) REFERENCES tasks(task_key) ON DELETE CASCADE
);

CREATE INDEX idx_feedback_task ON agent_feedback(task_id);
CREATE INDEX idx_feedback_type ON agent_feedback(feedback_type);
CREATE INDEX idx_feedback_created ON agent_feedback(created_at DESC);
```

### Appendix B: Code Locations Reference

```
go/agent/
├── feedback.go           [NEW] - FeedbackBus, types, handlers
├── base_agent.go         [MODIFY] - Add SendFeedback(), RequestDependency()
├── manager_agent.go      [MODIFY] - Add StartFeedbackProcessing(), handleDependencyRequest()
├── test_agent.go         [MODIFY] - Add checkPreconditions()
├── code_agent.go         [MODIFY - Phase 2] - Add precondition checks
├── queue.go              [MODIFY] - Fix CompleteTask() to always call UnblockDependentTasks()
└── task.go               [NO CHANGE] - Existing Metadata field can store agent state

main.go or init           [MODIFY] - Call manager.StartFeedbackProcessing(ctx)
```

### Appendix C: Feedback Flow Diagram

```
Agent Execution:
┌─────────────────────────────────────────────┐
│ 1. Agent.Execute(task)                      │
│    ├─ checkPreconditions()                  │
│    │  └─ Missing prereq? RequestDependency()│
│    │     └─ SendFeedback() → channel        │
│    │        [Agent returns error, task blocked]
│    │                                         │
│    ├─ [preconditions OK]                    │
│    ├─ Execute tools                         │
│    └─ Return result                         │
└─────────────────────────────────────────────┘
                    │
                    │ feedback sent
                    ▼
┌─────────────────────────────────────────────┐
│ FeedbackBus (channel)                       │
│  ├─ Buffered: 100                           │
│  ├─ Non-blocking send (100ms timeout)      │
│  └─ Async dispatch to handlers             │
└─────────────────────────────────────────────┘
                    │
                    │ routed
                    ▼
┌─────────────────────────────────────────────┐
│ Manager.handleDependencyRequest()           │
│  ├─ Parse feedback context                  │
│  ├─ Create new dependency task (TASK-N)    │
│  ├─ Block original task (depends on TASK-N)│
│  ├─ Mark TASK-N as READY                   │
│  └─ Queue for coordinator                  │
└─────────────────────────────────────────────┘
                    │
                    │ coordinator assigns
                    ▼
┌─────────────────────────────────────────────┐
│ 2. CodeAgent.Execute(dependency task)       │
│    └─ Creates required files/resources     │
└─────────────────────────────────────────────┘
                    │
                    │ completes
                    ▼
┌─────────────────────────────────────────────┐
│ Queue.CompleteTask()                        │
│    └─ UnblockDependentTasks()              │
│       └─ Original task: BLOCKED → READY   │
└─────────────────────────────────────────────┘
                    │
                    │ coordinator reassigns
                    ▼
┌─────────────────────────────────────────────┐
│ 3. Agent.Execute(original task - RETRY)    │
│    ├─ checkPreconditions() ✓ now pass     │
│    ├─ Execute tools                        │
│    └─ Success!                             │
└─────────────────────────────────────────────┘
```

---

**Status:** Ready to implement Phase 0 and Phase 1
**Estimated Effort:** Phase 0 (2h) + Phase 1 MVP (3-4h) = ~6 hours total
**Next Step:** Review with team, then start Phase 0 fixes