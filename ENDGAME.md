# Wilson ENDGAME Vision

**Last Updated:** October 16, 2025
**Status:** Vision Document + Production Roadmap

---

## 🎯 Vision Statement

Transform Wilson into a **fully autonomous multi-agent system** where specialized agents collaborate to complete complex tasks, review each other's work, and achieve goals with minimal human intervention.

### Core Principles

1. **Agent Autonomy:** Agents can pick up, execute, and complete tasks independently
2. **Specialized Intelligence:** Each agent uses models optimized for their domain
3. **Quality Assurance:** Built-in review processes with Definition of Ready/Done
4. **Collaborative Workflow:** Agents work together, building on each other's outputs
5. **Human-in-the-Loop:** User remains in control, can intervene at any point

---

## 🏗️ Architecture Overview (Async Multi-Agent)

```
┌─────────────────────────────────────────────────────────────────┐
│                         USER INTERFACE                          │
│         (Chat with Wilson - Always Responsive)                  │
│  "Build app" [instant] "What's 2+2?" [instant] "Status?" [...]  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                  CHAT AGENT (Wilson)                            │
│  Model: llama3:latest (small, always loaded, 4GB)              │
│  Mode: NON-BLOCKING - Returns immediately                       │
│  Role: Interpret intent, delegate async, report progress        │
│  Tools: All tools + delegate_task_async                         │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼ (async - returns task ID)
┌─────────────────────────────────────────────────────────────────┐
│                     COORDINATOR                                 │
│  DelegateTaskAsync() - spawns goroutine, returns immediately   │
│  Status broadcaster - real-time updates to UI                   │
└─────────┬───────────────────────────────────────┬───────────────┘
          │                                       │
          ▼                                       ▼
┌─────────────────────┐               ┌──────────────────────────┐
│  MANAGER AGENT      │               │    TASK QUEUE            │
│  (On-Demand)        │               │    (SQLite)              │
│                     │               │                          │
│  Model: llama3      │               │  - Tasks + DoR/DoD       │
│  Role: Planning     │               │  - Dependencies          │
│  Tools: Orchestrate │               │  - Status tracking       │
└──────────┬──────────┘               │  - Model used per task   │
           │                          └──────────────────────────┘
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    WORKER MANAGER                               │
│  Strategy: Spawn on-demand, Kill immediately after task        │
│  Max concurrent: 2 workers (configurable)                       │
│  Model lifecycle: Load when spawned, unload when killed         │
└─────┬──────────┬──────────┬──────────┬──────────────────────────┘
      │          │          │          │
      ▼          ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ CODE     │ │ RESEARCH │ │ TEST     │ │ REVIEW   │
│ WORKER   │ │ WORKER   │ │ WORKER   │ │ WORKER   │
│(goroutn) │ │(goroutn) │ │(goroutn) │ │(goroutn) │
│          │ │          │ │          │ │          │
│ Model:   │ │ Model:   │ │ Model:   │ │ Model:   │
│ qwen2.5- │ │ llama3   │ │ llama3   │ │ llama3   │
│ coder:   │ │ or       │ │ or       │ │ or       │
│ 14b      │ │ mixtral  │ │ phi3     │ │ claude-3 │
│ (~8GB)   │ │ (~6GB)   │ │ (~4GB)   │ │ (~6GB)   │
│          │ │          │ │          │ │          │
│ Status:  │ │ Status:  │ │ Status:  │ │ Status:  │
│ EPHEMER- │ │ EPHEMER- │ │ EPHEMER- │ │ EPHEMER- │
│ AL       │ │ AL       │ │ AL       │ │ AL       │
│          │ │          │ │          │ │          │
│ Life:    │ │ Life:    │ │ Life:    │ │ Life:    │
│ Spawn →  │ │ Spawn →  │ │ Spawn →  │ │ Spawn →  │
│ Load →   │ │ Load →   │ │ Load →   │ │ Load →   │
│ Execute→ │ │ Execute→ │ │ Execute→ │ │ Execute→ │
│ KILL     │ │ KILL     │ │ KILL     │ │ KILL     │
│          │ │          │ │          │ │          │
│ Tools:   │ │ Tools:   │ │ Tools:   │ │ Tools:   │
│ - read   │ │ - search │ │ - run    │ │ - read   │
│ - write  │ │ - fetch  │ │ - test   │ │ - analyze│
│ - compile│ │ - analyze│ │ - report │ │ - review │
└────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘
     │            │            │            │
     └────────────┴────────────┴────────────┘
                  │
                  ▼
     ┌────────────────────────────┐
     │    CONTEXT STORE           │
     │    (SQLite DB)             │
     │                            │
     │  - Tasks + Status          │
     │  - Artifacts               │
     │  - Agent Communications    │
     │  - Reviews                 │
     │  - Model usage per task    │
     │  - Resource tracking       │
     └────────────────────────────┘

Resource Profile (16GB Machine):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
IDLE:     Wilson (4GB) ═══════════════════════════════░░░░░░░░░░
ACTIVE:   Wilson (4GB) + Code Worker (8GB) ═══════════════════════
DONE:     Wilson (4GB) ═══════════════════════════════░░░░░░░░░░ [KILLED]

Key Characteristics:
• Wilson: Always responsive, never blocks
• Workers: Spawn fresh, work, die immediately
• Models: Loaded on-demand, unloaded after task
• Async: Chat + background work concurrently
• Status: Real-time updates showing which model is working
```

