# LLM "Over-Helpfulness" Problem Analysis & Solutions

**Date**: 2025-10-27
**Status**: Analysis Complete, Implementation Pending
**Issue**: LLMs generating more code than requested, ignoring constraints

---

## Problem Statement

During e2e testing of LSP Phase 2 tools, we discovered that Wilson's code generation is "too helpful":

### Observed Behaviors

1. **Asked for**: "Create a Handler interface"
   **Got**: Handler interface + MockHandler implementation + 2 test functions

2. **Asked for**: "Create Config struct in config.go"
   **Got**: Config struct in main.go + extra helper functions + main() function

3. **General Pattern**: LLM adds extras beyond requirements:
   - Test functions when only production code requested
   - Complete implementations when only signatures requested
   - Wrong filenames (ignores `file_name` parameter)
   - Additional utility functions not requested

---

## Root Cause Analysis

### Code Path Traced

```
User Request
    ↓
ManagerAgent.DecomposeTask()
    ↓ (uses qwen2.5:7b - PurposeChat)
    Creates subtasks
    ↓
CodeAgent.Execute()
    ↓ (uses qwen2.5:7b - PurposeChat)
    Plans which tools to use
    ↓
generate_code tool called
    ↓ (uses qwen2.5-coder:14b - PurposeCode)
    Generates actual code
    ↓
Code returned (often with extras)
```

### Three LLMs Involved

| LLM | Purpose | Role | Current Model |
|-----|---------|------|---------------|
| **Manager LLM** | Task decomposition | Break user request into subtasks | qwen2.5:7b |
| **Planning LLM** | Tool selection | Decide which tools to use | qwen2.5:7b |
| **Code LLM** | Code generation | Generate actual code | qwen2.5-coder:14b |

### Where "Over-Helpfulness" Happens

**PRIMARY CULPRIT**: `qwen2.5-coder:14b` in `generate_code` tool

**Evidence**:
1. File: `/capabilities/code_intelligence/generate_code.go`
2. System prompt (line 152): "You are a {language} code generator. Generate ONLY code..."
3. User prompt (line 113): "Generate {language} code that: {description}"

**Problem**: The prompt is too vague. It says "create interface" and the model thinks:
- "I should make this useful"
- "Interfaces need implementations"
- "Let me add examples/tests"
- "More code = more helpful"

### Why This Happens

1. **Training bias**: qwen2.5-coder trained on complete, production-ready code examples
2. **Context missing**: Model doesn't understand this is ONE atomic task in a multi-task workflow
3. **Vague descriptions**: "Create interface" could mean many things
4. **No validation**: System doesn't check if output matches requirements

---

## Current Mitigations (Already in Code)

Looking at `generate_code.go`, there ARE some mitigations:

1. **Line 121-123**: "CRITICAL: Do NOT include test functions (TestXxx). Tests will be created separately."
2. **Line 117-118**: "Keep main() minimal - it should only call other functions."
3. **Line 145**: "Respond with ONLY the code. No markdown code blocks..."

**But these mitigations aren't working!** Why?
- Prompts are WEAK ("Do NOT" is ignored)
- No enforcement/validation
- Model is trained to be "complete" and ignores constraints

---

## Proposed Solutions

### Solution 1: Specialized Manager LLM ⭐ **RECOMMENDED**

**Concept**: Use a different, smaller model for ManagerAgent task decomposition

**Rationale**:
- Task decomposition is STRUCTURE, not creativity
- Needs JSON output, not prose
- Should be deterministic, not creative
- Smaller model = faster, cheaper, more controllable

**Implementation**:
```go
// New LLM purpose
const PurposeTaskDecomposition llm.Purpose = "task_decomposition"

// In ManagerAgent
llmManager.RegisterLLM(PurposeTaskDecomposition, llm.Config{
    Provider: "ollama",
    Model:    "qwen2.5:7b", // Or even smaller: "qwen2.5:3b"
    // Specialized system prompt for structured output
})
```

**Benefits**:
- Clearer separation of concerns
- Can tune prompt specifically for JSON output
- Can use schema-guided generation
- Faster decomposition

---

### Solution 2: Stricter Code Generation Prompts ⭐ **HIGH IMPACT**

