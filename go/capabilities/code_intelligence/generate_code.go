package code_intelligence

import (
	"context"
	"fmt"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
	"wilson/llm"
)

// GenerateCodeTool generates code using specialized code model
// This separates code generation from tool orchestration
type GenerateCodeTool struct {
	llmManager *llm.Manager
}

// Metadata returns tool metadata
func (t *GenerateCodeTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:        "generate_code",
		Description: "Generate code using specialized code model. Use this when you need to write actual code. Returns raw code (not JSON).",
		Category:    CategoryFileSystem,
		RiskLevel:   RiskSafe,
		Enabled:     true,
		Parameters: []Parameter{
			{
				Name:        "language",
				Type:        "string",
				Required:    true,
				Description: "Programming language (go, python, javascript, rust, etc)",
				Example:     "go",
			},
			{
				Name:        "description",
				Type:        "string",
				Required:    true,
				Description: "Clear description of what the code should do",
				Example:     "HTTP server with /health endpoint",
			},
			{
				Name:        "requirements",
				Type:        "array",
				Required:    false,
				Description: "Specific requirements or constraints",
				Example:     `["Use standard library only", "Handle errors gracefully"]`,
			},
			{
				Name:        "context",
				Type:        "string",
				Required:    false,
				Description: "Additional context (existing code to extend, dependencies, etc)",
				Example:     "This will be part of a CLI tool",
			},
			{
				Name:        "style",
				Type:        "string",
				Required:    false,
				Description: "Code style preferences",
				Example:     "idiomatic Go with error handling",
			},
		},
		Examples: []string{
			`{"tool": "generate_code", "arguments": {"language": "go", "description": "Function to open applications by name on macOS"}}`,
			`{"tool": "generate_code", "arguments": {"language": "python", "description": "Parse JSON configuration file", "requirements": ["Use standard library", "Return dict"]}}`,
		},
	}
}

// Validate validates the arguments
func (t *GenerateCodeTool) Validate(args map[string]interface{}) error {
	if _, ok := args["language"]; !ok {
		return fmt.Errorf("language is required")
	}
	if _, ok := args["description"]; !ok {
		return fmt.Errorf("description is required")
	}
	return nil
}

// Execute generates code using the specialized code model
func (t *GenerateCodeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Use package-level LLM manager
	if packageLLMManager == nil {
		return "", fmt.Errorf("LLM manager not initialized for code generation")
	}

	language := args["language"].(string)
	description := args["description"].(string)

	// Build prompt for code model
	var prompt strings.Builder

	// ✅ SPECIAL HANDLING: Detect test file generation
	isTestFile := strings.Contains(strings.ToLower(description), "test")

	if isTestFile {
		// More specific guidance for test files
		prompt.WriteString(fmt.Sprintf("Generate %s unit tests. ", language))
		prompt.WriteString("Write standard unit tests using the testing package. ")

		// ✅ CRITICAL: Fix package name for Go tests
		if language == "go" {
			prompt.WriteString("IMPORTANT: Use 'package main' NOT 'package main_test'. ")
			prompt.WriteString("Test files in the same directory as the code should use the same package name. ")
		}

		prompt.WriteString("Test functions should call the code under test directly (not via main()). ")
		prompt.WriteString(fmt.Sprintf("Task: %s\n\n", description))
	} else {
		// Regular code generation
		prompt.WriteString(fmt.Sprintf("Generate %s code that: %s\n\n", language, description))

		// ✅ ROBUST: Always create testable code structure
		prompt.WriteString("IMPORTANT: Create separate functions/methods for business logic. ")
		prompt.WriteString("Keep main() minimal - it should only call other functions. ")
		prompt.WriteString("This ensures the code is testable.\n\n")
	}

	// Add requirements
	if reqs, ok := args["requirements"].([]interface{}); ok && len(reqs) > 0 {
		prompt.WriteString("Requirements:\n")
		for _, req := range reqs {
			prompt.WriteString(fmt.Sprintf("- %s\n", req))
		}
		prompt.WriteString("\n")
	}

	// Add context
	if contextStr, ok := args["context"].(string); ok && contextStr != "" {
		prompt.WriteString(fmt.Sprintf("Context: %s\n\n", contextStr))
	}

	// Add style guidance
	if style, ok := args["style"].(string); ok && style != "" {
		prompt.WriteString(fmt.Sprintf("Style: %s\n\n", style))
	}

	prompt.WriteString("Respond with ONLY the code. No markdown code blocks, no explanations, no JSON.")

	// Call code model (qwen2.5-coder:14b)
	req := llm.Request{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: fmt.Sprintf("You are a %s code generator. Generate ONLY code with no additional text, explanations, or formatting.", language),
			},
			{
				Role:    "user",
				Content: prompt.String(),
			},
		},
	}

	resp, err := packageLLMManager.Generate(ctx, llm.PurposeCode, req)
	if err != nil {
		return "", fmt.Errorf("code generation failed: %w", err)
	}

	// Clean up response (remove markdown if present)
	code := resp.Content
	code = cleanCodeResponse(code, language)

	return code, nil
}

// cleanCodeResponse removes common formatting artifacts from code model output
func cleanCodeResponse(code, language string) string {
	code = strings.TrimSpace(code)

	// Remove markdown code blocks if present
	codeBlockStart := "```" + language
	if strings.HasPrefix(code, codeBlockStart) {
		code = strings.TrimPrefix(code, codeBlockStart)
		code = strings.TrimSuffix(code, "```")
		code = strings.TrimSpace(code)
	} else if strings.HasPrefix(code, "```") {
		code = strings.TrimPrefix(code, "```")
		code = strings.TrimSuffix(code, "```")
		code = strings.TrimSpace(code)
	}

	return code
}

// Package-level LLM manager
var packageLLMManager *llm.Manager

// SetLLMManager sets the LLM manager for code generation tools
func SetLLMManager(manager *llm.Manager) {
	packageLLMManager = manager
}

func init() {
	// Register tool - LLM manager will be set via SetLLMManager
	tool := &GenerateCodeTool{}
	registry.Register(tool)
}
