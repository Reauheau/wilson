# Wilson Feedback Loop Architecture V2 - TaskContext-Enabled

**Status:** Ready for Implementation
**Version:** 2.0 (TaskContext-Enabled)
**Created:** 2025-10-22
**Last Updated:** 2025-10-22
**Purpose:** Self-healing and adaptive feedback system built on TaskContext foundation

---

## 🎯 Vision & Goals

Transform Wilson into a **self-improving, self-healing system** where:
- Agents detect failures and blockers automatically with **full context**
- Agents request missing dependencies with **error history awareness**
- Manager dynamically adjusts task plans based on **structured feedback data**
- System learns from errors using **ExecutionError patterns**
- Success rate improves from ~75% to >95%

**Key Breakthrough:** TaskContext provides rich execution state, eliminating blind retries and enabling intelligent recovery!

---

## 🆕 What's New in V2

### TaskContext Foundation (Implemented ✅)

```go
type TaskContext struct {
    // Identity
    TaskID   string
    TaskKey  string

    // Execution parameters
    ProjectPath string                 // Pre-extracted, no parsing needed!
    Input       map[string]interface{}

    // Dependencies & relationships
    DependsOn        []string   // Task keys this depends on
    DependencyFiles  []string   // Files created by dependencies
    DependencyErrors []string   // Errors from dependencies to learn from

    // Feedback context (CRITICAL for learning!)
    PreviousAttempts int
    PreviousErrors   []ExecutionError  // Rich error history!

    // Artifacts & history
    CreatedFiles  []string
    ModifiedFiles []string
}

type ExecutionError struct {
    Timestamp   time.Time
    Agent       string
    Phase       string        // "precondition", "execution", "verification"
    ErrorType   string        // "missing_file", "compile_error", "path_error"
    Message     string
    FilePath    string        // Where error occurred
    LineNumber  int           // Precise location
    CodeSnippet string        // Context
    Suggestion  string        // How to fix
    Metadata    map[string]interface{} // Extensible
}
```

**Key Methods Available:**
- `AddError(err ExecutionError)` - Record failures
- `ShouldRetry(maxAttempts int) bool` - Smart retry logic
- `GetErrorPatterns() []string` - Detect recurring issues
- `GetLastError() *ExecutionError` - Access most recent failure

**All agents already support:**
- `ExecuteWithContext(ctx context.Context, taskCtx *TaskContext)` - Main entry point
- `BaseAgent.currentContext *TaskContext` - Access during execution

---

## 📊 Current State Analysis

### ✅ Working Components

1. **TaskContext Infrastructure** (NEW - IMPLEMENTED)
   - All agents use ExecuteWithContext
   - AgentExecutor receives TaskContext for path extraction
   - ManagerAgent injects dependency context
   - Error tracking ready for feedback loop

2. **Task Queue** (`agent/queue.go`)
   - SQLite persistence
   - Full lifecycle: NEW → READY → ASSIGNED → IN_PROGRESS → DONE/FAILED
   - Dependency tracking via `DependsOn` and `Blocks` fields
   - ⚠️ **Gap:** `UnblockDependentTasks()` exists but not called consistently

3. **Manager Agent** (`agent/manager_agent.go`)
   - Task decomposition and execution
   - `injectDependencyContext()` - loads files/errors from dependencies
   - ⚠️ **Gap:** No feedback monitoring, no dynamic task insertion

4. **Agent Executor** (`agent/agent_executor.go`)
   - Auto-injection workflow: generate_code → write_file → compile
   - Uses TaskContext.ProjectPath (no more broken path parsing!)
   - ⚠️ **Gap:** No error recording to TaskContext

### ❌ Missing Components (To Be Implemented)

1. **FeedbackBus** - Event-driven communication channel
2. **AgentFeedback** - Enhanced with TaskContext
3. **Precondition Checks** - Context-aware validation
4. **Error Recording** - Capture failures in TaskContext.PreviousErrors
5. **Smart Retry** - Use error patterns for intelligent recovery

---

## 🏗️ Architecture Design

### Event-Driven Feedback Flow with TaskContext

