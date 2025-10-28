# Code Organization Issues

Functions in illogical locations that violate architectural boundaries.

---

## High Priority

### 1. CheckAndNotifyCompletedTasks - UI doing orchestration logic

**Current**: `interface/chat/interface.go:124-148`

**Problem**:
- Chat interface queries orchestration coordinator directly
- Business logic (task monitoring) mixed with UI layer
- Tight coupling: UI → Orchestration

**Fix**:
- Move to `agent/orchestration` package
- Chat interface should only display notifications, not fetch them
- Coordinator should expose `GetCompletedTasks()` or similar
- Main.go coordinates between orchestration and UI

---

### 2. requestConfirmation - Core logic doing UI

**Current**: `core/registry/executor.go:152-170`

**Problem**:
- Executor uses `fmt.Printf`, `bufio.Scanner`, `os.Stdin` directly
- Core business logic should be UI-agnostic
- Makes testing difficult (can't mock user input)
- Violates separation of concerns

**Fix**:
- Create `ConfirmationHandler` interface in executor
- Move stdin-based implementation to `ui` or `interface/chat`
- Executor accepts handler via constructor or field
- Main.go wires up the concrete implementation

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

1. **requestConfirmation** - Most critical architectural violation
2. **CheckAndNotifyCompletedTasks** - Clear layer violation
3. **ollama.Init** - Quick fix, clarify architecture
4. **SetLLMManager** - Requires design decision
5. **Command parsing** - Nice polish
6. **Startup banner** - Nice polish

---

## Notes

- Issues #1 and #2 are the most important - they violate clear boundaries
- Helper functions in executor (findSimilarTools, formatArgs, levenshteinDistance) are fine where they are
- Overall architecture is clean, these are minor issues that accumulated over time
