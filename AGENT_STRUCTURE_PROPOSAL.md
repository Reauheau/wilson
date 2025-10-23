# Agent Directory Structure Proposal

**Current State (Oct 23, 2025):** 36 files (23 source + 13 tests), 7,600+ lines in flat directory structure
**Problem:** Growing complexity, hard to find files, manager_agent.go exploded to 1,348 lines (+23%)
**Goal:** Clear organization, easy navigation, supports future growth
**Status:** ✅ Ready to Execute - Feedback loop already implemented, structure needed urgently

---

## 📊 Current Structure Analysis

### Current Files (36 total: 23 source + 13 tests)

**Specialist Agents (6 files + 2 tests):**
- `code_agent.go` (451 lines) - Code generation + preconditions ⚠️ Large
- `code_agent_test.go` (tests)
- `test_agent.go` (290 lines) - Test execution + preconditions
- `review_agent.go` (316 lines) - Quality review + preconditions
- `review_agent_test.go` (tests)
- `research_agent.go` (193 lines) - Web research
- `analysis_agent.go` (265 lines) - Content analysis
- `chat_agent.go` (247 lines) - User interface

**Orchestration (4 files + 2 tests):**
- `manager_agent.go` (1,348 lines) 🔴 **CRITICAL - TOO LARGE** (+256 lines since proposal)
- `manager_agent_context_test.go` (context loading tests)
- `coordinator.go` (319 lines)
- `queue.go` (557 lines)
- `queue_test.go`
- `task.go` (265 lines)
- `task_test.go`

**Base Infrastructure (4 files + 1 test):**
- `base_agent.go` (243 lines) - BaseAgent with feedback support
- `agent_executor.go` (472 lines) - Tool execution loop
- `agent_executor_test.go` (tests)
- `llm_validator.go` (165 lines)
- `registry.go` (111 lines)
- `registry_test.go`
- `types.go` (98 lines)
- `task_context.go` (146 lines) - Rich execution context

**Feedback Loop (2 files + 2 tests):** ✅ **ALREADY IMPLEMENTED**
- `feedback.go` (426 lines) - FeedbackBus, types, handlers, TaskContext integration
- `feedback_test.go` (comprehensive tests)
- `compile_error_classifier.go` (229 lines) - Hybrid error handling (8 types)
- `compile_error_classifier_test.go`

**Validation & Quality (3 files + 2 tests):**
- `verifier.go` (266 lines)
- `quality_validators.go` (458 lines)
- `quality_validators_test.go`
- `dor_dod.go` (364 lines)
- `dor_dod_test.go`

**Chat Specific (3 files + 1 test):**
- `chat_handler.go` (94 lines)
- `intent.go` (192 lines)
- `intent_test.go`
- `shared_prompt.go` (74 lines)

### Key Changes Since Original Proposal

**✅ Feedback Loop Implemented (Oct 23):**
- feedback.go created (426 lines)
- Preconditions embedded in agents (code_agent.go, test_agent.go, review_agent.go)
- Compile error classifier added (229 lines, not in original proposal)
- Manager feedback handlers added to manager_agent.go

**🔴 Manager Agent Explosion:**
- Original proposal: 1,092 lines
- Current: 1,348 lines (+23% growth)
- Now contains: decomposition, dependency injection, context loading, feedback handlers, lifecycle management

**📊 Total Growth:**
- Original: ~6,900 lines
- Current: ~7,600 lines (+10% growth in 1 day)
- Validates proposal's prediction of rapid growth

---

## 🎯 Proposed Structure (Updated for Current Reality)

### Domain-Based Structure (Recommended)

