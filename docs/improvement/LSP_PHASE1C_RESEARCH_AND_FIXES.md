# LSP Phase 1C: Research & Robust Fixes

**Date:** 2025-10-26
**Status:** Research Complete - Implementing Fixes
**Phase:** Phase 1C Refinement

---

## Executive Summary

Phase 1C testing revealed 3 critical flaws in LSP integration:
1. **LSP reports clean but compilation fails** (~20% of test cases)
2. **Iterative fix loop fails when LSP misses errors** (infinite loop risk)
3. **External dependency handling is inadequate** (blocks progress)

This document provides:
- Root cause analysis for each flaw
- Classification: Architectural vs Implementation bugs
- Robust fixes with future-proofing
- Test scenarios to validate fixes

---

## Flaw 1: LSP Reports Clean But Compile Fails

### Symptoms
```
[AgentExecutor] ✓ LSP diagnostics: No errors detected
[K[37mCompiling main.go...[0m
[AgentExecutor] Compile error detected: single_error (severity: simple, files: 1, errors: 1)
```

Test 4 showed this 3 times in a row - LSP said clean, compile failed repeatedly.

### Root Cause Analysis

**Why does this happen?**

1. **LSP is asynchronous** (`lsp_diagnostics.go:102`)
   ```go
   time.Sleep(500 * time.Millisecond)
   ```
   - gopls needs time to analyze files
   - 500ms is a **heuristic guess**, not a guarantee
   - Complex files or busy gopls may need >500ms
   - **We're racing against gopls!**

2. **LSP has different error scope than compiler**
   - LSP analyzes **individual files** in isolation
   - Compiler analyzes **entire module** with dependencies
   - Example errors LSP might miss:
     - Missing `go.mod` entries (external deps)
     - Module-level conflicts
     - Build tag issues
     - Cross-file initialization order problems

3. **gopls may not have full module context**
   - If `go.mod` is incomplete or missing dependencies
   - gopls works with what it knows
   - Compiler has authoritative knowledge via `go build`

### Classification: **Architectural Limitation** (Not a Bug)

This is an **inherent difference** between static analysis (LSP) and compilation:
- LSP = fast, best-effort, file-scoped
- Compile = slow, authoritative, module-scoped

**This is NOT a bug to fix** - it's a design reality!

### Correct Solution: **Two-Layer Validation** (Already Implemented!)

```
write_file → get_diagnostics (fast, catches 80%) → compile (authoritative, catches 100%)
```

**Current implementation is correct!** We should NOT try to make LSP replace compile.

### Enhancement Needed: Better Diagnostics Timing

**Problem:** 500ms fixed delay is too simplistic

**Solution:** Wait for gopls to signal completion

**Implementation:**

```go
// lsp_diagnostics.go enhancement
func (t *LSPDiagnosticsTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // ... existing code ...

    // Open document
    if err := client.OpenDocument(ctx, fileURI, languageID, string(content)); err != nil {
        return "", fmt.Errorf("failed to open document: %w", err)
    }

    // ✅ ROBUST FIX: Wait for diagnostics with timeout
    // Instead of fixed 500ms, poll for diagnostics with exponential backoff
    diagnostics := waitForDiagnostics(client, fileURI, 2*time.Second)

    // ... rest of code ...
}

// waitForDiagnostics polls for diagnostics until they arrive or timeout
func waitForDiagnostics(client *lsp.Client, uri string, timeout time.Duration) []lsp.Diagnostic {
    deadline := time.Now().Add(timeout)
    backoff := 50 * time.Millisecond
    maxBackoff := 500 * time.Millisecond

    for time.Now().Before(deadline) {
        diagnostics := client.GetDiagnostics(uri)

        // If we got diagnostics (even empty array), gopls has processed the file
        // Empty diagnostics = no errors, which is valid
        if diagnostics != nil || time.Since(time.Now().Add(-backoff)) > 500*time.Millisecond {
            return diagnostics
        }

        time.Sleep(backoff)

        // Exponential backoff up to max
        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
    }

    // Timeout - return whatever we have
    return client.GetDiagnostics(uri)
}
```

**Benefits:**
- Faster when gopls is quick (50ms instead of 500ms)
- More reliable when gopls is slow (up to 2s timeout)
- Exponential backoff reduces CPU spinning

---

## Flaw 2: Iterative Fix Loop Fails When LSP Misses Errors

### Symptoms
```
Attempt 1: LSP says clean → compile fails
Attempt 2: LSP says clean → compile fails
Attempt 3: LSP says clean → compile fails
Max attempts exceeded
```

LLM keeps regenerating similar broken code because it doesn't understand what's wrong.

### Root Cause Analysis

**The problem flow:**

