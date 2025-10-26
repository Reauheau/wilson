# LSP Phase 2: Advanced Tools Implementation Design

**Date:** 2025-10-27
**Status:** Design & Research Complete
**Phase:** Phase 2 Implementation Plan
**Estimated Effort:** 3-5 days

---

## Executive Summary

Phase 1 LSP integration provided **foundational code intelligence** (diagnostics, navigation, symbols). Phase 2 adds **advanced capabilities** that transform Wilson into a sophisticated code analysis and refactoring assistant.

**Phase 2 Goals:**
1. **Deep code understanding** - Type definitions, implementations, workspace-wide symbols
2. **Automated code improvement** - LSP-based formatting and suggested fixes
3. **Safer refactoring** - Code actions with LSP validation

**Key Insight:** Phase 1 gave Wilson "eyes" to see code. Phase 2 gives Wilson "understanding" of architecture and "hands" to improve code safely.

---

## Research Findings

### Current LSP Infrastructure (Phase 1)

**Existing Components:**
- `lsp/manager.go` - Multi-language server lifecycle ✅
- `lsp/client.go` - JSON-RPC communication, has 5 client methods ✅
- `lsp/types.go` - LSP protocol types (needs Phase 2 types added)
- `lsp/cache.go` - Response caching (30s TTL)

**Phase 1 Tools Implemented:**
1. `get_diagnostics` - Real-time errors/warnings
2. `go_to_definition` - Find symbol definitions
3. `find_references` - Find all usages
4. `get_hover_info` - Signatures and docs
5. `get_symbols` - Document symbols (file scope)

**Tool Pattern (from lsp_goto_definition.go:1-196):**
```
1. Tool struct with lspManager
2. Metadata() - name, description, parameters, examples
3. Validate() - parameter validation
4. Execute() - tool logic
   - Get LSP client via packageLSPManager
   - Open document with client.OpenDocument()
   - Call client method (e.g., client.GoToDefinition())
   - Parse and format result
   - Return JSON
5. init() - registry.Register(tool)
```

**Agent Integration (code_agent.go:35-84):**
- Tools listed in SetAllowedTools()
- LSP tools in Phase 1 section (lines 47-52)
- System prompt teaches LLM when to use tools (lines 358-540)

**Auto-Injection (executor.go:281-349):**
- write_file → get_diagnostics (automatic)
- Pattern: Check tool, inject, add to conversation history

---

## Phase 2 Tools: Requirements & Design

### Tool 1: find_implementations

**LSP Method:** `textDocument/implementation`
**Client Method:** NEEDS TO BE ADDED to lsp/client.go
**Tool File:** `capabilities/code_intelligence/lsp_find_implementations.go` (NEW)

#### Use Cases

**Scenario 1: Understanding Polymorphism**
```
User: "Show me all the agent types"
Wilson: *uses get_symbols to find Agent interface*
Wilson: *uses find_implementations on Agent*
Wilson: "Found 5 implementations:
  - CodeAgent (agent/agents/code_agent.go:22)
  - TestAgent (agent/agents/test_agent.go:18)
  - ManagerAgent (agent/orchestration/manager_agent.go:45)
  - ReviewAgent (agent/agents/review_agent.go:12)
  - DebugAgent (agent/agents/debug_agent.go:9)"
```

**Scenario 2: Refactoring Interface**
```
User: "I want to add a new method to the Agent interface"
Wilson: *uses find_implementations*
Wilson: "Warning: This will require changes to 5 implementing types.
  I recommend implementing the new method with a default in BaseAgent first."
```

**Scenario 3: Architectural Analysis**
```
User: "How many database implementations do we have?"
Wilson: *searches for database interface*
Wilson: *uses find_implementations*
Wilson: "Found 3: PostgresDB, MySQL, SQLiteDB"
```

#### Implementation Requirements

