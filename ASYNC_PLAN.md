# Wilson Async Plan (Dual-Model Edition)

**Document Version:** 2.0
**Updated:** October 20, 2025
**Status:** Design Document
**Related:** ENDGAME.md, TODO.md

---

## Executive Summary

This document outlines the path to achieve **asynchronous, autonomous multi-agent task execution with dual-model support**. Your vision: Wilson (small chat model) stays responsive while background agents use larger, specialized models for actual work.

**Key Innovation:** Run 2 models simultaneously:
- **Wilson (Chat):** Small, fast model (e.g., llama3:latest) for conversation
- **Worker Agents:** Larger, specialized models (e.g., qwen2.5-coder:14b for code tasks)

**Status:** ~70% there! Infrastructure exists, need async coordination + model isolation.

---

## Your Ideal Design vs Current State

### Your Ideal Flow
```
You: "Hey Wilson, could you build this app for me?"
Wilson (llama3:latest): "Sure, let me distribute this task to the manager."
  [Wilson chat model stays active and responsive]
  [Status line: "Manager is planning... 📋"]
  [Code Agent spawns with qwen2.5-coder:14b in background]
  [Status line: "Code Agent (qwen2.5-coder): writing auth module... ⚙️"]

You: "Wilson, what's the weather?" (meanwhile, still chatting)
Wilson (llama3:latest): "It's sunny! By the way, the build task is 60% complete..."
  [Wilson responds immediately with small model]
  [Code Agent continues working with large model in background]

  [Status line: "Test Agent (llama3): running tests... 🧪"]
Wilson: "Task complete! Your app is ready with 85% test coverage."
```

### Current State
```
You: "Hey Wilson, could you build this app for me?"
Wilson (llama3:latest): [Uses delegate_task tool synchronously]
  [Blocks Wilson's chat while agent executes]
  [Both Wilson and agent potentially use same model]
  [No resource optimization - idle when not needed]
Wilson: "I've delegated this task to the code agent."
  [Result returned after completion - you couldn't chat during execution]
```

---

## Dual-Model Architecture Benefits

### Resource Optimization
**Problem:** Running large models (14B params) continuously wastes resources
**Solution:**
- Wilson: Always-on small model (~4GB RAM, fast responses)
- Workers: On-demand large models (~8-16GB RAM, spawn only when needed)
- **Savings:** ~75% idle resource usage reduction

### Performance Gains
**Problem:** Small models give poor code quality, large models are slow for chat
**Solution:**
- Wilson chat: Instant responses from llama3:latest
- Code tasks: High quality from qwen2.5-coder:14b
- Research: Deep analysis from mixtral or claude-3
- **Result:** Best tool for each job

### Concurrent Operation
**Problem:** Single model can't handle chat + work simultaneously
**Solution:**
- Wilson model: Dedicated to chat thread
- Worker models: Isolated per agent/task
- **No contention:** Chat never waits for agent work

---

## Gap Analysis

### ✅ What Already Works

1. **LLM Manager with Purpose-Based Routing** (`go/llm/manager.go`)
   - Multi-model support (chat, code, analysis, vision)
   - Purpose-based client selection
   - Fallback mechanism
   - Thread-safe access (RWMutex)
   - **PERFECT for dual-model design!**

2. **Agent Model Isolation** (`go/agent/base_agent.go`)
   - Each agent has `purpose` field (PurposeChat, PurposeCode, etc.)
   - `CallLLM()` uses `a.purpose` to get correct model
   - Already isolates model selection per agent
   - **Already supports dual-model!**

3. **Manager Agent & Task Infrastructure** (Phase 4 Complete)
   - Task creation, subtask breakdown
   - DoR/DoD validation
   - 8 states, 6 types
   - Dependencies, reviews

4. **Orchestration Tools** (Just migrated!)
   - `poll_tasks`, `claim_task`, `delegate_task`
   - `update_task_progress`, `unblock_tasks`
   - `request_review`, `submit_review`, `get_review_status`
   - **Ready for async workers!**

5. **Context Store** (SQLite)
   - Tasks, artifacts, communications
   - Multi-session persistence

6. **Chat Interface** (`go/interface/chat/`)
   - Status line with spinner
   - Separated from agent logic

### ❌ What's Missing (The Async + Dual-Model Gap)

#### 1. **Non-Blocking Task Execution** (Critical)
**Current:** `delegate_task` blocks Wilson's chat thread
**Needed:**
- Return task ID immediately
- Spawn goroutine for execution
- Wilson chat thread never blocks
- Agent work happens in separate goroutines with separate models