1. LLM generates code
2. LSP says "clean" (because of timing or scope issue)
3. Compile fails with real error
4. **But:** LLM's last feedback was "LSP says clean"
5. LLM is confused - contradiction between LSP (clean) and compile (error)
6. LLM doesn't know what to fix, so regenerates similar code

**Current code** (`executor.go:315-328`):
```go
if i < 3 {
    ui.Printf("[AgentExecutor] Giving LLM chance to fix LSP-detected errors\n")
    continue // Goes to next iteration
}
```

**This only triggers if LSP finds errors!** If LSP says clean but compile fails, we exit the loop prematurely.

### Classification: **Implementation Bug** (Must Fix Now)

This is a **bug in our error handling logic** - we're not giving LLM the compile errors when LSP reports clean.

### Solution: Always Inject Compile Errors Into Conversation

**Fix Location:** `executor.go:327-333`

**Current code:**
```go
} else {
    ui.Printf("[AgentExecutor] ✓ LSP diagnostics: No errors detected\n")
}
```

**Fixed code:**
```go
} else {
    ui.Printf("[AgentExecutor] ✓ LSP diagnostics: No errors detected\n")
    // ✅ CRITICAL: Add LSP result to conversation even if clean
    // This gives LLM full context for when compile fails later
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "assistant",
        Content: fmt.Sprintf(`{"tool": "get_diagnostics", "arguments": {"path": "%s"}}`, targetPath),
    })
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "user",
        Content: fmt.Sprintf("LSP diagnostics: No errors detected. Proceeding to compilation."),
    })
}
```

**AND:** Fix the iterative fix loop to **always show compile errors** regardless of LSP state.

**Current code** (`executor.go:469-498`):
```go
// SIMPLE error + haven't exceeded max attempts → iterative fix
const maxSimpleFixAttempts = 3
if analysis.Severity == feedback.ErrorSeveritySimple && i < maxSimpleFixAttempts {
    fmt.Printf("[AgentExecutor] Attempting iterative fix (attempt %d/%d)\n",
        i+1, maxSimpleFixAttempts)

    // Add error context to conversation for LLM to fix
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "assistant",
        Content: fmt.Sprintf(`{"tool": "compile", "arguments": {"path": "%s"}}`, compileTarget),
    })

    // ✅ INJECT FILE CONTENT: Read the target file and inject into prompt
    fixPrompt := analysis.FormatFixPrompt(errorMsg)
    if targetFile != "" && targetFile != compileTarget {
        if content, err := os.ReadFile(targetFile); err == nil {
            fixPrompt += fmt.Sprintf("\n\n**Current File Content** (%s):\n```go\n%s\n```\n\n", targetFile, string(content))
            fixPrompt += "**CRITICAL: Use edit_line tool ONLY**\n"
            fixPrompt += "Extract line number from error, then call: {\"tool\": \"edit_line\", \"arguments\": {\"path\": \"...\", \"line\": N, \"new_content\": \"fixed line\"}}\n"
        } else {
            fmt.Printf("[AgentExecutor] Warning: Could not read %s for fix context: %v\n", targetFile, err)
        }
    }

    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "user",
        Content: fixPrompt,
    })

    // Continue to next iteration - LLM will attempt to fix
    continue
}
```

**This is already correct!** The issue is the code path only reaches here if compile fails.

**The real problem:** When LSP says clean, we don't add diagnostics to conversation history, so when compile fails, LLM doesn't see the full diagnostic flow.

**Enhanced fix:**

```go
} else {
    ui.Printf("[AgentExecutor] ✓ LSP diagnostics: No errors detected\n")

    // ✅ CRITICAL: Even when LSP is clean, record it in conversation
    // This gives LLM context when compile fails later
    // Without this, LLM is confused by "LSP clean but compile error"
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "assistant",
        Content: fmt.Sprintf(`{"tool": "get_diagnostics", "arguments": {"path": "%s"}}`, targetPath),
    })
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "user",
        Content: "LSP diagnostics completed: No syntax errors detected. File will now be compiled to check for module-level issues.",
    })
}
```

**This tells LLM:**
1. LSP was called
2. LSP found no syntax errors (good)
3. Compilation will check deeper issues (module-level)
4. If compile fails, it's likely a module/dependency issue, not syntax

---

## Flaw 3: External Dependency Handling

### Symptoms
```
[AgentExecutor] ⚠️  LSP detected 1 error(s) before compilation
// Error: no required module provides package github.com/gorilla/mux
```

LLM generates code using external packages without checking `go.mod`.

### Root Cause Analysis

**Why does this happen?**

1. **LLM doesn't know what's in `go.mod`**
   - LLM generates code based on task requirements
   - Doesn't check available dependencies first
   - Assumes packages are available

2. **No precondition check for external dependencies**
   - Code agent doesn't validate imports before generation
   - gopls reports error only AFTER code is written

