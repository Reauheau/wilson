# Wilson Git Context Integration Plan

**Date:** October 27, 2025
**Status:** Ready to Implement
**Priority:** HIGH - Critical for agent awareness
**Estimated Effort:** 1-2 days
**Dependencies:** Git tools already implemented (see GIT_TOOLS_IMPLEMENTATION.md)

---

## 🎯 Goal

Integrate the 7 existing git tools into Wilson's agent system to provide git context awareness:
- Agents know what files are modified
- Agents understand current branch
- Agents can view commit history
- Branch-aware behavior (stricter checks on main/master)

**Impact:** +30% effectiveness for real-world development workflows

---

## 📚 Background

### What We Have ✅
1. **Git Tools** - 7 tools fully implemented and tested (`go/capabilities/git/`)
2. **Agent Architecture** - Ready for new tools
3. **TaskContext** - Needs git fields added

### What's Missing 🔴
1. Git context not injected into TaskContext
2. Git tools not added to agent allowed lists
3. No git-aware prompts in agents

---

## 🏗️ Implementation Plan

### Step 1: Enhance TaskContext

**File:** `go/agent/base/task_context.go`

**Add these fields:**
```go
type TaskContext struct {
    // ... existing fields ...

    // Git Context (NEW)
    GitRoot          string   // Git repository root (absolute path)
    GitBranch        string   // Current branch name
    GitModifiedFiles []string // Modified files from git status
    GitStagedFiles   []string // Staged files
    GitUntrackedFiles []string // Untracked files
    GitClean         bool     // No uncommitted changes
    GitAheadBehind   string   // "ahead 2, behind 1" or empty
}
```

**Why these fields:**
- `GitRoot` → Enables relative path calculations, workspace awareness
- `GitBranch` → Branch-specific behavior (stricter checks on main/master)
- `GitModifiedFiles` → Agents can read these first for context
- `GitClean` → Safety check before destructive operations

---

### Step 2: Manager Enriches Context

**File:** `go/agent/orchestration/manager.go`

**Add method:**
```go
func (m *ManagerAgent) enrichTaskContextWithGit(taskCtx *base.TaskContext) error {
    // Check if project path is in a git repo
    gitRoot, err := git.FindGitRoot(taskCtx.ProjectPath)
    if err != nil {
        // Not a git repo - that's okay
        return nil
    }

    taskCtx.GitRoot = gitRoot

    // Get git status
    statusTool := &git.GitStatusTool{}
    result, err := statusTool.Execute(context.Background(), map[string]interface{}{
        "path": gitRoot,
    })
    if err != nil {
        // Git command failed - log but don't fail task
        fmt.Printf("[Manager] Warning: git_status failed: %v\n", err)
        return nil
    }

    // Parse JSON result
    var status map[string]interface{}
    if err := json.Unmarshal([]byte(result), &status); err != nil {
        return fmt.Errorf("failed to parse git status: %w", err)
    }

    // Populate git context
    taskCtx.GitBranch = status["branch"].(string)
    taskCtx.GitModifiedFiles = toStringSlice(status["modified"])
    taskCtx.GitStagedFiles = toStringSlice(status["staged"])
    taskCtx.GitUntrackedFiles = toStringSlice(status["untracked"])
    taskCtx.GitClean = status["clean"].(bool)

    // Parse ahead/behind
    if ahead, ok := status["ahead"].(float64); ok {
        if behind, ok := status["behind"].(float64); ok {
            if ahead > 0 || behind > 0 {
                taskCtx.GitAheadBehind = fmt.Sprintf("ahead %d, behind %d", int(ahead), int(behind))
            }
        }
    }

    fmt.Printf("[Manager] Git context enriched: branch=%s, modified=%d files\n",
        taskCtx.GitBranch, len(taskCtx.GitModifiedFiles))

    return nil
}

// Helper to convert []interface{} to []string
func toStringSlice(val interface{}) []string {
    if arr, ok := val.([]interface{}); ok {
        result := make([]string, 0, len(arr))
        for _, item := range arr {
            if str, ok := item.(string); ok {
                result = append(result, str)
            }
        }
        return result
    }
    return []string{}
}
```

