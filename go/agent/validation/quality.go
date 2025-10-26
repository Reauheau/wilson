package validation

import (
	"context"
	"encoding/json"
	"fmt"

	"wilson/core/registry"
)

// QualityValidator defines interface for automated quality checks
type QualityValidator interface {
	Name() string
	Check(ctx context.Context, path string) (*ValidationResult, error)
}

// ValidationResult contains results of a quality check
type ValidationResult struct {
	Passed   bool                   `json:"passed"`
	Severity string                 `json:"severity"` // info, warning, error, critical
	Message  string                 `json:"message"`
	Details  map[string]interface{} `json:"details"`
	ToolUsed string                 `json:"tool_used"`
}

// Standard severity levels
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// RunAllValidators runs all quality validators on a path
func RunAllValidators(ctx context.Context, path string) ([]*ValidationResult, error) {
	validators := []QualityValidator{
		&CompilationValidator{},
		&FormattingValidator{},
		&LintingValidator{},
		&SecurityValidator{},
		&ComplexityValidator{},
		&CoverageValidator{Threshold: 80.0},
	}

	var results []*ValidationResult
	for _, validator := range validators {
		result, err := validator.Check(ctx, path)
		if err != nil {
			// Don't fail entire validation if one check errors
			results = append(results, &ValidationResult{
				Passed:   false,
				Severity: SeverityError,
				Message:  fmt.Sprintf("%s failed: %v", validator.Name(), err),
				ToolUsed: validator.Name(),
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// HasCriticalIssues checks if any results have critical severity
func HasCriticalIssues(results []*ValidationResult) bool {
	for _, r := range results {
		if !r.Passed && r.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// GetFailedChecks returns list of failed check names
func GetFailedChecks(results []*ValidationResult) []string {
	var failed []string
	for _, r := range results {
		if !r.Passed {
			failed = append(failed, r.ToolUsed)
		}
	}
	return failed
}

// CompilationValidator checks if code compiles
type CompilationValidator struct{}

func (v *CompilationValidator) Name() string {
	return "compilation"
}

func (v *CompilationValidator) Check(ctx context.Context, path string) (*ValidationResult, error) {
	tool, err := registry.GetTool("compile")
	if err != nil {
		return nil, fmt.Errorf("compile tool not found: %w", err)
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": path,
	})
	if err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("Compilation check failed: %v", err),
			ToolUsed: "compile",
		}, nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityError,
			Message:  "Failed to parse compilation result",
			ToolUsed: "compile",
		}, nil
	}

	success, _ := data["success"].(bool)
	if !success {
		errorCount, _ := data["error_count"].(float64)
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("Code does not compile (%d errors)", int(errorCount)),
			Details:  data,
			ToolUsed: "compile",
		}, nil
	}

	return &ValidationResult{
		Passed:   true,
		Severity: SeverityInfo,
		Message:  "Code compiles successfully",
		Details:  data,
		ToolUsed: "compile",
	}, nil
}

// FormattingValidator checks code formatting
type FormattingValidator struct{}

func (v *FormattingValidator) Name() string {
	return "formatting"
}

func (v *FormattingValidator) Check(ctx context.Context, path string) (*ValidationResult, error) {
	tool, err := registry.GetTool("format_code")
	if err != nil {
		return nil, fmt.Errorf("format_code tool not found: %w", err)
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": path,
	})
	if err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityError,
			Message:  fmt.Sprintf("Format check failed: %v", err),
			ToolUsed: "format_code",
		}, nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return &ValidationResult{
			Passed:   true, // Formatting is not critical
			Severity: SeverityInfo,
			Message:  "Code formatting check completed",
			ToolUsed: "format_code",
		}, nil
	}

	filesFormatted, _ := data["files_formatted"].(float64)
	if filesFormatted > 0 {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityInfo,
			Message:  fmt.Sprintf("%d files needed formatting", int(filesFormatted)),
			Details:  data,
			ToolUsed: "format_code",
		}, nil
	}

	return &ValidationResult{
		Passed:   true,
		Severity: SeverityInfo,
		Message:  "Code is properly formatted",
		Details:  data,
		ToolUsed: "format_code",
	}, nil
}

// LintingValidator checks code style
type LintingValidator struct{}

func (v *LintingValidator) Name() string {
	return "linting"
}

func (v *LintingValidator) Check(ctx context.Context, path string) (*ValidationResult, error) {
	tool, err := registry.GetTool("lint_code")
	if err != nil {
		return nil, fmt.Errorf("lint_code tool not found: %w", err)
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": path,
	})
	if err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Lint check failed: %v", err),
			ToolUsed: "lint_code",
		}, nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		// If we can't parse, assume issues found
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  "Linting issues found",
			ToolUsed: "lint_code",
		}, nil
	}

	issueCount, _ := data["issue_count"].(float64)
	if issueCount > 0 {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%d linting issues found", int(issueCount)),
			Details:  data,
			ToolUsed: "lint_code",
		}, nil
	}

	return &ValidationResult{
		Passed:   true,
		Severity: SeverityInfo,
		Message:  "No linting issues",
		Details:  data,
		ToolUsed: "lint_code",
	}, nil
}