```
┌─────────────────────────────────────────────────────────────┐
│                         USER                                 │
└────────────┬────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│                   MANAGER AGENT                              │
│  • Creates TaskContext with full execution state             │
│  • Monitors feedback channel (event-driven)                  │
│  • Dynamically creates dependency tasks                      │
│  • Uses error patterns for smart decisions                   │
└─────┬───────────────────────┬───────────────────────────────┘
      │                       │
      │ feedback chan         │ task assignment
      │ (with TaskContext)    │ (ExecuteWithContext)
      ▼                       ▼
┌──────────────────┐    ┌─────────────────────────────────────┐
│ FEEDBACK BUS     │    │  WORKER AGENTS (Code, Test, Review) │
│ (Go channel)     │◄───┤  • Check preconditions with context │
│                  │    │  • Execute with tools                │
│ Buffered: 100    │    │  • Record errors in TaskContext     │
│                  │    │  • Send feedback with full context  │
└──────────────────┘    └─────────────────────────────────────┘
```

**Key Enhancement:** Feedback now carries TaskContext, enabling:
- Manager sees error history before creating dependency
- Manager can check if retry makes sense (`ShouldRetry()`)
- Manager can analyze error patterns (`GetErrorPatterns()`)
- Agents can learn from dependency errors (`DependencyErrors`)

---

## 🔧 Implementation Specification

### Phase 0: Foundation Fixes (PREREQUISITE - 1 hour)

#### Fix 1: Auto-Unblock on Task Completion

**File:** `agent/queue.go` - Modify `CompleteTask` method

```go
func (q *TaskQueue) CompleteTask(taskID int, result string, artifactIDs []int) error {
	task, err := q.GetTask(taskID)
	if err != nil {
		return err
	}

	if err := task.Complete(result, artifactIDs); err != nil {
		return err
	}

	if err := q.UpdateTask(task); err != nil {
		return err
	}

	// ✅ ADD: Always unblock dependents when task completes
	if err := q.UnblockDependentTasks(task.TaskKey); err != nil {
		// Log but don't fail - unblocking is non-critical
		fmt.Printf("Warning: Failed to unblock dependents of %s: %v\n", task.TaskKey, err)
	}

	return nil
}
```

**Test:** Task A blocks on Task B → Task B completes → Task A automatically unblocked

---

### Phase 1: Minimal Viable Feedback (MVP - 4 hours)

**Goal:** Prove concept with TaskContext-aware feedback working end-to-end

#### Component 1: Enhanced Feedback Types

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
	Context      map[string]interface{} `json:"context"`      // Additional context
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
```

#### Component 2: BaseAgent Feedback Methods with TaskContext

**File:** `agent/base_agent.go` (ADD TO EXISTING)

```go
// SendFeedback sends feedback via the feedback bus with full TaskContext
func (a *BaseAgent) SendFeedback(ctx context.Context, feedbackType FeedbackType,
	severity FeedbackSeverity, message string,
	context map[string]interface{}, suggestion string) error {

	bus := GetFeedbackBus()

	feedback := &AgentFeedback{
		TaskID:       a.currentTaskID,
		AgentName:    a.name,
		FeedbackType: feedbackType,
		Severity:     severity,
		Message:      message,
		Context:      context,
		Suggestion:   suggestion,
		TaskContext:  a.currentContext, // ✅ Full execution context!
	}

	return bus.Send(feedback)
}

// RequestDependency requests a missing dependency with error context
func (a *BaseAgent) RequestDependency(ctx context.Context, description string,
	taskType ManagedTaskType, reason string) error {

	// Include error history in context
	errorInfo := make(map[string]interface{})
	if a.currentContext != nil {
		errorInfo["previous_attempts"] = a.currentContext.PreviousAttempts
		errorInfo["error_patterns"] = a.currentContext.GetErrorPatterns()
		if lastErr := a.currentContext.GetLastError(); lastErr != nil {
			errorInfo["last_error_type"] = lastErr.ErrorType
			errorInfo["last_error_message"] = lastErr.Message
		}
	}

	errorInfo["dependency_description"] = description
	errorInfo["dependency_type"] = string(taskType)
	errorInfo["reason"] = reason

	return a.SendFeedback(ctx,
		FeedbackTypeDependencyNeeded,
		FeedbackSeverityCritical,
		fmt.Sprintf("Cannot proceed: %s", reason),
		errorInfo,
		"Create and complete the missing dependency before retrying this task",
	)
}

