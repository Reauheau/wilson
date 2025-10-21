# Solution: Task Decomposition Architecture

## Problem Statement

Current implementation has the Code Agent trying to handle complex multi-file tasks in a single execution loop. This leads to:

1. **LLM confusion**: 7B model struggles to remember "create main.go AND test file"
2. **No decomposition**: Complex tasks (multiple files, edits, tests) aren't broken down
3. **Brittle execution**: Works for single-file tasks, fails for complex requests
4. **Unused architecture**: Manager Agent, Review Agent, Test Agent exist but aren't used

## Root Cause Analysis

**Current Flow:**
```
User: "Create app + tests + docs"
  ↓
ChatAgent → delegate_task → CodeAgent
  ↓
CodeAgent tries to do EVERYTHING in 9 iterations:
  - generate main.go
  - generate test file  ← Often forgets this
  - run tests           ← Never happens
  - compile             ← Works
  - documentation       ← Never happens
```

**The Missing Layer: TASK DECOMPOSITION**

The Code Agent receives "Create Go program + test file + build" as ONE task, but it should be FOUR separate subtasks:
1. Generate main.go
2. Generate main_test.go
3. Run tests
4. Build project

## The ENDGAME Solution

According to ENDGAME.md (lines 169-221), the intended architecture is:

```
User Request → ChatAgent → ManagerAgent → Subtasks → Specialist Agents
```

### Manager Agent's Role (Currently Unused!)

The Manager Agent should:
1. **Receive** complex task from ChatAgent
2. **Analyze** requirements and decompose into subtasks
3. **Create** subtasks in task queue with DoR/DoD
4. **Assign** subtasks to appropriate agents (Code, Test, Review)
5. **Monitor** progress and dependencies
6. **Coordinate** handoffs between agents

### Proper Multi-Agent Flow

**Example: "Create Go program + test file"**

```
1. ChatAgent receives request
   ↓
2. ChatAgent delegates to ManagerAgent (not Code Agent directly!)
   ↓
3. ManagerAgent decomposes into subtasks:
   ├─ TASK-001: Generate main.go [CodeAgent]
   │   DoD: Compiles successfully, no errors
   ├─ TASK-002: Generate main_test.go [CodeAgent]
   │   DoD: Test file created, depends on TASK-001
   ├─ TASK-003: Run tests [TestAgent]
   │   DoD: All tests pass, depends on TASK-002
   └─ TASK-004: Build project [CodeAgent]
       DoD: Binary created, depends on TASK-003
   ↓
4. ManagerAgent assigns TASK-001 to CodeAgent
   ↓
5. CodeAgent executes: generate main.go → compile → done
   ↓
6. ManagerAgent sees TASK-001 complete, assigns TASK-002
   ↓
7. CodeAgent executes: generate main_test.go → compile → done
   ↓
8. ManagerAgent assigns TASK-003 to TestAgent
   ↓
9. TestAgent runs tests → reports results
   ↓
10. ManagerAgent assigns TASK-004 to CodeAgent
    ↓
11. CodeAgent builds project
    ↓
12. ManagerAgent marks all tasks complete, notifies ChatAgent
    ↓
13. ChatAgent reports to user: "Done! Created app with tests."
```

## Implementation Plan

### Phase 1: Activate Manager Agent (2 hours)

**Current State:**
- ManagerAgent exists but isn't used
- ChatAgent delegates directly to CodeAgent
- No task decomposition

**Changes:**
1. Update ChatAgent to delegate complex tasks to ManagerAgent (not CodeAgent)
2. Add `decompose_task` capability to ManagerAgent
3. ManagerAgent creates subtasks in task queue
4. ManagerAgent assigns subtasks to specialist agents

**Detection Logic:**
```go
// In ChatAgent
if isSimpleTask(userRequest) {
    // Direct execution: "list files", "what's 2+2"
    handleDirectly()
} else if isSingleAgentTask(userRequest) {
    // Single agent: "research X", "write function Y"
    delegateToAgent(appropriateAgent)
} else {
    // Complex/multi-step: "build app + tests", "refactor module X"
    delegateToManager()
}
```

### Phase 2: Task Decomposition Prompt (1 hour)

Create system prompt for ManagerAgent to decompose tasks:

```
You are the MANAGER AGENT. You decompose complex tasks into atomic subtasks.

RULES:
1. Each subtask should be completable by ONE agent in ONE execution
2. Subtasks must have clear DoD (Definition of Done)
3. Specify dependencies between subtasks
4. Assign to appropriate agent: Code, Test, Research, Review

EXAMPLE:

User: "Create Go CLI for opening apps on macOS + tests + build"

Decomposition:
[
  {
    "title": "Generate main.go",
    "agent": "Code",
    "dependencies": [],
    "dod": "File created, compiles without errors"
  },
  {
    "title": "Generate main_test.go",
    "agent": "Code",
    "dependencies": ["TASK-001"],
    "dod": "Test file created with unit tests"
  },
  {
    "title": "Run tests",
    "agent": "Test",
    "dependencies": ["TASK-002"],
    "dod": "All tests pass"
  },
  {
    "title": "Build project",
    "agent": "Code",
    "dependencies": ["TASK-003"],
    "dod": "Binary created and executable"
  }
]
```

