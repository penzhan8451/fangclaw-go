// Package memory provides SQLite-based persistence for FangClaw.
package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/glebarez/sqlite"
)

// DB represents the SQLite database connection.
type DB struct {
	*sql.DB
	Path string
}

// NewDB creates a new database connection.
func NewDB(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db, Path: path}, nil
}

// Migrate runs database migrations.
func (db *DB) Migrate() error {
	migrations := []string{
		// Agents table (updated with manifest, state, session_id)
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			manifest TEXT NOT NULL,
			state TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			session_id TEXT NOT NULL DEFAULT ''
		)`,

		// Sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			messages TEXT NOT NULL,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,

		// Memory/KV store (for backward compatibility)
		`CREATE TABLE IF NOT EXISTS memory (
			agent_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (agent_id, key),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,

		// KV store (with versioning)
		`CREATE TABLE IF NOT EXISTS kv_store (
			agent_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (agent_id, key),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,

		// Usage tracking
		`CREATE TABLE IF NOT EXISTS usage (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			session_id TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			provider TEXT NOT NULL,
			usage TEXT NOT NULL,
			cost_usd REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,

		// Memories table
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			content TEXT NOT NULL,
			source TEXT NOT NULL,
			scope TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 1.0,
			metadata TEXT,
			created_at TEXT NOT NULL,
			accessed_at TEXT NOT NULL,
			access_count INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER NOT NULL DEFAULT 0,
			embedding BLOB
		)`,

		// Audit trail
		`CREATE TABLE IF NOT EXISTS audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL,
			action TEXT NOT NULL,
			agent_id TEXT,
			details TEXT,
			hash TEXT NOT NULL
		)`,

		// Triggers table
		`CREATE TABLE IF NOT EXISTS triggers (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			pattern TEXT NOT NULL,
			prompt_template TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			created_at TEXT NOT NULL,
			fire_count INTEGER DEFAULT 0,
			max_fires INTEGER DEFAULT 0,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,

		// Workflows table
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			steps TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,

		// Cron jobs table
		`CREATE TABLE IF NOT EXISTS cron_jobs (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			spec TEXT NOT NULL,
			prompt TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			next_run TEXT,
			last_run TEXT,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,

		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_agent_id ON memory(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kv_store_agent_id ON kv_store(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_agent_id ON usage(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_session_id ON usage(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_created_at ON usage(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_agent_id ON memories(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_source ON memories(source)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_deleted ON memories(deleted)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_triggers_agent_id ON triggers(agent_id)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// Agent operations

// AgentRecord represents an agent in the database.
type AgentRecord struct {
	ID            string
	Name          string
	State         string
	ModelProvider string
	ModelName     string
	CreatedAt     time.Time
	Metadata      map[string]string
}

// SaveAgent saves an agent to the database.
func (db *DB) SaveAgent(agent *AgentRecord) error {
	metadata, _ := json.Marshal(agent.Metadata)
	_, err := db.Exec(`
		INSERT OR REPLACE INTO agents (id, name, state, model_provider, model_name, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, agent.ID, agent.Name, agent.State, agent.ModelProvider, agent.ModelName, agent.CreatedAt.Format(time.RFC3339), string(metadata))
	return err
}

// GetAgent retrieves an agent by ID.
func (db *DB) GetAgent(id string) (*AgentRecord, error) {
	var agent AgentRecord
	var metadata []byte
	err := db.QueryRow(`
		SELECT id, name, state, model_provider, model_name, created_at, metadata
		FROM agents WHERE id = ?
	`, id).Scan(&agent.ID, &agent.Name, &agent.State, &agent.ModelProvider, &agent.ModelName, &agent.CreatedAt, &metadata)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(metadata) > 0 {
		json.Unmarshal(metadata, &agent.Metadata)
	}

	return &agent, nil
}

// ListAgents lists all agents.
func (db *DB) ListAgents() ([]*AgentRecord, error) {
	rows, err := db.Query(`
		SELECT id, name, state, model_provider, model_name, created_at, metadata
		FROM agents
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*AgentRecord
	for rows.Next() {
		var agent AgentRecord
		var metadata []byte
		if err := rows.Scan(&agent.ID, &agent.Name, &agent.State, &agent.ModelProvider, &agent.ModelName, &agent.CreatedAt, &metadata); err != nil {
			return nil, err
		}
		if len(metadata) > 0 {
			json.Unmarshal(metadata, &agent.Metadata)
		}
		agents = append(agents, &agent)
	}

	return agents, nil
}

// DeleteAgent deletes an agent.
func (db *DB) DeleteAgent(id string) error {
	_, err := db.Exec("DELETE FROM agents WHERE id = ?", id)
	return err
}

// Memory operations

// MemoryRecord represents a memory KV pair.
type MemoryRecord struct {
	AgentID   string
	Key       string
	Value     string
	UpdatedAt time.Time
}

// SetMemory sets a memory value.
func (db *DB) SetMemory(agentID, key, value string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO memory (agent_id, key, value, updated_at)
		VALUES (?, ?, ?, ?)
	`, agentID, key, value, time.Now().Format(time.RFC3339))
	return err
}

// GetMemory retrieves a memory value.
func (db *DB) GetMemory(agentID, key string) (*MemoryRecord, error) {
	var record MemoryRecord
	err := db.QueryRow(`
		SELECT agent_id, key, value, updated_at
		FROM memory WHERE agent_id = ? AND key = ?
	`, agentID, key).Scan(&record.AgentID, &record.Key, &record.Value, &record.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// ListMemory lists all memory for an agent.
func (db *DB) ListMemory(agentID string) ([]*MemoryRecord, error) {
	rows, err := db.Query(`
		SELECT agent_id, key, value, updated_at
		FROM memory WHERE agent_id = ?
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*MemoryRecord
	for rows.Next() {
		var record MemoryRecord
		if err := rows.Scan(&record.AgentID, &record.Key, &record.Value, &record.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, &record)
	}

	return memories, nil
}

// DeleteMemory deletes a memory value.
func (db *DB) DeleteMemory(agentID, key string) error {
	_, err := db.Exec("DELETE FROM memory WHERE agent_id = ? AND key = ?", agentID, key)
	return err
}

// KVStore operations (for kv_store table)

// KVRecord represents a KV pair from kv_store table.
type KVRecord struct {
	AgentID   string
	Key       string
	Value     []byte
	Version   int
	UpdatedAt time.Time
}

// SetKV sets a value in kv_store.
func (db *DB) SetKV(agentID, key string, value []byte) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO kv_store (agent_id, key, value, version, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(agent_id, key) DO UPDATE SET 
			value = ?, version = version + 1, updated_at = ?
	`, agentID, key, value, now, value, now)
	return err
}

// GetKV retrieves a value from kv_store.
func (db *DB) GetKV(agentID, key string) (*KVRecord, error) {
	var record KVRecord
	var updatedAtStr string
	err := db.QueryRow(`
		SELECT agent_id, key, value, version, updated_at
		FROM kv_store WHERE agent_id = ? AND key = ?
	`, agentID, key).Scan(&record.AgentID, &record.Key, &record.Value, &record.Version, &updatedAtStr)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
	return &record, nil
}

// ListKV lists all KV pairs for an agent from kv_store.
func (db *DB) ListKV(agentID string) ([]*KVRecord, error) {
	rows, err := db.Query(`
		SELECT agent_id, key, value, version, updated_at
		FROM kv_store WHERE agent_id = ? ORDER BY key
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*KVRecord
	for rows.Next() {
		var record KVRecord
		var updatedAtStr string
		if err := rows.Scan(&record.AgentID, &record.Key, &record.Value, &record.Version, &updatedAtStr); err != nil {
			return nil, err
		}
		record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		records = append(records, &record)
	}

	return records, nil
}

// DeleteKV deletes a value from kv_store.
func (db *DB) DeleteKV(agentID, key string) error {
	_, err := db.Exec("DELETE FROM kv_store WHERE agent_id = ? AND key = ?", agentID, key)
	return err
}

// Audit operations

// AuditRecord represents an audit log entry.
type AuditRecord struct {
	ID        int
	Timestamp time.Time
	Action    string
	AgentID   string
	Details   string
	Hash      string
}

// AddAudit adds an audit log entry.
func (db *DB) AddAudit(action, agentID, details string) error {
	hash := fmt.Sprintf("%x", time.Now().UnixNano())
	_, err := db.Exec(`
		INSERT INTO audit (timestamp, action, agent_id, details, hash)
		VALUES (?, ?, ?, ?, ?)
	`, time.Now().Format(time.RFC3339), action, agentID, details, hash)
	return err
}

// ListAudit lists audit entries.
func (db *DB) ListAudit(limit int) ([]*AuditRecord, error) {
	rows, err := db.Query(`
		SELECT id, timestamp, action, agent_id, details, hash
		FROM audit ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*AuditRecord
	for rows.Next() {
		var entry AuditRecord
		if err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Action, &entry.AgentID, &entry.Details, &entry.Hash); err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// Trigger operations

// TriggerRecord represents a trigger in the database.
type TriggerRecord struct {
	ID             string
	AgentID        string
	Pattern        string
	PromptTemplate string
	Enabled        bool
	CreatedAt      time.Time
	FireCount      int
	MaxFires       int
}

// SaveTrigger saves a trigger.
func (db *DB) SaveTrigger(trigger *TriggerRecord) error {
	enabled := 0
	if trigger.Enabled {
		enabled = 1
	}
	_, err := db.Exec(`
		INSERT OR REPLACE INTO triggers (id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, trigger.ID, trigger.AgentID, trigger.Pattern, trigger.PromptTemplate, enabled, trigger.CreatedAt.Format(time.RFC3339), trigger.FireCount, trigger.MaxFires)
	return err
}

// GetTrigger retrieves a trigger by ID.
func (db *DB) GetTrigger(id string) (*TriggerRecord, error) {
	var trigger TriggerRecord
	var enabled int
	err := db.QueryRow(`
		SELECT id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires
		FROM triggers WHERE id = ?
	`, id).Scan(&trigger.ID, &trigger.AgentID, &trigger.Pattern, &trigger.PromptTemplate, &enabled, &trigger.CreatedAt, &trigger.FireCount, &trigger.MaxFires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	trigger.Enabled = enabled == 1
	return &trigger, nil
}

// ListTriggers lists all triggers.
func (db *DB) ListTriggers() ([]*TriggerRecord, error) {
	rows, err := db.Query(`
		SELECT id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires
		FROM triggers
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []*TriggerRecord
	for rows.Next() {
		var trigger TriggerRecord
		var enabled int
		if err := rows.Scan(&trigger.ID, &trigger.AgentID, &trigger.Pattern, &trigger.PromptTemplate, &enabled, &trigger.CreatedAt, &trigger.FireCount, &trigger.MaxFires); err != nil {
			return nil, err
		}
		trigger.Enabled = enabled == 1
		triggers = append(triggers, &trigger)
	}

	return triggers, nil
}

// DeleteTrigger deletes a trigger.
func (db *DB) DeleteTrigger(id string) error {
	_, err := db.Exec("DELETE FROM triggers WHERE id = ?", id)
	return err
}

// Workflow operations

// WorkflowRecord represents a workflow in the database.
type WorkflowRecord struct {
	ID          string
	Name        string
	Description string
	Steps       string
	CreatedAt   time.Time
}

// SaveWorkflow saves a workflow.
func (db *DB) SaveWorkflow(workflow *WorkflowRecord) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO workflows (id, name, description, steps, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, workflow.ID, workflow.Name, workflow.Description, workflow.Steps, workflow.CreatedAt.Format(time.RFC3339))
	return err
}

// GetWorkflow retrieves a workflow by ID.
func (db *DB) GetWorkflow(id string) (*WorkflowRecord, error) {
	var workflow WorkflowRecord
	err := db.QueryRow(`
		SELECT id, name, description, steps, created_at
		FROM workflows WHERE id = ?
	`, id).Scan(&workflow.ID, &workflow.Name, &workflow.Description, &workflow.Steps, &workflow.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &workflow, nil
}

// ListWorkflows lists all workflows.
func (db *DB) ListWorkflows() ([]*WorkflowRecord, error) {
	rows, err := db.Query(`
		SELECT id, name, description, steps, created_at
		FROM workflows
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []*WorkflowRecord
	for rows.Next() {
		var workflow WorkflowRecord
		if err := rows.Scan(&workflow.ID, &workflow.Name, &workflow.Description, &workflow.Steps, &workflow.CreatedAt); err != nil {
			return nil, err
		}
		workflows = append(workflows, &workflow)
	}

	return workflows, nil
}

// DeleteWorkflow deletes a workflow.
func (db *DB) DeleteWorkflow(id string) error {
	_, err := db.Exec("DELETE FROM workflows WHERE id = ?", id)
	return err
}

// Cron job operations

// CronJobRecord represents a cron job in the database.
type CronJobRecord struct {
	ID      string
	AgentID string
	Spec    string
	Prompt  string
	Enabled bool
	NextRun *time.Time
	LastRun *time.Time
}

// SaveCronJob saves a cron job.
func (db *DB) SaveCronJob(job *CronJobRecord) error {
	enabled := 0
	if job.Enabled {
		enabled = 1
	}
	var nextRun, lastRun interface{}
	if job.NextRun != nil {
		nextRun = job.NextRun.Format(time.RFC3339)
	}
	if job.LastRun != nil {
		lastRun = job.LastRun.Format(time.RFC3339)
	}
	_, err := db.Exec(`
		INSERT OR REPLACE INTO cron_jobs (id, agent_id, spec, prompt, enabled, next_run, last_run)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.AgentID, job.Spec, job.Prompt, enabled, nextRun, lastRun)
	return err
}

// GetCronJob retrieves a cron job by ID.
func (db *DB) GetCronJob(id string) (*CronJobRecord, error) {
	var job CronJobRecord
	var enabled int
	var nextRun, lastRun sql.NullString
	err := db.QueryRow(`
		SELECT id, agent_id, spec, prompt, enabled, next_run, last_run
		FROM cron_jobs WHERE id = ?
	`, id).Scan(&job.ID, &job.AgentID, &job.Spec, &job.Prompt, &enabled, &nextRun, &lastRun)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.Enabled = enabled == 1
	if nextRun.Valid {
		t, _ := time.Parse(time.RFC3339, nextRun.String)
		job.NextRun = &t
	}
	if lastRun.Valid {
		t, _ := time.Parse(time.RFC3339, lastRun.String)
		job.LastRun = &t
	}
	return &job, nil
}

// ListCronJobs lists all cron jobs.
func (db *DB) ListCronJobs() ([]*CronJobRecord, error) {
	rows, err := db.Query(`
		SELECT id, agent_id, spec, prompt, enabled, next_run, last_run
		FROM cron_jobs
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*CronJobRecord
	for rows.Next() {
		var job CronJobRecord
		var enabled int
		var nextRun, lastRun sql.NullString
		if err := rows.Scan(&job.ID, &job.AgentID, &job.Spec, &job.Prompt, &enabled, &nextRun, &lastRun); err != nil {
			return nil, err
		}
		job.Enabled = enabled == 1
		if nextRun.Valid {
			t, _ := time.Parse(time.RFC3339, nextRun.String)
			job.NextRun = &t
		}
		if lastRun.Valid {
			t, _ := time.Parse(time.RFC3339, lastRun.String)
			job.LastRun = &t
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// DeleteCronJob deletes a cron job.
func (db *DB) DeleteCronJob(id string) error {
	_, err := db.Exec("DELETE FROM cron_jobs WHERE id = ?", id)
	return err
}

// Session operations

// SessionRecord represents a session in the database.
type SessionRecord struct {
	ID        string
	AgentID   string
	CreatedAt time.Time
	Messages  string
}

// SaveSession saves a session.
func (db *DB) SaveSession(session *SessionRecord) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO sessions (id, agent_id, created_at, messages)
		VALUES (?, ?, ?, ?)
	`, session.ID, session.AgentID, session.CreatedAt.Format(time.RFC3339), session.Messages)
	return err
}

// GetSession retrieves a session by ID.
func (db *DB) GetSession(id string) (*SessionRecord, error) {
	var session SessionRecord
	err := db.QueryRow(`
		SELECT id, agent_id, created_at, messages
		FROM sessions WHERE id = ?
	`, id).Scan(&session.ID, &session.AgentID, &session.CreatedAt, &session.Messages)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// ListSessions lists all sessions for an agent.
func (db *DB) ListSessions(agentID string) ([]*SessionRecord, error) {
	rows, err := db.Query(`
		SELECT id, agent_id, created_at, messages
		FROM sessions WHERE agent_id = ? ORDER BY created_at DESC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*SessionRecord
	for rows.Next() {
		var session SessionRecord
		if err := rows.Scan(&session.ID, &session.AgentID, &session.CreatedAt, &session.Messages); err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// DeleteSession deletes a session.
func (db *DB) DeleteSession(id string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}