---

## 🤖 Agent Types & Responsibilities

### 1. Chat Agent (Wilson) - User Interface

**Role:** Primary interface, intent interpretation, user communication

**Model:** llama3:latest (small, conversational, always loaded - 4GB RAM)

**Responsibilities:**
- Understand user requests via intent classification (Chat/Tool/Delegate)
- Handle simple queries directly with minimal prompt (~50 tokens, <50ms response)
- Execute tool requests with full prompt (~2000 tokens)
- **Delegate complex tasks asynchronously** - returns immediately, never blocks
- Report progress back to user with real-time status updates
- Monitor background tasks while remaining responsive

**Optimization (Oct 16, 2025):**
- **Intent Classification:** Keyword-based detection determines response path
- **Dual-Mode Prompts:** Minimal chat prompt (50 tokens) vs full tool prompt (2000 tokens) = 40x smaller for simple chat
- **Prompt Caching:** Thread-safe caching eliminates regeneration overhead
- **Fast Path:** Simple chat <50ms (was 3-5s), tool execution ~2-3s

**Async Upgrade (Phase 5):**
- **Non-Blocking Delegation:** `delegate_task_async()` spawns workers in background, returns task ID immediately
- **Concurrent Chat:** Can answer questions while workers execute tasks
- **Task-Aware:** Knows about background tasks, can report progress without blocking
- **Status Display:** Shows which models are working in real-time

**Tools:**
- `delegate_task_async(task_description, success_criteria)` - Non-blocking
- `check_task_progress(task_id)` - Query background tasks
- All basic tools available for direct execution

**Example Interaction:**
```
User: "Hello" → IntentChat → Minimal prompt → Fast response (~50ms)
User: "List files" → IntentTool → Full prompt → Tool execution (~2s)
User: "Build a web scraper" → IntentDelegate → Spawns worker async, returns immediately
  └→ Wilson: "Task TASK-001 started with Code Agent (qwen2.5-coder:14b)"
User: "What's 2+2?" → IntentChat → Wilson answers immediately (~50ms) while worker executes
  └→ Wilson: "4. By the way, your scraper is 40% complete."
```

---

### 2. Manager Agent - Task Orchestrator

**Role:** Task decomposition, assignment, progress tracking, quality assurance

**Model:** llama3:latest (planning) or gpt-4o/claude-3-opus for complex tasks

