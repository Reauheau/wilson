# Agent Directory Structure Proposal

**Current State:** 24 files, 6900+ lines in flat directory structure
**Problem:** Growing complexity, hard to find files, feedback loop will add 5-10+ more files
**Goal:** Clear organization, easy navigation, supports future growth

---

## 📊 Current Structure Analysis

### Current Files (24 total)

**Specialist Agents (6 files):**
- `code_agent.go` (395 lines) - Code generation
- `test_agent.go` (219 lines) - Test execution
- `review_agent.go` (295 lines) - Quality review
- `research_agent.go` (193 lines) - Web research
- `analysis_agent.go` (265 lines) - Content analysis
- `chat_agent.go` (247 lines) - User interface

**Orchestration (4 files):**
- `manager_agent.go` (1092 lines) ⚠️ **TOO LARGE**
- `coordinator.go` (319 lines)
- `queue.go` (557 lines)
- `task.go` (265 lines)

**Base Infrastructure (5 files):**
- `base_agent.go` (161 lines)
- `agent_executor.go` (377 lines)
- `registry.go` + `registry_test.go` (387 lines total)
- `types.go` (various types)

**Validation & Quality (4 files):**
- `verifier.go` (266 lines)
- `quality_validators.go` (458 lines)
- `dor_dod.go` (364 lines)
- `llm_validator.go` (165 lines)

**Chat Specific (3 files):**
- `chat_handler.go` (varies)
- `intent.go` (192 lines)
- `shared_prompt.go` (varies)

**Context (1 file):**
- `task_context.go` (varies)

### Upcoming Files (Feedback Loop)

From `FEEDBACK_LOOP_DESIGN.md`:
- `feedback.go` - FeedbackBus, types, handlers
- `feedback_handler.go` - Manager handlers (extracted from manager_agent.go)
- `preconditions.go` - Precondition checks
- Potentially: `patterns.go`, `metrics.go`, `feedback_store.go`

---

## 🎯 Proposed Structure

### Option A: Domain-Based (Recommended)

```
go/agent/
├── README.md                          # Directory overview
│
├── types.go                           # Shared types (Agent, Task, Result, etc.)
├── registry.go                        # Agent registration
├── registry_test.go
│
├── base/                              # Base agent infrastructure
│   ├── base_agent.go                  # BaseAgent implementation
│   ├── executor.go                    # Tool execution loop (rename from agent_executor.go)
│   ├── llm_validator.go               # LLM response validation
│   └── task_context.go                # Rich task context
│
├── agents/                            # Specialist agent implementations
│   ├── chat_agent.go                  # User-facing chat interface
│   ├── code_agent.go                  # Code generation
│   ├── test_agent.go                  # Test execution
│   ├── review_agent.go                # Quality gates
│   ├── research_agent.go              # Web research
│   ├── analysis_agent.go              # Content analysis
│   └── shared_prompt.go               # Shared prompt builders
│
├── orchestration/                     # Task coordination
│   ├── coordinator.go                 # Async execution, concurrency
│   ├── manager.go                     # Task decomposition (rename from manager_agent.go)
│   ├── manager_decompose.go           # Decomposition logic (extract from manager.go)
│   ├── manager_dependency.go          # Dependency injection (extract from manager.go)
│   ├── queue.go                       # Task queue operations
│   ├── queue_test.go                  # (rename from queue_phase0_test.go)
│   └── task.go                        # Task types and lifecycle
│
├── feedback/                          # Feedback loop system (NEW)
│   ├── bus.go                         # FeedbackBus, event-driven messaging
│   ├── types.go                       # Feedback types, severity
│   ├── handlers.go                    # Manager feedback handlers
│   ├── preconditions.go               # Precondition checks
│   └── metrics.go                     # Feedback metrics (Phase 2)
│
├── validation/                        # Quality gates and validation
│   ├── verifier.go                    # Result verification
│   ├── quality.go                     # Quality validators (rename from quality_validators.go)
│   ├── dor_dod.go                     # Definition of Ready/Done
│   └── workflow.go                    # Workflow validation (extract from verifier.go)
│
└── chat/                              # Chat-specific logic
    ├── handler.go                     # Chat handling (rename from chat_handler.go)
    └── intent.go                      # Intent classification
```

**Line counts after refactor:**
- `base/` - ~700 lines (4 files)
- `agents/` - ~1800 lines (7 files)
- `orchestration/` - ~2200 lines (7 files, manager split 3 ways)
- `feedback/` - ~500 lines (5 files)
- `validation/` - ~1100 lines (4 files)
- `chat/` - ~250 lines (2 files)

**Benefits:**
- Clear separation of concerns
- Manager split into manageable pieces (~400 lines each)
- Feedback system isolated and extensible
- Easy to find: "Where's the queue?" → `orchestration/queue.go`
- Supports future growth: Add `learning/`, `patterns/` directories