**Current problem**: Prompts use soft language ("IMPORTANT", "CRITICAL")

**Fix**: Use commanding, unambiguous language:

```diff
- prompt.WriteString("CRITICAL: Do NOT include test functions (TestXxx). ")
+ prompt.WriteString("STRICT REQUIREMENT: Generate ZERO test functions. ")
+ prompt.WriteString("If you include ANY TestXxx functions, the output will be REJECTED. ")
+ prompt.WriteString("Production code ONLY. No tests. No examples. No mocks. ")
```

**Add atomic task context**:
```go
prompt.WriteString("CONTEXT: You are generating ONE file in a multi-task workflow. ")
prompt.WriteString("Do NOT try to be complete. Do NOT add extras. ")
prompt.WriteString("Generate EXACTLY what is requested. Nothing more. ")
```

**File**: `/capabilities/code_intelligence/generate_code.go` lines 120-125

---

### Solution 3: Output Validation & Rejection ⭐ **HIGH IMPACT**

**Concept**: Validate generated code BEFORE saving

```go
func validateGeneratedCode(code string, requirements CodeRequirements) error {
    // Check 1: No test functions in production code
    if !requirements.IsTestFile && containsTestFunctions(code) {
        return fmt.Errorf("REJECTED: Production code contains test functions")
    }

    // Check 2: Respect file name constraints
    if requirements.FileName != "" {
        // Ensure code is for requested filename
        if !isValidForFile(code, requirements.FileName) {
            return fmt.Errorf("REJECTED: Code doesn't match file type %s", requirements.FileName)
        }
    }

    // Check 3: No extras beyond description
    if requirements.MinimalOnly {
        if hasExtraFunctions(code, requirements.ExpectedFunctions) {
            return fmt.Errorf("REJECTED: Code contains extra functions")
        }
    }

    return nil
}
```

**Implementation**: Add validation step between code generation and write_file

**File**: `/agent/base/executor.go` around line 250 (after generate_code, before write_file)

---

### Solution 4: Feedback Loop for Wrong Output 🔄 **ROBUST**

**Concept**: If output doesn't match, automatically retry with corrective feedback

```go
if err := validateGeneratedCode(code, requirements); err != nil {
    // Don't fail immediately - give LLM one chance to correct
    feedback := fmt.Sprintf("Previous output was rejected: %v\n\n", err)
    feedback += "Generate again with STRICT adherence to requirements.\n"

    // Retry with feedback
    code, err = retryWithFeedback(ctx, originalPrompt, feedback)
}
```

**Benefits**:
- Self-correcting
- Learns from mistakes
- More robust than single-shot

**File**: `/capabilities/code_intelligence/generate_code.go` - add retry logic to Execute()

---

### Solution 5: Enforce File Name Constraints ⚠️ **CRITICAL FIX**

**Current bug**: `file_name` parameter is IGNORED

**Example**: Asked for "config.go", got "main.go"

**Root cause**: generate_code tool doesn't take filename as parameter!

**Fix**: Add filename to requirements:

```diff
func (t *GenerateCodeTool) Metadata() ToolMetadata {
    Parameters: []Parameter{
        // ... existing parameters ...
+       {
+           Name:        "target_filename",
+           Type:        "string",
+           Required:    false,
+           Description: "Specific filename to generate (e.g., 'config.go'). Code MUST be appropriate for this filename.",
+           Example:     "config.go",
+       },
    }
}
```

Then in prompt:
```go
if filename, ok := args["target_filename"].(string); ok {
    prompt.WriteString(fmt.Sprintf("TARGET FILE: %s\n", filename))
    prompt.WriteString("Generate code that belongs in this specific file.\n")
    prompt.WriteString("Do NOT generate a complete program - only code for THIS file.\n\n")
}
```

---

### Solution 6: Atomic Task Markers 🏷️ **ARCHITECTURAL**

**Concept**: Add metadata to tasks to guide LLM behavior

