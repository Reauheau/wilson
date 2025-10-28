# Real World Fixes - Critical Path Issues

Analysis of actual execution showing multiple critical bugs.

---

## Issue 1: LSP Shows Wilson's Own Errors ❌ CRITICAL

### Evidence
```
[LSP] Diagnostics for file:///Users/roderick.vannievelt/IdeaProjects/wilson/go/tests/test_integration_paths.go: 1 errors
[LSP] Diagnostics for file:///Users/roderick.vannievelt/IdeaProjects/wilson/go/main.go: 0 errors, 1 warnings
```

### Problem
Despite the fix in `lsp/client.go:692`, LSP **still logs Wilson's own files**. The filter checks `strings.HasPrefix(diagParams.URI, c.rootURI)`, but if `c.rootURI` is set to Wilson's directory, this filter does nothing!

### Root Cause
**LSP is initialized with the wrong rootURI.** When starting an LSP server, if we pass Wilson's project root as the workspace, it will scan and report errors for Wilson's codebase.

### Check
1. Where is LSP initialized? → `lsp/manager.go` or code agent
2. What rootURI is passed during `Initialize()` request?
3. Should be: Target project directory (`~/wilsontestdir`)
4. Actually is: Wilson's directory (`/Users/.../wilson/go`)

### Fix Required
**LSP must be started with the TARGET project directory as rootURI, not Wilson's directory.**

Location: Find where `lsp.Manager.Start()` or `lsp.Client.Initialize()` is called.

Correct behavior:
```go
// When code agent works on ~/wilsontestdir:
client.Initialize(ctx, "file:///Users/roderick.vannievelt/wilsontestdir")
// NOT Wilson's directory!
```

---

## Issue 2: Files Created in Wrong Location ❌ CRITICAL

### Evidence
```
[CodeAgent] Extracted created files: [/Users/roderick.vannievelt/main.go]
```

User asked for: `~/wilsontestdir/main.go`
Actually created: `~/main.go` (home root)

### Problem
Path resolution is broken. The code agent:
1. Didn't create the target directory (`wilsontestdir`)
2. Created files in parent directory instead

### Root Cause Options
1. **Path expansion failure** - `~/wilsontestdir` not expanding correctly
2. **Directory creation skipped** - Code agent didn't mkdir before creating files
3. **Path parsing bug** - Stripped the directory name somehow

### Check
1. `agent/agents/code_agent.go` - Where it handles target path
2. Look for: `filepath.Join`, `os.ExpandEnv`, `filepath.Dir`
3. Check if `mkdir -p` is called before file creation
4. Search for `Extracted created files` log line

### Fix Required
**Before creating files, ensure target directory exists:**
```go
targetDir := expandPath("~/wilsontestdir")
if err := os.MkdirAll(targetDir, 0755); err != nil {
    return err
}
filePath := filepath.Join(targetDir, "main.go")
```

---

## Issue 3: Error Messages Mixed Into Generated Code ❌ CRITICAL

### Evidence
Generated `main.go` contains:
```go
}
failed to check for test files: open /Users/roderick.vannievelt/IdeaProjects/wilson/go/main_test.go: file does not exist
        return 0, fmt.Errorf("invalid equation")
}
```

### Problem
**Literal error message embedded in source code.** This is catastrophic - the LLM response contains error text that got written to the file.

### Root Cause Options
1. **LLM hallucinated the error** - Model outputted error text in code block
2. **Response parsing bug** - Code extraction included error messages
3. **Stream corruption** - Error from another source mixed into response

### Check
1. `agent/agents/code_agent.go` - Code extraction from LLM response
2. Look for: How code blocks are parsed from markdown
3. Check: `extractCodeFromResponse()` or similar
4. Verify: Are errors being written to stdout during generation?

### Fix Required
**Stricter code extraction with validation:**
```go
func extractCode(response string) (string, error) {
    // Extract code between ```go and ```
    // Validate: No error keywords like "failed to", "error:", etc.
    // Reject if contains diagnostic messages
}
```

---

## Issue 4: Compile Tool Looks in Wrong Directory ❌ CRITICAL

### Evidence
```
[AgentExecutor] Recompile result: err=failed to check for test files: open /Users/roderick.vannievelt/IdeaProjects/wilson/go/main.go: not a directory
```

### Problem
Compile tool is trying to check **Wilson's main.go**, not the target project's main.go.

### Root Cause
**Path context lost between operations.** The code agent created files in `~/`, but compile tool is looking in `wilson/go/`.