**lsp/types.go additions:**
```go
// ImplementationParams represents parameters for find-implementations
type ImplementationParams struct {
    TextDocumentPositionParams
}
```

**lsp/client.go addition:**
```go
// FindImplementations requests all implementations of an interface
func (c *Client) FindImplementations(ctx context.Context, uri string, line, character int) ([]Location, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not initialized")
    }

    params := ImplementationParams{
        TextDocumentPositionParams: TextDocumentPositionParams{
            TextDocument: TextDocumentIdentifier{URI: uri},
            Position:     Position{Line: line, Character: character},
        },
    }

    result, err := c.SendRequest(ctx, "textDocument/implementation", params)
    if err != nil {
        return nil, err
    }

    var locations []Location
    if err := json.Unmarshal(result, &locations); err != nil {
        return nil, fmt.Errorf("failed to parse implementation result: %w", err)
    }

    return locations, nil
}
```

**Tool wrapper parameters:**
- `file` (string, required) - File containing interface reference
- `line` (number, required) - Line number (1-based)
- `character` (number, optional) - Character position (0-based)

**Tool output:**
```json
{
  "query": {"file": "agent/base.go", "line": 15},
  "found": true,
  "implementation_count": 5,
  "implementations": [
    {
      "type": "CodeAgent",
      "file": "agent/agents/code_agent.go",
      "line": 22,
      "preview": "type CodeAgent struct {"
    },
    ...
  ]
}
```

**Risk Level:** RiskSafe (read-only)

**Where to add in code_agent.go:** Line 52, after find_references

**System Prompt Addition (code_agent.go:~520):**
```
**find_implementations** - Find all types that implement an interface 🏗️
Use when:
  - User asks "what implements X interface?"
  - Planning changes to interface (need to know impact)
  - Understanding polymorphism in codebase
  - Architectural analysis

{"tool": "find_implementations", "arguments": {"file": "agent/base.go", "line": 15}}
```

---

### Tool 2: get_type_definition

**LSP Method:** `textDocument/typeDefinition`
**Client Method:** NEEDS TO BE ADDED to lsp/client.go
**Tool File:** `capabilities/code_intelligence/lsp_get_type_definition.go` (NEW)

#### Use Cases

**Scenario 1: Understanding Data Structures**
```
User: "What fields does TaskContext have?"
Wilson: *sees variable "ctx TaskContext"*
Wilson: *uses get_type_definition*
Wilson: "TaskContext has these fields:
  - ProjectPath string
  - GitRoot string
  - TaskID string
  - PreviousAttempts int
  - DependencyFiles []string"
```

**Scenario 2: Following Type Chains**
```
Wilson: *sees task.Input*
Wilson: *uses get_type_definition on Input*
Wilson: "Input is map[string]interface{}"
```

**Scenario 3: Code Generation with Correct Types**
```
User: "Create a function that takes a TaskContext"
Wilson: *uses get_type_definition to see TaskContext struct*
Wilson: *generates function with correct fields accessed*
```

#### Implementation Requirements

**lsp/types.go additions:**
```go
// TypeDefinitionParams represents parameters for type-definition
type TypeDefinitionParams struct {
    TextDocumentPositionParams
}
```

**lsp/client.go addition:**
```go
// GetTypeDefinition requests the type definition of a variable
func (c *Client) GetTypeDefinition(ctx context.Context, uri string, line, character int) ([]Location, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not initialized")
    }

    params := TypeDefinitionParams{
        TextDocumentPositionParams: TextDocumentPositionParams{
            TextDocument: TextDocumentIdentifier{URI: uri},
            Position:     Position{Line: line, Character: character},
        },
    }

    result, err := c.SendRequest(ctx, "textDocument/typeDefinition", params)
    if err != nil {
        return nil, err
    }

    var locations []Location
    if err := json.Unmarshal(result, &locations); err != nil {
        return nil, fmt.Errorf("failed to parse type definition result: %w", err)
    }

    return locations, nil
}
```

