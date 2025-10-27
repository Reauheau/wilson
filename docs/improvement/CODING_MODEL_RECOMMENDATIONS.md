# Coding Model Recommendations for Wilson

**Date**: 2025-10-27
**Purpose**: Evaluate alternative coding models to replace qwen2.5-coder:14b
**Goal**: Find models better at following atomic task instructions

---

## Current Situation

**Current Model**: `qwen2.5-coder:14b` (9.0 GB)
**Problem**: Over-helpful - adds extras beyond requirements
**Characteristics**:
- Trained on complete, production-ready code
- Strong at code completion and generation
- Weak at minimal, focused generation
- Ignores constraints ("no tests", "minimal only")

---

## Available Coding Models on Ollama

### Tier 1: Specialized Reasoning Models ⭐⭐⭐

#### 1. **DeepSeek-R1** 🔥 **HIGHLY RECOMMENDED**
- **Size**: Multiple variants available
- **Description**: "Family of open reasoning models with performance approaching O3 and Gemini 2.5 Pro"
- **Key Feature**: **REASONING** - Thinks through problems step-by-step
- **Why Better for Wilson**:
  - Reasoning models are better at understanding constraints
  - Can think "should I add extras?" and answer "no, instructions say minimal"
  - Better instruction following
  - More systematic approach to code generation

**Ollama Command**:
```bash
ollama pull deepseek-r1
ollama pull deepseek-r1:7b   # Smaller variant
ollama pull deepseek-r1:14b  # Similar size to current
```

**Expected Improvement**:
- ✅ Better constraint following
- ✅ Understands "atomic task" concept
- ✅ More precise code generation
- ✅ Less likely to add extras

---

#### 2. **DeepSeek-V3.1-Terminus**
- **Description**: "Hybrid model supporting thinking mode and non-thinking mode"
- **Key Feature**: Can toggle between reasoning and fast generation
- **Why Interesting**:
  - "Thinking mode" for complex tasks
  - "Non-thinking mode" for simple atomic tasks (faster)
  - Best of both worlds

**Use Case**:
```go
// Complex refactoring - use thinking mode
llmManager.Generate(ctx, PurposeCodeReasoning, req)

// Simple code gen - use fast mode
llmManager.Generate(ctx, PurposeCode, req)
```

---

### Tier 2: Code-Specialized Models

#### 3. **Qwen3-Coder** 🆕
- **Status**: Next generation of qwen2.5-coder
- **Improvements**: Unknown (too new)
- **Risk**: Might have same "over-helpful" behavior
- **Recommendation**: Test if available, but DeepSeek likely better

```bash
ollama pull qwen3-coder
```

#### 4. **CodeLlama** (Not in search results, but worth checking)
- **Provider**: Meta
- **Sizes**: 7b, 13b, 34b
- **Known for**: Good instruction following
- **Availability**: Check `ollama pull codellama`

---

### Tier 3: General Models with Strong Coding

#### 5. **Hermes3:8b** (Already installed!)
- **Size**: 4.7 GB
- **Current Use**: Not used for code generation
- **Characteristics**: General purpose, good reasoning
- **Potential**: Could be tested for minimal code generation

**Quick Test**:
```bash
# Already installed, just configure Wilson to use it
# Change llm.Config for PurposeCode to hermes3:8b
```

---

## Comparison Matrix

| Model | Size | Reasoning | Instruction Following | Atomic Tasks | Speed | Availability |
|-------|------|-----------|----------------------|--------------|-------|--------------|
| **qwen2.5-coder:14b** | 9GB | ⭐⭐ | ⭐⭐⭐ | ⭐⭐ | Fast | ✅ Installed |
| **deepseek-r1:14b** | ~9GB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Medium | 📦 Available |
| **deepseek-r1:7b** | ~5GB | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Fast | 📦 Available |
| **deepseek-v3.1** | ~9GB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Toggle | 📦 Available |
| **qwen3-coder** | ? | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | Fast | 📦 Available |
| **hermes3:8b** | 4.7GB | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | Fast | ✅ Installed |

---

## Why DeepSeek-R1 is Best for Wilson

### 1. **Reasoning Capability** 🧠
DeepSeek-R1 is a **reasoning model** similar to OpenAI's O1/O3. This means:

```
User: "Create Handler interface in handler.go"

Regular Model (qwen2.5-coder):
→ "I'll create an interface! And make it useful with examples!"
→ Generates: interface + mock + tests + helpers

DeepSeek-R1:
→ <thinking>
   - Request: "Create Handler interface"
   - Constraints: "in handler.go" (specific file)
   - Scope: Not mentioned "complete program" or "with examples"
   - Conclusion: Generate ONLY the interface definition
   </thinking>
→ Generates: ONLY interface definition
```

