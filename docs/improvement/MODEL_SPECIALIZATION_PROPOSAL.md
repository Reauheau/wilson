# Wilson Model Specialization - Detailed Breakdown

## Current Model Usage Analysis

### **PurposeChat** (qwen2.5:7b - 4.7GB)
Currently used by:
1. **ChatAgent** (line 20, chat_agent.go) - User interaction, intent classification
2. **CodeAgent** (line 24, code_agent.go) - Tool calling orchestration ⚠️ PROBLEM AREA
3. **ManagerAgent** (line 584, manager_agent.go) - Task decomposition

### **PurposeCode** (qwen2.5-coder:14b - 9GB)
Currently used by:
1. **TestAgent** (line 22, test_agent.go) - Test orchestration
2. **generate_code tool** - Actual code generation

### **PurposeAnalysis** (qwen2.5:7b - 4.7GB)
Currently used by:
1. **AnalysisAgent** (line 19, analysis_agent.go) - Content analysis
2. **ReviewAgent** (line 19, review_agent.go) - Code review
3. **ResearchAgent** (line 19, research_agent.go) - Research tasks

---

## Problem: Three Distinct Roles Under "Chat"

### 1. **User Interaction** (ChatAgent)
**Current**: PurposeChat (qwen2.5:7b)
**Requirements**:
- Natural conversation
- Intent classification
- Quick response time
- Always loaded (KeepAlive)

### 2. **Tool Calling Orchestration** (CodeAgent, TestAgent)
**Current**: PurposeChat for CodeAgent, PurposeCode for TestAgent ⚠️ INCONSISTENT
**Requirements**:
- Reliable JSON generation
- Precise tool selection
- Following structured conventions
- Making consistent decisions (filename, order, etc.)
**THIS IS WHERE FAILURES OCCUR** ❌

### 3. **Task Decomposition** (ManagerAgent)
**Current**: PurposeChat (qwen2.5:7b)
**Requirements**:
- Breaking complex tasks into subtasks
- Understanding dependencies
- Strategic planning
- Not real-time (can be slower)

---

## Proposed: 5 Specialized Purposes

```go
const (
    PurposeChat         Purpose = "chat"         // User interaction
    PurposeOrchestration Purpose = "orchestration" // Tool calling
    PurposePlanning     Purpose = "planning"      // Task decomposition
    PurposeCode         Purpose = "code"          // Code generation
    PurposeAnalysis     Purpose = "analysis"      // Analysis & reasoning
)
```

### **Mapping**

| Agent/Component | Current Purpose | Proposed Purpose | Model Recommendation |
|----------------|-----------------|------------------|---------------------|
| **ChatAgent** | PurposeChat | PurposeChat | hermes3:8b (fast, conversational) |
| **CodeAgent** | PurposeChat | **PurposeOrchestration** | **hermes3:8b** (tool specialist) ⭐ |
| **TestAgent** | PurposeCode | **PurposeOrchestration** | **hermes3:8b** (consistency) ⭐ |
| **ManagerAgent** | PurposeChat | **PurposePlanning** | mistral-small:22b or qwen2.5:14b |
| **generate_code** | PurposeCode | PurposeCode | deepseek-coder-v2:16b |
| **AnalysisAgent** | PurposeAnalysis | PurposeAnalysis | qwen2.5:14b |
| **ReviewAgent** | PurposeAnalysis | PurposeAnalysis | qwen2.5:14b |
| **ResearchAgent** | PurposeAnalysis | PurposeAnalysis | qwen2.5:14b |

---

## Recommended Configuration Options

### **Option 1: Minimal Change (Same Memory)** ⚡ QUICK WIN
```yaml
models:
  chat: hermes3:8b           # User interaction (4.7GB, KeepAlive)
  orchestration: hermes3:8b  # SAME MODEL, different purpose
  planning: hermes3:8b       # SAME MODEL, different purpose
  code: qwen2.5-coder:14b    # Code generation (9GB, ephemeral)
  analysis: qwen2.5:7b       # Analysis (4.7GB, ephemeral)
```
**Memory**: Same as now (~5GB idle, ~15GB active)
**Benefit**: Fixes orchestration by using hermes3 (tool specialist)
**Implementation**: Easy - just add new purpose mappings

