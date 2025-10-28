# Code Organization Issues

Functions in illogical locations that violate architectural boundaries.

---

## ✅ Completed Issues

### ~~1. CheckAndNotifyCompletedTasks - UI doing orchestration logic~~ ✅ **FIXED**

**Implementation**: Commit `672f9fa` (Oct 28, 2024)
- ✅ Created `GetNewlyCompletedTasks()` in coordinator (orchestration layer)
- ✅ Created `TaskNotification` DTO in `ui/task_notification.go`
- ✅ Created `DisplayTaskCompletionNotifications()` in `ui/notifications.go`
- ✅ Updated main.go to coordinate between layers
- ✅ Removed `CheckAndNotifyCompletedTasks` from `interface/chat`
- **Result**: Clean layer separation - orchestration provides data, UI displays it, main.go coordinates

**Files Changed**:
- `agent/orchestration/coordinator.go` - Added `GetNewlyCompletedTasks()`
- `ui/task_notification.go` - NEW: DTO for passing data between layers
- `ui/notifications.go` - NEW: Display functions
- `interface/chat/interface.go` - Removed UI logic (71 lines)
- `main.go` - Coordinates between layers

---

### ~~2. requestConfirmation - Core logic doing UI~~ ✅ **FIXED**

**Implementation**: (Oct 28, 2024)
- ✅ Created `ConfirmationHandler` interface in `core/registry/confirmation.go`
- ✅ Created `TerminalConfirmation` implementation in `ui/confirmation.go`
- ✅ Created test implementations (`AlwaysConfirm`, `AlwaysDeny`)
- ✅ Updated Executor to use injected handler
- ✅ Removed `requestConfirmation()` method and UI dependencies
- ✅ Updated main.go to inject terminal confirmation
- **Result**: Core logic is now 100% UI-agnostic and testable

**Files Changed**:
- `core/registry/confirmation.go` - NEW: Interface and test implementations
- `ui/confirmation.go` - NEW: Terminal UI implementation
- `core/registry/executor.go` - Removed UI logic, uses interface
- `main.go` - Injects `TerminalConfirmation`

---

## Medium Priority

### 3. SetLLMManager coupling

**Current**: `setup/llm.go:54-56`

```go
web.SetLLMManager(manager)
code_intelligence.SetLLMManager(manager)
```

**Problem**:
- Setup package must know about every capability that needs LLM
- Not scalable - every new capability requires code change here
- Push-based configuration creates coupling

**Fix Options**:
- A) Global LLM manager that capabilities pull from
- B) Registration pattern where capabilities register themselves
- C) Move LLM manager to capabilities init functions

---

### 4. ollama.Init() placement

**Current**: `main.go:49-53`

**Problem**:
- Initialized separately from LLM manager (duplicate system?)
- Marked as deprecated in IDE warnings
- Should be in setup package if still needed

**Fix**:
- Move to `setup/bootstrap.go` if still required
- OR remove entirely if superseded by LLM manager
- Document migration path

---

## Low Priority (Nice to Have)

### 5. Command parsing in main.go

**Current**: `main.go:114-130`

**Problem**:
- Special commands (help, exit) parsed directly in main loop
- Will grow as more commands are added
- Mixing application flow with command interpretation

**Fix**:
- Create `InputHandler` or `CommandParser` in `interface/chat`
- Return structured command types (Exit, Help, Chat, etc.)
- Keeps main.go focused on flow, not parsing

---

### 6. Startup banner in main.go

**Current**: `main.go:55-67`

**Problem**:
- UI display code scattered in main
- Not critical but could be cleaner

**Fix**:
- Create `ui.PrintStartupBanner(cfg)` function
- Keeps main.go focused on application flow

---

## Implementation Order

1. ~~**requestConfirmation**~~ - ✅ **DONE** (Strategy Pattern with DI)
2. ~~**CheckAndNotifyCompletedTasks**~~ - ✅ **DONE** (Observer Pattern)
3. **ollama.Init** - Quick fix, clarify architecture
4. **SetLLMManager** - Requires design decision
5. **Command parsing** - Nice polish
6. **Startup banner** - Nice polish

---

## Notes

- ~~Issues #1 and #2 are the most important~~ - ✅ **COMPLETED** (Oct 28, 2024)
- Helper functions in executor (findSimilarTools, levenshteinDistance) are fine where they are
- Overall architecture is clean, these are minor issues that accumulated over time
- Issues #1 and #2 now serve as good examples of proper layer separation for future refactoring