**Execution Mode:** **On-Demand** (Phase 5 Async)
- Spawned when Wilson delegates complex tasks
- Uses chat model (llama3) or specialized planning model
- Creates subtasks and assigns to workers
- Workers execute asynchronously in background
- Manager monitors via task queue (SQLite)

**Responsibilities:**
- Break complex tasks into subtasks
- Assign subtasks to appropriate specialist agents via Worker Manager
- Track task dependencies
- Monitor progress and quality
- Handle blockers and re-assignments
- Ensure Definition of Done is met
- **Spawn workers asynchronously** - doesn't wait for completion

**Tools:**
- `create_subtask(parent_task, description, assignee, dependencies, DoD)`
- `assign_task(task_id, agent_name)` - Triggers worker spawn
- `get_task_status(task_id)` - Query task queue
- `mark_task_complete(task_id)`
- `request_review(task_id, reviewer_agent)` - Spawns review worker

**Decision Logic:**
```python
def assign_task(task):
    if "research" in task.type:
        return research_agent
    elif "code" in task.type:
        return code_agent
    elif "test" in task.type:
        return test_agent
    elif "review" in task.type:
        return review_agent
```

**Example Task Breakdown:**
```
Main Task: "Build web scraper for product prices"

Subtasks:
1. Research: Find target websites and HTML structure
   → Assigned to: Research Agent
   → DoD: Document HTML selectors, robots.txt compliance

2. Code: Implement scraper with rate limiting
   → Assigned to: Code Agent
   → Dependencies: [Task 1]
   → DoD: Code works, handles errors, respects rate limits

3. Test: Verify scraper works on all target sites
   → Assigned to: Test Agent
   → Dependencies: [Task 2]
   → DoD: All tests pass, edge cases handled

4. Review: Code review and quality check
   → Assigned to: Review Agent
   → Dependencies: [Task 3]
   → DoD: No blockers, suggestions addressed
```

---

### 3. Research Agent - Information Gathering

**Role:** Web research, documentation reading, data extraction

**Model:** mixtral:8x7b or gpt-4o (analysis, summarization)

**Responsibilities:**
- Search for information online
- Analyze documentation
- Extract relevant data
- Summarize findings
- Store research artifacts

**Tools:**
- `search_web`
- `research_topic`
- `fetch_page`
- `analyze_content`
- `store_artifact`

**Quality Criteria:**
- Sources cited
- Information accurate and current
- Relevant to task requirements
- Properly formatted and stored

---

### 4. Code Agent - Implementation

**Role:** Write, modify, and refactor production-ready code with automated quality gates

**Model:** qwen2.5-coder:14b (or deepseek-coder:33b on high-end machines)

**Execution Mode:** **Ephemeral Worker** (Phase 5 Async)
- Spawned on-demand when code task assigned
- Loads model when spawned (~2-3s)
- Executes task with dedicated model instance
- **Killed immediately after task completion** (no idle period)
- Fresh worker spawned for revisions (clean state guaranteed)

**Resource Profile:**
- Model size: ~8GB RAM (14B params) or ~16GB (32B params)
- Lifecycle: Spawn → Load → Execute → Kill (total: task duration + 3s)
- Concurrent limit: Configurable (default: 2 max workers)

**Responsibilities:**
- Generate code based on specifications
- Modify existing code
- Follow coding standards
- Add documentation and comments
- Create necessary files/directories

**Advanced Capabilities (4-Phase Upgrade Complete):**
- **Phase 1: Code Intelligence** - AST parsing, symbol search, structure analysis, import management
- **Phase 2: Iterative Compilation** - Compile-test-fix loops, structured error parsing, test execution
- **Phase 3: Cross-File Awareness** - Dependency mapping, pattern discovery, impact analysis
- **Phase 4: Quality Gates** - Auto-formatting, linting, security scanning, complexity checks, coverage verification

**Tools:**
- `read_file`
- `write_file`
- `search_files`
- `run_command` (for formatting, linting)
- `store_artifact`

