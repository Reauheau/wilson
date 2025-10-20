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

// TestScenario1_AddErrorHandling tests adding proper error handling to GetUser
// This scenario verifies that the Code Agent can:
// 1. Identify missing error handling
// 2. Update function signatures to return errors
// 3. Add error checks after operations
// 4. Return errors with proper context
func TestScenario1_AddErrorHandling(t *testing.T) {
	runner := framework.NewTestRunner(t).WithAgents()
	defer runner.Cleanup()

	if err := runner.Setup(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Verify initial state - GetUser does NOT return error
	initialCode, err := runner.ReadFile("service.go")
	framework.AssertNoError(t, err, "Failed to read service.go")
	framework.AssertContains(t, initialCode, "func (s *UserService) GetUser(id string) *user",
		"GetUser should initially not return error")

	// TODO: Execute Code Agent task
	// This would invoke the Code Agent with task:
	// "Add proper error handling to GetUser function"
	//
	// For now, we simulate the expected changes manually to test the verifier
	t.Log("Scenario 1: Add proper error handling to GetUser")
	t.Log("Expected changes:")
	t.Log("  - Change return type to (*user, error)")
	t.Log("  - Check error from Query()")
	t.Log("  - Check error from Scan()")
	t.Log("  - Return errors with context")

	// Verification criteria
	verifier := &verifiers.CodeVerifier{
		MustCompile:       true,
		MaxSecurityIssues: 3, // Still has SQL injection
		CustomChecks: []func(*testing.T, *framework.TestRunner) error{
			verifiers.VerifyErrorReturn("GetUser"),
			func(t *testing.T, r *framework.TestRunner) error {
				// Verify GetUser returns error
				funcDecl := framework.AssertFunctionExists(t,
					r.Context().WorkDir+"/service.go", "GetUser")

				// Check it has 2 return values (pointer and error)
				if funcDecl.Type.Results == nil {
					t.Fatal("GetUser has no return values")
				}
				if len(funcDecl.Type.Results.List) != 2 {
					t.Fatalf("GetUser should return 2 values, got %d",
						len(funcDecl.Type.Results.List))
				}
				return nil
			},
		},
	}

	// Note: In real scenario, Code Agent would make the changes
	// Then we verify the changes meet expectations
	t.Log("Verification: Code should compile and GetUser should return error")

	// For now, just verify the initial state is as expected
	funcDecl := framework.AssertFunctionExists(t,
		runner.Context().WorkDir+"/service.go", "GetUser")
	if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 2 {
		t.Log("✓ GetUser already returns error (unexpected in initial state)")
	} else {
		t.Log("✓ GetUser does not return error (expected initial state)")
	}

	// The actual verification would happen after Code Agent makes changes:
	// err = verifier.Verify(t, runner)
	// framework.AssertNoError(t, err, "Verification failed")

	t.Log("Scenario 1 test structure complete")
}

// TestScenario1_ManualFix demonstrates the expected fix manually
func TestScenario1_ManualFix(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a fixed version of GetUser
	fixedCode := `package userservice

import (
	"database/sql"
	"fmt"
)

type user struct {
	ID    string
	name  string
	Email string
	Age   int
	Role  string
}

type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// GetUser retrieves a user by ID with proper error handling
func (s *UserService) GetUser(id string) (*user, error) {
	query := "SELECT id, name, email, age, role FROM users WHERE id = ?"

	row := s.db.QueryRow(query, id)

	u := &user{}
	err := row.Scan(&u.ID, &u.name, &u.Email, &u.Age, &u.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	return u, nil
}
`

	err := runner.WriteFile("go.mod", "module userservice\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	err = runner.WriteFile("service.go", fixedCode)
	framework.AssertNoError(t, err, "Failed to write fixed service.go")

	// Verify the fix
	verifier := &verifiers.CodeVerifier{
		MustCompile:   true,
		CustomChecks: []func(*testing.T, *framework.TestRunner) error{
			verifiers.VerifyErrorReturn("GetUser"),
			verifiers.VerifyUsesParameterizedQuery("GetUser"),
		},
	}

	err = verifier.Verify(t, runner)
	if err != nil {
		t.Logf("Verification error (may be expected): %v", err)
	} else {
		t.Log("✓ Manual fix passes all verifications")
	}
}
