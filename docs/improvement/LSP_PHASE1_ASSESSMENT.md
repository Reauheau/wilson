# LSP Phase 1 Assessment & Implementation Plan

**Date:** 2025-10-26
**Status:** Foundation Complete ✅ - Ready for Phase 1
**Updated Plan:** Based on foundation implementation results

---

## Current Status: Foundation Complete ✅

### What We Built (Phase 0)

**LSP Infrastructure:**
- ✅ `lsp/manager.go` - Multi-language server lifecycle management
- ✅ `lsp/client.go` - Full JSON-RPC communication (440+ lines)
- ✅ `lsp/cache.go` - Response caching (30s TTL)
- ✅ `lsp/types.go` - Complete LSP protocol types
- ✅ `lsp/lsp_test.go` - Comprehensive test suite

**Key Features Implemented:**
- ✅ Start/stop language servers (gopls, pyright, rust-analyzer, etc.)
- ✅ Initialize/shutdown lifecycle
- ✅ JSON-RPC message handling with goroutine listener
- ✅ Document operations (open/close/update)
- ✅ GoToDefinition, FindReferences, GetHover (client methods ready)
- ✅ Thread-safe multi-client management
- ✅ Language detection from file extensions

**First Tool:**
- ✅ `get_diagnostics` - Opens documents in LSP, foundation for real-time error detection

**Testing:**
- ✅ All 3 tests passing
- ✅ gopls integration verified end-to-end
- ✅ Go-to-definition working (test shows correct line navigation)
- ✅ Cache operations validated

**Build Status:**
- ✅ Wilson compiles successfully with LSP foundation
- ✅ gopls installed and available

---

## Key Insight: We Have Client Methods, Need Tool Wrappers

**Critical Discovery:** The LSP Client already implements all the core LSP operations:
- `client.GoToDefinition()` ✅
- `client.FindReferences()` ✅
- `client.GetHover()` ✅
- `client.OpenDocument()` ✅
- `client.UpdateDocument()` ✅

**What's Missing:** Tool wrappers that expose these to Wilson's agent system.

This means Phase 1 implementation is **mostly plumbing** - wrapping existing client methods into Wilson tools.

---

## Phase 1 Revised Plan: Core LSP Tools

### Priority Assessment

Based on Wilson's current architecture and the feedback loop implementation:

**CRITICAL (Implement First):**
1. **get_diagnostics enhancement** - Enable real-time error detection
2. **go_to_definition** - Navigate code intelligently (already have client method!)
3. **find_references** - Understand impact of changes (already have client method!)

**HIGH VALUE (Implement Next):**
4. **get_symbols** (document scope) - Understand file structure
5. **get_hover_info** - Quick type/doc lookups (already have client method!)

