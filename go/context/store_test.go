package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (*Store, string) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	require.NoError(t, err, "Failed to create test store")
	require.NotNil(t, store, "Store should not be nil")

	return store, dbPath
}

func TestNewStore(t *testing.T) {
	store, dbPath := setupTestDB(t)
	defer store.Close()

	// Verify database file was created
	_, err := os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")

	// Verify store is usable
	assert.NotNil(t, store.db, "Store database connection should not be nil")
}

func TestCreateContext(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	req := CreateContextRequest{
		Key:         "test-context",
		Type:        TypeTask,
		Title:       "Test Context",
		Description: "A test context for testing",
		CreatedBy:   "test-user",
		Metadata:    map[string]interface{}{"priority": "high"},
	}

	ctx, err := store.CreateContext(req)
	require.NoError(t, err, "Failed to create context")
	require.NotNil(t, ctx, "Context should not be nil")

	// Verify context fields
	assert.Equal(t, req.Key, ctx.Key)
	assert.Equal(t, req.Type, ctx.Type)
	assert.Equal(t, req.Title, ctx.Title)
	assert.Equal(t, req.Description, ctx.Description)
	assert.Equal(t, req.CreatedBy, ctx.CreatedBy)
	assert.Equal(t, StatusActive, ctx.Status)
	assert.NotZero(t, ctx.ID)
	assert.NotZero(t, ctx.CreatedAt)
	assert.NotZero(t, ctx.UpdatedAt)
}

func TestCreateContextDuplicateKey(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	req := CreateContextRequest{
		Key:   "duplicate-key",
		Type:  TypeTask,
		Title: "First Context",
	}

	// First creation should succeed
	_, err := store.CreateContext(req)
	require.NoError(t, err)

	// Second creation with same key should fail
	_, err = store.CreateContext(req)
	assert.Error(t, err, "Creating context with duplicate key should fail")
}

func TestGetContext(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create a context
	req := CreateContextRequest{
		Key:   "get-test",
		Type:  TypeResearch,
		Title: "Get Test Context",
	}
	created, err := store.CreateContext(req)
	require.NoError(t, err)

	// Retrieve the context
	retrieved, err := store.GetContext("get-test")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify fields match
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Key, retrieved.Key)
	assert.Equal(t, created.Type, retrieved.Type)
	assert.Equal(t, created.Title, retrieved.Title)
}

func TestGetContextNotFound(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	_, err := store.GetContext("nonexistent-key")
	assert.Error(t, err, "Getting nonexistent context should fail")
	assert.Contains(t, err.Error(), "not found")
}

func TestStoreArtifact(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create a context first
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "artifact-test",
		Type:  TypeTask,
		Title: "Artifact Test",
	})
	require.NoError(t, err)

	// Store an artifact
	req := StoreArtifactRequest{
		ContextKey: ctx.Key,
		Type:       ArtifactAnalysis,
		Content:    "Test analysis content",
		Source:     "test-tool",
		Agent:      "test-agent",
		Metadata: ArtifactMetadata{
			Model: "test-model",
			Tags:  []string{"test", "analysis"},
		},
	}

	artifact, err := store.StoreArtifact(req)
	require.NoError(t, err)
	require.NotNil(t, artifact)

	// Verify artifact fields
	assert.NotZero(t, artifact.ID)
	assert.Equal(t, ctx.ID, artifact.ContextID)
	assert.Equal(t, req.Type, artifact.Type)
	assert.Equal(t, req.Content, artifact.Content)
	assert.Equal(t, req.Source, artifact.Source)
	assert.Equal(t, req.Agent, artifact.Agent)
	assert.NotZero(t, artifact.CreatedAt)
}

func TestGetArtifacts(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create context
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "multi-artifact-test",
		Type:  TypeTask,
		Title: "Multi Artifact Test",
	})
	require.NoError(t, err)

	// Store multiple artifacts
	for i := 0; i < 3; i++ {
		req := StoreArtifactRequest{
			ContextKey: ctx.Key,
			Type:       ArtifactSummary,
			Content:    "Content " + string(rune('A'+i)),
			Agent:      "test-agent",
		}
		_, err := store.StoreArtifact(req)
		require.NoError(t, err)
	}

	// Retrieve artifacts
	artifacts, err := store.GetArtifacts(ctx.ID)
	require.NoError(t, err)
	assert.Len(t, artifacts, 3, "Should have 3 artifacts")

	// Verify order (should be chronological)
	for i := 0; i < 2; i++ {
		assert.True(t, artifacts[i].CreatedAt.Before(artifacts[i+1].CreatedAt) || artifacts[i].CreatedAt.Equal(artifacts[i+1].CreatedAt))
	}
}

