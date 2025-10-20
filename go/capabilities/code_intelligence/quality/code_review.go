package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// CodeReviewTool orchestrates all quality checks
type CodeReviewTool struct{}

func init() {
	registry.Register(&CodeReviewTool{})
}

func (t *CodeReviewTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "code_review",
		Description:     "Comprehensive code review that runs all quality checks: formatting, linting, security, complexity, and coverage. Returns aggregated quality score.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Path to review (default: current directory)",
				Example:     ".",
			},
			{
				Name:        "checks",
				Type:        "string",
				Required:    false,
				Description: "Comma-separated checks to run: format,lint,security,complexity,coverage (default: all)",
				Example:     "lint,security,complexity",
			},
		},
		Examples: []string{
			`{"tool": "code_review", "arguments": {}}`,
			`{"tool": "code_review", "arguments": {"path": "./agent"}}`,
			`{"tool": "code_review", "arguments": {"checks": "lint,security"}}`,
		},
	}
}

func (t *CodeReviewTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *CodeReviewTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	checks := "format,lint,security,complexity,coverage"
	if c, ok := input["checks"].(string); ok && c != "" {
		checks = c
	}

	// Run checks
	review := &CodeReview{
		Path:   path,
		Checks: checks,
		Results: map[string]interface{}{},
		Issues: []ReviewIssue{},
	}

	checkList := splitChecks(checks)

	// Run each check
	for _, check := range checkList {
		switch check {
		case "format":
			if err := review.runFormatCheck(ctx); err != nil {
				review.Errors = append(review.Errors, fmt.Sprintf("format check failed: %v", err))
			}
		case "lint":
			if err := review.runLintCheck(ctx); err != nil {
				review.Errors = append(review.Errors, fmt.Sprintf("lint check failed: %v", err))
			}
		case "security":
			if err := review.runSecurityCheck(ctx); err != nil {
				review.Errors = append(review.Errors, fmt.Sprintf("security check failed: %v", err))
			}
		case "complexity":
			if err := review.runComplexityCheck(ctx); err != nil {
				review.Errors = append(review.Errors, fmt.Sprintf("complexity check failed: %v", err))
			}
		case "coverage":
			if err := review.runCoverageCheck(ctx); err != nil {
				review.Errors = append(review.Errors, fmt.Sprintf("coverage check failed: %v", err))
			}
		}
	}

	// Calculate score
	review.OverallScore = review.calculateScore()
	review.Passed = review.OverallScore >= 80 && len(review.criticalIssues()) == 0

	// Build result
	result := map[string]interface{}{
		"path":           path,
		"checks_run":     checkList,
		"overall_score":  review.OverallScore,
		"passed":         review.Passed,
		"issues_count":   len(review.Issues),
		"critical_count": len(review.criticalIssues()),
		"issues":         review.Issues,
		"results":        review.Results,
	}

	if len(review.Errors) > 0 {
		result["errors"] = review.Errors
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// CodeReview holds review state
type CodeReview struct {
	Path         string
	Checks       string
	Results      map[string]interface{}
	Issues       []ReviewIssue
	Errors       []string
	OverallScore int
	Passed       bool
}

// ReviewIssue represents a code review issue
type ReviewIssue struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// runFormatCheck runs formatting check
func (r *CodeReview) runFormatCheck(ctx context.Context) error {
	tool, err := registry.GetTool("format_code")
	if err != nil {
		return err
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": r.Path,
	})
	if err != nil {
		return err
	}

	// Parse result to extract issues
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err == nil {
		r.Results["format"] = data
		if formatted, ok := data["files_formatted"].(float64); ok && formatted > 0 {
			r.Issues = append(r.Issues, ReviewIssue{
				Type:     "format",
				Severity: "info",
				Message:  fmt.Sprintf("%.0f files needed formatting", formatted),
			})
		}
	}

	return nil
}

// runLintCheck runs linting check
func (r *CodeReview) runLintCheck(ctx context.Context) error {
	tool, err := registry.GetTool("lint_code")
	if err != nil {
		return err
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": r.Path,
	})
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err == nil {
		r.Results["lint"] = data
		if issues, ok := data["issues"].([]interface{}); ok {
			for _, issue := range issues {
				if issueMap, ok := issue.(map[string]interface{}); ok {
					r.Issues = append(r.Issues, ReviewIssue{
						Type:     "lint",
						Severity: getString(issueMap, "severity"),
						File:     getString(issueMap, "file"),
						Line:     getInt(issueMap, "line"),
						Message:  getString(issueMap, "message"),
					})
				}
			}
		}
	}

	return nil
}

// runSecurityCheck runs security scan
func (r *CodeReview) runSecurityCheck(ctx context.Context) error {
	tool, err := registry.GetTool("security_scan")
	if err != nil {
		return err
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": r.Path,
	})
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err == nil {
		r.Results["security"] = data
		if issues, ok := data["issues"].([]interface{}); ok {
			for _, issue := range issues {
				if issueMap, ok := issue.(map[string]interface{}); ok {
					r.Issues = append(r.Issues, ReviewIssue{
						Type:     "security",
						Severity: getString(issueMap, "severity"),
						File:     getString(issueMap, "file"),
						Line:     getInt(issueMap, "line"),
						Message:  getString(issueMap, "message"),
					})
				}
			}
		}
	}

	return nil
}

// runComplexityCheck runs complexity analysis
func (r *CodeReview) runComplexityCheck(ctx context.Context) error {
	tool, err := registry.GetTool("complexity_check")
	if err != nil {
		return err
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": r.Path,
	})
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err == nil {
		r.Results["complexity"] = data
		if violations, ok := data["violations"].([]interface{}); ok {
			for _, violation := range violations {
				if vMap, ok := violation.(map[string]interface{}); ok {
					r.Issues = append(r.Issues, ReviewIssue{
						Type:     "complexity",
						Severity: "warning",
						File:     getString(vMap, "file"),
						Line:     getInt(vMap, "line"),
						Message:  getString(vMap, "suggestion"),
					})
				}
			}
		}
	}

	return nil
}

// runCoverageCheck runs coverage check
func (r *CodeReview) runCoverageCheck(ctx context.Context) error {
	tool, err := registry.GetTool("coverage_check")
	if err != nil {
		return err
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"package": r.Path,
	})
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err == nil {
		r.Results["coverage"] = data
		if passed, ok := data["passed_threshold"].(bool); ok && !passed {
			r.Issues = append(r.Issues, ReviewIssue{
				Type:     "coverage",
				Severity: "warning",
				Message:  fmt.Sprintf("Coverage below threshold: %v", data["total_coverage"]),
			})
		}
	}

	return nil
}

// calculateScore calculates overall quality score
func (r *CodeReview) calculateScore() int {
	score := 100

	// Deduct points for issues
	for _, issue := range r.Issues {
		switch issue.Severity {
		case "critical":
			score -= 20
		case "high":
			score -= 10
		case "error":
			score -= 10
		case "warning":
			score -= 3
		case "medium":
			score -= 3
		case "info":
			score -= 1
		case "low":
			score -= 1
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// criticalIssues returns critical issues
func (r *CodeReview) criticalIssues() []ReviewIssue {
	var critical []ReviewIssue
	for _, issue := range r.Issues {
		if issue.Severity == "critical" || issue.Severity == "high" {
			critical = append(critical, issue)
		}
	}
	return critical
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func splitChecks(checks string) []string {
	var result []string
	for _, check := range strings.Split(checks, ",") {
		check = strings.TrimSpace(check)
		if check != "" {
			result = append(result, check)
		}
	}
	return result
}
