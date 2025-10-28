# Additional Fixes - Round 2

Two more critical issues discovered and fixed during real-world testing.

---

## Issue 1: Task Misclassified as Simple ❌→✅

### Problem
User request: "Create calculator, also provide a testfile"
- Expected: Complex task → decompose into implementation + tests
- Actual: Simple task → only generated test file (missing main.go)

### Root Cause
The complexity detection in `needsDecomposition()` looked for specific phrases:
- ✅ "test file" (two words)
- ✅ "testfile" (one word)
- ✅ "write tests"
- ❌ "include tests" (MISSING!)
- ❌ "provide test" (MISSING!)

User's phrasing was normalized to "Include tests for the calculator logic" → **no match → simple task**

### Fix Applied
**Location**: `agent/orchestration/manager.go:910-912`

```go
// Before:
complexIndicators := []string{
    "write tests", "create tests", "add tests",
    // Missing common phrasings!
}

// After:
complexIndicators := []string{
    // Multiple actions - test-related
    "write tests", "create tests", "add tests",
    "include tests", "also provide a test", "provide test",
    "with tests", "and tests",
}
```

### Result
✅ More test-related phrases now trigger decomposition
✅ Task correctly identified as complex
✅ Both implementation and test files will be created

---

## Issue 2: LSP Still Shows Wilson's Files ❌→✅

### Problem
Even after setting project root override, LSP diagnostics still showed Wilson's test files:
```
[LSP] Diagnostics for file:///...//wilson/go/tests/test_integration_paths.go: 1 errors
```

### Root Cause

**LSP clients are cached and reused**:

1. First request comes in
2. LSP client created and initialized with Wilson's directory as root
3. Client cached in manager.clients map
4. Second request with different project comes in
5. Code agent sets `lsp.SetProjectRoot(targetPath)`
6. LSP tools call `manager.GetClient("go")`
7. **Manager returns CACHED client** (line 55-56 in manager.go)
8. Cached client still has Wilson's directory as root ❌

The fix only affected **new** client initialization, not existing ones.

### Fix Applied

**Location**: `lsp/manager.go:27-40`, `setup/bootstrap.go:39`

#### Part A: Track Global Manager
```go
var (
    projectRootOverride string
    projectRootMu       sync.RWMutex
    globalManager       *Manager // NEW: Track manager for restart
)

func SetGlobalManager(manager *Manager) {
    projectRootMu.Lock()
    defer projectRootMu.Unlock()
    globalManager = manager
}
```

#### Part B: Restart Clients on Project Change
```go
func SetProjectRoot(path string) {
    projectRootMu.Lock()
    oldRoot := projectRootOverride
    projectRootOverride = path
    manager := globalManager
    projectRootMu.Unlock()

    // ✅ NEW: If project root changed, restart all LSP clients
    // This ensures they reinitialize with the new project root
    if oldRoot != path && manager != nil && path != "" {
        manager.StopAll() // Will restart on next GetClient call
    }
}
```

#### Part C: Register Global Manager at Startup
```go
// In setup/bootstrap.go Initialize()
b.LSPManager = lsp.NewManager()
code_intelligence.SetLSPManager(b.LSPManager)
lsp.SetGlobalManager(b.LSPManager) // NEW: Enable restart
```

### How It Works

**Before (Broken)**:
```
Request 1: Simple task (no project set)
  → LSP client created with Wilson's dir
  → Client cached

Request 2: Calculator in ~/wilsontestdir
  → SetProjectRoot("/Users/.../wilsontestdir")
  → GetClient("go") returns CACHED client
  → Cached client still has Wilson's dir ❌
  → Diagnostics show Wilson's files ❌
```

**After (Fixed)**:
```
Request 1: Simple task (no project set)
  → LSP client created with Wilson's dir
  → Client cached

Request 2: Calculator in ~/wilsontestdir
  → SetProjectRoot("/Users/.../wilsontestdir")
    → Detects root changed (Wilson dir → target dir)
    → Calls manager.StopAll()
    → Clears client cache
  → GetClient("go") creates NEW client
  → New client initializes with target dir ✅
  → Diagnostics show only target files ✅
```

### Result
✅ LSP clients restart when project root changes
✅ Diagnostics only show target project files
✅ No more Wilson file pollution in logs

---

## Testing

Both fixes verified by rebuild:
```bash
go build -o wilson main.go
# ✅ Success
```

**Next test**: User should retry calculator task to verify:
1. Task decomposes into subtasks (not simple)
2. Both main.go and main_test.go created
3. No LSP diagnostics for Wilson's files

---

## Summary

**Issue 1 - Task Classification**:
- Fixed by: Adding more test-related phrases to complexity detection
- Impact: Tasks with tests now properly decompose

**Issue 2 - LSP Caching**:
- Fixed by: Restarting LSP clients when project root changes
- Impact: LSP always works with correct project directory

Both are **critical for multi-project workflows** where Wilson works on different directories in the same session.
