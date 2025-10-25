# Review Agent - Implementation Gaps & Improvement Plan

**Status:** Analysis Complete
**Date:** October 23, 2025
**Author:** Claude Code Analysis

---

## 📋 Executive Summary

The Review Agent is **70% complete** with solid foundations but critical workflow gaps. All quality gate tools exist and work correctly, but the agent doesn't actually use them in the intended automated workflow. This document outlines 6 major gaps and provides actionable solutions.

**Key Finding:** The agent stores reviews as text artifacts instead of using the fully-implemented `get_review_status` and `submit_review` workflow tools.

**Impact:** Reviews are stored inconsistently, don't update task status properly, and don't integrate with the feedback loop for handling blockers.

---

## ✅ Current Implementation Strengths

### 1. **Solid Foundation** (`go/agent/review_agent.go`)

The Review Agent has excellent architecture:

- **Location:** `go/agent/review_agent.go:13-55`
- Extends `BaseAgent` with review-specific capabilities
- Uses `llm.PurposeAnalysis` model (appropriate for critical thinking)
- Handles tasks of type `"review"`
- Integrated with TaskContext for rich execution state

### 2. **Complete Tool Arsenal** (`review_agent.go:22-49`)

All necessary tools are available:

**File Operations:**
- `read_file`, `search_files`, `list_files`

**Context Operations:**
- `search_artifacts`, `retrieve_context`, `store_artifact`, `leave_note`

**Quality Gate Tools (All Implemented ✅):**
- `compile` - Build verification
- `format_code` - Code formatting checks
- `lint_code` - Style and best practices
- `security_scan` - Vulnerability detection
- `complexity_check` - Code complexity analysis
- `coverage_check` - Test coverage verification
- `code_review` - Orchestrates all quality checks

**Review Workflow Tools (Fully Functional ✅):**
- `get_review_status` - Query review records from database
- `submit_review` - Update review status, unblock tasks, send notifications

**Orchestration Tools:**
- `poll_tasks`, `claim_task`, `update_task_progress`, etc.

### 3. **Precondition Checks** (`review_agent.go:129-139`)

Well-designed validation:
```go
func (a *ReviewAgent) checkPreconditions(ctx context.Context, task *Task) error {
    // Check: Has code to review in DependencyFiles
    if a.currentContext != nil && len(a.currentContext.DependencyFiles) == 0 {
        return a.RequestDependency(ctx,
            "Create code to review",
            ManagedTaskTypeCode,
            "No code artifacts found to review")
    }
    return nil
}
```

**Strengths:**
- Validates code exists in `DependencyFiles` before reviewing
- Sends feedback via `RequestDependency()` when blocked
- Integrated with feedback loop architecture

### 4. **Excellent System Prompt** (`review_agent.go:141-259`)

The system prompt is comprehensive and well-structured:

**Anti-Hallucination Rules:**
```
❌ NEVER DO THIS (HALLUCINATION):
"The code looks good, it follows best practices"

✅ ALWAYS DO THIS (ACTUAL EXECUTION):
{"tool": "compile", "arguments": {"target": "."}}
{"tool": "lint_code", "arguments": {"path": "."}}
```

**5-Step Automated Workflow:**
1. Get review context (`get_review_status`)
2. Run automated quality gates
3. Analyze results
4. Manual review (if automated checks pass)
5. Submit review decision (`submit_review`)

**Quality Dimensions:**
- Correctness, Quality, Design, Performance, Security, Testing, Documentation

**Severity Levels:**
- Critical, Major, Minor, Info

### 5. **Comprehensive Tests** (`review_agent_test.go`)

Test coverage includes:
- `TestReviewAgent_checkPreconditions_WithDependencyFiles` - Happy path
- `TestReviewAgent_checkPreconditions_NoDependencyFiles` - Missing code
- `TestReviewAgent_checkPreconditions_NilContext` - Edge case
- `TestReviewAgent_checkPreconditions_SingleFile` - Single dependency
- `TestReviewAgent_checkPreconditions_MultipleFiles` - Multiple dependencies

