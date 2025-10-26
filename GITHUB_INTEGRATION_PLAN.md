# Wilson GitHub Integration Plan

**Date:** October 26, 2025
**Status:** Planning Phase
**Priority:** HIGH - Production Feature
**Estimated Effort:** 3-4 days

---

## 🎯 Executive Summary

Integrate GitHub tooling into Wilson's agent system to enable:
- **Git context awareness** - Agents know what files are modified, current branch, commit history
- **GitHub API operations** - Issues, PRs, code search, repository operations via MCP
- **Smart workflows** - Branch-aware code generation, PR-based reviews, commit message generation

**Impact:** +40% effectiveness for real-world development workflows

---

## 📚 Background & Current State

### What We Have ✅

1. **Git Tools Implementation (Complete)**
   - Location: `go/capabilities/git/`
   - 7 tools: `git_status`, `git_diff`, `git_log`, `git_show`, `git_blame`, `git_branch`, `git_stash`
   - All tools tested with 21 passing unit tests
   - Self-registering via `init()`
   - Status: ✅ Ready for agent integration

2. **MCP Infrastructure (Complete)**
   - Location: `go/mcp/`
   - MCP client with tool discovery and execution
   - Auto-bridges MCP tools to Wilson's tool system
   - Format: `mcp_<server>_<tool>`
   - Status: ✅ Production-ready

3. **TaskContext (Needs Enhancement)**
   - Location: `go/agent/base/task_context.go`
   - Current fields: ProjectPath, DependencyFiles, PreviousErrors
   - Missing: GitRoot, GitBranch, GitModifiedFiles
   - Status: ⏳ Needs git fields added

4. **Agent Architecture (Refactored, Stable)**
   - Location: `go/agent/agents/`
   - BaseAgent pattern with allowed tools
   - 6 specialized agents: Chat, Code, Test, Review, Research, Analysis
   - Status: ✅ Ready for new tools

### What We're Missing 🔴

1. **Git context not injected into TaskContext**
   - Agents don't know current branch
   - Agents don't know what files are modified
   - No git repo root awareness

2. **Git tools not added to agent allowed lists**
   - Agents can't call git_status, git_diff, etc.
   - Tools exist but aren't accessible

3. **GitHub MCP server not configured**
   - No GitHub API access (issues, PRs, code search)
   - MCP infrastructure ready but GitHub server not enabled

4. **No git-aware prompts**
   - Agents don't consider git state in decision-making
   - No branch-specific behavior
   - No detection of uncommitted changes

---

## 🏗️ Architecture Design

### Phase 1: Git Context Integration (Day 1)

#### 1.1 Enhance TaskContext

**File:** `go/agent/base/task_context.go`

```go
type TaskContext struct {
    // ... existing fields ...

    // Git Context (NEW - Phase 1)
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
- `GitRoot`: Enables relative path calculations, workspace awareness
- `GitBranch`: Branch-specific behavior (e.g., stricter checks on main/master)
- `GitModifiedFiles`: Agents can read these first for context
- `GitClean`: Safety check before destructive operations
- `GitAheadBehind`: PR readiness indicator

#### 1.2 Manager Enriches Context

**File:** `go/agent/orchestration/manager.go`

```go
// Add to ExecuteTaskPlan() before executing task:
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

    // Parse ahead/behind (e.g., "ahead 2, behind 1")
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

**Call site:** In `ExecuteTaskPlan()` at line ~758, add:
```go
// Create TaskContext for fix task
taskCtx := NewTaskContext(task)

// ✅ NEW: Enrich with git context
if err := m.enrichTaskContextWithGit(taskCtx); err != nil {
    fmt.Printf("[ManagerAgent] Warning: Failed to enrich git context: %v\n", err)
}

// Load artifacts from dependent tasks and inject into context
if err := m.injectDependencyContext(task, taskCtx); err != nil {
    // ... existing code ...
}
```

#### 1.3 Add Git Tools to Agents

**Files:** `go/agent/agents/*.go` (CodeAgent, TestAgent, ReviewAgent, ChatAgent)