// RecordError records an error in TaskContext for learning
func (a *BaseAgent) RecordError(errorType, phase, message, filePath string,
	lineNumber int, suggestion string) {

	if a.currentContext == nil {
		return
	}

	execError := ExecutionError{
		Timestamp:  time.Now(),
		Agent:      a.name,
		Phase:      phase,
		ErrorType:  errorType,
		Message:    message,
		FilePath:   filePath,
		LineNumber: lineNumber,
		Suggestion: suggestion,
	}

	a.currentContext.AddError(execError)
}
```

#### Component 3: Manager Handler with TaskContext Intelligence

**File:** `agent/manager_agent.go` (ADD TO EXISTING)

```go
// StartFeedbackProcessing registers handlers and starts the feedback bus
func (m *ManagerAgent) StartFeedbackProcessing(ctx context.Context) {
	bus := GetFeedbackBus()

	// Register handlers
	bus.RegisterHandler(FeedbackTypeDependencyNeeded, m.handleDependencyRequest)
	bus.RegisterHandler(FeedbackTypeRetryRequest, m.handleRetryRequest)

	// Start processing
	bus.Start(ctx)

	fmt.Println("[ManagerAgent] Feedback processing started")
}

// handleDependencyRequest creates missing dependency task using TaskContext intelligence
func (m *ManagerAgent) handleDependencyRequest(ctx context.Context, feedback *AgentFeedback) error {
	fmt.Printf("[ManagerAgent] Processing dependency request from %s for task %s\n",
		feedback.AgentName, feedback.TaskID)

	// ✅ SMART DECISION: Check if we should create dependency or just retry
	if feedback.TaskContext != nil {
		// If too many retries, might be a different issue
		if !feedback.TaskContext.ShouldRetry(3) {
			fmt.Printf("[ManagerAgent] Task %s has %d attempts, escalating instead of creating dependency\n",
				feedback.TaskID, feedback.TaskContext.PreviousAttempts)
			return m.escalateToUser(ctx, feedback)
		}

		// Check error patterns
		patterns := feedback.TaskContext.GetErrorPatterns()
		if len(patterns) > 0 {
			fmt.Printf("[ManagerAgent] Detected error patterns for %s: %v\n",
				feedback.TaskID, patterns)
		}
	}

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

	// Parse task ID
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

	// ✅ COPY FULL CONTEXT from current task (including project path!)
	depTask.Input = make(map[string]interface{})
	for k, v := range currentTask.Input {
		depTask.Input[k] = v
	}

	// ✅ ADD ERROR CONTEXT for dependency to learn from
	if feedback.TaskContext != nil {
		if lastErr := feedback.TaskContext.GetLastError(); lastErr != nil {
			depTask.Input["trigger_error"] = map[string]interface{}{
				"type":    lastErr.ErrorType,
				"message": lastErr.Message,
				"file":    lastErr.FilePath,
			}
		}
	}

	SetDefaultDORCriteria(depTask)
	SetDefaultDODCriteria(depTask)

	// Create dependency and block current task
	if err := m.queue.CreateTask(depTask); err != nil {
		return fmt.Errorf("failed to create dependency task: %w", err)
	}

	currentTask.DependsOn = append(currentTask.DependsOn, depTask.TaskKey)
	currentTask.Block(fmt.Sprintf("Waiting for prerequisite: %s", depTask.TaskKey))

	if err := m.queue.UpdateTask(currentTask); err != nil {
		return fmt.Errorf("failed to update current task: %w", err)
	}

	fmt.Printf("[ManagerAgent] ✓ Created dependency %s → blocks %s\n", depTask.TaskKey, currentTask.TaskKey)

	// Mark dependency as ready
	if err := m.ValidateAndMarkReady(ctx, depTask.ID); err != nil {
		return fmt.Errorf("failed to mark dependency ready: %w", err)
	}

	fmt.Printf("[ManagerAgent] Dependency task %s queued for execution\n", depTask.TaskKey)

	return nil
}

// handleRetryRequest processes retry requests based on error patterns
func (m *ManagerAgent) handleRetryRequest(ctx context.Context, feedback *AgentFeedback) error {
	// Smart retry logic based on TaskContext
	if feedback.TaskContext != nil {
		patterns := feedback.TaskContext.GetErrorPatterns()
		if len(patterns) > 0 {
			// Same error repeating - don't just retry, adjust strategy
			fmt.Printf("[ManagerAgent] Retry with adjusted strategy for patterns: %v\n", patterns)
		}
	}

	// Unblock and let coordinator retry
	var taskID int
	fmt.Sscanf(feedback.TaskID, "TASK-%d", &taskID)
	if taskID > 0 {
		return m.UnblockTask(ctx, taskID)
	}

	return nil
}