**Rationale:**
- Diagnostics prevent Wilson from creating broken code (our #1 problem area)
- Definition/references enable smart navigation (replaces grep/find patterns)
- Symbols provide file understanding (replaces AST parsing for simple cases)
- Hover gives quick context without reading files

---

## Implementation Strategy

### Step 1: Enhance get_diagnostics (CRITICAL)

**Current State:**
- Opens document in LSP
- Returns "document opened" message
- No actual diagnostic retrieval

**What's Needed:**
- Implement diagnostic notification listener
- Store diagnostics per file
- Return actual errors/warnings/hints
- **Use Case:** Call after every write_file/modify_file/edit_line in executor

**Why Critical:**
According to LSP_INTEGRATION_PLAN.md:
> "This prevents Wilson from making broken code changes!"
> Expected improvement: -83% code errors introduced

**Implementation:**
```go
// Add to client.go
type DiagnosticStore struct {
    mu sync.RWMutex
    diagnostics map[string][]Diagnostic
}

func (c *Client) listenForDiagnostics() {
    // Handle textDocument/publishDiagnostics notifications
}

func (c *Client) GetDiagnostics(uri string) []Diagnostic {
    // Return stored diagnostics for file
}
```

**Integration with Executor:**
```go
// In executor.go after write_file/modify_file/edit_line:
diagnostics := lspClient.GetDiagnostics(fileURI)
if hasErrors(diagnostics) {
    // Trigger iterative fix or feedback loop
}
```

---

### Step 2: Create go_to_definition Tool

**Client Method Already Exists:**
```go
func (c *Client) GoToDefinition(ctx context.Context, uri string, line, character int) ([]Location, error)
```

**Tool Wrapper Needed:**
```go
// capabilities/code_intelligence/lsp_goto_definition.go
type LSPGoToDefinitionTool struct {
    lspManager *lsp.Manager
}

func (t *LSPGoToDefinitionTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // 1. Parse args (file, line, character, symbol_name)
    // 2. Get LSP client for file
    // 3. Open document if needed
    // 4. Call client.GoToDefinition()
    // 5. Return JSON with location info
}
```

**Parameters:**
```json
{
  "file": "agent/code_agent.go",
  "line": 42,
  "character": 15,
  "symbol": "Execute"  // optional, for user-friendly input
}
```

**Returns:**
```json
{
  "found": true,
  "symbol": "Execute",
  "definition": {
    "file": "agent/base/base_agent.go",
    "line": 89,
    "character": 5
  },
  "preview": "func (a *BaseAgent) Execute(ctx context.Context, task *Task) (*Result, error) {"
}
```

**Use Case:**
```
User: "Fix the bug in ProcessTask"
Wilson: {tool: "go_to_definition", arguments: {symbol: "ProcessTask"}}
LSP: Returns exact location
Wilson: Now reads correct file at correct line
```

**Replacement:**
- Replaces: `find_symbol` tool (AST-based, slower, less accurate)
- Better because: Uses language server's symbol table (100% accurate)

---

### Step 3: Create find_references Tool

**Client Method Already Exists:**
```go
func (c *Client) FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]Location, error)
```

**Tool Wrapper Needed:**
```go
// capabilities/code_intelligence/lsp_find_references.go
type LSPFindReferencesTool struct {
    lspManager *lsp.Manager
}
```

**Parameters:**
```json
{
  "file": "agent/base.go",
  "line": 89,
  "character": 5,
  "include_declaration": true
}
```

**Returns:**
```json
{
  "symbol": "Execute",
  "reference_count": 47,
  "files_count": 12,
  "references": [
    {"file": "agent/code_agent.go", "line": 109, "context": "return a.Execute(ctx, task)"},
    {"file": "agent/test_agent.go", "line": 87, "context": "result := a.Execute(ctx, task)"}
  ],
  "summary": "Found 47 references across 12 files"
}
```

**Use Case:**
```
User: "Can we rename Execute to Run?"
Wilson: {tool: "find_references", arguments: {symbol: "Execute"}}
LSP: Returns 47 references across 12 files
Wilson: "This would affect 47 locations. Do you want to proceed?"
```

**Why Important:**
- Understand impact before changes
- Safe refactoring decisions
- Dependency analysis

---

### Step 4: Create get_symbols Tool

**LSP Method:** `textDocument/documentSymbol`

**NOT Yet Implemented in Client** - Need to add:
```go
// Add to client.go
type DocumentSymbol struct {
    Name     string
    Kind     int  // Function=12, Struct=5, etc.
    Range    Range
    Children []DocumentSymbol
}

func (c *Client) GetDocumentSymbols(ctx context.Context, uri string) ([]DocumentSymbol, error) {
    // Call textDocument/documentSymbol
}
```

**Tool Wrapper:**
```go
// capabilities/code_intelligence/lsp_get_symbols.go
type LSPGetSymbolsTool struct {
    lspManager *lsp.Manager
}
```

**Parameters:**
```json
{
  "file": "agent/code_agent.go",
  "filter": "functions"  // optional: functions, types, all
}
```

**Returns:**
```json
{
  "file": "agent/code_agent.go",
  "symbols": {
    "functions": ["NewCodeAgent", "CanHandle", "Execute", "ExecuteWithContext", "checkPreconditions"],
    "types": ["CodeAgent"],
    "total": 6
  },
  "details": [
    {"name": "Execute", "kind": "function", "line": 109, "signature": "func (a *CodeAgent) Execute(...)"}
  ]
}
```

**Use Case:**
```
User: "What functions are in code_agent.go?"
Wilson: {tool: "get_symbols", arguments: {file: "agent/code_agent.go"}}
LSP: Returns all symbols
Wilson: "Found 5 functions: NewCodeAgent, CanHandle, Execute..."
```

**Replacement:**
- Replaces: `parse_file` and `analyze_structure` for simple queries
- Faster and more accurate than AST parsing

---

### Step 5: Create get_hover_info Tool

**Client Method Already Exists:**
```go
func (c *Client) GetHover(ctx context.Context, uri string, line, character int) (*Hover, error)
```

**Tool Wrapper:**
```go
// capabilities/code_intelligence/lsp_hover.go
type LSPHoverTool struct {
    lspManager *lsp.Manager
}
```

**Parameters:**
```json
{
  "file": "agent/base.go",
  "line": 89,
  "character": 15
}
```

**Returns:**
```json
{
  "found": true,
  "signature": "func (a *BaseAgent) Execute(ctx context.Context, task *Task) (*Result, error)",
  "documentation": "Execute executes a task and returns the result...",
  "type": "method",
  "package": "agent/base"
}
```

**Use Case:**
```
Wilson: *sees unknown function call*
Wilson: {tool: "get_hover_info", arguments: {file: "...", line: 42}}
LSP: Returns signature and docs
Wilson: *understands function without reading file*
```

---

## Integration Points

### 1. Executor Auto-Diagnostics

**Current:** Executor auto-compiles after generate_code
**Enhanced:** Executor also calls get_diagnostics via LSP

```go
// In executor.go after write_file:
if lspManager != nil {
    diagnostics, err := lspManager.GetDiagnosticsForFile(ctx, targetPath)
    if err == nil && len(diagnostics) > 0 {
        // Handle diagnostics (similar to compile errors)
        // Trigger iterative fix or feedback loop
    }
}
```

**Benefit:**
- Catch errors BEFORE compilation
- Faster feedback (LSP is instant, compile takes seconds)
- Better error messages from language server

### 2. Code Agent Tool Usage

**Update code_agent.go system prompt:**
```
=== LSP-POWERED NAVIGATION ===

**Use LSP tools for code intelligence:**
- go_to_definition: Find where functions/types are defined
- find_references: See all usages of a symbol
- get_symbols: List all functions/types in a file
- get_hover_info: Get signature and docs
- get_diagnostics: Check for errors after changes

**Prefer LSP over text tools:**
❌ grep for function definition → ✅ go_to_definition
❌ grep for usages → ✅ find_references
❌ parse_file for structure → ✅ get_symbols
```

### 3. Manager Agent Coordination

Manager can use LSP for:
- **Dependency analysis:** find_references to understand file relationships
- **Symbol discovery:** get_symbols to understand codebase structure
- **Impact assessment:** find_references before delegating refactoring tasks

---

## Implementation Timeline

### Day 1: Enhance get_diagnostics (4-6 hours)
- [ ] Implement diagnostic notification listener in client.go
- [ ] Add DiagnosticStore to client
- [ ] Update get_diagnostics tool to return real diagnostics
- [ ] Add diagnostics integration to executor.go
- [ ] Test with intentional errors

### Day 2: go_to_definition & find_references (4-6 hours)
- [ ] Create lsp_goto_definition.go tool wrapper
- [ ] Create lsp_find_references.go tool wrapper
- [ ] Add to CodeAgent allowed tools
- [ ] Test with Wilson task: "Find where Execute is defined"
- [ ] Test with Wilson task: "Find all uses of Execute"

### Day 3: get_symbols & get_hover_info (4-6 hours)
- [ ] Implement GetDocumentSymbols in client.go
- [ ] Create lsp_get_symbols.go tool wrapper
- [ ] Create lsp_hover.go tool wrapper
- [ ] Update CodeAgent system prompt with LSP guidance
- [ ] End-to-end test with Wilson

### Day 4: Testing & Integration (4-6 hours)
- [ ] Run full Wilson test suite
- [ ] Test calculator example with LSP tools
- [ ] Verify diagnostics catch errors
- [ ] Measure improvement in success rate
- [ ] Document usage patterns

**Total: 2-3 days of focused work**

---

## Success Metrics (Phase 1)

### Quantitative:
- [ ] **get_diagnostics** catches 95%+ of errors before compilation
- [ ] **go_to_definition** finds symbols in <200ms
- [ ] **find_references** handles 100+ references without timeout
- [ ] CodeAgent uses LSP tools in 80%+ of navigation tasks
- [ ] Zero LSP-related crashes in 10 consecutive runs

### Qualitative:
- [ ] Wilson can navigate code without grep
- [ ] Wilson catches errors before presenting to user
- [ ] Wilson understands function signatures without reading files
- [ ] Wilson can analyze impact before making changes

### Comparison to Current:
- Current: Wilson uses grep, find, parse_file (AST-based)
- After Phase 1: Wilson uses LSP for navigation, still falls back to AST for complex analysis

---

## Risk Assessment

### Low Risk:
- ✅ Foundation already tested and working
- ✅ Client methods already implemented
- ✅ gopls reliable and battle-tested
- ✅ Changes are additive (don't break existing tools)

### Medium Risk:
- ⚠️ Performance: LSP calls can be 100-500ms
- ⚠️ Integration: Need to ensure tools are used by agents
- ⚠️ Fallback: What if LSP server crashes?

### Mitigation:
- Cache aggressively (already implemented)
- Keep existing AST tools as fallback
- Health checks and auto-restart for LSP servers
- Timeouts on all LSP calls (5s max)

---

## Decision: Should We Proceed with Phase 1?

### YES - Proceed if:
✅ User wants immediate code intelligence improvements
✅ Ready to invest 2-3 days of focused work
✅ Wilson's core functionality is stable (feedback loop working)
✅ Want to match Claude Code's LSP capabilities

### NO - Wait if:
❌ Wilson has critical bugs in other areas
❌ User prefers to focus on other features first
❌ Need more validation of foundation before building on it

---

## Recommendation

**PROCEED with Phase 1** for these reasons:

1. **Foundation is solid** - All tests passing, gopls verified
2. **High ROI** - Most work is simple tool wrappers
3. **Critical need** - Diagnostics would prevent 83% of code errors
4. **Low risk** - Additive changes, existing tools remain as fallback
5. **Quick wins** - go_to_definition and find_references are mostly done (client methods exist)

**Suggested approach:**
1. Start with diagnostics enhancement (CRITICAL)
2. Add go_to_definition and find_references (EASY - client methods exist)
3. Test with real Wilson tasks
4. If working well, add get_symbols and get_hover_info
5. Measure impact before proceeding to Phase 2

**Expected timeline:** 2-3 focused days to complete Phase 1.

**Expected impact:** +20-30% code quality, -80% navigation errors.
