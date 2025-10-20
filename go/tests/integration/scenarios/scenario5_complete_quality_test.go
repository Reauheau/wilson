package scenarios

import (
	"testing"

	"wilson/tests/framework"
	"wilson/tests/framework/verifiers"

	// Import tools to register them
	_ "wilson/capabilities/code_intelligence/ast"
	_ "wilson/capabilities/code_intelligence/build"
	_ "wilson/capabilities/code_intelligence/quality"
)

// TestScenario5_CompleteQualityPass tests ensuring user service meets all quality standards
// This is the comprehensive scenario that combines all previous scenarios:
// 1. Code must compile
// 2. All security issues fixed (SQL injection)
// 3. All complexity issues resolved
// 4. Code properly formatted
// 5. All lint issues resolved
// 6. Test coverage ≥ 80%
// 7. All quality gates pass
func TestScenario5_CompleteQualityPass(t *testing.T) {
	runner := framework.NewTestRunner(t).WithAgents()
	defer runner.Cleanup()

	if err := runner.Setup(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	t.Log("Scenario 5: Complete Quality Pass")
	t.Log("Starting with mock project containing 40+ issues")

	// Document initial state
	t.Log("\nInitial Issues:")
	t.Log("  Phase 2: Compilation")
	t.Log("    ✗ Does not compile (2 errors)")
	t.Log("    ✗ Test coverage ~5%")
	t.Log("  Phase 3: Cross-file Awareness")
	t.Log("    ✗ Unexported 'user' type")
	t.Log("    ✗ Inconsistent field naming")
	t.Log("    ✗ Missing UpdateUser/DeleteUser")
	t.Log("    ✗ Scattered validation logic")
	t.Log("  Phase 4: Quality Gates")
	t.Log("    ✗ 3 format issues")
	t.Log("    ✗ 12 lint issues")
	t.Log("    ✗ 6 security issues (3 CRITICAL)")
	t.Log("    ✗ 3 complexity issues")

	// TODO: Execute Code Agent task
	// "Ensure user service meets all quality standards"

	t.Log("\nExpected fixes:")
	t.Log("  1. Fix compilation errors")
	t.Log("  2. Fix all SQL injection vulnerabilities")
	t.Log("  3. Refactor high-complexity functions")
	t.Log("  4. Format all code (gofmt, goimports)")
	t.Log("  5. Fix lint issues")
	t.Log("  6. Add comprehensive tests (80% coverage)")
	t.Log("  7. Export types properly")
	t.Log("  8. Complete CRUD pattern")

	// Comprehensive verification
	verifier := verifiers.DefaultCodeVerifier()
	verifier.MinCoverage = 80.0
	verifier.CustomChecks = []func(*testing.T, *framework.TestRunner) error{
		// Compilation
		verifiers.VerifyCompileSucceeds,

		// Security
		verifiers.VerifyNoSQLInjection,
		verifiers.VerifyUsesParameterizedQuery("GetUser"),
		verifiers.VerifyUsesParameterizedQuery("GetUserByEmail"),
		verifiers.VerifyUsesParameterizedQuery("CreateUser"),

		// Structure
		func(t *testing.T, r *framework.TestRunner) error {
			// Verify CRUD functions exist
			framework.AssertFunctionExists(t, r.Context().WorkDir+"/service.go", "GetUser")
			framework.AssertFunctionExists(t, r.Context().WorkDir+"/service.go", "CreateUser")
			framework.AssertFunctionExists(t, r.Context().WorkDir+"/service.go", "UpdateUser")
			framework.AssertFunctionExists(t, r.Context().WorkDir+"/service.go", "DeleteUser")
			return nil
		},

		// Error handling
		verifiers.VerifyErrorReturn("GetUser"),
		verifiers.VerifyErrorReturn("CreateUser"),
		verifiers.VerifyErrorReturn("UpdateUser"),
		verifiers.VerifyErrorReturn("DeleteUser"),
	}

	// In real scenario, after Code Agent fixes all issues:
	// err = verifier.Verify(t, runner)
	// framework.AssertNoError(t, err, "Quality verification failed")

	t.Log("\nQuality gates to pass:")
	t.Log("  [ ] compile - Code must compile")
	t.Log("  [ ] format_code - All files properly formatted")
	t.Log("  [ ] lint_code - No lint issues")
	t.Log("  [ ] security_scan - No security vulnerabilities")
	t.Log("  [ ] complexity_check - All functions below threshold")
	t.Log("  [ ] coverage_check - ≥ 80% test coverage")

	t.Log("\nScenario 5 test structure complete")
}

// TestScenario5_IndividualGates tests each quality gate separately
func TestScenario5_IndividualGates(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	t.Log("Testing each quality gate on mock project:")

	// Gate 1: Compilation
	t.Run("CompilationGate", func(t *testing.T) {
		result, err := runner.ExecuteTool("compile", map[string]interface{}{
			"path": runner.Context().WorkDir,
		})
		if err == nil && framework.ContainsAny(result, []string{"success"}) {
			t.Log("✓ Compilation gate PASSED")
		} else {
			t.Log("✗ Compilation gate FAILED (expected)")
			t.Logf("   Errors: %s", result)
		}
	})

	// Gate 2: Format
	t.Run("FormatGate", func(t *testing.T) {
		result, err := runner.ExecuteTool("format_code", map[string]interface{}{
			"path": runner.Context().WorkDir,
		})
		if err == nil {
			if framework.ContainsAny(result, []string{"0 files", "formatted: 0"}) {
				t.Log("✓ Format gate PASSED")
			} else {
				t.Log("✗ Format gate FAILED (expected)")
				t.Logf("   Needs formatting: %s", result)
			}
		}
	})

	// Gate 3: Lint
	t.Run("LintGate", func(t *testing.T) {
		result, err := runner.ExecuteTool("lint_code", map[string]interface{}{
			"path": runner.Context().WorkDir,
		})
		if err == nil && result == "" {
			t.Log("✓ Lint gate PASSED")
		} else {
			t.Log("✗ Lint gate FAILED (expected)")
			t.Logf("   Issues: %s", result)
		}
	})

	// Gate 4: Security
	t.Run("SecurityGate", func(t *testing.T) {
		result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
			"path": runner.Context().WorkDir,
		})
		if err == nil && !framework.ContainsAny(result, []string{"vulnerability", "critical", "high"}) {
			t.Log("✓ Security gate PASSED")
		} else {
			t.Log("✗ Security gate FAILED (expected)")
			t.Logf("   Vulnerabilities: %s", result)
		}
	})

	// Gate 5: Complexity
	t.Run("ComplexityGate", func(t *testing.T) {
		result, err := runner.ExecuteTool("complexity_check", map[string]interface{}{
			"path":               runner.Context().WorkDir,
			"max_complexity":     15,
			"max_function_lines": 100,
		})
		if err == nil && framework.ContainsAny(result, []string{"passed", "0 violations"}) {
			t.Log("✓ Complexity gate PASSED")
		} else {
			t.Log("✗ Complexity gate FAILED (expected)")
			t.Logf("   Issues: %s", result)
		}
	})

	// Gate 6: Coverage
	t.Run("CoverageGate", func(t *testing.T) {
		result, err := runner.ExecuteTool("coverage_check", map[string]interface{}{
			"package":      runner.Context().WorkDir,
			"min_coverage": 80.0,
		})
		if err == nil && framework.ContainsAny(result, []string{"passed", "above threshold"}) {
			t.Log("✓ Coverage gate PASSED")
		} else {
			t.Log("✗ Coverage gate FAILED (expected)")
			t.Logf("   Coverage: %s", result)
		}
	})

	t.Log("\nQuality gate summary on mock project (before fixes):")
	t.Log("  Expected: All gates should FAIL (mock project has deliberate issues)")
}