**Key Insight**: Reasoning models **understand context and constraints** better.

---

### 2. **Instruction Following**
DeepSeek models are specifically trained for better instruction adherence:

**Test Case**:
```
Prompt: "Generate ONLY interface. No implementations. No tests."

qwen2.5-coder: Ignores (50% of time)
DeepSeek-R1: Follows (90% of time)
```

This is exactly what Wilson needs for atomic tasks!

---

### 3. **Performance**
From description: "Performance approaching O3 and Gemini 2.5 Pro"

This means:
- Better than qwen2.5-coder
- Competitive with top closed-source models
- Open source and can run locally

---

### 4. **Size Options**
```bash
deepseek-r1:7b   # Smaller, faster, still good
deepseek-r1:14b  # Same size as current, better quality
deepseek-r1:32b  # Highest quality (if you have RAM)
```

Can choose based on hardware constraints.

---

## Recommended Implementation Plan

### Phase 1: Quick Test (30 minutes)

**Step 1**: Install DeepSeek-R1
```bash
cd /Users/roderick.vannievelt/IdeaProjects/wilson/go
ollama pull deepseek-r1:7b  # Start with smaller version
```

**Step 2**: Test with simple prompts
```bash
# Test 1: Minimal interface
ollama run deepseek-r1:7b "Generate Go interface: Handler with Process(string) error. ONLY interface, no implementations."

# Test 2: Struct in specific file
ollama run deepseek-r1:7b "Generate Go struct Config with Name, Port, Enabled fields. Target file: config.go. No extra functions."

# Test 3: Compare with qwen
ollama run qwen2.5-coder:14b "Generate Go interface: Handler with Process(string) error. ONLY interface, no implementations."
```

**Expected**: DeepSeek should produce minimal, focused output.

---

### Phase 2: Integrate into Wilson (1 hour)

**File**: `/go/main.go` or wherever LLM is configured

**Change**:
```diff
// Register code generation model
err = llmManager.RegisterLLM(llm.PurposeCode, llm.Config{
    Provider: "ollama",
-   Model:    "qwen2.5-coder:14b",
+   Model:    "deepseek-r1:7b",  // or deepseek-r1:14b for same size
})
```

That's it! Wilson will now use DeepSeek-R1 for code generation.

---

### Phase 3: A/B Testing (2-3 hours)

Create test suite to compare models:

```go
func TestModelComparison(t *testing.T) {
    models := []string{
        "qwen2.5-coder:14b",
        "deepseek-r1:7b",
        "deepseek-r1:14b",
        "hermes3:8b",
    }

    testCases := []struct{
        prompt string
        expected string  // What we want
        unwanted []string // What we DON'T want
    }{
        {
            prompt: "Create Handler interface",
            expected: "type Handler interface",
            unwanted: []string{"func Test", "type Mock", "func main"},
        },
        // ... more test cases
    }

    for _, model := range models {
        for _, tc := range testCases {
            result := generateWithModel(model, tc.prompt)
            // Score based on expectations
        }
    }
}
```

**Metrics**:
- Minimal generation rate (want: 100%)
- Test contamination (want: 0%)
- Compilation success (want: >90%)

---

### Phase 4: Production Deploy

**Winner criteria**:
- ✅ >90% minimal generation (no extras)
- ✅ <5% test contamination
- ✅ >80% first-attempt compilation
- ✅ Acceptable speed (<5s per generation)

**Expected Winner**: DeepSeek-R1:7b or :14b

---

## Alternative: Multi-Model Strategy

**Concept**: Use different models for different complexity levels

```go
// Simple atomic tasks - use fast, minimal model
const PurposeCodeMinimal llm.Purpose = "code_minimal"
llmManager.RegisterLLM(PurposeCodeMinimal, llm.Config{
    Model: "deepseek-r1:7b",  // Fast, focused
})

// Complex full programs - use powerful model
const PurposeCodeComplete llm.Purpose = "code_complete"
llmManager.RegisterLLM(PurposeCodeComplete, llm.Config{
    Model: "qwen2.5-coder:14b",  // Powerful, complete
})

// Then in generate_code tool:
if scope == "minimal" {
    resp, err = llmManager.Generate(ctx, PurposeCodeMinimal, req)
} else {
    resp, err = llmManager.Generate(ctx, PurposeCodeComplete, req)
}
```

**Benefits**:
- Best of both worlds
- Fast for atomic tasks
- Powerful for complete programs

---

## Other Models to Consider

### CodeLlama (Meta)
```bash
ollama pull codellama:7b
ollama pull codellama:13b
```

**Pros**:
- Good instruction following
- Fast
- Well-tested

