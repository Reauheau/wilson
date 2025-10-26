package lsp

import (
	"sync"
	"time"
)

// ResponseCache caches LSP responses to avoid redundant queries
type ResponseCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// CacheEntry represents a cached LSP response
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// NewResponseCache creates a new response cache with 30s TTL
func NewResponseCache() *ResponseCache {
	return &ResponseCache{
		entries: make(map[string]*CacheEntry),
		ttl:     30 * time.Second,
	}
}

// Get retrieves a value from cache if not expired
func (c *ResponseCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		// Expired
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value in cache
func (c *ResponseCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a key from cache
func (c *ResponseCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// InvalidateFile removes all cache entries for a file
func (c *ResponseCache) InvalidateFile(filePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all entries that start with this file path
	for key := range c.entries {
		if len(key) >= len(filePath) && key[:len(filePath)] == filePath {
			delete(c.entries, key)
		}
	}
}

// Clear removes all entries from cache
func (c *ResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}
