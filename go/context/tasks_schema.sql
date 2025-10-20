-- Task Management Schema for ENDGAME Phase 1
-- Supports task tracking, dependencies, DoR/DoD, and agent coordination

-- Tasks table: Core task tracking
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_task_id INTEGER,
    task_key TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    type TEXT NOT NULL,

    -- Assignment
    assigned_to TEXT,
    assigned_at TIMESTAMP,

    -- Status
    status TEXT DEFAULT 'new',
    priority INTEGER DEFAULT 0,

    -- Definition of Ready/Done (stored as JSON)
    dor_criteria TEXT,
    dor_met BOOLEAN DEFAULT FALSE,
    dod_criteria TEXT,
    dod_met BOOLEAN DEFAULT FALSE,

    -- Dependencies (stored as JSON arrays of task_keys)
    depends_on TEXT,
    blocks TEXT,

    -- Results
    result TEXT,
    artifact_ids TEXT,  -- JSON array

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,

    -- Review
    review_status TEXT,
    review_comments TEXT,
    reviewer TEXT,

    -- Metadata (JSON)
    metadata TEXT,

    FOREIGN KEY (parent_task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Task Reviews table: Review workflow tracking
CREATE TABLE IF NOT EXISTS task_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    reviewer_agent TEXT NOT NULL,
    review_type TEXT,
    status TEXT,
    findings TEXT,  -- JSON array
    comments TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Agent Communications table: Inter-agent messaging
CREATE TABLE IF NOT EXISTS agent_communications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_agent TEXT NOT NULL,
    to_agent TEXT,  -- NULL = broadcast
    message_type TEXT,
    content TEXT,
    context_ref TEXT,  -- Task key or artifact ID
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_key ON tasks(task_key);

CREATE INDEX IF NOT EXISTS idx_task_reviews_task ON task_reviews(task_id);
CREATE INDEX IF NOT EXISTS idx_task_reviews_status ON task_reviews(status);

CREATE INDEX IF NOT EXISTS idx_agent_comms_to ON agent_communications(to_agent);
CREATE INDEX IF NOT EXISTS idx_agent_comms_from ON agent_communications(from_agent);
CREATE INDEX IF NOT EXISTS idx_agent_comms_type ON agent_communications(message_type);
