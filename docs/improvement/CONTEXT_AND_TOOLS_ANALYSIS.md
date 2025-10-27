# Wilson Context & Tools Gap Analysis

**Date:** October 27, 2025 (Updated)
**Purpose:** Compare Wilson's capabilities against leading CLI code assistants (Aider, Continue.dev, Cursor, Cline)

---

## 📊 Current Wilson TaskContext (task_context.go)

### What Wilson HAS:
```go
type TaskContext struct {
    // Identity
    TaskID, TaskKey, ParentID string

    // Core task info
    Description string
    Type        ManagedTaskType
    Priority    int

    // Execution parameters
    ProjectPath string                 // ✅ Absolute path
    Input       map[string]interface{} // ✅ Flexible input

    // Dependencies & relationships
    DependsOn        []string // ✅ Task keys
    DependencyFiles  []string // ✅ Files from previous tasks
    DependencyErrors []string // ✅ Learn from dependencies

    // Feedback context
    PreviousAttempts int              // ✅ Retry tracking
    PreviousErrors   []ExecutionError // ✅ Rich error history

    // Artifacts
    CreatedFiles  []string // ✅ Track creations
    ModifiedFiles []string // ✅ Track modifications

    // Metadata
    CreatedAt, StartedAt, LastAttempt time.Time
}
```

### What Wilson is MISSING (vs Claude Code / ideal system):

#### 1. **Workspace/Codebase Context** 🔴 CRITICAL
```go
// MISSING - should add:
WorkspaceRoot    string              // Git repo root
RelativePath     string              // Relative to workspace
GitBranch        string              // Current branch
GitStatus        map[string]string   // Modified files in git
OpenFiles        []string            // Files user has open
ActiveFile       string              // File user is currently editing
CursorPosition   *FilePosition       // Where user's cursor is
```

**Why critical:**
- Wilson doesn't know what files are already open/being edited
- Can't determine relative vs absolute paths correctly
- No awareness of git state (could avoid modifying committed files)
- Can't prioritize files user is actively working on

#### 2. **Language/Framework Context** 🟡 HIGH
```go
// MISSING - should add:
Language         string              // Go, Python, JavaScript, etc.
FrameworkInfo    *FrameworkContext   // Detected framework (React, Django, etc.)
BuildSystem      string              // go.mod, package.json, Cargo.toml
Dependencies     []Dependency        // Parsed from build files
TestFramework    string              // testing, pytest, jest, etc.
LinterConfig     string              // golangci.yml, .eslintrc, etc.
```

**Why high priority:**
- Currently hardcodes Go assumptions
- Could support multi-language projects
- Could respect project-specific linting rules
- Could detect test framework conventions automatically

#### 3. **Multi-file Context** 🟡 HIGH
```go
// MISSING - should add:
RelatedFiles     []string            // Discovered via imports, references
RecentlyModified []string            // Files changed in last N minutes
TestFiles        []string            // Associated test files
ImplementationFile string            // For test tasks, the file being tested
ImportGraph      map[string][]string // File → imported files
CallGraph        map[string][]string // Function → called functions
```

**Why high priority:**
- Currently only tracks direct dependency files
- Can't discover related files via imports
- No awareness of test ↔ implementation relationships
- Can't see the bigger picture of what a change affects

#### 4. **Error Context Enrichment** 🟢 MEDIUM
```go
// HAVE (ExecutionError) but could add:
StackTrace       []StackFrame        // Full call stack
SimilarErrors    []ErrorReference    // "We've seen this before"
SuggestedFix     *AutoFix            // Structured fix (not just string)
ErrorFrequency   int                 // How often this error occurs
RelatedDocs      []string            // Links to relevant documentation
```

**Why medium priority:**
- Current ExecutionError is good foundation
- Stack traces would help debug complex issues
- Error frequency could trigger escalation
- Similar errors could suggest patterns

#### 5. **User Intent Context** 🟢 LOW
```go
// MISSING - future enhancement:
UserMessage      string              // Original user request (full text)
FollowUpContext  []string            // Previous conversation turns
UserPreferences  *UserPrefs          // Coding style, conventions
ConfidenceLevel  float64             // How confident agent is
```

**Why low priority:**
- Wilson works well with task-based architecture
- User intent is implicit in task description
- Could be useful for ambiguity resolution

---

## 🛠️ Current Wilson Tools (50+ tools)