### **Option 2: Balanced Specialization** ⭐ RECOMMENDED
```yaml
models:
  chat: hermes3:8b              # User interaction (4.7GB, KeepAlive)
  orchestration: hermes3:8b     # Tool calling (REUSE, no extra memory!)
  planning: qwen2.5:14b         # Better decomposition (9GB, ephemeral)
  code: deepseek-coder-v2:16b   # Better code (10GB, ephemeral)
  analysis: qwen2.5:14b         # Better reasoning (9GB, ephemeral)
```
**Memory**: ~5GB idle, ~15GB active (1 worker at a time)
**Benefit**: Each role has optimal model
**Implementation**: Moderate - need to pull new models

### **Option 3: Maximum Specialization** (32GB+ RAM)
```yaml
models:
  chat: hermes3:8b              # User interaction (4.7GB, KeepAlive)
  orchestration: mistral-small:22b # Maximum reliability (14GB, KeepAlive)
  planning: mistral-small:22b   # Strategic thinking (REUSE)
  code: deepseek-coder-v2:16b   # Best code (10GB, ephemeral)
  analysis: qwen2.5:32b         # Maximum reasoning (20GB, ephemeral)
```
**Memory**: ~20GB idle, ~30GB active
**Benefit**: Each role has the absolute best model
**Implementation**: Requires significant RAM

---

## Why This Matters

### **Current Problem Traced**
1. User says: "create calculator and a test file"
2. ChatAgent → ManagerAgent (PurposeChat/qwen2.5:7b)
   - Decomposes into tasks ✓
3. CodeAgent (PurposeChat/qwen2.5:7b) orchestrates tools
   - **Chooses wrong filename** ❌
   - **Unreliable JSON** ❌
   - **Inconsistent decisions** ❌

### **Why qwen2.5:7b Fails at Orchestration**
- Trained for chat, not agent workflows
- Not optimized for structured output
- No specific function-calling training
- Makes "creative" decisions (bad for agents!)

### **Why Hermes 3 Will Work**
- Specifically trained for tool calling
- Reliable JSON generation
- Follows conventions precisely
- Makes consistent decisions
- **Used by many agent frameworks in production**

---

## Implementation Plan

### **Phase 1: Add PurposeOrchestration** (1-2 hours)

#### Step 1: Update types
```go
// llm/types.go
const (
    PurposeChat          Purpose = "chat"
    PurposeOrchestration Purpose = "orchestration" // NEW
    PurposeCode          Purpose = "code"
    PurposeAnalysis      Purpose = "analysis"
)
```

#### Step 2: Update agents
```go
// agent/code_agent.go:24
base := NewBaseAgent("Code", llm.PurposeOrchestration, llmManager, contextMgr)

// agent/test_agent.go:22
base := NewBaseAgent("Test", llm.PurposeOrchestration, llmManager, contextMgr)
```

#### Step 3: Update config
```yaml
# config/models.yaml
models:
  chat:
    model: "hermes3:8b"
    keep_alive: true
  orchestration:
    model: "hermes3:8b"  # Same as chat for now
    keep_alive: false
  code:
    model: "qwen2.5-coder:14b"
    keep_alive: false
  analysis:
    model: "qwen2.5:7b"
    keep_alive: false
```

#### Step 4: Test
```bash
# Should now use hermes3 for tool calling
echo "create calculator with tests" | ./wilson
```

---

### **Phase 2: Add PurposePlanning** (Optional, 1 hour)

#### Step 1: Add purpose
```go
const PurposePlanning Purpose = "planning"
```

#### Step 2: Update ManagerAgent
```go
// agent/manager_agent.go:584
_, err := m.llmManager.Generate(ctx, llm.PurposePlanning, req)
```

#### Step 3: Configure planning model
```yaml
models:
  planning:
    model: "qwen2.5:14b"  # Better strategic thinking
    keep_alive: false
```

