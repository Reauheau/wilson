# Wilson

**A local-first, async multi-agent AI assistant built for autonomous task execution.**

Wilson is a Go-based CLI tool that orchestrates specialized AI agents to collaboratively complete complex tasks with minimal human intervention. Built on Ollama for local model execution, ensuring privacy and zero API costs.

## Key Features

- **Async Dual-Model Architecture** - Small chat model (always responsive) + large worker models (on-demand)
- **Multi-Agent Collaboration** - Research, Code, Test, and Review agents work together autonomously
- **Resource Efficient** - Kill-after-task strategy: 4GB idle, 12GB active, back to 4GB when done
- **Non-Blocking** - Chat with Wilson while background tasks execute
- **Code Intelligence** - AST parsing, compilation loops, test execution with 90%+ success rate
- **Quality Assurance** - Built-in DoR/DoD validation and agent review processes

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                      USER                            │
│              (Always responsive CLI)                 │
└─────────────────────┬────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────┐
│           WILSON (Chat Agent)                        │
│  Model: qwen2.5:7b (always loaded, 4GB)            │
│  Role: Intent classification, tool execution,        │
│        async task delegation                         │
│  Status: NON-BLOCKING - Returns immediately          │
└─────────────────────┬────────────────────────────────┘
                      │
                      ▼ (async - returns task ID)
┌──────────────────────────────────────────────────────┐
│                  COORDINATOR                         │
│  DelegateTaskAsync() - spawns goroutine             │
│  Status broadcaster - real-time updates             │
└──────────┬───────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────┐
│              WORKER MANAGER                          │
│  Strategy: Spawn on-demand, kill after completion   │
│  Max concurrent: 2 workers (configurable)           │
│  Model lifecycle: Load → Execute → Unload           │
└──────┬──────────────┬──────────────┬─────────────────┘
       │              │              │
       ▼              ▼              ▼
┌────────────┐ ┌────────────┐ ┌────────────┐
│ CODE       │ │ RESEARCH   │ │ TEST       │
│ WORKER     │ │ WORKER     │ │ WORKER     │
│(goroutine) │ │(goroutine) │ │(goroutine) │
│            │ │            │ │            │
│ Model:     │ │ Model:     │ │ Model:     │
│ qwen2.5-   │ │ qwen2.5    │ │ qwen2.5    │
│ coder:14b  │ │ 7b         │ │ 7b         │
│ (~8GB)     │ │ (~4GB)     │ │ (~4GB)     │
│            │ │            │ │            │
│ Tools:     │ │ Tools:     │ │ Tools:     │
│ - read     │ │ - search   │ │ - run      │
│ - write    │ │ - fetch    │ │ - test     │
│ - compile  │ │ - analyze  │ │ - report   │
│            │ │            │ │            │
│ Life:      │ │ Life:      │ │ Life:      │
│ EPHEMERAL  │ │ EPHEMERAL  │ │ EPHEMERAL  │
└──────┬─────┘ └──────┬─────┘ └──────┬─────┘
       │              │              │
       └──────────────┴──────────────┘
                      │
                      ▼
         ┌────────────────────────┐
         │   CONTEXT STORE        │
         │   (SQLite DB)          │
         │                        │
         │ - Tasks + Status       │
         │ - Artifacts            │
         │ - Agent Notes          │
         │ - Reviews              │
         └────────────────────────┘
```

**Resource Profile (16GB Machine):**
- **Idle:** 4GB (Wilson only)
- **Active:** 12GB (Wilson + 1 worker with model loaded)
- **Done:** 4GB (Worker killed, memory released)

**Worker Lifecycle:** Spawn → Load Model → Execute → Kill Immediately

## Quick Start

### Prerequisites

- Go 1.21+
- [Ollama](https://ollama.ai) installed and running
- 16GB+ RAM recommended (8GB minimum)

### Installation

```bash
# Clone the repository
git clone https://github.com/reauheau/wilson.git
cd wilson

# Pull required models (choose based on your RAM - see Model Recommendations below)
ollama pull qwen2.5:7b          # Chat & analysis (4GB)
ollama pull qwen2.5-coder:14b   # Code generation (8GB)

# Build and run
cd go
go build -o wilson main.go
./wilson
```

### Global Command Setup

For convenience, set up Wilson as a global command. See [SETUP_ALIAS.md](SETUP_ALIAS.md) for instructions.

```bash
# After setup, run from anywhere:
wilson
```

## Usage Examples

**Simple chat:**
```
You: Hello Wilson
Wilson: Hi! I'm ready to help. [<50ms response]
```

**Complex task (async):**
```
You: Build a REST API for user management
Wilson: Task TASK-001 started. Using Code Agent with qwen2.5-coder:14b.
  [Status: Code Agent (qwen2.5-coder:14b): implementing endpoints (40%) ⚙️]

You: What's 2+2?  [IMMEDIATE response while agent works]
Wilson: 4. Your API task is 60% complete.

Wilson: Done! Created 5 endpoints with auth, all tests passing (92% coverage).
```

## Model Recommendations

Choose models based on your available RAM. Edit `go/config/tools.yaml` to configure:

### Low-End (8GB RAM)
```yaml
chat: qwen2.5:3b        # 2GB - Good tool calling, basic conversation
analysis: qwen2.5:3b    # 2GB - Decent analysis
code: qwen2.5:7b        # 4GB - Smaller code model
```
**Best for:** Basic tasks, limited resources, single worker

### Mid-Range (16GB RAM) - **RECOMMENDED**
```yaml
chat: qwen2.5:7b        # 4GB - Excellent tool calling, good conversation
analysis: qwen2.5:7b    # 4GB - Strong analysis
code: qwen2.5-coder:14b # 8GB - Professional code generation
```
**Best for:** Most users, 2 concurrent workers, balanced performance

### High-End (32GB+ RAM)
```yaml
chat: qwen2.5:7b           # 4GB - Fast chat
analysis: qwen2.5:14b      # 8GB - Deep analysis
code: qwen2.5-coder:32b    # 16GB - Production-grade code
```
**Best for:** Complex projects, 2-4 concurrent workers, maximum quality

### Model Characteristics

| Purpose | Recommended | Why |
|---------|-------------|-----|
| **Chat** | qwen2.5:3b or 7b | Better tool calling than llama3, always loaded |
| **Analysis** | qwen2.5:7b | Good reasoning, research, web analysis |
| **Code** | qwen2.5-coder:14b | Specialized for code, best quality/size ratio |

**Note:** All qwen2.5 models have better structured output (tool calling) than llama3, even at smaller sizes.

## Documentation

- **[ENDGAME.md](ENDGAME.md)** - Vision and architecture overview
- **[DONE.md](DONE.md)** - Implementation history and key learnings
- **[TODO.md](TODO.md)** - Roadmap and upcoming features
- **[SESSION_INSTRUCTIONS.md](SESSION_INSTRUCTIONS.md)** - Development guidelines

## Tech Stack

- **Language:** Go
- **LLM Runtime:** Ollama (local models)
- **Database:** SQLite
- **Architecture:** Multi-agent with async coordination

## Statistics

- **Codebase:** ~10,000 lines of Go
- **Agents:** 6 (Chat, Manager, Code, Test, Research, Review)
- **Tools:** 30+ (filesystem, code intelligence, orchestration, web, system)
- **Tests:** 62 (44 unit + 18 integration)

## License

[Your License Here]

## Author

Roderick van Nievelt

---

*"The goal is not to replace the developer, but to amplify their capabilities by handling the repetitive, the tedious, and the time-consuming."*