---

### Option B: Layer-Based (Alternative)

```
go/agent/
├── core/                              # Core abstractions
│   ├── types.go
│   ├── registry.go
│   ├── base_agent.go
│   └── task_context.go
│
├── execution/                         # Execution layer
│   ├── executor.go
│   ├── coordinator.go
│   ├── llm_validator.go
│   └── verifier.go
│
├── management/                        # Management layer
│   ├── manager.go
│   ├── queue.go
│   ├── task.go
│   └── dor_dod.go
│
├── feedback/                          # Feedback layer
│   ├── bus.go
│   ├── handlers.go
│   └── preconditions.go
│
├── workers/                           # Worker agents
│   ├── chat.go
│   ├── code.go
│   ├── test.go
│   ├── review.go
│   ├── research.go
│   └── analysis.go
│
└── validation/
    └── quality.go
```

**Benefits:**
- Architectural clarity (layers visible)
- Easy to understand flow: core → management → execution → workers

**Drawbacks:**
- Less intuitive file names (workers/code.go vs agents/code_agent.go)
- Harder to find specific functionality

---

## 🔧 Migration Plan

### Phase 1: Create Structure (30 min)

```bash
# Create directories
mkdir -p go/agent/{base,agents,orchestration,feedback,validation,chat}

# Move files (no code changes yet)
mv go/agent/base_agent.go go/agent/base/
mv go/agent/agent_executor.go go/agent/base/executor.go
mv go/agent/llm_validator.go go/agent/base/
mv go/agent/task_context.go go/agent/base/

mv go/agent/code_agent.go go/agent/agents/
mv go/agent/test_agent.go go/agent/agents/
mv go/agent/review_agent.go go/agent/agents/
mv go/agent/research_agent.go go/agent/agents/
mv go/agent/analysis_agent.go go/agent/agents/
mv go/agent/chat_agent.go go/agent/agents/
mv go/agent/shared_prompt.go go/agent/agents/

mv go/agent/coordinator.go go/agent/orchestration/
mv go/agent/manager_agent.go go/agent/orchestration/manager.go
mv go/agent/queue.go go/agent/orchestration/
mv go/agent/queue_phase0_test.go go/agent/orchestration/queue_test.go
mv go/agent/task.go go/agent/orchestration/

mv go/agent/verifier.go go/agent/validation/
mv go/agent/quality_validators.go go/agent/validation/quality.go
mv go/agent/dor_dod.go go/agent/validation/

mv go/agent/chat_handler.go go/agent/chat/handler.go
mv go/agent/intent.go go/agent/chat/
```

### Phase 2: Fix Import Paths (1 hour)

Update imports throughout codebase:
```go
// Old
import "wilson/agent"

// New - specific imports
import (
    "wilson/agent"                    // types.go, registry.go (stay at root)
    "wilson/agent/base"               // BaseAgent, Executor
    "wilson/agent/agents"             // CodeAgent, TestAgent, etc.
    "wilson/agent/orchestration"      // Manager, Coordinator, Queue
    "wilson/agent/feedback"           // FeedbackBus (new)
    "wilson/agent/validation"         // Verifier, Quality
    "wilson/agent/chat"               // ChatHandler, Intent
)
```

**Files to update (search for `"wilson/agent"`):**
- `main.go`
- `go/interface/chat/interface.go`
- `go/capabilities/orchestration/*.go`
- All agent files (update cross-references)

### Phase 3: Split manager_agent.go (1 hour)

**Current:** 1092 lines in one file
**After:**

```go
// orchestration/manager.go (~400 lines)
// Core manager, initialization, task lifecycle

// orchestration/manager_decompose.go (~350 lines)
// Task decomposition, heuristic logic, LLM planning
func (m *ManagerAgent) DecomposeTask(...)
func (m *ManagerAgent) heuristicDecompose(...)
func (m *ManagerAgent) needsDecomposition(...)
func extractProjectPath(...)
func extractCoreDescription(...)

// orchestration/manager_dependency.go (~350 lines)
// Dependency management, context injection
func (m *ManagerAgent) injectDependencyContext(...)
func (m *ManagerAgent) waitForDependencies(...)
func (m *ManagerAgent) checkParentCompletion(...)
```

### Phase 4: Implement Feedback (2-3 hours)

Following `FEEDBACK_LOOP_DESIGN.md` Phase 1:

```go
// feedback/bus.go
// FeedbackBus, channel management, routing

// feedback/types.go
// FeedbackType, FeedbackSeverity, AgentFeedback

// feedback/handlers.go
// Manager handlers (extracted from manager.go)
func (m *ManagerAgent) handleDependencyRequest(...)
func (m *ManagerAgent) handleBlocker(...)

// feedback/preconditions.go
// Precondition checks for agents
func CheckTestFilesExist(task *Task) error
func CheckDirectoryExists(path string) error
```

