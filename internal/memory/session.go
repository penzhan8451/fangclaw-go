// Package memory implements the memory substrate for OpenFang.
package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// SessionStore implements session management.
type SessionStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSessionStore creates a new session store.
func NewSessionStore(dbPath string) (*SessionStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SessionStore{
		db: db,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return store, nil
}

// initSchema initializes the database schema.
func (s *SessionStore) initSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			messages BLOB NOT NULL,
			context_window_tokens INTEGER NOT NULL DEFAULT 0,
			label TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);
	`)
	if err != nil {
		return err
	}

	// Function to check if column exists
	columnExists := func(colName string) bool {
		var dummy int
		err := s.db.QueryRow("SELECT 1 FROM pragma_table_info('sessions') WHERE name = ?", colName).Scan(&dummy)
		return err == nil
	}

	// Add context_window_tokens if missing
	if !columnExists("context_window_tokens") {
		_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN context_window_tokens INTEGER NOT NULL DEFAULT 0")
	}

	// Add label if missing
	if !columnExists("label") {
		_, _ = s.db.Exec("ALTER TABLE sessions ADD COLUMN label TEXT")
	}

	return nil
}

// GetSession loads a session from the database.
func (s *SessionStore) GetSession(sessionID types.SessionID) (*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var agentIDStr string
	var messagesBlob []byte
	var tokens int64
	var label sql.NullString
	var createdAt, updatedAt string

	err := s.db.QueryRow(`
		SELECT agent_id, messages, context_window_tokens, label, created_at, updated_at
		FROM sessions WHERE id = ?
	`, sessionID.String()).Scan(&agentIDStr, &messagesBlob, &tokens, &label, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID: %w", err)
	}

	var messages []types.Message
	if err := json.Unmarshal(messagesBlob, &messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	var sessionLabel *string
	if label.Valid {
		sessionLabel = &label.String
	}

	createdTime, _ := time.Parse(time.RFC3339, createdAt)
	updatedTime, _ := time.Parse(time.RFC3339, updatedAt)

	return &types.Session{
		ID:                  sessionID,
		AgentID:             agentID,
		Messages:            messages,
		ContextWindowTokens: uint64(tokens),
		Label:               sessionLabel,
		CreatedAt:           createdTime,
		UpdatedAt:           updatedTime,
	}, nil
}

// SaveSession saves a session to the database.
func (s *SessionStore) SaveSession(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	messagesBlob, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	var label interface{}
	if session.Label != nil {
		label = *session.Label
	} else {
		label = nil
	}

	_, err = s.db.Exec(`
		INSERT INTO sessions (id, agent_id, messages, context_window_tokens, label, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			messages = excluded.messages,
			context_window_tokens = excluded.context_window_tokens,
			label = excluded.label,
			updated_at = excluded.updated_at
	`, session.ID.String(), session.AgentID.String(), messagesBlob,
		session.ContextWindowTokens, label, now, now)

	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// DeleteSession deletes a session.
func (s *SessionStore) DeleteSession(sessionID types.SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID.String())
	return err
}

// DeleteAgentSessions deletes all sessions for an agent.
func (s *SessionStore) DeleteAgentSessions(agentID types.AgentID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM sessions WHERE agent_id = ?", agentID.String())
	return err
}

// ListSessions lists all sessions with metadata.
func (s *SessionStore) ListSessions() ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, agent_id, messages, created_at, label
		FROM sessions ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var idStr, agentIDStr, createdAt string
		var messagesBlob []byte
		var label sql.NullString

		err := rows.Scan(&idStr, &agentIDStr, &messagesBlob, &createdAt, &label)
		if err != nil {
			continue
		}

		var messages []types.Message
		json.Unmarshal(messagesBlob, &messages)

		sessionInfo := map[string]interface{}{
			"session_id":    idStr,
			"agent_id":      agentIDStr,
			"message_count": len(messages),
			"created_at":    createdAt,
		}

		if label.Valid {
			sessionInfo["label"] = label.String
		}

		result = append(result, sessionInfo)
	}

	return result, nil
}

// ListAgentSessions lists all sessions for a specific agent.
func (s *SessionStore) ListAgentSessions(agentID types.AgentID) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, agent_id, messages, created_at, label
		FROM sessions WHERE agent_id = ? ORDER BY created_at DESC
	`, agentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var idStr, agentIDStr, createdAt string
		var messagesBlob []byte
		var label sql.NullString

		err := rows.Scan(&idStr, &agentIDStr, &messagesBlob, &createdAt, &label)
		if err != nil {
			continue
		}

		var messages []types.Message
		json.Unmarshal(messagesBlob, &messages)

		sessionInfo := map[string]interface{}{
			"session_id":    idStr,
			"agent_id":      agentIDStr,
			"message_count": len(messages),
			"created_at":    createdAt,
		}

		if label.Valid {
			sessionInfo["label"] = label.String
		}

		result = append(result, sessionInfo)
	}

	return result, nil
}

// Close closes the database.
func (s *SessionStore) Close() error {
	return s.db.Close()
}