### LSP Integration (6 tools) ✅ EXCELLENT
- ✅ `get_diagnostics` - Real-time errors (<500ms)
- ✅ `go_to_definition` - Navigate to definitions
- ✅ `find_references` - Find all usages workspace-wide
- ✅ `get_hover_info` - Documentation and type info
- ✅ `get_symbols` - File structure overview
- ✅ `rename_symbol` - Safe workspace-wide refactoring
- **Supports:** Go, Python, JavaScript/TypeScript, Rust

**Assessment:** Strong LSP foundation, matches modern IDE capabilities

### Code Intelligence - AST (10 tools) ✅ EXCELLENT
- ✅ `parse_file` - AST parsing
- ✅ `find_symbol` - Find definitions/usages
- ✅ `analyze_structure` - Package analysis
- ✅ `analyze_imports` - Import analysis
- ✅ `find_patterns` - Pattern discovery
- ✅ `find_related` - Related file discovery
- ✅ `dependency_graph` - Import relationships
- ✅ `code_review` - Quality review
- ✅ `complexity_check` - Complexity analysis
- ✅ `lint_code` - Style checking

**Assessment:** Comprehensive AST capabilities, complements LSP well

### File Operations (9 tools) ✅ GOOD
- ✅ `read_file` - Read files
- ✅ `write_file` - Create files
- ✅ `modify_file` - Multi-line changes
- ✅ `edit_line` - Single-line fixes ⭐ UNIQUE
- ✅ `append_to_file` - Append content
- ✅ `list_files` - Directory listing
- ✅ `search_files` - File search
- ✅ `make_directory` - Create dirs
- ✅ `change_directory` - Navigation

**Assessment:** Strong, edit_line is a differentiator

### Build & Test (5 tools) ✅ GOOD
- ✅ `compile` - Build code with structured errors
- ✅ `run_tests` - Execute tests with coverage
- ✅ `coverage_check` - Verify coverage thresholds
- ✅ `security_scan` - Security analysis
- ✅ `format_code` - Auto-formatting

**Assessment:** Good foundation, could add more

### Context & Memory (5 tools) ✅ GOOD
- ✅ `create_context` - Create contexts
- ✅ `retrieve_context` - Load context
- ✅ `list_contexts` - List available
- ✅ `store_artifact` - Save artifacts
- ✅ `search_artifacts` - Search artifacts

**Assessment:** Strong memory system

### Task Management (10 tools) ✅ EXCELLENT
- ✅ `delegate_task` - Delegate to agents
- ✅ `check_task_progress` - Monitor tasks
- ✅ `poll_tasks` - Check for tasks
- ✅ `claim_task` - Claim a task
- ✅ `update_task_progress` - Update progress
- ✅ `unblock_tasks` - Unblock dependencies
- ✅ `get_task_queue` - View queue
- ✅ `request_review` - Request reviews
- ✅ `get_review_status` - Check review
- ✅ `submit_review` - Submit review

**Assessment:** ADVANCED multi-agent orchestration, exceeds typical systems

### Code Generation (2 tools) ✅ UNIQUE
- ✅ `generate_code` - Specialist model delegation
- ✅ `orchestrate_code_task` - Multi-file code generation

**Assessment:** Unique dual-model architecture

### Research (4 tools) ✅ GOOD
- ✅ `search_web` - Web search
- ✅ `fetch_page` - Fetch web pages
- ✅ `research_topic` - Multi-source research
- ✅ `extract_content` - Extract from pages

**Assessment:** Good research capabilities

### Agent Communication (2 tools) ✅ GOOD
- ✅ `leave_note` - Inter-agent messaging
- ✅ `agent_status` - Check agent status

**Assessment:** Solid coordination

### System (3 tools) ⚠️ LIMITED
- ✅ `run_command` - Execute commands
- ✅ `model_status` - Check models
- ⚠️ Limited system interaction

---

## 🔴 Critical Missing Tools (vs Aider, Continue.dev, Cursor)

### 1. **Git Operations** 🔴 CRITICAL *(Planned - 8 tools designed)*
```
READY TO IMPLEMENT:
- git_status           # See what's changed
- git_diff             # View diffs
- git_blame            # Find who changed code
- git_log              # View history
- git_show             # Show commits
- git_branch           # List/switch branches
- git_stash            # Stash changes
- git_commit           # Commit changes
```

