// Package userservice provides user management functionality
package userservice

// ISSUES IN THIS FILE:
// 1. Type 'user' should be exported (User) - Phase 4: lint
// 2. Field 'name' should be exported (Name) for consistency - Phase 4: lint
// 3. Missing constructor function (NewUser) - Phase 3: pattern discovery should suggest
// 4. No validation methods - Phase 3: pattern discovery
// 5. Missing godoc for fields - Phase 4: lint

// user represents a user in the system
type user struct {
	ID    int
	name  string // Inconsistent: should be Name (exported)
	Email string
	Age   int
	Role  string
}

// UserRole represents user roles
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
	RoleGuest UserRole = "guest"
)

// IsValid checks if a role is valid
func (r UserRole) IsValid() bool {
	switch r {
	case RoleAdmin, RoleUser, RoleGuest:
		return true
	}
	return false
}

// UpdateRequest represents a user update request
// ISSUE: No validation method
type UpdateRequest struct {
	Name  string
	Email string
	Age   int
}