---

## ⚠️ Critical Gaps Identified

### **Gap 1: Execute() Doesn't Use Workflow Tools** (Priority: CRITICAL)

**Problem:**
The system prompt instructs the agent to use `get_review_status` and `submit_review`, but the actual `Execute()` method doesn't follow this workflow.

**Current Implementation** (`review_agent.go:70-127`):
```go
func (a *ReviewAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    result := &Result{
        TaskID: task.ID,
        Agent:  a.name,
    }

    // Precondition checks
    if err := a.checkPreconditions(ctx, task); err != nil {
        // ... error handling
    }

    // Build prompts
    systemPrompt := a.buildSystemPrompt()
    userPrompt := a.buildUserPrompt(task, currentCtx)

    // ❌ PROBLEM: Just calls LLM directly
    response, err := a.CallLLM(ctx, systemPrompt, userPrompt)

    // ❌ PROBLEM: Stores review as text artifact
    artifact, err := a.StoreArtifact("review", response, "review_agent")

    // ❌ PROBLEM: Only leaves a note, doesn't update task status
    noteText := fmt.Sprintf("Completed review: %s. Review report stored as artifact #%d.",
        task.Description, artifact.ID)
    _ = a.LeaveNote("Manager", noteText)

    result.Success = true
    result.Output = response
    return result, nil
}
```

**What SHOULD Happen** (per system prompt lines 173-217):
1. Call `get_review_status` to retrieve review context
2. **Run automated quality gates BEFORE LLM** (compile, lint, security, etc.)
3. Build prompt with actual tool results
4. LLM analyzes results and makes decision
5. Call `submit_review` with structured findings
6. Task status updated automatically (approved/needs_changes/rejected)
7. Dependent tasks unblocked if approved

**Impact:**
- Reviews stored as unstructured text instead of structured data
- Task status not updated properly (`tasks.status`, `tasks.review_status`)
- No automatic unblocking of dependent tasks
- No notifications sent to agents
- Quality gates not enforced (LLM might hallucinate without running tools)

---

### **Gap 2: No Database-Backed Review Process** (Priority: HIGH)

**Problem:**
The workflow tools (`get_review_status`, `submit_review`) expect a `task_reviews` table, but it's unclear if this table exists or is being populated.

**Evidence:**
- `get_review_status.go:79-100` queries `task_reviews` table
- `submit_review.go:145-157` updates `task_reviews` and `tasks` tables

**Expected Schema:**
```sql
CREATE TABLE IF NOT EXISTS task_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    reviewer_agent TEXT NOT NULL,
    review_type TEXT NOT NULL,  -- "quality", "security", "performance"
    status TEXT NOT NULL,        -- "pending", "approved", "needs_changes", "rejected"
    findings TEXT,               -- JSON array of findings
    comments TEXT,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);
```

**Questions to Verify:**
1. Does `task_reviews` table exist in the database schema?
2. Is it created during initialization?
3. How are review records created (who calls `INSERT INTO task_reviews`)?

**Current Workaround:**
Manager Agent likely creates tasks of type `"review"`, but this doesn't create a record in `task_reviews` table, causing `get_review_status` to fail.

---

### **Gap 3: No Review Request Tool** (Priority: HIGH)

**Problem:**
There's no mechanism to create a review record in the database when a review is requested.

**Missing Tool:** `request_review`

