package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"wilson/config"
	contextpkg "wilson/context"
	"wilson/llm"
	"wilson/core/registry"
	. "wilson/core/types"
)

type ResearchTopicTool struct{}

type ResearchResult struct {
	Query       string              `json:"query"`
	Sources     []SourceAnalysis    `json:"sources"`
	Summary     string              `json:"summary"`
	TotalSites  int                 `json:"total_sites"`
	Successful  int                 `json:"successful"`
	Failed      int                 `json:"failed"`
	Duration    string              `json:"duration"`
}

type SourceAnalysis struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	Analysis    string `json:"analysis,omitempty"`
	ContentSize int    `json:"content_size,omitempty"`
}

func (t *ResearchTopicTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "research_topic",
		Description:     "PRIMARY WEB TOOL: Searches, fetches, and analyzes web sources automatically. Use for: weather, news, current info, articles, documentation lookups. Handles everything end-to-end",
		Category:        CategoryWeb,
		RiskLevel:       RiskModerate,
		RequiresConfirm: false, // Auto-execute for research queries
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "query",
				Type:        "string",
				Required:    true,
				Description: "The research query/topic",
				Example:     "How do large language models work",
			},
			{
				Name:        "max_sites",
				Type:        "number",
				Required:    false,
				Description: "Maximum number of sites to analyze (default: 3, max: 10)",
				Example:     "3",
			},
			{
				Name:        "depth",
				Type:        "string",
				Required:    false,
				Description: "Research depth: 'quick' (summaries only) or 'detailed' (full analysis). Default: quick",
				Example:     "quick",
			},
		},
		Examples: []string{
			`{"tool": "research_topic", "arguments": {"query": "How do LLMs work", "max_sites": 3}}`,
			`{"tool": "research_topic", "arguments": {"query": "Ollama API best practices", "max_sites": 5, "depth": "detailed"}}`,
		},
	}
}

func (t *ResearchTopicTool) Validate(args map[string]interface{}) error {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return fmt.Errorf("query parameter is required")
	}

	if maxSites, ok := args["max_sites"].(float64); ok {
		if maxSites < 1 || maxSites > 10 {
			return fmt.Errorf("max_sites must be between 1 and 10")
		}
	}

	if depth, ok := args["depth"].(string); ok {
		if depth != "quick" && depth != "detailed" {
			return fmt.Errorf("depth must be 'quick' or 'detailed'")
		}
	}

	return nil
}

func (t *ResearchTopicTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithProgress(ctx, args, nil)
}

func (t *ResearchTopicTool) ExecuteWithProgress(ctx context.Context, args map[string]interface{}, progress ProgressCallback) (string, error) {
	startTime := time.Now()

	query, _ := args["query"].(string)

	// Parse optional parameters
	maxSites := 3
	if ms, ok := args["max_sites"].(float64); ok {
		maxSites = int(ms)
	}

	depth := "quick"
	if d, ok := args["depth"].(string); ok {
		depth = d
	}

	// Step 1: Search for the topic
	if progress != nil {
		progress("Searching the web...")
	}

	searchTool := &SearchWebTool{}
	searchResult, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": query,
	})
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	// Parse search results to get URLs
	results, err := parseSearchOutput(searchResult)
	if err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	// Limit to max_sites
	if len(results) > maxSites {
		results = results[:maxSites]
	}

	if len(results) == 0 {
		return "No search results found for the query.", nil
	}

	// Step 2: Fetch and analyze sites concurrently
	if progress != nil {
		progress(fmt.Sprintf("Fetching %d sites...", len(results)))
	}
	sources := t.fetchAndAnalyzeSitesWithProgress(ctx, results, depth, progress)

	// Step 3: Generate consolidated summary
	if progress != nil {
		progress("Analyzing and summarizing...")
	}
	summary, err := t.generateConsolidatedSummary(ctx, query, sources, depth)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	// Create result
	result := ResearchResult{
		Query:      query,
		Sources:    sources,
		Summary:    summary,
		TotalSites: len(results),
		Successful: countSuccessful(sources),
		Failed:     countFailed(sources),
		Duration:   time.Since(startTime).Round(time.Millisecond).String(),
	}

	// Store research result in context if available
	t.storeResearchResult(ctx, result)

	// Format output
	return formatResearchResult(result), nil
}