```go
type TaskScope string

const (
    ScopeMinimal  TaskScope = "minimal"   // Generate ONLY what's requested
    ScopeComplete TaskScope = "complete"  // Generate complete, production-ready code
    ScopePartial  TaskScope = "partial"   // Part of larger system
)

type CodeRequirements struct {
    Scope         TaskScope
    FileName      string
    IsTestFile    bool
    IsInterface   bool  // If true, NO implementations
    IsStruct      bool  // If true, NO methods (unless requested)
    IsMinimal     bool  // If true, ZERO extras
}
```

Then pass this to generate_code:

```go
if scope == ScopeMinimal {
    prompt.WriteString("SCOPE: MINIMAL GENERATION\n")
    prompt.WriteString("Generate the BARE MINIMUM requested.\n")
    prompt.WriteString("This is ONE atomic task in a larger workflow.\n")
    prompt.WriteString("Do NOT add: tests, examples, helpers, utilities, or extras.\n\n")
}
```

---

## Recommended Implementation Order

### Phase 1: Quick Wins (1-2 hours)
1. ✅ **Stricter prompts** (Solution 2) - Immediate impact
2. ✅ **Enforce filename** (Solution 5) - Critical bug fix
3. ✅ **Add atomic context** (Solution 6 - simple version)

### Phase 2: Validation (2-3 hours)
4. ✅ **Output validation** (Solution 3) - Catch problems early
5. ✅ **Feedback loop** (Solution 4) - Self-correction

### Phase 3: Architecture (4-6 hours)
6. ✅ **Specialized Manager LLM** (Solution 1) - Long-term best practice
7. ✅ **Full TaskScope system** (Solution 6 - complete)

---

## Specific Code Changes Needed

### File 1: `/capabilities/code_intelligence/generate_code.go`

**Lines 112-124** - Replace with:
```go
} else {
    // Regular code generation - ATOMIC TASK MODE
    prompt.WriteString(fmt.Sprintf("Generate %s code: %s\n\n", language, description))

    // ✅ CRITICAL: Atomic task context
    prompt.WriteString("TASK SCOPE: ATOMIC\n")
    prompt.WriteString("You are generating ONE component in a multi-step workflow.\n")
    prompt.WriteString("Do NOT try to create a complete program.\n")
    prompt.WriteString("Do NOT add extras, helpers, or utilities.\n\n")

    // ✅ CRITICAL: NO TESTS in production code
    prompt.WriteString("STRICT REQUIREMENT: ZERO test functions.\n")
    prompt.WriteString("If you include ANY function starting with 'Test', output will be REJECTED.\n")
    prompt.WriteString("This file is for PRODUCTION CODE only.\n\n")

    // ✅ OPTIONAL: File-specific constraints
    if filename, ok := args["target_filename"].(string); ok {
        prompt.WriteString(fmt.Sprintf("TARGET FILE: %s\n", filename))
        prompt.WriteString("Generate ONLY code that belongs in this specific file.\n\n")
    }
}
```

**Lines 27-48** - Add parameter:
```go
{
    Name:        "target_filename",
    Type:        "string",
    Required:    false,
    Description: "Target filename (e.g., 'config.go'). Code must be appropriate for this file.",
    Example:     "config.go",
},
{
    Name:        "scope",
    Type:        "string",
    Required:    false,
    Description: "Generation scope: 'minimal' (bare minimum) or 'complete' (production-ready)",
    Example:     "minimal",
},
```

---

### File 2: `/agent/orchestration/manager.go`

**Lines 637-640** - Add filename to task input:
```go
task1.Input = map[string]interface{}{
    "project_path":    projectPath,
    "file_type":       "implementation",
    "file_name":       extractFileName(request), // ✅ NEW: Extract requested filename
    "scope":           "minimal",                 // ✅ NEW: Atomic task scope
}
```

**Add helper function**:
```go
// extractFileName attempts to extract a specific filename from user request
// Examples: "create config.go" → "config.go", "handler interface" → "handler.go"
func extractFileName(request string) string {
    // Look for explicit filename with extension
    if matches := regexp.MustCompile(`([a-z_]+\.(go|py|js|ts|rs))`).FindStringSubmatch(request); len(matches) > 0 {
        return matches[1]
    }

    // Look for type hint (e.g., "handler interface" → "handler.go")
    lower := strings.ToLower(request)
    if strings.Contains(lower, "interface") {
        // Extract word before "interface"
        words := strings.Fields(lower)
        for i, word := range words {
            if word == "interface" && i > 0 {
                return words[i-1] + ".go"
            }
        }
    }

    // No specific filename found
    return ""
}
```