```
go/agent/
├── README.md                          # Directory overview
│
├── types.go                           # Shared types (98 lines)
├── registry.go                        # Agent registration (111 lines)
├── registry_test.go
│
├── base/                              # Base agent infrastructure (~1,050 lines)
│   ├── base_agent.go                  # BaseAgent with feedback (243 lines)
│   ├── executor.go                    # Tool execution loop (472 lines, rename from agent_executor.go)
│   ├── executor_test.go               # (move agent_executor_test.go)
│   ├── llm_validator.go               # LLM response validation (165 lines)
│   └── task_context.go                # Rich task context (146 lines)
│
├── agents/                            # Specialist agents (~1,900 lines + tests)
│   ├── chat_agent.go                  # User-facing chat (247 lines)
│   ├── code_agent.go                  # Code generation + preconditions (451 lines)
│   ├── code_agent_test.go             # (move with source)
│   ├── test_agent.go                  # Test execution + preconditions (290 lines)
│   ├── review_agent.go                # Quality gates + preconditions (316 lines)
│   ├── review_agent_test.go           # (move with source)
│   ├── research_agent.go              # Web research (193 lines)
│   ├── analysis_agent.go              # Content analysis (265 lines)
│   └── shared_prompt.go               # Shared prompt builders (74 lines)
│
├── orchestration/                     # Task coordination (~2,500 lines)
│   ├── coordinator.go                 # Async execution, concurrency (319 lines)
│   ├── manager.go                     # Core manager (rename from manager_agent.go, ~350 lines)
│   │                                  # - ManagerAgent struct, registration
│   │                                  # - CRUD (CreateTask, GetTaskStatus, ListAllTasks)
│   │                                  # - Statistics (GetQueueStatistics)
│   ├── manager_decompose.go           # Task decomposition logic (~400 lines) ⭐
│   │                                  # - DecomposeTask, heuristics
│   │                                  # - extractProjectPath, extractCoreDescription
│   ├── manager_dependency.go          # Dependency management (~350 lines) ⭐
│   │                                  # - injectDependencyContext
│   │                                  # - loadRequiredFiles (context loading)
│   │                                  # - waitForDependencies, checkParentCompletion
│   ├── manager_feedback.go            # Feedback handlers (~250 lines) ⭐ NEW
│   │                                  # - StartFeedbackProcessing
│   │                                  # - handleDependencyRequest, handleRetryRequest
│   │                                  # - escalateToUser
│   ├── manager_agent_context_test.go  # (move with manager)
│   ├── queue.go                       # Task queue operations (557 lines)
│   ├── queue_test.go                  # (move with queue)
│   ├── task.go                        # Task types and lifecycle (265 lines)
│   └── task_test.go                   # (move with task)
│
├── feedback/                          # Feedback loop system (~655 lines) ✅ IMPLEMENTED
│   ├── feedback.go                    # FeedbackBus, types, handlers (426 lines)
│   │                                  # (Keep as single file - manageable size)
│   ├── feedback_test.go               # (comprehensive tests)
│   ├── compile_classifier.go          # Hybrid error handling (229 lines, rename)
│   └── compile_classifier_test.go     # (8 error types, iterative fix)
│
├── validation/                        # Quality gates (~1,090 lines)
│   ├── verifier.go                    # Result verification (266 lines)
│   ├── quality.go                     # Quality validators (458 lines, rename from quality_validators.go)
│   ├── quality_test.go                # (move quality_validators_test.go)
│   ├── dor_dod.go                     # Definition of Ready/Done (364 lines)
│   └── dor_dod_test.go                # (move with source)
│
└── chat/                              # Chat-specific logic (~360 lines)
    ├── handler.go                     # Chat handling (94 lines, rename from chat_handler.go)
    ├── intent.go                      # Intent classification (192 lines)
    ├── intent_test.go                 # (move with source)
    └── shared_prompt.go               # (74 lines) - OR move to agents/

**Total after refactor:** ~7,600 lines (same as current, just organized)

```

**Line counts after refactor:**
- `base/` - ~1,050 lines (5 files)
- `agents/` - ~1,900 lines (9 files with tests)
- `orchestration/` - ~2,500 lines (10 files, manager split 4 ways)
- `feedback/` - ~655 lines (4 files) ✅ Already complete
- `validation/` - ~1,090 lines (5 files with tests)
- `chat/` - ~360 lines (4 files with tests)

**Benefits:**
- ✅ Manager split into 4 focused files (~250-400 lines each)
- ✅ Feedback system already isolated (just needs to move)
- ✅ Tests live with their source files (easier to find)
- ✅ Easy navigation: "feedback handlers?" → `orchestration/manager_feedback.go`
- ✅ Supports future growth: Each domain can expand independently

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

## 🔧 Migration Plan (Updated for Current State)

