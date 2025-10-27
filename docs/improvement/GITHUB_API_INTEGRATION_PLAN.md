# Wilson GitHub API Integration Plan

**Date:** October 27, 2025
**Status:** Future Work (After Git Context Integration)
**Priority:** MEDIUM - Nice to Have
**Estimated Effort:** 2-3 days
**Dependencies:**
- Git context integration complete (see GIT_CONTEXT_INTEGRATION_PLAN.md)
- MCP infrastructure ready

---

## 🎯 Goal

Integrate GitHub API operations into Wilson via MCP (Model Context Protocol) to enable:
- Create and manage issues
- Create pull requests
- Search code across GitHub
- Read issue requirements
- Automated PR workflows

**Impact:** +20% effectiveness for GitHub-based workflows

---

## 📚 Background

### What We Have ✅
1. **MCP Infrastructure** - Client with tool discovery and execution (`go/mcp/`)
2. **Git Context** - Agents know branch, modified files (after Phase 1)
3. **Agent Architecture** - Ready for MCP tools

### What's Missing 🔴
1. GitHub MCP server not configured
2. GitHub tools not added to agent allowed lists
3. No GitHub-aware workflows

---

## 🏗️ Phase 1: GitHub MCP Setup

### Install GitHub MCP Server

**Prerequisites:**
- GitHub Personal Access Token (PAT) with repo access
- Node.js installed

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
      },
      "enabled": true
    }
  }
}
```

### Expected MCP Tools

Once configured, these tools will auto-register as `mcp_github_*`:
- `mcp_github_create_issue`
- `mcp_github_create_pull_request`
- `mcp_github_list_issues`
- `mcp_github_get_issue`
- `mcp_github_search_repositories`
- `mcp_github_get_file_contents`
- `mcp_github_push_files`
- `mcp_github_create_branch`
- `mcp_github_search_code`

---

## 🏗️ Phase 2: Add GitHub Tools to Agents

### CodeAgent
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // GitHub MCP tools
    "mcp_github_create_pull_request", // Create PR from code changes
    "mcp_github_create_branch",       // Create feature branches
    "mcp_github_search_code",         // Search codebase
    "mcp_github_get_file_contents",   // Read files from GitHub
})
```

### ReviewAgent
```go
base.SetAllowedTools([]string{
    // ... existing tools ...

    // GitHub MCP tools
    "mcp_github_get_issue",           // Read issue requirements
    "mcp_github_create_pull_request", // Submit reviewed code as PR
    "mcp_github_list_issues",         // Find related issues
})
```

### ChatAgent
```go
baseAgent.SetAllowedTools([]string{
    // ... existing tools ...

    // GitHub MCP tools
    "mcp_github_list_issues",         // Answer "what are my issues?"
    "mcp_github_create_issue",        // Create issues from chat
    "mcp_github_search_repositories", // Search GitHub
})
```

---

## 🏗️ Phase 3: GitHub Context in TaskContext

**File:** `go/agent/base/task_context.go`

**Add optional GitHub fields:**
```go
type TaskContext struct {
    // ... existing fields ...

    // GitHub Context (OPTIONAL)
    GitHubRepo       string // e.g., "owner/repo"
    GitHubIssue      string // Related issue number
    GitHubPR         string // Related PR number
    GitHubRemoteURL  string // Git remote URL
}
```

**Populate in Manager:**
```go
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

## 🏗️ Phase 4: Smart Workflows

### PR Creation Workflow

**ReviewAgent** - Auto-create PR after successful review:
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

### Issue-Driven Development

**ChatAgent** - Link tasks to GitHub issues:
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

## 🧪 Testing Strategy

### Integration Tests

**Test 1: GitHub MCP Setup**
```bash
# Set up GitHub token
export GITHUB_PERSONAL_ACCESS_TOKEN="ghp_..."

# Start Wilson, check MCP tools registered
./wilson
# Expected: mcp_github_* tools appear in registry
```

**Test 2: List Issues**
```bash
./wilson <<< "list my open issues in wilson repository"
# Expected: Uses mcp_github_list_issues, shows issues
```

**Test 3: Create PR**
```bash
# Make changes on feature branch
git checkout -b feature/test
echo "test" > test.txt
git add . && git commit -m "test"

