package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/ollama"
	"wilson/session"
)

// ChatHandler handles direct chat interactions (not task-based)
type ChatHandler struct {
	agent    *ChatAgent
	history  *session.History
	executor *registry.Executor
}

// NewChatHandler creates a new chat handler
func NewChatHandler(agent *ChatAgent, history *session.History, executor *registry.Executor) *ChatHandler {
	return &ChatHandler{
		agent:    agent,
		history:  history,
		executor: executor,
	}
}

// ChatRequest represents a chat request
type ChatRequest struct {
	UserInput string
	Context   context.Context
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Text          string
	ToolUsed      string
	ToolCancelled bool
	Success       bool
	Artifacts     []string
}

// HandleChat processes a chat request and returns a response
func (h *ChatHandler) HandleChat(ctx context.Context, userInput string) (*ChatResponse, error) {
	// Add user message to history
	h.history.AddMessage("user", userInput)

	// Classify user intent
	intent := ClassifyIntent(userInput)

	// Handle code creation intent - delegate to Code Agent (prevents hallucinations)
	if intent == IntentCode {
		return h.handleCodeCreation(ctx, userInput)
	}

	// Handle delegation intent specially
	if intent == IntentDelegate {
		// Complex task - delegate to specialist agent
		return h.handleDelegation(ctx, userInput)
	}

	// Select system prompt based on intent
	var systemPrompt string
	switch intent {
	case IntentChat:
		// Simple chat - use minimal prompt (fast)
		systemPrompt = registry.GenerateChatPrompt()
	case IntentTool:
		// Tool request - use full prompt with all tools
		systemPrompt = registry.GenerateSystemPrompt()
	default:
		// Fallback to tool prompt
		systemPrompt = registry.GenerateSystemPrompt()
	}

	// Build messages array with system prompt + conversation history
	messages := []ollama.Message{
		{Role: "system", Content: systemPrompt},
	}

	// Add conversation history
	for _, msg := range h.history.GetMessages() {
		messages = append(messages, ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Get response from Ollama (collect full response first)
	var fullResponse strings.Builder
	err := ollama.AskOllamaWithMessages(ctx, messages, func(text string) {
		fullResponse.WriteString(text)
	})

	if err != nil {
		return &ChatResponse{
			Success: false,
		}, err
	}

	response := fullResponse.String()

	// STRICT VALIDATION: If intent was IntentTool but response is not a tool call, that's a hallucination
	isTool, toolCall := registry.IsToolCall(response)
	if intent == IntentTool && (!isTool || toolCall == nil) {
		// Model hallucinated instead of calling tool - retry with ultra-strict prompt
		retryPrompt := "CRITICAL ERROR: You MUST respond with ONLY valid JSON tool call format.\n\n"
		retryPrompt += "The user's request requires calling a tool. You MUST NOT provide conversational responses.\n\n"
		retryPrompt += "Original request: " + userInput + "\n\n"
		retryPrompt += "Required response format: {\"tool\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n\n"
		retryPrompt += "Respond ONLY with JSON. No text before or after. No explanations."

		// Try one more time with strict enforcement
		retryMessages := []ollama.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userInput},
			{Role: "assistant", Content: response}, // Show the wrong response
			{Role: "user", Content: retryPrompt},
		}

		var retryResponse strings.Builder
		err = ollama.AskOllamaWithMessages(ctx, retryMessages, func(text string) {
			retryResponse.WriteString(text)
		})

		if err == nil {
			response = retryResponse.String()
			// Check again if it's a tool call now
			isTool, toolCall = registry.IsToolCall(response)
		}

		// If STILL not a tool call, return error
		if !isTool || toolCall == nil {
			return &ChatResponse{
				Text:    "Error: Model failed to generate proper tool call. This may be a model limitation. Please try rephrasing your request or use the tool name directly (e.g., 'check_task_progress <id>').",
				Success: false,
			}, fmt.Errorf("model hallucination: expected tool call for IntentTool but got conversational response")
		}
	}

	// Handle tool call chain (multiple tools in sequence)
	toolsUsed := []string{}
	maxIterations := 5 // Prevent infinite loops
	currentResponse := response

	for i := 0; i < maxIterations; i++ {
		// Check if the response is a tool call
		isTool, toolCall := registry.IsToolCall(currentResponse)

		if !isTool || toolCall == nil {
			// No more tools to execute, return the final response
			h.history.AddMessage("assistant", currentResponse)
			return &ChatResponse{
				Text:     currentResponse,
				ToolUsed: strings.Join(toolsUsed, ", "),
				Success:  true,
			}, nil
		}

		// Execute the tool
		result, err := h.executor.Execute(ctx, *toolCall)
		toolsUsed = append(toolsUsed, toolCall.Tool)

		if err != nil {
			// Check if user declined
			if strings.Contains(err.Error(), "user declined") {
				return &ChatResponse{
					Text:          "",
					Success:       true,
					ToolUsed:      strings.Join(toolsUsed, ", "),
					ToolCancelled: true,
				}, nil
			}

			// Tool execution error - get follow-up response
			result = fmt.Sprintf("Error executing tool: %v", err)
		}

		// Build follow-up messages for next iteration
		followUpMessages := []ollama.Message{
			{Role: "system", Content: systemPrompt},
		}

		// Add conversation history
		for _, msg := range h.history.GetMessages() {
			followUpMessages = append(followUpMessages, ollama.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		// Add tool call as assistant message
		followUpMessages = append(followUpMessages, ollama.Message{
			Role:    "assistant",
			Content: fmt.Sprintf(`{"tool": "%s", "arguments": %s}`, toolCall.Tool, formatArgs(toolCall.Arguments)),
		})

		// Add tool result and prompt for next step
		followUpMessages = append(followUpMessages, ollama.Message{
			Role:    "user",
			Content: fmt.Sprintf("Tool '%s' executed successfully. Here are the results:\n\n%s\n\nIf this was part of a multi-step request and more operations are needed, call the NEXT tool now using JSON format. Otherwise, provide a helpful natural response to the user.", toolCall.Tool, result),
		})

		// Get follow-up response
		var followUpResponse strings.Builder
		err = ollama.AskOllamaWithMessages(ctx, followUpMessages, func(text string) {
			followUpResponse.WriteString(text)
		})

		if err != nil {
			return &ChatResponse{
				Success: false,
			}, err
		}

		currentResponse = followUpResponse.String()
	}

	// Max iterations reached
	h.history.AddMessage("assistant", currentResponse)
	return &ChatResponse{
		Text:     currentResponse + "\n\n(Note: Reached maximum tool chain limit)",
		ToolUsed: strings.Join(toolsUsed, ", "),
		Success:  true,
	}, nil
}

// handleDelegation handles delegation intent by calling delegate_task tool
// handleCodeCreation delegates code/project creation tasks directly to Code Agent
// This prevents hallucinations where Wilson describes creating files instead of actually creating them
func (h *ChatHandler) handleCodeCreation(ctx context.Context, userInput string) (*ChatResponse, error) {
	// Build delegation tool call specifically for Code Agent
	toolCall := ToolCall{
		Tool: "delegate_task",
		Arguments: map[string]interface{}{
			"to_agent":    "code",
			"task_type":   "code",
			"description": userInput,
		},
	}

	// Execute delegation to Code Agent
	result, err := h.executor.Execute(ctx, toolCall)
	if err != nil {
		return &ChatResponse{
			Text:    fmt.Sprintf("Failed to delegate to Code Agent: %v", err),
			Success: false,
		}, err
	}

	// Return Code Agent's response
	return &ChatResponse{
		Text:     result,
		ToolUsed: "delegate_task (Code Agent)",
		Success:  true,
	}, nil
}

func (h *ChatHandler) handleDelegation(ctx context.Context, userInput string) (*ChatResponse, error) {
	// Determine which agent to delegate to based on keywords
	toAgent := "analysis" // Default
	taskType := "general"

	lowerInput := strings.ToLower(userInput)
	if strings.Contains(lowerInput, "code") || strings.Contains(lowerInput, "implement") ||
		strings.Contains(lowerInput, "build") || strings.Contains(lowerInput, "refactor") {
		toAgent = "code"
		taskType = "code"
	} else if strings.Contains(lowerInput, "research") || strings.Contains(lowerInput, "analyze") ||
		strings.Contains(lowerInput, "search") || strings.Contains(lowerInput, "find information") {
		toAgent = "analysis"
		taskType = "research"
	}

	// Build delegation tool call
	toolCall := ToolCall{
		Tool: "delegate_task",
		Arguments: map[string]interface{}{
			"to_agent":    toAgent,
			"task_type":   taskType,
			"description": userInput,
		},
	}

	// Execute delegation
	result, err := h.executor.Execute(ctx, toolCall)
	if err != nil {
		return &ChatResponse{
			Success: false,
		}, fmt.Errorf("delegation failed: %w", err)
	}

	// Create response
	response := fmt.Sprintf("I've delegated this task to the %s agent.\n\n%s", toAgent, result)

	// Add to history
	h.history.AddMessage("assistant", response)

	return &ChatResponse{
		Text:     response,
		ToolUsed: "delegate_task",
		Success:  true,
	}, nil
}

// ClearHistory clears the conversation history
func (h *ChatHandler) ClearHistory() {
	h.history.Clear()
}

// formatArgs formats arguments as JSON string
func formatArgs(args map[string]interface{}) string {
	data, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(data)
}