### Phase 1: Create Structure (15 min)

```bash
cd /Users/roderick.vannievelt/IdeaProjects/wilson/go/agent

# Create directories
mkdir -p base agents orchestration feedback validation chat
```

### Phase 2: Move Files (30 min) - NO CODE CHANGES

```bash
# Base infrastructure (5 files)
mv base_agent.go base/
mv agent_executor.go base/executor.go
mv agent_executor_test.go base/executor_test.go
mv llm_validator.go base/
mv task_context.go base/

# Specialist agents (9 files with tests)
mv chat_agent.go agents/
mv code_agent.go agents/
mv code_agent_test.go agents/
mv test_agent.go agents/
mv review_agent.go agents/
mv review_agent_test.go agents/
mv research_agent.go agents/
mv analysis_agent.go agents/
mv shared_prompt.go agents/

# Orchestration (10 files with tests)
mv coordinator.go orchestration/
mv manager_agent.go orchestration/manager.go  # Will split in Phase 3
mv manager_agent_context_test.go orchestration/
mv queue.go orchestration/
mv queue_test.go orchestration/
mv task.go orchestration/
mv task_test.go orchestration/

# Feedback system (4 files with tests) ✅ Already implemented
mv feedback.go feedback/
mv feedback_test.go feedback/
mv compile_error_classifier.go feedback/compile_classifier.go
mv compile_error_classifier_test.go feedback/compile_classifier_test.go

# Validation (5 files with tests)
mv verifier.go validation/
mv quality_validators.go validation/quality.go
mv quality_validators_test.go validation/quality_test.go
mv dor_dod.go validation/
mv dor_dod_test.go validation/

# Chat interface (4 files with tests)
mv chat_handler.go chat/handler.go
mv intent.go chat/
mv intent_test.go chat/

# Keep at root level
# - types.go
# - registry.go
# - registry_test.go
```

### Phase 3: Fix Import Paths (1 hour)

Update imports throughout codebase:
```go
// Old
import "wilson/agent"

// New - specific imports
import (
    "wilson/agent"                    // types.go, registry.go (stay at root)
    "wilson/agent/base"               // BaseAgent, Executor, TaskContext
    "wilson/agent/agents"             // CodeAgent, TestAgent, ReviewAgent, etc.
    "wilson/agent/orchestration"      // Manager, Coordinator, Queue, Task
    "wilson/agent/feedback"           // FeedbackBus, CompileClassifier
    "wilson/agent/validation"         # Verifier, Quality, DOR/DOD
    "wilson/agent/chat"               // ChatHandler, Intent
)
```

**Files to update (search for `"wilson/agent"` or `agent.`):**
- `go/main.go`
- `go/interface/chat/interface.go`
- `go/capabilities/orchestration/*.go`
- All files within `agent/` subdirectories (cross-references)

**Test command after each file:**
```bash
go build -o wilson main.go  # Should compile
```

### Phase 4: Split manager.go (1.5 hours) ⭐ **CRITICAL**

**Current:** orchestration/manager.go (1,348 lines)
**Target:** Split into 4 focused files

**Step 1: Extract feedback handlers** (orchestration/manager_feedback.go ~250 lines)
```go
// All feedback-related methods
func (m *ManagerAgent) StartFeedbackProcessing()
func (m *ManagerAgent) handleDependencyRequest()
func (m *ManagerAgent) handleRetryRequest()
func (m *ManagerAgent) escalateToUser()
// + all helper functions for feedback
```

**Step 2: Extract decomposition logic** (orchestration/manager_decompose.go ~400 lines)
```go
// Task breakdown and planning
func (m *ManagerAgent) DecomposeTask(...)
func (m *ManagerAgent) heuristicDecompose(...)
func (m *ManagerAgent) needsDecomposition(...)
func extractProjectPath(...)
func extractCoreDescription(...)
// + decomposition prompt building
```

**Step 3: Extract dependency management** (orchestration/manager_dependency.go ~350 lines)
```go
// Dependency and context management
func (m *ManagerAgent) injectDependencyContext(...)
func (m *ManagerAgent) loadRequiredFiles(...)        // Context loading
func (m *ManagerAgent) extractFilenameFromError(...)
func (m *ManagerAgent) waitForDependencies(...)
func (m *ManagerAgent) checkParentCompletion(...)
```