./wilson <<< "create a pull request for these changes"
# Expected: Uses mcp_github_create_pull_request, creates PR
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
# 5. CodeAgent implements fix
# 6. TestAgent runs tests
# 7. ReviewAgent reviews code
# 8. ReviewAgent suggests creating PR
# 9. User approves, PR created via mcp_github_create_pull_request
```

---

## 📊 Success Criteria

### Phase 1: GitHub MCP Setup
- ✅ GitHub MCP server installed and configured
- ✅ MCP tools appear in registry (mcp_github_*)
- ✅ Agents can call GitHub tools
- ✅ Integration test: list issues works

### Phase 2: GitHub Tools in Agents
- ✅ CodeAgent has GitHub tools
- ✅ ReviewAgent has GitHub tools
- ✅ ChatAgent has GitHub tools
- ✅ Integration test: create PR works

### Phase 3: GitHub Context
- ✅ TaskContext has GitHub fields
- ✅ Manager detects GitHub repo
- ✅ GitHub repo info available to agents

### Phase 4: Smart Workflows
- ✅ PR creation workflow
- ✅ Issue-driven development
- ✅ End-to-end test passes

---

## 🎯 Expected Benefits

### Before GitHub Integration:
- ❌ Manual GitHub operations
- ❌ No issue tracking integration
- ❌ Manual PR creation
- ❌ No GitHub code search

### After GitHub Integration:
- ✅ Automatic PR creation
- ✅ Issue-driven task planning
- ✅ GitHub code search
- ✅ Issue requirement fetching
- ✅ Branch-aware PR workflows

**Estimated Impact:** +20% effectiveness for GitHub-based teams

---

## 🚀 Implementation Plan

### Day 1: MCP Setup (3-4 hours)
1. Install GitHub MCP server (30min)
2. Configure MCP with GitHub token (30min)
3. Test MCP tool discovery (30min)
4. Add GitHub tools to agent allowed lists (1h)
5. Test GitHub tool calls manually (1h)

### Day 2: Context Integration (3-4 hours)
1. Add GitHub context fields to TaskContext (30min)
2. Implement enrichTaskContextWithGitHub (1h)
3. Test GitHub repo detection (30min)
4. Create integration tests (1h)

### Day 3: Smart Workflows (4-5 hours)
1. Implement PR creation workflow (1.5h)
2. Implement issue-driven development (1h)
3. Create end-to-end test (1h)
4. Debug workflow issues (1h)
5. Documentation updates (30min)

---

## ⚠️ Risk Mitigation

### Risk 1: GitHub API Rate Limits
**Mitigation:** Cache frequently accessed data, use conditional requests

### Risk 2: Sensitive Data Exposure
**Mitigation:** Never log GitHub tokens, use environment variables only

### Risk 3: MCP Server Failures
**Mitigation:** Graceful degradation - agents work without GitHub if server fails

---

## 🔮 Future Enhancements (Phase 5+)

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

### ~/.wilson/config.yaml (GitHub Settings)
```yaml
github:
  # Auto-create PRs after successful reviews
  auto_create_pr: true

  # Fetch issue requirements automatically
  auto_fetch_issues: true

  # Default PR base branch
  default_base_branch: "main"
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

## 📚 References

- **Git Context Integration:** `docs/improvement/GIT_CONTEXT_INTEGRATION_PLAN.md` (Phase 1 prerequisite)
- **Git Tools Implementation:** `docs/improvement/GIT_TOOLS_IMPLEMENTATION.md`
- **MCP Setup:** `docs/MCP_SETUP.md`
- **GitHub MCP Server:** https://github.com/modelcontextprotocol/servers/tree/main/src/github

---

**Status:** Future Work
**Blockers:** Git context integration must be complete first
**Next Step:** Wait for Phase 1 completion, then install GitHub MCP server

**Owner:** Wilson Development Team
**Last Updated:** October 27, 2025
