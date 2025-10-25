# Wilson Git Integration Plan

**Date:** October 25, 2025
**Goal:** Add 7 essential Git tools to match Claude Code capabilities
**Location:** `go/capabilities/git/` (new directory)
**Impact:** +30% effectiveness for real-world usage
**Effort:** 1-2 days

---

## 📦 Tools to Implement (Priority Order)

### 1. `git_status` - See modified files ⭐ CRITICAL
**Priority:** 🔴 P0 (Do first)
**Impact:** Know what files user has changed
**Complexity:** Easy (1-2 hours)

```go
// Returns: modified, staged, untracked files
// Example output:
{
  "modified": ["go/agent/code_agent.go", "go/agent/manager_agent.go"],
  "staged": ["README.md"],
  "untracked": ["GITHUB_PLAN.md"],
  "branch": "master",
  "clean": false
}
```

### 2. `git_diff` - View file changes ⭐ HIGH
**Priority:** 🟡 P1
**Impact:** See exact changes in files
**Complexity:** Easy (1-2 hours)

```go
// Arguments: file (optional), staged (bool)
// Returns: unified diff format
// If no file specified, shows all changes
```

### 3. `git_log` - View commit history
**Priority:** 🟡 P1
**Impact:** Understand code evolution
**Complexity:** Easy (1 hour)

```go
// Arguments: max_count (default 10), file (optional)
// Returns: commit history with hash, author, date, message
```

### 4. `git_show` - Show commit details
**Priority:** 🟢 P2
**Impact:** Inspect specific commits
**Complexity:** Easy (1 hour)

```go
// Arguments: commit_hash
// Returns: commit metadata + diff
```

### 5. `git_blame` - Find who changed lines
**Priority:** 🟢 P2
**Impact:** Trace code ownership
**Complexity:** Medium (2 hours)

```go
// Arguments: file, start_line (optional), end_line (optional)
// Returns: line-by-line author, date, commit
```

### 6. `git_branch` - List/switch branches
**Priority:** 🟢 P2
**Impact:** Branch awareness
**Complexity:** Easy (1 hour)

```go
// Arguments: action ("list" | "current" | "switch"), branch_name (for switch)
// Returns: current branch or branch list
```

### 7. `git_stash` - Stash changes
**Priority:** 🔵 P3
**Impact:** Clean workspace temporarily
**Complexity:** Easy (1 hour)

```go
// Arguments: action ("save" | "pop" | "list")
// Returns: stash result or list
```

---

## 📁 File Structure

```
go/capabilities/git/
├── common.go                # Shared utilities
├── git_status.go           # ⭐ P0
├── git_diff.go             # P1
├── git_log.go              # P1
├── git_show.go             # P2
├── git_blame.go            # P2
├── git_branch.go           # P2
└── git_stash.go            # P3
```

