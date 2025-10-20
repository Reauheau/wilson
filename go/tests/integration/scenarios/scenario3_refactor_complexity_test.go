package scenarios

import (
	"testing"

	"wilson/tests/framework"
)

// TestScenario3_RefactorComplexFunction tests refactoring CreateUser to reduce complexity
// This scenario verifies that the Code Agent can:
// 1. Detect high cyclomatic complexity
// 2. Extract validation logic into helper functions
// 3. Use early returns to reduce nesting
// 4. Reduce complexity below threshold (15)
// 5. Reduce function length below 100 lines
func TestScenario3_RefactorComplexFunction(t *testing.T) {
	runner := framework.NewTestRunner(t).WithAgents()
	defer runner.Cleanup()

	if err := runner.Setup(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Analyze initial complexity
	initialCode, err := runner.ReadFile("service.go")
	framework.AssertNoError(t, err, "Failed to read service.go")

	// Count nesting levels - should be deep (8 levels)
	framework.AssertContains(t, initialCode, "\t\t\t\t\t\t\t\t\t\t",
		"Should have deeply nested code")

	t.Log("Scenario 3: Refactor CreateUser to reduce complexity")
	t.Log("Initial state:")
	t.Log("  - Cyclomatic complexity: ~25")
	t.Log("  - Function length: ~90 lines")
	t.Log("  - Nesting depth: 8 levels")

	// Parse and calculate complexity
	node, _ := framework.ParseGoFile(t, runner.Context().WorkDir+"/service.go")
	createUserFunc := framework.FindFunction(node, "CreateUser")
	if createUserFunc == nil {
		t.Fatal("CreateUser function not found")
	}

	initialComplexity := framework.CalculateComplexity(createUserFunc)
	t.Logf("Initial CreateUser complexity: %d", initialComplexity)

	if initialComplexity <= 15 {
		t.Error("Expected CreateUser to have complexity > 15 initially")
	}

	// TODO: Execute Code Agent task
	// "Refactor CreateUser to reduce complexity"

	t.Log("Expected changes:")
	t.Log("  1. Extract validation into validateUser() function")
	t.Log("  2. Use early returns for error cases")
	t.Log("  3. Simplify nested if statements")
	t.Log("  4. Reduce complexity below 15")
	t.Log("  5. Maintain same functionality")

	// Verification would check:
	// - Complexity < 15
	// - Function length < 100 lines
	// - Still compiles
	// - Tests still pass
	// - Functionality preserved

	t.Log("Scenario 3 test structure complete")
}

// TestScenario3_ManualRefactoring demonstrates the expected refactoring
func TestScenario3_ManualRefactoring(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	err := runner.WriteFile("go.mod", "module userservice\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Refactored version with reduced complexity
	refactoredCode := `package userservice

import (
	"database/sql"
	"errors"
	"fmt"
)

type UserService struct {
	db *sql.DB
}

type CreateUserParams struct {
	Name     string
	Email    string
	Role     string
	Age      int
	Active   bool
	Verified bool
	Premium  bool
}

// validateUserParams validates user creation parameters
func validateUserParams(params CreateUserParams) error {
	if params.Name == "" {
		return errors.New("name is required")
	}
	if len(params.Name) < 2 {
		return errors.New("name must be at least 2 characters")
	}

	if params.Email == "" {
		return errors.New("email is required")
	}
	if len(params.Email) < 5 {
		return errors.New("email must be at least 5 characters")
	}

	if params.Role == "" {
		return errors.New("role is required")
	}
	validRoles := map[string]bool{"admin": true, "user": true, "guest": true}
	if !validRoles[params.Role] {
		return errors.New("invalid role: must be admin, user, or guest")
	}

	if params.Age < 18 {
		return errors.New("user must be 18 or older")
	}
	if params.Age > 120 {
		return errors.New("age must be 120 or less")
	}

	if !params.Active {
		return errors.New("cannot create inactive user")
	}

	if !params.Verified && params.Premium {
		return errors.New("cannot create premium user without verification")
	}

	return nil
}

// CreateUser creates a new user with simplified logic
func (s *UserService) CreateUser(name, email, role string, age int, active, verified, premium bool) error {
	params := CreateUserParams{
		Name:     name,
		Email:    email,
		Role:     role,
		Age:      age,
		Active:   active,
		Verified: verified,
		Premium:  premium,
	}

	// Validate all parameters
	if err := validateUserParams(params); err != nil {
		return err
	}

	// Insert user - using parameterized query
	query := "INSERT INTO users (name, email, role, age, active, verified, premium) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := s.db.Exec(query, name, email, role, age, active, verified, premium)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}
`

	err = runner.WriteFile("service.go", refactoredCode)
	framework.AssertNoError(t, err, "Failed to write refactored service.go")

	// Verify complexity is reduced
	node, _ := framework.ParseGoFile(t, runner.Context().WorkDir+"/service.go")

	// Check CreateUser complexity
	createUserFunc := framework.FindFunction(node, "CreateUser")
	if createUserFunc != nil {
		complexity := framework.CalculateComplexity(createUserFunc)
		t.Logf("Refactored CreateUser complexity: %d", complexity)

		if complexity > 15 {
			t.Errorf("CreateUser complexity %d still > 15", complexity)
		} else {
			t.Logf("✓ CreateUser complexity reduced to %d (≤ 15)", complexity)
		}
	}

	// Check validateUserParams complexity
	validateFunc := framework.FindFunction(node, "validateUserParams")
	if validateFunc != nil {
		complexity := framework.CalculateComplexity(validateFunc)
		t.Logf("validateUserParams complexity: %d", complexity)

		if complexity > 15 {
			t.Logf("⚠ validateUserParams complexity %d is high but acceptable", complexity)
		}
	}

	t.Log("✓ Refactored version has reduced complexity")
}

// TestScenario3_ComplexityCheck verifies complexity detection
func TestScenario3_ComplexityCheck(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run complexity check tool
	result, err := runner.ExecuteTool("complexity_check", map[string]interface{}{
		"path":               runner.Context().WorkDir,
		"max_complexity":     15,
		"max_function_lines": 100,
	})

	t.Logf("Complexity check result: %s", result)

	// Should identify CreateUser as too complex
	framework.AssertContains(t, result, "CreateUser", "Should identify CreateUser")
	framework.AssertContains(t, result, "complexity", "Should mention complexity")
}
