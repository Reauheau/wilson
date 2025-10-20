package web

import (
	"sync"
	"wilson/llm"
)

// Global LLM manager (set during initialization)
var (
	llmManager   *llm.Manager
	llmManagerMu sync.RWMutex
)

// SetLLMManager sets the global LLM manager for web tools
func SetLLMManager(manager *llm.Manager) {
	llmManagerMu.Lock()
	defer llmManagerMu.Unlock()
	llmManager = manager
}

// GetLLMManager returns the global LLM manager
func GetLLMManager() *llm.Manager {
	llmManagerMu.RLock()
	defer llmManagerMu.RUnlock()
	return llmManager
}