**Call site:** In `ExecuteTaskPlan()`, add before executing task:
```go
// Create TaskContext for task
taskCtx := NewTaskContext(task)

// ✅ NEW: Enrich with git context
if err := m.enrichTaskContextWithGit(taskCtx); err != nil {
    fmt.Printf("[ManagerAgent] Warning: Failed to enrich git context: %v\n", err)
}

// Continue with task execution...
```

---

### Step 3: Add Git Tools to Agents

**CodeAgent** (`go/agent/agents/code_agent.go`):
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // Git tools
    "git_status",   // Check repo state
    "git_diff",     // View file changes
    "git_log",      // View commit history
    "git_show",     // Show commit details
    "git_blame",    // Find code authors
    "git_branch",   // Branch operations
})
```

**TestAgent** (`go/agent/agents/test_agent.go`):
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // Git tools
    "git_status",   // Detect test files
    "git_diff",     // See what code changed
    "git_log",      // Understand test history
})
```

**ReviewAgent** (`go/agent/agents/review_agent.go`):
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // Git tools
    "git_status",   // Check what's being reviewed
    "git_diff",     // Critical for reviewing changes
    "git_log",      // Understand change history
    "git_show",     // Inspect specific commits
    "git_blame",    // Find code ownership
})
```

**ChatAgent** (`go/agent/agents/chat_agent.go`):
```go
baseAgent.SetAllowedTools([]string{
    // ... existing tools ...

    // Git tools
    "git_status",   // Answer "what files did I change?"
    "git_log",      // Answer "what were my recent commits?"
    "git_branch",   // Answer "what branch am I on?"
})
```

---

### Step 4: Update Agent Prompts

**CodeAgent** - Add to `buildUserPrompt()`:
```go
// Add git context section
if len(taskCtx.GitModifiedFiles) > 0 {
    prompt.WriteString("\n🔀 **Git Context**:\n")
    prompt.WriteString(fmt.Sprintf("- Branch: %s\n", taskCtx.GitBranch))
    prompt.WriteString(fmt.Sprintf("- Modified files (%d):\n", len(taskCtx.GitModifiedFiles)))
    for i, file := range taskCtx.GitModifiedFiles {
        if i >= 5 {
            prompt.WriteString(fmt.Sprintf("  ... and %d more\n", len(taskCtx.GitModifiedFiles)-5))
            break
        }
        prompt.WriteString(fmt.Sprintf("  - %s\n", file))
    }

    if taskCtx.GitBranch != "master" && taskCtx.GitBranch != "main" {
        prompt.WriteString("\n⚠️  You're on a feature branch. Consider reading modified files for context.\n")
    }

    if !taskCtx.GitClean {
        prompt.WriteString("⚠️  Uncommitted changes detected. Use git_diff to see changes before modifying files.\n")
    }
    prompt.WriteString("\n")
}
```

**ReviewAgent** - Add branch-aware checks:
```go
// In buildUserPrompt():
if taskCtx.GitBranch == "master" || taskCtx.GitBranch == "main" {
    prompt += "\n⚠️  **CRITICAL**: Changes target main branch. Apply strictest review standards.\n"
    prompt += "Check: security, breaking changes, backward compatibility, documentation.\n\n"
}

