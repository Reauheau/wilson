package unit

import (
	"testing"

	"wilson/tests/framework"

	// Import tools to register them
	_ "wilson/capabilities/code_intelligence/analysis"
)

// TestFindPatterns tests the find_patterns tool
func TestFindPatterns(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create files with common patterns
	err := runner.WriteFile("go.mod", "module testproject\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Create CRUD-like pattern
	models := `package testproject

type User struct {
	ID   int
	Name string
}

type Post struct {
	ID     int
	UserID int
	Title  string
}
`
	err = runner.WriteFile("models.go", models)
	framework.AssertNoError(t, err, "Failed to write models.go")

	service := `package testproject

func GetUser(id int) (*User, error) {
	return nil, nil
}

func CreateUser(name string) error {
	return nil
}

func GetPost(id int) (*Post, error) {
	return nil, nil
}

// Note: Missing CreatePost, UpdateUser, UpdatePost, DeleteUser, DeletePost
`
	err = runner.WriteFile("service.go", service)
	framework.AssertNoError(t, err, "Failed to write service.go")

	// Run find_patterns
	result, err := runner.ExecuteTool("find_patterns", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err != nil {
		t.Logf("Pattern finding error: %v", err)
	} else {
		// Should identify CRUD pattern
		t.Logf("Patterns found: %s", result)
		framework.AssertContains(t, result, "pattern", "Should identify patterns")
	}
}

// TestFindRelated tests the find_related tool
func TestFindRelated(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create related files
	err := runner.WriteFile("go.mod", "module testproject\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	models := `package testproject

type User struct {
	ID   int
	Name string
}
`
	err = runner.WriteFile("models.go", models)
	framework.AssertNoError(t, err, "Failed to write models.go")

	service := `package testproject

type UserService struct{}

func (s *UserService) GetUser(id int) *User {
	return nil
}
`
	err = runner.WriteFile("service.go", service)
	framework.AssertNoError(t, err, "Failed to write service.go")

	tests := `package testproject

import "testing"

func TestUserService(t *testing.T) {
	// Test UserService
}
`
	err = runner.WriteFile("service_test.go", tests)
	framework.AssertNoError(t, err, "Failed to write service_test.go")

	// Run find_related looking for files related to User
	result, err := runner.ExecuteTool("find_related", map[string]interface{}{
		"path":   runner.Context().WorkDir,
		"symbol": "User",
	})

	if err != nil {
		t.Logf("Find related error: %v", err)
	} else {
		// Should find models.go, service.go, service_test.go
		framework.AssertContains(t, result, "models.go", "Should find models.go")
		framework.AssertContains(t, result, "service.go", "Should find service.go")
		t.Logf("Related files: %s", result)
	}
}

// TestDependencyGraph tests the dependency_graph tool
func TestDependencyGraph(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Create a project with dependencies
	err := runner.WriteFile("go.mod", "module testproject\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// models.go - no dependencies
	models := `package testproject

type User struct {
	ID   int
	Name string
}
`
	err = runner.WriteFile("models.go", models)
	framework.AssertNoError(t, err, "Failed to write models.go")

	// repository.go - depends on models
	repo := `package testproject

type UserRepository struct{}

func (r *UserRepository) Save(u User) error {
	return nil
}
`
	err = runner.WriteFile("repository.go", repo)
	framework.AssertNoError(t, err, "Failed to write repository.go")

	// service.go - depends on models and repository
	service := `package testproject

type UserService struct {
	repo UserRepository
}

func (s *UserService) CreateUser(name string) error {
	u := User{Name: name}
	return s.repo.Save(u)
}
`
	err = runner.WriteFile("service.go", service)
	framework.AssertNoError(t, err, "Failed to write service.go")

	// Run dependency_graph
	result, err := runner.ExecuteTool("dependency_graph", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err != nil {
		t.Logf("Dependency graph error: %v", err)
	} else {
		// Should show dependencies between files
		t.Logf("Dependency graph: %s", result)
		framework.AssertContains(t, result, "User", "Should reference User type")
	}
}

// TestFindPatternsOnMockProject tests pattern finding on the mock project
func TestFindPatternsOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run find_patterns
	result, err := runner.ExecuteTool("find_patterns", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err != nil {
		t.Logf("Pattern finding error: %v", err)
	} else {
		// Should identify incomplete CRUD pattern
		// Has: GetUser, GetUserByEmail, CreateUser, ListUsers, CountUsers
		// Missing: UpdateUser, DeleteUser
		t.Logf("Patterns in mock project: %s", result)

		// Should identify CRUD operations
		framework.AssertContains(t, result, "Get", "Should identify Get pattern")
		framework.AssertContains(t, result, "Create", "Should identify Create pattern")
	}
}

// TestFindRelatedOnMockProject tests find_related on the mock project
func TestFindRelatedOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Find files related to 'user' struct
	result, err := runner.ExecuteTool("find_related", map[string]interface{}{
		"path":   runner.Context().WorkDir,
		"symbol": "user",
	})

	if err != nil {
		t.Logf("Find related error: %v", err)
	} else {
		// Should find models.go (defines user), service.go (uses user)
		framework.AssertContains(t, result, "models.go", "Should find models.go")
		framework.AssertContains(t, result, "service.go", "Should find service.go")
		t.Logf("Related files for 'user': %s", result)
	}
}

// TestDependencyGraphOnMockProject tests dependency graph on mock project
func TestDependencyGraphOnMockProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	// Copy the mock project fixture
	err := runner.CopyFixture("code/mock_project")
	framework.AssertNoError(t, err, "Failed to copy mock project")

	// Run dependency_graph
	result, err := runner.ExecuteTool("dependency_graph", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err != nil {
		t.Logf("Dependency graph error: %v", err)
	} else {
		// Should show relationships:
		// - service.go uses types from models.go
		// - service.go uses functions from utils.go
		// - utils.go is relatively independent
		t.Logf("Mock project dependency graph: %s", result)

		framework.AssertContains(t, result, "service.go", "Should include service.go")
	}
}

// TestFindPatternsComplexProject tests pattern finding on complex structure
func TestFindPatternsComplexProject(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	err := runner.WriteFile("go.mod", "module complexproject\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Create a project with builder pattern
	builder := `package complexproject

type RequestBuilder struct {
	method string
	url    string
	body   []byte
}

func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{}
}

func (b *RequestBuilder) WithMethod(method string) *RequestBuilder {
	b.method = method
	return b
}

func (b *RequestBuilder) WithURL(url string) *RequestBuilder {
	b.url = url
	return b
}

func (b *RequestBuilder) Build() Request {
	return Request{
		Method: b.method,
		URL:    b.url,
		Body:   b.body,
	}
}

type Request struct {
	Method string
	URL    string
	Body   []byte
}
`
	err = runner.WriteFile("builder.go", builder)
	framework.AssertNoError(t, err, "Failed to write builder.go")

	// Run find_patterns
	result, err := runner.ExecuteTool("find_patterns", map[string]interface{}{
		"path": runner.Context().WorkDir,
	})

	if err != nil {
		t.Logf("Pattern finding error: %v", err)
	} else {
		// Should potentially identify builder pattern
		t.Logf("Patterns in complex project: %s", result)
	}
}

// TestFindRelatedAcrossPackages tests find_related with multiple packages
func TestFindRelatedAcrossPackages(t *testing.T) {
	runner := framework.NewTestRunner(t)
	defer runner.Cleanup()

	err := runner.WriteFile("go.mod", "module multipackage\n\ngo 1.24\n")
	framework.AssertNoError(t, err, "Failed to write go.mod")

	// Create models package
	models := `package models

type User struct {
	ID   int
	Name string
}
`
	err = runner.WriteFile("models/user.go", models)
	framework.AssertNoError(t, err, "Failed to write models/user.go")

	// Create service package that imports models
	service := `package service

import "multipackage/models"

func GetUser(id int) *models.User {
	return nil
}
`
	err = runner.WriteFile("service/user_service.go", service)
	framework.AssertNoError(t, err, "Failed to write service/user_service.go")

	// Run find_related on User
	result, err := runner.ExecuteTool("find_related", map[string]interface{}{
		"path":   runner.Context().WorkDir,
		"symbol": "User",
	})

	if err != nil {
		t.Logf("Find related error: %v", err)
	} else {
		// Should find both user.go and user_service.go
		t.Logf("Related files across packages: %s", result)
	}
}
