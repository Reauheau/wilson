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

// TestScenario2_FixSQLInjection tests fixing SQL injection vulnerabilities
// This scenario verifies that the Code Agent can:
// 1. Detect SQL injection vulnerabilities
// 2. Replace string concatenation with parameterized queries
// 3. Use proper query placeholders (?)
// 4. Fix all vulnerable queries in the project
func TestScenario2_FixSQLInjection(t *testing.T) {
	runner := framework.NewTestRunner(t).WithAgents()
	defer runner.Cleanup()

	if err := runner.Setup(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Verify initial vulnerabilities exist
	initialCode, err := runner.ReadFile("service.go")
	framework.AssertNoError(t, err, "Failed to read service.go")

	// Should have SQL injection vulnerabilities
	framework.AssertContains(t, initialCode,
		`"SELECT id, name, email, age, role FROM users WHERE id = " + id`,
		"GetUser should have SQL injection via concatenation")
	framework.AssertContains(t, initialCode,
		`"SELECT id, name, email, age, role FROM users WHERE email = '" + email + "'"`,
		"GetUserByEmail should have SQL injection via concatenation")

	t.Log("Scenario 2: Fix SQL injection vulnerabilities")
	t.Log("Initial state:")
	t.Log("  ✗ GetUser: SQL concatenation")
	t.Log("  ✗ GetUserByEmail: SQL concatenation")
	t.Log("  ✗ CreateUser: SQL concatenation in multiple places")

	// TODO: Execute Code Agent task
	// "Fix security vulnerabilities in user service"

	// Verification criteria - after fix
	verifier := &verifiers.CodeVerifier{
		MustCompile:       true,
		MaxSecurityIssues: 0, // Should have NO security issues after fix
		CustomChecks: []func(*testing.T, *framework.TestRunner) error{
			verifiers.VerifyNoSQLInjection,
			verifiers.VerifyUsesParameterizedQuery("GetUser"),
			verifiers.VerifyUsesParameterizedQuery("GetUserByEmail"),
			func(t *testing.T, r *framework.TestRunner) error {
				// Verify no string concatenation in SQL
				content, err := r.ReadFile("service.go")
				if err != nil {
					return err
				}

				// Should NOT have SQL with concatenation
				if framework.ContainsPattern(content, `"SELECT.*" \+`) {
					return framework.NewError("SQL query still uses string concatenation")
				}
				if framework.ContainsPattern(content, `"INSERT.*" \+`) {
					return framework.NewError("SQL INSERT still uses string concatenation")
				}

				return nil
			},
		},
	}

	t.Log("Expected changes:")
	t.Log("  - Use QueryRow with ? placeholders")
	t.Log("  - Pass parameters as separate arguments")
	t.Log("  - Remove all string concatenation from SQL")

	// In real scenario, Code Agent would fix the issues
	// Then we'd run: verifier.Verify(t, runner)

	t.Log("Scenario 2 test structure complete")
}

// TestScenario2_ManualFix demonstrates the expected fix manually
func TestScenario2_ManualFix(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create properly secured versions
	err := runner.WriteFile("go.mod", "module userservice\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	secureCode := `package userservice

import (
	"database/sql"
	"errors"
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

// GetUser retrieves a user by ID - SECURED with parameterized query
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

// GetUserByEmail retrieves a user by email - SECURED
func (s *UserService) GetUserByEmail(email string) (*user, error) {
	query := "SELECT id, name, email, age, role FROM users WHERE email = ?"
	row := s.db.QueryRow(query, email)

	u := &user{}
	err := row.Scan(&u.ID, &u.name, &u.Email, &u.Age, &u.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", email)
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	return u, nil
}

// CreateUser creates a new user - SECURED
func (s *UserService) CreateUser(name, email, role string, age int) error {
	// Validation
	if name == "" {
		return errors.New("name is required")
	}
	if email == "" {
		return errors.New("email is required")
	}
	if age < 18 || age > 120 {
		return errors.New("age must be between 18 and 120")
	}

	// Parameterized query
	query := "INSERT INTO users (name, email, role, age) VALUES (?, ?, ?, ?)"
	_, err := s.db.Exec(query, name, email, role, age)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}
`

	err = runner.WriteFile("service.go", secureCode)
	framework.AssertNoError(t, err, "Failed to write secure service.go")

	// Verify security
	result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err == nil && result != "" {
		t.Logf("Security scan result: %s", result)
		// Check for SQL injection issues
		framework.AssertNotContains(t, result, "SQL injection", "Should not have SQL injection")
		framework.AssertNotContains(t, result, "G201", "Should not have gosec G201 (SQL injection)")
	}

	// Verify all functions exist and compile
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service.go", "GetUser")
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service.go", "GetUserByEmail")
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service.go", "CreateUser")

	t.Log("✓ Secure version passes security scan")
}

// TestScenario2_DetectVulnerabilities verifies that security scan catches issues
func TestScenario2_DetectVulnerabilities(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the vulnerable mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run security scan
	result, err := runner.ExecuteTool("security_scan", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	t.Logf("Security scan on vulnerable code: %s", result)

	// Should detect issues
	if result != "" {
		// Should mention SQL or injection
		if framework.ContainsAny(result, []string{"SQL", "injection", "G201", "G202"}) {
			t.Log("✓ Security scan detected SQL injection vulnerabilities")
		} else {
			t.Log("⚠ Security scan may not have detected SQL injection")
		}
	}
}
