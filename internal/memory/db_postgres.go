// Package memory provides PostgreSQL storage implementation for FangClaw.
// This file implements the Storage interface using PostgreSQL with pgvector extension.
package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresDB implements the Storage interface using PostgreSQL.
type PostgresDB struct {
	db     *sql.DB
	config PostgresConfig
}

// NewPostgresDB creates a new PostgreSQL storage instance.
func NewPostgresDB(config PostgresConfig) (*PostgresDB, error) {
	if config.Port == 0 {
		config.Port = 5432
	}
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		config.Host, config.Port, config.Database,
		config.User, config.Password, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	return &PostgresDB{db: db, config: config}, nil
}

// Close closes the database connection.
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// Migrate runs database migrations.
func (p *PostgresDB) Migrate() error {
	// Enable pgvector extension
	if _, err := p.db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return fmt.Errorf("failed to create pgvector extension: %w", err)
	}

	migrations := []string{
		// Agents table
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			manifest TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			session_id TEXT NOT NULL DEFAULT ''
		)`,

		// Sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL DEFAULT '',
			agent_model_provider TEXT NOT NULL DEFAULT '',
			agent_model_name TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			messages JSONB NOT NULL
		)`,

		// Memory/KV store (for backward compatibility)
		`CREATE TABLE IF NOT EXISTS memory (
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (agent_id, key)
		)`,

		// KV store (with versioning)
		`CREATE TABLE IF NOT EXISTS kv_store (
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			key TEXT NOT NULL,
			value BYTEA NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (agent_id, key)
		)`,

		// Usage tracking
		`CREATE TABLE IF NOT EXISTS usage (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			session_id TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			provider TEXT NOT NULL,
			usage JSONB NOT NULL,
			cost_usd REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL
		)`,

		// Memories table with vector support
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			source TEXT NOT NULL,
			scope TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 1.0,
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL,
			accessed_at TIMESTAMPTZ NOT NULL,
			access_count INTEGER NOT NULL DEFAULT 0,
			deleted BOOLEAN NOT NULL DEFAULT FALSE,
			embedding vector(1536)
		)`,

		// Create vector index for similarity search
		`CREATE INDEX IF NOT EXISTS idx_memories_embedding ON memories 
		 USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,

		// Audit trail
		`CREATE TABLE IF NOT EXISTS audit (
			id SERIAL PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL,
			action TEXT NOT NULL,
			agent_id TEXT,
			details TEXT,
			hash TEXT NOT NULL
		)`,

		// Triggers table
		`CREATE TABLE IF NOT EXISTS triggers (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			pattern TEXT NOT NULL,
			prompt_template TEXT NOT NULL,
			enabled BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL,
			fire_count INTEGER DEFAULT 0,
			max_fires INTEGER DEFAULT 0
		)`,

		// Trigger history table
		`CREATE TABLE IF NOT EXISTS trigger_history (
			id TEXT PRIMARY KEY,
			trigger_id TEXT NOT NULL REFERENCES triggers(id) ON DELETE CASCADE,
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			event_type TEXT NOT NULL,
			event_description TEXT NOT NULL,
			sent_message TEXT NOT NULL,
			agent_response TEXT NOT NULL,
			session_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,

		// Workflows table
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			steps JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,

		// Cron jobs table
		`CREATE TABLE IF NOT EXISTS cron_jobs (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			spec TEXT NOT NULL,
			prompt TEXT NOT NULL,
			enabled BOOLEAN DEFAULT TRUE,
			next_run TIMESTAMPTZ,
			last_run TIMESTAMPTZ
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
		`CREATE INDEX IF NOT EXISTS idx_memories_deleted ON memories(deleted) WHERE deleted = FALSE`,
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_triggers_agent_id ON triggers(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trigger_history_trigger_id ON trigger_history(trigger_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trigger_history_agent_id ON trigger_history(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trigger_history_created_at ON trigger_history(created_at)`,
	}

	for _, m := range migrations {
		if _, err := p.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// Agent operations

// SaveAgent saves an agent to the database.
func (p *PostgresDB) SaveAgent(agent *AgentRecord) error {
	metadata, _ := json.Marshal(agent.Metadata)
	_, err := p.db.Exec(`
		INSERT INTO agents (id, name, state, model_provider, model_name, created_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			state = EXCLUDED.state,
			model_provider = EXCLUDED.model_provider,
			model_name = EXCLUDED.model_name,
			updated_at = NOW(),
			metadata = EXCLUDED.metadata
	`, agent.ID, agent.Name, agent.State, agent.ModelProvider, agent.ModelName, agent.CreatedAt.Format(time.RFC3339), string(metadata))
	return err
}

// GetAgent retrieves an agent by ID.
func (p *PostgresDB) GetAgent(id string) (*AgentRecord, error) {
	var agent AgentRecord
	var metadata []byte
	err := p.db.QueryRow(`
		SELECT id, name, state, model_provider, model_name, created_at, metadata
		FROM agents WHERE id = $1
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
func (p *PostgresDB) ListAgents() ([]*AgentRecord, error) {
	rows, err := p.db.Query(`
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
func (p *PostgresDB) DeleteAgent(id string) error {
	_, err := p.db.Exec("DELETE FROM agents WHERE id = $1", id)
	return err
}

// Session operations

// SaveSession saves a session.
func (p *PostgresDB) SaveSession(session *SessionRecord) error {
	_, err := p.db.Exec(`
		INSERT INTO sessions (id, agent_id, agent_name, agent_model_provider, agent_model_name, created_at, messages)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			messages = EXCLUDED.messages
	`, session.ID, session.AgentID, session.AgentName, session.AgentModelProvider, session.AgentModelName, session.CreatedAt.Format(time.RFC3339), session.Messages)
	return err
}

// GetSession retrieves a session by ID.
func (p *PostgresDB) GetSession(id string) (*SessionRecord, error) {
	var session SessionRecord
	err := p.db.QueryRow(`
		SELECT id, agent_id, agent_name, agent_model_provider, agent_model_name, created_at, messages
		FROM sessions WHERE id = $1
	`, id).Scan(&session.ID, &session.AgentID, &session.AgentName, &session.AgentModelProvider, &session.AgentModelName, &session.CreatedAt, &session.Messages)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// ListAllSessions lists all sessions.
func (p *PostgresDB) ListAllSessions() ([]*SessionRecord, error) {
	rows, err := p.db.Query(`
		SELECT id, agent_id, agent_name, agent_model_provider, agent_model_name, created_at, messages
		FROM sessions ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*SessionRecord
	for rows.Next() {
		var session SessionRecord
		if err := rows.Scan(&session.ID, &session.AgentID, &session.AgentName, &session.AgentModelProvider, &session.AgentModelName, &session.CreatedAt, &session.Messages); err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// DeleteSession deletes a session.
func (p *PostgresDB) DeleteSession(id string) error {
	_, err := p.db.Exec("DELETE FROM sessions WHERE id = $1", id)
	return err
}

// Memory operations

// SetMemory sets a memory value.
func (p *PostgresDB) SetMemory(agentID, key, value string) error {
	_, err := p.db.Exec(`
		INSERT INTO memory (agent_id, key, value, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (agent_id, key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = EXCLUDED.updated_at
	`, agentID, key, value)
	return err
}

// GetMemory retrieves a memory value.
func (p *PostgresDB) GetMemory(agentID, key string) (*MemoryRecord, error) {
	var record MemoryRecord
	err := p.db.QueryRow(`
		SELECT agent_id, key, value, updated_at
		FROM memory WHERE agent_id = $1 AND key = $2
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
func (p *PostgresDB) ListMemory(agentID string) ([]*MemoryRecord, error) {
	rows, err := p.db.Query(`
		SELECT agent_id, key, value, updated_at
		FROM memory WHERE agent_id = $1
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
func (p *PostgresDB) DeleteMemory(agentID, key string) error {
	_, err := p.db.Exec("DELETE FROM memory WHERE agent_id = $1 AND key = $2", agentID, key)
	return err
}

// KVStore operations

// SetKV sets a value in kv_store.
func (p *PostgresDB) SetKV(agentID, key string, value []byte) error {
	_, err := p.db.Exec(`
		INSERT INTO kv_store (agent_id, key, value, version, updated_at)
		VALUES ($1, $2, $3, 1, NOW())
		ON CONFLICT (agent_id, key) DO UPDATE SET
			value = EXCLUDED.value,
			version = kv_store.version + 1,
			updated_at = EXCLUDED.updated_at
	`, agentID, key, value)
	return err
}

// GetKV retrieves a value from kv_store.
func (p *PostgresDB) GetKV(agentID, key string) (*KVRecord, error) {
	var record KVRecord
	var updatedAt time.Time
	err := p.db.QueryRow(`
		SELECT agent_id, key, value, version, updated_at
		FROM kv_store WHERE agent_id = $1 AND key = $2
	`, agentID, key).Scan(&record.AgentID, &record.Key, &record.Value, &record.Version, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record.UpdatedAt = updatedAt
	return &record, nil
}

// ListKV lists all KV pairs for an agent.
func (p *PostgresDB) ListKV(agentID string) ([]*KVRecord, error) {
	rows, err := p.db.Query(`
		SELECT agent_id, key, value, version, updated_at
		FROM kv_store WHERE agent_id = $1 ORDER BY key
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*KVRecord
	for rows.Next() {
		var record KVRecord
		var updatedAt time.Time
		if err := rows.Scan(&record.AgentID, &record.Key, &record.Value, &record.Version, &updatedAt); err != nil {
			return nil, err
		}
		record.UpdatedAt = updatedAt
		records = append(records, &record)
	}

	return records, nil
}

// DeleteKV deletes a value from kv_store.
func (p *PostgresDB) DeleteKV(agentID, key string) error {
	_, err := p.db.Exec("DELETE FROM kv_store WHERE agent_id = $1 AND key = $2", agentID, key)
	return err
}

// Audit operations

// AddAudit adds an audit log entry.
func (p *PostgresDB) AddAudit(action, agentID, details string) error {
	hash := fmt.Sprintf("%x", time.Now().UnixNano())
	_, err := p.db.Exec(`
		INSERT INTO audit (timestamp, action, agent_id, details, hash)
		VALUES (NOW(), $1, $2, $3, $4)
	`, action, agentID, details, hash)
	return err
}

// ListAudit lists audit entries.
func (p *PostgresDB) ListAudit(limit int) ([]*AuditRecord, error) {
	rows, err := p.db.Query(`
		SELECT id, timestamp, action, agent_id, details, hash
		FROM audit ORDER BY id DESC LIMIT $1
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

// SaveTrigger saves a trigger.
func (p *PostgresDB) SaveTrigger(trigger *TriggerRecord) error {
	_, err := p.db.Exec(`
		INSERT INTO triggers (id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			pattern = EXCLUDED.pattern,
			prompt_template = EXCLUDED.prompt_template,
			enabled = EXCLUDED.enabled,
			fire_count = EXCLUDED.fire_count,
			max_fires = EXCLUDED.max_fires
	`, trigger.ID, trigger.AgentID, trigger.Pattern, trigger.PromptTemplate, trigger.Enabled, trigger.CreatedAt.Format(time.RFC3339), trigger.FireCount, trigger.MaxFires)
	return err
}

// GetTrigger retrieves a trigger by ID.
func (p *PostgresDB) GetTrigger(id string) (*TriggerRecord, error) {
	var trigger TriggerRecord
	var createdAt time.Time
	err := p.db.QueryRow(`
		SELECT id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires
		FROM triggers WHERE id = $1
	`, id).Scan(&trigger.ID, &trigger.AgentID, &trigger.Pattern, &trigger.PromptTemplate, &trigger.Enabled, &createdAt, &trigger.FireCount, &trigger.MaxFires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	trigger.CreatedAt = createdAt
	return &trigger, nil
}

// ListTriggers lists all triggers.
func (p *PostgresDB) ListTriggers() ([]*TriggerRecord, error) {
	rows, err := p.db.Query(`
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
		var createdAt time.Time
		if err := rows.Scan(&trigger.ID, &trigger.AgentID, &trigger.Pattern, &trigger.PromptTemplate, &trigger.Enabled, &createdAt, &trigger.FireCount, &trigger.MaxFires); err != nil {
			return nil, err
		}
		trigger.CreatedAt = createdAt
		triggers = append(triggers, &trigger)
	}

	return triggers, nil
}

// DeleteTrigger deletes a trigger.
func (p *PostgresDB) DeleteTrigger(id string) error {
	_, err := p.db.Exec("DELETE FROM triggers WHERE id = $1", id)
	return err
}

// Trigger history operations

// SaveTriggerHistory saves a trigger history record.
func (p *PostgresDB) SaveTriggerHistory(record *TriggerHistoryRecord) error {
	_, err := p.db.Exec(`
		INSERT INTO trigger_history (
			id, trigger_id, agent_id, event_type, event_description,
			sent_message, agent_response, session_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			sent_message = EXCLUDED.sent_message,
			agent_response = EXCLUDED.agent_response
	`, record.ID, record.TriggerID, record.AgentID, record.EventType, record.EventDescription,
		record.SentMessage, record.AgentResponse, record.SessionID, record.CreatedAt.Format(time.RFC3339))
	return err
}

// ListTriggerHistory lists trigger history records.
func (p *PostgresDB) ListTriggerHistory(triggerID, agentID string, limit int) ([]*TriggerHistoryRecord, error) {
	query := `
		SELECT id, trigger_id, agent_id, event_type, event_description,
		       sent_message, agent_response, session_id, created_at
		FROM trigger_history
		WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if triggerID != "" {
		query += fmt.Sprintf(" AND trigger_id = $%d", argIdx)
		args = append(args, triggerID)
		argIdx++
	}
	if agentID != "" {
		query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, agentID)
		argIdx++
	}

	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
	}

	rows, err := p.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*TriggerHistoryRecord
	for rows.Next() {
		var record TriggerHistoryRecord
		var createdAt time.Time
		if err := rows.Scan(&record.ID, &record.TriggerID, &record.AgentID,
			&record.EventType, &record.EventDescription, &record.SentMessage,
			&record.AgentResponse, &record.SessionID, &createdAt); err != nil {
			return nil, err
		}
		record.CreatedAt = createdAt
		records = append(records, &record)
	}

	return records, nil
}

// Workflow operations

// SaveWorkflow saves a workflow.
func (p *PostgresDB) SaveWorkflow(workflow *WorkflowRecord) error {
	_, err := p.db.Exec(`
		INSERT INTO workflows (id, name, description, steps, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			steps = EXCLUDED.steps
	`, workflow.ID, workflow.Name, workflow.Description, workflow.Steps, workflow.CreatedAt.Format(time.RFC3339))
	return err
}

// GetWorkflow retrieves a workflow by ID.
func (p *PostgresDB) GetWorkflow(id string) (*WorkflowRecord, error) {
	var workflow WorkflowRecord
	err := p.db.QueryRow(`
		SELECT id, name, description, steps, created_at
		FROM workflows WHERE id = $1
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
func (p *PostgresDB) ListWorkflows() ([]*WorkflowRecord, error) {
	rows, err := p.db.Query(`
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
func (p *PostgresDB) DeleteWorkflow(id string) error {
	_, err := p.db.Exec("DELETE FROM workflows WHERE id = $1", id)
	return err
}

// Cron job operations

// SaveCronJob saves a cron job.
func (p *PostgresDB) SaveCronJob(job *CronJobRecord) error {
	var nextRun, lastRun interface{}
	if job.NextRun != nil {
		nextRun = job.NextRun.Format(time.RFC3339)
	}
	if job.LastRun != nil {
		lastRun = job.LastRun.Format(time.RFC3339)
	}
	_, err := p.db.Exec(`
		INSERT INTO cron_jobs (id, agent_id, spec, prompt, enabled, next_run, last_run)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			spec = EXCLUDED.spec,
			prompt = EXCLUDED.prompt,
			enabled = EXCLUDED.enabled,
			next_run = EXCLUDED.next_run,
			last_run = EXCLUDED.last_run
	`, job.ID, job.AgentID, job.Spec, job.Prompt, job.Enabled, nextRun, lastRun)
	return err
}

// GetCronJob retrieves a cron job by ID.
func (p *PostgresDB) GetCronJob(id string) (*CronJobRecord, error) {
	var job CronJobRecord
	var nextRun, lastRun sql.NullTime
	err := p.db.QueryRow(`
		SELECT id, agent_id, spec, prompt, enabled, next_run, last_run
		FROM cron_jobs WHERE id = $1
	`, id).Scan(&job.ID, &job.AgentID, &job.Spec, &job.Prompt, &job.Enabled, &nextRun, &lastRun)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if nextRun.Valid {
		job.NextRun = &nextRun.Time
	}
	if lastRun.Valid {
		job.LastRun = &lastRun.Time
	}
	return &job, nil
}

// ListCronJobs lists all cron jobs.
func (p *PostgresDB) ListCronJobs() ([]*CronJobRecord, error) {
	rows, err := p.db.Query(`
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
		var nextRun, lastRun sql.NullTime
		if err := rows.Scan(&job.ID, &job.AgentID, &job.Spec, &job.Prompt, &job.Enabled, &nextRun, &lastRun); err != nil {
			return nil, err
		}
		if nextRun.Valid {
			job.NextRun = &nextRun.Time
		}
		if lastRun.Valid {
			job.LastRun = &lastRun.Time
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// DeleteCronJob deletes a cron job.
func (p *PostgresDB) DeleteCronJob(id string) error {
	_, err := p.db.Exec("DELETE FROM cron_jobs WHERE id = $1", id)
	return err
}

// VectorSearch performs vector similarity search using pgvector.
func (p *PostgresDB) VectorSearch(embedding []float32, limit int, agentID string) ([]struct {
	ID       string
	Content  string
	Distance float64
}, error) {
	// Convert embedding to PostgreSQL vector format
	vecStr := vectorToPostgresArray(embedding)

	query := `
		SELECT id, content, embedding <=> $1 as distance
		FROM memories
		WHERE deleted = false
	`
	args := []interface{}{vecStr}
	argIdx := 2

	if agentID != "" {
		query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, agentID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY embedding <=> $1 LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := p.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ID       string
		Content  string
		Distance float64
	}

	for rows.Next() {
		var r struct {
			ID       string
			Content  string
			Distance float64
		}
		if err := rows.Scan(&r.ID, &r.Content, &r.Distance); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, nil
}

// vectorToPostgresArray converts a float slice to PostgreSQL vector format.
func vectorToPostgresArray(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