**CodeAgent (`code_agent.go`):** Add at line ~35
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // Git tools (PHASE 1)
    "git_status",   // Check repo state
    "git_diff",     // View file changes
    "git_log",      // View commit history
    "git_show",     // Show commit details
    "git_blame",    // Find code authors
    "git_branch",   // Branch operations
})
```

**TestAgent (`test_agent.go`):** Add git tools at line ~35
```go
base.SetAllowedTools([]string{
    // ... existing tools ...
    "git_status",   // Useful for detecting test files
    "git_diff",     // Useful for seeing what code changed
    "git_log",      // Useful for understanding test history
})
```

**ReviewAgent (`review_agent.go`):** Add git tools at line ~30
```go
base.SetAllowedTools([]string{
    // ... existing tools ...
    "git_status",   // Check what's being reviewed
    "git_diff",     // Critical for reviewing changes
    "git_log",      // Understand change history
    "git_show",     // Inspect specific commits
    "git_blame",    // Find code ownership
})
```

**ChatAgent (`chat_agent.go`):** Add git tools at line ~26
```go
baseAgent.SetAllowedTools([]string{
    // ... existing tools ...
    "git_status",   // Answer "what files did I change?"
    "git_log",      // Answer "what were my recent commits?"
    "git_branch",   // Answer "what branch am I on?"
})
```

#### 1.4 Update Agent Prompts

**CodeAgent (`code_agent.go`):** Add to `buildUserPrompt()` at line ~560

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

**ReviewAgent (`review_agent.go`):** Add git-aware review prompts

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

**TestAgent (`test_agent.go`):** Add test coverage hints

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

### Phase 2: GitHub MCP Integration (Day 2)

#### 2.1 Install GitHub MCP Server

**Prerequisites:**
- GitHub Personal Access Token (PAT) with repo access
- Node.js installed (for @modelcontextprotocol/server-github)

**Installation:**
```bash
# Install GitHub MCP server globally
npm install -g @modelcontextprotocol/server-github

# Set up environment variable
export GITHUB_PERSONAL_ACCESS_TOKEN="ghp_xxxxxxxxxxxx"
```

**Configuration:** `~/.wilson/mcp_config.json`

```json
{
  "servers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

#### 2.2 Enable GitHub Tools in Agents

**Expected MCP tools from GitHub server:**
- `mcp_github_create_issue`
- `mcp_github_create_pull_request`
- `mcp_github_list_issues`
- `mcp_github_get_issue`
- `mcp_github_search_repositories`
- `mcp_github_get_file_contents`
- `mcp_github_push_files`
- `mcp_github_create_branch`
- `mcp_github_search_code`

**Add to CodeAgent (`code_agent.go`):**
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // GitHub MCP tools (PHASE 2)
    "mcp_github_create_pull_request", // Create PR from code changes
    "mcp_github_create_branch",       // Create feature branches
    "mcp_github_search_code",         // Search codebase
    "mcp_github_get_file_contents",   // Read files from GitHub
})
```

**Add to ReviewAgent (`review_agent.go`):**
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // GitHub MCP tools (PHASE 2)
    "mcp_github_get_issue",           // Read issue requirements
    "mcp_github_create_pull_request", // Submit reviewed code as PR
    "mcp_github_list_issues",         // Find related issues
})
```

**Add to ChatAgent (`chat_agent.go`):**
```go
baseAgent.SetAllowedTools([]string{
    // ... existing tools ...

    // GitHub MCP tools (PHASE 2)
    "mcp_github_list_issues",         // Answer "what are my issues?"
    "mcp_github_create_issue",        // Create issues from chat
    "mcp_github_search_repositories", // Search GitHub
})
```

#### 2.3 Add GitHub Context to TaskContext (Optional)

**File:** `go/agent/base/task_context.go`

