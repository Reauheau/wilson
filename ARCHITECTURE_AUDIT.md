# Wilson Architecture Audit - What Already Exists

## Executive Summary

**Discovery:** Wilson has a COMPLETE multi-agent task management system already built but NOT ACTIVATED. We have:
- ✅ Full task queue with SQLite persistence
- ✅ ManagedTask with DoR/DoD validation
- ✅ Manager Agent for orchestration
- ✅ Test Agent ready to use
- ✅ Review Agent with quality gates
- ✅ Dependency tracking and status management

**Current Problem:** ChatAgent bypasses this entire system and delegates directly to CodeAgent, causing the "forgot to create test file" issue.

---

## What We Have (Unused!)

### 1. Task Queue System (`queue.go` - 400+ lines)

**Fully Implemented Features:**
- SQLite-backed persistent task queue
- CRUD operations for tasks
- Task status tracking (NEW → READY → ASSIGNED → IN_PROGRESS → IN_REVIEW → DONE)
- Dependency management
- Parent/child task relationships
- Task filtering and queries

**Key Functions:**
```go
CreateTask(task *ManagedTask) error
GetTask(id int) (*ManagedTask, error)
UpdateTaskStatus(id int, status ManagedTaskStatus) error
GetTasksByStatus(status ManagedTaskStatus) ([]*ManagedTask, error)
GetTasksByAgent(agentName string) ([]*ManagedTask, error)
GetDependentTasks(taskKey string) ([]*ManagedTask, error)
GetBlockedTasks() ([]*ManagedTask, error)
```

**Current Usage:** ZERO - not connected to anything!

---

### 2. ManagedTask Type (`task.go` - 260+ lines)

**Comprehensive Task Model:**
```go
type ManagedTask struct {
    ID            int
    ParentTaskID  *int           // Subtask support
    TaskKey       string         // TASK-001 format
    Title         string
    Description   string
    Type          ManagedTaskType // code, test, review, research

    // Assignment
    AssignedTo    string
    AssignedAt    *time.Time

    // Status Management
    Status        ManagedTaskStatus
    Priority      int

    // Definition of Ready/Done
    DORCriteria   []string       // e.g., "Dependencies resolved"
    DORMet        bool
    DODCriteria   []string       // e.g., "Tests pass", "Code compiles"
    DODMet        bool

    // Dependencies
    DependsOn     []string       // Task keys this depends on
    Blocks        []string       // Task keys blocked by this

    // Results
    Result        string
    ArtifactIDs   []int          // Links to context store

    // Review
    ReviewStatus  ReviewStatus   // approved, needs_changes, rejected
    ReviewComments string
    Reviewer      string

    // Timestamps
    CreatedAt     time.Time
    StartedAt     *time.Time
    CompletedAt   *time.Time
}
```

**Built-in Methods:**
```go
IsReady() bool         // Checks DoR
IsDone() bool          // Checks DoD
CanStart() bool        // Checks if can begin
Assign(agent string)   // Assign to agent
Start() error          // Mark in progress
Complete(result string, artifacts []int) error
Block(reason string)   // Mark as blocked
Unblock()              // Remove blocked status
RequestReview(reviewer string)
```

**Current Usage:** ZERO - ManagedTask never instantiated!

---

### 3. Manager Agent (`manager_agent.go` - 400+ lines)

**Already Implements:**
```go
type ManagerAgent struct {
    name      string
    queue     *TaskQueue         // ✅ Has task queue
    db        *sql.DB
    agentPool map[string]ManagedAgentInfo  // ✅ Tracks agents
}

// Task Creation
CreateTask(ctx, title, desc, type) (*ManagedTask, error)
CreateSubtask(ctx, parentID, title, desc, type) (*ManagedTask, error)

// DoR/DoD Validation
ValidateAndMarkReady(ctx, taskID) error
ValidateDOD(ctx, taskID) error

// Task Assignment
AssignTask(ctx, taskID, agentName) error
ReassignTask(ctx, taskID, newAgent) error

// Dependency Management
CheckDependencies(ctx, taskID) error
UnblockDependentTasks(ctx, taskKey) error

// Status Management
UpdateTaskStatus(ctx, taskID, status) error
GetTaskStatus(ctx, taskID) (*ManagedTask, error)

// Communication
LogCommunication(ctx, fromAgent, messageType, content, taskKey) error
```