func TestSearchArtifacts(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create context
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "search-test",
		Type:  TypeResearch,
		Title: "Search Test",
	})
	require.NoError(t, err)

	// Store artifacts with different content
	artifacts := []string{
		"The quick brown fox jumps over the lazy dog",
		"Python is a great programming language",
		"Go is excellent for concurrent programming",
	}

	for _, content := range artifacts {
		req := StoreArtifactRequest{
			ContextKey: ctx.Key,
			Type:       ArtifactAnalysis,
			Content:    content,
			Agent:      "test-agent",
		}
		_, err := store.StoreArtifact(req)
		require.NoError(t, err)
	}

	// Search for "programming"
	results, err := store.SearchArtifacts(SearchArtifactsRequest{
		Query: "programming",
		Limit: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2, "Should find at least 2 results with 'programming'")

	// Verify results contain expected content
	found := false
	for _, result := range results {
		if result.Artifact.Content == "Python is a great programming language" ||
			result.Artifact.Content == "Go is excellent for concurrent programming" {
			found = true
		}
	}
	assert.True(t, found, "Should find artifacts containing 'programming'")
}

func TestSearchArtifactsWithContextFilter(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create two contexts
	ctx1, _ := store.CreateContext(CreateContextRequest{Key: "ctx1", Type: TypeTask, Title: "Context 1"})
	ctx2, _ := store.CreateContext(CreateContextRequest{Key: "ctx2", Type: TypeTask, Title: "Context 2"})

	// Store artifacts in both
	store.StoreArtifact(StoreArtifactRequest{ContextKey: ctx1.Key, Type: ArtifactAnalysis, Content: "test content in ctx1", Agent: "agent"})
	store.StoreArtifact(StoreArtifactRequest{ContextKey: ctx2.Key, Type: ArtifactAnalysis, Content: "test content in ctx2", Agent: "agent"})

	// Search only in ctx1
	results, err := store.SearchArtifacts(SearchArtifactsRequest{
		Query:      "test",
		ContextKey: ctx1.Key,
		Limit:      10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should only find artifact in ctx1")
	assert.Equal(t, ctx1.Key, results[0].Context.Key)
}

func TestUpdateContextStatus(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create context
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "status-test",
		Type:  TypeTask,
		Title: "Status Test",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusActive, ctx.Status)

	// Update status
	err = store.UpdateContextStatus(ctx.Key, StatusCompleted)
	require.NoError(t, err)

	// Verify status changed
	updated, err := store.GetContext(ctx.Key)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updated.Status)
}

func TestListContexts(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create contexts with different statuses
	store.CreateContext(CreateContextRequest{Key: "active1", Type: TypeTask, Title: "Active 1"})
	store.CreateContext(CreateContextRequest{Key: "active2", Type: TypeTask, Title: "Active 2"})

	ctx3, _ := store.CreateContext(CreateContextRequest{Key: "completed1", Type: TypeTask, Title: "Completed 1"})
	store.UpdateContextStatus(ctx3.Key, StatusCompleted)

	// List all contexts
	all, err := store.ListContexts("", 10)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List only active
	active, err := store.ListContexts(StatusActive, 10)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	// List only completed
	completed, err := store.ListContexts(StatusCompleted, 10)
	require.NoError(t, err)
	assert.Len(t, completed, 1)
}

func TestAddNote(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create context
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "note-test",
		Type:  TypeTask,
		Title: "Note Test",
	})
	require.NoError(t, err)

	// Add note
	note, err := store.AddNote(ctx.Key, "agent-a", "agent-b", "Task completed, ready for next step")
	require.NoError(t, err)
	require.NotNil(t, note)

	assert.NotZero(t, note.ID)
	assert.Equal(t, ctx.ID, note.ContextID)
	assert.Equal(t, "agent-a", note.FromAgent)
	assert.Equal(t, "agent-b", note.ToAgent)
	assert.Equal(t, "Task completed, ready for next step", note.Note)
	assert.NotZero(t, note.CreatedAt)
}

func TestGetNotes(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create context
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "notes-test",
		Type:  TypeTask,
		Title: "Notes Test",
	})
	require.NoError(t, err)

	// Add multiple notes
	store.AddNote(ctx.Key, "agent-a", "agent-b", "First note")
	store.AddNote(ctx.Key, "agent-b", "agent-c", "Second note")
	store.AddNote(ctx.Key, "agent-c", "", "Broadcast note")

	// Get notes
	notes, err := store.GetNotes(ctx.ID)
	require.NoError(t, err)
	assert.Len(t, notes, 3)

	// Verify chronological order
	assert.Equal(t, "First note", notes[0].Note)
	assert.Equal(t, "Second note", notes[1].Note)
	assert.Equal(t, "Broadcast note", notes[2].Note)
}

func TestContextWithArtifactsAndNotes(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	// Create context
	ctx, err := store.CreateContext(CreateContextRequest{
		Key:   "full-test",
		Type:  TypeResearch,
		Title: "Full Context Test",
	})
	require.NoError(t, err)

	// Add artifacts
	store.StoreArtifact(StoreArtifactRequest{
		ContextKey: ctx.Key,
		Type:       ArtifactWebSearch,
		Content:    "Search results",
		Agent:      "search-agent",
	})

	// Add notes
	store.AddNote(ctx.Key, "agent-a", "agent-b", "Research complete")

	// Retrieve full context
	fullCtx, err := store.GetContext(ctx.Key)
	require.NoError(t, err)

	// Verify everything is loaded
	assert.Len(t, fullCtx.Artifacts, 1)
	assert.Len(t, fullCtx.Notes, 1)
	assert.Equal(t, "Search results", fullCtx.Artifacts[0].Content)
	assert.Equal(t, "Research complete", fullCtx.Notes[0].Note)
}
