package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	. "wilson/core/types"
)

// Registry manages all available tools
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// Global registry instance
var globalRegistry = &Registry{
	tools: make(map[string]Tool),
}

// Register adds a tool to the registry
func Register(tool Tool) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	metadata := tool.Metadata()
	globalRegistry.tools[metadata.Name] = tool
}

// GetTool retrieves a tool by name
func GetTool(name string) (Tool, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	tool, exists := globalRegistry.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}

	metadata := tool.Metadata()
	if !metadata.Enabled {
		return nil, fmt.Errorf("tool '%s' is disabled", name)
	}

	return tool, nil
}

// GetAllTools returns all registered tools
func GetAllTools() []Tool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	tools := make([]Tool, 0, len(globalRegistry.tools))
	for _, tool := range globalRegistry.tools {
		tools = append(tools, tool)
	}

	return tools
}

// GetEnabledTools returns only enabled tools
func GetEnabledTools() []Tool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	tools := make([]Tool, 0)
	for _, tool := range globalRegistry.tools {
		if tool.Metadata().Enabled {
			tools = append(tools, tool)
		}
	}

	return tools
}

// GetToolNames returns all tool names
func GetAllToolNames() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	names := make([]string, 0, len(globalRegistry.tools))
	for name := range globalRegistry.tools {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// GetToolsByCategory returns tools grouped by category
func GetToolsByCategory() map[ToolCategory][]Tool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	categorized := make(map[ToolCategory][]Tool)
	for _, tool := range globalRegistry.tools {
		metadata := tool.Metadata()
		if metadata.Enabled {
			categorized[metadata.Category] = append(categorized[metadata.Category], tool)
		}
	}

	return categorized
}

// Cached prompts (regenerated only when tools change)
var (
	cachedChatPrompt string
	cachedToolPrompt string
	promptsGenerated bool
	promptMu         sync.RWMutex
)

// GenerateChatPrompt creates a minimal chat prompt (no tools)
func GenerateChatPrompt() string {
	promptMu.RLock()
	if promptsGenerated && cachedChatPrompt != "" {
		defer promptMu.RUnlock()
		return cachedChatPrompt
	}
	promptMu.RUnlock()

	promptMu.Lock()
	defer promptMu.Unlock()

	// Double-check after acquiring write lock
	if promptsGenerated && cachedChatPrompt != "" {
		return cachedChatPrompt
	}

	var prompt strings.Builder
	prompt.WriteString("You are Wilson, a helpful AI assistant. ")
	prompt.WriteString("Be conversational, friendly, and concise. ")
	prompt.WriteString("Answer questions naturally and helpfully.\n")

	cachedChatPrompt = prompt.String()
	return cachedChatPrompt
}

