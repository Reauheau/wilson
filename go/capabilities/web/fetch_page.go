package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wilson/config"
	contextpkg "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type FetchPageTool struct{}

func (t *FetchPageTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "fetch_page",
		Description:     "RAW HTML fetch - ONLY use when user explicitly asks to 'fetch' or 'download' a page. For general web queries, use search_web + analyze_content instead",
		Category:        CategoryWeb,
		RiskLevel:       RiskModerate,
		RequiresConfirm: true,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "url",
				Type:        "string",
				Required:    true,
				Description: "The URL to fetch",
				Example:     "https://docs.python.org/3/",
			},
		},
		Examples: []string{
			`{"tool": "fetch_page", "arguments": {"url": "https://docs.python.org/3/"}}`,
		},
	}
}

func (t *FetchPageTool) Validate(args map[string]interface{}) error {
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return fmt.Errorf("url parameter is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only http and https schemes are supported")
	}

	return nil
}

func (t *FetchPageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, _ := args["url"].(string)
	parsedURL, _ := url.Parse(urlStr)

	// Check domain whitelist if configured
	cfg := config.Get()
	if toolCfg, ok := cfg.Tools.Tools["fetch_page"]; ok {
		if len(toolCfg.AllowedDomains) > 0 {
			allowed := false
			for _, pattern := range toolCfg.AllowedDomains {
				if matchDomain(parsedURL.Host, pattern) {
					allowed = true
					break
				}
			}
			if !allowed {
				return "", fmt.Errorf("domain %s is not in the allowed list", parsedURL.Host)
			}
		}
	}

	// Get timeout and max size from config
	timeout := 30 * time.Second
	maxSize := int64(5 * 1024 * 1024) // 5MB default

	if toolCfg, ok := cfg.Tools.Tools["fetch_page"]; ok {
		if toolCfg.Timeout != nil {
			timeout = *toolCfg.Timeout
		}
		if toolCfg.MaxFileSize != nil {
			maxSize = int64(*toolCfg.MaxFileSize)
		}
	}

	// Fetch the page
	content, contentType, err := fetchURL(ctx, urlStr, timeout, maxSize)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}

	// Auto-store fetched content if context manager available and auto-store enabled
	manager := contextpkg.GetGlobalManager()
	if manager != nil && manager.IsAutoStoreEnabled() && manager.GetActiveContext() != "" {
		_, _ = manager.StoreArtifact(contextpkg.StoreArtifactRequest{
			ContextKey: manager.GetActiveContext(),
			Type:       contextpkg.ArtifactWebPage,
			Content:    content,
			Source:     "fetch_page",
			Agent:      "web_tools",
			Metadata: contextpkg.ArtifactMetadata{
				Tags: []string{"fetch", "web", urlStr, contentType},
			},
		})
	}

	// Format result
	result := fmt.Sprintf("Fetched: %s\n", urlStr)
	result += fmt.Sprintf("Content-Type: %s\n", contentType)
	result += fmt.Sprintf("Size: %d bytes\n\n", len(content))
	result += content

	// Truncate if too long (to prevent overwhelming the LLM)
	maxOutput := 50000
	if len(result) > maxOutput {
		result = result[:maxOutput] + "\n\n... (content truncated)"
	}

	return result, nil
}

func fetchURL(ctx context.Context, urlStr string, timeout time.Duration, maxSize int64) (string, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", urlStr, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Wilson Assistant/1.0 (Educational/Research Bot)")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")

	limitedReader := io.LimitReader(resp.Body, maxSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if int64(len(body)) > maxSize {
		return "", "", fmt.Errorf("content size exceeds maximum allowed size of %d bytes", maxSize)
	}

	return string(body), contentType, nil
}

func matchDomain(hostname, pattern string) bool {
	if pattern == hostname {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:]
		return strings.HasSuffix(hostname, "."+suffix) || hostname == suffix
	}

	if strings.HasSuffix(pattern, ".*") {
		prefix := pattern[:len(pattern)-2]
		return strings.HasPrefix(hostname, prefix+".")
	}

	return false
}

func init() {
	registry.Register(&FetchPageTool{})
}
