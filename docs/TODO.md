# Wilson TODO

## Current Status (Oct 21, 2025)

**Core system complete and production-ready** - All ENDGAME Phases 1-5 implemented.

### What's Working
- **MCP Integration:** Filesystem (14 tools), GitHub/Postgres/Slack/Memory ready to enable
- **Multi-Agent System:** 6 agents (Chat, Manager, Research, Code, Test, Review)
- **Async Architecture:** Non-blocking dual-model (chat always responsive, workers on-demand)
- **Code Intelligence:** AST parsing, compile loops, cross-file awareness, quality gates
- **Task Management:** DoR/DoD validation, dependencies, autonomous coordination
- **40+ tools:** Filesystem, code intelligence, web, context, orchestration, MCP

### Model Configuration
- Chat: qwen2.5:7b (4GB, always loaded, fast tool calling)
- Analysis: qwen2.5:7b (4GB)
- Code: qwen2.5-coder:14b (8GB, ephemeral workers)

**Resource Profile:** 4GB idle → 12GB active → 4GB after task (workers killed immediately)

---

## Next Steps

### 1. Production Validation
**Priority:** High
**Goal:** Use Wilson for real work, find issues, improve stability

Tasks:
- Use Wilson for actual development projects
- Test all agent types in real scenarios
- Document common failure modes
- Improve error messages and recovery
- Test MCP servers (GitHub, databases)

**Why important:** Real usage reveals issues testing misses

---

## Future Work

### Session Context Phase 2 (Not Started)
Handle long conversations without token limits.

Tasks:
- Sliding window (last N messages)
- Token counting/estimation
- Auto-summarization of old messages
- Important message pinning
- Graceful token limit handling

See DONE.md: "Conversation History Trade-offs"

**Priority:** Medium (only if hitting token limits in practice)

### Session Context Phase 3: Conversation Persistence
**Priority:** Low
**Goal:** Resume conversations across restarts

Tasks:
- Store conversation in DB
- Resume prompt on startup
- Session commands (/clear, /save, /resume)
- Searchable conversation history

**Estimated Effort:** 4-6 hours

---

### 2. Advanced Agent Features
**Priority:** Medium
**Goal:** Improve agent intelligence and coordination

Tasks:
- **Learning System:** Agents learn from past successful/failed tasks
- **Performance Metrics:** Track success rates, optimize assignments
- **Task Templates:** Pre-defined workflows for common tasks (API, CLI, web scraper)
- **Better Error Recovery:** Automatic retries, fallback strategies
- **Cost Tracking:** Model usage statistics per task

**Estimated Effort:** 1-2 weeks

---

### 3. Developer Experience Improvements
**Priority:** Medium
**Goal:** Make Wilson easier to use and debug

Tasks:
- Better progress visualization (progress bars, structured status)
- Improved error messages (what went wrong, how to fix)
- Configuration wizard for first-time setup
- Model recommendation based on available RAM
- Tool usage analytics (which tools most used)

**Estimated Effort:** 1 week

---

### 4. Optional Advanced Features
**Priority:** Low (nice-to-have)

**Rollback/Undo:**
- Revert file changes if task fails
- Git integration for automatic checkpoints
- Before/after diffs

**Team Mode:**
- Multi-user coordination
- Shared task queues
- Agent resource pooling

**Web Interface:**
- Browser-based UI (alternative to CLI)
- Better for long-running tasks
- Shareable task links

---

## Rollout Strategy

1. **Supervised (Current → 3mo):** User reviews all agent outputs
2. **Semi-Autonomous (3-6mo):** Low-risk tasks run autonomously
3. **Fully Autonomous (6-12mo):** Agents handle most work independently
4. **Team Mode (12+mo):** Multi-user collaboration support

---

## Completed Phases ✅

All completed work documented in **DONE.md**:
- ✅ ENDGAME Phases 1-5 (multi-agent, async, coordination)
- ✅ MCP Integration (Phases 1-3)
- ✅ Code Intelligence (Phases 1-4)
- ✅ Session Context Phase 1 (20-turn history)
- ✅ Web tools (search, research, scraping)
- ✅ Chatbot optimization (intent classification, async delegation)

**Full implementation history:** See DONE.md

---

## Future Improvements (Deferred)

### Additional Feedback Handlers
**Priority:** Low (types already defined, add handlers when needed)

The following feedback types are already defined in the codebase but don't have handlers yet:
- `FeedbackTypeBlocker` - Unrecoverable errors, escalate to user immediately
- `FeedbackTypeContextNeeded` - Request specific files (mostly covered by auto context loading)
- `FeedbackTypeSuccess` - Positive signals for learning and pattern recognition
- `FeedbackTypeTimeout` - Task taking too long (needs timeout mechanism first)

**Why deferred:** Current automatic context loading covers most use cases. Wait for patterns to emerge before adding handlers.

**Estimated Effort:** 2-3 hours when needed

### Parallel Dependency Execution
**Priority:** Medium-High (2-3x speedup for independent tasks)
**Estimated Effort:** 4 hours

**Problem:** Dependencies execute sequentially even when independent.

**Current behavior:**
```
TASK-001: Run tests (blocked)
  ↓ waits for...
TASK-002: Create user_test.go (10s)
  ↓ waits for...
TASK-003: Create handler_test.go (10s)
Total: 20 seconds
```

**Improved behavior:**
```
TASK-001: Run tests (blocked)
  ↓ waits for BOTH...
TASK-002: Create user_test.go (10s) ──┐
TASK-003: Create handler_test.go (10s) ┴─ Parallel
Total: 10 seconds (2x faster!)
```

**Implementation approach:**
- Detect independent tasks (no shared dependencies)
- Execute in parallel using goroutines with sync.WaitGroup
- Maintain dependency graph for ordering
- Limit concurrency (max 3-5 parallel tasks)

**Benefits:**
- 2-3x faster for multi-file workflows
- Better resource utilization
- Scales with available CPU cores

**Risks:**
- More complex coordination logic
- Harder to debug when tasks fail
- Race conditions if dependencies misidentified

**Recommendation:** Implement after production validation shows it's a real bottleneck.
