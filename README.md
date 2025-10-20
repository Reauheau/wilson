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
USER
  ↓
Wilson (Chat Agent - llama3, 4GB, always on)
  ↓ async delegation (returns immediately)
Coordinator
  ↓ spawns on-demand
Worker Manager (max 2 concurrent)
  ↓
┌─────────────┬─────────────┬─────────────┬─────────────┐
│ Code Worker │ Test Worker │ Research    │ Review      │
│ (ephemeral) │ (ephemeral) │ (ephemeral) │ (ephemeral) │
│             │             │             │             │
│ qwen2.5-    │ llama3      │ mixtral     │ claude-3    │
│ coder:14b   │ (~4GB)      │ (~6GB)      │ (~6GB)      │
│ (~8GB)      │             │             │             │
└─────────────┴─────────────┴─────────────┴─────────────┘
  ↓
SQLite (tasks, artifacts, reviews)
```

**Lifecycle:** Spawn → Load Model → Execute → Kill Immediately

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

# Pull required models
ollama pull qwen2.5:7b
ollama pull qwen2.5-coder:14b

# Build and run
go build -o wilson .
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

## Configuration

Models can be easily swapped in `config.yaml` to match your machine:

**Low-end (8GB):** llama3 for everything, 1 worker max
**Mid-range (16GB):** llama3 + qwen2.5-coder:14b, 2 workers (default)
**High-end (32GB+):** llama3 + qwen2.5-coder:32b + mixtral, 2-4 workers

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
