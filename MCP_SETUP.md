# MCP Server Setup Guide

This guide shows how to enable and configure popular MCP servers with Wilson.

---

## Quick Start

### 1. Filesystem (Already Enabled)

✅ **No API keys needed** - Works out of the box!

**Tools:** read_file, write_file, list_directory, directory_tree, and 10 more

**Status:** Enabled by default in `go/config/tools.yaml`

---

## Adding External Integrations

### 2. GitHub Integration

**What you get:** Create issues, list repos, manage PRs, search code

**Setup:**

1. **Get GitHub Personal Access Token:**
   - Go to https://github.com/settings/tokens
   - Click "Generate new token (classic)"
   - Select scopes: `repo`, `read:org`, `write:discussion`
   - Copy the token

2. **Set environment variable:**
   ```bash
   # Add to ~/.zshrc or ~/.bashrc
   export GITHUB_TOKEN="ghp_your_token_here"

   # Reload shell
   source ~/.zshrc
   ```

3. **Enable in Wilson:**
   ```yaml
   # go/config/tools.yaml
   mcp:
     servers:
       github:
         enabled: true  # Change from false
   ```

4. **Restart Wilson**

**Available tools:**
- `mcp_github_create_issue` - Create GitHub issue
- `mcp_github_list_repos` - List your repositories
- `mcp_github_get_pull_request` - Get PR details
- `mcp_github_create_pull_request` - Create new PR
- And more!

---

### 3. Postgres Database

**What you get:** Query databases, list tables, describe schemas

**Setup:**

1. **Get database URL:**
   ```
   postgresql://user:password@localhost:5432/dbname
   ```

2. **Set environment variable:**
   ```bash
   export DATABASE_URL="postgresql://user:password@localhost:5432/mydb"
   ```

3. **Enable in Wilson:**
   ```yaml
   # go/config/tools.yaml
   mcp:
     servers:
       postgres:
         enabled: true
   ```

4. **Restart Wilson**

**Available tools:**
- `mcp_postgres_query` - Run SQL queries
- `mcp_postgres_list_tables` - List database tables
- `mcp_postgres_describe_table` - Get table schema
- And more!

**Example usage:**
```
You: "Show me all tables in the database"
Wilson: [Uses mcp_postgres_list_tables]
Wilson: "Found 15 tables: users, posts, comments..."
```

---

### 4. Slack Integration

**What you get:** Send messages, read channels, manage workspace

**Setup:**

1. **Create Slack App:**
   - Go to https://api.slack.com/apps
   - Click "Create New App" → "From scratch"
   - Name: "Wilson Bot"
   - Select your workspace

2. **Add Bot Token Scopes:**
   - OAuth & Permissions → Bot Token Scopes
   - Add: `chat:write`, `channels:read`, `channels:history`, `users:read`

3. **Install to Workspace:**
   - Click "Install to Workspace"
   - Copy the "Bot User OAuth Token" (starts with `xoxb-`)

4. **Get Team ID:**
   - Go to your Slack workspace
   - Click workspace name → Settings & Administration → Workspace Settings
   - Team ID is in the URL: `https://app.slack.com/client/{TEAM_ID}`

5. **Set environment variables:**
   ```bash
   export SLACK_BOT_TOKEN="xoxb-your-token"
   export SLACK_TEAM_ID="T01234ABCD"
   ```

6. **Enable in Wilson:**
   ```yaml
   # go/config/tools.yaml
   mcp:
     servers:
       slack:
         enabled: true
   ```

7. **Restart Wilson**

**Available tools:**
- `mcp_slack_send_message` - Send message to channel
- `mcp_slack_list_channels` - List all channels
- `mcp_slack_get_channel_history` - Read channel messages
- And more!

---

### 5. Memory (Persistent Storage)

**What you get:** Key-value storage that persists across sessions

**Setup:**

✅ **No API keys needed!**

```yaml
# go/config/tools.yaml
mcp:
  servers:
    memory:
      enabled: true
```

**Available tools:**
- `mcp_memory_store` - Store key-value pair
- `mcp_memory_recall` - Retrieve value by key
- `mcp_memory_list` - List all stored memories
- `mcp_memory_delete` - Remove memory

**Example usage:**
```
You: "Remember that my favorite color is blue"
Wilson: [Uses mcp_memory_store]
Wilson: "I'll remember that!"

You: "What's my favorite color?"
Wilson: [Uses mcp_memory_recall]
Wilson: "Your favorite color is blue"
```

---

## Multiple Servers at Once

You can enable multiple MCP servers simultaneously:

```yaml
mcp:
  enabled: true
  servers:
    filesystem:
      enabled: true
    github:
      enabled: true
    memory:
      enabled: true
    postgres:
      enabled: true
```