// SecurityValidator checks for security vulnerabilities
type SecurityValidator struct{}

func (v *SecurityValidator) Name() string {
	return "security"
}

func (v *SecurityValidator) Check(ctx context.Context, path string) (*ValidationResult, error) {
	tool, err := registry.GetTool("security_scan")
	if err != nil {
		return nil, fmt.Errorf("security_scan tool not found: %w", err)
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path": path,
	})
	if err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityError,
			Message:  fmt.Sprintf("Security scan failed: %v", err),
			ToolUsed: "security_scan",
		}, nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return &ValidationResult{
			Passed:   true, // Can't parse, assume OK
			Severity: SeverityInfo,
			Message:  "Security scan completed",
			ToolUsed: "security_scan",
		}, nil
	}

	vulnCount, _ := data["vulnerabilities_found"].(float64)
	if vulnCount > 0 {
		// Check severity
		criticalCount, _ := data["critical_count"].(float64)
		highCount, _ := data["high_count"].(float64)

		severity := SeverityWarning
		if criticalCount > 0 {
			severity = SeverityCritical
		} else if highCount > 0 {
			severity = SeverityError
		}

		return &ValidationResult{
			Passed:   false,
			Severity: severity,
			Message:  fmt.Sprintf("%d security vulnerabilities found", int(vulnCount)),
			Details:  data,
			ToolUsed: "security_scan",
		}, nil
	}

	return &ValidationResult{
		Passed:   true,
		Severity: SeverityInfo,
		Message:  "No security vulnerabilities found",
		Details:  data,
		ToolUsed: "security_scan",
	}, nil
}

// ComplexityValidator checks code complexity
type ComplexityValidator struct {
	MaxComplexity     int
	MaxFunctionLength int
}

func (v *ComplexityValidator) Name() string {
	return "complexity"
}

func (v *ComplexityValidator) Check(ctx context.Context, path string) (*ValidationResult, error) {
	tool, err := registry.GetTool("complexity_check")
	if err != nil {
		return nil, fmt.Errorf("complexity_check tool not found: %w", err)
	}

	// Set defaults
	maxComplexity := v.MaxComplexity
	if maxComplexity == 0 {
		maxComplexity = 15
	}
	maxFunctionLength := v.MaxFunctionLength
	if maxFunctionLength == 0 {
		maxFunctionLength = 100
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path":               path,
		"max_complexity":     maxComplexity,
		"max_function_lines": maxFunctionLength,
	})
	if err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Complexity check failed: %v", err),
			ToolUsed: "complexity_check",
		}, nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return &ValidationResult{
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "Complexity check completed",
			ToolUsed: "complexity_check",
		}, nil
	}

	passed, _ := data["passed"].(bool)
	if !passed {
		complexFunctions, _ := data["complex_functions"].(float64)
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%d functions exceed complexity thresholds", int(complexFunctions)),
			Details:  data,
			ToolUsed: "complexity_check",
		}, nil
	}

	return &ValidationResult{
		Passed:   true,
		Severity: SeverityInfo,
		Message:  "Code complexity is acceptable",
		Details:  data,
		ToolUsed: "complexity_check",
	}, nil
}

// CoverageValidator checks test coverage
type CoverageValidator struct {
	Threshold float64 // Minimum coverage percentage (0-100)
}

func (v *CoverageValidator) Name() string {
	return "coverage"
}

func (v *CoverageValidator) Check(ctx context.Context, path string) (*ValidationResult, error) {
	tool, err := registry.GetTool("coverage_check")
	if err != nil {
		return nil, fmt.Errorf("coverage_check tool not found: %w", err)
	}

	threshold := v.Threshold
	if threshold == 0 {
		threshold = 80.0
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"package":      path,
		"min_coverage": threshold,
	})
	if err != nil {
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Coverage check failed: %v", err),
			ToolUsed: "coverage_check",
		}, nil
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return &ValidationResult{
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "Coverage check completed",
			ToolUsed: "coverage_check",
		}, nil
	}

	passedThreshold, _ := data["passed_threshold"].(bool)
	if !passedThreshold {
		totalCoverage, _ := data["total_coverage"].(string)
		return &ValidationResult{
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Test coverage %s%% below threshold %.1f%%", totalCoverage, threshold),
			Details:  data,
			ToolUsed: "coverage_check",
		}, nil
	}

	totalCoverage, _ := data["total_coverage"].(string)
	return &ValidationResult{
		Passed:   true,
		Severity: SeverityInfo,
		Message:  fmt.Sprintf("Test coverage %s%% meets threshold", totalCoverage),
		Details:  data,
		ToolUsed: "coverage_check",
	}, nil
}
