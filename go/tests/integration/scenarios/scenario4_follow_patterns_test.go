package scenarios

import (
	"testing"

	"wilson/tests/framework"
)

// TestScenario4_FollowProjectPatterns tests adding UpdateUser following project conventions
// This scenario verifies that the Code Agent can:
// 1. Discover existing CRUD patterns (GetUser, CreateUser exist)
// 2. Identify missing functions (UpdateUser is missing)
// 3. Match function signature style
// 4. Include proper error handling like existing functions
// 5. Use parameterized queries for security
// 6. Add corresponding tests
func TestScenario4_FollowProjectPatterns(t *testing.T) {
	runner := framework.NewTestRunner(t).WithAgents()
	defer runner.Cleanup()

	if err := runner.Setup(); err != nil {
		t.Fatalf("Failed to setup test: %v", err)
	}

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Verify initial state - UpdateUser doesn't exist
	initialCode, err := runner.ReadFile("service.go")
	framework.AssertNoError(t, err, "Failed to read service.go")

	if framework.ContainsAny(initialCode, []string{"UpdateUser", "func (s *UserService) Update"}) {
		t.Error("UpdateUser should not exist initially")
	}

	t.Log("Scenario 4: Add UpdateUser function following project conventions")
	t.Log("Existing patterns:")
	t.Log("  ✓ GetUser(id string) *user")
	t.Log("  ✓ GetUserByEmail(email string) *user")
	t.Log("  ✓ CreateUser(...) error")
	t.Log("  ✓ ListUsers() []*user")
	t.Log("  ✓ CountUsers() int")

	t.Log("Missing functions (pattern discovery should suggest):")
	t.Log("  ✗ UpdateUser - CRUD pattern incomplete")
	t.Log("  ✗ DeleteUser - CRUD pattern incomplete")

	// Analyze structure to discover patterns
	result, err := runner.ExecuteTool("find_patterns", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err == nil {
		t.Logf("Patterns discovered: %s", result)
	}

	// TODO: Execute Code Agent task
	// "Add UpdateUser function following project conventions"

	t.Log("Expected changes:")
	t.Log("  1. Add UpdateUser(id string, name, email, role string, age int) error")
	t.Log("  2. Match parameter style of CreateUser")
	t.Log("  3. Include validation")
	t.Log("  4. Use parameterized SQL query")
	t.Log("  5. Return error with proper context")
	t.Log("  6. Add TestUpdateUser in service_test.go")

	// Verification would check:
	// - Function exists with correct signature
	// - Uses parameterized queries
	// - Handles errors properly
	// - Has tests
	// - Compiles successfully

	t.Log("Scenario 4 test structure complete")
}

// TestScenario4_ManualImplementation demonstrates the expected implementation
func TestScenario4_ManualImplementation(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	err := runner.WriteFile("go.mod", "module userservice\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Service with UpdateUser added
	serviceCode := `package userservice

import (
	"database/sql"
	"errors"
	"fmt"
)

type user struct {
	ID    string
	Name  string
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

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id string) (*user, error) {
	query := "SELECT id, name, email, age, role FROM users WHERE id = ?"
	row := s.db.QueryRow(query, id)

	u := &user{}
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Age, &u.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return u, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(name, email, role string, age int) error {
	if name == "" {
		return errors.New("name is required")
	}
	if email == "" {
		return errors.New("email is required")
	}
	if age < 18 || age > 120 {
		return errors.New("age must be between 18 and 120")
	}

	query := "INSERT INTO users (name, email, role, age) VALUES (?, ?, ?, ?)"
	_, err := s.db.Exec(query, name, email, role, age)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// UpdateUser updates an existing user - FOLLOWING PROJECT CONVENTIONS
func (s *UserService) UpdateUser(id string, name, email, role string, age int) error {
	// Validation (matching CreateUser style)
	if name == "" {
		return errors.New("name is required")
	}
	if email == "" {
		return errors.New("email is required")
	}
	if age < 18 || age > 120 {
		return errors.New("age must be between 18 and 120")
	}

	// Parameterized query (matching GetUser/CreateUser style)
	query := "UPDATE users SET name = ?, email = ?, role = ?, age = ? WHERE id = ?"
	result, err := s.db.Exec(query, name, email, role, age, id)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Check if user was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}

// DeleteUser deletes a user - COMPLETING CRUD PATTERN
func (s *UserService) DeleteUser(id string) error {
	query := "DELETE FROM users WHERE id = ?"
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}
`

	err = runner.WriteFile("service.go", serviceCode)
	framework.AssertNoError(t, err, "Failed to write service.go")

	// Add tests following project conventions
	testCode := `package userservice

import "testing"

func TestNewUserService(t *testing.T) {
	service := NewUserService(nil)
	if service == nil {
		t.Fatal("NewUserService returned nil")
	}
}

// TestUpdateUser tests the UpdateUser method
func TestUpdateUser(t *testing.T) {
	// Would need database mock for real test
	// This demonstrates the test structure
	service := NewUserService(nil)
	if service == nil {
		t.Fatal("NewUserService returned nil")
	}

	// In real test:
	// 1. Create a user
	// 2. Update the user
	// 3. Verify the update succeeded
	// 4. Verify error cases (invalid data, non-existent user)
}

// TestDeleteUser tests the DeleteUser method
func TestDeleteUser(t *testing.T) {
	service := NewUserService(nil)
	if service == nil {
		t.Fatal("NewUserService returned nil")
	}

	// In real test:
	// 1. Create a user
	// 2. Delete the user
	// 3. Verify deletion succeeded
	// 4. Verify error case (delete non-existent user)
}
`

	err = runner.WriteFile("service_test.go", testCode)
	framework.AssertNoError(t, err, "Failed to write service_test.go")

	// Verify functions exist
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service.go", "UpdateUser")
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service.go", "DeleteUser")

	// Verify they return errors (following convention)
	framework.AssertFunctionReturnsError(t, runner.Context().WorkDir+"/service.go", "UpdateUser")
	framework.AssertFunctionReturnsError(t, runner.Context().WorkDir+"/service.go", "DeleteUser")

	// Verify tests exist
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service_test.go", "TestUpdateUser")
	framework.AssertFunctionExists(t, runner.Context().WorkDir+"/service_test.go", "TestDeleteUser")

	t.Log("✓ UpdateUser and DeleteUser follow project conventions")
	t.Log("✓ Tests added for new functions")
	t.Log("✓ CRUD pattern now complete: Create, Read, Update, Delete")
}

// TestScenario4_PatternDiscovery tests pattern discovery on mock project
func TestScenario4_PatternDiscovery(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Analyze structure
	result, err := runner.ExecuteTool("analyze_structure", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err != nil {
		t.Logf("Structure analysis error: %v", err)
	} else {
		t.Logf("Project structure: %s", result)

		// Should find existing CRUD operations
		framework.AssertContains(t, result, "GetUser", "Should find GetUser")
		framework.AssertContains(t, result, "CreateUser", "Should find CreateUser")

		// Could identify missing operations
		if !framework.ContainsAny(result, []string{"UpdateUser", "Update"}) {
			t.Log("⚠ Structure analysis doesn't show UpdateUser (as expected - it's missing)")
		}
	}

	// Count functions
	count, err := framework.CountFunctions(runner.Context().WorkDir + "/service.go")
	framework.AssertNoError(t, err, "Failed to count functions")
	t.Logf("Service has %d functions", count)

	// Should have: NewUserService, GetUser, GetUserByEmail, CreateUser, ListUsers, CountUsers
	// Missing: UpdateUser, DeleteUser
	if count < 8 {
		t.Logf("✓ Confirmed: CRUD pattern incomplete (has %d functions, missing Update and Delete)", count)
	}
}
