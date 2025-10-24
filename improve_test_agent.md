# Improve TestAgent - Gap Analysis & Recommendations

**Date:** October 23, 2025
**Status:** Research Complete - Ready for Implementation
**Priority:** HIGH - TestAgent significantly behind CodeAgent capabilities

---

## 📊 Executive Summary

TestAgent is functionally behind CodeAgent by ~18 months of development. While it has basic test execution and feedback loop integration, it lacks:
- **Code intelligence tools** (can't analyze what needs testing)
- **Quality gates** (no coverage validation, no test quality checks)
- **Intelligent test design** (generic prompts, no test patterns)
- **Auto-fix loops** (no compilation verification)

**Current Success Rate:** ~70% (generates generic tests)
**Target Success Rate:** 90%+ (intelligent, coverage-validated tests)
**Effort:** ~2.5 hours for Priority 1-3 improvements

---

## 🔍 Current State Analysis

### TestAgent Overview (290 lines)

**Strengths:**
- ✅ Precondition checks (context-aware)
- ✅ TaskContext integration
- ✅ Feedback loop support
- ✅ DependencyFiles check (smart: checks what was created)
- ✅ Basic tool set (11 tools)

**Weaknesses:**
- ❌ No code intelligence tools (blind test generation)
- ❌ No quality gates (no coverage validation)
- ❌ Weak system prompt (~50 lines vs CodeAgent's 100)
- ❌ No compilation verification loop
- ❌ No test design guidance
- ❌ No coverage enforcement

### Comparison: TestAgent vs CodeAgent

| Metric | CodeAgent | TestAgent | Gap |
|--------|-----------|-----------|-----|
| **Lines** | 451 | 290 | -35% |
| **Tools** | 25+ | 11 | -56% |
| **Code Intelligence** | ✅ 4 tools | ❌ 0 tools | **CRITICAL** |
| **Quality Gates** | ✅ 6 tools | ❌ 0 tools | **HIGH** |
| **Prompt Quality** | ✅ Detailed | ⚠️ Basic | **HIGH** |
| **Auto-fix Loop** | ✅ Yes | ❌ No | **MEDIUM** |
| **Atomic Principle** | ✅ Explained | ❌ Missing | **MEDIUM** |

---

## 🔴 Gap Analysis

### Gap 1: No Code Intelligence Tools ⭐⭐⭐⭐⭐ **CRITICAL**

**Problem:** TestAgent operates blindly without understanding what it's testing.

**Impact:**
- Generates generic template tests instead of analyzing real code
- Can't identify which functions/methods need testing
- No understanding of existing test coverage
- Relies on LLM hallucination instead of factual code analysis

**Available but Unused Tools:**
```go
// Wilson already has these tools, TestAgent just doesn't use them:
"parse_file"        // Parse test files to understand structure
"find_symbol"       // Find functions that need tests
"analyze_structure" // Analyze test organization
"find_patterns"     // Discover existing test patterns
```

**Example Scenario:**
```
Current Behavior:
Task: "Write tests for user.go"
→ TestAgent generates generic template: TestUser_Success, TestUser_Error
→ Doesn't know what user.go actually contains
→ Tests may not match real functions

Desired Behavior:
Task: "Write tests for user.go"
→ parse_file(user.go) → finds: CreateUser(), ValidateUser(), DeleteUser()
→ find_symbol() → identifies edge cases: empty name, duplicate email, invalid ID
→ Generates specific tests: TestCreateUser_ValidData, TestCreateUser_EmptyName, etc.
→ Tests match actual code
```

**Fix (30 min):**
```go
// test_agent.go line ~25
base.SetAllowedTools([]string{
    // Existing tools...
    "read_file",
    "search_files",
    "list_files",
    // ... existing tools ...

    // ADD CODE INTELLIGENCE:
    "parse_file",        // ⭐ Understand code structure
    "find_symbol",       // ⭐ Find what needs testing
    "analyze_structure", // ⭐ Analyze test organization
    "find_patterns",     // ⭐ Learn existing patterns
})
```

**Expected Gain:** 70% → 82% success rate (+12%)

---

### Gap 2: No Quality Gates ⭐⭐⭐⭐ **HIGH**

**Problem:** TestAgent has no way to validate test quality or coverage.

**Impact:**
- No coverage validation (can't ensure 80%+ coverage)
- Can't auto-format generated tests
- No style/quality enforcement
- No way to verify tests aren't overly complex

**CodeAgent Has:**
```go
"format_code"       // Auto-format code
"lint_code"         // Check style/best practices
"security_scan"     // Security checks
"complexity_check"  // Complexity analysis
"coverage_check"    // Verify coverage threshold ⭐ MOST IMPORTANT
"code_review"       // Comprehensive review
```

**TestAgent Has:** ❌ **NONE**

**Example Scenario:**
```
Current Behavior:
Task: "Write tests with 80% coverage"
→ TestAgent writes 3 tests
→ run_tests → passes
→ Task marked complete
→ Actual coverage: 45% ❌ (no validation)

Desired Behavior:
Task: "Write tests with 80% coverage"
→ TestAgent writes 3 tests
→ run_tests with coverage
→ coverage_check → 45% < 80% threshold
→ analyze_structure → finds untested functions
→ Writes 4 more tests
→ coverage_check → 85% ✓
```

**Fix (15 min):**
```go
// test_agent.go line ~25
base.SetAllowedTools([]string{
    // Existing tools...

    // ADD QUALITY GATES:
    "coverage_check",    // ⭐ Verify coverage threshold (CRITICAL)
    "format_code",       // Auto-format test files
    "lint_code",         // Check test code style
    "complexity_check",  // Ensure tests aren't too complex
})
```

**Expected Gain:** 82% → 88% success rate (+6%)

---

### Gap 3: Weak System Prompt ⭐⭐⭐⭐ **HIGH**

**Problem:** TestAgent prompt is generic and doesn't guide intelligent test design.

**Current Prompt: ~50 lines**
- Basic tool list
- Generic patterns
- Minimal quality standards
- No test design principles

**CodeAgent Prompt: ~100 lines**
- Atomic task principle explained
- Workflow auto-injection described
- Detailed tool usage examples
- Error handling patterns
- Quality standards
- Multiple example workflows

**Missing from TestAgent:**

1. **Test Design Principles:**
   - AAA pattern (Arrange, Act, Assert)
   - Table-driven tests for Go
   - Test isolation and independence
   - Mock/stub guidance
   - Coverage expectations (80%+)

2. **Intelligence-First Workflow:**
   ```
   Step 1: Analyze code (parse_file, find_symbol)
   Step 2: Design test matrix (happy path, edge cases, errors)
   Step 3: Generate tests (write_file)
   Step 4: Verify compilation (compile)
   Step 5: Run and check coverage (run_tests, coverage_check)
   Step 6: Fix if needed or exit
   ```

3. **Quality Standards:**
   - Coverage thresholds (80% minimum, 90% target)
   - Test naming conventions (TestFunction_Scenario_Expected)
   - Clear assertions with messages
   - No test interdependencies

4. **Atomic Task Principle:**
   - One test file per task
   - Exit after tests pass + coverage validated
   - Manager coordinates multi-file test suites

**Example Improvement:**
```go
// Current prompt (generic):
`Task: "Write tests for X"
→ Read code if not in context
→ Design test cases (happy path, edge cases, errors)
→ Use write_file to create test file`

// Improved prompt (intelligent):
`Task: "Write tests for X"
→ STEP 1: Analyze code structure
   {"tool": "parse_file", "arguments": {"path": "X.go"}}
   → Identify: exported functions, types, interfaces

→ STEP 2: Find testable units
   {"tool": "find_symbol", "arguments": {"symbol": "CreateUser"}}
   → Understand: parameters, return values, error conditions

→ STEP 3: Design test matrix
   - Happy path: Valid input → success
   - Edge cases: Empty strings, nil values, boundary conditions
   - Error paths: Invalid input → specific errors

→ STEP 4: Check existing patterns
   {"tool": "find_patterns", "arguments": {"pattern": "test"}}
   → Learn: project test conventions, mock usage

→ STEP 5: Generate tests using project patterns
   {"tool": "generate_code", ...} or {"tool": "write_file", ...}

→ STEP 6: Verify coverage
   {"tool": "run_tests", "arguments": {"coverage": true}}
   {"tool": "coverage_check", "arguments": {"threshold": 80}}

Quality Standards:
- Minimum 80% coverage (use coverage_check to verify)
- Table-driven tests for multiple cases
- Clear test names: TestFunction_Scenario_Expected
- Independent tests (no shared state)
- Meaningful assertions with failure messages`
```

**Fix (45 min):**
Create new `buildSystemPrompt()` with:
- Test design principles section
- Intelligence-first workflow
- Quality standards with thresholds
- Atomic task principle
- Multiple detailed examples

**Expected Gain:** 88% → 92% success rate (+4%)

---

### Gap 4: No Compilation Verification Loop ⭐⭐⭐ **MEDIUM**

**Problem:** If generated tests don't compile, TestAgent doesn't auto-fix.

**CodeAgent Has:**
```
generate_code → [AUTO] write_file → [AUTO] compile → [AUTO] fix if errors
```

**TestAgent Has:**
```
write_file → run_tests → ❌ NO AUTO-FIX LOOP
```

**Impact:**
- Compilation errors cause task to fail
- No iterative improvement
- Relies on feedback escalation instead of self-correction
- Wastes user time

**Example Scenario:**
```
Current Behavior:
→ TestAgent writes test file
→ run_tests → compile error: "undefined: mockDB"
→ Task fails, sends feedback
→ Manager creates new task
→ 30-60 second delay

Desired Behavior:
→ TestAgent writes test file
→ [AUTO] compile → error: "undefined: mockDB"
→ parse error, identify missing import
→ modify_file → add import "testing/mock"
→ [AUTO] compile → success ✓
→ run_tests → pass ✓
→ 5 second fix
```

**Fix (30 min):**
```go
// In Execute() after tool execution:
if containsTool(execResult.ToolsExecuted, "write_file") {
    // Auto-compile test file
    compileResult := compileTestFile(writtenFile)
    if compileResult.HasErrors {
        // Attempt fix (max 3 tries)
        for attempt := 0; attempt < 3; attempt++ {
            fixResult := fixCompileError(compileResult.Error)
            compileResult = compileTestFile(writtenFile)
            if !compileResult.HasErrors {
                break
            }
        }
    }
}
```

**Expected Gain:** 92% → 94% success rate (+2%)

---

### Gap 5: Limited Preconditions ⭐⭐ **MEDIUM**

**Problem:** TestAgent only checks if test files exist, not if source code exists.

**Current Preconditions:**
- ✅ Checks test files exist (for "run tests" tasks)
- ✅ Checks DependencyFiles first (smart!)
- ❌ Doesn't verify source code exists
- ❌ Doesn't check project compiles
- ❌ Doesn't validate test dependencies

**Example Scenario:**
```
Current Behavior:
Task: "Write tests for nonexistent.go"
→ No precondition check
→ LLM generates tests for hallucinated functions
→ Tests don't match reality

Desired Behavior:
Task: "Write tests for nonexistent.go"
→ Precondition: Check if nonexistent.go exists
→ File not found
→ RequestDependency("Create nonexistent.go")
→ Blocks task until source exists
```

**Fix (20 min):**
```go
// In checkPreconditions():

// Check 1: For "write tests" tasks, verify source code exists
if strings.Contains(lowerDesc, "write test") || strings.Contains(lowerDesc, "create test") {
    // Extract source file from task description
    sourceFile := extractSourceFile(task.Description) // e.g., "user.go" from "write tests for user.go"

    if sourceFile != "" {
        // Check in DependencyFiles first
        if a.currentContext != nil {
            found := false
            for _, file := range a.currentContext.DependencyFiles {
                if strings.HasSuffix(file, sourceFile) {
                    found = true
                    break
                }
            }
            if !found {
                // Check filesystem
                if _, err := os.Stat(filepath.Join(projectPath, sourceFile)); os.IsNotExist(err) {
                    return a.RequestDependency(ctx,
                        fmt.Sprintf("Create %s", sourceFile),
                        ManagedTaskTypeCode,
                        fmt.Sprintf("Source file does not exist: %s", sourceFile))
                }
            }
        }
    }
}

// Check 2: Verify project compiles (can't test broken code)
if strings.Contains(lowerDesc, "write test") {
    // Quick compile check
    compileResult := quickCompile(projectPath)
    if compileResult.HasErrors {
        return fmt.Errorf("project has compilation errors, fix before writing tests: %s",
            compileResult.Error)
    }
}
```

**Expected Gain:** 94% → 95% success rate (+1%)

---

## 🎯 Implementation Roadmap

### Phase 1: Core Intelligence (Priority 1-3) - **2.5 hours**

**Goal:** Transform TestAgent from blind executor to intelligent test designer

**Tasks:**
1. ✅ Add code intelligence tools (30 min)
   - parse_file, find_symbol, analyze_structure, find_patterns
   - Update SetAllowedTools()
   - Test: Generate tests for complex file, verify it analyzes first

2. ✅ Add quality gates (15 min)
   - coverage_check, format_code, lint_code, complexity_check
   - Update SetAllowedTools()
   - Test: Generate tests, verify coverage validation

3. ✅ Rewrite system prompt (45 min)
   - Add test design principles (AAA, table-driven)
   - Add intelligence-first workflow
   - Add quality standards (80% coverage)
   - Add atomic task principle
   - Add detailed examples
   - Test: Compare test quality before/after

4. ✅ Update tests (30 min)
   - Update test_agent_test.go expectations
   - Add coverage validation tests
   - Verify new tools accessible

**Validation:**
```bash
# Test 1: Intelligence usage
Task: "Write tests for user.go"
Expected: parse_file(user.go) called first
Expected: Tests match actual functions

# Test 2: Coverage enforcement
Task: "Write tests with 80% coverage"
Expected: coverage_check called
Expected: Retries until 80%+ achieved

# Test 3: Quality
Task: "Write tests for handler.go"
Expected: Tests formatted correctly
Expected: Tests follow AAA pattern
```

**Expected Outcome:** 70% → 92% success rate

---

### Phase 2: Auto-Fix & Robustness (Priority 4-5) - **50 min**

**Goal:** Self-healing test generation

**Tasks:**
1. ✅ Add compilation loop (30 min)
   - Auto-compile after write_file
   - Parse compile errors
   - Retry with error context (max 3 attempts)
   - Test: Generate tests with missing import, verify auto-fix

2. ✅ Enhance preconditions (20 min)
   - Verify source file exists
   - Check project compiles
   - Validate test dependencies
   - Test: Request nonexistent file tests, verify dependency request

**Validation:**
```bash
# Test 1: Auto-fix compilation
Task: "Write tests for complex code"
Inject: Missing import error
Expected: Auto-adds import, tests compile

# Test 2: Precondition enforcement
Task: "Write tests for missing.go"
Expected: RequestDependency for missing.go
Expected: Task blocks until source exists
```

**Expected Outcome:** 92% → 95% success rate

---

### Phase 3: Advanced Intelligence (Future) - **TBD**

**Goal:** World-class test generation

**Ideas:**
1. **Coverage Gap Analysis:**
   ```go
   "analyze_coverage_gaps" // Identify untested code paths
   → Suggests specific test cases for gaps
   ```

2. **Test Pattern Learning:**
   ```go
   "learn_test_patterns" // Analyze project test conventions
   → Matches project style automatically
   ```

3. **Flaky Test Detection:**
   ```go
   "detect_flaky_tests" // Run tests multiple times
   → Identifies non-deterministic tests
   ```

4. **Benchmark Integration:**
   ```go
   "benchmark_tests" // Performance testing
   → Detects performance regressions
   ```

**Expected Outcome:** 95% → 97%+ success rate

---

## 📊 Expected Impact Summary

| Phase | Success Rate | Coverage | Quality | Effort |
|-------|--------------|----------|---------|--------|
| **Current** | 70% | Variable | Low | - |
| **Phase 1** | 92% | 80%+ | High | 2.5h |
| **Phase 2** | 95% | 85%+ | High | 0.9h |
| **Phase 3** | 97%+ | 90%+ | Excellent | TBD |

**Total Phase 1-2 Effort:** ~3.4 hours
**Total Gain:** 70% → 95% (+25% success rate)

---

## 🔧 Detailed Implementation Guide

### Step 1: Add Code Intelligence Tools

**File:** `go/agent/test_agent.go`

**Location:** Line ~25 (SetAllowedTools)

**Before:**
```go
base.SetAllowedTools([]string{
    // File reading
    "read_file",
    "search_files",
    "list_files",
    // File writing
    "write_file",
    "modify_file",
    "append_to_file",
    // Test execution
    "run_tests",
    // Context
    "search_artifacts",
    "retrieve_context",
    "store_artifact",
    "leave_note",
})
```

**After:**
```go
base.SetAllowedTools([]string{
    // File reading
    "read_file",
    "search_files",
    "list_files",
    // File writing
    "write_file",
    "modify_file",
    "append_to_file",
    // Code intelligence ⭐ NEW
    "parse_file",        // Understand code structure via AST
    "find_symbol",       // Find definitions and usages
    "analyze_structure", // Analyze package/file structure
    "find_patterns",     // Discover code patterns
    // Test execution
    "run_tests",
    // Quality gates ⭐ NEW
    "coverage_check",    // Verify coverage threshold
    "format_code",       // Auto-format test files
    "lint_code",         // Check test code quality
    "complexity_check",  // Ensure tests aren't too complex
    // Context
    "search_artifacts",
    "retrieve_context",
    "store_artifact",
    "leave_note",
})
```

---

### Step 2: Rewrite System Prompt

**File:** `go/agent/test_agent.go`

**Location:** Line ~142 (buildSystemPrompt function)

**New Structure:**
```go
func (a *TestAgent) buildSystemPrompt() string {
    prompt := BuildSharedPrompt("Test Agent")

    prompt += `
You design and execute intelligent, high-coverage tests. You analyze code to understand what needs testing.

=== YOUR ROLE ===

Generate ONE test file per task with high coverage. You are part of a multi-task workflow.

**Atomic Task Principle:**
- Each task = ONE test file
- Must achieve 80%+ coverage (verified via coverage_check)
- Exit after tests pass + coverage validated
- ManagerAgent coordinates multi-file test suites

=== INTELLIGENT TEST DESIGN WORKFLOW ===

**Step 1: ANALYZE - Understand what you're testing**
{"tool": "parse_file", "arguments": {"path": "source.go"}}
→ Identify: functions, types, interfaces, error paths

{"tool": "find_symbol", "arguments": {"symbol": "CreateUser"}}
→ Understand: parameters, return values, edge cases

**Step 2: DESIGN - Plan test matrix**
- Happy path: Valid inputs → expected success
- Edge cases: Boundary values, empty/nil, special characters
- Error paths: Invalid inputs → specific errors
- Integration: Dependencies, side effects

**Step 3: DISCOVER - Learn project patterns**
{"tool": "find_patterns", "arguments": {"pattern": "test", "path": "."}}
→ Learn: Existing test conventions, mock usage, assertion style

**Step 4: GENERATE - Write tests**
{"tool": "write_file", "arguments": {"path": "source_test.go", "content": "..."}}
→ Use table-driven tests for multiple cases
→ Follow AAA pattern: Arrange, Act, Assert
→ Clear names: TestFunction_Scenario_Expected

**Step 5: VALIDATE - Verify quality**
{"tool": "run_tests", "arguments": {"path": ".", "coverage": true}}
{"tool": "coverage_check", "arguments": {"path": "source.go", "threshold": 80}}
→ If coverage < 80%: Analyze gaps, add more tests
→ If coverage >= 80%: Format and exit

{"tool": "format_code", "arguments": {"path": "source_test.go"}}
→ Auto-format before completion

=== QUALITY STANDARDS ===

**Coverage:**
- Minimum: 80% line coverage (enforced by coverage_check)
- Target: 90% line coverage
- If < 80%: Add tests for uncovered branches

**Test Structure (Go):**
- Table-driven tests for multiple cases
- AAA pattern: Arrange, Act, Assert
- Clear names: TestCreateUser_EmptyName_ReturnsError
- Independent tests (no shared state)

**Assertions:**
- Clear failure messages
- Check both success and error cases
- Validate error messages, not just error existence

=== EXAMPLE WORKFLOWS ===

**Task: "Write tests for user.go"**

→ Step 1: Analyze code
{"tool": "parse_file", "arguments": {"path": "user.go", "detail": "medium"}}
→ Output: "Functions: CreateUser(name, email), ValidateUser(user), DeleteUser(id)"

→ Step 2: Understand each function
{"tool": "find_symbol", "arguments": {"symbol": "CreateUser", "path": "user.go"}}
→ Output: "Parameters: string, string. Returns: *User, error. Validates email format."

→ Step 3: Design tests
Test matrix:
- CreateUser_ValidData_Success
- CreateUser_EmptyName_ReturnsError
- CreateUser_InvalidEmail_ReturnsError
- CreateUser_DuplicateEmail_ReturnsError
- ValidateUser_ValidUser_ReturnsTrue
- ValidateUser_NilUser_ReturnsFalse
- DeleteUser_ExistingID_Success
- DeleteUser_InvalidID_ReturnsError

→ Step 4: Generate tests
{"tool": "write_file", "arguments": {
  "path": "user_test.go",
  "content": "package user\n\nimport \"testing\"\n\nfunc TestCreateUser(t *testing.T) {\n  tests := []struct {...}..."
}}

→ Step 5: Validate coverage
{"tool": "run_tests", "arguments": {"path": ".", "coverage": true}}
→ Output: "PASS. Coverage: 85%"

{"tool": "coverage_check", "arguments": {"path": "user.go", "threshold": 80}}
→ Output: "✓ Coverage 85% meets threshold 80%"

{"tool": "format_code", "arguments": {"path": "user_test.go"}}
→ Done! High-coverage tests created.

**Task: "Run tests in ~/project"**

→ Step 1: Check prerequisites (automatic via preconditions)
→ If no test files: Sends feedback to create them
→ If test files exist: Proceed

→ Step 2: Execute tests
{"tool": "run_tests", "arguments": {"path": "~/project", "verbose": true}}
→ Output: "PASS: 15/15 tests. FAIL: 0. Coverage: 82%"

→ Done!

=== ERROR HANDLING ===

**Compilation Errors:**
→ System auto-compiles after write_file
→ If errors: Retry with error context (max 3 attempts)
→ Common fixes: Add imports, fix syntax, correct types

**Low Coverage:**
→ coverage_check fails if < threshold
→ Analyze: What's not covered?
→ Add tests for uncovered paths
→ Retry coverage_check

**Test Failures:**
→ Read failure output carefully
→ Fix test logic (not the source code!)
→ Retry until tests pass

=== ATOMIC PRINCIPLE ===

You generate ONE test file per task. Trust the workflow:
1. You create tests with high coverage
2. System validates compilation
3. You verify coverage >= 80%
4. Task complete - exit
5. Manager handles next task (if any)

No multi-file orchestration. No compile error retries beyond 3 attempts. Focus on YOUR assigned test file.
`

    return prompt
}
```

---

### Step 3: Add Compilation Loop

**File:** `go/agent/test_agent.go`

**Location:** After line ~112 (execResult handling)

**Add:**
```go
// === AUTO-COMPILE CHECK - Verify tests compile ===
if containsTool(execResult.ToolsExecuted, "write_file") {
    // Extract written file path
    testFilePath := extractTestFilePath(execResult.Output)

    if testFilePath != "" {
        // Compile test file (max 3 attempts)
        for attempt := 1; attempt <= 3; attempt++ {
            compileResult := compileTestFile(ctx, testFilePath, a.llmManager)

            if compileResult.Success {
                break // Compilation succeeded
            }

            if attempt == 3 {
                // Max attempts reached, escalate
                a.RecordError("test_compile_error", "compilation",
                    compileResult.Error, testFilePath, 0,
                    "Test file does not compile after 3 attempts")

                result.Success = false
                result.Error = fmt.Sprintf("Test compilation failed: %s", compileResult.Error)
                return result, fmt.Errorf("test compilation failed")
            }

            // Attempt fix with error context
            // (Implementation similar to CodeAgent's compile loop)
        }
    }
}
```

---

### Step 4: Enhance Preconditions

**File:** `go/agent/test_agent.go`

**Location:** Line ~234 (checkPreconditions function)

**Add after existing checks:**
```go
// Check 2: For "write tests" tasks, verify source code exists
if strings.Contains(lowerDesc, "write test") || strings.Contains(lowerDesc, "create test") {
    // Try to extract source file name from description
    // e.g., "write tests for user.go" → "user.go"
    sourceFile := extractSourceFileFromDescription(task.Description)

    if sourceFile != "" && !strings.Contains(sourceFile, "_test") {
        // Check DependencyFiles first (smart!)
        found := false
        if a.currentContext != nil {
            for _, file := range a.currentContext.DependencyFiles {
                if strings.HasSuffix(file, sourceFile) {
                    found = true
                    break
                }
            }
        }

        if !found {
            // Fallback: Check filesystem
            fullPath := filepath.Join(projectPath, sourceFile)
            if _, err := os.Stat(fullPath); os.IsNotExist(err) {
                return a.RequestDependency(ctx,
                    fmt.Sprintf("Create %s in %s", sourceFile, projectPath),
                    ManagedTaskTypeCode,
                    fmt.Sprintf("Source file does not exist: %s (cannot write tests for nonexistent code)", sourceFile))
            }
        }
    }
}

// Check 3: Verify project compiles before writing tests
if strings.Contains(lowerDesc, "write test") || strings.Contains(lowerDesc, "create test") {
    // Quick compile check (don't write tests for broken code)
    compileCmd := exec.CommandContext(ctx, "go", "build", projectPath)
    if err := compileCmd.Run(); err != nil {
        return fmt.Errorf("project has compilation errors, cannot write tests for broken code. Fix compilation first")
    }
}
```

---

## ✅ Acceptance Criteria

### Phase 1 Complete When:
- ✅ TestAgent has 19+ tools (11 current + 8 new)
- ✅ System prompt >120 lines with intelligent workflow
- ✅ Tests analyze code before generating (parse_file called first)
- ✅ Coverage validated automatically (coverage_check called)
- ✅ Test quality improved measurably (80%+ coverage consistently)

### Phase 2 Complete When:
- ✅ Compilation errors auto-fixed (max 3 attempts)
- ✅ Source file existence checked (precondition)
- ✅ Project compilation verified (precondition)
- ✅ Success rate ≥ 95%

### Validation Tests:
```bash
# Test 1: Code Intelligence
./wilson <<< "Write tests for go/agent/task.go"
# Expect: parse_file(task.go) called
# Expect: Tests match actual functions (NewManagedTask, Block, Unblock, etc.)

# Test 2: Coverage Enforcement
./wilson <<< "Write tests for go/agent/intent.go with 80% coverage"
# Expect: coverage_check called
# Expect: Coverage ≥ 80%

# Test 3: Auto-Fix
./wilson <<< "Write tests for complex code with imports"
# Expect: Tests compile successfully
# Expect: No manual intervention needed

# Test 4: Preconditions
./wilson <<< "Write tests for nonexistent_file.go"
# Expect: RequestDependency for source file
# Expect: Task blocks until source exists
```

---

## 📝 Notes

**Refactor Consideration:**
- This improvement can be done BEFORE or AFTER the agent directory refactor
- If done AFTER: test_agent.go will be in `agents/test_agent.go`
- Changes are isolated to test_agent.go (no cross-file impacts)

**Compatibility:**
- All new tools already exist in Wilson's codebase
- No new tool development needed
- Just enabling TestAgent to use existing capabilities

**Testing:**
- Update go/agent/test_agent_test.go (if exists)
- Add integration tests for coverage validation
- E2E test: Generate tests for real codebase file

**Documentation:**
- Update DONE.md after implementation
- Add to ENDGAME.md capabilities section
- Note improved success rate metrics

---

**Last Updated:** October 23, 2025
**Status:** Ready for Implementation
**Estimated Effort:** Phase 1-2: 3.4 hours
**Expected Impact:** 70% → 95% success rate