**Current Usage:** Manager Agent exists but ChatAgent never uses it!

---

### 4. Test Agent (`test_agent.go` - 220 lines)

**Fully Built Agent:**
- Inherits from BaseAgent
- Has comprehensive system prompt for test design
- Tools: write_file, modify_file, run_tests, coverage_check
- Anti-hallucination rules built into prompt
- Artifact storage for test results
- Communication with Review Agent

**System Prompt Highlights:**
```
YOU MUST ACTUALLY CREATE TEST FILES - NEVER JUST DESCRIBE TESTS!
✅ {"tool": "write_file", "arguments": {"path": "user_test.go", ...}}
✅ {"tool": "run_tests", "arguments": {"package": "user"}}
```

**Current Usage:** TestAgent created in main.go but never delegated tasks!

---

### 5. Review Agent (`review_agent.go` - 270 lines)

**Quality Assurance Agent:**
- Uses analysis model (qwen2.5:7b)
- Tools: read_file, lint_code, security_scan, complexity_check, code_review
- Review workflow: poll_tasks, claim_task, submit_review
- Integrated with quality validators

**System Prompt Highlights:**
```
ENDGAME Phase 3: Review workflow integration
- Automated quality gate checks
- Security scanning
- Code style enforcement
- Review submission with findings
```

**Current Usage:** ReviewAgent created but never assigned tasks!

---

### 6. DoR/DoD System (`dor_dod.go` - 330 lines)

**Validation Framework:**
```go
type DORValidator struct { task *ManagedTask }
type DODValidator struct { task *ManagedTask }

// DoR Checks
func (v *DORValidator) ValidateDescription() error
func (v *DORValidator) ValidateDependencies() error
func (v *DORValidator) ValidateResources() error
func (v *DORValidator) ValidateAcceptanceCriteria() error
func (v *DORValidator) MarkReady() error  // Runs all checks

// DoD Checks
func (v *DODValidator) ValidateFunctional() error
func (v *DODValidator) ValidateQuality() error
func (v *DODValidator) ValidateTesting() error
func (v *DODValidator) ValidateReview() error
func (v *DODValidator) MarkDone() error   // Runs all checks

// Default Criteria Setting
SetDefaultDORCriteria(task *ManagedTask)
SetDefaultDODCriteria(task *ManagedTask)
```

**Current Usage:** Code exists but validators never called!

---

### 7. Quality Validators (`quality_validators.go` - 390 lines)

**Automated Quality Checks:**
```go
type SecurityValidator struct {}
func (v *SecurityValidator) Validate(ctx, path) ([]Finding, error)
// Checks: SQL injection, XSS, hardcoded secrets, unsafe operations

type PerformanceValidator struct {}
func (v *PerformanceValidator) Validate(ctx, path) ([]Finding, error)
// Checks: N+1 queries, unbounded loops, memory leaks

type StyleValidator struct {}
func (v *StyleValidator) Validate(ctx, path) ([]Finding, error)
// Checks: naming conventions, complexity, documentation

type TestCoverageValidator struct {}
func (v *TestCoverageValidator) Validate(ctx, path) ([]Finding, error)
// Checks: coverage %, missing tests, edge cases
```

**Current Usage:** Validators exist but never invoked!

---

## Current vs. Intended Flow

### Current Flow (WRONG):

```
User: "Create app + tests"
  ↓
ChatAgent.HandleChat()
  ↓
ChatAgent.Execute() → Creates simple Task
  ↓
Delegates to CodeAgent directly
  ↓
CodeAgent tries to do EVERYTHING
  ↓
Forgets to create test file ❌
```

### Intended Flow (ENDGAME Architecture):

