package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	contextpkg "wilson/context"
	"wilson/core/registry"
	. "wilson/core/types"
)

type ExtractContentTool struct{}

func (t *ExtractContentTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "extract_content",
		Description:     "Extract clean text content from HTML, removing scripts, styles, and navigation",
		Category:        CategoryWeb,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "html",
				Type:        "string",
				Required:    true,
				Description: "The HTML content to parse",
			},
		},
		Examples: []string{
			`{"tool": "extract_content", "arguments": {"html": "<html><body><h1>Title</h1><p>Content</p></body></html>"}}`,
		},
	}
}

func (t *ExtractContentTool) Validate(args map[string]interface{}) error {
	html, ok := args["html"].(string)
	if !ok || html == "" {
		return fmt.Errorf("html parameter is required")
	}
	return nil
}

func (t *ExtractContentTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	html, _ := args["html"].(string)

	// Parse HTML and extract content
	content, err := extractCleanText(html)
	if err != nil {
		return "", fmt.Errorf("failed to extract content: %w", err)
	}

	// Auto-store extracted content if context manager available and auto-store enabled
	manager := contextpkg.GetGlobalManager()
	if manager != nil && manager.IsAutoStoreEnabled() && manager.GetActiveContext() != "" {
		_, _ = manager.StoreArtifact(contextpkg.StoreArtifactRequest{
			ContextKey: manager.GetActiveContext(),
			Type:       contextpkg.ArtifactExtractedText,
			Content:    content,
			Source:     "extract_content",
			Agent:      "web_tools",
			Metadata: contextpkg.ArtifactMetadata{
				Tags: []string{"extract", "text", "web"},
			},
		})
	}

	return content, nil
}

func extractCleanText(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Remove unwanted elements
	doc.Find("script, style, nav, header, footer, iframe, noscript").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	// Extract title
	title := doc.Find("title").First().Text()

	// Extract main content
	var mainContent string

	// Try common content containers
	for _, selector := range []string{"main", "article", "#content", ".content", "#main", ".main"} {
		content := doc.Find(selector).First()
		if content.Length() > 0 {
			mainContent = content.Text()
			break
		}
	}

	// If no main content found, use body
	if mainContent == "" {
		mainContent = doc.Find("body").Text()
	}

	// Clean up whitespace
	mainContent = cleanWhitespace(mainContent)

	// Combine title and content
	var result strings.Builder
	if title != "" {
		result.WriteString(strings.TrimSpace(title))
		result.WriteString("\n\n")
	}
	result.WriteString(mainContent)

	return result.String(), nil
}

func cleanWhitespace(text string) string {
	lines := strings.Split(text, "\n")

	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func init() {
	registry.Register(&ExtractContentTool{})
}