**Tool wrapper parameters:**
- `file` (string, required) - File containing variable
- `line` (number, required) - Line number (1-based)
- `character` (number, optional) - Character position (0-based)

**Tool output:**
```json
{
  "query": {"file": "agent/executor.go", "line": 105},
  "found": true,
  "type_definition": {
    "type_name": "TaskContext",
    "file": "agent/base/task_context.go",
    "line": 18,
    "kind": "struct",
    "preview": "type TaskContext struct {"
  }
}
```

**Risk Level:** RiskSafe (read-only)

**Where to add in code_agent.go:** Line 53, after find_implementations

**System Prompt Addition:**
```
**get_type_definition** - Jump to type definition of a variable 🔍
Use when:
  - Need to understand what fields/methods a type has
  - Following data flow through variables
  - Generating code that uses unfamiliar types

{"tool": "get_type_definition", "arguments": {"file": "agent/executor.go", "line": 105}}
```

---

### Tool 3: get_symbols (workspace scope)

**LSP Method:** `workspace/symbol`
**Client Method:** NEEDS TO BE ADDED to lsp/client.go
**Tool File:** ENHANCE EXISTING `lsp_get_symbols.go`

**Current Status:** Phase 1 implemented document scope only
**Phase 2:** Add workspace scope capability

#### Use Cases

**Scenario 1: Project-Wide Symbol Search**
```
User: "Find all functions related to validation"
Wilson: *uses get_symbols with query="Validate" scope="workspace"*
Wilson: "Found 12 validation functions:
  - ValidateTask (agent/validation/validator.go:45)
  - ValidateInput (agent/validation/validator.go:67)
  - ValidateEmail (util/validator.go:23)
  ..."
```

**Scenario 2: Finding Symbols Without Location**
```
User: "Where is ProcessTask defined?"
Wilson: *uses get_symbols scope=workspace query="ProcessTask"*
Wilson: "ProcessTask found in agent/executor.go:234"
```

**Scenario 3: Architecture Discovery**
```
User: "List all agent types"
Wilson: *uses get_symbols scope=workspace query="Agent"*
Wilson: "Found 5 *Agent types, 1 Agent interface, 3 agent-related structs"
```

#### Implementation Requirements

**lsp/types.go additions:**
```go
// WorkspaceSymbolParams represents parameters for workspace symbol search
type WorkspaceSymbolParams struct {
    Query string `json:"query"`
}

// SymbolInformation represents symbol information (workspace scope)
type SymbolInformation struct {
    Name          string       `json:"name"`
    Kind          SymbolKind   `json:"kind"`
    Location      Location     `json:"location"`
    ContainerName string       `json:"containerName,omitempty"`
}

// SymbolKind represents the kind of symbol
type SymbolKind int

const (
    SymbolKindFile        SymbolKind = 1
    SymbolKindModule      SymbolKind = 2
    SymbolKindNamespace   SymbolKind = 3
    SymbolKindPackage     SymbolKind = 4
    SymbolKindClass       SymbolKind = 5
    SymbolKindMethod      SymbolKind = 6
    SymbolKindProperty    SymbolKind = 7
    SymbolKindField       SymbolKind = 8
    SymbolKindConstructor SymbolKind = 9
    SymbolKindEnum        SymbolKind = 10
    SymbolKindInterface   SymbolKind = 11
    SymbolKindFunction    SymbolKind = 12
    SymbolKindVariable    SymbolKind = 13
    SymbolKindConstant    SymbolKind = 14
    SymbolKindString      SymbolKind = 15
    SymbolKindNumber      SymbolKind = 16
    SymbolKindBoolean     SymbolKind = 17
    SymbolKindArray       SymbolKind = 18
    SymbolKindStruct      SymbolKind = 22
)
```

