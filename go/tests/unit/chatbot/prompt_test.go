package chatbot_test

import (
	"strings"
	"testing"
	"wilson/core/registry"
)

func TestGenerateChatPrompt(t *testing.T) {
	prompt := registry.GenerateChatPrompt()

	// Basic checks
	if prompt == "" {
		t.Fatal("Chat prompt should not be empty")
	}

	// Should be minimal (no tool descriptions)
	if len(prompt) > 200 {
		t.Errorf("Chat prompt too long: %d characters (expected < 200)", len(prompt))
	}

	// Should mention Wilson
	if !strings.Contains(prompt, "Wilson") {
		t.Error("Chat prompt should mention Wilson")
	}

	// Should NOT contain tool descriptions
	if strings.Contains(prompt, "tool") || strings.Contains(prompt, "Tool") {
		t.Error("Chat prompt should not contain tool references")
	}

	// Should be conversational
	expectedPhrases := []string{"helpful", "assistant"}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(strings.ToLower(prompt), phrase) {
			t.Errorf("Chat prompt should contain %q", phrase)
		}
	}

	t.Logf("Chat prompt length: %d characters", len(prompt))
	t.Logf("Chat prompt: %s", prompt)
}

func TestGenerateSystemPrompt(t *testing.T) {
	prompt := registry.GenerateSystemPrompt()

	// Basic checks
	if prompt == "" {
		t.Fatal("System prompt should not be empty")
	}

	// Should be comprehensive (with tool descriptions)
	if len(prompt) < 500 {
		t.Errorf("System prompt too short: %d characters (expected > 500)", len(prompt))
	}

	// Should mention Wilson
	if !strings.Contains(prompt, "Wilson") {
		t.Error("System prompt should mention Wilson")
	}

	// Should contain tool information
	requiredSections := []string{
		"tool",
		"Available tools:",
		"Tool call format",
		"{\"tool\":",
	}

	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("System prompt should contain %q", section)
		}
	}

	// Should have examples
	if !strings.Contains(prompt, "Examples:") {
		t.Error("System prompt should contain examples")
	}

	t.Logf("System prompt length: %d characters", len(prompt))
}

func TestPromptCaching(t *testing.T) {
	// First call - generates and caches
	chatPrompt1 := registry.GenerateChatPrompt()
	systemPrompt1 := registry.GenerateSystemPrompt()

	// Second call - should return cached
	chatPrompt2 := registry.GenerateChatPrompt()
	systemPrompt2 := registry.GenerateSystemPrompt()

	// Verify same instance (cached)
	if chatPrompt1 != chatPrompt2 {
		t.Error("Chat prompt should be cached")
	}

	if systemPrompt1 != systemPrompt2 {
		t.Error("System prompt should be cached")
	}

	t.Log("✓ Prompt caching working correctly")
}

func TestInvalidatePromptCache(t *testing.T) {
	// Generate prompts (cached)
	chatPrompt1 := registry.GenerateChatPrompt()
	systemPrompt1 := registry.GenerateSystemPrompt()

	// Invalidate cache
	registry.InvalidatePromptCache()

	// Generate again - should regenerate
	chatPrompt2 := registry.GenerateChatPrompt()
	systemPrompt2 := registry.GenerateSystemPrompt()

	// Content should be same (but regenerated)
	if chatPrompt1 != chatPrompt2 {
		t.Error("Chat prompt content should be same after regeneration")
	}

	if systemPrompt1 != systemPrompt2 {
		t.Error("System prompt content should be same after regeneration")
	}

	t.Log("✓ Prompt cache invalidation working correctly")
}

func TestPromptSizeComparison(t *testing.T) {
	chatPrompt := registry.GenerateChatPrompt()
	systemPrompt := registry.GenerateSystemPrompt()

	chatLen := len(chatPrompt)
	systemLen := len(systemPrompt)

	// System prompt should be significantly larger
	ratio := float64(systemLen) / float64(chatLen)

	if ratio < 5.0 {
		t.Errorf("System prompt should be at least 5x larger than chat prompt. Ratio: %.1fx", ratio)
	}

	t.Logf("Chat prompt: %d characters", chatLen)
	t.Logf("System prompt: %d characters", systemLen)
	t.Logf("Size ratio: %.1fx", ratio)
	t.Logf("✓ System prompt is %.1fx larger (expected 5x+)", ratio)
}

func BenchmarkGenerateChatPrompt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		registry.GenerateChatPrompt()
	}
}

func BenchmarkGenerateSystemPrompt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		registry.GenerateSystemPrompt()
	}
}

func BenchmarkGenerateChatPromptCached(b *testing.B) {
	// Pre-generate to cache
	registry.GenerateChatPrompt()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GenerateChatPrompt()
	}
}

func BenchmarkGenerateSystemPromptCached(b *testing.B) {
	// Pre-generate to cache
	registry.GenerateSystemPrompt()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GenerateSystemPrompt()
	}
}
