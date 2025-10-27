package setup

import (
	"context"
	"fmt"

	code_intelligence "wilson/capabilities/code_intelligence"
	"wilson/capabilities/web"
	"wilson/config"
	"wilson/llm"
)

// InitializeLLMManager creates and configures the LLM manager
func InitializeLLMManager(ctx context.Context, cfg *config.Config) *llm.Manager {
	manager := llm.NewManager()

	// Register LLMs from configuration
	if cfg.LLMs != nil {
		for name, llmCfg := range cfg.LLMs {
			// Convert config name to Purpose
			var purpose llm.Purpose
			switch name {
			case "chat":
				purpose = llm.PurposeChat
			case "analysis":
				purpose = llm.PurposeAnalysis
			case "code":
				purpose = llm.PurposeCode
			case "vision":
				purpose = llm.PurposeVision
			default:
				fmt.Printf("Warning: Unknown LLM purpose '%s', skipping\n", name)
				continue
			}

			// Create LLM config
			config := llm.Config{
				Provider:    llmCfg.Provider,
				Model:       llmCfg.Model,
				Temperature: llmCfg.Temperature,
				BaseURL:     llmCfg.BaseURL,
				APIKey:      llmCfg.APIKey,
				Fallback:    llmCfg.Fallback,
				Options:     llmCfg.Options,
			}

			// Register the LLM (silent)
			if err := manager.RegisterLLM(purpose, config); err != nil {
				fmt.Printf("Warning: Failed to register %s LLM: %v\n", name, err)
			}
		}
	}

	// Set the LLM manager for tools that need it
	web.SetLLMManager(manager)
	code_intelligence.SetLLMManager(manager)

	return manager
}