```
User: "Create app + tests"
  ↓
ChatAgent detects complex task
  ↓
Delegates to ManagerAgent
  ↓
ManagerAgent.CreateTask() → Creates ManagedTask in queue
  ↓
ManagerAgent decomposes into subtasks:
  ├─ TASK-001: Generate main.go [Code Agent]
  │   DoR: Spec clear ✓
  │   DoD: Compiles without errors
  ├─ TASK-002: Generate main_test.go [Code Agent]
  │   DoR: main.go complete
  │   DoD: Test file created
  ├─ TASK-003: Run tests [Test Agent]
  │   DoR: Test file exists
  │   DoD: All tests pass
  └─ TASK-004: Review code [Review Agent]
      DoR: Tests pass
      DoD: Quality gates pass, approved
  ↓
ManagerAgent assigns TASK-001 to CodeAgent
  ↓
CodeAgent.Execute(TASK-001)
  ↓
CodeAgent completes, marks DoD met
  ↓
ManagerAgent.ValidateDOD() → TASK-001 done ✓
  ↓
ManagerAgent assigns TASK-002 to CodeAgent
  ↓
... (continues for all subtasks)
  ↓
All tasks complete → Manager notifies ChatAgent
  ↓
ChatAgent reports to user: "All files created, tests passing!"
```

---

## Why Current Approach Fails

### Problem 1: No Task Decomposition
```go
// Current: ChatAgent creates ONE task
task := &Task{
    Description: "Create app + tests", // Too complex for one agent!
}
agent.Execute(ctx, task)  // CodeAgent tries to remember everything
```

**Solution:** Use ManagerAgent to decompose:
```go
// Proper: Manager creates SUBTASKS
parentTask := manager.CreateTask(ctx, "Build app", desc, ManagedTaskTypeCode)
manager.CreateSubtask(ctx, parentTask.ID, "Generate main.go", ...)
manager.CreateSubtask(ctx, parentTask.ID, "Generate tests", ...)
manager.CreateSubtask(ctx, parentTask.ID, "Run tests", ...)
```

### Problem 2: No State Persistence
```go
// Current: Task exists only in memory during execution
// If CodeAgent forgets something, it's lost
```

**Solution:** Task Queue persists to SQLite:
```go
// Proper: Tasks saved to database
queue.CreateTask(task)  // Persisted!
// Agent can check: "What am I supposed to do?"
queue.GetTasksByStatus(ManagedTaskStatusReady)
```

### Problem 3: No Quality Gates
```go
// Current: CodeAgent says "done" but who validates?
result.Success = true  // Self-reported!
```

**Solution:** DoD Validation:
```go
// Proper: Manager validates DoD before marking complete
validator := NewDODValidator(task)
if err := validator.ValidateFunctional(); err != nil {
    // Task NOT complete until DoD met
}
if err := validator.ValidateTesting(); err != nil {
    // Tests must pass
}
task.Complete(result, artifactIDs)  // Only after validation
```

### Problem 4: Wrong Agent for Job
```go
// Current: CodeAgent trying to run tests
// But CodeAgent is for CODE GENERATION, not execution!
```

**Solution:** Specialist Agents:
```go
// Proper: Right agent for right job
manager.AssignTask(TASK-001, "Code")    // Generate code
manager.AssignTask(TASK-002, "Test")    // Run tests
manager.AssignTask(TASK-003, "Review")  // Quality check
```

---

## Implementation Path

### Phase 1: Connect Manager Agent (2 hours)

**Files to modify:**
1. `chat_agent.go` - Detect complex tasks, delegate to Manager
2. `main.go` - Initialize Manager Agent with task queue DB
3. `coordinator.go` - Wire Manager into coordinator

**Changes:**
```go
// In ChatAgent.Execute()
if isComplexTask(task) {
    return m.delegateToManager(ctx, task)  // NEW!
} else {
    return m.delegateToSpecialist(ctx, task)  // Existing
}

// In main.go
db, _ := sql.Open("sqlite3", "wilson_tasks.db")
manager := agent.NewManagerAgent(db)
coordinator.SetManager(manager)  // NEW!
```

### Phase 2: Task Decomposition (2 hours)

**Add to ManagerAgent:**
```go
func (m *ManagerAgent) DecomposeTask(ctx context.Context, userRequest string) ([]*ManagedTask, error) {
    // Use LLM to analyze request and create subtasks
    systemPrompt := "You are a task decomposition specialist..."

    // Call LLM to break down task
    subtasks := llm.Generate(systemPrompt, userRequest)

    // Create subtasks in queue
    for _, sub := range subtasks {
        task := NewManagedTask(sub.Title, sub.Desc, sub.Type)
        SetDefaultDORCriteria(task)
        SetDefaultDODCriteria(task)
        m.queue.CreateTask(task)
    }

    return subtasks, nil
}
```