**Quality Criteria:**
- Code compiles/runs
- Follows project conventions
- Includes comments
- Error handling present
- No obvious security issues

**Async Status Example:**
```
[Status: Code Agent (qwen2.5-coder:14b, 8GB): loading model... ⏳]
[Status: Code Agent (qwen2.5-coder:14b, 8GB): writing auth module (40%) ⚙️]
[Status: Code Agent (qwen2.5-coder:14b, 8GB): compiling code (80%) 🔨]
[Status: Code Agent: task complete, model unloaded ✓]
```

---

### 5. Test Agent - Verification

**Role:** Test code, validate functionality, report issues

**Model:** phi3:14b or llama3:latest (logical reasoning)

**Responsibilities:**
- Write and run tests
- Validate functionality
- Test edge cases
- Document test results
- Report failures to Manager

**Tools:**
- `run_command` (test frameworks)
- `read_file` (test results)
- `analyze_content` (failure analysis)
- `store_artifact` (test reports)

**Quality Criteria:**
- All tests pass
- Coverage meets threshold (e.g., 80%)
- Edge cases covered
- Performance acceptable

---

### 6. Review Agent - Quality Assurance

**Role:** Review work, ensure quality standards, provide feedback

**Model:** claude-3-opus or gpt-4o (critical analysis, reasoning)

**Responsibilities:**
- Review code quality
- Check for security issues
- Validate against requirements
- Provide improvement suggestions
- Approve or request changes

**Tools:**
- `read_file`
- `read_context` (task requirements)
- `search_artifacts` (previous work)
- `store_artifact` (review report)

**Quality Criteria:**
- No critical issues
- Meets functional requirements
- Code quality acceptable
- Documentation present
- Security considerations addressed

---

## 📋 Task Lifecycle with DoR/DoD

### Definition of Ready (DoR)

Before a task can be assigned, it must have:

```yaml
task:
  id: TASK-123
  title: "Implement user authentication"
  description: "Detailed description..."

  definition_of_ready:
    - Clear acceptance criteria defined
    - Dependencies identified and available
    - Required resources accessible
    - Assigned agent has necessary tools
    - Success criteria measurable

  ready: true  # Set by Manager Agent
```

### Definition of Done (DoD)

Task is complete when:

```yaml
task:
  id: TASK-123

  definition_of_done:
    functional:
      - Feature works as specified
      - All acceptance criteria met
      - No known critical bugs

    code_quality:
      - Code follows standards
      - Includes documentation
      - Error handling present
      - No security vulnerabilities

    testing:
      - Unit tests written and passing
      - Edge cases covered
      - Performance acceptable

    review:
      - Peer reviewed by Review Agent
      - All review comments addressed
      - Approved for completion

  status: "done"  # Set by Manager Agent after all criteria met
```

### Task States

```
NEW → READY → ASSIGNED → IN_PROGRESS → IN_REVIEW → DONE
                              ↓
                          BLOCKED
                              ↓
                          READY (after unblocked)
```

---

## 🔄 Workflow Examples

### Example 1: Simple Research Task

```
User: "What are the best practices for Ollama API usage?"

1. Chat Agent (Wilson):
   - Understands: Research task, straightforward
   - Decision: Handle directly (no need for Manager)
   - Action: Delegates to Research Agent

2. Research Agent:
   - Uses research_topic tool
   - Fetches 3-5 sources
   - Analyzes and summarizes
   - Stores findings in context

3. Chat Agent (Wilson):
   - Receives summary from Research Agent
   - Presents to user in natural language

Timeline: ~60 seconds
```

---

### Example 2: Complex Multi-Step Task