#### 2. **Background Worker Pool** (Critical)
**Current:** No worker goroutines
**Needed:**
- **Spawn-on-demand workers** (efficient - you're solo user)
- Each worker gets its own model instance
- Poll → claim → execute with worker's model → report
- Graceful shutdown

**Key Dual-Model Feature:**
```go
type AgentWorker struct {
    agent        Agent
    modelPurpose llm.Purpose  // Worker's dedicated model
    llmManager   *llm.Manager // Access to model pool
    pollInterval time.Duration
}

func (w *AgentWorker) executeTask(task *Task) {
    // Worker uses its OWN model (e.g., PurposeCode)
    // Wilson's chat model never touched
    result, err := w.agent.Execute(ctx, task)
}
```

#### 3. **Model Instance Management** (New - Critical for Dual)
**Current:** Single model instance per purpose
**Needed:**
- Wilson's PurposeChat model: Always loaded, never unloaded
- Worker models: Load on spawn, unload after idle timeout
- Model health checks before task execution
- Fallback if model unavailable

**Design:**
```go
type ModelPool struct {
    active map[string]*ModelInstance  // model_name -> instance
    mu     sync.RWMutex
}

type ModelInstance struct {
    client      llm.Client
    lastUsed    time.Time
    refCount    int  // Active users
    keepAlive   bool // Don't auto-unload (for Wilson's chat)
}
```

#### 4. **Live Status Updates** (High Priority)
**Current:** Status line only shows "Wilson is thinking..."
**Needed:**
- Real-time task progress broadcasting
- **Show which model is working:** "Code Agent (qwen2.5-coder:14b): writing auth... ⚙️"
- Wilson's chat model usage never shown (transparent)

#### 5. **Concurrent Chat** (Medium Priority - Already Mostly There!)
**Current:** Chat loop is single-threaded
**Already Good:**
- `session.History` can add mutex easily
- `HandleChat()` already delegates to agents
- Just needs to not block on `delegate_task`

**Needed:**
- Make `delegate_task` async
- Wilson's chat stays on main thread with PurposeChat model
- Worker tasks spawn in goroutines with their models

#### 6. **Task-Aware Context** (Medium Priority)
**Needed:**
```go
// Wilson knows about background tasks without blocking
systemPrompt += "\n\nBackground tasks:\n"
for _, task := range coordinator.GetActiveTasks() {
    systemPrompt += fmt.Sprintf("- %s (%s, using %s): %s\n",
        task.ID, task.Type, task.ModelName, task.Status)
}
```

---

## Dual-Model Implementation Details

### Model Loading Strategy

```
┌─────────────────────────────────────────┐
│         LLM Manager (Manager.go)        │
│                                         │
│  Registered Models:                     │
│  • chat: llama3:latest (always loaded)  │
│  • code: qwen2.5-coder:14b (on-demand)  │
│  • analysis: mixtral (on-demand)        │
└─────────────────────────────────────────┘
              │
    ┌─────────┴──────────┐
    │                    │
┌───▼───────┐   ┌────────▼──────────┐
│  Wilson   │   │  Worker Pool      │
│  (Chat)   │   │                   │
│           │   │  CodeWorker:      │
│  Model:   │   │   • Model: code   │
│  llama3   │   │   • Spawn when    │
│  Status:  │   │     task assigned │
│  Always   │   │                   │
│  Active   │   │  TestWorker:      │
└───────────┘   │   • Model: chat   │
                │   • Spawn when    │
                │     task assigned │
                └───────────────────┘
```

### Resource Timeline (Kill-After-Task Strategy)

```
Time    Wilson (Chat)         Code Agent Worker         Memory    Action
─────────────────────────────────────────────────────────────────────────────
0:00    Idle (llama3)         Not spawned               4GB       Waiting
0:01    User: "Build app"     Not spawned               4GB       Task delegated
0:02    Wilson: "Sure!"       Spawning worker...        4GB       Worker spawning
0:03    Chat ready            Loading qwen2.5-coder     10GB      Loading model
0:04    User: "Weather?"      Working (coding)          12GB      Task executing
0:05    Wilson: "Sunny"       Working (coding)          12GB      Still working
0:10    Chat ready            Task complete!            12GB      Task done
0:11    Chat ready            KILLED (immediate)        4GB       Worker terminated
                                                                   Model unloaded

--- If review finds issues ---
0:15    User: "Fix issues"    Not spawned               4GB       New task created
0:16    Wilson: "Fixing..."   Spawning NEW worker       4GB       Fresh start
0:17    Chat ready            Loading qwen2.5-coder     10GB      Model reload
0:18    Chat ready            Working (fixing)          12GB      Clean state
0:25    Chat ready            Fixed! KILLED             4GB       Immediate cleanup
```

### Configuration Format

`config.yaml`:
```yaml
llms:
  chat:
    provider: ollama
    model: llama3:latest
    temperature: 0.7
    keep_alive: true  # NEW: Never unload

  code:
    provider: ollama
    model: qwen2.5-coder:14b
    temperature: 0.2
    keep_alive: false  # NEW: Unload after 5min idle
    idle_timeout: 300  # NEW: seconds

  analysis:
    provider: ollama
    model: mixtral
    temperature: 0.3
    keep_alive: false
    idle_timeout: 300
```

---

## Implementation Roadmap (Updated for Dual-Model)

### Phase 0: Model Lifecycle Management (NEW - 2-3 hours)
**Goal:** Support on-demand model loading/unloading

**Changes:**
1. **Add Model Instance Tracking** (`go/llm/manager.go`)
   ```go
   type Manager struct {
       clients   map[Purpose]Client
       configs   map[Purpose]Config
       instances map[Purpose]*ModelInstance  // NEW
       mu        sync.RWMutex
   }

   type ModelInstance struct {
       client      Client
       lastUsed    time.Time
       refCount    int
       keepAlive   bool
       idleTimeout time.Duration
   }
   ```

2. **Add Reference Counting** (`go/llm/manager.go`)
   ```go
   func (m *Manager) AcquireModel(purpose Purpose) (Client, func(), error) {
       // Increment refCount, return client + release func
       // Load model if not loaded
   }

   func (m *Manager) releaseModel(purpose Purpose) {
       // Decrement refCount
       // Start idle timeout if refCount == 0 && !keepAlive
   }
   ```

3. **Add Idle Cleanup Goroutine** (`go/llm/manager.go`)
   ```go
   func (m *Manager) startCleanupRoutine(ctx context.Context) {
       ticker := time.NewTicker(30 * time.Second)
       for {
           select {
           case <-ctx.Done():
               return
           case <-ticker.C:
               m.unloadIdleModels()
           }
       }
   }
   ```

4. **Update Config** (`go/config/config.go`)
   ```go
   type LLMConfig struct {
       Provider     string
       Model        string
       Temperature  float64
       KeepAlive    bool           // NEW
       IdleTimeout  int            // NEW: seconds
   }
   ```

**Testing:**
```go
// Model stays loaded while in use
client, release, _ := manager.AcquireModel(llm.PurposeCode)
defer release()
// Use client...

// After release(), model unloads after IdleTimeout if !KeepAlive
```

---

### Phase 1: Async Foundation (2-3 hours)
**Goal:** Make delegation non-blocking, Wilson never waits

**Changes:**
1. **Modify `delegate_task` tool** (`go/capabilities/orchestration/delegate_task.go`)
   ```go
   // Before: synchronous
   result, err := coordinator.DelegateTask(ctx, req)

   // After: async
   taskID := coordinator.DelegateTaskAsync(ctx, req)
   return fmt.Sprintf("Task %s started. I'll update you on progress.", taskID)
   ```

2. **Add `coordinator.DelegateTaskAsync()`** (`go/agent/coordinator.go`)
   ```go
   func (c *Coordinator) DelegateTaskAsync(ctx context.Context, req DelegationRequest) string {
       task := createTask(req)
       c.tasks[task.ID] = task

       // Spawn goroutine - DOES NOT block Wilson
       go func() {
           // Get agent (e.g., Code Agent with PurposeCode)
           agent, _ := c.registry.Get(req.ToAgent)

           // Agent uses its OWN model (not Wilson's!)
           result, err := c.ExecuteTask(ctx, task, agent)

           c.storeResult(task.ID, result, err)
           c.notifyCompletion(task.ID)
       }()

       return task.ID
   }
   ```

3. **Create `check_task_progress` tool** (`go/capabilities/orchestration/check_task_progress.go`)
   ```go
   type ProgressResponse struct {
       TaskID       string
       Status       string
       AssignedTo   string
       ModelUsed    string  // NEW: Show which model is working
       Progress     float64
       Subtasks     []SubtaskInfo
   }
   ```

**Key Dual-Model Feature:**
- Wilson's `delegate_task` call returns immediately (uses llama3 for ~50ms)
- Code Agent spawns in background, loads qwen2.5-coder:14b (~2-3s)
- Wilson ready for next chat immediately

---

### Phase 2: Background Worker Pool (3-4 hours)
**Goal:** Agents automatically execute tasks with their specialized models

**Changes:**
1. **Add Agent Worker** (`go/agent/worker.go` - NEW FILE)
   ```go
   type AgentWorker struct {
       agent        Agent
       llmManager   *llm.Manager
       pollInterval time.Duration
       ctx          context.Context
       cancel       context.CancelFunc
       modelClient  llm.Client        // NEW: Worker's model instance
       releaseModel func()             // NEW: Cleanup function
   }

   func (w *AgentWorker) Start() {
       go w.workLoop()
   }

   func (w *AgentWorker) workLoop() {
       ticker := time.NewTicker(w.pollInterval)
       defer ticker.Stop()
       defer w.releaseModel()  // Cleanup model on exit

       for {
           select {
           case <-w.ctx.Done():
               return
           case <-ticker.C:
               w.pollAndExecute()
           }
       }
   }

   func (w *AgentWorker) pollAndExecute() {
       // Acquire model for this worker
       client, release, err := w.llmManager.AcquireModel(w.agent.Purpose())
       if err != nil {
           return // Model not available, skip this cycle
       }
       defer release()

       // Poll for tasks
       tasks := w.pollForTasks()
       for _, task := range tasks {
           if claimed := w.claimTask(task.ID); claimed {
               // Execute with worker's model
               w.executeTask(task)
           }
       }
   }
   ```

2. **Add Spawn-on-Demand Manager** (`go/agent/worker_manager.go` - NEW FILE)
   ```go
   type WorkerManager struct {
       workers    map[string]*AgentWorker
       llmManager *llm.Manager
       mu         sync.Mutex
       config     WorkerConfig
   }

   type WorkerConfig struct {
       MaxConcurrent  int
       KillAfterTask  bool  // Always true - immediate cleanup
       PreloadModels  bool  // Always false - on-demand only
   }

   func (wm *WorkerManager) SpawnWorker(agent Agent, task *Task) {
       wm.mu.Lock()
       defer wm.mu.Unlock()

       workerID := fmt.Sprintf("%s-%s", agent.Name(), task.ID)

       // Don't spawn if already exists
       if _, exists := wm.workers[workerID]; exists {
           return
       }

       // Create worker (will load model on first execute)
       worker := NewAgentWorker(agent, wm.llmManager, 5*time.Second, ctx)
       worker.Start()
       wm.workers[workerID] = worker

       // IMMEDIATE cleanup after task done (no idle period)
       go func() {
           <-task.CompletedChan
           // Kill immediately - always fresh workers for new tasks
           wm.KillWorker(workerID)  // Terminates goroutine, releases model
       }()
   }

   func (wm *WorkerManager) KillWorker(workerID string) {
       wm.mu.Lock()
       defer wm.mu.Unlock()

       worker, exists := wm.workers[workerID]
       if !exists {
           return
       }

       // Cancel worker context (stops work loop)
       worker.Stop()

       // Remove from tracking
       delete(wm.workers, workerID)

       // Worker's defer releaseModel() will unload the model
   }
   ```

3. **Update main.go** - NO always-on workers, NO pre-warming!
   ```go
   // Load configuration
   workerConfig := agent.WorkerConfig{
       MaxConcurrent: cfg.Workers.MaxConcurrent,  // Default: 2
       KillAfterTask: true,   // Always kill immediately
       PreloadModels: false,  // Never pre-warm - load on-demand only
   }

   // Create worker manager (not individual workers)
   workerMgr := agent.NewWorkerManager(llmManager, workerConfig, ctx)
   agent.SetGlobalWorkerManager(workerMgr)

   // ONLY Wilson's chat model loaded at startup
   // Workers spawn on-demand when tasks assigned (2-3s loading time)
   // Workers killed immediately after task completion (no idle period)
   // Revisions spawn fresh workers (clean state guaranteed)
   ```

**Resource Optimization (Kill-After-Task Strategy):**
- Idle: Wilson only (4GB RAM)
- Task starts: Wilson + worker loading (8GB → 12GB over 2-3s)
- Task active: Wilson + 1 worker (12GB RAM)
- Task completes: Worker killed immediately (12GB → 4GB within 1s)
- **No idle period:** Worker never waits for potential next task
- Multiple tasks: Wilson + up to max_concurrent workers (default: 2)
- **Fresh workers for revisions:** Review finds issues → old worker gone → new worker spawned

---

### Phase 3: Live Status Updates (2-3 hours)
**Goal:** Real-time progress with model visibility

**Changes:**
1. **Status Broadcasting** (`go/agent/status_broadcaster.go` - NEW FILE)
   ```go
   type StatusUpdate struct {
       TaskID      string
       Agent       string
       ModelName   string   // NEW: Show which model
       ModelSize   string   // NEW: e.g., "14B params"
       Status      string
       Message     string
       Progress    float64
       ResourceMB  int      // NEW: Current RAM usage
   }
   ```

2. **Enhanced Status Display** (`go/interface/chat/interface.go`)
   ```go
   func (i *Interface) ShowTaskStatus(update StatusUpdate) {
       // Show model being used
       msg := fmt.Sprintf("%s (%s): %s (%.0f%%)",
           update.Agent,
           update.ModelName,
           update.Message,
           update.Progress*100)

       i.status.ShowWithSpinner(msg)
   }
   ```

**Example Output:**
```
[Status: Code Agent (qwen2.5-coder:14b): writing auth module (33%) ⚙️]
[Status: Test Agent (llama3): running unit tests (80%) 🧪]
```

---

### Phase 4: Concurrent Chat (1-2 hours)
**Goal:** Chat with Wilson while tasks run with different models

**Changes:**
1. **Thread-safe History** (`go/session/history.go`)
   ```go
   type History struct {
       messages []Message
       maxTurns int
       mu       sync.RWMutex  // ADD THIS
   }
   ```

2. **Task-Aware System Prompt** (`go/agent/chat_agent.go`)
   ```go
   func (a *ChatAgent) buildSystemPrompt() string {
       prompt := basePrompt

       // Add active task context
       tasks := coordinator.GetActiveTasks()
       if len(tasks) > 0 {
           prompt += "\n\nActive background tasks you're coordinating:\n"
           for _, task := range tasks {
               prompt += fmt.Sprintf("- %s (%s): %s (using %s model)\n",
                   task.ID, task.Type, task.Status, task.ModelName)
           }
           prompt += "You can check their status or answer questions while they work.\n"
       }

       return prompt
   }
   ```

**Wilson's Awareness:**
```
You: "What's 2+2?"
Wilson (knows Code Agent is working in background):
  "4. By the way, the Code Agent is 60% done writing your auth module
   using the qwen2.5-coder:14b model."
```

---

### Phase 5: Model Health & Fallback (1-2 hours)
**Goal:** Graceful handling of model unavailability

**Changes:**
1. **Pre-Task Model Check** (`go/agent/worker.go`)
   ```go
   func (w *AgentWorker) executeTask(task *Task) (*Result, error) {
       // Check if preferred model available
       client, release, err := w.llmManager.AcquireModel(w.agent.Purpose())
       if err != nil {
           // Try fallback
           client, release, err = w.llmManager.AcquireModel(llm.PurposeChat)
           if err != nil {
               return nil, fmt.Errorf("no model available for task")
           }
           task.UsedFallback = true
       }
       defer release()

       // Execute with acquired model
       return w.agent.Execute(ctx, task)
   }
   ```

2. **Model Status Tool** (`go/capabilities/system/model_status.go` - NEW)
   ```go
   // Returns active models, RAM usage, availability
   ```

---

## Updated Architecture Diagram (Dual-Model)

```
┌─────────────────────────────────────────────────────────────────┐
│                         USER (You)                              │
│                  Terminal Chat Interface                        │
└───────────────────┬─────────────────────┬───────────────────────┘
                    │                     │
         ┌──────────▼──────────┐ ┌────────▼──────────┐
         │  Chat Handler       │ │  Status Display   │
         │  (main goroutine)   │ │  (shows models)   │
         │                     │ │                   │
         │  Wilson Chat Agent  │ │  "Code Agent      │
         │  Model: llama3      │ │   (qwen2.5:14b)   │
         │  Status: Always On  │ │   writing... 60%" │
         └──────────┬──────────┘ └────────▲──────────┘
                    │                     │
                    │              StatusUpdate(model_name)
         ┌──────────▼─────────────────────┴──────────┐
         │          Coordinator                       │
         │  - DelegateTaskAsync() [non-blocking]     │
         │  - Status broadcasting                     │
         └──────────┬──────────────────┬──────────────┘
                    │                  │
      ┌─────────────▼─────┐   ┌────────▼────────┐
      │  Manager Agent    │   │  Task Queue     │
      │  (on-demand)      │   │  (SQLite)       │
      │  Model: chat      │   │  + model_used   │
      └─────────┬─────────┘   └─────────────────┘
                │
      ┌─────────┴─────────────────────────────────┐
      │            Worker Manager                 │
      │  (spawns workers on-demand)               │
      └─────────┬──────────────────┬──────────────┘
                │                  │
      ┌─────────▼────────┐  ┌──────▼──────────┐
      │ Code Worker      │  │  Test Worker    │
      │ (goroutine)      │  │  (goroutine)    │
      │                  │  │                 │
      │ Agent: Code      │  │ Agent: Test     │
      │ Model: qwen:14b  │  │ Model: llama3   │
      │ Status: Working  │  │ Status: Idle    │
      │ RAM: ~8GB        │  │ RAM: ~4GB       │
      │                  │  │                 │
      │ Loop:            │  │ Loop:           │
      │ 1.poll_tasks     │  │ 1.poll_tasks    │
      │ 2.claim_task     │  │ 2.claim_task    │
      │ 3.acquire_model  │  │ 3.acquire_model │
      │ 4.execute        │  │ 4.execute       │
      │ 5.release_model  │  │ 5.release_model │
      └──────────────────┘  └─────────────────┘
                │                   │
                └───────────────────┘
                          │
             ┌────────────▼─────────────┐
             │    LLM Manager           │
             │                          │
             │  Model Pool:             │
             │  • chat (llama3)         │
             │    - Always loaded       │
             │    - RefCount: 1         │
             │  • code (qwen2.5:14b)    │
             │    - Loaded on demand    │
             │    - RefCount: 1         │
             │    - Idle timeout: 5min  │
             └──────────────────────────┘
```

---

## Design Decisions (Confirmed)

### Resource-First Strategy
**Philosophy:** Optimize for minimal resource usage, spawn fresh for quality

1. **Model Loading: On-Demand Only**
   - No pre-warming of models on startup
   - Models load on first task assignment (~2-3s latency acceptable)
   - Only Wilson's chat model loaded at startup
   - **Rationale:** Minimize idle memory, faster startup

2. **Worker Lifecycle: Kill Immediately After Task**
   - Workers terminate as soon as task completes
   - No keep-alive period
   - Revisions (e.g., after review feedback) spawn fresh workers
   - **Rationale:** Maximize resource availability, ensure clean state
   - **Example:** Review Agent finds issues → Code Agent worker terminated → New Code Agent worker spawned for fixes

3. **Concurrency Limit: Default 2 Workers**
   - Max 2 concurrent workers by default
   - Configurable per-machine capacity
   - Wilson's chat model doesn't count toward limit
   - **Rationale:** Safe for 16GB RAM machines (4GB Wilson + 2x6GB workers = 16GB max)

4. **Machine-Adaptive Configuration**
   - Easy model swapping via config.yaml
   - Different profiles for different machines
   - Example profiles included for:
     - Low-end (8GB RAM): llama3 everywhere
     - Mid-range (16GB RAM): llama3 chat + qwen2.5-coder:7b
     - High-end (32GB RAM): llama3 chat + qwen2.5-coder:14b + mixtral

---

## Estimated Effort (Updated)

| Phase | Description | Files | Effort | Status |
|-------|-------------|-------|--------|--------|
| 0 | Model Lifecycle (on-demand only) | 2 modified | 2-3h | ✅ **COMPLETE** |
| 1 | Async Foundation | 2 new, 1 modified | 2-3h | ✅ **COMPLETE** |
| 2 | Model Lifecycle + Concurrency | 1 modified (simplified) | 1h | ✅ **COMPLETE** |
| 3 | Status Updates (with model info) | 2 modified | 30min | ✅ **COMPLETE** |
| 4 | Concurrent Chat (task-aware) | 2 modified | 30min | ✅ **COMPLETE** |
| 5 | Model Health & Fallback | 3 modified, 1 new | 1h | ✅ **COMPLETE** |
| **Total** | | **~15 files** | **11-17 hours** | **✅ ALL COMPLETE!** |

---

## Dual-Model Success Criteria

After implementation, you should be able to:

✅ **Chat with Wilson while code agent works**
```
You: "Build a todo app"
Wilson (llama3, instant): "Starting task TASK-001. Using Code Agent with qwen2.5-coder."
  [Status: Code Agent (qwen2.5-coder:14b): loading model... ⏳]
You: "What's 2+2?"
Wilson (llama3, instant): "4. Your todo app task is 20% complete."
  [Status: Code Agent (qwen2.5-coder:14b): writing database models (40%) ⚙️]
```

✅ **See which model is doing what**
```
[Status: Code Agent (qwen2.5-coder:14b, 8GB RAM): implementing auth (60%) ⚙️]
[Status: Test Agent (llama3, 4GB RAM): writing tests (80%) 🧪]
```

✅ **Efficient resource usage (Kill-After-Task)**
```
Idle state:
- Wilson: llama3 loaded (4GB)
- Workers: None (0GB)
- Total: 4GB

Active state (1 code task):
- Wilson: llama3 loaded (4GB)
- Code Worker: qwen2.5-coder:14b (8GB)
- Total: 12GB

After completion (IMMEDIATE):
- Wilson: llama3 loaded (4GB)
- Code Worker: KILLED - unloaded (0GB)
- Total: 4GB [IMMEDIATE CLEANUP!]

Revision workflow:
- Review Agent finds issues (4GB - only Wilson)
- New Code Worker spawns (4GB → 12GB in 2-3s)
- Fixes applied with fresh state
- Worker killed immediately after (12GB → 4GB)
```

✅ **Query model status**
```
You: "What models are running?"
Wilson: "Currently active:
  • Chat (me): llama3:latest (always on, 4GB)
  • Code Agent: qwen2.5-coder:14b (working on TASK-001, 8GB)
  Total RAM: 12GB"
```

✅ **Fallback when model unavailable**
```
[Status: Code Agent: qwen2.5-coder:14b not available, using llama3 fallback ⚠️]
Wilson: "FYI - Code Agent is using the chat model as fallback. Quality may vary."
```

---

## Risk Mitigation (Updated for Dual-Model)

### Risk 1: Model Loading Latency
**Problem:** Loading 14B model takes 2-3 seconds on every task (no pre-warming)
**Solution:**
- Accept 2-3s latency as tradeoff for resource savings
- Return task ID immediately, show "loading model..." status
- User sees: "Task started. Loading code model..." then "Coding..."
- **Design choice:** We chose resource efficiency over first-task speed
- Future optimization: Optional model pre-warming config for power users

### Risk 2: Memory Exhaustion
**Problem:** Multiple large models overflow RAM
**Solution:**
- Set `max_concurrent_workers` config (default: 2)
- Model manager enforces memory limits
- Queue tasks if at capacity
- User sees: "Task queued (2 workers active, max capacity)"

### Risk 3: Model Deadlock
**Problem:** All workers acquire models, Wilson starved
**Solution:**
- Wilson's chat model always has priority (KeepAlive: true)
- Worker models use AcquireModel() with timeout
- Fail fast if model unavailable > 30s

### Risk 4: Orphaned Model Instances
**Problem:** Worker crashes, model stays loaded
**Solution:**
- Reference counting with defer release()
- Worker context cancellation triggers cleanup
- No idle timeout needed - workers killed immediately after task
- Periodic cleanup goroutine as safety net (every 60s)
- Force-unload any model with refCount=0 (should never happen with immediate kill)

### Risk 5: Context Confusion
**Problem:** Worker's model context leaks into Wilson's chat
**Solution:**
- Each agent has isolated LLM context
- Worker goroutines have separate contexts
- No shared state between Wilson and workers

---

## Testing Strategy (Dual-Model)

### Unit Tests
```go
// Test model acquisition and release
func TestModelLifecycle(t *testing.T) {
    manager := llm.NewManager()

    // Acquire model
    client, release, err := manager.AcquireModel(llm.PurposeCode)
    assert.NoError(t, err)
    assert.Equal(t, 1, manager.GetRefCount(llm.PurposeCode))

    // Release model
    release()
    assert.Equal(t, 0, manager.GetRefCount(llm.PurposeCode))

    // Model should unload after IdleTimeout
    time.Sleep(6 * time.Second)
    assert.False(t, manager.IsLoaded(llm.PurposeCode))
}

// Test async delegation doesn't block
func TestAsyncDelegationNonBlocking(t *testing.T) {
    coordinator := NewCoordinator(registry)

    start := time.Now()
    taskID := coordinator.DelegateTaskAsync(ctx, req)
    elapsed := time.Since(start)

    // Should return immediately (< 100ms)
    assert.Less(t, elapsed.Milliseconds(), 100)
    assert.NotEmpty(t, taskID)
}
```

### Integration Tests
```go
// Test dual model operation
func TestDualModelConcurrency(t *testing.T) {
    // Start Wilson chat
    chatAgent := NewChatAgent(llmManager, contextMgr)

    // Delegate code task (spawns worker with code model)
    taskID := coordinator.DelegateTaskAsync(ctx, CodeTaskRequest)

    // Chat with Wilson immediately (uses chat model)
    start := time.Now()
    response, err := chatAgent.Execute(ctx, SimpleTask{"What's 2+2?"})
    elapsed := time.Since(start)

    // Chat should respond quickly despite background work
    assert.NoError(t, err)
    assert.Less(t, elapsed.Milliseconds(), 500) // Fast response
    assert.Contains(t, response.Output, "4")

    // Verify code task still running with different model
    task, _, _ := coordinator.GetTaskStatus(taskID)
    assert.Equal(t, TaskInProgress, task.Status)
    assert.Equal(t, "qwen2.5-coder:14b", task.ModelUsed)
}
```

### Manual Testing
```bash
# Test dual-model scenario
$ go run main.go
You: build a complex authentication system
Wilson: Task TASK-001 started. Using Code Agent with qwen2.5-coder:14b model.
[Status: Code Agent (qwen2.5-coder:14b): loading model... ⏳]
You: what's the capital of France?  # IMMEDIATE RESPONSE TEST
Wilson: Paris. By the way, your auth task is 10% complete.
[Status: Code Agent (qwen2.5-coder:14b): writing user model (20%) ⚙️]
You: how's the task going?
Wilson: Task TASK-001 is 45% complete. Code Agent has written 3/7 files.
[Status: Code Agent (qwen2.5-coder:14b): implementing password hashing (60%) ⚙️]
<Wait for completion>
Wilson: Task TASK-001 complete! Auth system ready with 85% test coverage.
[Status: Code Agent: task complete, unloading model... ✓]
<5 minutes later>
$ ps aux | grep ollama  # Code model should be unloaded
# Only llama3 (Wilson's chat model) should remain
```

---

## Configuration Examples

### Base Configuration (Mid-Range: 16GB RAM)
`config.yaml`:
```yaml
llms:
  chat:
    provider: ollama
    model: llama3:latest
    temperature: 0.7
    keep_alive: true  # Wilson's model never unloads
    # ~4GB RAM

  code:
    provider: ollama
    model: qwen2.5-coder:14b
    temperature: 0.2
    keep_alive: false  # Kill immediately after task
    idle_timeout: 0  # No idle period - immediate unload
    # ~8GB RAM when active

  analysis:
    provider: ollama
    model: llama3:latest  # Reuse chat model for analysis
    temperature: 0.3
    keep_alive: false
    idle_timeout: 0

workers:
  max_concurrent: 2  # Safe for 16GB (4GB + 2x6GB = 16GB)
  spawn_mode: "on_demand"  # Always on-demand
  kill_after_task: true  # Terminate immediately after completion
  preload_models: false  # No pre-warming - load on first use

resources:
  max_memory_mb: 16384  # 16GB total limit
  warn_threshold_mb: 14336  # Warn at 14GB (87%)
```

---

### Machine Profiles

#### Low-End Machine (8GB RAM)
```yaml
llms:
  chat:
    provider: ollama
    model: llama3:latest  # 4GB
    keep_alive: true

  code:
    provider: ollama
    model: llama3:latest  # Reuse same model (no extra RAM)
    keep_alive: false
    idle_timeout: 0

workers:
  max_concurrent: 1  # Only 1 worker at a time
  kill_after_task: true

resources:
  max_memory_mb: 8192
  warn_threshold_mb: 7168
```

#### High-End Machine (32GB RAM)
```yaml
llms:
  chat:
    provider: ollama
    model: llama3:latest  # 4GB
    keep_alive: true

  code:
    provider: ollama
    model: qwen2.5-coder:32b  # High quality, 16GB
    keep_alive: false
    idle_timeout: 0

  analysis:
    provider: ollama
    model: mixtral:8x7b  # 12GB
    keep_alive: false
    idle_timeout: 0

workers:
  max_concurrent: 2  # Can run code + analysis simultaneously
  kill_after_task: true

resources:
  max_memory_mb: 32768
  warn_threshold_mb: 28672
```

#### Cloud/GPU Machine (64GB+ RAM)
```yaml
llms:
  chat:
    provider: ollama
    model: llama3:latest
    keep_alive: true

  code:
    provider: ollama
    model: deepseek-coder:33b  # Best quality
    keep_alive: false
    idle_timeout: 0

  analysis:
    provider: ollama
    model: mixtral:8x22b  # Best reasoning
    keep_alive: false
    idle_timeout: 0

workers:
  max_concurrent: 4  # Can run multiple large models
  kill_after_task: true

resources:
  max_memory_mb: 65536
  warn_threshold_mb: 57344
```

### Easy Model Switching
Users can quickly swap models by editing config.yaml:
```bash
# Switch code model to smaller version
sed -i 's/qwen2.5-coder:14b/qwen2.5-coder:7b/' config.yaml

# Switch to larger model if resources available
sed -i 's/qwen2.5-coder:7b/qwen2.5-coder:32b/' config.yaml

# Use local profile for quick switching
cp config-lowend.yaml config.yaml    # For 8GB machine
cp config-midrange.yaml config.yaml  # For 16GB machine
cp config-highend.yaml config.yaml   # For 32GB machine
```

---

## Key Advantages Over Single-Model Design

| Aspect | Single Model | Dual-Model (This Design) |
|--------|-------------|--------------------------|
| **Chat Responsiveness** | Slow when busy | Always fast |
| **Code Quality** | Limited by small model | High quality from 14B model |
| **RAM Usage (Idle)** | 8-12GB | 4GB (62% savings) |
| **RAM Usage (Working)** | 8-12GB | 12GB |
| **Concurrent Tasks** | Impossible | Yes, with isolated models |
| **Resource Efficiency** | Always loaded | Load on demand |
| **User Experience** | Blocking | Non-blocking |

---

## Summary

**The Good News:** Your infrastructure is ~70% ready for dual-model async!

**What exists:**
- ✅ LLM Manager with purpose-based routing
- ✅ Agents with isolated model selection
- ✅ Task infrastructure and orchestration tools
- ✅ Context store and chat interface

**What's needed:**
1. Model lifecycle management (2-3h)
2. Async delegation (2-3h)
3. Spawn-on-demand workers with model acquisition (3-4h)
4. Live status with model visibility (2-3h)
5. Concurrent chat with task awareness (1-2h)

**Total: 11-17 hours** to achieve your ideal dual-model async design.

**Next Steps:**
1. Implement Phase 0: Model lifecycle management
2. Test model loading/unloading
3. Implement Phase 1: Async delegation
4. Add spawn-on-demand workers (Phase 2)
5. Test dual-model concurrent operation
6. Add status updates showing which model is working
7. Iterate based on resource usage and UX

**The Result:** Wilson stays responsive with a small model while background agents use large, specialized models for high-quality work - all automatically managed with optimal resource usage through aggressive cleanup and on-demand spawning.

**Key Philosophy:** Resource efficiency through immediate cleanup. Every worker is ephemeral - spawn fresh, work fast, die immediately. This ensures minimal idle resources and clean state for every task.

Let's build the dual-model async future! 🚀🤖
