package userservice

import (
	"database/sql"
	"errors"
	"fmt"
)

// ISSUES IN THIS FILE:
// 1. GetUser: SQL injection vulnerability - Phase 4: security_scan
// 2. GetUser: No error handling on Query/Scan - Phase 2: compile will catch, Phase 4: security
// 3. CreateUser: High cyclomatic complexity (>15) - Phase 4: complexity_check
// 4. CreateUser: Function too long (>100 lines) - Phase 4: complexity_check
// 5. UpdateUser: Missing entirely - Phase 3: pattern discovery should suggest
// 6. DeleteUser: Missing entirely - Phase 3: pattern discovery should suggest
// 7. No nil checks - Phase 4: lint
// 8. Inconsistent error handling patterns - Phase 3: pattern discovery

// UserService handles user operations
type UserService struct {
	db *sql.DB
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// GetUser retrieves a user by ID
// CRITICAL ISSUES:
// - SQL injection via string concatenation
// - No error handling on Query
// - No error handling on Scan
// - Returns pointer that could be nil without indication
func (s *UserService) GetUser(id string) *user {
	// SQL INJECTION VULNERABILITY!
	query := "SELECT id, name, email, age, role FROM users WHERE id = " + id

	row := s.db.Query(query) // ERROR NOT CHECKED!

	u := &user{}
	row.Scan(&u.ID, &u.name, &u.Email, &u.Age, &u.Role) // ERROR NOT CHECKED!

	return u
}

// GetUserByEmail retrieves a user by email
// ISSUE: Same SQL injection and error handling problems
func (s *UserService) GetUserByEmail(email string) *user {
	query := "SELECT id, name, email, age, role FROM users WHERE email = '" + email + "'"
	row := s.db.Query(query)

	u := &user{}
	row.Scan(&u.ID, &u.name, &u.Email, &u.Age, &u.Role)
	return u
}

// CreateUser creates a new user
// CRITICAL ISSUES:
// - Cyclomatic complexity: ~25 (max should be 15)
// - Function length: ~90 lines (approaching limit)
// - Deeply nested if statements (hard to read)
// - No error wrapping
// - Inconsistent validation approach
func (s *UserService) CreateUser(name, email, role string, age int, active bool, verified bool, premium bool) error {
	// ISSUE: Massive nested validation logic
	if name != "" {
		if len(name) >= 2 {
			if email != "" {
				if len(email) >= 5 {
					if role != "" {
						if role == "admin" || role == "user" || role == "guest" {
							if age >= 18 {
								if age <= 120 {
									if active {
										if verified {
											if premium {
												// Premium verified active adult users with valid name/email/role
												query := "INSERT INTO users (name, email, role, age, active, verified, premium) VALUES ('" +
													name + "', '" + email + "', '" + role + "', " +
													fmt.Sprintf("%d", age) + ", true, true, true)"
												_, err := s.db.Exec(query)
												if err != nil {
													return err
												}
												return nil
											} else {
												// Non-premium verified active adult users
												query := "INSERT INTO users (name, email, role, age, active, verified, premium) VALUES ('" +
													name + "', '" + email + "', '" + role + "', " +
													fmt.Sprintf("%d", age) + ", true, true, false)"
												_, err := s.db.Exec(query)
												if err != nil {
													return err
												}
												return nil
											}
										} else {
											// Unverified active users
											if premium {
												return errors.New("cannot create premium user without verification")
											}
											query := "INSERT INTO users (name, email, role, age, active, verified, premium) VALUES ('" +
												name + "', '" + email + "', '" + role + "', " +
												fmt.Sprintf("%d", age) + ", true, false, false)"
											_, err := s.db.Exec(query)
											if err != nil {
												return err
											}
											return nil
										}
									} else {
										// Inactive users
										return errors.New("cannot create inactive user")
									}
								} else {
									return errors.New("age must be 120 or less")
								}
							} else {
								return errors.New("user must be 18 or older")
							}
						} else {
							return errors.New("invalid role: must be admin, user, or guest")
						}
					} else {
						return errors.New("role is required")
					}
				} else {
					return errors.New("email must be at least 5 characters")
				}
			} else {
				return errors.New("email is required")
			}
		} else {
			return errors.New("name must be at least 2 characters")
		}
	} else {
		return errors.New("name is required")
	}
}

// ListUsers lists all users
// ISSUE: No pagination, no error handling
func (s *UserService) ListUsers() []*user {
	query := "SELECT id, name, email, age, role FROM users"
	rows, _ := s.db.Query(query) // ERROR NOT CHECKED!
	defer rows.Close()

	var users []*user
	for rows.Next() {
		u := &user{}
		rows.Scan(&u.ID, &u.name, &u.Email, &u.Age, &u.Role) // ERROR NOT CHECKED!
		users = append(users, u)
	}

	return users
}

// CountUsers counts total users
func (s *UserService) CountUsers() int {
	query := "SELECT COUNT(*) FROM users"
	var count int
	s.db.QueryRow(query).Scan(&count) // ERROR NOT CHECKED!
	return count
}

// NOTE: UpdateUser and DeleteUser are missing - pattern discovery should suggest them!
