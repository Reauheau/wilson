package setup

import (
	"fmt"
	"time"

	"wilson/config"
	contextpkg "wilson/context"
)

// InitializeContextManager creates and configures the context manager
func InitializeContextManager(cfg *config.Config) *contextpkg.Manager {
	if !cfg.Context.Enabled {
		return nil
	}

	// Create context manager
	manager, err := contextpkg.NewManager(cfg.Context.DBPath, cfg.Context.AutoStore)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize context store: %v\n", err)
		return nil
	}

	// Set as global manager
	contextpkg.SetGlobalManager(manager)

	// Create or get default session context (silent)
	if cfg.Context.DefaultContext != "" {
		sessionKey := fmt.Sprintf("%s-%s", cfg.Context.DefaultContext, time.Now().Format("2006-01-02"))
		_, err := manager.GetOrCreateContext(
			sessionKey,
			contextpkg.TypeSession,
			fmt.Sprintf("Session %s", time.Now().Format("2006-01-02")),
		)
		if err != nil {
			fmt.Printf("Warning: Failed to create default context: %v\n", err)
		} else {
			manager.SetActiveContext(sessionKey)
		}
	}

	return manager
}