---

### File 3: `/agent/agents/code_agent.go`

**Lines 159-160** - Pass filename to LLM:
```go
systemPrompt := a.buildSystemPrompt()
userPrompt := a.buildUserPrompt(task, currentCtx)

// ✅ NEW: Add file constraints to prompt
if fileName, ok := task.Input["file_name"].(string); ok && fileName != "" {
    userPrompt += fmt.Sprintf("\n\n**TARGET FILE**: %s\n", fileName)
    userPrompt += "Generate code ONLY for this specific file. Do not create a complete program.\n"
}

if scope, ok := task.Input["scope"].(string); ok && scope == "minimal" {
    userPrompt += "\n**GENERATION SCOPE**: MINIMAL\n"
    userPrompt += "Generate the bare minimum requested. This is one atomic task in a workflow.\n"
}
```

---

## Testing Strategy

### Test 1: Interface Generation
```bash
# Before fix: Gets interface + mock + tests
# After fix: Gets ONLY interface

wilson "Create Handler interface with Process(data string) error method in handler.go"

# Expected: handler.go with ONLY:
# - package declaration
# - Handler interface definition
# - NO implementations, NO tests, NO mocks
```

### Test 2: Struct Generation
```bash
# Before fix: Gets struct in main.go + helpers + main()
# After fix: Gets struct in config.go ONLY

wilson "Create Config struct in config.go with Name, Port, Enabled fields"

# Expected: config.go with ONLY:
# - package declaration
# - Config struct definition
# - NO functions, NO methods, NO main()
```

### Test 3: Minimal vs Complete
```bash
# Minimal mode
wilson "Create basic HTTP handler [minimal]"
# Expected: ONLY handler function, no server, no main

# Complete mode
wilson "Create production HTTP server [complete]"
# Expected: Complete server with main(), error handling, etc.
```

---

## Metrics to Track

Post-implementation, measure:

1. **Over-generation rate**: % of tasks where LLM generates extras
   - Baseline: ~80% (from e2e tests)
   - Target: <10%

2. **Filename accuracy**: % of tasks where correct filename is used
   - Baseline: ~40% (main.go instead of requested)
   - Target: >90%

3. **First-attempt success**: % of code generations that compile without iteration
   - Baseline: ~30% (3 fix attempts needed)
   - Target: >80%

4. **Test contamination**: % of production files containing test functions
   - Baseline: ~50% (saw this in e2e tests)
   - Target: 0%

---

## Alternative: Model Fine-Tuning

**Long-term solution**: Fine-tune qwen2.5-coder on Wilson-specific examples

**Training data**:
- Pairs of (task description, minimal code output)
- Emphasis on atomic, focused generation
- Examples of what NOT to do

**Benefits**:
- Model learns Wilson's workflow
- More reliable than prompt engineering
- Faster (no need for verbose prompts)

**Costs**:
- Requires GPU resources
- Time to generate training data
- Need to maintain fine-tuned model

**Recommendation**: Start with prompt engineering (Phase 1-2), consider fine-tuning later if needed

---

## Conclusion

The "over-helpfulness" problem is **solvable with prompt engineering and validation**.

**Key insight**: qwen2.5-coder is trained to generate complete, production-ready code. Wilson's workflow needs **atomic, minimal generation**. This mismatch causes the issues.

**Solution**: Guide the model with:
1. Explicit scope constraints ("minimal", "atomic")
2. Strong prohibitions ("ZERO test functions")
3. File-specific context ("config.go ONLY")
4. Validation and feedback loops

**Priority**: Implement Phase 1 (quick wins) immediately - this will solve 80% of problems.

---

## Next Steps

1. ✅ LSP Phase 2 marked complete (done)
2. ⏳ Review this analysis with team
3. ⏳ Implement Phase 1 fixes (2 hours)
4. ⏳ Re-run e2e tests to validate
5. ⏳ Document results and iterate

**Ready to implement when you are!** 🚀
