package orchestration

import (
	"fmt"
	"strings"
)

// DORValidator validates Definition of Ready criteria
type DORValidator struct {
	task *ManagedTask
}

// DODValidator validates Definition of Done criteria
type DODValidator struct {
	task *ManagedTask
}

// NewDORValidator creates a new DoR validator for a task
func NewDORValidator(task *ManagedTask) *DORValidator {
	return &DORValidator{task: task}
}

// NewDODValidator creates a new DoD validator for a task
func NewDODValidator(task *ManagedTask) *DODValidator {
	return &DODValidator{task: task}
}

// ValidateCriteria checks if all criteria are met
// Returns true if all criteria pass, false otherwise
// Returns validation errors if any criteria fail
func (v *DORValidator) ValidateCriteria() (bool, []string) {
	if v.task == nil {
		return false, []string{"task is nil"}
	}

	var errors []string

	// Check if DoR criteria exist
	if len(v.task.DORCriteria) == 0 {
		errors = append(errors, "no Definition of Ready criteria defined")
		return false, errors
	}

	// Validate task has title and description
	if strings.TrimSpace(v.task.Title) == "" {
		errors = append(errors, "task must have a title")
	}

	if strings.TrimSpace(v.task.Description) == "" {
		errors = append(errors, "task must have a description")
	}

	// Check dependencies are resolved
	if len(v.task.DependsOn) > 0 {
		errors = append(errors, fmt.Sprintf("task has %d unresolved dependencies", len(v.task.DependsOn)))
	}

	// Check if task type is valid
	if !isValidTaskType(v.task.Type) {
		errors = append(errors, fmt.Sprintf("invalid task type: %s", v.task.Type))
	}

	return len(errors) == 0, errors
}

// MarkReady marks the DoR as met if validation passes
func (v *DORValidator) MarkReady() error {
	valid, errors := v.ValidateCriteria()
	if !valid {
		return fmt.Errorf("DoR validation failed: %s", strings.Join(errors, "; "))
	}

	v.task.DORMet = true
	if v.task.Status == ManagedTaskStatusNew {
		v.task.Status = ManagedTaskStatusReady
	}
	return nil
}

// ValidateCriteria checks if all DoD criteria are met
func (v *DODValidator) ValidateCriteria() (bool, []string) {
	if v.task == nil {
		return false, []string{"task is nil"}
	}

	var errors []string

	// Check if DoD criteria exist
	if len(v.task.DODCriteria) == 0 {
		errors = append(errors, "no Definition of Done criteria defined")
		return false, errors
	}

	// Task must be in progress or in review to be completed
	if v.task.Status != ManagedTaskStatusInProgress && v.task.Status != ManagedTaskStatusInReview {
		errors = append(errors, fmt.Sprintf("task must be in_progress or in_review, currently: %s", v.task.Status))
	}

	// Task must have been started
	if v.task.StartedAt == nil {
		errors = append(errors, "task has not been started")
	}

	// Task must have a result
	if strings.TrimSpace(v.task.Result) == "" {
		errors = append(errors, "task must have a result")
	}

	return len(errors) == 0, errors
}

// MarkDone marks the DoD as met if validation passes
func (v *DODValidator) MarkDone() error {
	valid, errors := v.ValidateCriteria()
	if !valid {
		return fmt.Errorf("DoD validation failed: %s", strings.Join(errors, "; "))
	}

	v.task.DODMet = true
	return nil
}

// CriteriaChecker provides helper methods to check common criteria
type CriteriaChecker struct{}

// NewCriteriaChecker creates a new criteria checker
func NewCriteriaChecker() *CriteriaChecker {
	return &CriteriaChecker{}
}

// CheckHasTitle checks if task has a non-empty title
func (c *CriteriaChecker) CheckHasTitle(task *ManagedTask) bool {
	return strings.TrimSpace(task.Title) != ""
}

// CheckHasDescription checks if task has a non-empty description
func (c *CriteriaChecker) CheckHasDescription(task *ManagedTask) bool {
	return strings.TrimSpace(task.Description) != ""
}

// CheckHasType checks if task has a valid type
func (c *CriteriaChecker) CheckHasType(task *ManagedTask) bool {
	return isValidTaskType(task.Type)
}

// CheckNoDependencies checks if task has no unresolved dependencies
func (c *CriteriaChecker) CheckNoDependencies(task *ManagedTask) bool {
	return len(task.DependsOn) == 0
}

// CheckHasResult checks if task has a result
func (c *CriteriaChecker) CheckHasResult(task *ManagedTask) bool {
	return strings.TrimSpace(task.Result) != ""
}

// CheckHasArtifacts checks if task has at least one artifact
func (c *CriteriaChecker) CheckHasArtifacts(task *ManagedTask) bool {
	return len(task.ArtifactIDs) > 0
}

// CheckIsStarted checks if task has been started
func (c *CriteriaChecker) CheckIsStarted(task *ManagedTask) bool {
	return task.StartedAt != nil
}

// CheckIsAssigned checks if task has been assigned
func (c *CriteriaChecker) CheckIsAssigned(task *ManagedTask) bool {
	return strings.TrimSpace(task.AssignedTo) != "" && task.AssignedAt != nil
}

// CheckIsReviewed checks if task has been reviewed and approved
func (c *CriteriaChecker) CheckIsReviewed(task *ManagedTask) bool {
	return task.ReviewStatus == ReviewStatusApproved
}