Wilson will connect to all enabled servers on startup.

---

## Troubleshooting

### Server fails to connect

**Check logs:**
```
[MCP] Failed to connect to server 'github': ...
```

**Common issues:**
1. **Missing API key** - Ensure environment variable is set
2. **npx not found** - Install Node.js: `brew install node`
3. **Wrong package name** - Check https://github.com/modelcontextprotocol/servers
4. **Network issues** - First run downloads from npm, needs internet

### Verify environment variables

```bash
echo $GITHUB_TOKEN
echo $DATABASE_URL
echo $SLACK_BOT_TOKEN
```

Should print your values (not empty).

### Test single server

Disable all servers except the one you're debugging:

```yaml
mcp:
  servers:
    filesystem:
      enabled: false
    github:
      enabled: true  # Only test this one
```

---

### 6. Telegram (Chat with Wilson from Your Phone!)

**What you get:** Control Wilson from Telegram, get responses on your phone

**Setup:**

✅ **No API keys needed if you just want to send commands!**

**But for full integration (Wilson can message you back):**

1. **Create Telegram Bot:**
   - Open Telegram and search for `@BotFather`
   - Send `/newbot`
   - Choose a name: "Wilson Assistant"
   - Choose a username: `your_wilson_bot` (must end in `bot`)
   - BotFather will give you a token like: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`

2. **Set environment variable:**
   ```bash
   export TELEGRAM_BOT_TOKEN="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
   ```

3. **Enable in Wilson:**
   ```yaml
   # go/config/tools.yaml
   mcp:
     servers:
       telegram:
         enabled: true
   ```

4. **Start Wilson** (keep it running)

5. **Chat with your bot:**
   - Open Telegram
   - Search for your bot: `@your_wilson_bot`
   - Send: `/start`
   - Try: "List files in my home directory"

**Available tools:**
- `mcp_telegram_send_message` - Send message to chat
- `mcp_telegram_get_updates` - Receive messages (polling)
- `mcp_telegram_answer_callback_query` - Handle button presses
- `mcp_telegram_edit_message` - Edit sent messages
- And more!

**How it works:**
```
You (Telegram) → Bot → Wilson (running on your computer)
Wilson processes → Sends response → Bot → You (Telegram)
```

**Example usage:**
```
You (Telegram): "What's the status of my projects?"
Wilson: [Uses filesystem tools to check]
Wilson → Telegram: "Found 5 active projects..."

You (Telegram): "Create a GitHub issue for the bug"
Wilson: [Uses mcp_github_create_issue]
Wilson → Telegram: "Issue #42 created"
```

**Pro tips:**
- Wilson must be running on your computer to respond
- Messages are processed in real-time
- You can use all Wilson's capabilities remotely
- Great for: checking logs, running commands, getting updates

**Security note:**
- Only you can message your bot by default
- Add allowed users in bot settings if needed
- Don't share your bot token

---

## Finding More MCP Servers

**Official servers:**
https://github.com/modelcontextprotocol/servers

**Popular community servers:**
- `@modelcontextprotocol/server-google-drive` - Google Drive access
- `@modelcontextprotocol/server-sqlite` - SQLite database
- `@modelcontextprotocol/server-puppeteer` - Web scraping
- `@modelcontextprotocol/server-sequential-thinking` - Multi-step reasoning
- `@modelcontextprotocol/server-brave-search` - Web search
- And many more!

**To add a new server:**

1. Find the npm package name
2. Add to `go/config/tools.yaml`:
   ```yaml
   your_server:
     name: "your_server"
     command: "npx"
     args: ["-y", "@scope/package-name", "arg1", "arg2"]
     enabled: true
     env:
       API_KEY: "${YOUR_API_KEY}"  # If needed
   ```
3. Restart Wilson

---

## Security Notes

**API Keys:**
- ✅ Store in environment variables (not in config files)
- ✅ Use `${VARIABLE_NAME}` syntax in config
- ❌ Never commit API keys to git
- ❌ Never hardcode keys in tools.yaml

**Database Access:**
- Use read-only database users when possible
- Limit access to specific databases/schemas
- Monitor queries via Wilson's audit log (`.wilson/audit.log`)

**Workspace Access:**
- Filesystem server only accesses paths you specify
- MCP servers respect their own security models
- Wilson's audit log tracks all tool usage

---

## Next Steps

1. Enable servers you need in `go/config/tools.yaml`
2. Set up required API keys as environment variables
3. Restart Wilson
4. Check logs for `[MCP] Successfully connected to server 'name'`
5. Use `-help` in Wilson to see available tools

**Questions?** Check [MCP_IMPLEMENTATION_PLAN.md](MCP_IMPLEMENTATION_PLAN.md) for architecture details.