if len(taskCtx.GitModifiedFiles) > 10 {
    prompt += fmt.Sprintf("\n⚠️  **LARGE CHANGESET**: %d files modified. Focus on high-risk areas first.\n\n",
        len(taskCtx.GitModifiedFiles))
}
```

**TestAgent** - Add coverage hints:
```go
// In buildUserPrompt():
if len(taskCtx.GitModifiedFiles) > 0 {
    prompt += "\n📝 **Git Context - Modified Files**:\n"
    prompt += "Focus testing on these recently changed files:\n"
    for _, file := range taskCtx.GitModifiedFiles {
        if !strings.HasSuffix(file, "_test.go") {
            prompt += fmt.Sprintf("  - %s\n", file)
        }
    }
    prompt += "\n"
}
```

---

### Step 5: Branch-Aware Safety Checks

**CodeAgent** - Add precondition check:
```go
// In Execute(), after checkPreconditions():
if taskCtx.GitBranch == "master" || taskCtx.GitBranch == "main" {
    // Extra caution on main branch
    if !taskCtx.GitClean {
        return nil, fmt.Errorf(
            "refusing to modify files on %s with uncommitted changes - commit or stash first",
            taskCtx.GitBranch)
    }
}
```

---

## 🧪 Testing Strategy

### Unit Tests
```bash
# Test git context enrichment
go test -v ./agent/orchestration/... -run TestEnrichGitContext

# Test agent tool access
go test -v ./agent/agents/... -run TestCodeAgent_GitTools
```

### Integration Tests

**Test 1: Git Context Awareness**
```bash
cd /tmp/test_repo
git init
echo "test" > file.txt
git add file.txt

./wilson <<< "what files are modified?"
# Expected: Uses git_status, reports file.txt
```

**Test 2: Branch-Aware Behavior**
```bash
git checkout -b feature/test
./wilson <<< "create main.go with hello world"
# Expected: Sees branch=feature/test, allows creation

git checkout main
./wilson <<< "create main.go with hello world"
# Expected: Sees branch=main, applies stricter checks
```

**Test 3: Git Context in Prompts**
```bash
echo "// test change" >> README.md
./wilson <<< "add a new feature"
# Expected: Code agent sees README.md in modified files, mentions it in response
```

---

## 📊 Success Criteria

- ✅ TaskContext has git fields
- ✅ Manager enriches context with git status
- ✅ All agents have git tools in allowed list
- ✅ Agent prompts use git context
- ✅ Unit tests pass
- ✅ Integration test: "what files are modified?" works
- ✅ Branch-aware behavior working (stricter on main)

---

## 🚀 Implementation Checklist

### Day 1: Core Integration (6-8 hours)

**Morning (3-4 hours):**
- [ ] Add git fields to TaskContext (30min)
- [ ] Implement enrichTaskContextWithGit in Manager (1h)
- [ ] Add git tools to all agents' allowed lists (30min)
- [ ] Update agent prompts with git context (1h)
- [ ] Write unit tests (1h)

**Afternoon (3-4 hours):**
- [ ] Test git context enrichment manually (1h)
- [ ] Fix any issues found (2h)
- [ ] Verify all agents can call git tools (1h)

**Deliverable:** Git context fully integrated, agents git-aware

---

## 🎯 Expected Benefits

### Before Git Context:
- ❌ No awareness of git state
- ❌ Can't see what files are modified
- ❌ No branch-specific behavior
- ❌ May modify committed files by accident

### After Git Context:
- ✅ Full git context awareness
- ✅ Agents read modified files first
- ✅ Stricter checks on main branch
- ✅ Better context for code changes
- ✅ Branch-aware workflows

**Estimated Impact:** +30% effectiveness for real-world development

---

## 🔮 Future Enhancements

### Git Status Caching (Phase 2)
- Cache git status for 30 seconds
- Reduce overhead of repeated calls

### Smart File Detection (Phase 2)
- Warn before modifying clean files
- Auto-detect related files via imports

### Commit Message Generation (Phase 2)
- Generate smart commit messages from diffs
- Learn from project's commit history

---

## 📚 References

- **Git Tools Implementation:** `docs/improvement/GIT_TOOLS_IMPLEMENTATION.md`
- **GitHub API Integration:** `docs/improvement/GITHUB_API_INTEGRATION_PLAN.md` (Phase 2)
- **Context Analysis:** `docs/improvement/CONTEXT_AND_TOOLS_ANALYSIS.md`

---

**Status:** Ready to implement
**Blockers:** None
**Next Step:** Add git fields to TaskContext

**Owner:** Wilson Development Team
**Last Updated:** October 27, 2025
