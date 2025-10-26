package chat

import (
	"strings"
)

// Intent represents the user's intent
type Intent int

const (
	// IntentChat - Simple conversational chat
	IntentChat Intent = iota
	// IntentTool - Request to use a tool
	IntentTool
	// IntentDelegate - Complex task requiring delegation to Manager Agent
	IntentDelegate
	// IntentCode - Code/project creation requiring Code Agent
	IntentCode
)

// String returns the string representation of an intent
func (i Intent) String() string {
	switch i {
	case IntentChat:
		return "chat"
	case IntentTool:
		return "tool"
	case IntentDelegate:
		return "delegate"
	case IntentCode:
		return "code"
	default:
		return "unknown"
	}
}

// Tool-related keywords
var toolKeywords = []string{
	"list", "show", "display", "get",
	"find", "search", "look for", "locate",
	"read", "open", "view", "check",
	"write", "create file", "save",
	"delete", "remove",
	"run", "execute", "compile", "test",
	"analyze", "parse", "inspect",
	"download", "fetch",
	// Task management operations
	"check_task_progress", "task progress", "check progress", "task status",
	"list tasks", "show tasks",
	// Filesystem operations (high priority - checked first)
	"mkdir", "make directory", "make dir", "make folder",
	"create directory", "create a directory", "create dir", "create a dir",
	"create folder", "create a folder",
}

// Code/project creation keywords (should go to Code Agent)
var codeKeywords = []string{
	"create project", "create a project", "new project",
	"create go file", "create .go", "create a go file",
	"create python file", "create .py",
	"create javascript", "create .js",
	"write code", "write a program", "write a script",
	"build app", "build an app", "build a app",
	"build program", "build a program",
	"make a go", "make go file",
	"go files that", "go file that",
	"create files", "create multiple files",
	"implement function", "implement a function",
	"app that", "program that", "script that", "tool that",
}

// Delegation keywords (complex multi-agent tasks)
var delegationKeywords = []string{
	"refactor", "restructure", "reorganize",
	"fix bug", "fix the bug", "fix a bug", "debug",
	"add feature", "enhance", "improve",
	"design", "architect",
	"full application", "complete system",
	"end to end", "from scratch",
}

// ClassifyIntent determines the user's intent based on their input
func ClassifyIntent(input string) Intent {
	lowerInput := strings.ToLower(input)

	// Strategy: Check for specific patterns first, then fall back to general patterns

	// 1. Check for CODE CREATION patterns (highest priority for hallucination prevention)
	for _, keyword := range codeKeywords {
		if strings.Contains(lowerInput, keyword) {
			return IntentCode
		}
	}

	// 2. Check for delegation patterns (complex multi-agent tasks)
	for _, keyword := range delegationKeywords {
		if strings.Contains(lowerInput, keyword) {
			// Don't delegate simple filesystem operations even if they match
			if !isFilesystemOperation(lowerInput) {
				return IntentDelegate
			}
		}
	}

	// 3. Check for tool keywords (simple commands)
	for _, keyword := range toolKeywords {
		if strings.Contains(lowerInput, keyword) {
			// Additional heuristics to avoid false positives
			// e.g., "I like to read books" shouldn't trigger tool intent
			if isLikelyToolRequest(lowerInput, keyword) {
				return IntentTool
			}
		}
	}

	// Default to chat
	return IntentChat
}

// isFilesystemOperation checks if the input is a simple filesystem operation
func isFilesystemOperation(input string) bool {
	fsKeywords := []string{"dir", "directory", "folder", "file", "mkdir", "create dir", "create folder", "make dir"}
	for _, kw := range fsKeywords {
		if strings.Contains(input, kw) {
			return true
		}
	}
	return false
}

// isLikelyToolRequest applies heuristics to determine if it's really a tool request
func isLikelyToolRequest(input, keyword string) bool {
	// Specific file indicators that strongly suggest tool usage
	specificIndicators := []string{
		"directory", "folder", "dir",
		".go", ".md", ".txt", ".json", ".yaml", ".py", ".js",
	}

	for _, indicator := range specificIndicators {
		if strings.Contains(input, indicator) {
			return true
		}
	}

	// Generic indicators (need more context)
	genericIndicators := []string{"file", "path", "test"}
	hasGenericIndicator := false
	for _, indicator := range genericIndicators {
		if strings.Contains(input, indicator) {
			hasGenericIndicator = true
			break
		}
	}

	// If has generic indicator AND short, it's likely a tool request
	words := strings.Fields(input)
	if hasGenericIndicator && len(words) <= 7 {
		return true
	}

	// If the input is a short command-like phrase, likely a tool request
	if len(words) <= 5 {
		return true
	}

	// Check if it's in imperative mood (command form)
	// Commands often start with the verb
	trimmedInput := strings.TrimSpace(input)
	// Handle polite requests "could you", "can you", "please"
	for _, prefix := range []string{"could you ", "can you ", "would you ", "please "} {
		if strings.HasPrefix(trimmedInput, prefix) {
			trimmedInput = strings.TrimPrefix(trimmedInput, prefix)
			trimmedInput = strings.TrimSpace(trimmedInput)
			break
		}
	}

	if strings.Index(trimmedInput, keyword) == 0 {
		return true
	}

	// If contains question words, likely asking about something (less likely to be tool)
	questionWords := []string{"what", "why", "how", "when", "where", "who"}
	for _, qw := range questionWords {
		if strings.HasPrefix(strings.TrimSpace(input), qw) {
			return false
		}
	}

	// Default: if keyword is present and not a question, likely a tool request
	return true
}