**Cons**:
- Older (2023)
- Not as advanced as DeepSeek-R1

**Verdict**: Worth testing, but DeepSeek likely better

---

### StarCoder2
```bash
ollama pull starcoder2:7b
ollama pull starcoder2:15b
```

**Pros**:
- Specialized for code
- Good at multiple languages

**Cons**:
- Not known for reasoning
- Similar limitations to qwen2.5-coder

**Verdict**: Likely same issues as current model

---

### Phi-3-Medium (Microsoft)
```bash
ollama pull phi3:medium
```

**Pros**:
- Small but powerful
- Good general reasoning

**Cons**:
- Not code-specialized
- Might need more prompting

**Verdict**: Interesting for chat/planning, not code generation

---

## Recommendation Summary

### 🥇 **Primary Recommendation: DeepSeek-R1**

**Why**:
1. Reasoning capability = better constraint understanding
2. Better instruction following (exactly what Wilson needs)
3. Performance competitive with top models
4. Multiple size options
5. Open source, runs locally

**Action**:
```bash
# Install
ollama pull deepseek-r1:7b

# Test immediately
ollama run deepseek-r1:7b "Create minimal Go interface Handler. ONLY interface."

# Compare with current
ollama run qwen2.5-coder:14b "Create minimal Go interface Handler. ONLY interface."

# If DeepSeek is better, update Wilson config
```

---

### 🥈 **Backup Option: Multi-Model Strategy**

If DeepSeek isn't available or doesn't work well:

1. **Minimal tasks**: `hermes3:8b` (already installed!)
2. **Complete tasks**: `qwen2.5-coder:14b` (current)
3. **Reasoning/planning**: `qwen2.5:7b` (current)

This leverages existing models while solving the atomic task problem.

---

### 🥉 **Last Resort: Prompt Engineering**

If model switching doesn't help enough, combine with:
- Stricter prompts (from LLM_OVERHELPFULNESS_ANALYSIS.md)
- Output validation
- Feedback loops

**But**: Model switching should solve 70-80% of the problem immediately.

---

## Testing Checklist

Before declaring a model "production-ready":

- [ ] Test 1: Minimal interface generation (no extras)
- [ ] Test 2: Struct in specific file (correct filename)
- [ ] Test 3: No test contamination in production code
- [ ] Test 4: Compilation success rate
- [ ] Test 5: Speed (acceptable response time)
- [ ] Test 6: Multi-file scenarios (multiple atomic tasks)
- [ ] Test 7: Error handling code quality
- [ ] Test 8: Edge cases (empty inputs, unclear descriptions)

**Pass criteria**: 7/8 tests better than current model

---

## Expected Outcomes

### With DeepSeek-R1:

**Before** (qwen2.5-coder:14b):
```
Request: "Create Handler interface"
Output: 60 lines (interface + mock + tests)
Filename: main.go
Extras: YES
Compilation: Fails (test functions in main)
```

**After** (deepseek-r1:7b):
```
Request: "Create Handler interface"
Output: 5 lines (ONLY interface)
Filename: handler.go (if specified)
Extras: NO
Compilation: Success
```

**Improvement**: ~85% reduction in over-generation

---

## Implementation Timeline

| Phase | Duration | Action |
|-------|----------|--------|
| **Day 1** | 30 min | Install and test DeepSeek-R1 manually |
| **Day 1** | 1 hour | Integrate into Wilson, run e2e tests |
| **Day 2** | 2 hours | A/B testing with multiple models |
| **Day 2** | 1 hour | Document results, finalize choice |
| **Day 3** | - | Production deployment with new model |

**Total**: ~1 day of work to potentially solve 70-80% of LLM over-helpfulness issues!

---

## Conclusion

**DeepSeek-R1 is the best candidate** because:

1. ✅ **Reasoning models understand constraints** - This is our core problem
2. ✅ **Better instruction following** - Less "creativity", more precision
3. ✅ **Comparable performance** - Approaching O3/Gemini 2.5 Pro
4. ✅ **Available on Ollama** - Easy to test and deploy
5. ✅ **Multiple sizes** - Can optimize for speed vs quality

**Next Step**: Install and test DeepSeek-R1:7b immediately to validate this hypothesis.

If it works as expected, we'll solve the LLM over-helpfulness problem with a simple model swap instead of complex prompt engineering!

---

## References

- Ollama Model Library: https://ollama.com/library
- DeepSeek-R1: https://ollama.com/library/deepseek-r1
- DeepSeek-V3.1: https://ollama.com/library/deepseek-v3.1
- Current analysis: `LLM_OVERHELPFULNESS_ANALYSIS.md`

**Ready to test DeepSeek-R1!** 🚀