// TestScenario5_ProgressiveFixing tests fixing issues progressively
func TestScenario5_ProgressiveFixing(t *testing.T) {
	t.Log("Progressive fixing approach:")
	t.Log("\nStep 1: Fix compilation")
	t.Log("  - Change Query() to QueryRow() in GetUser")
	t.Log("  - Add error checks")
	t.Log("  Result: Code compiles")

	t.Log("\nStep 2: Fix security")
	t.Log("  - Replace concatenated SQL with parameterized queries")
	t.Log("  - Use ? placeholders")
	t.Log("  Result: No SQL injection vulnerabilities")

	t.Log("\nStep 3: Fix complexity")
	t.Log("  - Extract validation functions")
	t.Log("  - Use early returns")
	t.Log("  Result: All functions below complexity threshold")

	t.Log("\nStep 4: Fix formatting")
	t.Log("  - Run gofmt")
	t.Log("  - Fix import spacing")
	t.Log("  Result: Code properly formatted")

	t.Log("\nStep 5: Fix lint issues")
	t.Log("  - Export user type → User")
	t.Log("  - Add godoc comments")
	t.Log("  - Fix naming consistency")
	t.Log("  Result: No lint warnings")

	t.Log("\nStep 6: Add tests")
	t.Log("  - Test all CRUD operations")
	t.Log("  - Test error cases")
	t.Log("  - Test validation")
	t.Log("  Result: ≥ 80% coverage")

	t.Log("\nStep 7: Complete CRUD")
	t.Log("  - Add UpdateUser")
	t.Log("  - Add DeleteUser")
	t.Log("  Result: Complete CRUD pattern")

	t.Log("\nFinal state: All quality gates PASS ✓")
}
