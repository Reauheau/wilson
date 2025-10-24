# Wilson Model Specialization Recommendations

## Current Issues
- **qwen2.5:7b (chat)** - Unreliable tool calling, poor filename decisions
- **qwen2.5-coder:14b (code)** - Works okay but could be better
- **qwen2.5:7b (analysis)** - Same model as chat, could be specialized

---

## Recommended Model Architecture

### 1. **Orchestration & Tool Calling** (MOST CRITICAL)

**Current**: qwen2.5:7b (4.7GB)
**Problem**: Poor structured output, unreliable filename decisions

**Top Recommendations**:

#### Option A: **Hermes 3 (8B)** - ⭐ BEST FOR TOOL CALLING
- **Model**: `hermes3:8b` (already installed!)
- **Size**: 4.7GB (same as qwen2.5:7b)
- **Why**:
  - Specifically trained for function/tool calling
  - Excellent structured output (JSON)
  - Strong instruction following
  - Good at agent workflows
- **Trade-off**: Slightly slower than qwen but much more reliable
- **Recommendation**: **USE THIS FOR CHAT & CODE AGENT ORCHESTRATION**

#### Option B: **Mistral Small** (22B)
- **Model**: `mistral-small:22b`
- **Size**: ~14GB
- **Why**:
  - Native function calling support
  - Excellent at following complex instructions
  - Very reliable tool use
- **Trade-off**: Larger memory footprint
- **Use case**: If you have 16GB+ RAM and need maximum reliability

#### Option C: **Llama 3.1 (8B) or Llama 3.2 (3B)**
- **Model**: `llama3.1:8b` or `llama3.2:3b`
- **Size**: 4.7GB (8B) or 2GB (3B)
- **Why**:
  - Good tool calling capabilities
  - Fast inference
  - Well-tested in production
- **Trade-off**: Not as specialized as Hermes 3 for tools

---

### 2. **Code Generation**

**Current**: qwen2.5-coder:14b (9GB)
**Status**: Working reasonably well

**Top Recommendations**:

#### Option A: **DeepSeek Coder V2 (16B)** - ⭐ BEST CODE QUALITY
- **Model**: `deepseek-coder-v2:16b`
- **Size**: ~10GB
- **Why**:
  - State-of-the-art code generation
  - Excellent at Go, Python, JS
  - Better than qwen2.5-coder at complex logic
  - Fewer bugs in generated code
- **Recommendation**: **UPGRADE TO THIS**

#### Option B: **CodeQwen 2.5 (7B)** - LIGHTER ALTERNATIVE
- **Model**: `codeqwen:7b`
- **Size**: ~4.3GB
- **Why**:
  - Lighter than current 14B model
  - Specifically trained for code (not chat)
  - Good balance of quality/speed
- **Use case**: If memory is constrained

#### Option C: **Keep qwen2.5-coder:14b**
- If code quality is already acceptable
- Focus improvements elsewhere

---

### 3. **Analysis & Reasoning**

**Current**: qwen2.5:7b (same as orchestration)
**Problem**: Not specialized for analysis

**Top Recommendations**:

#### Option A: **Qwen2.5 (14B or 32B)** - ⭐ BEST REASONING
- **Model**: `qwen2.5:14b` or `qwen2.5:32b`
- **Size**: 9GB (14B) or 20GB (32B)
- **Why**:
  - Excellent reasoning capabilities
  - Strong at analysis and summarization
  - Better context understanding than 7B
- **Recommendation**: **Use 14B if you have RAM**

#### Option B: **Hermes 3 (8B)** - REUSE ORCHESTRATION MODEL
- **Model**: `hermes3:8b`
- **Why**:
  - Already loaded for orchestration
  - Save memory by reusing
  - Good enough for most analysis tasks
- **Use case**: Memory-constrained setup

---

## Recommended Configuration

### **Conservative (12GB RAM Available)**
```yaml
models:
  chat: hermes3:8b          # 4.7GB - Tool calling orchestration
  code: qwen2.5-coder:14b    # 9GB - Code generation
  analysis: hermes3:8b       # Reuse chat model
```
**Total**: ~14GB (4.7 + 9 + reuse)

### **Balanced (16GB RAM Available)** ⭐ RECOMMENDED
```yaml
models:
  chat: hermes3:8b              # 4.7GB - Tool calling orchestration
  code: deepseek-coder-v2:16b   # 10GB - Better code generation
  analysis: qwen2.5:14b         # 9GB - Better reasoning
```
**Total**: ~24GB (with worker ephemeral loading)
**Idle**: ~5GB (only chat loaded)
**Active**: ~15GB (chat + 1 worker)