### Phase 5: Add README (15 min)

```go
// agent/README.md
# Agent System Architecture

## Directory Structure

- **`types.go`, `registry.go`** - Core types and agent registration
- **`base/`** - Base agent infrastructure (BaseAgent, Executor, LLM validation)
- **`agents/`** - Specialist agent implementations (Code, Test, Review, etc.)
- **`orchestration/`** - Task coordination (Manager, Coordinator, Queue)
- **`feedback/`** - Self-healing feedback loop system
- **`validation/`** - Quality gates (DoR/DoD, verifiers, quality checks)
- **`chat/`** - Chat-specific logic (intent classification, handlers)

## Key Files

- `agents/code_agent.go` - Code generation worker
- `orchestration/manager.go` - Task decomposition and orchestration
- `orchestration/coordinator.go` - Async execution, concurrency control
- `feedback/bus.go` - Event-driven feedback system
- `validation/dor_dod.go` - Definition of Ready/Done criteria

## Adding a New Agent

1. Create `agents/new_agent.go`
2. Extend `BaseAgent` from `base/base_agent.go`
3. Implement `Execute(ctx, task)` method
4. Register in `registry.go`
5. Add to `orchestration/manager.go` routing

## Adding a New Feedback Type

1. Add constant to `feedback/types.go`
2. Implement handler in `feedback/handlers.go`
3. Register handler in `manager.StartFeedbackProcessing()`
4. Use in agent via `agent.SendFeedback()`
```

---

## 🎯 Benefits Summary

### Before (Current)
```
agent/
├── 24 files in flat structure
├── manager_agent.go (1092 lines, hard to navigate)
├── Mixed concerns (agents, orchestration, validation)
└── Will become 30+ files with feedback loop
```

### After (Proposed)
```
agent/
├── 6 directories (clear domains)
├── ~7 files per directory (manageable)
├── Manager split into 3 focused files (~400 lines each)
├── Feedback system isolated
└── Easy to find: "feedback?" → feedback/, "queue?" → orchestration/
```

**Navigation Examples:**
- "Where's the task queue?" → `orchestration/queue.go`
- "How does dependency injection work?" → `orchestration/manager_dependency.go`
- "Where are feedback handlers?" → `feedback/handlers.go`
- "Where's TestAgent?" → `agents/test_agent.go`

**Extensibility:**
- Add new agent: Single file in `agents/`
- Add new feedback type: Edit `feedback/types.go` + `feedback/handlers.go`
- Add learning system: New `learning/` directory
- Add pattern matching: New `patterns/` directory

---

## 🚨 Risks & Mitigations

### Risk 1: Import Path Changes Break Tests
**Mitigation:** Update all imports in one commit, run full test suite
**Command:** `go test ./... -v` before/after

### Risk 2: IDE Loses Track of References
**Mitigation:** Use Go refactoring tools: `gofmt`, `gopls rename`
**VSCode:** Right-click → Rename Symbol (updates all references)

### Risk 3: Circular Dependencies
**Mitigation:** Keep `types.go` and `registry.go` at root level
**Rule:** Subdirectories can import root, not each other (except via interfaces)

### Risk 4: Too Much Churn for Active Development
**Mitigation:** Do refactor BEFORE feedback loop implementation
**Timeline:** 3-4 hours total, clean slate for feedback work

---

## 🗓️ Recommended Timeline

**Now (Before Feedback Loop):**
- Phase 1-3: Structure + imports + split manager (2.5 hours)
- Test everything still works
- Commit: "Refactor: Organize agent directory by domain"

**Next (During Feedback Loop):**
- Phase 4: Implement feedback in new `feedback/` directory (2-3 hours)
- Clean separation, no pollution of orchestration/
- Phase 5: Add README (15 min)

**Total Time:** ~3 hours upfront, saves hours later in maintenance

---

## ✅ Acceptance Criteria

After refactor:
- ✅ All tests pass: `go test ./go/agent/... -v`
- ✅ Wilson builds: `go build -o wilson main.go`
- ✅ Wilson runs: `./wilson <<< "hello"`
- ✅ No file exceeds 500 lines (except generated code)
- ✅ Directory names clearly indicate purpose
- ✅ README.md documents structure
- ✅ Imports updated consistently

---

## 📝 Decision: Choose Option A

**Rationale:**
1. Domain-based more intuitive than layer-based
2. Aligns with Go convention (package = domain)
3. Easier to find files: "test agent" → `agents/test_agent.go`
4. Supports future growth: Add domains as directories
5. Feedback loop gets isolated space

**Next Steps:**
1. Review this proposal
2. Get approval
3. Execute migration (Phases 1-3)
4. Test thoroughly
5. Implement feedback loop in clean structure

---

**Last Updated:** 2025-10-22
**Status:** Awaiting Approval