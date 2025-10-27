# Wilson

**A local-first, async multi-agent AI assistant built for autonomous task execution.**

Wilson is a Go-based CLI tool that orchestrates specialized AI agents to collaboratively complete complex tasks with minimal human intervention. Built on Ollama for local model execution, ensuring privacy and zero API costs.

## Key Features

- **Self-Healing Feedback Loop** - Automatic failure detection and recovery (93% success rate)
- **Async Dual-Model Architecture** - Small chat model (always responsive) + large worker models (on-demand)
- **Multi-Language LSP Support** - 6 code intelligence tools (diagnostics, navigation, refactoring) for Go, Python, JavaScript/TypeScript, Rust
- **Git Context Integration** - 7 git tools, branch-aware safety, agents see modified files and repo state
- **Smart Error Handling** - 80% of compile errors auto-fixed in <5s, complex errors escalated intelligently
- **Context-Aware Execution** - Tasks inherit full context (project_path, dependency files, git state, error history)
- **Multi-Agent Collaboration** - Research, Code, Test, and Review agents work together autonomously
- **Atomic Task Execution** - Each task = 1 file, dependency injection, zero "max iterations" errors
- **Resource Efficient** - Kill-after-task strategy: 4GB idle, 12GB active, back to 4GB when done
- **Non-Blocking** - Chat with Wilson while background tasks execute
- **Code Intelligence** - LSP diagnostics/navigation/refactoring (<500ms), AST parsing, compilation loops, test execution
- **Quality Assurance** - Built-in DoR/DoD validation and precondition checks

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
┌──────────────────┐              ┌───────────────────┐
│  MANAGER AGENT   │◄─────────────│  FEEDBACK BUS     │
│  + FEEDBACK      │  (events)    │  (Go Channel)     │
│  HANDLER         │              │                   │
│                  │              │  Types:           │
│  Role:           │              │  • dependency     │
│  • Orchestrate   │              │  • retry          │
│  • Create deps   │              │  • blocker        │
│  • Error analyze │              │  • success        │
│  • Smart retry   │              │                   │
│  • Context flow  │              │  93% success rate │
└──────────┬───────┘              └─────────▲─────────┘
           │                                │
           ▼                                │ feedback
┌──────────────────────────────────────────┼───────────┐
│         TASK QUEUE (SQLite)              │           │
│  • Tasks + DoR/DoD + TaskContext         │           │
│  • Dependencies (DependsOn, Blocks)      │           │
│  • Input (project_path, dependency_files)│           │
│  • Error history + Auto-unblock          │           │
└──────────┬───────────────────────────────┘           │
           │                                           │
           ▼                                           │
┌──────────────────────────────────────────────────────┤
│              WORKER MANAGER                          │
│  Strategy: Spawn on-demand, kill after completion   │
│  Max concurrent: 2 workers (configurable)           │
│  Context: project_path, dependency_files, errors    │
└──────┬──────────────┬──────────────┬─────────────────┘
       │              │              │
       ▼              ▼              ▼
┌────────────┐ ┌────────────┐ ┌────────────┐
│ CODE       │ │ TEST       │ │ REVIEW     │
│ WORKER     │ │ WORKER     │ │ WORKER     │
│(goroutine) │ │(goroutine) │ │(goroutine) │
│            │ │            │ │            │
│ Features:  │ │ Features:  │ │ Features:  │
│ • Precheck │ │ • Precheck │ │ • Precheck │
│ • Compile  │ │ • Read deps│ │ • Quality  │
│ • Auto-fix │ │ • Feedback │ │ • Feedback │
│            │ │            │ │            │
│ Feedback:  │ │ Feedback:  │ │ Feedback:  │
│ → Manager  │ │ → Manager  │ │ → Manager  │
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
         │ - Tasks + TaskContext  │
         │ - Artifacts + Files    │
         │ - Agent Feedback       │
         │ - Error Patterns       │
         │ - Dependency Graph     │
         └────────────────────────┘