// escalateToUser escalates to user when automatic recovery fails
func (m *ManagerAgent) escalateToUser(ctx context.Context, feedback *AgentFeedback) error {
	fmt.Printf("\n⚠️  ESCALATION NEEDED ⚠️\n")
	fmt.Printf("Task: %s\n", feedback.TaskID)
	fmt.Printf("Agent: %s\n", feedback.AgentName)
	fmt.Printf("Issue: %s\n", feedback.Message)

	if feedback.TaskContext != nil {
		fmt.Printf("Attempts: %d\n", feedback.TaskContext.PreviousAttempts)
		patterns := feedback.TaskContext.GetErrorPatterns()
		if len(patterns) > 0 {
			fmt.Printf("Error patterns: %v\n", patterns)
		}
	}

	fmt.Printf("Suggestion: %s\n\n", feedback.Suggestion)

	// For now, just log. Phase 2 could add user interaction
	return nil
}
```

#### Component 4: TestAgent Context-Aware Preconditions

**File:** `agent/test_agent.go` (MODIFY Execute method)

```go
func (a *TestAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
	result := &Result{
		TaskID: task.ID,
		Agent:  a.name,
	}

	// ✅ CONTEXT-AWARE PRECONDITION CHECK
	if err := a.checkPreconditions(ctx, task); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Precondition failed: %v", err)

		// Record error in TaskContext for learning
		a.RecordError("precondition_failed", "precondition", err.Error(), "", 0,
			"Ensure prerequisites are met before running tests")

		return result, err
	}

	// ... rest of existing Execute logic ...
}

// checkPreconditions validates prerequisites with TaskContext awareness
func (a *TestAgent) checkPreconditions(ctx context.Context, task *Task) error {
	lowerDesc := strings.ToLower(task.Description)

	// Check: "run tests" requires test files to exist
	if strings.Contains(lowerDesc, "run test") || strings.Contains(lowerDesc, "execute test") {
		projectPath := "."

		// ✅ Use TaskContext if available (better than parsing task.Input)
		if a.currentContext != nil && a.currentContext.ProjectPath != "" {
			projectPath = a.currentContext.ProjectPath
		} else if pathVal, ok := task.Input["project_path"]; ok {
			if pathStr, ok := pathVal.(string); ok {
				projectPath = pathStr
			}
		}

		// ✅ SMART: Check DependencyFiles first (we know what was created!)
		if a.currentContext != nil && len(a.currentContext.DependencyFiles) > 0 {
			hasTestFiles := false
			for _, file := range a.currentContext.DependencyFiles {
				if strings.Contains(file, "_test.go") || strings.Contains(file, "_test.") {
					hasTestFiles = true
					break
				}
			}

			if hasTestFiles {
				// Dependencies created test files - we're good!
				return nil
			}
		}

		// Fallback: Check filesystem
		testFiles, err := filepath.Glob(filepath.Join(projectPath, "*_test.go"))
		if err != nil || len(testFiles) == 0 {
			// ✅ SMART: Include context in dependency request
			reason := fmt.Sprintf("No test files found in %s", projectPath)

			// Check if we already tried this before (from error history)
			if a.currentContext != nil {
				if lastErr := a.currentContext.GetLastError(); lastErr != nil {
					if lastErr.ErrorType == "missing_test_files" {
						reason += " (previous attempt also failed - check if code files exist)"
					}
				}
			}

			return a.RequestDependency(ctx,
				fmt.Sprintf("Create test files in %s", projectPath),
				ManagedTaskTypeCode,
				reason,
			)
		}
	}

	return nil
}
```

#### Component 5: Error Recording in AgentExecutor

**File:** `agent/agent_executor.go` (ENHANCE auto-injection error handling)

```go
// In the compile auto-injection section (around line 244):

compileResult, compileErr := ate.executor.Execute(ctx, compileCall)
result.ToolsExecuted = append(result.ToolsExecuted, "compile")

