# Wilson - How To Guide

**Last Updated:** October 14, 2025
**Version:** Phase 4.5 Complete + Web Search Improvements (Phases 1-3)

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Quick Start](#quick-start)
3. [Project Structure](#project-structure)
4. [Running Wilson](#running-wilson)
5. [Using Agents](#using-agents)
6. [Available Tools](#available-tools)
7. [Configuration](#configuration)
8. [Testing](#testing)
9. [Database & Storage](#database--storage)
10. [Development Commands](#development-commands)
11. [Common Tasks](#common-tasks)
12. [Troubleshooting](#troubleshooting)

---

## Project Overview

**Wilson** is a multi-agent AI assistant with:
- Multiple specialized LLMs for different purposes (chat, analysis, code)
- 17 tools across 5 categories (context, agent, web, filesystem, system)
- Persistent memory via SQLite (contexts, artifacts, agent notes)
- Agent coordination for complex task delegation
- **NEW: Web search with HTML scraping (actual results!)**
- **NEW: Automatic research storage for agent collaboration**
- **NEW: Multi-site research orchestrator**
- Local execution via Ollama (no API keys needed)

### Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Wilson CLI                        │
├─────────────────────────────────────────────────────┤
│  Chat Agent (Orchestrator) ← llama3                 │
│    ├─ All tools available                           │
│    └─ Can delegate to specialists                   │
├─────────────────────────────────────────────────────┤
│  Analysis Agent (Specialist) ← mixtral/llama3       │
│    ├─ Web tools only                                │
│    └─ Research & analysis focused                   │
├─────────────────────────────────────────────────────┤
│  Code Agent (Stub) ← codellama (future)             │
│    ├─ Filesystem tools only                         │
│    └─ Not yet implemented                           │
├─────────────────────────────────────────────────────┤
│              Context Store (SQLite)                  │
│  ├─ Contexts (tasks/projects/sessions)              │
│  ├─ Artifacts (outputs from agents)                 │
│  └─ Agent Notes (inter-agent communication)         │
└─────────────────────────────────────────────────────┘
```

### Key Features

- **Multi-LLM Architecture:** Different models for different purposes
- **Persistent Context:** All work saved in SQLite database
- **Tool System:** Plugin-based, self-registering tools
- **Agent Coordination:** Task delegation and collaboration
- **Audit Logging:** Track all tool executions
- **Safety:** Confirmation for dangerous operations

---

## Quick Start

### Prerequisites

1. **Go** (1.21+)
   ```bash
   go version
   ```

2. **Ollama** (running locally)
   ```bash
   # Install Ollama
   curl -fsSL https://ollama.com/install.sh | sh

   # Start Ollama
   ollama serve

   # Pull required model
   ollama pull llama3

   # Optional: Pull analysis model
   ollama pull mixtral:8x7b
   ```

3. **SQLite** (for database inspection)
   ```bash
   sqlite3 --version
   ```

### Installation

```bash
# Clone/navigate to project
cd /Users/roderick.vannievelt/IdeaProjects/wilson/go

# Install dependencies
go mod tidy

# Build Wilson
go build -o wilson main.go
```

### Setup Global Command (Recommended)

Make Wilson available from anywhere:

```bash
# The launcher script is already created at wilson.sh
# Add alias to your shell
echo "alias wilson='$HOME/IdeaProjects/wilson/wilson.sh'" >> ~/.zshrc

# Reload shell
source ~/.zshrc

# Now you can run Wilson from anywhere!
wilson
```

**What the launcher does:**
1. Checks if Ollama is running → starts it if needed
2. Checks if Wilson is built → builds it if needed
3. Runs Wilson with all arguments passed through

---

## Project Structure

```
wilson/
├── go/                          # Main Go project
│   ├── main.go                  # Entry point
│   ├── agent/                   # Agent system
│   │   ├── types.go            # Agent interface
│   │   ├── registry.go         # Agent discovery
│   │   ├── coordinator.go      # Task delegation
│   │   ├── base_agent.go       # Common functionality
│   │   ├── chat_agent.go       # Orchestrator
│   │   ├── analysis_agent.go   # Research specialist
│   │   └── code_agent.go       # Code specialist (stub)
│   ├── context/                 # Persistent memory
│   │   ├── types.go            # Context/Artifact types
│   │   ├── store.go            # SQLite operations
│   │   └── manager.go          # High-level operations
│   ├── llm/                     # Multi-LLM manager
│   │   ├── types.go            # Client interface
│   │   ├── manager.go          # Purpose-based routing
│   │   └── ollama.go           # Ollama provider
│   ├── tools/                   # Tool system
│   │   ├── types.go            # Tool interface
│   │   ├── registry.go         # Tool discovery
│   │   ├── executor.go         # Tool execution
│   │   ├── agent/              # Agent tools
│   │   ├── context/            # Context tools
│   │   ├── filesystem/         # Filesystem tools
│   │   ├── system/             # System tools
│   │   └── web/                # Web tools
│   ├── config/                  # Configuration
│   │   ├── types.go            # Config structures
│   │   ├── config.go           # Config loader
│   │   └── tools.yaml          # Main config file
│   ├── ollama/                  # Ollama client
│   │   └── client.go           # LLM communication
│   ├── .wilson/                 # Data directory
│   │   ├── memory.db           # SQLite database
│   │   └── audit.log           # Tool execution log
│   ├── manual_test.sh           # Test script
│   └── test_results.txt         # Test output
├── python/                      # Python components (STT/TTS - future)
├── TODO.md                      # Development roadmap
├── MANUAL_TEST_RESULTS.md       # Test documentation
└── HOWTO.md                     # This file
```

---

## Running Wilson

### Basic Usage

**Option 1: Using the alias (recommended)**
```bash
# From anywhere in your terminal
wilson
```

**Option 2: Direct execution**
```bash
cd go
./wilson
```

**The launcher script automatically:**
- ✅ Checks if Ollama is running
- ✅ Starts Ollama if needed
- ✅ Builds Wilson if binary is missing
- ✅ Runs Wilson from any directory

This starts an interactive chat session where you can:
- Ask questions
- Request tool usage
- Create contexts
- Delegate tasks to agents

### Example Session

```
You: What agents are available?
Wilson: [Uses agent_status tool]
Available Agents:
• chat (orchestrator) - All tools
• analysis (research & analysis) - Web tools
• code (future) - Filesystem tools

You: Create a context called 'research' for researching Ollama
Wilson: [Uses create_context tool]
✓ Created context: research
  Key: research
  Type: research
  Status: active

You: Search the web for Ollama documentation
Wilson: [Uses search_web tool]
Found 10 results:
1. Ollama - Get up and running with Llama...
...

You: exit
Goodbye!
```

### Exit Commands

- `exit` - Quit Wilson
- `quit` - Quit Wilson
- `Ctrl+C` - Force quit

---

## Using Agents

### Agent Overview

Wilson has 3 agents with different capabilities:

#### 1. Chat Agent (Orchestrator)
- **Purpose:** Main user interface, task breakdown, delegation
- **LLM:** llama3 (chat)
- **Tools:** All 16 tools
- **Can Delegate:** Yes
- **Use For:** General conversation, complex multi-step tasks

#### 2. Analysis Agent (Specialist)
- **Purpose:** Research, web searches, content analysis
- **LLM:** mixtral:8x7b (falls back to llama3)
- **Tools:** Web tools + context tools
- **Can Delegate:** No
- **Use For:** In-depth research, summarization, data extraction

#### 3. Code Agent (Stub - Future)
- **Purpose:** Code generation and analysis
- **LLM:** codellama (when configured)
- **Tools:** Filesystem tools + context tools
- **Can Delegate:** No
- **Use For:** Not yet implemented

### Checking Agent Status

```
You: What agents are available?
Wilson: [Shows agent_status]
```

Or use the tool directly:
```
You: Show me agent status
```

### Delegating Tasks

```
You: Delegate a research task to the analysis agent to search for Go best practices
Wilson: [Uses delegate_task tool]
✓ Task delegated to analysis agent
  Task ID: task-xxx
  Type: research
```

Check task status:
```
You: What's the status of task task-xxx?
Wilson: [Uses agent_status with task_id]
```

### How Agents Work

1. **User asks Wilson something**
2. **Chat Agent** (orchestrator) decides:
   - Can I answer directly?
   - Do I need to use tools?
   - Should I delegate to a specialist?
3. **Specialist Agent** (if delegated):
   - Executes using its allowed tools
   - Stores results as artifacts
   - Returns to Chat Agent
4. **Chat Agent** synthesizes and responds to user

### Adding New Agents

To add a new agent, you need to:

1. **Create agent file** in `go/agent/`:
   ```go
   // go/agent/new_agent.go
   type NewAgent struct {
       *BaseAgent
   }

   func NewNewAgent(llmMgr *llm.Manager, contextMgr *context.Manager) *NewAgent {
       base := &BaseAgent{
           name:         "new",
           purpose:      llm.PurposeXXX,
           allowedTools: []string{"tool1", "tool2"},
           llmManager:   llmMgr,
           contextMgr:   contextMgr,
       }

       return &NewAgent{BaseAgent: base}
   }

   func (a *NewAgent) CanHandle(task *Task) bool {
       return task.Type == TaskTypeXXX
   }

   func (a *NewAgent) Execute(ctx context.Context, task *Task) (*Result, error) {
       // Implementation
   }
   ```

2. **Register in main.go**:
   ```go
   newAgent := agent.NewNewAgent(llmMgr, contextMgr)
   registry.Register(newAgent)
   ```

3. **Update config** (if new LLM needed):
   ```yaml
   llms:
     new_purpose:
       provider: ollama
       model: some-model
       temperature: 0.5
   ```

---

## Available Tools

Wilson has 17 tools across 5 categories:

### Context Tools (6)

| Tool | Description | Risk | Confirm |
|------|-------------|------|---------|
| `create_context` | Create new context (task/research/etc) | Safe | No |
| `store_artifact` | Store findings/outputs | Safe | No |
| `retrieve_context` | Get context with all artifacts | Safe | No |
| `search_artifacts` | Full-text search across artifacts | Safe | No |
| `list_contexts` | List all contexts | Safe | No |
| `leave_note` | Inter-agent communication | Safe | No |

### Agent Tools (2)

| Tool | Description | Risk | Confirm |
|------|-------------|------|---------|
| `agent_status` | Check agents and task status | Safe | No |
| `delegate_task` | Assign work to specialist agent | Safe | No |

### Web Tools (5) ⭐ NEW IMPROVEMENTS

| Tool | Description | Risk | Confirm |
|------|-------------|------|---------|
| `search_web` | ✨ DuckDuckGo search **(NOW WORKS!)** | Safe | No |
| `fetch_page` | Download web page **(auto-stores)** | Moderate | **Yes** |
| `extract_content` | Parse HTML to text **(auto-stores)** | Safe | No |
| `analyze_content` | LLM-powered analysis **(auto-stores)** | Safe | No |
| `research_topic` | ✨ **NEW:** Multi-site research orchestrator | Moderate | **Yes** |

**Recent Improvements (Phases 1-3):**
- ✅ **Phase 1:** `search_web` now uses HTML scraping → returns 10 real results instead of "No results found"
- ✅ **Phase 2:** All web tools now auto-store results in context for agent collaboration
- ✅ **Phase 3:** New `research_topic` tool → search + fetch + analyze + synthesize in one command!

### Filesystem Tools (3)

| Tool | Description | Risk | Confirm |
|------|-------------|------|---------|
| `list_files` | List directory contents | Safe | No |
| `read_file` | Read file contents | Safe | No |
| `search_files` | Find files by pattern | Safe | No |

### System Tools (1)

| Tool | Description | Risk | Confirm |
|------|-------------|------|---------|
| `run_command` | Execute shell commands | **Dangerous** | **Yes** |

---

## Configuration

### Main Config File

**Location:** `go/config/tools.yaml`

### Key Sections

#### 1. Workspace
```yaml
workspace:
  path: "/Users/roderick.vannievelt/IdeaProjects/wilson"
```

#### 2. LLM Configuration
```yaml
llms:
  chat:
    provider: "ollama"
    model: "llama3:latest"
    temperature: 0.7
    base_url: "http://localhost:11434"

  analysis:
    provider: "ollama"
    model: "mixtral:8x7b"
    temperature: 0.3
    base_url: "http://localhost:11434"
    fallback: "llama3:latest"  # Falls back if mixtral unavailable
```

#### 3. Tool Configuration
```yaml
tools:
  tool_name:
    enabled: true
    requires_confirm: false  # Set to true for dangerous tools
    # Tool-specific settings
```

#### 4. Context Store
```yaml
context:
  enabled: true
  db_path: ".wilson/memory.db"
  auto_store: true  # Automatically store tool results
  default_context: "session"
```

#### 5. Audit Logging
```yaml
audit:
  enabled: true
  log_path: ".wilson/audit.log"
  log_level: "info"
```

### Disabling/Enabling Tools

Edit `config/tools.yaml`:
```yaml
tools:
  fetch_page:
    enabled: false  # Disable tool
```

### Changing Models

```yaml
llms:
  chat:
    model: "llama3.1:latest"  # Use different model
```

Then pull the model:
```bash
ollama pull llama3.1
```

---

## Testing

### Unit Tests (43 tests)

Run all unit tests:
```bash
cd go
go test ./context ./agent ./llm -v
```

Run specific package:
```bash
go test ./context -v   # Context store (14 tests)
go test ./agent -v     # Agent registry (13 tests)
go test ./llm -v       # LLM manager (16 tests)
```

### Manual Testing Script

Run verification script:
```bash
cd go
./manual_test.sh
```

This checks:
- ✅ Build succeeds
- ✅ Database created with correct schema
- ✅ All tools registered
- ✅ Audit log working
- ✅ Default context exists

Output saved to: `go/test_results.txt`

### What's Covered

**Automated (Unit Tests):**
- ✅ Context CRUD operations
- ✅ Artifact storage and retrieval
- ✅ Full-text search
- ✅ Agent registration and discovery
- ✅ Task routing (FindCapable)
- ✅ LLM client management
- ✅ Fallback logic
- ✅ Concurrent access safety

**Automated (Verification Script):**
- ✅ Build verification
- ✅ Database schema
- ✅ Tool registration
- ✅ Agent initialization

### What's NOT Covered (Intentionally)

- ❌ Interactive tool usage (requires manual testing)
- ❌ LLM responses (too variable)
- ❌ Web fetching (external dependencies)
- ❌ Full end-to-end workflows (too complex for current stage)

### Manual Testing Checklist

Test these interactively by running Wilson:

**Context Store:**
1. Create context: `"create a context for testing"`
2. Store artifact: `"store this as an artifact: test data"`
3. Search: `"search for artifacts about testing"`
4. List: `"list all contexts"`
5. Retrieve: `"retrieve the testing context"`

**Agent System:**
1. Status: `"what agents are available?"`
2. Delegate: `"delegate a research task to analysis agent"`
3. Check: `"check task status"`

**Web Tools:**
1. Search: `"search the web for Ollama docs"`
2. Fetch: `"fetch the page at github.com/ollama/ollama"` (requires confirmation)
3. Extract: `"extract content from that page"`
4. Analyze: `"analyze and summarize that content"`
5. **Research (NEW):** `"research the topic 'How do LLMs work'"` (multi-site orchestrator)

---

## Database & Storage

### Database Location

**Path:** `go/.wilson/memory.db`

### Schema

**Tables:**
- `contexts` - Task/project containers
- `artifacts` - Agent outputs and findings
- `agent_notes` - Inter-agent messages

### Inspecting Database

```bash
cd go
sqlite3 .wilson/memory.db
```

**Useful queries:**
```sql
-- List all contexts
SELECT id, context_key, context_type, status, title
FROM contexts;

-- Count artifacts
SELECT COUNT(*) FROM artifacts;

-- Recent artifacts
SELECT artifact_type, agent, created_at
FROM artifacts
ORDER BY created_at DESC
LIMIT 10;

-- Search artifacts
SELECT content
FROM artifacts
WHERE content LIKE '%search term%';

-- Agent notes
SELECT from_agent, to_agent, note, created_at
FROM agent_notes
ORDER BY created_at DESC;
```

### Audit Log

**Location:** `/IdeaProjects/wilson/.wilson/audit.log`

**Format:** JSON (one per line)

**View recent entries:**
```bash
tail -10 /Users/roderick.vannievelt/IdeaProjects/wilson/.wilson/audit.log | jq
```

**Search for specific tool:**
```bash
grep "search_web" /Users/roderick.vannievelt/IdeaProjects/wilson/.wilson/audit.log | jq
```

### Backing Up Data

```bash
# Backup database
cp go/.wilson/memory.db go/.wilson/memory.db.backup

# Backup audit log
cp .wilson/audit.log .wilson/audit.log.backup
```

### Cleaning Up Old Data

```bash
# Archive old contexts
sqlite3 go/.wilson/memory.db "UPDATE contexts SET status='archived' WHERE created_at < date('now', '-30 days');"

# Delete archived contexts (and cascading artifacts/notes)
sqlite3 go/.wilson/memory.db "DELETE FROM contexts WHERE status='archived';"
```

---

## Development Commands

### Building

```bash
cd go

# Standard build
go build -o wilson main.go

# Build with race detection
go build -race -o wilson main.go

# Build for different OS
GOOS=linux GOARCH=amd64 go build -o wilson-linux main.go
```

### Running

```bash
# Run directly
go run main.go

# Run with built binary
./wilson

# Run with verbose logging (if implemented)
./wilson -v
```

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run with coverage
go test ./... -cover

# Run specific test
go test ./context -run TestCreateContext -v

# Run with race detection
go test ./... -race
```

### Dependencies

```bash
# Add new dependency
go get github.com/package/name

# Update dependencies
go get -u ./...

# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint (requires golangci-lint)
golangci-lint run

# Vet code
go vet ./...
```

---

## Common Tasks

### Add a New Tool

1. **Create tool file** in appropriate category:
   ```go
   // go/tools/category/new_tool.go
   package category

   import (
       "context"
       "wilson/tools"
   )

   type NewTool struct{}

   func (t *NewTool) Metadata() tools.ToolMetadata {
       return tools.ToolMetadata{
           Name:            "new_tool",
           Description:     "Does something useful",
           Category:        "category",
           RiskLevel:       tools.RiskSafe,
           RequiresConfirm: false,
           Enabled:         true,
           Parameters: []tools.Parameter{
               {
                   Name:        "param1",
                   Type:        "string",
                   Required:    true,
                   Description: "Parameter description",
               },
           },
           Examples: []string{
               `{"tool": "new_tool", "arguments": {"param1": "value"}}`,
           },
       }
   }

   func (t *NewTool) Validate(args map[string]interface{}) error {
       // Validation logic
       return nil
   }

   func (t *NewTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
       // Implementation
       return "result", nil
   }

   func init() {
       tools.Register(&NewTool{})
   }
   ```

2. **Import in main.go**:
   ```go
   _ "wilson/tools/category"  // Import to register tools
   ```

3. **Add config** (optional):
   ```yaml
   tools:
     new_tool:
       enabled: true
       requires_confirm: false
   ```

### Add a New LLM Purpose

1. **Define purpose** in `go/llm/types.go`:
   ```go
   const (
       PurposeChat     Purpose = "chat"
       PurposeAnalysis Purpose = "analysis"
       PurposeCode     Purpose = "code"
       PurposeVision   Purpose = "vision"
       PurposeNewThing Purpose = "newthing"  // Add this
   )
   ```

2. **Add to config**:
   ```yaml
   llms:
     newthing:
       provider: ollama
       model: some-model
       temperature: 0.5
   ```

3. **Register in main.go**:
   ```go
   case "newthing":
       purpose = llm.PurposeNewThing
   ```

### Change Default Context

Edit `config/tools.yaml`:
```yaml
context:
  default_context: "my-custom-session"
```

### Add Allowed Domain for fetch_page

Edit `config/tools.yaml`:
```yaml
tools:
  fetch_page:
    allowed_domains:
      - "example.com"
      - "*.example.com"
```

---

## Troubleshooting

### Wilson won't start

**Check Ollama:**
```bash
curl http://localhost:11434/api/version
```

If not running:
```bash
ollama serve
```

**Check model is pulled:**
```bash
ollama list
```

If llama3 not present:
```bash
ollama pull llama3
```

### Database errors

**Database not found:**
```bash
# Check if it exists
ls -la go/.wilson/memory.db

# If missing, Wilson will create it on next run
```

**Database locked:**
```bash
# Check for other Wilson processes
ps aux | grep wilson

# Kill if found
killall wilson
```

**Corrupted database:**
```bash
# Restore from backup
cp go/.wilson/memory.db.backup go/.wilson/memory.db

# Or delete and start fresh (loses all data!)
rm go/.wilson/memory.db
```

### Tools not working

**Check tool is enabled:**
```bash
grep -A 3 "tool_name:" config/tools.yaml
```

**Check tool registered:**
```bash
# Run Wilson and look for tool in list
./wilson
# Look for "Loaded X tools"
```

### Agent errors

**Analysis agent not available:**
- Check mixtral is pulled: `ollama list`
- Or it will fallback to llama3 (check logs)

**Task delegation fails:**
- Check agent exists: "what agents are available?"
- Check agent can handle task type
- Check LLM is available for that agent

### Performance issues

**Slow responses:**
- Check Ollama is using GPU acceleration
- Try smaller models (llama3:8b instead of mixtral:8x7b)
- Reduce temperature in config

**High memory usage:**
- Check for multiple Ollama processes
- Restart Ollama: `killall ollama && ollama serve`

### Web tools failing

**Search not working:**
- DuckDuckGo API may be rate-limited
- Try again after a few minutes

**Fetch page blocked:**
- Check domain in allowed_domains list
- Domain must be explicitly whitelisted

---

## Next Steps

### Recommended Learning Path

1. **Start Wilson** and explore basic commands
2. **Create a context** for a small project
3. **Store some artifacts** as you work
4. **Search artifacts** to see full-text search
5. **Try agent delegation** for research tasks
6. **Check database** to see persistence

### Future Enhancements (Phase 5+)

See `TODO.md` for full roadmap:
- **Phase 4c:** Advanced context features (embeddings, summarization)
- **Phase 5:** Model Context Protocol (MCP) integration
- **Phase 6:** Python integration (STT/TTS)

### Contributing

When adding features:
1. Follow existing patterns (see similar tools/agents)
2. Add unit tests for stable core components
3. Update configuration if needed
4. Test with `./manual_test.sh`
5. Update this HOWTO.md

---

## Quick Reference

### Essential Commands

```bash
# Build and run
cd go && go build -o wilson main.go && ./wilson

# Run tests
go test ./context ./agent ./llm -v

# Verify system
./manual_test.sh

# Inspect database
sqlite3 .wilson/memory.db "SELECT * FROM contexts;"

# View audit log
tail -f ../.wilson/audit.log | jq

# Clean build
go clean && go build -o wilson main.go
```

### Key Files

- `go/main.go` - Entry point
- `go/config/tools.yaml` - Configuration
- `go/.wilson/memory.db` - Database
- `.wilson/audit.log` - Tool execution log
- `TODO.md` - Development roadmap
- `MANUAL_TEST_RESULTS.md` - Test documentation

### Getting Help

- Check `TODO.md` for project status and roadmap
- Check `MANUAL_TEST_RESULTS.md` for testing info
- Check logs: `.wilson/audit.log`
- Check database: `sqlite3 .wilson/memory.db`

---

**Last Updated:** October 14, 2025
**Current Phase:** 4.5 Complete + Web Search Fixes (Phases 1-3) - Ready for Phase 5 (MCP Integration)

---

## 🆕 Recent Updates (Web Search Improvements)

### Phase 1: Working Web Search ✅
- Fixed DuckDuckGo search to return actual results
- Now uses HTML scraping instead of limited Instant Answer API
- Returns 10 real URLs with titles and snippets

### Phase 2: Automatic Research Storage ✅
- All web tool results now automatically stored in context
- Enables agent collaboration and knowledge accumulation
- Results persist in `.wilson/memory.db` for future reference

### Phase 3: Research Orchestrator ✅
- New `research_topic` tool for end-to-end multi-site research
- Searches → fetches (concurrent) → extracts → analyzes → synthesizes
- Single command gets comprehensive answer from multiple sources
- Example: `"research the topic 'How do large language models work' with 5 sites"`
