package context

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages the SQLite database for contexts
type Store struct {
	db *sql.DB
}

// NewStore creates a new context store
func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &Store{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// initSchema creates the database schema
func (s *Store) initSchema() error {
	schema := `
	-- Task/project containers
	CREATE TABLE IF NOT EXISTS contexts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		context_key TEXT UNIQUE NOT NULL,
		context_type TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		title TEXT,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_by TEXT,
		metadata TEXT
	);

	-- Agent outputs and findings
	CREATE TABLE IF NOT EXISTS artifacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		context_id INTEGER NOT NULL,
		artifact_type TEXT NOT NULL,
		content TEXT NOT NULL,
		source TEXT,
		agent TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT,
		FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE
	);

	-- Inter-agent messages
	CREATE TABLE IF NOT EXISTS agent_notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		context_id INTEGER NOT NULL,
		from_agent TEXT NOT NULL,
		to_agent TEXT,
		note TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_contexts_key ON contexts(context_key);
	CREATE INDEX IF NOT EXISTS idx_contexts_status ON contexts(status);
	CREATE INDEX IF NOT EXISTS idx_contexts_type ON contexts(context_type);
	CREATE INDEX IF NOT EXISTS idx_artifacts_context ON artifacts(context_id);
	CREATE INDEX IF NOT EXISTS idx_artifacts_type ON artifacts(artifact_type);
	CREATE INDEX IF NOT EXISTS idx_artifacts_agent ON artifacts(agent);
	CREATE INDEX IF NOT EXISTS idx_notes_context ON agent_notes(context_id);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Try to create FTS5 table (optional, for full-text search)
	ftsSchema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
		content,
		artifact_type,
		content=artifacts,
		content_rowid=id
	);

	CREATE TRIGGER IF NOT EXISTS artifacts_ai AFTER INSERT ON artifacts BEGIN
		INSERT INTO artifacts_fts(rowid, content, artifact_type)
		VALUES (new.id, new.content, new.artifact_type);
	END;

	CREATE TRIGGER IF NOT EXISTS artifacts_ad AFTER DELETE ON artifacts BEGIN
		DELETE FROM artifacts_fts WHERE rowid = old.id;
	END;

	CREATE TRIGGER IF NOT EXISTS artifacts_au AFTER UPDATE ON artifacts BEGIN
		DELETE FROM artifacts_fts WHERE rowid = old.id;
		INSERT INTO artifacts_fts(rowid, content, artifact_type)
		VALUES (new.id, new.content, new.artifact_type);
	END;
	`

	// FTS5 is optional - if it fails, we'll use regular queries
	_, _ = s.db.Exec(ftsSchema)

	return nil
}

// CreateContext creates a new context
func (s *Store) CreateContext(req CreateContextRequest) (*Context, error) {
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	now := time.Now()
	result, err := s.db.Exec(`
		INSERT INTO contexts (context_key, context_type, title, description, created_by, created_at, updated_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, req.Key, req.Type, req.Title, req.Description, req.CreatedBy, now, now, string(metadataJSON))

	if err != nil {
		return nil, fmt.Errorf("failed to insert context: %w", err)
	}

	_, err = result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return s.GetContext(req.Key)
}

// GetContext retrieves a context by key
func (s *Store) GetContext(key string) (*Context, error) {
	var ctx Context
	var metadataJSON string

	err := s.db.QueryRow(`
		SELECT id, context_key, context_type, status, title, description, created_at, updated_at, created_by, metadata
		FROM contexts WHERE context_key = ?
	`, key).Scan(&ctx.ID, &ctx.Key, &ctx.Type, &ctx.Status, &ctx.Title, &ctx.Description, &ctx.CreatedAt, &ctx.UpdatedAt, &ctx.CreatedBy, &metadataJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("context not found: %s", key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query context: %w", err)
	}

	if err := json.Unmarshal([]byte(metadataJSON), &ctx.Metadata); err != nil {
		ctx.Metadata = make(map[string]interface{})
	}

	// Load artifacts
	artifacts, err := s.GetArtifacts(ctx.ID)
	if err == nil {
		ctx.Artifacts = artifacts
	}

	// Load notes
	notes, err := s.GetNotes(ctx.ID)
	if err == nil {
		ctx.Notes = notes
	}

	return &ctx, nil
}

// UpdateContextStatus updates the status of a context
func (s *Store) UpdateContextStatus(key string, status string) error {
	_, err := s.db.Exec(`
		UPDATE contexts SET status = ?, updated_at = ? WHERE context_key = ?
	`, status, time.Now(), key)
	return err
}

// ListContexts lists contexts with optional filtering
func (s *Store) ListContexts(status string, limit int) ([]Context, error) {
	query := `SELECT id, context_key, context_type, status, title, description, created_at, updated_at, created_by, metadata FROM contexts`
	args := []interface{}{}

	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY updated_at DESC`

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query contexts: %w", err)
	}
	defer rows.Close()

	var contexts []Context
	for rows.Next() {
		var ctx Context
		var metadataJSON string

		if err := rows.Scan(&ctx.ID, &ctx.Key, &ctx.Type, &ctx.Status, &ctx.Title, &ctx.Description, &ctx.CreatedAt, &ctx.UpdatedAt, &ctx.CreatedBy, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan context: %w", err)
		}

		if err := json.Unmarshal([]byte(metadataJSON), &ctx.Metadata); err != nil {
			ctx.Metadata = make(map[string]interface{})
		}

		contexts = append(contexts, ctx)
	}

	return contexts, nil
}

// StoreArtifact stores an artifact in a context
func (s *Store) StoreArtifact(req StoreArtifactRequest) (*Artifact, error) {
	// Get context ID
	var contextID int
	err := s.db.QueryRow(`SELECT id FROM contexts WHERE context_key = ?`, req.ContextKey).Scan(&contextID)
	if err != nil {
		return nil, fmt.Errorf("context not found: %s", req.ContextKey)
	}

	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	now := time.Now()
	result, err := s.db.Exec(`
		INSERT INTO artifacts (context_id, artifact_type, content, source, agent, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, contextID, req.Type, req.Content, req.Source, req.Agent, now, string(metadataJSON))

	if err != nil {
		return nil, fmt.Errorf("failed to insert artifact: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	// Update context updated_at
	_, _ = s.db.Exec(`UPDATE contexts SET updated_at = ? WHERE id = ?`, now, contextID)

	return s.GetArtifact(int(id))
}

// GetArtifact retrieves an artifact by ID
func (s *Store) GetArtifact(id int) (*Artifact, error) {
	var artifact Artifact
	var metadataJSON string

	err := s.db.QueryRow(`
		SELECT id, context_id, artifact_type, content, source, agent, created_at, metadata
		FROM artifacts WHERE id = ?
	`, id).Scan(&artifact.ID, &artifact.ContextID, &artifact.Type, &artifact.Content, &artifact.Source, &artifact.Agent, &artifact.CreatedAt, &metadataJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to query artifact: %w", err)
	}

	if err := json.Unmarshal([]byte(metadataJSON), &artifact.Metadata); err != nil {
		artifact.Metadata = ArtifactMetadata{}
	}

	return &artifact, nil
}

// GetArtifacts retrieves all artifacts for a context
func (s *Store) GetArtifacts(contextID int) ([]Artifact, error) {
	rows, err := s.db.Query(`
		SELECT id, context_id, artifact_type, content, source, agent, created_at, metadata
		FROM artifacts WHERE context_id = ? ORDER BY created_at ASC
	`, contextID)

	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []Artifact
	for rows.Next() {
		var artifact Artifact
		var metadataJSON string

		if err := rows.Scan(&artifact.ID, &artifact.ContextID, &artifact.Type, &artifact.Content, &artifact.Source, &artifact.Agent, &artifact.CreatedAt, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan artifact: %w", err)
		}

		if err := json.Unmarshal([]byte(metadataJSON), &artifact.Metadata); err != nil {
			artifact.Metadata = ArtifactMetadata{}
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// SearchArtifacts performs full-text search on artifacts
func (s *Store) SearchArtifacts(req SearchArtifactsRequest) ([]SearchResult, error) {
	// Check if FTS5 table exists
	var hasFTS bool
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='artifacts_fts'`).Scan(&hasFTS)
	if err != nil {
		hasFTS = false
	}

	var query string
	var args []interface{}

	if hasFTS {
		// Use FTS5 search
		query = `
			SELECT a.id, a.context_id, a.artifact_type, a.content, a.source, a.agent, a.created_at, a.metadata,
			       c.id, c.context_key, c.context_type, c.status, c.title, c.description, c.created_at, c.updated_at, c.created_by, c.metadata
			FROM artifacts a
			JOIN artifacts_fts fts ON a.id = fts.rowid
			JOIN contexts c ON a.context_id = c.id
			WHERE artifacts_fts MATCH ?
		`
		args = append(args, req.Query)
	} else {
		// Use LIKE search (fallback)
		query = `
			SELECT a.id, a.context_id, a.artifact_type, a.content, a.source, a.agent, a.created_at, a.metadata,
			       c.id, c.context_key, c.context_type, c.status, c.title, c.description, c.created_at, c.updated_at, c.created_by, c.metadata
			FROM artifacts a
			JOIN contexts c ON a.context_id = c.id
			WHERE a.content LIKE ?
		`
		args = append(args, "%"+req.Query+"%")
	}

	if req.ContextKey != "" {
		query += ` AND c.context_key = ?`
		args = append(args, req.ContextKey)
	}

	if len(req.Types) > 0 {
		query += ` AND a.artifact_type IN (`
		for i, t := range req.Types {
			if i > 0 {
				query += `, `
			}
			query += `?`
			args = append(args, t)
		}
		query += `)`
	}

	if req.Agent != "" {
		query += ` AND a.agent = ?`
		args = append(args, req.Agent)
	}

	query += ` ORDER BY a.created_at DESC`

	if req.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, req.Limit)
	} else {
		query += ` LIMIT 10`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search artifacts: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var artifactMetadataJSON, contextMetadataJSON string

		if err := rows.Scan(
			&result.Artifact.ID, &result.Artifact.ContextID, &result.Artifact.Type, &result.Artifact.Content,
			&result.Artifact.Source, &result.Artifact.Agent, &result.Artifact.CreatedAt, &artifactMetadataJSON,
			&result.Context.ID, &result.Context.Key, &result.Context.Type, &result.Context.Status,
			&result.Context.Title, &result.Context.Description, &result.Context.CreatedAt, &result.Context.UpdatedAt,
			&result.Context.CreatedBy, &contextMetadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		json.Unmarshal([]byte(artifactMetadataJSON), &result.Artifact.Metadata)
		json.Unmarshal([]byte(contextMetadataJSON), &result.Context.Metadata)

		results = append(results, result)
	}

	return results, nil
}

