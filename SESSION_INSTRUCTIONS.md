# Session Instructions for Claude

**Purpose:** Quick setup guide for Claude when working on Wilson

---

## Communication Style

- **Pragmatic, not enthusiastic** - No excessive emoji or celebration
- **Concise by default** - Verbose explanations only when requested
- **Technical accuracy first** - Prioritize correctness over validation
- **Documentation only when asked** - Don't create .md files unless explicitly requested

---

## Session Startup

### Essential Reading (if relevant to task)
- `ENDGAME.md` - Architecture vision and multi-agent system design
- `TODO.md` - Current priorities
- `DONE.md` - Recent completions and patterns
- `FEEDBACK_LOOP_DESIGN.md` - Self-healing system design (when implementing feedback features)

### Quick Orientation
1. Check `git status` and recent commits
2. Scan `TODO.md` for active work
3. Review `DONE.md` for recent context

---

## Task Management

### TodoWrite Usage
**Use for:** Multi-step tasks (3+ steps), debugging sessions, feature implementations
**Don't use for:** Single-step tasks, quick fixes, simple questions
**Completion:** Mark done immediately (don't batch), move valuable items to DONE.md, delete trivial ones

---

## Wilson Quick Reference

### Core Principles
1. **Atomic Tasks** - One task = one file/change, Manager orchestrates multi-file workflows
2. **Generic > Specific** - Use `[placeholder]` not `main.go`, read from `task.Input["dependency_files"]`
3. **Precondition Checks** - Validate before executing, send feedback if missing dependencies
4. **Feedback > Failures** - Send structured feedback to Manager instead of hitting max iterations

### Architecture
```
User → ChatAgent → ManagerAgent (decomposes) → Worker Agents (Code/Test/Review)
                         ↓
                   Task Queue (SQLite) + Feedback Loop
```

### Agent Pattern
```go
// 1. Check preconditions first
// 2. Execute with tools (via LLM)
// 3. Verify results
// 4. Send feedback if blocked
```

### Task Decomposition Pattern
```
User: "Create [project] in [path] with tests"
→ Task 1 (code): Implement [feature] → outputs [source_files]
→ Task 2 (code): Write tests for [source_files] (reads Input["dependency_files"])
→ Task 3 (test): Execute tests in [path]
→ Task 4 (code): Build project
```

### Key Files
- `agent/manager_agent.go` - Task decomposition, dependency injection
- `agent/coordinator.go` - Async execution, concurrency control
- `agent/agent_executor.go` - Tool execution loop
- `agent/code_agent.go`, `test_agent.go`, `review_agent.go` - Worker agents
- `agent/queue.go` - Task queue operations

### Database Tables
- `tasks` - Task queue with DoR/DoD
- `agent_feedback` - Feedback loop communication (designed, not yet implemented)
- `contexts`, `artifacts`, `agent_notes` - Context storage

### Common Issues & Fixes
- **"Max iterations"** → Add precondition check, send dependency feedback
- **"JSON validation failed"** → Simplify prompt, use code LLM for structured output
- **"Generic test templates"** → Ensure dependency_files injection, prompt to read first
- **"Compile errors loop"** → Exit after first compile (atomic principle)

---

## Development Standards

### Testing
- Run tests after changes: `go test ./go/agent/... -v`
- Name: `test_[component]_[scenario].go`
- Document failures in DONE.md

### Git Commits
```
[Component] Brief description

- Detail 1
- Detail 2

Impact: [what changed]
```

### Tools
Must implement: `Metadata()`, `Validate()`, `Execute()`
Categories: filesystem, web, context, orchestration, system
Validate: Required params, types, security (relative paths), logic

### Agents
- Use `BuildSharedPrompt(name)` + specific instructions
- Show tool patterns with examples, not methodology
- No hardcoded paths/files in prompts
- One responsibility per agent (atomic principle)

---

## Quick Commands

```bash
# Build
go build -o wilson main.go

# Test
go test ./go/agent/... -v

# DB queries
sqlite3 wilson.db "SELECT task_key, status, title FROM tasks ORDER BY created_at DESC LIMIT 10"
sqlite3 wilson.db "SELECT * FROM agent_feedback WHERE processed = 0"
```

---

**Last Updated:** 2025-10-22