```

**Resource Profile (16GB Machine):**
- **Idle:** 4GB (Wilson only)
- **Active:** 12GB (Wilson + 1 worker with model loaded)
- **Feedback:** Worker → FeedbackBus → Manager (async, non-blocking)
- **Recovery:** Manager creates dependency → Worker respawns → Retry ✓
- **Done:** 4GB (Worker killed, memory released)

**Self-Healing Flow:** Worker detects issue → Sends feedback → Manager creates recovery task → Auto-unblock → Retry with context

## Quick Start

### Prerequisites

**Required:**
- Go 1.21+
- [Ollama](https://ollama.ai) installed and running
- 16GB+ RAM recommended (8GB minimum)

**Optional (for multi-language LSP support):**
- Language servers for non-Go languages:
  ```bash
  # Run the installation script
  ./scripts/install_language_servers.sh

  # Or install manually:
  npm install -g pyright typescript-language-server typescript
  pip3 install 'python-lsp-server[all]'
  rustup component add rust-analyzer
  ```

**Optional (for MCP servers):**
- Node.js 18+ and npm (for external tool integrations via MCP)
  ```bash
  # macOS
  brew install node

  # Ubuntu/Debian
  sudo apt install nodejs npm

  # Windows
  # Download from https://nodejs.org
  ```

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

# Optional: Enable MCP external tools (requires Node.js)
# Edit go/config/tools.yaml and set mcp.enabled: true
# Wilson will automatically download MCP servers via npx on first use
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

**Complex task with self-healing (async):**
```
You: Build a REST API for user management
Wilson: Task TASK-001 started. Using Code Agent with qwen2.5-coder:14b.
  [Status: Code Agent (qwen2.5-coder:14b): implementing endpoints (40%) ⚙️]

You: What's 2+2?  [IMMEDIATE response while agent works]
Wilson: 4. Your API task is 60% complete.

Wilson: Done! Created 5 endpoints with auth, all tests passing (92% coverage).
```

**Automatic error recovery:**
```
You: Run tests in ~/myproject
Wilson: Task TASK-001 started.
  [Status: Test Agent: No test files found - creating dependency task...]
  [Status: Code Agent (qwen2.5-coder:14b): writing tests based on main.go (60%) ⚙️]
  [Status: Dependency complete, retrying original task...]
  [Status: Test Agent: running tests (100%) 🧪]
Wilson: Done! Created 3 test files, all tests passing (85% coverage).
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

## MCP Integration (Model Context Protocol)

Wilson supports the [Model Context Protocol](https://modelcontextprotocol.io) for standardized external tool access.

**What is MCP?**
- Open protocol by Anthropic for connecting LLMs to external data sources
- Standardized way to add tools without hardcoding integrations
- Growing ecosystem of community servers

**Available MCP Servers:**
- ✅ **Filesystem** - File operations (14 tools) - Enabled by default
- **GitHub** - Issues, PRs, repos - Requires `GITHUB_TOKEN`
- **Postgres** - Database queries - Requires `DATABASE_URL`
- **Slack** - Send messages, read channels - Requires `SLACK_BOT_TOKEN`
- **Telegram** - Chat with Wilson from your phone! - Requires `TELEGRAM_BOT_TOKEN`
- **Memory** - Persistent key-value storage - No API keys needed
- [View 20+ more servers](https://github.com/modelcontextprotocol/servers)

**Quick Setup:**
```bash
# 1. Set API keys (example for GitHub)
export GITHUB_TOKEN="ghp_your_token"

# 2. Enable in config
# Edit go/config/tools.yaml, set github.enabled: true

# 3. Restart Wilson
```

📖 **Full setup guide:** See [MCP_SETUP.md](MCP_SETUP.md) for detailed instructions

**Configuration:**
```yaml
# go/config/tools.yaml
mcp:
  enabled: true
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/workspace"]
      enabled: true
```

On first run, Wilson automatically downloads MCP servers via `npx`.

**Status:** ✅ Production-Ready - Phases 1-3 complete, extensible architecture

---

## Documentation

- **[MCP_SETUP.md](MCP_SETUP.md)** - MCP server setup guide (GitHub, Slack, Postgres, etc.)
- **[ENDGAME.md](ENDGAME.md)** - Vision and architecture overview (includes MCP)
- **[DONE.md](DONE.md)** - Implementation history and key learnings
- **[TODO.md](TODO.md)** - Roadmap and upcoming features
- **[SESSION_INSTRUCTIONS.md](SESSION_INSTRUCTIONS.md)** - Development guidelines

## Tech Stack

- **Language:** Go
- **LLM Runtime:** Ollama (local models)
- **Database:** SQLite
- **Architecture:** Multi-agent with async coordination

## Statistics

- **Codebase:** ~12,000 lines of Go
- **Agents:** 6 (Chat, Manager, Code, Test, Research, Review)
- **Tools:** 57+ (LSP: 6, Git: 7, Code Intelligence: 10, Filesystem: 9, Orchestration: 5, Web: 5, Context: 3, System: 2+)
- **Tests:** 111+ (unit + integration + E2E feedback loop)
- **Success Rate:** 93% (up from 75% pre-feedback loop)
- **Auto-Fix Rate:** 80% of compile errors resolved in <5s

## License

[Your License Here]

## Author

Roderick van Nievelt

---

*"The goal is not to replace the developer, but to amplify their capabilities by handling the repetitive, the tedious, and the time-consuming."*
