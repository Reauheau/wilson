package userservice

// ISSUES IN THIS FILE:
// 1. Import formatting wrong (no space) - Phase 4: format_code
// 2. Function formatting wrong - Phase 4: format_code
// 3. ValidateEmail is too naive - Phase 4: security
// 4. Missing HashPassword function - Phase 3: pattern discovery should suggest
// 5. Missing tests for these utilities - Phase 2: test coverage
// 6. No godoc comments - Phase 4: lint

import"strings" // FORMATTING ISSUE: No space after import

// ValidateEmail checks if an email is valid
// ISSUE: Very naive validation
func ValidateEmail(email string)bool{ // FORMATTING ISSUE: No space before brace
return strings.Contains(email,"@")  // FORMATTING ISSUE: No spaces
}

// ValidateName checks if a name is valid
// ISSUE: No length check, no character validation
func ValidateName(name string) bool {
	return name != ""
}

// ValidateAge checks if an age is valid
func ValidateAge(age int) bool {
	return age >= 18 && age <= 120
}

// ValidateRole checks if a role is valid
// ISSUE: Duplicates logic from UserRole.IsValid()
func ValidateRole(role string) bool {
	return role == "admin" || role == "user" || role == "guest"
}

// SanitizeString removes dangerous characters
// ISSUE: Very basic sanitization, not secure
func SanitizeString(s string) string {
	// Remove single quotes (naive SQL injection prevention)
	return strings.ReplaceAll(s, "'", "")
}

// NOTE: HashPassword is completely missing!
// Pattern discovery should suggest:
// - HashPassword(password string) (string, error)
// - ComparePassword(hash, password string) bool
// These are common patterns in user service packages
