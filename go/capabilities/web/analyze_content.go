package web

import (
	"context"
	"fmt"
	"strings"

	"wilson/config"
	contextpkg "wilson/context"
	"wilson/llm"
	"wilson/core/registry"
	. "wilson/core/types"
)

type AnalyzeContentTool struct{}

func (t *AnalyzeContentTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "analyze_content",
		Description:     "Analyze web content from a URL using specialized LLM. Use this after search_web to answer questions about web pages (weather, articles, docs, etc). Can summarize, extract key points, or answer specific questions",
		Category:        CategoryWeb,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "content",
				Type:        "string",
				Required:    true,
				Description: "The text content to analyze",
			},
			{
				Name:        "mode",
				Type:        "string",
				Required:    true,
				Description: "Analysis mode: summarize, extract_key_points, or answer_question",
				Example:     "summarize",
			},
			{
				Name:        "question",
				Type:        "string",
				Required:    false,
				Description: "The question to answer (only for answer_question mode)",
			},
		},
		Examples: []string{
			`{"tool": "analyze_content", "arguments": {"content": "Long article text...", "mode": "summarize"}}`,
			`{"tool": "analyze_content", "arguments": {"content": "Documentation...", "mode": "answer_question", "question": "How do I install it?"}}`,
		},
	}
}

func (t *AnalyzeContentTool) Validate(args map[string]interface{}) error {
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return fmt.Errorf("content parameter is required")
	}

	mode, ok := args["mode"].(string)
	if !ok || mode == "" {
		return fmt.Errorf("mode parameter is required")
	}

	validModes := map[string]bool{
		"summarize":          true,
		"extract_key_points": true,
		"answer_question":    true,
	}
	if !validModes[mode] {
		return fmt.Errorf("invalid mode: %s (must be summarize, extract_key_points, or answer_question)", mode)
	}

	if mode == "answer_question" {
		question, ok := args["question"].(string)
		if !ok || question == "" {
			return fmt.Errorf("question parameter is required for answer_question mode")
		}
	}

	return nil
}

func (t *AnalyzeContentTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Get LLM manager
	manager := GetLLMManager()
	if manager == nil {
		return "", fmt.Errorf("LLM manager not configured - web tools initialization may have failed")
	}

	content, _ := args["content"].(string)
	mode, _ := args["mode"].(string)
	question, _ := args["question"].(string)

	// Get max content length from config
	maxContentLen := 10000
	cfg := config.Get()
	if toolCfg, ok := cfg.Tools.Tools["analyze_content"]; ok {
		if toolCfg.MaxContentLen != nil {
			maxContentLen = *toolCfg.MaxContentLen
		}
	}

	// Truncate content if too long
	if len(content) > maxContentLen {
		content = content[:maxContentLen] + "\n\n[Content truncated...]"
	}

	// Get the prompt for this mode
	prompt := getPrompt(mode, question)

	// Determine which LLM to use
	purpose := llm.PurposeAnalysis
	if toolCfg, ok := cfg.Tools.Tools["analyze_content"]; ok {
		if toolCfg.LLM == "chat" {
			purpose = llm.PurposeChat
		}
	}

	// Prepare LLM request
	req := llm.Request{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: content,
			},
		},
	}

	// Call LLM
	resp, err := manager.Generate(ctx, purpose, req)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// Auto-store analysis results if context manager available and auto-store enabled
	ctxManager := contextpkg.GetGlobalManager()
	if ctxManager != nil && ctxManager.IsAutoStoreEnabled() && ctxManager.GetActiveContext() != "" {
		_, _ = ctxManager.StoreArtifact(contextpkg.StoreArtifactRequest{
			ContextKey: ctxManager.GetActiveContext(),
			Type:       contextpkg.ArtifactAnalysis,
			Content:    resp.Content,
			Source:     "analyze_content",
			Agent:      "analysis_llm",
			Metadata: contextpkg.ArtifactMetadata{
				Model:      resp.Model,
				TokensUsed: resp.TokensUsed,
				Tags:       []string{"analysis", mode, "llm"},
			},
		})
	}

	// Format result
	result := fmt.Sprintf("Analysis (%s mode):\n", mode)
	result += fmt.Sprintf("Model used: %s\n\n", resp.Model)
	result += resp.Content

	return result, nil
}

func getPrompt(mode, question string) string {
	// Try to get custom prompts from config
	cfg := config.Get()
	if toolCfg, ok := cfg.Tools.Tools["analyze_content"]; ok {
		if prompt, ok := toolCfg.Prompts[mode]; ok {
			return strings.ReplaceAll(prompt, "{question}", question)
		}
	}

	// Default prompts
	switch mode {
	case "summarize":
		return "Provide a concise summary of the following content in 3-5 bullet points. Focus on the key information and main takeaways. Use clear, direct language."
	case "extract_key_points":
		return "Extract and list the main points, important facts, and key information from the following content. Present them as a bulleted list."
	case "answer_question":
		return fmt.Sprintf("Based on the following content, answer this question: %s\n\nProvide a clear, concise answer based only on the information given. If the content doesn't contain enough information to answer the question, say so.", question)
	default:
		return "Analyze the following content and provide relevant insights."
	}
}

func init() {
	registry.Register(&AnalyzeContentTool{})
}