### Phase 3: Sequential Execution (1 hour)

Implement dependency-aware execution:

```go
// In ManagerAgent
func (m *ManagerAgent) ExecuteTaskPlan(ctx context.Context, tasks []*ManagedTask) error {
    for _, task := range tasks {
        // Wait for dependencies
        if err := m.waitForDependencies(task); err != nil {
            return err
        }

        // Assign to agent
        agent := m.getAgent(task.AssignedAgent)

        // Execute
        result, err := agent.Execute(ctx, task)
        if err != nil {
            return fmt.Errorf("task %s failed: %w", task.TaskKey, err)
        }

        // Validate DoD
        if !m.validateDoD(task, result) {
            return fmt.Errorf("task %s failed DoD validation", task.TaskKey)
        }

        // Mark complete
        m.queue.CompleteTask(task.ID)
    }

    return nil
}
```

### Phase 4: Update Code Agent (30 minutes)

**Simplify Code Agent:**
- Remove multi-file logic from Code Agent
- Code Agent only handles ONE file generation per task
- No more "check if test file needed" logic - Manager handles that

```go
// Code Agent BEFORE (complex):
// "Generate main.go AND test file AND compile AND test"

// Code Agent AFTER (simple):
// Task: "Generate main.go"
// → Call generate_code
// → Auto-inject write_file
// → Auto-inject compile
// → Done

// Next task: "Generate main_test.go"
// → Call generate_code with test description
// → Auto-inject write_file
// → Auto-inject compile
// → Done
```

### Phase 5: Activate Test Agent (1 hour)

Currently exists but unused. Enable it:

```go
// TestAgent responsibilities:
// - Receive task: "Run tests for <path>"
// - Execute: go test ./...
// - Parse results
// - Report: pass/fail, coverage %
// - Store artifact: test report
```

### Phase 6: Activate Review Agent (Optional - 1 hour)

For quality assurance:

```go
// ReviewAgent responsibilities:
// - Receive task: "Review code in <path>"
// - Read all files
// - Check for issues (security, style, bugs)
// - Provide feedback
// - Approve or request changes
```

## Benefits of This Architecture

### 1. **Reliability**
- Each agent does ONE thing well
- No LLM memory issues ("did I create test file?")
- Clear success criteria per subtask

### 2. **Scalability**
- Easy to add new agent types
- Parallel execution possible (future: TASK-001 and TASK-002 run concurrently)
- Manager coordinates complexity

### 3. **Visibility**
- User sees progress: "Task 1/4 complete"
- Clear failure points: "TASK-002 failed: test file syntax error"
- Retry individual tasks, not entire flow

### 4. **Maintainability**
- Simple agents, complex coordination
- Each agent has focused prompt
- Easy to debug (which subtask failed?)

## Migration Strategy

### Week 1: Foundation
- ✅ Implement ManagerAgent decomposition
- ✅ Update ChatAgent delegation logic
- ✅ Add task queue execution

### Week 2: Agents
- ✅ Simplify Code Agent (single task focus)
- ✅ Activate Test Agent
- ✅ Add Review Agent (optional)

### Week 3: Testing & Refinement
- ✅ Test complex scenarios
- ✅ Tune decomposition prompts
- ✅ Add parallel execution (optional)

## Success Metrics

After implementation, Wilson should handle:

1. ✅ **Single file**: "Create calculator.go" → 1 subtask → Success
2. ✅ **Multi-file**: "Create app + tests" → 2 subtasks → Both files created
3. ✅ **Complex**: "Build CLI with tests, docs, build" → 4 subtasks → All complete
4. ✅ **Edit + Test**: "Refactor auth + add tests" → 3 subtasks (read, edit, test) → Success
5. ✅ **Full workflow**: "Create feature X" → 6 subtasks (research, code, test, review, docs, build) → Production-ready

## Current vs. Target State

### Current (Broken):
```
User: "Create app + tests"
  ↓
Code Agent tries to do both → Generates main.go twice → Forgets tests
```

### Target (Reliable):
```
User: "Create app + tests"
  ↓
Manager: Break into 2 subtasks
  ├─ Code Agent: Generate main.go ✓
  └─ Code Agent: Generate test file ✓
Result: Both files created successfully
```

## Next Steps

1. **Read this document** - Understand the architecture
2. **Discuss priorities** - Which phases to implement first?
3. **Start with Phase 1** - Activate Manager Agent
4. **Test incrementally** - Each phase should improve reliability
5. **Remove debug logs** - Once decomposition works

---

**Key Insight:** The problem isn't the Code Agent's intelligence - it's that we're asking ONE agent to remember and coordinate MULTIPLE steps. The solution is TASK DECOMPOSITION via the Manager Agent, exactly as designed in ENDGAME.md.