```go
type TaskContext struct {
    // ... existing fields ...

    // GitHub Context (PHASE 2 - optional)
    GitHubRepo       string // e.g., "owner/repo"
    GitHubIssue      string // Related issue number
    GitHubPR         string // Related PR number
    GitHubRemoteURL  string // Git remote URL
}
```

**Populate in Manager:**
```go
// Parse remote URL to extract GitHub repo
func (m *ManagerAgent) enrichTaskContextWithGitHub(taskCtx *base.TaskContext) error {
    if taskCtx.GitRoot == "" {
        return nil // Not a git repo
    }

    // Get remote URL: git config --get remote.origin.url
    remoteURL, err := git.RunGitCommandInDir(taskCtx.GitRoot, "config", "--get", "remote.origin.url")
    if err != nil {
        return nil // No remote configured
    }

    taskCtx.GitHubRemoteURL = strings.TrimSpace(remoteURL)

    // Parse GitHub repo from URL
    // https://github.com/owner/repo.git -> owner/repo
    // git@github.com:owner/repo.git -> owner/repo
    if strings.Contains(remoteURL, "github.com") {
        repo := extractGitHubRepo(remoteURL)
        taskCtx.GitHubRepo = repo
        fmt.Printf("[Manager] GitHub repo detected: %s\n", repo)
    }

    return nil
}

func extractGitHubRepo(url string) string {
    // Remove .git suffix
    url = strings.TrimSuffix(url, ".git")

    // Handle HTTPS: https://github.com/owner/repo
    if strings.HasPrefix(url, "https://github.com/") {
        return strings.TrimPrefix(url, "https://github.com/")
    }

    // Handle SSH: git@github.com:owner/repo
    if strings.HasPrefix(url, "git@github.com:") {
        return strings.TrimPrefix(url, "git@github.com:")
    }

    return ""
}
```

---

### Phase 3: Smart Workflows (Day 3)

#### 3.1 Branch-Aware Code Generation

**CodeAgent:** Check branch before generating code

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

#### 3.2 Commit Message Generation

**New Tool:** `go/capabilities/git/git_commit_message.go`

```go
package git

import (
    "context"
    "fmt"
    "strings"

    "wilson/llm"
    "wilson/core/registry"
    . "wilson/core/types"
)

type GitCommitMessageTool struct {
    llmManager *llm.Manager
}

func (t *GitCommitMessageTool) Metadata() ToolMetadata {
    return ToolMetadata{
        Name:        "generate_commit_message",
        Description: "Generate a commit message based on git diff",
        Category:    "git",
        RiskLevel:   RiskSafe,
        Enabled:     true,
    }
}

func (t *GitCommitMessageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // Get staged changes
    diff, err := RunGitCommand("diff", "--cached")
    if err != nil || diff == "" {
        diff, err = RunGitCommand("diff")
        if err != nil {
            return "", fmt.Errorf("no changes to commit")
        }
    }

    // Truncate diff if too long (> 2000 chars)
    if len(diff) > 2000 {
        diff = diff[:2000] + "\n... (truncated)"
    }

    // Call LLM to generate commit message
    prompt := fmt.Sprintf(`Analyze this git diff and generate a concise commit message.

Rules:
- Start with imperative verb (Add, Fix, Update, Remove, Refactor)
- Max 50 chars for subject line
- Add body if changes are complex
- Follow conventional commits format

Diff:
%s

Generate commit message:`, diff)

    req := llm.Request{
        Messages: []llm.Message{
            {Role: "user", Content: prompt},
        },
    }

    resp, err := t.llmManager.Generate(ctx, llm.PurposeChat, req)
    if err != nil {
        return "", fmt.Errorf("LLM generation failed: %w", err)
    }

    return strings.TrimSpace(resp.Content), nil
}

func init() {
    registry.Register(&GitCommitMessageTool{})
}
```

#### 3.3 PR Creation Workflow

**ReviewAgent:** Auto-create PR after successful review

```go
// In Execute(), after review passes:
if taskCtx.GitHubRepo != "" && taskCtx.GitBranch != "master" && taskCtx.GitBranch != "main" {
    // Suggest creating PR
    prompt := fmt.Sprintf(`Code review passed. GitHub repo detected: %s.