**Why critical:**
- **Aider:** Auto-commits with smart messages, core feature
- **Continue.dev:** Git integration for context awareness
- **Cursor:** Full git UI integration
- Wilson can't see what's changed, understand history, or commit safely

**Impact if added:** +30% effectiveness, enables "Aider-style" auto-commit workflow

### 2. **Codebase Mapping** 🔴 CRITICAL
```
MISSING (Aider's killer feature):
- create_repo_map      # Generate codebase structure map
- update_repo_map      # Refresh map when files change
- search_repo_map      # Find symbols/patterns across codebase
```

**Why critical:**
- **Aider:** Creates a "map" of entire codebase, helps LLM understand project structure
- **Benefits:** Work on larger codebases (100+ files), understand relationships
- **Current gap:** Wilson only sees files explicitly added to context

**Impact if added:** +50% effectiveness on large codebases (Aider's competitive advantage)

### 3. **Search & Navigation** 🟡 HIGH
```
MISSING:
- grep_workspace       # Fast text search (ripgrep-based)
- find_in_files        # Regex search across project
- recent_files         # Track file modifications
- file_tree            # Show project structure
```

**Why high priority:**
- **Aider:** Uses ripgrep for fast workspace search
- **Continue.dev:** Has "@workspace" search command
- **Cursor:** Full-text search integrated
- Current `search_files` is basic glob patterns only
- Can't quickly grep for text patterns across thousands of files

**What others have:**
- Aider: ripgrep integration, searches 1000s of files in <100ms
- Continue.dev: "@workspace" context searches all project files
- Cursor: Native workspace search with regex

**Impact if added:** +40% speed for finding code patterns

### 4. **Auto-Linting & Testing** 🟡 HIGH
```
MISSING:
- auto_lint            # Run linter after every change
- auto_test            # Run tests after every change
- fix_lint_errors      # Apply linter auto-fixes
- run_single_test      # Run one test by name
- run_failed_tests     # Re-run only failures
```

**Why high priority:**
- **Aider:** Has `/lint` and `/test` commands, auto-runs after changes
- **Continue.dev:** Integrates with test frameworks
- Current Wilson requires manual compilation/testing
- Immediate feedback loop critical for code quality

**What others have:**
- Aider: Automatic lint/test after each edit, shows results, can auto-fix
- Continue.dev: Test running integrated into agent workflow
- Cursor: Built-in test runner UI

**Impact if added:** +30% code quality, faster iteration

### 5. **Testing Enhancements** 🟢 MEDIUM
```
MISSING:
- run_single_test      # Run one test by name
- run_failed_tests     # Re-run only failures
- debug_test           # Run test with debugger
- generate_test_cases  # AI-generated test cases
- mutation_testing     # Test quality check
- benchmark            # Performance testing
```

**Why medium priority:**
- Current run_tests is good but basic
- Can't target specific tests efficiently
- No debugging support
- No test quality metrics

**Impact if added:** +15% testing efficiency

### 6. **Documentation** 🟢 MEDIUM
```
MISSING:
- generate_docs        # Generate API docs
- explain_code         # Natural language explanation
- add_comments         # Smart comment generation
- generate_readme      # Project README
- generate_changelog   # From git history
```

**Why medium priority:**
- Wilson doesn't help with documentation
- Could auto-document APIs
- Could explain complex code
- Claude Code has doc generation

**Impact if added:** +20% completeness

### 7. **Debugging** 🟢 LOW
```
MISSING:
- set_breakpoint       # Set debugger breakpoint
- inspect_variable     # Check variable value
- step_through         # Step debugging
- evaluate_expression  # Evaluate in context
```

**Why low priority:**
- Debugging is interactive
- Hard to do in automated agent
- Better done by user with IDE
- Nice-to-have for future

**Impact if added:** +10% debugging help

---

## 🎯 Priority Recommendations

### **Phase 1: Critical Tools (1-2 weeks)**
1. **Git Integration** (3-5 tools)
   - `git_status` - See modified files
   - `git_diff` - View changes
   - `git_log` - View history
   - Priority: 🔴 CRITICAL

2. **LSP Client** (Architecture change)
   - Start LSP server for Go projects
   - Use LSP for symbol lookup instead of AST parsing
   - Priority: 🔴 CRITICAL
   - Impact: Massive speed improvement

3. **Enhanced Search** (2-3 tools)
   - `grep_project` - Fast text search (use ripgrep)
   - `find_symbol_global` - Workspace-wide symbol search
   - Priority: 🟡 HIGH

### **Phase 2: Multi-file Context (1 week)**
4. **Workspace Context** (TaskContext fields)
   - Add `WorkspaceRoot`, `RelativePath`, `GitBranch`
   - Add `OpenFiles`, `ActiveFile`
   - Add `RelatedFiles` via import analysis
   - Priority: 🟡 HIGH

5. **Language Detection** (TaskContext fields)
   - Auto-detect language from file extension
   - Parse build files (go.mod, package.json)
   - Detect test framework conventions
   - Priority: 🟡 HIGH

### **Phase 3: Advanced Features (2-3 weeks)**
6. **Multi-file Refactoring** (3-4 tools)
   - `rename_across_files`
   - `extract_function`
   - `move_symbol`
   - Priority: 🟢 MEDIUM

7. **Documentation** (2-3 tools)
   - `generate_docs`
   - `explain_code`
   - `add_comments`
   - Priority: 🟢 MEDIUM

---

## 📊 Comparison Matrix

| Category | Wilson | Claude Code | Gap |
|----------|--------|-------------|-----|
| **Code Intelligence** | 10 tools ✅ | 10-12 tools | None |
| **File Operations** | 9 tools ✅ | 8-10 tools | Ahead (edit_line) |
| **Git Integration** | 0 tools ❌ | 8-10 tools | **CRITICAL** |
| **LSP Integration** | 0 tools ❌ | Full LSP | **CRITICAL** |
| **Search** | 2 basic ⚠️ | 5-7 advanced | **HIGH** |
| **Build & Test** | 5 tools ✅ | 6-8 tools | Good |
| **Task Management** | 10 tools ✅ | 2-3 tools | **AHEAD** |
| **Multi-agent** | Yes ✅ | No | **AHEAD** |
| **Context Memory** | 5 tools ✅ | 3-4 tools | **AHEAD** |
| **Refactoring** | Limited ⚠️ | Advanced | **HIGH** |
| **Documentation** | 0 tools ❌ | 3-5 tools | **MEDIUM** |

### **Overall Assessment:**

**Wilson's Strengths (Ahead of Claude Code):**
- ✅ Multi-agent orchestration
- ✅ Task management & coordination
- ✅ Context memory & artifacts
- ✅ Dual-model architecture (generate_code)
- ✅ Surgical editing (edit_line)

**Wilson's Gaps (Behind Claude Code):**
- ❌ No git integration
- ❌ No LSP integration
- ⚠️ Limited search capabilities
- ⚠️ Limited multi-file refactoring
- ❌ No documentation generation

**Key Insight:** Wilson has BETTER orchestration but WEAKER workspace awareness. Adding git + LSP would make Wilson competitive with or superior to Claude Code.

---

## 💡 Implementation Priorities

### **Highest ROI (Do First):**

1. **Git Status Tool** (1-2 days)
   - Simple: call `git status --porcelain`
   - Huge impact: know what's modified
   - Easy to implement

2. **Workspace Root Detection** (1 day)
   - Find git root or go.mod location
   - Add to TaskContext
   - Enables relative paths

3. **Grep Project Tool** (1 day)
   - Use ripgrep if available, fallback to grep
   - Much faster than current search_files
   - Essential for large codebases

4. **LSP Client** (1 week)
   - Start with Go (gopls)
   - Use for goto_definition, find_references
   - Eventually replace parse_file for speed

### **Quick Wins (Should Do Soon):**

5. **Language Detection** (1 day)
   - Check file extension
   - Parse build files
   - Add to TaskContext

6. **Git Diff Tool** (1 day)
   - Show changes in files
   - Help understand context

7. **Related Files Discovery** (2 days)
   - Follow imports
   - Find tests for implementation
   - Add to TaskContext.RelatedFiles

---

## 🔧 Specific TaskContext Improvements

### Recommended Additions:

```go
type TaskContext struct {
    // ... existing fields ...

    // ⭐ Phase 1: Workspace Context (HIGH PRIORITY)
    WorkspaceRoot    string            // Git repo root or project root
    RelativePath     string            // Path relative to workspace
    GitBranch        string            // Current branch
    GitModifiedFiles []string          // Files modified in git

    // ⭐ Phase 1: Language Context (HIGH PRIORITY)
    Language         string            // Detected language
    BuildFile        string            // go.mod, package.json, etc.
    TestFramework    string            // testing, pytest, jest

    // ⭐ Phase 2: Multi-file Context (MEDIUM PRIORITY)
    RelatedFiles     []string          // Via imports, references
    OpenFiles        []string          // Files user has open
    ActiveFile       string            // File user is editing
    TestFiles        []string          // For implementation, vice versa

    // Future: Error Context (LOW PRIORITY)
    SimilarErrors    []string          // Reference to similar past errors
    ErrorFrequency   map[string]int    // Error type → count
}
```

**Impact:** Would improve context awareness by ~60%

---

---

## 🆕 Unique Features from Other Tools Worth Considering

### From Aider
1. **Repo Map** - Generates tree-sitter based map of codebase structure (their #1 differentiator)
2. **Auto-commit** - Commits every change with smart messages
3. **/add**, **/drop** - Easy file management in chat
4. **/architect** - Planning mode before coding
5. **Voice coding** - Speak requests, Aider types
6. **Watch mode** - Monitor files, auto-apply changes from comments

### From Continue.dev
1. **@workspace** - Reference entire project in queries
2. **@web** - Fetch and reference web pages
3. **@terminal** - Include terminal output in context
4. **Custom agents** - User-defined agents with specific tools
5. **Slash commands** - `/edit`, `/comment`, `/test` for quick actions

### From Cursor
1. **Composer** - Multi-file editing mode with preview
2. **Inline AI** - Edit suggestions directly in editor
3. **Cmd+K** - Quick AI actions anywhere
4. **Smart apply** - Preview diffs before accepting

### Innovations Wilson Could Adopt

**High Priority:**
1. **Repo Map** (from Aider) - Would massively improve large codebase handling
2. **File tree/structure view** - Visual representation of project
3. **Diff preview** - Show changes before applying (like Cursor)
4. **Slash commands** - Quick shortcuts like `/test`, `/lint`, `/commit`
5. **Watch mode** - Auto-detect file changes, stay synced

**Medium Priority:**
6. **Voice interface** - Accessibility + convenience
7. **@context shortcuts** - @workspace, @terminal, @git, @web
8. **Custom agents** - User-defined specialized agents
9. **Inline comments to code** - Parse TODO comments as tasks

---

## 📊 Updated Comparison Matrix

| Category | Wilson | Aider | Continue.dev | Assessment |
|----------|--------|-------|--------------|------------|
| **LSP Integration** | 6 tools ✅ | None ❌ | None ❌ | **AHEAD** |
| **Git Integration** | Designed ⏳ | Full ✅ | Partial ✅ | **BEHIND** (ready to implement) |
| **Repo Mapping** | None ❌ | Tree-sitter ✅ | @workspace ✅ | **CRITICAL GAP** |
| **Search** | Basic ⚠️ | Ripgrep ✅ | @workspace ✅ | **BEHIND** |
| **Auto-lint/test** | Manual ⚠️ | Auto ✅ | Integrated ✅ | **BEHIND** |
| **Multi-agent** | Yes ✅ | No ❌ | Custom ✅ | **AHEAD** |
| **Task Management** | 10 tools ✅ | None ❌ | None ❌ | **UNIQUE** |
| **File Operations** | 9 tools ✅ | 3 basic ✅ | Standard ✅ | **AHEAD** (edit_line) |
| **Context Memory** | SQLite ✅ | Chat only ⚠️ | Limited ⚠️ | **AHEAD** |
| **Multi-language** | 4 langs ✅ | 100+ ✅ | Many ✅ | **GOOD** |

### Key Insights

**Wilson's Unique Strengths:**
1. ✅ LSP integration (no one else has this)
2. ✅ Multi-agent orchestration
3. ✅ Persistent context/memory (SQLite)
4. ✅ Task management system
5. ✅ Surgical editing (edit_line)

**Critical Gaps to Fill:**
1. ❌ Repo mapping (Aider's killer feature)
2. ❌ Fast workspace search (ripgrep)
3. ⏳ Git integration (designed, needs implementation)
4. ❌ Auto-lint/test after changes
5. ❌ Diff preview before applying

**Quick Wins:**
- Implement Git tools (already designed)
- Add ripgrep-based search
- Auto-run diagnostics after edits (leverage existing LSP)
- Simple file tree command

---

**Last Updated:** October 27, 2025
**Status:** Research Complete - LSP implemented, Git designed, Repo mapping identified as #1 gap
**Next Step:** Implement Git tools (1-2 days), then research repo mapping approaches