```
User: "Build a CLI tool that monitors website uptime and sends alerts"

1. Chat Agent (Wilson):
   - Understands: Complex, multi-component task
   - Decision: Delegate to Manager Agent
   - Creates main task: TASK-001

2. Manager Agent:
   - Analyzes requirements
   - Creates task breakdown:

   TASK-001: Build uptime monitor CLI
   ├── TASK-002: Research existing uptime tools [Research Agent]
   ├── TASK-003: Design CLI interface [Code Agent]
   ├── TASK-004: Implement HTTP checker [Code Agent] → depends on 002, 003
   ├── TASK-005: Implement alert system [Code Agent] → depends on 004
   ├── TASK-006: Write tests [Test Agent] → depends on 004, 005
   ├── TASK-007: Code review [Review Agent] → depends on 006
   └── TASK-008: Create documentation [Research Agent] → depends on 007

3. Research Agent (TASK-002):
   - Researches uptime monitoring tools
   - Documents common patterns
   - Stores findings
   - Marks task complete

4. Code Agent (TASK-003):
   - Designs CLI structure
   - Creates argument parser spec
   - Documents API
   - Marks task complete

5. Code Agent (TASK-004):
   - Implements HTTP checker
   - Uses research findings
   - Handles edge cases
   - Self-review against DoD
   - Marks task complete

6. Code Agent (TASK-005):
   - Implements alerts
   - Integrates with checker
   - Marks task complete

7. Test Agent (TASK-006):
   - Writes unit tests
   - Writes integration tests
   - Runs tests
   - Reports: 95% coverage ✓
   - Marks task complete

8. Review Agent (TASK-007):
   - Reviews all code
   - Checks for issues
   - Finds: "Add rate limiting"
   - Marks task as NEEDS_CHANGES
   - Assigns back to Code Agent

9. Code Agent (Re-work TASK-004):
   - Adds rate limiting
   - Updates tests
   - Marks ready for re-review

10. Review Agent (Re-review TASK-007):
    - Verifies changes
    - All criteria met ✓
    - Marks task complete

11. Research Agent (TASK-008):
    - Creates README
    - Documents usage
    - Adds examples
    - Marks task complete

12. Manager Agent:
    - All subtasks complete ✓
    - Marks TASK-001 as DONE
    - Notifies Wilson

13. Chat Agent (Wilson):
    - Presents summary to user
    - Shows GitHub link / file paths
    - Asks if anything needs adjustment

Timeline: ~10-15 minutes
```

---

### Example 3: Collaborative Bug Fix

```
User: "Fix the bug in the authentication module"

1. Wilson → Manager Agent
   - Task: Debug and fix auth issue

2. Manager creates subtasks:
   - TASK-101: Research the bug [Research Agent]
   - TASK-102: Analyze code [Code Agent]
   - TASK-103: Fix bug [Code Agent]
   - TASK-104: Test fix [Test Agent]
   - TASK-105: Review fix [Review Agent]

3. Research Agent:
   - Searches error logs
   - Finds similar issues online
   - Documents potential causes

4. Code Agent:
   - Reads auth module
   - Uses research findings
   - Identifies root cause
   - Proposes fix

5. Code Agent:
   - Implements fix
   - Adds regression test

6. Test Agent:
   - Runs all auth tests
   - Verifies fix works
   - Checks no regressions

7. Review Agent:
   - Reviews fix quality
   - Validates against security best practices
   - Approves

8. Wilson → User:
   - "Fixed! The issue was X, resolved by Y."
   - "All tests passing ✓"

Timeline: ~5 minutes
```

---

## 🎯 Two Primary Use Cases

### Use Case 1: Wilson as Main Chat Agent (Async Non-Blocking)

**Scenario:** User wants direct, conversational interaction with async background work

**Flow:**
```
User ←→ Wilson (Chat Agent) [ALWAYS RESPONSIVE]
            ↓ (for simple tasks)
         Executes directly (<50ms)
            ↓ (for complex tasks)
         Delegates async to Manager → Returns immediately
            ↓
         Manager spawns workers in background
            ↓
         Workers execute concurrently (Wilson still responsive)
            ↓
         Status updates shown in real-time
            ↓
         Wilson presents results when complete
```

