package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"wilson/core/registry"
	. "wilson/core/types"
)

// CoverageCheckTool checks test coverage
type CoverageCheckTool struct{}

func init() {
	registry.Register(&CoverageCheckTool{})
}

func (t *CoverageCheckTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "coverage_check",
		Description:     "Check test coverage using go test -cover. Verifies that code has adequate test coverage.",
		Category:        CategoryFileSystem,
		RiskLevel:       RiskSafe,
		RequiresConfirm: false,
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "package",
				Type:        "string",
				Required:    false,
				Description: "Package path to check (default: ./...)",
				Example:     "./agent",
			},
			{
				Name:        "min_coverage",
				Type:        "number",
				Required:    false,
				Description: "Minimum coverage percentage required (default: 80)",
				Example:     "80",
			},
		},
		Examples: []string{
			`{"tool": "coverage_check", "arguments": {}}`,
			`{"tool": "coverage_check", "arguments": {"package": "./agent", "min_coverage": 70}}`,
		},
	}
}

func (t *CoverageCheckTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *CoverageCheckTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	pkg := "./..."
	if p, ok := input["package"].(string); ok && p != "" {
		pkg = p
	}

	minCoverage := 80.0
	if mc, ok := input["min_coverage"].(float64); ok {
		minCoverage = mc
	}

	// Run go test with coverage
	cmd := exec.CommandContext(ctx, "go", "test", "-cover", pkg)
	output, err := cmd.CombinedOutput()

	// Parse coverage from output
	result := &CoverageResult{
		Package:     pkg,
		MinCoverage: minCoverage,
		Packages:    []PackageCoverage{},
	}

	lines := strings.Split(string(output), "\n")
	// Pattern: ok      package/path    0.123s  coverage: 85.6% of statements
	re := regexp.MustCompile(`^ok\s+([^\s]+)\s+[^\s]+\s+coverage:\s+([\d.]+)%`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches != nil && len(matches) >= 3 {
			coverage, _ := strconv.ParseFloat(matches[2], 64)
			result.Packages = append(result.Packages, PackageCoverage{
				Package:  matches[1],
				Coverage: coverage,
			})

			result.TotalCoverage += coverage
		}
	}

	// Calculate average
	if len(result.Packages) > 0 {
		result.TotalCoverage = result.TotalCoverage / float64(len(result.Packages))
	}

	result.PassedThreshold = result.TotalCoverage >= minCoverage

	// Handle test failures
	if err != nil {
		result.TestsFailed = true
		result.Error = string(output)
	}

	// Build JSON result
	resultMap := map[string]interface{}{
		"package":          pkg,
		"min_coverage":     minCoverage,
		"total_coverage":   fmt.Sprintf("%.1f", result.TotalCoverage),
		"passed_threshold": result.PassedThreshold,
		"tests_failed":     result.TestsFailed,
		"packages":         result.Packages,
	}

	if result.TestsFailed {
		resultMap["error"] = result.Error
	}

	resultJSON, err := json.MarshalIndent(resultMap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

// CoverageResult holds coverage check results
type CoverageResult struct {
	Package         string            `json:"package"`
	MinCoverage     float64           `json:"min_coverage"`
	TotalCoverage   float64           `json:"total_coverage"`
	PassedThreshold bool              `json:"passed_threshold"`
	TestsFailed     bool              `json:"tests_failed"`
	Packages        []PackageCoverage `json:"packages"`
	Error           string            `json:"error,omitempty"`
}

// PackageCoverage holds coverage for a single package
type PackageCoverage struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
}