**lsp/client.go addition:**
```go
// GetWorkspaceSymbols requests symbols across the entire workspace
func (c *Client) GetWorkspaceSymbols(ctx context.Context, query string) ([]SymbolInformation, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not initialized")
    }

    params := WorkspaceSymbolParams{
        Query: query,
    }

    result, err := c.SendRequest(ctx, "workspace/symbol", params)
    if err != nil {
        return nil, err
    }

    var symbols []SymbolInformation
    if err := json.Unmarshal(result, &symbols); err != nil {
        return nil, fmt.Errorf("failed to parse workspace symbols: %w", err)
    }

    return symbols, nil
}
```

**Tool wrapper enhancement:**
Add `scope` parameter to existing get_symbols:
- `scope` ("document" | "workspace", default "document")
- `query` (string, required for workspace scope)

**Tool output (workspace mode):**
```json
{
  "scope": "workspace",
  "query": "Validate",
  "symbol_count": 12,
  "symbols": [
    {
      "name": "ValidateTask",
      "kind": "function",
      "file": "agent/validation/validator.go",
      "line": 45,
      "container": "validator"
    },
    ...
  ]
}
```

**Risk Level:** RiskSafe (read-only)

**System Prompt Addition:**
```
**get_symbols** - List symbols in file OR search entire workspace 📋
Document scope: List all functions/types in a file
Workspace scope: Search for symbols across entire project

// File scope (Phase 1)
{"tool": "get_symbols", "arguments": {"file": "agent/executor.go"}}

// Workspace scope (Phase 2 - NEW)
{"tool": "get_symbols", "arguments": {"scope": "workspace", "query": "Validate"}}
```

---

### Tool 4: format_code (LSP-based)

**LSP Method:** `textDocument/formatting` OR `textDocument/rangeFormatting`
**Client Method:** NEEDS TO BE ADDED to lsp/client.go
**Tool File:** ENHANCE EXISTING `capabilities/code_intelligence/quality/format_code.go`

**Current Status:** Uses gofmt/goimports directly (Go-only)
**Phase 2:** Add LSP-based formatting (multi-language)

#### Use Cases

**Scenario 1: Auto-Format After Generation**
```
Wilson: *generates new code*
Wilson: *auto-calls format_code via LSP*
Wilson: "Code generated and formatted"
```

**Scenario 2: Multi-Language Formatting**
```
User: "Format this Python file"
Wilson: *uses LSP formatting (not gofmt)*
Wilson: "Formatted using pyls"
```

**Scenario 3: Range Formatting**
```
User: "Fix the indentation in this function"
Wilson: *uses rangeFormatting for specific lines*
Wilson: "Formatted lines 45-67"
```

#### Implementation Requirements

**lsp/types.go additions:**
```go
// DocumentFormattingParams represents parameters for document formatting
type DocumentFormattingParams struct {
    TextDocument TextDocumentIdentifier `json:"textDocument"`
    Options      FormattingOptions      `json:"options"`
}

// DocumentRangeFormattingParams represents parameters for range formatting
type DocumentRangeFormattingParams struct {
    TextDocument TextDocumentIdentifier `json:"textDocument"`
    Range        Range                  `json:"range"`
    Options      FormattingOptions      `json:"options"`
}

// FormattingOptions represents formatting options
type FormattingOptions struct {
    TabSize      int  `json:"tabSize"`
    InsertSpaces bool `json:"insertSpaces"`
}

// TextEdit represents a text edit
type TextEdit struct {
    Range   Range  `json:"range"`
    NewText string `json:"newText"`
}
```