func (t *ResearchTopicTool) fetchAndAnalyzeSites(ctx context.Context, results []SearchResult, depth string) []SourceAnalysis {
	return t.fetchAndAnalyzeSitesWithProgress(ctx, results, depth, nil)
}

func (t *ResearchTopicTool) fetchAndAnalyzeSitesWithProgress(ctx context.Context, results []SearchResult, depth string, progress ProgressCallback) []SourceAnalysis {
	var wg sync.WaitGroup
	sources := make([]SourceAnalysis, len(results))

	// Use a semaphore to limit concurrent requests (max 3 at a time)
	semaphore := make(chan struct{}, 3)

	for i, result := range results {
		wg.Add(1)
		go func(idx int, sr SearchResult) {
			defer wg.Done()

			// Report progress
			if progress != nil {
				progress(fmt.Sprintf("Fetching page %d/%d: %s", idx+1, len(results), sr.Title))
			}

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Add small delay to avoid overwhelming servers
			time.Sleep(time.Duration(idx) * 500 * time.Millisecond)

			sources[idx] = t.fetchAndAnalyzeSite(ctx, sr, depth)
		}(i, result)
	}

	wg.Wait()
	return sources
}

func (t *ResearchTopicTool) fetchAndAnalyzeSite(ctx context.Context, result SearchResult, depth string) SourceAnalysis {
	source := SourceAnalysis{
		URL:   result.URL,
		Title: result.Title,
	}

	// Create a timeout context for this operation
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Step 1: Fetch the page
	fetchTool := &FetchPageTool{}
	html, err := fetchTool.Execute(fetchCtx, map[string]interface{}{
		"url": result.URL,
	})
	if err != nil {
		source.Success = false
		source.Error = fmt.Sprintf("Fetch failed: %v", err)
		return source
	}

	// Step 2: Extract clean content
	extractTool := &ExtractContentTool{}
	content, err := extractTool.Execute(fetchCtx, map[string]interface{}{
		"html": html,
	})
	if err != nil {
		source.Success = false
		source.Error = fmt.Sprintf("Extraction failed: %v", err)
		return source
	}

	source.ContentSize = len(content)

	// Truncate content if too long
	maxContentLen := 5000
	if depth == "detailed" {
		maxContentLen = 10000
	}
	if len(content) > maxContentLen {
		content = content[:maxContentLen] + "\n\n[Content truncated...]"
	}

	// Step 3: Analyze content
	analyzeTool := &AnalyzeContentTool{}
	mode := "summarize"
	if depth == "detailed" {
		mode = "extract_key_points"
	}

	analysis, err := analyzeTool.Execute(fetchCtx, map[string]interface{}{
		"content": content,
		"mode":    mode,
	})
	if err != nil {
		source.Success = false
		source.Error = fmt.Sprintf("Analysis failed: %v", err)
		return source
	}

	source.Success = true
	source.Analysis = analysis

	return source
}

func (t *ResearchTopicTool) generateConsolidatedSummary(ctx context.Context, query string, sources []SourceAnalysis, depth string) (string, error) {
	// Get LLM manager
	manager := GetLLMManager()
	if manager == nil {
		return "", fmt.Errorf("LLM manager not configured")
	}

	// Build consolidated content
	var consolidatedContent strings.Builder
	consolidatedContent.WriteString(fmt.Sprintf("Research Topic: %s\n\n", query))
	consolidatedContent.WriteString("=== Sources Analyzed ===\n\n")

	successCount := 0
	for i, source := range sources {
		if source.Success {
			successCount++
			consolidatedContent.WriteString(fmt.Sprintf("Source %d: %s\n", i+1, source.Title))
			consolidatedContent.WriteString(fmt.Sprintf("URL: %s\n", source.URL))
			consolidatedContent.WriteString(fmt.Sprintf("Content size: %d chars\n\n", source.ContentSize))
			consolidatedContent.WriteString(source.Analysis)
			consolidatedContent.WriteString("\n\n---\n\n")
		}
	}

	if successCount == 0 {
		return "Failed to analyze any sources successfully.", nil
	}

	// Prepare prompt based on depth
	var systemPrompt string
	if depth == "detailed" {
		systemPrompt = `You are a research assistant. Synthesize the information from multiple sources into a comprehensive answer.

Instructions:
1. Provide a detailed overview of the topic
2. Highlight key insights from each source
3. Note any disagreements or different perspectives
4. Organize information logically
5. Include relevant examples or specifics mentioned
6. Cite sources by number when referencing specific information

Format your response clearly with sections and bullet points.`
	} else {
		systemPrompt = `You are a research assistant. Synthesize the information from multiple sources into a concise summary.

Instructions:
1. Provide a clear, concise answer to the research topic
2. Include the most important points from the sources
3. Keep it brief but informative (3-5 paragraphs)
4. Mention if sources agree or disagree on key points

Be direct and factual.`
	}

	// Get analysis LLM config
	cfg := config.Get()
	purpose := llm.PurposeAnalysis
	if toolCfg, ok := cfg.Tools.Tools["analyze_content"]; ok {
		if toolCfg.LLM == "chat" {
			purpose = llm.PurposeChat
		}
	}

	// Call LLM for synthesis
	req := llm.Request{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: consolidatedContent.String(),
			},
		},
	}

	resp, err := manager.Generate(ctx, purpose, req)
	if err != nil {
		return "", fmt.Errorf("LLM synthesis failed: %w", err)
	}

	return resp.Content, nil
}