### Phase 3: Sequential Execution (1 hour)

**Add to ManagerAgent:**
```go
func (m *ManagerAgent) ExecuteTaskPlan(ctx context.Context, parentTaskID int) error {
    // Get all subtasks
    subtasks := m.queue.GetSubtasks(parentTaskID)

    for _, task := range subtasks {
        // Wait for dependencies
        m.CheckDependencies(ctx, task.ID)

        // Assign to appropriate agent
        agent := m.getAgentForType(task.Type)

        // Execute
        result, err := agent.Execute(ctx, convertToSimpleTask(task))
        if err != nil {
            task.Status = ManagedTaskStatusFailed
            return err
        }

        // Validate DoD
        validator := NewDODValidator(task)
        if err := validator.MarkDone(); err != nil {
            return err
        }

        // Mark complete
        task.Complete(result.Output, extractArtifactIDs(result))
        m.queue.UpdateTask(task)

        // Unblock dependent tasks
        m.UnblockDependentTasks(ctx, task.TaskKey)
    }

    return nil
}
```

### Phase 4: Activate Test & Review Agents (1 hour)

**Wire into Coordinator:**
```go
// In coordinator.go
func (c *Coordinator) getAgentForType(taskType ManagedTaskType) Agent {
    switch taskType {
    case ManagedTaskTypeCode:
        return c.codeAgent
    case ManagedTaskTypeTest:
        return c.testAgent    // NOW USED!
    case ManagedTaskTypeReview:
        return c.reviewAgent  // NOW USED!
    case ManagedTaskTypeResearch:
        return c.researchAgent
    default:
        return c.chatAgent
    }
}
```

---

## Success Metrics After Implementation

### Test 1: Simple Task (Baseline)
```
Input: "Create calculator.go"
Expected: 1 task → Code Agent → 1 file ✓
Current: WORKS
After: Still works (no change for simple tasks)
```

### Test 2: Multi-File Task
```
Input: "Create app + test file"
Expected: 2 tasks → Code Agent (x2) → 2 files ✓
Current: FAILS - only creates 1 file
After: WORKS - Manager creates 2 subtasks, both executed
```

### Test 3: Full Workflow
```
Input: "Build CLI tool with tests and review"
Expected: 4 tasks → Code + Test + Review → Complete ✓
Current: FAILS - only generates main file
After: WORKS - Manager orchestrates all agents
```

### Test 4: Complex Dependencies
```
Input: "Refactor auth module, add tests, ensure security"
Expected: 5 tasks with dependencies → All agents → Production ready ✓
Current: FAILS - too complex for single agent
After: WORKS - Manager handles dependencies, quality gates
```

---

## Key Insights

1. **We don't need to BUILD the system** - it's already built!
2. **We need to CONNECT the system** - wire Manager into ChatAgent
3. **The architecture is CORRECT** - exactly as ENDGAME specifies
4. **Current approach BYPASSES the architecture** - that's why it fails

**Bottom Line:** Stop trying to make CodeAgent smarter. Start using the Manager Agent that's already built for this exact purpose.

---

## Next Steps

1. **Remove debug logs** - clean up current implementation
2. **Implement Phase 1** - Connect ChatAgent → Manager Agent
3. **Test decomposition** - Verify Manager creates subtasks correctly
4. **Activate specialists** - TestAgent and ReviewAgent start receiving tasks
5. **Add quality gates** - DoR/DoD validation enforced
6. **Celebrate** - Wilson now works as designed! 🎉

---

**Files Ready to Use:**
- ✅ `queue.go` - Full task queue implementation
- ✅ `task.go` - ManagedTask with DoR/DoD
- ✅ `manager_agent.go` - Orchestration logic
- ✅ `test_agent.go` - Test specialist
- ✅ `review_agent.go` - Quality assurance
- ✅ `dor_dod.go` - Validation framework
- ✅ `quality_validators.go` - Automated checks

**What's Missing:**
- ❌ Connection from ChatAgent to ManagerAgent
- ❌ Task decomposition logic in ManagerAgent
- ❌ Sequential execution in ManagerAgent

**Estimated Time to Complete:** 6-8 hours

**Expected Result:** Wilson handles complex multi-file tasks reliably, as designed in ENDGAME.md