---

### **Phase 3: Upgrade Specialist Models** (Optional)

Pull better models:
```bash
ollama pull deepseek-coder-v2:16b  # Better code generation
ollama pull qwen2.5:14b            # Better reasoning
```

Update config to use them.

---

## Expected Results

### **Before** (current)
```
Task: Create calculator with tests
→ TASK-1: Implement functionality
  → CodeAgent (qwen2.5:7b chat)
    → Chooses filename: main_test.go ❌ WRONG
    → Creates implementation in test file ❌
  → ERROR: No main.go found
```

### **After** (with PurposeOrchestration + hermes3)
```
Task: Create calculator with tests
→ TASK-1: Implement functionality
  → CodeAgent (hermes3:8b orchestration)
    → Chooses filename: main.go ✓ CORRECT
    → Creates implementation properly ✓
→ TASK-2: Add tests
  → CodeAgent (hermes3:8b orchestration)
    → Reads main.go ✓
    → Chooses filename: main_test.go ✓
    → Creates tests properly ✓
→ SUCCESS ✓
```

---

## Memory Management

### Current (2 purposes, 2 models)
```
Idle:   chat (qwen2.5:7b) = 4.7GB
Active: + code worker      = +9GB = 13.7GB total
```

### After Phase 1 (3 purposes, 2 models)
```
Idle:   chat (hermes3:8b) = 4.7GB
Active: + orchestration (SAME hermes3, already loaded) = 4.7GB
Active: + code worker                                   = +9GB = 13.7GB total
```
**No extra memory!** Orchestration reuses chat model.

### After Phase 2 (4 purposes, 3 models)
```
Idle:   chat (hermes3:8b)                = 4.7GB
Active: + planning (qwen2.5:14b, loaded on-demand) = +9GB = 13.7GB
OR
Active: + orchestration (hermes3, reuse)           = 4.7GB
OR
Active: + code worker                              = +9GB = 13.7GB
```
**Only 1 worker at a time**, so max is still ~14GB.

---

## Key Insight: Model Reuse

**You can map multiple purposes to the same model!**

```yaml
models:
  chat:
    model: "hermes3:8b"
  orchestration:
    model: "hermes3:8b"  # SAME MODEL
  planning:
    model: "hermes3:8b"  # SAME MODEL
```

**Benefits**:
- Same memory footprint
- Semantic separation in code
- Easy to specialize later
- Model stays loaded (no reload cost)

---

## Testing Strategy

### Test 1: Filename Selection
```bash
# Before: Creates main_test.go for implementation
# After: Creates main.go correctly
echo "create calculator" | ./wilson
```

### Test 2: Multi-file Generation
```bash
# Before: Wrong filenames, files in wrong order
# After: Correct filenames, proper sequence
echo "create calculator with tests" | ./wilson
```

### Test 3: Tool Reliability
```bash
# Before: Sometimes skips read_file, wrong tool order
# After: Consistent tool usage, reads before generating
# Check logs for tool execution order
```

### Test 4: Memory Profile
```bash
# Monitor that hermes3 stays loaded
# Monitor worker models load/unload correctly
watch -n 1 'ps aux | grep ollama'
```

---

## Rollback Plan

If hermes3 doesn't work well:

```yaml
# Revert to qwen2.5:7b
models:
  orchestration:
    model: "qwen2.5:7b"
```

Or try alternatives:
- `mistral-small:22b` (larger but very reliable)
- `llama3.1:8b` (similar size, good tool calling)
- `phi3:medium` (Microsoft's model, precise)

---

## Conclusion

**The issue isn't your architecture - it's asking a chat model to do agent work.**

By introducing `PurposeOrchestration` and using Hermes 3, you:
1. ✅ Separate concerns (chat vs orchestration vs planning)
2. ✅ Use specialized models for each role
3. ✅ Fix the filename selection bug
4. ✅ Same memory footprint (can reuse models)
5. ✅ Easy to specialize further later

**Recommendation**: Start with Phase 1 (PurposeOrchestration + hermes3) - it's low-risk, same memory, high reward.