func (t *ResearchTopicTool) storeResearchResult(ctx context.Context, result ResearchResult) {
	manager := contextpkg.GetGlobalManager()
	if manager == nil || !manager.IsAutoStoreEnabled() || manager.GetActiveContext() == "" {
		return
	}

	// Store the full research result
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}

	_, _ = manager.StoreArtifact(contextpkg.StoreArtifactRequest{
		ContextKey: manager.GetActiveContext(),
		Type:       "research_result",
		Content:    string(resultJSON),
		Source:     "research_topic",
		Agent:      "research_orchestrator",
		Metadata: contextpkg.ArtifactMetadata{
			Tags: []string{"research", "multi-source", result.Query},
		},
	})
}

func parseSearchOutput(output string) ([]SearchResult, error) {
	// Extract URLs from formatted search output
	// Format is: "1. Title\n   URL: <url>\n   Snippet"
	lines := strings.Split(output, "\n")
	var results []SearchResult
	var currentResult *SearchResult

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for title line (starts with number)
		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && line[1] == '.' {
			if currentResult != nil {
				results = append(results, *currentResult)
			}
			currentResult = &SearchResult{
				Title: strings.TrimSpace(line[2:]),
			}
		} else if strings.HasPrefix(line, "URL: ") && currentResult != nil {
			currentResult.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL: "))
		}
	}

	if currentResult != nil {
		results = append(results, *currentResult)
	}

	return results, nil
}

func formatResearchResult(result ResearchResult) string {
	var output strings.Builder

	output.WriteString("═══════════════════════════════════════════════════════════\n")
	output.WriteString(fmt.Sprintf("  RESEARCH REPORT: %s\n", result.Query))
	output.WriteString("═══════════════════════════════════════════════════════════\n\n")

	output.WriteString(fmt.Sprintf("📊 Research Statistics:\n"))
	output.WriteString(fmt.Sprintf("   • Total sites analyzed: %d\n", result.TotalSites))
	output.WriteString(fmt.Sprintf("   • Successful: %d\n", result.Successful))
	output.WriteString(fmt.Sprintf("   • Failed: %d\n", result.Failed))
	output.WriteString(fmt.Sprintf("   • Duration: %s\n\n", result.Duration))

	output.WriteString("📝 CONSOLIDATED SUMMARY:\n")
	output.WriteString("───────────────────────────────────────────────────────────\n")
	output.WriteString(result.Summary)
	output.WriteString("\n\n")

	output.WriteString("📚 SOURCES:\n")
	output.WriteString("───────────────────────────────────────────────────────────\n")
	for i, source := range result.Sources {
		output.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, source.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", source.URL))
		if source.Success {
			output.WriteString(fmt.Sprintf("   ✅ Analyzed successfully (%d chars)\n", source.ContentSize))
		} else {
			output.WriteString(fmt.Sprintf("   ❌ Failed: %s\n", source.Error))
		}
	}

	output.WriteString("\n═══════════════════════════════════════════════════════════\n")
	output.WriteString("💾 All findings have been stored in the context for future reference.\n")

	return output.String()
}

func countSuccessful(sources []SourceAnalysis) int {
	count := 0
	for _, s := range sources {
		if s.Success {
			count++
		}
	}
	return count
}

func countFailed(sources []SourceAnalysis) int {
	count := 0
	for _, s := range sources {
		if !s.Success {
			count++
		}
	}
	return count
}

func init() {
	registry.Register(&ResearchTopicTool{})
}