**lsp/client.go additions:**
```go
// FormatDocument requests formatting for entire document
func (c *Client) FormatDocument(ctx context.Context, uri string, tabSize int, insertSpaces bool) ([]TextEdit, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not initialized")
    }

    params := DocumentFormattingParams{
        TextDocument: TextDocumentIdentifier{URI: uri},
        Options: FormattingOptions{
            TabSize:      tabSize,
            InsertSpaces: insertSpaces,
        },
    }

    result, err := c.SendRequest(ctx, "textDocument/formatting", params)
    if err != nil {
        return nil, err
    }

    var edits []TextEdit
    if err := json.Unmarshal(result, &edits); err != nil {
        return nil, fmt.Errorf("failed to parse formatting result: %w", err)
    }

    return edits, nil
}

// FormatRange requests formatting for a specific range
func (c *Client) FormatRange(ctx context.Context, uri string, startLine, startChar, endLine, endChar int, tabSize int, insertSpaces bool) ([]TextEdit, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not initialized")
    }

    params := DocumentRangeFormattingParams{
        TextDocument: TextDocumentIdentifier{URI: uri},
        Range: Range{
            Start: Position{Line: startLine, Character: startChar},
            End:   Position{Line: endLine, Character: endChar},
        },
        Options: FormattingOptions{
            TabSize:      tabSize,
            InsertSpaces: insertSpaces,
        },
    }

    result, err := c.SendRequest(ctx, "textDocument/rangeFormatting", params)
    if err != nil {
        return nil, err
    }

    var edits []TextEdit
    if err := json.Unmarshal(result, &edits); err != nil {
        return nil, fmt.Errorf("failed to parse range formatting result: %w", err)
    }

    return edits, nil
}
```

**Tool wrapper enhancement:**
Keep existing gofmt/goimports path, ADD LSP path:
- Try LSP formatting first
- Fall back to gofmt if LSP unavailable
- Support range formatting with `start_line`, `end_line` parameters

**Risk Level:** RiskModerate (modifies code)

**Auto-Injection Opportunity:**
Similar to get_diagnostics after write_file, could auto-format after write_file succeeds

**System Prompt Update:**
```
**format_code** - Auto-format code (LSP + language-specific tools) 🎨
Use after:
  - Generating new code
  - Making manual edits
  - Before presenting changes to user

{"tool": "format_code", "arguments": {"path": "main.go"}}
```

---

### Tool 5: get_code_actions

**LSP Method:** `textDocument/codeAction`
**Client Method:** NEEDS TO BE ADDED to lsp/client.go
**Tool File:** `capabilities/code_intelligence/lsp_get_code_actions.go` (NEW)

#### Use Cases

**Scenario 1: Auto-Fix Import Errors**
```
Wilson: *gets diagnostics showing missing import*
Wilson: *calls get_code_actions*
Wilson: "LSP suggests: Add import 'fmt'"
Wilson: *auto-applies the fix*
```

**Scenario 2: Quick Fixes**
```
LSP Diagnostic: "unused variable: result"
Wilson: *calls get_code_actions*
Wilson: "Available fixes:
  1. Remove unused variable
  2. Use variable with _ = result
  3. Return the variable"
Wilson: *applies fix 1*
```

**Scenario 3: Refactoring Suggestions**
```
User: "This function is too long"
Wilson: *calls get_code_actions on function*
Wilson: "LSP suggests:
  - Extract method
  - Inline variable
  - Simplify conditional"
```

#### Implementation Requirements

**lsp/types.go additions:**
```go
// CodeActionParams represents parameters for code actions
type CodeActionParams struct {
    TextDocument TextDocumentIdentifier `json:"textDocument"`
    Range        Range                  `json:"range"`
    Context      CodeActionContext      `json:"context"`
}

// CodeActionContext represents code action context
type CodeActionContext struct {
    Diagnostics []Diagnostic `json:"diagnostics"`
}

// CodeAction represents a code action
type CodeAction struct {
    Title       string                 `json:"title"`
    Kind        string                 `json:"kind,omitempty"` // quickfix, refactor, etc.
    Diagnostics []Diagnostic           `json:"diagnostics,omitempty"`
    Edit        *WorkspaceEdit         `json:"edit,omitempty"`
    Command     *Command               `json:"command,omitempty"`
    IsPreferred bool                   `json:"isPreferred,omitempty"`
}

// WorkspaceEdit represents changes to workspace
type WorkspaceEdit struct {
    Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// Command represents a server command
type Command struct {
    Title     string        `json:"title"`
    Command   string        `json:"command"`
    Arguments []interface{} `json:"arguments,omitempty"`
}
```