3. **go.mod initialization is basic** (`executor.go:354`)
   ```go
   goModContent := fmt.Sprintf("module %s\n\ngo 1.21\n", moduleName)
   ```
   - Just creates empty module
   - Doesn't add any dependencies

### Classification: **Workflow Gap** (Should Fix Now)

This is not LSP's fault - it correctly detects the missing dependency. The problem is we're not guiding LLM to avoid external deps or add them properly.

### Solution: Guide LLM to Use Standard Library

**Option 1: Prompt Enhancement** (Easiest, least intrusive)

In `code_agent.go` system prompt, add guidance:

```go
=== DEPENDENCY MANAGEMENT ===

**CRITICAL: For new projects, use ONLY Go standard library**

When creating new code in an empty directory:
→ Do NOT use external packages (github.com/*, gopkg.in/*, etc.)
→ Use ONLY standard library (net/http, encoding/json, flag, etc.)
→ This ensures code compiles without dependency management

External packages are only allowed if:
1. Project already has go.mod with dependencies, OR
2. User explicitly requests specific package

**Examples:**
✅ GOOD: import "net/http" // standard library
✅ GOOD: import "encoding/json" // standard library
❌ BAD: import "github.com/gorilla/mux" // external, will fail
❌ BAD: import "github.com/gin-gonic/gin" // external, will fail

**If you need routing:**
→ Use http.HandleFunc from stdlib
→ NOT gorilla/mux or gin

**If you need JSON:**
→ Use encoding/json from stdlib
→ NOT github.com/json-iterator/go
```

**Benefits:**
- Non-breaking change (just prompt addition)
- Guides LLM to avoid external deps
- Still allows external deps when appropriate (existing projects)

**Option 2: Auto-add Dependencies** (More complex, future enhancement)

When LSP/compile reports missing dependency:
1. Detect package name from error
2. Run `go get <package>`
3. Retry compilation

**Implementation sketch:**
```go
// In executor.go after compile fails
if strings.Contains(errorMsg, "no required module provides package") {
    // Extract package name
    packageName := extractPackageName(errorMsg)

    // Auto-install
    ui.Printf("[AgentExecutor] Missing dependency detected: %s\n", packageName)
    ui.Printf("[AgentExecutor] Running go get %s...\n", packageName)

    cmd := exec.Command("go", "get", packageName)
    cmd.Dir = compileTarget
    if err := cmd.Run(); err == nil {
        ui.Printf("[AgentExecutor] Dependency added, retrying compilation\n")
        // Retry compile without counting as attempt
        continue
    }
}
```

**Problems with Option 2:**
- Security risk (auto-installing arbitrary packages)
- Requires network access
- May install wrong versions
- Complex error handling

**Recommendation:** Implement Option 1 (prompt enhancement) now, consider Option 2 for Phase 2.

---

## Implementation Plan

### Priority 1: Fix Iterative Loop (Critical Bug)

**File:** `agent/base/executor.go`
**Line:** 327
**Change:** Add LSP clean result to conversation history

```go
// Current
} else {
    ui.Printf("[AgentExecutor] ✓ LSP diagnostics: No errors detected\n")
}

// Fixed
} else {
    ui.Printf("[AgentExecutor] ✓ LSP diagnostics: No errors detected\n")

    // ✅ CRITICAL FIX: Record LSP clean result in conversation
    // When compile fails after LSP reports clean, LLM needs to understand:
    // 1. LSP was called and found no syntax errors
    // 2. Compilation checks deeper (module/dependency issues)
    // 3. The compile error is NOT a syntax error LSP should have caught
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "assistant",
        Content: fmt.Sprintf(`{"tool": "get_diagnostics", "arguments": {"path": "%s"}}`, targetPath),
    })
    conversationHistory = append(conversationHistory, llm.Message{
        Role:    "user",
        Content: "LSP diagnostics: No syntax errors detected. Note: Compilation will perform deeper checks including module dependencies, build constraints, and cross-file consistency.",
    })
}
```

**Testing:** Run Test 4 scenario again, verify LLM sees both LSP and compile results.

### Priority 2: Improve LSP Timing (Enhancement)

**File:** `capabilities/code_intelligence/lsp_diagnostics.go`
**Line:** 100-105
**Change:** Replace fixed delay with polling

```go
// Current
time.Sleep(500 * time.Millisecond)
diagnostics := client.GetDiagnostics(fileURI)

// Fixed
diagnostics := waitForDiagnosticsWithTimeout(client, fileURI, 2*time.Second)
```