**Advantages:**
- **Never blocks** - Wilson always responsive
- Conversational during background work
- Can interrupt/redirect mid-task
- Real-time status updates showing models
- Chat while agents work

**Example (Async):**
```
You: "Create a REST API for user management"
Wilson: "Task TASK-001 started. Using Code Agent with qwen2.5-coder:14b."
  [Status: Code Agent (qwen2.5-coder:14b): loading model... ⏳]

You: "What's the capital of France?"  [IMMEDIATE!]
Wilson: "Paris. Your API task is 20% complete."
  [Status: Code Agent (qwen2.5-coder:14b): implementing endpoints (40%) ⚙️]

You: "How's it going?"
Wilson: "60% done. Code Agent has created 3/5 endpoints."
  [Status: Test Agent (llama3): writing tests (80%) 🧪]

Wilson: "Done! Created 5 endpoints with auth, all tests passing (92% coverage)."
  [Worker killed - memory: 12GB → 4GB]
```

---

### Use Case 2: Manager-Driven Autonomous Mode (Multi-Worker Parallel)

**Scenario:** User delegates large task and lets agents work autonomously with multiple workers

**Flow:**
```
User → Wilson: "Build feature X"  [instant response]
       ↓
    Wilson → Manager: Delegates async
       ↓
    Manager creates plan, spawns multiple workers
       ↓
    Workers execute in parallel (up to max_concurrent)
       ↓
    Agents coordinate via task queue (SQLite)
       ↓
    Manager monitors progress, spawns new workers as tasks complete
       ↓
    Wilson → User: Real-time status updates, final summary
```

**Advantages:**
- **True parallel execution** (multiple workers concurrently)
- **User can chat anytime** - Wilson never blocked
- Agents self-coordinate via task queue
- Can run for hours, Wilson stays responsive
- **Resource efficient** - workers killed after each task

**Example (Async Multi-Worker):**
```
You: "Build a complete CLI app for task management, let me know when done"
Wilson: "Task TASK-001 started. Manager is creating plan..."
  [Status: Manager (llama3): breaking down tasks... 📋]

[2 minutes later - Manager spawns 2 workers]
  [Status: Code Worker 1 (qwen2.5-coder:14b): implementing commands (30%) ⚙️]
  [Status: Research Worker (llama3): analyzing CLI patterns (60%) 📚]

You: "What's the progress?"  [IMMEDIATE!]
Wilson: "50% done. 3/6 subtasks complete. Code Worker writing parser logic."

[30 minutes later - workers cycling through tasks]
  [Status: Test Worker (llama3): running integration tests (85%) 🧪]

Wilson: "Task complete! Built CLI with 15 commands, 85% test coverage,
         Review Agent approved. Ready: ./task-cli --help"
  [All workers killed - memory back to 4GB]

You: "Can you add a --verbose flag?"
Wilson: "Task TASK-002 started. Spawning Code Worker..."
  [Fresh worker, clean state, no contamination from previous work]
```

---

**Vision Owner:** Roderick van Nievelt
**Document Status:** Vision Document - High-level overview of endgame capabilities
**Last Updated:** October 16, 2025

---

*"The goal is not to replace the developer, but to amplify their capabilities by handling the repetitive, the tedious, and the time-consuming - freeing them to focus on creativity, strategy, and problem-solving."*

---

## 🚀 Phase 5: Async Multi-Agent Architecture (Next)

**Status:** Designed - Ready for Implementation
**Document:** ASYNC_PLAN.md
**Effort:** 11-17 hours
**Priority:** Critical

### Vision: Dual-Model Async Execution

Transform Wilson into a truly non-blocking system where:
- **Wilson (Chat):** Always responsive with small model (llama3, 4GB)
- **Worker Agents:** Spawn on-demand with large models (qwen2.5-coder:14b, 8GB)
- **Resource-First:** Kill workers immediately after tasks, spawn fresh for revisions
- **Concurrent Operation:** Chat while agents work in background