**lsp/client.go addition:**
```go
// GetCodeActions requests code actions for a range with diagnostics
func (c *Client) GetCodeActions(ctx context.Context, uri string, startLine, startChar, endLine, endChar int, diagnostics []Diagnostic) ([]CodeAction, error) {
    if !c.initialized {
        return nil, fmt.Errorf("client not initialized")
    }

    params := CodeActionParams{
        TextDocument: TextDocumentIdentifier{URI: uri},
        Range: Range{
            Start: Position{Line: startLine, Character: startChar},
            End:   Position{Line: endLine, Character: endChar},
        },
        Context: CodeActionContext{
            Diagnostics: diagnostics,
        },
    }

    result, err := c.SendRequest(ctx, "textDocument/codeAction", params)
    if err != nil {
        return nil, err
    }

    var actions []CodeAction
    if err := json.Unmarshal(result, &actions); err != nil {
        return nil, fmt.Errorf("failed to parse code actions: %w", err)
    }

    return actions, nil
}

// ApplyCodeAction applies a code action's edit
func (c *Client) ApplyCodeAction(ctx context.Context, action CodeAction) error {
    // This is a helper - in practice the tool will apply edits directly
    // by reading the WorkspaceEdit and applying text changes
    return nil
}
```

**Tool wrapper parameters:**
- `file` (string, required)
- `line` (number, required) - Line with issue
- `diagnostics` (array, optional) - Diagnostics from get_diagnostics

**Tool output:**
```json
{
  "file": "main.go",
  "line": 42,
  "action_count": 3,
  "actions": [
    {
      "title": "Add import \"fmt\"",
      "kind": "quickfix",
      "is_preferred": true,
      "changes": {
        "file:///path/main.go": [
          {
            "range": {"start": {"line": 3, "character": 0}, "end": {"line": 3, "character": 0}},
            "new_text": "import \"fmt\"\n"
          }
        ]
      }
    },
    ...
  ]
}
```

**Risk Level:** RiskSafe (just lists actions), RiskModerate (if auto-applying)

**Auto-Injection Opportunity:**
After LSP diagnostics show errors, automatically call get_code_actions and apply preferred quickfixes

**System Prompt Addition:**
```
**get_code_actions** - Get LSP-suggested fixes and refactorings 💡
Use when:
  - Diagnostics show errors (LSP might have auto-fix)
  - Looking for refactoring suggestions
  - User asks "how can I improve this?"

{"tool": "get_code_actions", "arguments": {"file": "main.go", "line": 42}}
```

---

## Implementation Strategy

### Phase 2A: Type Intelligence (2 days)
1. Add LSP types to lsp/types.go
2. Add client methods to lsp/client.go:
   - FindImplementations()
   - GetTypeDefinition()
   - GetWorkspaceSymbols()
3. Create tool wrappers:
   - lsp_find_implementations.go
   - lsp_get_type_definition.go
4. Enhance lsp_get_symbols.go for workspace scope
5. Add to code_agent.go allowed tools
6. Update system prompts

### Phase 2B: Code Actions & Formatting (1-2 days)
1. Add code action types to lsp/types.go
2. Add formatting types to lsp/types.go
3. Add client methods:
   - GetCodeActions()
   - FormatDocument()
   - FormatRange()
4. Create lsp_get_code_actions.go
5. Enhance quality/format_code.go with LSP path
6. Add to code_agent.go allowed tools
7. Update system prompts

### Phase 2C: Auto-Injection & Testing (1 day)
1. Add auto-injection patterns:
   - After diagnostics → get_code_actions (if errors)
   - After write_file → format_code (optional)
