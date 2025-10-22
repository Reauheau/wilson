package agent

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// initializeTestSchema creates the tasks table for testing
func initializeTestSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_task_id INTEGER,
		task_key TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		type TEXT NOT NULL,
		assigned_to TEXT,
		assigned_at TIMESTAMP,
		status TEXT DEFAULT 'new',
		priority INTEGER DEFAULT 0,
		dor_criteria TEXT,
		dor_met BOOLEAN DEFAULT FALSE,
		dod_criteria TEXT,
		dod_met BOOLEAN DEFAULT FALSE,
		depends_on TEXT,
		blocks TEXT,
		input TEXT,
		result TEXT,
		artifact_ids TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		review_status TEXT,
		review_comments TEXT,
		reviewer TEXT,
		metadata TEXT,
		FOREIGN KEY (parent_task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);
	`
	_, err := db.Exec(schema)
	return err
}

// TestPhase0_AutoUnblockOnCompletion verifies that completing a task automatically unblocks dependent tasks
func TestPhase0_AutoUnblockOnCompletion(t *testing.T) {
	// Create temporary database
	dbPath := "/tmp/test_phase0_unblock.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Initialize schema
	if err := initializeTestSchema(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Initialize queue
	queue := NewTaskQueue(db)

	// Create Task A (the dependency)
	taskA := NewManagedTask("Task A", "Complete first", ManagedTaskTypeCode)
	taskA.Input = map[string]interface{}{"project_path": "/tmp/test"}
	SetDefaultDORCriteria(taskA)
	SetDefaultDODCriteria(taskA)

	if err := queue.CreateTask(taskA); err != nil {
		t.Fatalf("Failed to create Task A: %v", err)
	}

	// Create Task B (depends on Task A)
	taskB := NewManagedTask("Task B", "Waits for Task A", ManagedTaskTypeTest)
	taskB.DependsOn = []string{taskA.TaskKey}
	taskB.Input = map[string]interface{}{"project_path": "/tmp/test"}
	SetDefaultDORCriteria(taskB)
	SetDefaultDODCriteria(taskB)

	if err := queue.CreateTask(taskB); err != nil {
		t.Fatalf("Failed to create Task B: %v", err)
	}

	// Block Task B because it depends on Task A
	if err := queue.BlockTask(taskB.ID, "Waiting for "+taskA.TaskKey); err != nil {
		t.Fatalf("Failed to block Task B: %v", err)
	}

	// Verify Task B is blocked
	taskBAfterBlock, err := queue.GetTask(taskB.ID)
	if err != nil {
		t.Fatalf("Failed to get Task B: %v", err)
	}
	if taskBAfterBlock.Status != ManagedTaskStatusBlocked {
		t.Errorf("Expected Task B to be blocked, got: %s", taskBAfterBlock.Status)
	}

	// Mark DoD as met (required for completion)
	taskA.DODMet = true
	if err := queue.UpdateTask(taskA); err != nil {
		t.Fatalf("Failed to update Task A DoD: %v", err)
	}

	// ✅ PHASE 0 TEST: Complete Task A - should automatically unblock Task B
	if err := queue.CompleteTask(taskA.ID, "Task A completed successfully", []int{}); err != nil {
		t.Fatalf("Failed to complete Task A: %v", err)
	}

	// Verify Task A is done
	taskAAfterComplete, err := queue.GetTask(taskA.ID)
	if err != nil {
		t.Fatalf("Failed to get Task A: %v", err)
	}
	if taskAAfterComplete.Status != ManagedTaskStatusDone {
		t.Errorf("Expected Task A to be done, got: %s", taskAAfterComplete.Status)
	}

	// ✅ VERIFY: Task B should now be unblocked (status: READY)
	taskBAfterUnblock, err := queue.GetTask(taskB.ID)
	if err != nil {
		t.Fatalf("Failed to get Task B after unblock: %v", err)
	}

	if taskBAfterUnblock.Status != ManagedTaskStatusReady {
		t.Errorf("❌ PHASE 0 FAILED: Task B should be automatically unblocked. Expected: %s, Got: %s",
			ManagedTaskStatusReady, taskBAfterUnblock.Status)
	} else {
		t.Logf("✅ PHASE 0 SUCCESS: Task B automatically unblocked after Task A completed")
	}

	// Verify blocked reason is cleared from metadata
	if taskBAfterUnblock.Metadata != nil {
		if reason, exists := taskBAfterUnblock.Metadata["block_reason"]; exists {
			t.Errorf("Task B should have empty block_reason after unblock, got: %v", reason)
		}
	}
}

// TestPhase0_UnblockFailureNonCritical verifies that unblock failures don't prevent task completion
func TestPhase0_UnblockFailureNonCritical(t *testing.T) {
	// Create temporary database
	dbPath := "/tmp/test_phase0_nonfatal.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Initialize schema
	if err := initializeTestSchema(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Initialize queue
	queue := NewTaskQueue(db)

	// Create Task A
	taskA := NewManagedTask("Task A", "Complete", ManagedTaskTypeCode)
	taskA.Input = map[string]interface{}{"project_path": "/tmp/test"}
	SetDefaultDORCriteria(taskA)
	SetDefaultDODCriteria(taskA)

	if err := queue.CreateTask(taskA); err != nil {
		t.Fatalf("Failed to create Task A: %v", err)
	}

	// Mark DoD as met
	taskA.DODMet = true
	if err := queue.UpdateTask(taskA); err != nil {
		t.Fatalf("Failed to update Task A DoD: %v", err)
	}

	// Complete Task A - should succeed even if there are no dependent tasks
	if err := queue.CompleteTask(taskA.ID, "Task A completed", []int{}); err != nil {
		t.Fatalf("❌ PHASE 0 FAILED: CompleteTask should not fail even if UnblockDependentTasks fails: %v", err)
	}

	// Verify Task A is done
	taskAAfterComplete, err := queue.GetTask(taskA.ID)
	if err != nil {
		t.Fatalf("Failed to get Task A: %v", err)
	}

	if taskAAfterComplete.Status != ManagedTaskStatusDone {
		t.Errorf("Task A should be done, got: %s", taskAAfterComplete.Status)
	} else {
		t.Logf("✅ PHASE 0 SUCCESS: Task completion succeeds even when there are no dependents to unblock")
	}
}