### Architecture Changes

```
┌─────────────────────────────────────────────┐
│  USER (You)                                 │
│  "Build app" → Wilson responds instantly    │
│  "What's 2+2?" → Wilson responds instantly  │
└────────────┬────────────────────────────────┘
             │
    ┌────────▼────────┐
    │  Wilson (Chat)  │  Model: llama3 (always loaded)
    │  Non-blocking   │  Status: Always responsive
    └────────┬────────┘
             │
    ┌────────▼─────────────┐
    │  Worker Manager      │  Spawns/kills on-demand
    │  Max 2 concurrent    │  No pre-warming
    └────┬─────────┬───────┘
         │         │
    ┌────▼────┐ ┌─▼─────────┐
    │ Code    │ │ Test      │  Models: qwen2.5-coder:14b
    │ Worker  │ │ Worker    │  Lifecycle: Kill after task
    │ (gortn) │ │ (gortn)   │  State: Fresh every time
    └─────────┘ └───────────┘
```

### Key Features

**1. Non-Blocking Delegation**
- `delegate_task` returns immediately (50ms)
- Workers spawn in background with their models
- Wilson ready for next question

**2. Resource Efficiency**
```
Idle:      4GB  (Wilson only)
Working:   12GB (Wilson + 1 worker loading model)
Done:      4GB  (Worker killed immediately)
```

**3. Machine Profiles**
- **Low-end (8GB):** llama3 everywhere, 1 worker max
- **Mid-range (16GB):** llama3 + qwen2.5-coder:14b, 2 workers
- **High-end (32GB):** llama3 + qwen2.5-coder:32b + mixtral, 2 workers
- **Cloud (64GB+):** deepseek-coder:33b + mixtral:8x22b, 4 workers

**4. Status Visibility**
```
[Status: Code Agent (qwen2.5-coder:14b, 8GB): writing auth (60%) ⚙️]
[Status: Test Agent (llama3, 4GB): running tests (80%) 🧪]
```

**5. Revision Workflow**
- Review finds issues → Old worker terminated
- New worker spawns → Fresh state, no contamination
- Fixes applied → Worker killed immediately

### Implementation Phases

| Phase | Description | Effort |
|-------|-------------|--------|
| 0 | Model lifecycle (on-demand only) | 2-3h |
| 1 | Async delegation | 2-3h |
| 2 | Worker pool (kill-after-task) | 3-4h |
| 3 | Status updates (show models) | 2-3h |
| 4 | Concurrent chat (task-aware) | 1-2h |
| 5 | Model health & fallback | 1-2h |

### Design Decisions

✅ **Model loading:** On-demand only (no pre-warming)
✅ **Worker lifecycle:** Kill immediately after task (no keep-alive)
✅ **Max concurrent:** 2 workers default (configurable per machine)
✅ **Easy switching:** Simple config.yaml profiles for different machines

### Success Criteria

After Phase 5 implementation:

```
You: "Build a todo app"
Wilson (llama3, <50ms): "Starting TASK-001. Using Code Agent."
  [Status: Code Agent (qwen2.5-coder:14b): loading model... ⏳]

You: "What's 2+2?"  [IMMEDIATE!]
Wilson (llama3, <50ms): "4. Your task is 20% complete."
  [Status: Code Agent (qwen2.5-coder:14b): writing code (40%) ⚙️]

You: "What's the status?"
Wilson: "60% done. Code Agent has written 3/5 files using qwen2.5-coder:14b."

[Task completes]
Wilson: "Done! Your todo app is ready. Tests: 85% coverage."
[Worker killed - memory drops from 12GB to 4GB within 1s]
```

For detailed technical design, see **ASYNC_PLAN.md**.

---

## 📝 Implementation Details

For the technical implementation plan, roadmap, and detailed tasks, see **TODO.md**.

---