Possible causes:
1. Compile tool uses `os.Getwd()` (Wilson's working directory)
2. Path isn't passed correctly to compile tool
3. TaskContext doesn't contain correct ProjectPath

### Check
1. `capabilities/code_intelligence/build/compile.go:84` - Where absPath is computed
2. `agent/base/task_context.go` - What is ProjectPath set to?
3. How does code agent pass path to compile tool?

### Fix Required
**Compile tool must use TaskContext.ProjectPath, not current working directory:**
```go
func (t *CompileTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
    path := "."
    if p, ok := input["path"].(string); ok && p != "" {
        path = p
    }

    // ❌ WRONG: Uses Wilson's directory
    // absPath, err := filepath.Abs(path)

    // ✅ RIGHT: Use explicit target directory
    // absPath should come from TaskContext.ProjectPath + relative path
}
```

---

## Issue 5: Target Directory Not Communicated to LSP

### Problem
When LSP starts for a code task in `~/wilsontestdir`, it needs to know:
- Root: `~/wilsontestdir`
- Not: Wilson's project root

### Current Behavior (Likely)
```go
// LSP always starts with Wilson's directory
lspClient.Start(ctx)
lspClient.Initialize(ctx, "file:///path/to/wilson/go")
```

### Required Behavior
```go
// LSP starts with target project directory from TaskContext
targetDir := taskCtx.ProjectPath // ~/wilsontestdir
lspClient.Start(ctx)
lspClient.Initialize(ctx, "file://" + targetDir)
```

### Check
1. `lsp/manager.go` - How clients are created and started
2. `agent/agents/code_agent.go` - Where LSP is used
3. Does code agent pass target directory to LSP?

---

## Issue 6: Path Expansion Not Happening

### Evidence
User requested: `~/wilsontestdir`
System interpreted as: Unclear, but files ended up in `~`

### Problem
`~` (tilde) must be expanded to actual home directory path.

### Check
1. Search codebase for: `os.UserHomeDir()`, `filepath.ExpandEnv`
2. Where user input is parsed (orchestration tool?)
3. Does TaskContext.ProjectPath contain raw `~` or expanded path?

### Fix Required
**Expand tilde in all path inputs:**
```go
func expandPath(path string) string {
    if strings.HasPrefix(path, "~/") {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[2:])
    }
    return path
}
```

---

## Issue 7: Iterative Fix Loop Uses Wrong Context

### Evidence
```
[AgentExecutor] Attempting iterative fix (attempt 1/3)
[AgentExecutor] Attempting iterative fix (attempt 2/3)
[AgentExecutor] Attempting iterative fix (attempt 3/3)
```

All 3 attempts failed, then created a fix task that also failed.

### Problem
**Fix attempts don't have correct context.** Each iteration:
1. Generates code (where? in what directory?)
2. Compiles (where? Wilson's directory?)
3. Fails because paths are confused

### Check
1. `agent/base/executor.go` - Iterative fix loop
2. Does it pass correct ProjectPath to each iteration?
3. Are file operations using absolute paths?

---

## Issue 8: Task Context Not Propagating

### Evidence
Multiple operations reference wrong paths:
- Code generation → `~/main.go` (wrong)
- Compilation → `wilson/go/main.go` (wrong)
- LSP diagnostics → `wilson/go/tests/*` (wrong)

### Problem
**ProjectPath in TaskContext is either:**
1. Not set correctly (`~/wilsontestdir`)
2. Not passed to tools
3. Overridden by `os.Getwd()` somewhere

### Check
1. `agent/orchestration/manager.go` - Where TaskContext is created
2. `agent/base/task_context.go` - What is ProjectPath?
3. Trace from user input → orchestration → code agent → tools

### Fix Required
**Every tool must respect TaskContext.ProjectPath:**
```go
// In every tool execution:
targetDir := getProjectPathFromContext() // ~/wilsontestdir
absPath := filepath.Join(targetDir, relativePath)
// NOT: absPath := filepath.Abs(relativePath) // Uses cwd!
```

---

## Priority Order for Fixes

### P0 - Blocks ALL code generation:
1. **Issue 2**: Files created in wrong location
2. **Issue 8**: TaskContext.ProjectPath not propagating
3. **Issue 6**: Path expansion (~ not resolved)

### P1 - Breaks compilation/validation:
4. **Issue 4**: Compile tool uses wrong directory
5. **Issue 1**: LSP initialized with wrong rootURI
6. **Issue 7**: Iterative fix uses wrong context

### P2 - Data corruption:
7. **Issue 3**: Error messages in generated code (CRITICAL but rare)

---

## Investigation Checklist

**For each issue, check these files:**

1. **Path handling:**
   - [ ] `agent/orchestration/manager.go` - TaskContext creation
   - [ ] `agent/base/task_context.go` - ProjectPath field
   - [ ] `agent/agents/code_agent.go` - How paths are used
   - [ ] `capabilities/orchestration/orchestrate_code_task.go` - Input parsing

2. **LSP initialization:**
   - [ ] `lsp/manager.go` - Client creation
   - [ ] `lsp/client.go` - Initialize() method
   - [ ] Code agent - Where LSP is requested

3. **Tool path resolution:**
   - [ ] `capabilities/code_intelligence/build/compile.go` - Line 84 (absPath)
   - [ ] `core/registry/executor.go` - How tools receive context
   - [ ] All file operation tools

4. **Code extraction:**
   - [ ] `agent/agents/code_agent.go` - Response parsing
   - [ ] LLM streaming - Are errors mixed with responses?

---

## Root Cause Summary

**The fundamental issue: WorkingDirectory vs ProjectPath confusion**

- **Wilson's working directory**: `/Users/.../wilson/go` (where Wilson runs)
- **Target project directory**: `~/wilsontestdir` (where user wants files)

**Current behavior**: Tools default to Wilson's working directory (cwd)
**Required behavior**: Tools must use TaskContext.ProjectPath

**This affects:**
- File creation (wrong location)
- Compilation (wrong files)
- LSP (wrong workspace)
- Path resolution (wrong base)

**The fix is architectural**: Every tool must be **context-aware**, not cwd-aware.