// GenerateSystemPrompt creates a system prompt from all enabled tools
func GenerateSystemPrompt() string {
	promptMu.RLock()
	if promptsGenerated && cachedToolPrompt != "" {
		defer promptMu.RUnlock()
		return cachedToolPrompt
	}
	promptMu.RUnlock()

	promptMu.Lock()
	defer promptMu.Unlock()

	// Double-check after acquiring write lock
	if promptsGenerated && cachedToolPrompt != "" {
		return cachedToolPrompt
	}

	var prompt strings.Builder

	// CRITICAL RULES FIRST - Most important for preventing hallucinations
	prompt.WriteString("=== CRITICAL: TOOL EXECUTION RULES ===\n\n")

	prompt.WriteString("**RULE 1: Direct Tool Name = Immediate Tool Call**\n")
	prompt.WriteString("If the user input contains or starts with a tool name (like check_task_progress, list_files, delegate_task),\n")
	prompt.WriteString("you MUST call that tool immediately. Do NOT provide a conversational response.\n\n")
	prompt.WriteString("Example:\n")
	prompt.WriteString("  User: \"check_task_progress abc-123\"\n")
	prompt.WriteString("  ❌ WRONG: \"You want to check the progress of task abc-123. Here's what I found...\"\n")
	prompt.WriteString("  ✅ RIGHT: {\"tool\": \"check_task_progress\", \"arguments\": {\"task_id\": \"abc-123\"}}\n\n")

	prompt.WriteString("**RULE 2: Action Verbs = Tool Call**\n")
	prompt.WriteString("When user asks you to DO something (create/write/make/build/search/list/check), you MUST:\n")
	prompt.WriteString("1. Use a tool - NEVER just describe what you would do\n")
	prompt.WriteString("2. Respond with ONLY valid JSON - no text before or after\n")
	prompt.WriteString("3. Use this EXACT format: {\"tool\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n")
	prompt.WriteString("4. For multi-step tasks: Do ONE step, wait for result, then do next step\n\n")

	prompt.WriteString("HALLUCINATION PREVENTION:\n")
	prompt.WriteString("❌ WRONG: \"I'll create a file called test.go with package main...\"\n")
	prompt.WriteString("❌ WRONG: \"Here's what I did: 1. Created main.go...\"\n")
	prompt.WriteString("❌ WRONG: \"To create a Go project, run: go mod init...\"\n")
	prompt.WriteString("❌ WRONG: \"You want to check X. Here's the information...\" (without calling tool)\n")
	prompt.WriteString("✅ RIGHT: {\"tool\": \"write_file\", \"arguments\": {\"path\": \"test.go\", \"content\": \"package main\"}}\n")
	prompt.WriteString("✅ RIGHT: {\"tool\": \"check_task_progress\", \"arguments\": {\"task_id\": \"abc-123\"}}\n\n")

	prompt.WriteString("=== IDENTITY ===\n")
	prompt.WriteString("You are Wilson, a helpful assistant with access to local system tools.\n")
	prompt.WriteString("You can perform file operations, run commands, and delegate complex tasks to specialist agents.\n\n")

	// Group tools by category
	categorized := GetToolsByCategory()

	// Sort categories for consistent output
	categories := make([]ToolCategory, 0, len(categorized))
	for category := range categorized {
		categories = append(categories, category)
	}

	prompt.WriteString("Available tools:\n")
	for _, category := range categories {
		tools := categorized[category]
		for _, tool := range tools {
			metadata := tool.Metadata()
			prompt.WriteString(fmt.Sprintf("- %s: %s\n", metadata.Name, metadata.Description))

			if len(metadata.Parameters) > 0 {
				prompt.WriteString("  Parameters: ")
				paramStrs := make([]string, 0, len(metadata.Parameters))
				for _, param := range metadata.Parameters {
					req := ""
					if param.Required {
						req = " (required)"
					}
					paramStrs = append(paramStrs, fmt.Sprintf("%s: %s%s", param.Name, param.Description, req))
				}
				prompt.WriteString(strings.Join(paramStrs, ", "))
				prompt.WriteString("\n")
			}
		}
	}

	prompt.WriteString("\nTool call format (respond with ONLY this, no other text):\n")
	prompt.WriteString(`{"tool": "tool_name", "arguments": {"param": "value"}}` + "\n\n")

	prompt.WriteString("Examples:\n")
	prompt.WriteString("User: 'list files in the go directory'\n")
	prompt.WriteString(`You: {"tool": "list_files", "arguments": {"path": "go"}}` + "\n\n")
	prompt.WriteString("User: 'create a file called test.txt with hello world'\n")
	prompt.WriteString(`You: {"tool": "write_file", "arguments": {"path": "test.txt", "content": "hello world"}}` + "\n\n")
	prompt.WriteString("User: 'make a directory called myproject'\n")
	prompt.WriteString(`You: {"tool": "make_directory", "arguments": {"path": "myproject"}}` + "\n\n")
	prompt.WriteString("User: 'create a go file that prints hello'\n")
	prompt.WriteString(`You: {"tool": "write_file", "arguments": {"path": "hello.go", "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}"}}` + "\n\n")
	prompt.WriteString("User: 'find all python files'\n")
	prompt.WriteString(`You: {"tool": "search_files", "arguments": {"pattern": "*.py"}}` + "\n\n")

	prompt.WriteString("MULTI-STEP EXAMPLE:\n")
	prompt.WriteString("User: 'create main.go and a test file for it'\n")
	prompt.WriteString(`You: {"tool": "write_file", "arguments": {"path": "main.go", "content": "package main..."}}` + "\n")
	prompt.WriteString("[Tool executes]\n")
	prompt.WriteString("System: Continue with the next step\n")
	prompt.WriteString(`You: {"tool": "write_file", "arguments": {"path": "main_test.go", "content": "package main..."}}` + "\n\n")

	prompt.WriteString("If the user is just chatting and doesn't need a tool, respond normally in plain text.\n")
	prompt.WriteString("If you need clarification before acting, ask a question in plain text.\n")

	cachedToolPrompt = prompt.String()
	promptsGenerated = true
	return cachedToolPrompt
}

// InvalidatePromptCache invalidates the cached prompts (call when tools change)
func InvalidatePromptCache() {
	promptMu.Lock()
	defer promptMu.Unlock()

	cachedChatPrompt = ""
	cachedToolPrompt = ""
	promptsGenerated = false
}