if compileErr != nil {
	// ✅ RECORD ERROR IN TASKCONTEXT for learning
	if ate.taskContext != nil {
		// Extract compile error details
		errorMsg := compileErr.Error()

		// Try to extract file and line number from compile error
		// Go format: "file.go:10:5: error message"
		filePath := targetPath
		lineNumber := 0

		parts := strings.Split(errorMsg, ":")
		if len(parts) >= 3 {
			lineNumber, _ = strconv.Atoi(parts[1])
		}

		ate.taskContext.AddError(ExecutionError{
			Timestamp:   time.Now(),
			Agent:       "AgentExecutor",
			Phase:       "compilation",
			ErrorType:   "compile_error",
			Message:     errorMsg,
			FilePath:    filePath,
			LineNumber:  lineNumber,
			CodeSnippet: "", // Could extract from file
			Suggestion:  "Fix compilation errors in generated code",
		})
	}

	result.Error = fmt.Sprintf("Auto-injected compile failed: %v", compileErr)
	return result, fmt.Errorf("auto-injected compile failed: %w", compileErr)
}
```

#### Component 6: Initialization

**File:** `main.go` (Modify initializeAgentSystem)

```go
func initializeAgentSystem(llmMgr *llm.Manager, contextMgr *contextpkg.Manager) *agent.ChatAgent {
	if llmMgr == nil || contextMgr == nil {
		return nil
	}

	// ... existing agent creation ...

	// Initialize Manager Agent with task queue
	db := contextMgr.GetDB()
	if db != nil {
		managerAgent := agent.NewManagerAgent(db)
		managerAgent.SetLLMManager(llmMgr)
		managerAgent.SetRegistry(agentRegistry)

		// ✅ START FEEDBACK PROCESSING
		managerAgent.StartFeedbackProcessing(context.Background())

		agent.SetGlobalManager(managerAgent)
	}

	// ... rest of existing code ...

	return chatAgent
}
```

---

### Phase 1 MVP Test Scenario

**Test: Missing Test Files with TaskContext Intelligence**

```
User: "Run tests in ~/IdeaProjects/wilsontestdir"

Expected Flow:
1. ManagerAgent creates TASK-001 (type: test)
   └─ TaskContext: ProjectPath = "/Users/.../wilsontestdir"

2. TestAgent.ExecuteWithContext(TASK-001, taskCtx)
   └─ checkPreconditions() with TaskContext

3. TestAgent detects: No test files in DependencyFiles
   └─ RequestDependency("Create test files")
   └─ Feedback includes:
       • TaskContext with empty DependencyFiles
       • ProjectPath for dependency to use
       • Error history (first attempt, no patterns yet)

4. ManagerAgent.handleDependencyRequest():
   ├─ Checks feedback.TaskContext.ShouldRetry(3) → true (first attempt)
   ├─ Creates TASK-002 (type: code)
   ├─ Copies ProjectPath to TASK-002.Input
   ├─ Blocks TASK-001 (depends on TASK-002)
   └─ Marks TASK-002 as READY

5. CodeAgent executes TASK-002
   ├─ TaskContext: ProjectPath = "/Users/.../wilsontestdir" ✓
   ├─ Creates test files
   └─ TaskContext.CreatedFiles = ["main_test.go"]

6. TASK-002 completes
   └─ UnblockDependentTasks() → TASK-001 becomes READY

7. TestAgent retries TASK-001
   ├─ New TaskContext includes:
   │  • DependencyFiles = ["main_test.go"] ✓
   │  • DependencyErrors = [] (no errors from dependency)
   ├─ checkPreconditions() sees test files in DependencyFiles
   └─ Executes tests successfully ✓

