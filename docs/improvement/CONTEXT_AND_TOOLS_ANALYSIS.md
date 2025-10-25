# Wilson Context & Tools Gap Analysis

**Date:** October 25, 2025
**Purpose:** Compare Wilson's TaskContext and tool suite against ideal coding assistant capabilities

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

## 🛠️ Current Wilson Tools (50 tools)

### Code Intelligence (10 tools) ✅ EXCELLENT
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

**Assessment:** Wilson has STRONG code intelligence, on par with Claude Code

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

## 🔴 Critical Missing Tools (vs Claude Code / ideal system)

### 1. **Git Operations** 🔴 CRITICAL
```
MISSING:
- git_status           # See what's changed
- git_diff             # View diffs
- git_blame            # Find who changed code
- git_log              # View history
- git_show             # Show commits
- git_branch           # List/switch branches
- git_stash            # Stash changes
```

**Why critical:**
- Can't see what user has modified
- Can't understand code history
- Can't avoid conflicts with uncommitted changes
- Can't create proper commit messages
- Claude Code has full git integration

**Impact if added:** +30% effectiveness for real-world usage

### 2. **LSP/IDE Integration** 🔴 CRITICAL
```
MISSING:
- goto_definition      # Jump to definition
- find_references      # Find all usages
- rename_symbol        # Safe rename across files
- get_hover_info       # Get docs for symbol
- get_diagnostics      # Real-time errors/warnings
- get_completions      # Code completions
- organize_imports     # Clean up imports
```

**Why critical:**
- Current tools require file parsing (slow)
- LSP provides instant, accurate info
- LSP handles multi-file refactoring safely
- LSP knows about ALL files in workspace
- Claude Code uses LSP extensively

**Impact if added:** +40% speed, +20% accuracy

### 3. **Multi-file Refactoring** 🟡 HIGH
```
MISSING:
- rename_across_files  # Safe rename in all files
- extract_function     # Extract to new function
- extract_variable     # Extract to variable
- inline_function      # Inline a function
- move_symbol          # Move to different file
- extract_interface    # Create interface
```

**Why high priority:**
- Current tools are single-file only
- Refactoring requires analyzing ALL usages
- Risk of breaking code in other files
- Claude Code has AST-based refactoring

**Impact if added:** +25% safety for refactorings

### 4. **Search & Navigation** 🟡 HIGH
```
MISSING:
- grep_project         # Fast text search across all files
- find_in_files        # Search with regex
- find_symbol_global   # Find symbol in workspace
- find_type            # Find type definitions
- find_interface_impl  # Find interface implementations
- recent_files         # Files recently opened/modified
```

**Why high priority:**
- Current search_files is basic
- Can't quickly find where symbol is used
- Can't navigate large codebases efficiently
- Claude Code has ripgrep integration

**Impact if added:** +35% speed for discovery

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

**Last Updated:** October 25, 2025
**Status:** Analysis Complete - Ready for implementation planning
**Next Step:** Prioritize Phase 1 tools (git, workspace context, search)