// AddNote adds an agent note to a context
func (s *Store) AddNote(contextKey, fromAgent, toAgent, note string) (*AgentNote, error) {
	var contextID int
	err := s.db.QueryRow(`SELECT id FROM contexts WHERE context_key = ?`, contextKey).Scan(&contextID)
	if err != nil {
		return nil, fmt.Errorf("context not found: %s", contextKey)
	}

	now := time.Now()
	result, err := s.db.Exec(`
		INSERT INTO agent_notes (context_id, from_agent, to_agent, note, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, contextID, fromAgent, toAgent, note, now)

	if err != nil {
		return nil, fmt.Errorf("failed to insert note: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &AgentNote{
		ID:        int(id),
		ContextID: contextID,
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		Note:      note,
		CreatedAt: now,
	}, nil
}

// GetNotes retrieves all notes for a context
func (s *Store) GetNotes(contextID int) ([]AgentNote, error) {
	rows, err := s.db.Query(`
		SELECT id, context_id, from_agent, to_agent, note, created_at
		FROM agent_notes WHERE context_id = ? ORDER BY created_at ASC
	`, contextID)

	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	var notes []AgentNote
	for rows.Next() {
		var note AgentNote
		if err := rows.Scan(&note.ID, &note.ContextID, &note.FromAgent, &note.ToAgent, &note.Note, &note.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan note: %w", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// Helper function to ensure directory exists
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}