Result: SUCCESS - No max iterations, intelligent recovery!
```

**Success Criteria:**
- ✅ Feedback includes full TaskContext
- ✅ Manager uses error patterns for decision-making
- ✅ Dependency receives correct ProjectPath
- ✅ Retry knows what files were created
- ✅ No blind retries - context-aware at every step

---

## 📈 Success Metrics

**Phase 1 MVP:**
- ✅ TestAgent "run tests" scenario works with TaskContext
- ✅ Zero "max iterations" for precondition failures
- ✅ Feedback includes error patterns
- ✅ Dependencies inherit correct context
- ✅ Retries are context-aware (check DependencyFiles)

**Phase 2 Goals:**
- Task success rate: 75% → 90%
- 80% of blocked tasks auto-unblock
- Avg retries per task: <2
- Error patterns detected in 95% of repeated failures

---

## 🎯 Key Improvements Over V1

### 1. **Smart Decisions, Not Blind Retries**

**V1:** Manager creates dependency on any request
**V2:** Manager checks `ShouldRetry()`, analyzes `GetErrorPatterns()`, escalates after repeated failures

### 2. **Context Inheritance**

**V1:** Dependency gets generic description
**V2:** Dependency gets full Input map including ProjectPath, trigger error details

### 3. **Intelligent Preconditions**

**V1:** Check filesystem existence
**V2:** Check DependencyFiles first (we know what was created!), fallback to filesystem

### 4. **Learning-Ready**

**V1:** No error tracking
**V2:** Every error recorded in TaskContext with file, line, type, suggestion

### 5. **Escalation Path**

**V1:** Infinite retry loops
**V2:** Escalate to user after pattern threshold exceeded

---

## 🚧 Phase 2: Production Features (Future)

After MVP proven:

1. **Database Persistence**
   - Store AgentFeedback in `agent_feedback` table
   - Query feedback history per task
   - Analytics: feedback count by type, resolution time

2. **More Feedback Types**
   - `FeedbackTypeContextNeeded`: Request file loading
   - `FeedbackTypeBlocker`: Unrecoverable errors
   - `FeedbackTypeSuccess`: Positive signals

3. **Advanced Error Analysis**
   - LLM-powered error pattern matching
   - Cross-file error correlation
   - Proactive error prevention

4. **User Interaction**
   - UI for escalated issues
   - Manual dependency provision
   - Context editing for retries

---

## 🔍 Testing Strategy

### Unit Tests

```go
// agent/feedback_test.go
func TestFeedbackBus_SendWithTaskContext(t *testing.T)
func TestManagerAgent_SmartRetryDecision(t *testing.T)
func TestTestAgent_DependencyFilesCheck(t *testing.T)

// agent/task_context_test.go
func TestTaskContext_GetErrorPatterns(t *testing.T)
func TestTaskContext_ShouldRetry(t *testing.T)
```

### Integration Test (E2E)

```go
func TestE2E_MissingTestFiles_WithContext(t *testing.T) {
	// Setup: Empty directory
	// Execute: "Run tests in /tmp/testdir"
	// Verify:
	//   - Feedback includes TaskContext
	//   - Dependency inherits ProjectPath
	//   - Retry sees DependencyFiles
	//   - Task succeeds
}
```

---

## 📋 Implementation Checklist

### Phase 0 (1 hour)
- [ ] Fix `queue.CompleteTask()` to call `UnblockDependentTasks()`
- [ ] Test auto-unblock flow

### Phase 1 MVP (4 hours)
- [ ] Create `agent/feedback.go` with TaskContext support
- [ ] Add feedback methods to `base_agent.go` (SendFeedback, RecordError)
- [ ] Implement `manager_agent.go` handlers with smart decisions
- [ ] Add context-aware preconditions to `test_agent.go`
- [ ] Add error recording to `agent_executor.go`
- [ ] Start feedback bus in `main.go`
- [ ] Write unit tests
- [ ] Run E2E test: "Run tests in empty directory"

### Phase 1 Validation
- [ ] Verify feedback includes TaskContext
- [ ] Verify manager checks error patterns
- [ ] Verify dependency inherits ProjectPath
- [ ] Verify retry sees DependencyFiles
- [ ] Zero "max iterations" errors

---

## 📚 Code Locations Reference

```
go/agent/
├── task_context.go        [DONE] - TaskContext, ExecutionError, helper methods
├── feedback.go            [NEW] - FeedbackBus, AgentFeedback with TaskContext
├── base_agent.go          [MODIFY] - Add SendFeedback(), RecordError()
├── manager_agent.go       [MODIFY] - Add handlers with smart decisions
├── test_agent.go          [MODIFY] - Context-aware preconditions
├── agent_executor.go      [MODIFY] - Record errors in TaskContext
├── queue.go               [FIX] - CompleteTask() calls UnblockDependentTasks()
└── base_agent.go          [DONE] - currentContext field already exists

main.go                    [MODIFY] - Call manager.StartFeedbackProcessing()
```

---

**Status:** Ready for Phase 0 and Phase 1 implementation
**Estimated Effort:** Phase 0 (1h) + Phase 1 (4h) = 5 hours total
**Next Step:** Start with Phase 0 queue fix, then implement feedback.go