**Expected Behavior:**
```go
// capabilities/orchestration/request_review.go
func Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    taskKey := args["task_key"].(string)
    reviewType := args["review_type"].(string) // "quality", "security", "performance"

    // Get task ID from task_key
    var taskID int
    err := db.QueryRow("SELECT id FROM tasks WHERE task_key = ?", taskKey).Scan(&taskID)
    if err != nil {
        return "", fmt.Errorf("task not found: %s", taskKey)
    }

    // Insert into task_reviews table
    result, err := db.Exec(`
        INSERT INTO task_reviews (task_id, reviewer_agent, review_type, status, created_at)
        VALUES (?, ?, ?, ?, ?)
    `, taskID, "Review", reviewType, "pending", time.Now())

    reviewID, _ := result.LastInsertId()

    // Update task status to "in_review"
    _, err = db.Exec(`
        UPDATE tasks SET status = ?, review_status = ? WHERE id = ?
    `, "in_review", "pending", taskID)

    return fmt.Sprintf(`{"review_id": %d, "task_key": "%s", "status": "pending"}`,
        reviewID, taskKey), nil
}
```

**Who Should Call This?**
- Manager Agent after a task completes
- Code Agent after generating code (self-review request)
- User via chat ("review task TASK-123")

---

### **Gap 4: Quality Gate Execution Not Enforced** (Priority: MEDIUM)

**Problem:**
The system prompt says:

> "RULE: Never approve code without running quality checks first!"

But the LLM could **hallucinate** a review without actually executing the tools.

**Current Risk:**
```
User Prompt: "Review the code in main.go"

LLM (hallucinating): "I reviewed the code. It looks good, follows best practices,
                      no issues found. APPROVED."

Reality: Never ran compile, lint_code, security_scan, etc.
```

**Better Approach:**

**Execute quality gates BEFORE calling LLM:**

```go
func (a *ReviewAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    // 1. Get project path
    projectPath := a.currentContext.ProjectPath

    // 2. Run quality gates BEFORE LLM call
    qualityResults := a.runQualityGates(ctx, projectPath)

    // 3. Build prompt with ACTUAL results
    userPrompt := a.buildUserPromptWithQualityResults(task, qualityResults)

    // 4. LLM only interprets results, doesn't decide whether to run checks
    response, err := a.CallLLM(ctx, systemPrompt, userPrompt)

    return result, nil
}

func (a *ReviewAgent) runQualityGates(ctx context.Context, path string) *QualityResults {
    results := &QualityResults{}

    // Compile check
    compileResult, _ := a.executeTool(ctx, "compile", map[string]interface{}{
        "target": path,
    })
    results.Compile = parseJSON(compileResult)

    // Lint check
    lintResult, _ := a.executeTool(ctx, "lint_code", map[string]interface{}{
        "path": path,
    })
    results.Lint = parseJSON(lintResult)

    // Security scan
    securityResult, _ := a.executeTool(ctx, "security_scan", map[string]interface{}{
        "path": path,
    })
    results.Security = parseJSON(securityResult)

    // Complexity check
    complexityResult, _ := a.executeTool(ctx, "complexity_check", map[string]interface{}{
        "path": path,
    })
    results.Complexity = parseJSON(complexityResult)

    // Coverage check
    coverageResult, _ := a.executeTool(ctx, "coverage_check", map[string]interface{}{
        "package": path,
    })
    results.Coverage = parseJSON(coverageResult)

    return results
}
```