// isValidTaskType checks if a task type is one of the defined constants
func isValidTaskType(taskType ManagedTaskType) bool {
	validTypes := map[ManagedTaskType]bool{
		ManagedTaskTypeResearch: true,
		ManagedTaskTypeCode:     true,
		ManagedTaskTypeTest:     true,
		ManagedTaskTypeReview:   true,
		ManagedTaskTypeAnalysis: true,
		ManagedTaskTypeGeneral:  true,
	}
	return validTypes[taskType]
}

// ValidateTaskTransition checks if a status transition is valid
func ValidateTaskTransition(from, to ManagedTaskStatus) error {
	// Define valid transitions
	validTransitions := map[ManagedTaskStatus][]ManagedTaskStatus{
		ManagedTaskStatusNew: {
			ManagedTaskStatusReady,
			ManagedTaskStatusBlocked,
		},
		ManagedTaskStatusReady: {
			ManagedTaskStatusAssigned,
			ManagedTaskStatusBlocked,
		},
		ManagedTaskStatusAssigned: {
			ManagedTaskStatusInProgress,
			ManagedTaskStatusReady,
			ManagedTaskStatusBlocked,
		},
		ManagedTaskStatusInProgress: {
			ManagedTaskStatusInReview,
			ManagedTaskStatusBlocked,
			ManagedTaskStatusFailed,
			ManagedTaskStatusDone, // Direct completion without review
		},
		ManagedTaskStatusInReview: {
			ManagedTaskStatusDone,
			ManagedTaskStatusInProgress, // Needs changes
			ManagedTaskStatusFailed,
		},
		ManagedTaskStatusBlocked: {
			ManagedTaskStatusReady,
			ManagedTaskStatusAssigned,
			ManagedTaskStatusInProgress,
			ManagedTaskStatusFailed,
		},
		ManagedTaskStatusDone: {
			// Terminal state - no transitions
		},
		ManagedTaskStatusFailed: {
			ManagedTaskStatusReady, // Retry
		},
	}

	allowedTransitions, exists := validTransitions[from]
	if !exists {
		return fmt.Errorf("invalid source status: %s", from)
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return nil
		}
	}

	return fmt.Errorf("invalid transition from %s to %s", from, to)
}

// EvaluateDoR evaluates DoR and returns detailed status
func EvaluateDoR(task *ManagedTask) (bool, map[string]bool, []string) {
	validator := NewDORValidator(task)
	valid, errors := validator.ValidateCriteria()

	checker := NewCriteriaChecker()
	checks := map[string]bool{
		"has_title":       checker.CheckHasTitle(task),
		"has_description": checker.CheckHasDescription(task),
		"has_valid_type":  checker.CheckHasType(task),
		"no_dependencies": checker.CheckNoDependencies(task),
	}

	return valid, checks, errors
}

// EvaluateDoD evaluates DoD and returns detailed status
func EvaluateDoD(task *ManagedTask) (bool, map[string]bool, []string) {
	validator := NewDODValidator(task)
	valid, errors := validator.ValidateCriteria()

	checker := NewCriteriaChecker()
	checks := map[string]bool{
		"is_started":    checker.CheckIsStarted(task),
		"has_result":    checker.CheckHasResult(task),
		"has_artifacts": checker.CheckHasArtifacts(task),
	}

	return valid, checks, errors
}

// SetDefaultDORCriteria sets sensible default DoR criteria for a task type
func SetDefaultDORCriteria(task *ManagedTask) {
	baseDoR := []string{
		"Task has clear title and description",
		"Task type is defined",
		"All dependencies are resolved",
	}

	switch task.Type {
	case ManagedTaskTypeCode:
		task.DORCriteria = append(baseDoR,
			"Requirements are clearly specified",
			"Design approach is outlined",
			"Test criteria are defined",
		)
	case ManagedTaskTypeTest:
		task.DORCriteria = append(baseDoR,
			"Code to test is available",
			"Test scenarios are identified",
			"Expected outcomes are defined",
		)
	case ManagedTaskTypeReview:
		task.DORCriteria = append(baseDoR,
			"Artifact to review is available",
			"Review criteria are specified",
			"Reviewer is assigned",
		)
	case ManagedTaskTypeResearch:
		task.DORCriteria = append(baseDoR,
			"Research question is clear",
			"Information sources are identified",
			"Output format is specified",
		)
	case ManagedTaskTypeAnalysis:
		task.DORCriteria = append(baseDoR,
			"Data/artifacts to analyze are available",
			"Analysis objectives are clear",
			"Output format is specified",
		)
	default:
		task.DORCriteria = baseDoR
	}
}

// SetDefaultDODCriteria sets sensible default DoD criteria for a task type
func SetDefaultDODCriteria(task *ManagedTask) {
	baseDoD := []string{
		"Task result is documented",
		"All acceptance criteria are met",
		"Task is reviewed if required",
	}

	switch task.Type {
	case ManagedTaskTypeCode:
		task.DODCriteria = append(baseDoD,
			"Code is written and compilable",
			"Unit tests pass",
			"Code is reviewed",
			"Artifacts are stored",
		)
	case ManagedTaskTypeTest:
		task.DODCriteria = append(baseDoD,
			"All test cases executed",
			"Test results documented",
			"Defects logged if found",
		)
	case ManagedTaskTypeReview:
		task.DODCriteria = append(baseDoD,
			"Review completed",
			"Findings documented",
			"Approval/rejection decision made",
		)
	case ManagedTaskTypeResearch:
		task.DODCriteria = append(baseDoD,
			"Research findings documented",
			"Sources are cited",
			"Findings stored as artifacts",
		)
	case ManagedTaskTypeAnalysis:
		task.DODCriteria = append(baseDoD,
			"Analysis completed",
			"Insights documented",
			"Recommendations provided",
			"Results stored as artifacts",
		)
	default:
		task.DODCriteria = baseDoD
	}
}