**Step 4: Keep core in manager.go** (~350 lines)
```go
// Core ManagerAgent
// - struct definition
// - NewManagerAgent, RegisterAgent, SetLLMManager, SetRegistry
// - Simple CRUD: CreateTask, CreateSubtask, GetTaskStatus, ListAllTasks
// - StartTask, CompleteTask, BlockTask, UnblockTask
// - Statistics: GetQueueStatistics
// - Auto-assignment: AutoAssignReadyTasks, selectBestAgent
```

**Test after each extraction:**
```bash
go test ./go/agent/orchestration/... -v
go build -o wilson main.go
```

### Phase 5: Verify Everything Works (30 min)

Run full test suite:
```bash
# All agent tests
go test ./go/agent/... -v

# Build Wilson
go build -o wilson main.go

# Test Wilson runs
./wilson <<< "hello"

# Check imports
grep -r "wilson/agent" go/ | grep import | wc -l  # Should show subdirectory imports
```

### Phase 6: Add README (15 min)

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

## 🗓️ Execution Timeline

**Total Time:** 3.5-4 hours (high-priority task)

**Phases:**
1. Create structure: 15 min
2. Move files (no code changes): 30 min
3. Fix import paths: 1 hour
4. Split manager.go: 1.5 hours ⭐ **Most critical**
5. Verify everything works: 30 min
6. Add README: 15 min

**Risk Mitigation:**
- Test after each phase
- Keep git commits atomic (one phase per commit)
- Run full test suite after imports fixed
- Validate Wilson still runs after each major change

**Expected Outcome:**
- manager.go: 1,348 lines → 4 files of ~250-400 lines each
- Clear domain boundaries
- Tests colocated with source
- Foundation for future features

---

## ✅ Acceptance Criteria

After refactor completion:
- ✅ All 106+ tests pass: `go test ./go/agent/... -v`
- ✅ Wilson builds: `go build -o wilson main.go`
- ✅ Wilson runs: `./wilson <<< "hello"` (chat works)
- ✅ Wilson can delegate: Complex task executes correctly
- ✅ No file exceeds 500 lines (manager split into 4)
- ✅ Directory names clearly indicate purpose (6 domains)
- ✅ README.md documents structure
- ✅ Imports updated consistently (subdirectory imports)
- ✅ Tests in same directory as source files

**Post-Refactor Verification:**
```bash
# Test suite
go test ./go/agent/... -v
# Expected: PASS (all 106+ tests)

# Build
go build -o wilson main.go
# Expected: No errors

# Run Wilson
./wilson <<< "hello"
# Expected: Wilson responds

# Check structure
ls go/agent/
# Expected: base/ agents/ orchestration/ feedback/ validation/ chat/ + root files

# Verify no large files
find go/agent -name "*.go" ! -name "*_test.go" -exec wc -l {} + | sort -rn | head -5
# Expected: Largest file <500 lines
```

---

## 📝 Decision: Execute Domain-Based Structure

**Rationale:**
1. ✅ Feedback loop already implemented - validates proposal urgency
2. ✅ manager_agent.go at 1,348 lines (23% growth) - unsustainable
3. ✅ 36 files in flat structure - navigation breaking down
4. ✅ Domain-based aligns with Go conventions
5. ✅ Tests with source files = better maintainability
6. ✅ Supports future growth (learning/, patterns/ directories)

**Critical Modifications from Original:**
1. ✅ Keep feedback.go as single file (426 lines manageable)
2. ✅ Add manager_feedback.go split (feedback handlers separate)
3. ✅ Move compile_error_classifier to feedback/ (part of error handling)
4. ✅ Colocate tests with source (not separate test/ directory)

**Why Now:**
- Manager at breaking point (1,348 lines)
- Feedback loop complete - organic growth validates need
- Before next feature: Clean slate prevents technical debt
- 3.5-4 hours investment saves hours in future maintenance

**Next Step:** Execute refactor (Phases 1-6)

---

**Last Updated:** 2025-10-23
**Status:** ✅ **APPROVED - READY TO EXECUTE**
**Priority:** HIGH - Do before next feature