**Benefits:**
- **Guarantees** quality gates are executed
- LLM only interprets results (can't skip checks)
- Faster (parallel tool execution possible)
- More reliable (no hallucination risk)

---

### **Gap 5: Incomplete Feedback Loop Integration** (Priority: MEDIUM)

**What's Implemented:**
- ✅ Precondition checks (`review_agent.go:129-139`)
- ✅ `RequestDependency()` when no code found
- ✅ `RecordError()` capability from BaseAgent

**What's Missing:**
- ❌ No error recording in `Execute()` when quality checks fail
- ❌ No feedback sent when review finds critical issues
- ❌ No integration with FeedbackBus for reactive handling

**Expected Behavior** (per `FEEDBACK_LOOP_DESIGN_V2.md`):

When review finds **critical issues**, the agent should:
1. Record error in TaskContext
2. Send feedback via FeedbackBus
3. Manager handles feedback:
   - Create fix task for critical issues
   - Block current task until fixed
   - Or escalate to user if unfixable

**Implementation:**

```go
func (a *ReviewAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    // ... run quality gates ...

    // Check for critical issues
    criticalIssues := extractCriticalIssues(qualityResults)

    if len(criticalIssues) > 0 {
        // Record error in TaskContext
        for _, issue := range criticalIssues {
            a.RecordError(
                issue.Type,           // "compile_error", "security_vulnerability"
                "review",             // phase
                issue.Message,
                issue.FilePath,
                issue.LineNumber,
                issue.Suggestion,
            )
        }

        // Send feedback to Manager
        err := a.SendFeedback(ctx,
            FeedbackTypeBlocker,
            FeedbackSeverityCritical,
            fmt.Sprintf("Review found %d critical issues", len(criticalIssues)),
            map[string]interface{}{
                "issues": criticalIssues,
                "review_status": "blocked",
            },
            "Fix critical issues before approval",
        )

        // Mark review as "needs_changes"
        // Manager will handle creating fix tasks
    }

    // ... continue with LLM analysis ...
}
```

**Benefits:**
- Automatic fix task creation for critical issues
- Consistent with feedback loop architecture
- No need for manual escalation

---

### **Gap 6: Review Type Specialization Not Used** (Priority: LOW)

**Problem:**
The `task_reviews` table has a `review_type` field (`get_review_status.go:80`), suggesting different review types:
- `"quality"` - Code quality, style, maintainability
- `"security"` - Security vulnerabilities, exploits
- `"performance"` - Performance bottlenecks, optimization

**Current Implementation:**
All reviews are generic. No specialization based on review type.

**Potential Enhancement:**

```go
func (a *ReviewAgent) selectQualityGates(reviewType string) []string {
    switch reviewType {
    case "security":
        return []string{"security_scan", "lint_code"}
    case "performance":
        return []string{"complexity_check", "coverage_check"}
    case "quality":
        return []string{"compile", "format_code", "lint_code", "complexity_check"}
    default:
        return []string{"compile", "lint_code", "security_scan", "complexity_check", "coverage_check"}
    }
}
```

**When to Implement:**
After core workflow is working. This is an optimization, not a blocker.

---

## 🎯 Recommended Implementation Plan

### **Phase 1: Fix Core Workflow** (Priority: CRITICAL, Time: 4-6 hours)

#### 1.1 Verify Database Schema

**File:** `go/context/manager.go` or database initialization

**Action:** Add `task_reviews` table creation:

```go
func (m *Manager) initializeSchema() error {
    _, err := m.db.Exec(`
        CREATE TABLE IF NOT EXISTS task_reviews (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            task_id INTEGER NOT NULL,
            reviewer_agent TEXT NOT NULL,
            review_type TEXT NOT NULL,
            status TEXT NOT NULL,
            findings TEXT,
            comments TEXT,
            created_at DATETIME NOT NULL,
            FOREIGN KEY (task_id) REFERENCES tasks(id)
        );

        CREATE INDEX IF NOT EXISTS idx_task_reviews_task_id ON task_reviews(task_id);
        CREATE INDEX IF NOT EXISTS idx_task_reviews_status ON task_reviews(status);
    `)
    return err
}
```

#### 1.2 Create `request_review` Tool

**File:** `go/capabilities/orchestration/request_review.go` (NEW)

**Action:** Implement review request tool (see Gap 3 for full code)

**Register Tool:**
```go
func init() {
    registry.Register(&RequestReviewTool{})
}
```

#### 1.3 Refactor `ReviewAgent.Execute()`

**File:** `go/agent/review_agent.go`

**Action:** Rewrite `Execute()` to use workflow tools:

```go
func (a *ReviewAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
    result := &Result{
        TaskID: task.ID,
        Agent:  a.name,
    }

    // ✅ STEP 1: Precondition checks
    if err := a.checkPreconditions(ctx, task); err != nil {
        result.Success = false
        result.Error = fmt.Sprintf("Precondition failed: %v", err)
        a.RecordError("precondition_failed", "precondition", err.Error(), "", 0,
            "Ensure code artifacts are available for review")
        return result, err
    }

    // ✅ STEP 2: Get or create review record
    reviewID, err := a.getOrCreateReview(ctx, task)
    if err != nil {
        result.Success = false
        result.Error = fmt.Sprintf("Failed to get review: %v", err)
        return result, err
    }

    // ✅ STEP 3: Run automated quality gates BEFORE LLM
    projectPath := a.currentContext.ProjectPath
    if projectPath == "" {
        projectPath = "."
    }

    qualityResults := a.runQualityGates(ctx, projectPath)

    // ✅ STEP 4: Check for critical issues
    criticalIssues := a.extractCriticalIssues(qualityResults)
    if len(criticalIssues) > 0 {
        // Record errors
        for _, issue := range criticalIssues {
            a.RecordError(issue.Type, "review", issue.Message,
                issue.FilePath, issue.LineNumber, issue.Suggestion)
        }

        // Submit review as "needs_changes"
        findings := a.buildFindings(qualityResults)
        comments := fmt.Sprintf("Found %d critical issues that must be fixed before approval.",
            len(criticalIssues))

        a.submitReview(ctx, reviewID, "needs_changes", findings, comments)

        result.Success = true
        result.Output = comments
        return result, nil
    }

    // ✅ STEP 5: LLM analyzes results (if automated checks passed)
    systemPrompt := a.buildSystemPrompt()
    userPrompt := a.buildUserPromptWithQualityResults(task, qualityResults)

    response, err := a.CallLLM(ctx, systemPrompt, userPrompt)
    if err != nil {
        result.Success = false
        result.Error = fmt.Sprintf("LLM error: %v", err)
        return result, err
    }

    // ✅ STEP 6: Parse LLM decision
    decision := a.parseReviewDecision(response)

    // ✅ STEP 7: Submit review via tool
    findings := a.buildFindings(qualityResults)
    findings = append(findings, decision.ManualFindings...)

    _, err = a.submitReview(ctx, reviewID, decision.Status, findings, decision.Comments)
    if err != nil {
        result.Success = false
        result.Error = fmt.Sprintf("Failed to submit review: %v", err)
        return result, err
    }

    // ✅ STEP 8: Store full response as artifact (for audit trail)
    artifact, _ := a.StoreArtifact("review", response, "review_agent")

    result.Success = true
    result.Output = response
    result.Metadata = map[string]interface{}{
        "review_id":   reviewID,
        "status":      decision.Status,
        "artifact_id": artifact.ID,
    }

    return result, nil
}
```

**Helper Methods to Add:**

```go
// getOrCreateReview gets existing review or creates a new one
func (a *ReviewAgent) getOrCreateReview(ctx context.Context, task *Task) (int, error) {
    // Try to get existing review
    statusJSON, err := a.executeTool(ctx, "get_review_status", map[string]interface{}{
        "task_key": task.TaskKey,
    })

    if err == nil {
        // Parse review ID from response
        var status map[string]interface{}
        json.Unmarshal([]byte(statusJSON), &status)
        if reviewID, ok := status["review_id"].(float64); ok {
            return int(reviewID), nil
        }
    }

    // Create new review
    requestJSON, err := a.executeTool(ctx, "request_review", map[string]interface{}{
        "task_key":    task.TaskKey,
        "review_type": "quality", // Default type
    })

    if err != nil {
        return 0, err
    }

    var request map[string]interface{}
    json.Unmarshal([]byte(requestJSON), &request)
    return int(request["review_id"].(float64)), nil
}

// runQualityGates executes all quality checks
func (a *ReviewAgent) runQualityGates(ctx context.Context, path string) *QualityResults {
    results := &QualityResults{}

    // Run compile check
    compileJSON, _ := a.executeTool(ctx, "compile", map[string]interface{}{
        "target": path,
    })
    json.Unmarshal([]byte(compileJSON), &results.Compile)

    // Run lint check
    lintJSON, _ := a.executeTool(ctx, "lint_code", map[string]interface{}{
        "path": path,
    })
    json.Unmarshal([]byte(lintJSON), &results.Lint)

    // Run security scan
    securityJSON, _ := a.executeTool(ctx, "security_scan", map[string]interface{}{
        "path": path,
    })
    json.Unmarshal([]byte(securityJSON), &results.Security)

    // Run complexity check
    complexityJSON, _ := a.executeTool(ctx, "complexity_check", map[string]interface{}{
        "path": path,
    })
    json.Unmarshal([]byte(complexityJSON), &results.Complexity)

    // Run coverage check
    coverageJSON, _ := a.executeTool(ctx, "coverage_check", map[string]interface{}{
        "package": path,
    })
    json.Unmarshal([]byte(coverageJSON), &results.Coverage)

    return results
}

// extractCriticalIssues finds critical/high severity issues
func (a *ReviewAgent) extractCriticalIssues(results *QualityResults) []CriticalIssue {
    var issues []CriticalIssue

    // Check compile errors (always critical)
    if !results.Compile.Success {
        for _, err := range results.Compile.Errors {
            issues = append(issues, CriticalIssue{
                Type:       "compile_error",
                Severity:   "critical",
                Message:    err.Message,
                FilePath:   err.File,
                LineNumber: err.Line,
                Suggestion: "Fix compilation error",
            })
        }
    }

    // Check security issues (high/critical only)
    for _, issue := range results.Security.Issues {
        if issue.Severity == "critical" || issue.Severity == "high" {
            issues = append(issues, CriticalIssue{
                Type:       "security_vulnerability",
                Severity:   issue.Severity,
                Message:    issue.Message,
                FilePath:   issue.File,
                LineNumber: issue.Line,
                Suggestion: issue.Recommendation,
            })
        }
    }

    return issues
}

// submitReview calls submit_review tool
func (a *ReviewAgent) submitReview(ctx context.Context, reviewID int, status string,
    findings []Finding, comments string) (string, error) {

    return a.executeTool(ctx, "submit_review", map[string]interface{}{
        "review_id": reviewID,
        "status":    status,
        "findings":  findings,
        "comments":  comments,
    })
}

// parseReviewDecision extracts decision from LLM response
func (a *ReviewAgent) parseReviewDecision(response string) ReviewDecision {
    // Try to parse JSON from response
    var decision ReviewDecision

    // Look for decision markers
    if strings.Contains(strings.ToUpper(response), "APPROVED") {
        decision.Status = "approved"
    } else if strings.Contains(strings.ToUpper(response), "REJECT") {
        decision.Status = "rejected"
    } else {
        decision.Status = "needs_changes"
    }

    decision.Comments = response
    decision.ManualFindings = []Finding{} // Parse from response if structured

    return decision
}
```

**Supporting Types:**

```go
type QualityResults struct {
    Compile    CompileResult
    Lint       LintResult
    Security   SecurityResult
    Complexity ComplexityResult
    Coverage   CoverageResult
}

type CriticalIssue struct {
    Type       string
    Severity   string
    Message    string
    FilePath   string
    LineNumber int
    Suggestion string
}

type Finding struct {
    Category string `json:"category"` // "compile", "security", "quality"
    Severity string `json:"severity"` // "critical", "high", "warning"
    Issue    string `json:"issue"`
    Location string `json:"location"`
}

type ReviewDecision struct {
    Status         string    // "approved" | "needs_changes" | "rejected"
    Comments       string
    ManualFindings []Finding // Findings from LLM analysis
}
```

#### 1.4 Update Tests

**File:** `go/agent/review_agent_test.go`

**Action:** Add tests for new workflow:

```go
func TestReviewAgent_Execute_WithQualityGates(t *testing.T) {
    // Test that Execute() runs quality gates before LLM
    // Mock tool execution
    // Verify quality gates were called
}

func TestReviewAgent_Execute_CriticalIssues(t *testing.T) {
    // Test that critical issues trigger needs_changes
    // Verify submit_review called with correct status
}

func TestReviewAgent_Execute_AllChecksPassed(t *testing.T) {
    // Test that clean code gets approved
    // Verify LLM called for final analysis
}
```

---

### **Phase 2: Feedback Loop Integration** (Priority: MEDIUM, Time: 2-3 hours)

#### 2.1 Add Feedback Sending for Blockers

**File:** `go/agent/review_agent.go`

**Action:** Send feedback when critical issues found:

```go
if len(criticalIssues) > 0 {
    // Record errors (already done in Phase 1)

    // ✅ ADD: Send feedback to Manager
    err := a.SendFeedback(ctx,
        FeedbackTypeBlocker,
        FeedbackSeverityCritical,
        fmt.Sprintf("Review blocked by %d critical issues", len(criticalIssues)),
        map[string]interface{}{
            "issues":        criticalIssues,
            "review_status": "blocked",
            "task_key":      task.TaskKey,
        },
        "Fix critical issues before re-review",
    )

    if err != nil {
        fmt.Printf("Warning: failed to send feedback: %v\n", err)
    }

    // Continue with submit_review (already implemented)
}
```

#### 2.2 Manager Handler for Review Blockers

**File:** `go/agent/manager_agent.go`

**Action:** Add handler for `FeedbackTypeBlocker`:

```go
func (m *ManagerAgent) StartFeedbackProcessing(ctx context.Context) {
    bus := GetFeedbackBus()

    // Existing handlers
    bus.RegisterHandler(FeedbackTypeDependencyNeeded, m.handleDependencyRequest)
    bus.RegisterHandler(FeedbackTypeRetryRequest, m.handleRetryRequest)

    // ✅ ADD: Blocker handler
    bus.RegisterHandler(FeedbackTypeBlocker, m.handleBlocker)

    bus.Start(ctx)
}

func (m *ManagerAgent) handleBlocker(ctx context.Context, feedback *AgentFeedback) error {
    fmt.Printf("[ManagerAgent] Processing blocker from %s for task %s\n",
        feedback.AgentName, feedback.TaskID)

    // Extract issues from context
    issues, ok := feedback.Context["issues"].([]interface{})
    if !ok || len(issues) == 0 {
        // No specific issues, just escalate
        return m.escalateToUser(ctx, feedback)
    }

    // Determine if we can create fix tasks or need to escalate
    fixable := true
    for _, issue := range issues {
        issueMap := issue.(map[string]interface{})
        issueType := issueMap["type"].(string)

        // Some issues are auto-fixable
        if issueType == "compile_error" || issueType == "lint_issue" {
            // Can create fix task
            continue
        }

        // Security vulnerabilities might need human review
        if issueType == "security_vulnerability" {
            severity := issueMap["severity"].(string)
            if severity == "critical" {
                fixable = false
                break
            }
        }
    }

    if !fixable {
        return m.escalateToUser(ctx, feedback)
    }

    // Create fix task (similar to handleDependencyRequest)
    taskKey := feedback.Context["task_key"].(string)

    fixTask := NewManagedTask(
        fmt.Sprintf("Fix critical issues in %s", taskKey),
        fmt.Sprintf("Fix %d critical issues found during review", len(issues)),
        ManagedTaskTypeCode,
    )

    // Copy context from blocked task
    // ... similar to handleDependencyRequest ...

    return nil
}
```

---

### **Phase 3: Optimizations** (Priority: LOW, Time: 1-2 hours)

#### 3.1 Review Type Specialization

Implement selective quality gates based on review type (see Gap 6).

#### 3.2 Parallel Quality Gate Execution

Run quality checks concurrently for faster reviews:

```go
func (a *ReviewAgent) runQualityGates(ctx context.Context, path string) *QualityResults {
    results := &QualityResults{}
    var wg sync.WaitGroup

    wg.Add(5)

    go func() {
        defer wg.Done()
        json, _ := a.executeTool(ctx, "compile", map[string]interface{}{"target": path})
        json.Unmarshal([]byte(json), &results.Compile)
    }()

    go func() {
        defer wg.Done()
        json, _ := a.executeTool(ctx, "lint_code", map[string]interface{}{"path": path})
        json.Unmarshal([]byte(json), &results.Lint)
    }()

    // ... other checks in parallel ...

    wg.Wait()
    return results
}
```

---

## 📊 Success Metrics

After implementing Phase 1 & 2, measure:

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Reviews use workflow tools | 100% | Check `submit_review` calls in logs |
| Task status updated correctly | 100% | Verify `tasks.status` after review |
| Critical issues trigger feedback | 100% | Check `agent_feedback` table |
| Dependent tasks unblocked | 100% | Verify auto-unblock on approval |
| Quality gates executed | 100% | Count tool calls before LLM |
| Review decision accuracy | >90% | Manual audit of review results |

---

## 🧪 Testing Strategy

### Unit Tests

**File:** `go/agent/review_agent_test.go`

```go
// Test quality gates are executed
func TestReviewAgent_RunsQualityGates(t *testing.T)

// Test critical issues are detected
func TestReviewAgent_DetectsCriticalIssues(t *testing.T)

// Test review submission
func TestReviewAgent_SubmitsReview(t *testing.T)

// Test feedback sent for blockers
func TestReviewAgent_SendsBlockerFeedback(t *testing.T)
```

### Integration Tests

**File:** `go/tests/integration/review_workflow_test.go` (NEW)

```go
func TestE2E_ReviewWorkflow_Approved(t *testing.T) {
    // Create task with clean code
    // Request review
    // Verify review runs quality gates
    // Verify task approved
    // Verify dependent tasks unblocked
}

func TestE2E_ReviewWorkflow_NeedsChanges(t *testing.T) {
    // Create task with code issues
    // Request review
    // Verify review finds issues
    // Verify task marked needs_changes
    // Verify fix task created
}
```

---

## 📝 Summary

### Gaps Priority Matrix

| Gap | Priority | Effort | Impact |
|-----|----------|--------|--------|
| #1: Execute() doesn't use workflow tools | 🔴 CRITICAL | 4-6h | HIGH |
| #2: No database-backed review process | 🟠 HIGH | 1-2h | HIGH |
| #3: No review request tool | 🟠 HIGH | 2h | HIGH |
| #4: Quality gates not enforced | 🟡 MEDIUM | 2h | MEDIUM |
| #5: Incomplete feedback loop | 🟡 MEDIUM | 2-3h | MEDIUM |
| #6: No review type specialization | 🟢 LOW | 1-2h | LOW |

### Implementation Timeline

- **Week 1:** Phase 1 (Core Workflow) - 6-8 hours
- **Week 2:** Phase 2 (Feedback Loop) - 2-3 hours
- **Week 3:** Phase 3 (Optimizations) - 1-2 hours (optional)

**Total Effort:** 9-13 hours to production-ready review workflow

---

## 🎯 Next Steps

1. **Verify Database Schema** - Check if `task_reviews` table exists
2. **Implement `request_review` Tool** - Enable review record creation
3. **Refactor `ReviewAgent.Execute()`** - Use workflow tools
4. **Add Tests** - Ensure quality gates and workflow work correctly
5. **Integration Testing** - End-to-end review workflow validation
6. **Deploy & Monitor** - Track metrics, adjust as needed

---

**Document Status:** Ready for Implementation
**Last Updated:** October 23, 2025