Would you like to create a pull request?

If yes, generate PR title and description based on:
- Branch: %s
- Modified files: %v
- Review findings: %s

Then call: mcp_github_create_pull_request`,
        taskCtx.GitHubRepo,
        taskCtx.GitBranch,
        taskCtx.GitModifiedFiles,
        reviewSummary)
}
```

#### 3.4 Issue-Driven Development

**ChatAgent:** Link tasks to GitHub issues

```go
// In Execute(), detect issue references in user input:
func detectIssueReference(userInput string) string {
    // Match patterns: #123, GH-123, issue 123
    patterns := []string{`#(\d+)`, `GH-(\d+)`, `issue\s+(\d+)`}

    for _, pattern := range patterns {
        re := regexp.MustCompile(pattern)
        if matches := re.FindStringSubmatch(userInput); len(matches) > 1 {
            return matches[1]
        }
    }
    return ""
}

// If issue detected, fetch details:
if issueNum := detectIssueReference(task.Description); issueNum != "" {
    // Call mcp_github_get_issue to fetch requirements
    // Inject into task context
    taskCtx.GitHubIssue = issueNum
}
```

---

### Phase 4: Advanced Features (Day 4)

#### 4.1 Git Status Caching

**Problem:** Calling `git status` every task is expensive

**Solution:** Cache git status, refresh every 30 seconds

```go
// In manager.go:
type gitStatusCache struct {
    status    map[string]interface{}
    gitRoot   string
    timestamp time.Time
    mu        sync.RWMutex
}

var globalGitCache = &gitStatusCache{}

func (m *ManagerAgent) getCachedGitStatus(gitRoot string) (map[string]interface{}, error) {
    globalGitCache.mu.RLock()

    // Check if cache is valid (< 30 seconds old, same repo)
    if globalGitCache.gitRoot == gitRoot &&
        time.Since(globalGitCache.timestamp) < 30*time.Second {
        status := globalGitCache.status
        globalGitCache.mu.RUnlock()
        return status, nil
    }
    globalGitCache.mu.RUnlock()

    // Refresh cache
    statusTool := &git.GitStatusTool{}
    result, err := statusTool.Execute(context.Background(), map[string]interface{}{
        "path": gitRoot,
    })
    if err != nil {
        return nil, err
    }

    var status map[string]interface{}
    if err := json.Unmarshal([]byte(result), &status); err != nil {
        return nil, err
    }

    // Update cache
    globalGitCache.mu.Lock()
    globalGitCache.status = status
    globalGitCache.gitRoot = gitRoot
    globalGitCache.timestamp = time.Now()
    globalGitCache.mu.Unlock()

    return status, nil
}
```

#### 4.2 Smart File Modification Detection

**Problem:** Agents might modify committed files by accident

**Solution:** Warn before modifying clean files

```go
// In code_agent.go, checkPreconditions():
func (a *CodeAgent) shouldWarnBeforeModifying(filePath string, taskCtx *base.TaskContext) bool {
    // Don't warn if file is already modified
    for _, modified := range taskCtx.GitModifiedFiles {
        if strings.HasSuffix(modified, filePath) {
            return false // Already modified, safe to edit
        }
    }

    // Don't warn if file is staged
    for _, staged := range taskCtx.GitStagedFiles {
        if strings.HasSuffix(staged, filePath) {
            return false
        }
    }

    // Don't warn if file is untracked (new file)
    for _, untracked := range taskCtx.GitUntrackedFiles {
        if strings.HasSuffix(untracked, filePath) {
            return false
        }
    }

    // File is clean (committed) - warn!
    return true
}
```

#### 4.3 Automatic .gitignore Management

**New Tool:** `go/capabilities/git/git_ignore.go`

```go
func (t *GitIgnoreTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    pattern := args["pattern"].(string)
    gitRoot := args["git_root"].(string)

    gitignorePath := filepath.Join(gitRoot, ".gitignore")

    // Read existing .gitignore
    content, err := os.ReadFile(gitignorePath)
    if err != nil && !os.IsNotExist(err) {
        return "", err
    }

    // Check if pattern already exists
    if strings.Contains(string(content), pattern) {
        return fmt.Sprintf("Pattern '%s' already in .gitignore", pattern), nil
    }

    // Append pattern
    f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return "", err
    }
    defer f.Close()

    if _, err := f.WriteString(pattern + "\n"); err != nil {
        return "", err
    }

    return fmt.Sprintf("Added '%s' to .gitignore", pattern), nil
}
```

---

## 🧪 Testing Strategy

### Unit Tests

```bash
# Test git tools
go test -v ./capabilities/git/...

