# Mock User Service Project

**Purpose:** Test fixture for Wilson Code Agent integration tests

This is a deliberately flawed Go project used to test all 4 phases of the Code Agent's capabilities.

## Project Structure

```
mock_project/
├── models.go        - User data models (45 lines)
├── service.go       - User service with business logic (145 lines)
├── utils.go         - Utility functions (45 lines)
└── service_test.go  - Incomplete tests (30 lines)
```

## Deliberate Issues by Phase

### Phase 1: Code Intelligence (AST, Symbols, Structure)
These issues test if Code Agent can understand code structure:
- ✅ Can parse all files
- ✅ Can find user struct definition
- ✅ Can find function definitions (GetUser, CreateUser, etc.)
- ✅ Can analyze package structure
- ✅ Can analyze imports

### Phase 2: Compilation & Testing
These issues test if Code Agent can compile and test:
- ❌ **Does NOT compile** initially due to:
  - `row := s.db.Query(query)` - row is `*sql.Rows`, but used like `*sql.Row`
  - Multiple unchecked errors that Go compiler allows but are problematic

- ❌ **Low test coverage** (~5%)
  - Only 1 trivial test
  - No actual functionality tested
  - Missing edge case tests

### Phase 3: Cross-File Awareness
These issues test pattern discovery and cross-file analysis:

**models.go:**
- `type user struct` - Should be `User` (exported)
- Field `name string` - Should be `Name` (exported, consistent)
- Missing `NewUser()` constructor - Common pattern
- Missing validation methods - Common pattern

**service.go:**
- Missing `UpdateUser()` function - CRUD pattern incomplete
- Missing `DeleteUser()` function - CRUD pattern incomplete
- Inconsistent error handling - No standard pattern

**utils.go:**
- Missing `HashPassword()` - Common in user services
- Missing `ComparePassword()` - Common pattern
- Validation functions duplicate UserRole logic

**Cross-file issues:**
- `user` type unexported but used in service
- Validation logic scattered (utils.go and service.go)
- No consistent error handling pattern

### Phase 4: Quality Gates
These issues test automated quality checks:

**Format Issues (format_code):**
- `utils.go:11` - `import"strings"` - No space
- `utils.go:15` - `func ValidateEmail(email string)bool{` - No spaces
- `utils.go:16` - `return strings.Contains(email,"@")` - No spaces

**Lint Issues (lint_code):**
- Unexported `user` type
- Inconsistent field naming (`name` vs `Name`, `Email`)
- Missing godoc on many functions
- Unchecked errors throughout

**Security Issues (security_scan):**
- **CRITICAL:** SQL injection in `GetUser()` (line 38)
- **CRITICAL:** SQL injection in `GetUserByEmail()` (line 46)
- **CRITICAL:** SQL injection in `CreateUser()` (multiple lines)
- **HIGH:** Unchecked errors on security-critical operations
- **MEDIUM:** Naive email validation
- **LOW:** Weak sanitization in `SanitizeString()`

**Complexity Issues (complexity_check):**
- `CreateUser()` - Cyclomatic complexity ~25 (max 15)
- `CreateUser()` - Function length ~90 lines (approaching 100 limit)
- Deep nesting (8 levels) in `CreateUser()`

**Coverage Issues (coverage_check):**
- Total coverage: ~5%
- GetUser: 0% coverage
- GetUserByEmail: 0% coverage
- CreateUser: 0% coverage
- ListUsers: 0% coverage
- CountUsers: 0% coverage

## Expected Code Agent Fixes

### Scenario 1: Add Error Handling
**Task:** "Add proper error handling to GetUser"
**Expected:**
- Changes return type to `(*user, error)`
- Checks error from Query()
- Checks error from Scan()
- Returns errors with context

### Scenario 2: Fix SQL Injection
**Task:** "Fix security vulnerabilities in user service"
**Expected:**
- Changes to parameterized queries (`?` placeholders)
- Removes string concatenation in SQL
- All queries use proper escaping

### Scenario 3: Refactor Complex Function
**Task:** "Refactor CreateUser to reduce complexity"
**Expected:**
- Extracts validation into helper function
- Reduces nesting (early returns)
- Complexity drops below 15
- Function length drops below 100 lines

### Scenario 4: Follow Project Patterns
**Task:** "Add UpdateUser function following project conventions"
**Expected:**
- Discovers CRUD pattern from GetUser/CreateUser
- Matches function signature style
- Includes proper error handling
- Uses parameterized queries
- Adds tests

### Scenario 5: Complete Quality Pass
**Task:** "Ensure user service meets all quality standards"
**Expected:**
- Formats all code (gofmt, goimports)
- Fixes all lint issues
- Fixes all security issues
- Reduces complexity
- Adds tests to reach 80% coverage
- All quality gates pass

## Issue Summary

| Category | Count | Severity |
|----------|-------|----------|
| Format Issues | 3 | Low |
| Lint Issues | 12 | Low-Medium |
| Security Issues | 6 | Critical-High |
| Complexity Issues | 3 | Medium |
| Missing Functions | 4 | Medium |
| Test Coverage | 95% missing | High |
| Error Handling | 10+ unchecked | High |

**Total Issues: ~40+**

## Testing Instructions

This mock project is used by:
1. **Unit tests** - Test individual quality tools on this code
2. **Integration tests** - Test Code Agent end-to-end scenarios
3. **Snapshot tests** - Compare fixes to expected outcomes

To test manually:
```bash
cd tests/fixtures/code/mock_project

# Check compilation (will have issues)
go build .

# Run security scan
# (use Wilson's security_scan tool)

# Check complexity
# (use Wilson's complexity_check tool)

# Check coverage
go test -cover
```

## Version History

- v1.0 - Initial version with 40+ deliberate issues across all 4 Code Agent phases
