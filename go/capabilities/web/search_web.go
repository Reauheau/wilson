package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"wilson/config"
	contextpkg "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type SearchWebTool struct{}

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (t *SearchWebTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "search_web",
		Description:     "Search the web using DuckDuckGo and return a list of results",
		Category:        CategoryWeb,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "query",
				Type:        "string",
				Required:    true,
				Description: "The search query",
				Example:     "Ollama API documentation",
			},
		},
		Examples: []string{
			`{"tool": "search_web", "arguments": {"query": "Ollama API documentation"}}`,
		},
	}
}

func (t *SearchWebTool) Validate(args map[string]interface{}) error {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return fmt.Errorf("query parameter is required")
	}
	return nil
}

func (t *SearchWebTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)

	// Get max results from config
	maxResults := 10
	cfg := config.Get()
	if toolCfg, ok := cfg.Tools.Tools["search_web"]; ok {
		if toolCfg.MaxResults != nil {
			maxResults = *toolCfg.MaxResults
		}
	}

	// Perform search
	results, err := searchDuckDuckGo(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	// Auto-store results if context manager available and auto-store enabled
	manager := contextpkg.GetGlobalManager()
	if manager != nil && manager.IsAutoStoreEnabled() && manager.GetActiveContext() != "" {
		// Store results as JSON for structured storage
		resultsJSON, err := json.Marshal(results)
		if err == nil {
			_, _ = manager.StoreArtifact(contextpkg.StoreArtifactRequest{
				ContextKey: manager.GetActiveContext(),
				Type:       contextpkg.ArtifactWebSearch,
				Content:    string(resultsJSON),
				Source:     "search_web",
				Agent:      "web_tools",
				Metadata: contextpkg.ArtifactMetadata{
					Tags: []string{"search", "web", query},
				},
			})
		}
	}

	// Format results
	return formatSearchResults(query, results), nil
}

func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	// Use HTML search endpoint instead of API for actual search results
	searchURL := "https://html.duckduckgo.com/html/"

	// Prepare POST data
	params := url.Values{}
	params.Add("q", query)
	params.Add("b", "") // Start index (empty = 0)
	params.Add("kl", "us-en") // Region

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", searchURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Wilson Assistant/1.0)")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML using goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	results := make([]SearchResult, 0, maxResults)

	// Find search result elements
	// DuckDuckGo HTML uses .result class for each search result
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}

		// Extract title and URL from .result__a link
		titleElem := s.Find(".result__a")
		title := strings.TrimSpace(titleElem.Text())
		href, exists := titleElem.Attr("href")

		// Extract snippet from .result__snippet
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		// Only add if we have valid data
		if exists && title != "" && href != "" {
			// DuckDuckGo sometimes uses redirect URLs, try to extract the actual URL
			actualURL := extractActualURL(href)

			results = append(results, SearchResult{
				Title:   title,
				URL:     actualURL,
				Snippet: snippet,
			})
		}
	})

	// Fallback if no results found
	if len(results) == 0 {
		return []SearchResult{
			{
				Title:   "No results found",
				URL:     "https://duckduckgo.com/?q=" + url.QueryEscape(query),
				Snippet: "No search results found for this query. Try rephrasing your search.",
			},
		}, nil
	}

	return results, nil
}

// extractActualURL extracts the real URL from DuckDuckGo redirect URLs
func extractActualURL(ddgURL string) string {
	// DuckDuckGo HTML results often use redirect URLs like:
	// /l/?uddg=https%3A%2F%2Fexample.com
	if strings.HasPrefix(ddgURL, "/l/?uddg=") {
		encoded := strings.TrimPrefix(ddgURL, "/l/?uddg=")
		if decoded, err := url.QueryUnescape(encoded); err == nil {
			return decoded
		}
	}

	// If it's a relative URL, make it absolute
	if strings.HasPrefix(ddgURL, "/") {
		return "https://duckduckgo.com" + ddgURL
	}

	return ddgURL
}

func formatSearchResults(query string, results []SearchResult) string {
	var output string
	output += fmt.Sprintf("Found %d results for \"%s\":\n\n", len(results), query)

	for i, result := range results {
		output += fmt.Sprintf("%d. %s\n", i+1, result.Title)
		output += fmt.Sprintf("   URL: %s\n", result.URL)
		output += fmt.Sprintf("   %s\n\n", truncate(result.Snippet, 200))
	}

	return output
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func init() {
	registry.Register(&SearchWebTool{})
}