# Test TaskContext enrichment
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

**Test 3: GitHub Integration**
```bash
# Set up GitHub token
export GITHUB_PERSONAL_ACCESS_TOKEN="ghp_..."

./wilson <<< "list my open issues in wilson repository"
# Expected: Uses mcp_github_list_issues, shows issues
```

### End-to-End Workflow Test

**Scenario:** Fix issue via GitHub

```bash
./wilson <<< "Fix issue #42 in the authentication module"

# Expected workflow:
# 1. ChatAgent detects issue #42
# 2. Calls mcp_github_get_issue to fetch requirements
# 3. Creates task with GitHub context
# 4. Manager enriches with git context (branch, modified files)
# 5. CodeAgent reads modified files using git_diff
# 6. CodeAgent implements fix
# 7. TestAgent runs tests
# 8. ReviewAgent reviews code
# 9. ReviewAgent suggests creating PR
# 10. User approves, PR created via mcp_github_create_pull_request
```

---

## 📊 Success Criteria

### Phase 1: Git Context (Day 1)
- ✅ TaskContext has git fields
- ✅ Manager enriches context with git status
- ✅ All agents have git tools in allowed list
- ✅ Agent prompts use git context
- ✅ Unit tests pass
- ✅ Integration test: "what files are modified?" works

### Phase 2: GitHub MCP (Day 2)
- ✅ GitHub MCP server installed and configured
- ✅ MCP tools appear in registry (mcp_github_*)
- ✅ Agents can call GitHub tools
- ✅ Integration test: list issues works
- ✅ Integration test: create PR works

### Phase 3: Smart Workflows (Day 3)
- ✅ Branch-aware code generation
- ✅ Commit message generation tool
- ✅ PR creation workflow
- ✅ Issue-driven development
- ✅ End-to-end test passes

### Phase 4: Advanced Features (Day 4)
- ✅ Git status caching (30s TTL)
- ✅ Smart file modification warnings
- ✅ .gitignore management
- ✅ Performance benchmarks (< 50ms overhead)

---

## 🚀 Implementation Plan

### Day 1: Git Context Integration (6-8 hours)

**Morning (3-4 hours):**
1. Add git fields to TaskContext (30min)
2. Implement enrichTaskContextWithGit in Manager (1h)
3. Add git tools to all agents' allowed lists (30min)
4. Update agent prompts with git context (1h)
5. Write unit tests (1h)

**Afternoon (3-4 hours):**
6. Test git context enrichment manually (1h)
7. Fix any issues found (2h)
8. Verify all agents can call git tools (1h)

**Deliverable:** Git context fully integrated, agents git-aware

---

### Day 2: GitHub MCP Integration (6-8 hours)

**Morning (3-4 hours):**
1. Install GitHub MCP server (30min)
2. Configure MCP with GitHub token (30min)
3. Test MCP tool discovery (30min)
4. Add GitHub context fields to TaskContext (30min)
5. Implement enrichTaskContextWithGitHub (1h)
6. Add GitHub tools to agent allowed lists (30min)

**Afternoon (3-4 hours):**
7. Test GitHub tool calls manually (1h)
8. Create integration tests (1h)
9. Debug and fix issues (1-2h)

**Deliverable:** GitHub API accessible via MCP, agents can create issues/PRs

---