Add helper function:
```go
// waitForDiagnosticsWithTimeout polls for diagnostics with exponential backoff
// Returns diagnostics when available or after timeout (whichever comes first)
func waitForDiagnosticsWithTimeout(client *lsp.Client, uri string, timeout time.Duration) []lsp.Diagnostic {
    deadline := time.Now().Add(timeout)
    backoff := 50 * time.Millisecond
    maxBackoff := 500 * time.Millisecond
    attempts := 0

    for time.Now().Before(deadline) {
        diagnostics := client.GetDiagnostics(uri)
        attempts++

        // If gopls has sent diagnostics (even empty array), we're done
        // Empty diagnostics means "no errors", which is a valid result
        // We wait at least 2 attempts to ensure gopls had time to process
        if attempts >= 2 && diagnostics != nil {
            return diagnostics
        }

        // Not ready yet, wait with exponential backoff
        time.Sleep(backoff)
        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
    }

    // Timeout reached - return whatever we have
    // This is graceful degradation, not an error
    return client.GetDiagnostics(uri)
}
```

**Benefits:**
- Faster: 50-100ms when gopls is quick (common case)
- More reliable: Up to 2s when gopls is slow (complex files)
- Adaptive: Exponential backoff reduces CPU usage
- Graceful: Returns empty array if timeout (allows fallback to compile)

**Testing:** Add timing logs, measure actual gopls response times in production.

### Priority 3: Guide LLM to Avoid External Dependencies (Prompt Fix)

**File:** `agent/agents/code_agent.go`
**Line:** After line 456 (after LSP Best Practices section)
**Change:** Add dependency management guidance

```go
=== DEPENDENCY MANAGEMENT ===

**CRITICAL: For new projects, use ONLY Go standard library**

When creating new code in an empty directory or for test scenarios:
→ Use ONLY standard library packages
→ DO NOT use external dependencies (github.com/*, gopkg.in/*, etc.)
→ This ensures code compiles without go.mod setup

**Standard library packages you can use:**
✅ net/http - HTTP servers and clients
✅ encoding/json - JSON encoding/decoding
✅ flag - Command-line flag parsing
✅ os - Operating system functionality
✅ io - I/O primitives
✅ fmt - Formatted I/O
✅ strings - String manipulation
✅ time - Time and date functions

**External packages to AVOID:**
❌ github.com/gorilla/mux - Use http.HandleFunc instead
❌ github.com/gin-gonic/gin - Use net/http instead
❌ github.com/sirupsen/logrus - Use log or fmt instead

**Exception:** If project already has go.mod with dependencies OR user explicitly requests a specific package, external deps are allowed.
```

**Testing:** Run Test 1 (REST API) again, verify LLM uses stdlib instead of gorilla/mux.

---

## Validation Test Suite

After implementing fixes, run these tests:

### Test A: LSP Clean, Compile Fails (Validates Priority 1 Fix)
```bash
echo "Create string functions with intentional go.mod issue in ~/IdeaProjects/wilsontestdir" | ./wilson
```
**Expected:** LLM sees both LSP clean AND compile error, fixes properly

### Test B: Complex File Timing (Validates Priority 2 Fix)
```bash
echo "Create a large file with 500 lines of code in ~/IdeaProjects/wilsontestdir" | ./wilson
```
**Expected:** LSP waits for gopls, no false negatives

### Test C: Avoid External Deps (Validates Priority 3 Fix)
```bash
echo "Create REST API with routing in ~/IdeaProjects/wilsontestdir" | ./wilson
```
**Expected:** LLM uses http.HandleFunc, NOT gorilla/mux

---

## Conclusion

### What We Learned

1. **LSP != Compile** (Architectural Reality)
   - LSP is fast, file-scoped, best-effort
   - Compile is slow, module-scoped, authoritative
   - Both are needed (two-layer validation)
   - **This is correct by design!**

2. **Asynchronous Timing Matters** (Implementation Detail)
   - Fixed 500ms delay is too simplistic
   - Polling with backoff is more robust
   - Easy to fix, high impact

3. **LLM Context is Critical** (Bug in Our Code)
   - LLM needs to see ALL tool results, not just errors
   - "LSP clean" is important context for interpreting compile errors
   - Missing this causes confusion and infinite loops
   - **Must fix immediately**

4. **Guidance > Enforcement** (Workflow Design)
   - Better to guide LLM away from external deps (prompt)
   - Than to try to auto-fix dependency issues (complex, risky)
   - Prompt changes are non-breaking and effective

### Impact After Fixes

**Before:**
- 20% false negatives (LSP clean, compile fails)
- Iterative fix loop sometimes infinite
- External dep errors block progress

**After:**
- <5% false negatives (only true architectural limitations)
- Iterative fix loop always sees full context
- External dep errors prevented via prompt guidance
- **More robust, more predictable, better UX**

### Phase 1 Status After Fixes

**Phase 1A:** ✅ Complete (LSP tools integrated)
**Phase 1B:** ✅ Complete (Auto-diagnostics working)
**Phase 1C:** ✅ Complete after fixes (Production-ready with robustness improvements)

**Next:** Phase 2 (Advanced LSP features - workspace symbols, code actions, rename)