**Pattern:** Each tool = 1 file (follows Wilson's filesystem/ structure)

---

## 🔧 Implementation Template

### File: `go/capabilities/git/common.go`
```go
package git

import (
    "os/exec"
    "strings"
)

// FindGitRoot finds the git repository root
func FindGitRoot(startPath string) (string, error) {
    cmd := exec.Command("git", "rev-parse", "--show-toplevel")
    cmd.Dir = startPath
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(output)), nil
}

// RunGitCommand executes git command in workspace
func RunGitCommand(args ...string) (string, error) {
    cmd := exec.Command("git", args...)
    output, err := cmd.CombinedOutput()
    return string(output), err
}
```

### File: `go/capabilities/git/git_status.go`
```go
package git

import (
    "context"
    "encoding/json"
    "strings"
    "wilson/core/registry"
    . "wilson/core/types"
)

type GitStatusTool struct{}

func (t *GitStatusTool) Metadata() ToolMetadata {
    return ToolMetadata{
        Name:        "git_status",
        Description: "Show git status: modified, staged, untracked files",
        Category:    "git",
        RiskLevel:   RiskSafe,
        Enabled:     true,
        Parameters:  []Parameter{}, // No parameters needed
    }
}

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // Run: git status --porcelain=v1 --branch
    output, err := RunGitCommand("status", "--porcelain=v1", "--branch")
    if err != nil {
        return "", err
    }

    // Parse output into structured format
    result := parseGitStatus(output)

    // Return JSON
    jsonBytes, _ := json.MarshalIndent(result, "", "  ")
    return string(jsonBytes), nil
}

func parseGitStatus(output string) map[string]interface{} {
    lines := strings.Split(output, "\n")
    modified := []string{}
    staged := []string{}
    untracked := []string{}
    branch := ""

    for _, line := range lines {
        if strings.HasPrefix(line, "##") {
            // Branch line
            branch = strings.TrimPrefix(line, "## ")
        } else if len(line) >= 3 {
            status := line[:2]
            file := strings.TrimSpace(line[3:])

            // Parse XY status codes
            if status[0] != ' ' && status[0] != '?' {
                staged = append(staged, file)
            }
            if status[1] == 'M' {
                modified = append(modified, file)
            }
            if status == "??" {
                untracked = append(untracked, file)
            }
        }
    }

    return map[string]interface{}{
        "branch":    branch,
        "modified":  modified,
        "staged":    staged,
        "untracked": untracked,
        "clean":     len(modified) == 0 && len(staged) == 0 && len(untracked) == 0,
    }
}

func init() {
    registry.Register(&GitStatusTool{})
}
```

---

## 🔗 Integration Points

### 1. Update TaskContext (in agent refactor)
```go
// Add to task_context.go after refactor completes:
type TaskContext struct {
    // ... existing fields ...

    // Git context (populated by manager when creating task)
    GitBranch        string   // Current branch
    GitModifiedFiles []string // From git_status
    WorkspaceRoot    string   // Git repo root
}
```

### 2. Manager Populates Git Context
```go
// In manager.go (after refactor):
func (m *ManagerAgent) enrichTaskContext(ctx *TaskContext) error {
    // Find git root
    gitRoot, err := git.FindGitRoot(ctx.ProjectPath)
    if err == nil {
        ctx.WorkspaceRoot = gitRoot

        // Get git status
        statusTool := &git.GitStatusTool{}
        result, err := statusTool.Execute(context.Background(), nil)
        if err == nil {
            // Parse and populate
            var status map[string]interface{}
            json.Unmarshal([]byte(result), &status)
            ctx.GitBranch = status["branch"].(string)
            ctx.GitModifiedFiles = toStringSlice(status["modified"])
        }
    }
    return nil
}
```

### 3. Code Agent Uses Git Context
```go
// In code_agent.go buildUserPrompt:
if len(ctx.GitModifiedFiles) > 0 {
    prompt += "\n⚠️  **Git Context**: User has uncommitted changes in:\n"
    for _, file := range ctx.GitModifiedFiles {
        prompt += fmt.Sprintf("  - %s\n", file)
    }
    prompt += "Consider these files for context.\n\n"
}
```

---

## 🎯 Phase 1 Implementation (Week 1)

### Day 1: Core Infrastructure
- [ ] Create `go/capabilities/git/` directory
- [ ] Implement `common.go` (FindGitRoot, RunGitCommand)
- [ ] Implement `git_status.go` ⭐
- [ ] Test: `go test ./capabilities/git/...`

### Day 2: Essential Tools
- [ ] Implement `git_diff.go`
- [ ] Implement `git_log.go`
- [ ] Test all tools manually
- [ ] Add to agent allowed tools

### Day 3: Integration (After agent refactor)
- [ ] Add git fields to TaskContext
- [ ] Manager enriches context with git info
- [ ] Code agent uses git context in prompts
- [ ] End-to-end test

---

## 🧪 Testing Strategy

### Unit Tests
```bash
# Test each tool individually
go test ./capabilities/git/git_status_test.go -v
go test ./capabilities/git/git_diff_test.go -v
```

### Integration Test
```bash
# Test in real git repo
cd /path/to/wilson
echo "test" >> test.txt
./wilson <<< "what files have I modified?"
# Should use git_status tool and report test.txt
```

---

## 📊 Success Criteria

- ✅ All 7 tools implemented and registered
- ✅ `git_status`, `git_diff`, `git_log` working (P0-P1)
- ✅ TaskContext enriched with git info
- ✅ Wilson aware of modified files in prompts
- ✅ No regressions in existing tests
- ✅ Git tools usable by all agents

---

## 🚀 Future Enhancements (Phase 2)

### Advanced Git Tools
- `git_commit` - Create commits
- `git_push` - Push changes
- `git_pull` - Pull updates
- `git_checkout` - Switch branches/files
- `git_merge` - Merge branches

### Smart Features
- Auto-detect workspace root on startup
- Cache git status (refresh every 30s)
- Warn before modifying committed files
- Suggest commit messages based on changes

---

**Status:** Ready to implement
**Blockers:** None - can proceed in parallel with agent refactor
**Next Step:** Create `go/capabilities/git/` and implement P0 tools