### Day 3: Smart Workflows (6-8 hours)

**Morning (3-4 hours):**
1. Implement branch-aware code generation (1h)
2. Create commit message generation tool (1.5h)
3. Implement PR creation workflow (1h)
4. Test workflows manually (30min)

**Afternoon (3-4 hours):**
5. Implement issue-driven development (1h)
6. Create end-to-end test (1h)
7. Debug workflow issues (1-2h)

**Deliverable:** Complete GitHub-integrated workflows working

---

### Day 4: Advanced Features + Polish (6-8 hours)

**Morning (3-4 hours):**
1. Implement git status caching (1h)
2. Add file modification warnings (1h)
3. Create .gitignore management tool (1h)

**Afternoon (3-4 hours):**
4. Performance benchmarks (1h)
5. Documentation updates (1h)
6. Final testing and bug fixes (1-2h)

**Deliverable:** Production-ready GitHub integration

---

## 🎯 Expected Benefits

### Before GitHub Integration:
- ❌ No awareness of git state
- ❌ Can't see what files are modified
- ❌ No branch-specific behavior
- ❌ Manual GitHub operations
- ❌ No issue tracking integration

### After GitHub Integration:
- ✅ Full git context awareness
- ✅ Agents read modified files first
- ✅ Stricter checks on main branch
- ✅ Automatic PR creation
- ✅ Issue-driven task planning
- ✅ Smart commit messages
- ✅ GitHub code search
- ✅ Branch-aware workflows

**Estimated Impact:**
- +40% effectiveness for real-world development
- +50% for teams using GitHub workflows
- +30% reduction in manual git operations
- +25% better code quality (branch-aware checks)

---

## 🔮 Future Enhancements (Phase 5+)

### Write Operations (Requires User Confirmation)
- `git_commit` - Create commits
- `git_push` - Push changes
- `git_pull` - Pull updates
- `git_merge` - Merge branches
- `git_rebase` - Rebase branches

### Advanced GitHub Features
- Automatic PR reviews (comment on PRs)
- GitHub Actions integration
- Release management
- Code owners awareness
- GitHub Projects integration

### Multi-Repository Support
- Cross-repo code search
- Dependency updates across repos
- Monorepo-aware workflows

---

## 📝 Configuration Examples

### ~/.wilson/config.yaml (Git Settings)

```yaml
git:
  # Cache git status for this many seconds
  status_cache_ttl: 30

  # Warn before modifying clean files
  warn_clean_file_edits: true

  # Protected branches (extra checks)
  protected_branches:
    - master
    - main
    - production

  # Auto-generate commit messages
  auto_commit_messages: true
```

### ~/.wilson/mcp_config.json (GitHub Setup)

```json
{
  "servers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      },
      "enabled": true
    }
  }
}
```

---

## ⚠️ Risk Mitigation

### Risk 1: GitHub API Rate Limits
**Mitigation:** Cache frequently accessed data, use conditional requests

### Risk 2: Sensitive Data Exposure
**Mitigation:** Never log GitHub tokens, use environment variables only

### Risk 3: Destructive Git Operations
**Mitigation:** All write operations require user confirmation (Phase 5+)

### Risk 4: MCP Server Failures
**Mitigation:** Graceful degradation - agents work without GitHub if server fails

---

## 📚 References

- **Git Tools Status:** `docs/improvement/GIT_TOOLS_STATUS.md`
- **GitHub Plan (Original):** `docs/improvement/GITHUB_PLAN.md`
- **MCP Setup:** `docs/MCP_SETUP.md` (if exists)
- **Context Analysis:** `docs/improvement/CONTEXT_AND_TOOLS_ANALYSIS.md`
- **Agent Refactor:** `docs/improvement/AGENT_STRUCTURE_PROPOSAL.md`

---

**Status:** Ready to implement
**Blockers:** None
**Next Step:** Start Phase 1 - Add git fields to TaskContext

**Owner:** Wilson Development Team
**Reviewers:** TBD
**Last Updated:** October 26, 2025