### **High Performance (32GB+ RAM)**
```yaml
models:
  chat: mistral-small:22b       # 14GB - Maximum reliability
  code: deepseek-coder-v2:16b   # 10GB - Best code quality
  analysis: qwen2.5:32b         # 20GB - Maximum reasoning
```
**Total**: Could load 2 models at once for speed

---

## Model Capabilities Matrix

| Model | Tool Calling | Code Gen | Reasoning | Size | Speed |
|-------|--------------|----------|-----------|------|-------|
| **hermes3:8b** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | 4.7GB | Fast |
| qwen2.5:7b | ⭐⭐ | ⭐⭐ | ⭐⭐⭐ | 4.7GB | Fast |
| qwen2.5:14b | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | 9GB | Medium |
| qwen2.5-coder:14b | ⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | 9GB | Medium |
| **deepseek-coder-v2:16b** | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | 10GB | Medium |
| mistral-small:22b | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 14GB | Slow |
| llama3.1:8b | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | 4.7GB | Fast |

---

## Implementation Steps

### Phase 1: Fix Orchestration (IMMEDIATE) ⚡
```bash
# Pull and test Hermes 3
ollama pull hermes3:8b

# Update config
# Change chat model from qwen2.5:7b to hermes3:8b
```
**Impact**: Should fix filename selection and tool calling reliability immediately

### Phase 2: Upgrade Code Generation (NEXT WEEK)
```bash
# Pull better code model
ollama pull deepseek-coder-v2:16b

# Update config
# Change code model from qwen2.5-coder:14b to deepseek-coder-v2:16b
```
**Impact**: Better code quality, fewer bugs

### Phase 3: Specialize Analysis (OPTIONAL)
```bash
# If you have RAM, use larger model for analysis
ollama pull qwen2.5:14b

# Update config
# Change analysis from qwen2.5:7b to qwen2.5:14b
```
**Impact**: Better reasoning and analysis quality

---

## Key Insights

### 1. **Tool Calling is the Bottleneck**
- The orchestration model (chat) makes 100+ decisions per task
- Every wrong decision cascades into failures
- **Hermes 3 is specifically trained for this** ✅

### 2. **Code Model is Secondary**
- qwen2.5-coder:14b is "good enough" for now
- Only upgrade if you see code quality issues
- Focus on orchestration first

### 3. **Memory Management Matters**
- Wilson's ephemeral worker pattern is great
- Only chat model stays loaded (4-5GB)
- Workers (code/analysis) are loaded on-demand
- This allows using larger specialist models

### 4. **Hermes 3 is the Game Changer**
- Already installed on your system!
- Same size as qwen2.5:7b
- Much better at structured output
- Should fix the filename selection issue

---

## Testing Strategy

### Test 1: Hermes 3 Orchestration
```bash
# Update config to use hermes3:8b for chat
# Run: "create calculator with tests"
# Expected: Correct filenames (main.go, main_test.go)
```

### Test 2: Compare Code Quality
```bash
# Generate same code with both models
# Compare: correctness, structure, error handling
```

### Test 3: Memory Profile
```bash
# Monitor RAM usage during multi-task workflows
# Ensure workers are properly killed after use
```

---

## Immediate Action

**RECOMMENDATION: Switch to Hermes 3 for orchestration NOW**

1. It's already installed (`hermes3:8b`)
2. Same memory footprint
3. Should fix your filename selection bug
4. Better tool calling across the board

This is a **low-risk, high-reward** change that directly addresses the problems you're seeing.

---

## Additional Models to Consider (Future)

### Specialized Options:
- **phi3:medium** (14B) - Microsoft's model, excellent reasoning
- **gemma2:9b** - Google's model, good at structured tasks
- **command-r:35b** - Cohere's model, great at RAG/search
- **yi-coder:9b** - Specifically for code, very fast

### Experimental:
- **granite-code:8b** - IBM's code model
- **starcoder2:15b** - Specialized for code completion
- **codestral:22b** - Mistral's code specialist (if released for Ollama)

---

## References

- Ollama Tool Calling: https://ollama.com/blog/tool-support
- Hermes 3 Release: Known for agent/tool calling improvements
- DeepSeek Coder V2: State-of-the-art code generation benchmarks
- Wilson's Architecture: Supports ephemeral workers (makes larger models viable)