2. Create E2E tests
3. Update documentation

---

## Agent Integration Points

### code_agent.go Changes

**Line 35-84: Add new tools to allowed list**
```go
// ===== LSP Code Intelligence (Phase 2) - ADVANCED =====
"find_implementations",  // Find all types implementing interface
"get_type_definition",   // Jump to type definition of variable
// get_symbols already exists, enhanced for workspace scope
"get_code_actions",      // Get LSP-suggested fixes
// format_code already exists in quality/, enhanced with LSP
```

**Lines 358-540: Add to system prompt**
Add sections for each new tool with examples and guidance

### executor.go Potential Changes

**Auto-injection after diagnostics (line ~345):**
```go
// If LSP found errors, check for auto-fixes
if hasErrors {
    actionsCall := ToolCall{
        Tool: "get_code_actions",
        Arguments: map[string]interface{}{
            "file": targetPath,
            "line": firstErrorLine,
        },
    }

    actionsResult, _ := ate.executor.Execute(ctx, actionsCall)
    // Parse actions, auto-apply preferred quickfixes
}
```

---

## Dependencies & Prerequisites

### Required for Phase 2
- ✅ Phase 1 complete (foundation exists)
- ✅ LSP Manager working
- ✅ Tool registration system
- ✅ gopls installed and accessible

### Optional Enhancements
- Auto-format on write (can add later)
- Code action auto-application (can add later)
- Caching for workspace symbols (can add later)

---

## Success Criteria

### Phase 2A Success:
- ✅ find_implementations finds all interface implementations
- ✅ get_type_definition shows type structure
- ✅ get_symbols(workspace) searches entire project
- ✅ All 3 tools work in E2E tests

### Phase 2B Success:
- ✅ format_code uses LSP for non-Go languages
- ✅ get_code_actions returns LSP suggestions
- ✅ Code actions can be applied programmatically
- ✅ All tools documented in system prompt

### Phase 2C Success:
- ✅ E2E tests validate all Phase 2 tools
- ✅ Auto-injection improves error fix rate
- ✅ Documentation complete
- ✅ No regressions in Phase 1 tools

---

## Risk Assessment

### Low Risk
- Adding client methods (isolated changes)
- New tool wrappers (follow existing pattern)
- System prompt updates (informational)

### Medium Risk
- Workspace symbol search (could be slow on large projects - need caching)
- Code action application (could break code if logic wrong - needs careful testing)
- Format code LSP path (fallback to gofmt ensures safety)

### Mitigation Strategies
- Cache workspace symbol results (5 min TTL)
- Require user confirmation for code action application initially
- Always test formatting changes with get_diagnostics before applying
- Comprehensive E2E tests

---

## Estimated Timeline

| Phase | Tasks | Effort |
|-------|-------|--------|
| 2A | Type intelligence (implementations, type def, workspace symbols) | 2 days |
| 2B | Code actions & LSP formatting | 1-2 days |
| 2C | Auto-injection, testing, docs | 1 day |
| **Total** | | **4-5 days** |

---

## Next Steps

1. **Review this design doc** - Get approval on approach
2. **Start Phase 2A** - Implement type intelligence tools
3. **Test incrementally** - Validate each tool before moving on
4. **Phase 2B** - Add code actions and formatting
5. **Phase 2C** - Polish, test, document

---

**Questions to Resolve:**
1. Should format_code auto-run after write_file? (Probably yes, but make it optional)
2. Should code actions auto-apply? (Start with manual, add auto later)
3. How long to cache workspace symbols? (Suggest 5 minutes, clear on file changes)
4. Priority order for implementation? (Suggest: implementations → type def → workspace → code actions → format)

---

**Status:** Ready for implementation
**Next Document:** LSP_PHASE2_IMPLEMENTATION_GUIDE.md (once approved)
