package base

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wilson/llm"
)

// CallLLMWithValidation calls an LLM and retries until valid JSON is received
// This is the production-ready approach for handling unreliable local models
func CallLLMWithValidation(
	ctx context.Context,
	llmManager *llm.Manager,
	purpose llm.Purpose,
	systemPrompt string,
	userPrompt string,
	maxRetries int,
	taskID string,
) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 5 // Default
	}

	var response string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Build request
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		}

		// Get response
		resp, callErr := llmManager.Generate(ctx, purpose, req)
		if callErr != nil {
			return "", fmt.Errorf("LLM error on attempt %d: %v", attempt, callErr)
		}

		response = resp.Content

		// Validate JSON - but extract if needed
		isValid, validationErr := ValidateToolCallJSON(response)
		if isValid {
			if attempt > 1 {
				fmt.Printf("[Validation] ✓ Valid JSON received on attempt %d\n", attempt)
			}
			// If JSON was extracted from mixed text, log it but still return original
			// (the executor will extract it again - that's fine)
			return response, nil
		}

		fmt.Printf("[Validation] ✗ Attempt %d/%d: Invalid JSON - %v\n", attempt, maxRetries, validationErr)

		// If not last attempt, modify prompt to include error feedback
		if attempt < maxRetries {
			userPrompt = fmt.Sprintf("%s\n\n❌ ERROR: Your previous response had INVALID JSON.\nError: %s\n\nTry again. Requirements:\n1. Respond with ONLY valid JSON\n2. All quotes inside strings MUST be escaped: \\\" not \"\n3. All backslashes must be escaped: \\\\ not \\\n4. Format: {\"tool\": \"tool_name\", \"arguments\": {...}}\n5. Do NOT include any text outside the JSON object",
				userPrompt, validationErr.Error())
		} else {
			// Last attempt failed
			return "", fmt.Errorf("failed to generate valid JSON after %d attempts. Last error: %v", maxRetries, validationErr)
		}

		// Update progress if we have taskID
		// TODO: Add callback interface to update progress without import cycle
		// coordinator := GetGlobalCoordinator()
		// if coordinator != nil && taskID != "" {
		// 	coordinator.UpdateTaskProgress(taskID, fmt.Sprintf("Retrying LLM (attempt %d/%d)...", attempt+1, maxRetries), nil)
		// }
	}

	return "", fmt.Errorf("unexpected error in validation loop")
}

// ValidateToolCallJSON checks if a response is valid JSON with tool call structure
// Now more lenient - extracts JSON from mixed text/JSON responses
func ValidateToolCallJSON(response string) (bool, error) {
	response = strings.TrimSpace(response)

	// Must not be empty
	if response == "" {
		return false, fmt.Errorf("empty response")
	}

	// Try direct parse first
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(response), &data); err != nil {
		// Direct parse failed - try to extract JSON from text
		extracted := extractJSON(response)
		if extracted == "" {
			return false, fmt.Errorf("no JSON found in response")
		}

		// Try parsing extracted JSON
		if err := json.Unmarshal([]byte(extracted), &data); err != nil {
			return false, fmt.Errorf("invalid JSON: %v", err)
		}
	}

	// Check for required fields
	tool, hasTool := data["tool"].(string)
	if !hasTool || tool == "" {
		return false, fmt.Errorf("missing or invalid 'tool' field")
	}

	arguments, hasArgs := data["arguments"]
	if !hasArgs || arguments == nil {
		return false, fmt.Errorf("missing 'arguments' field")
	}

	return true, nil
}

// extractJSON extracts JSON object from mixed text/JSON responses
// Looks for {"tool": ...} pattern and extracts complete JSON object
func extractJSON(text string) string {
	// Find start of JSON (look for {"tool":)
	startIdx := strings.Index(text, `{"tool"`)
	if startIdx == -1 {
		return ""
	}

	// Count braces to find matching closing brace
	braceCount := 0
	inString := false
	escaped := false

	for i := startIdx; i < len(text); i++ {
		char := text[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				// Found matching closing brace
				return text[startIdx : i+1]
			}
		}
	}

	return ""
}
