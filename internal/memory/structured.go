package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// StructuredStore provides key-value and agent persistence.
type StructuredStore struct {
	db *DB
}

// NewStructuredStore creates a new structured store.
func NewStructuredStore(db *DB) *StructuredStore {
	return &StructuredStore{db: db}
}

// Get retrieves a value from the key-value store.
func (s *StructuredStore) Get(agentID types.AgentID, key string) (interface{}, error) {
	var value []byte
	err := s.db.QueryRow(`
		SELECT value FROM kv_store WHERE agent_id = ? AND key = ?
	`, agentID.String(), key).Scan(&value)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get kv: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return result, nil
}

// Set sets a value in the key-value store.
func (s *StructuredStore) Set(agentID types.AgentID, key string, value interface{}) error {
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	_, err = s.db.Exec(`
		INSERT INTO kv_store (agent_id, key, value, version, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(agent_id, key) DO UPDATE SET
			value = ?, version = version + 1, updated_at = ?
	`, agentID.String(), key, valueBytes, now, valueBytes, now)

	if err != nil {
		return fmt.Errorf("failed to set kv: %w", err)
	}

	return nil
}

// Delete deletes a value from the key-value store.
func (s *StructuredStore) Delete(agentID types.AgentID, key string) error {
	_, err := s.db.Exec(`
		DELETE FROM kv_store WHERE agent_id = ? AND key = ?
	`, agentID.String(), key)

	if err != nil {
		return fmt.Errorf("failed to delete kv: %w", err)
	}

	return nil
}

// ListKV lists all key-value pairs for an agent.
func (s *StructuredStore) ListKV(agentID types.AgentID) (map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT key, value FROM kv_store WHERE agent_id = ? ORDER BY key
	`, agentID.String())

	if err != nil {
		return nil, fmt.Errorf("failed to list kv: %w", err)
	}
	defer rows.Close()

	result := make(map[string]interface{})
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan kv: %w", err)
		}

		var val interface{}
		if err := json.Unmarshal(value, &val); err != nil {
			return nil, fmt.Errorf("failed to unmarshal value: %w", err)
		}

		result[key] = val
	}

	return result, nil
}

// SaveAgent saves an agent entry to the database.
func (s *StructuredStore) SaveAgent(entry *types.AgentEntry) error {
	manifestBytes, err := json.Marshal(entry.Manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	stateBytes, err := json.Marshal(entry.State)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	_, err = s.db.Exec(`
		INSERT INTO agents (id, name, manifest, state, created_at, updated_at, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = ?, manifest = ?, state = ?, updated_at = ?, session_id = ?
	`,
		entry.ID.String(),
		entry.Name,
		manifestBytes,
		stateBytes,
		entry.CreatedAt.Format(time.RFC3339),
		now,
		entry.SessionID.String(),
		entry.Name,
		manifestBytes,
		stateBytes,
		now,
		entry.SessionID.String(),
	)

	if err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	return nil
}

// LoadAgent loads an agent entry from the database.
func (s *StructuredStore) LoadAgent(agentID types.AgentID) (*types.AgentEntry, error) {
	var id, name string
	var manifestBytes, stateBytes []byte
	var createdAtStr, updatedAtStr, sessionIDStr string

	err := s.db.QueryRow(`
		SELECT id, name, manifest, state, created_at, updated_at, session_id
		FROM agents WHERE id = ?
	`, agentID.String()).Scan(
		&id, &name, &manifestBytes, &stateBytes,
		&createdAtStr, &updatedAtStr, &sessionIDStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load agent: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	var manifest types.AgentManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	sessionID, err := types.ParseSessionID(sessionIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse session_id: %w", err)
	}

	return &types.AgentEntry{
		ID:        agentID,
		Name:      name,
		Manifest:  manifest,
		State:     state,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		SessionID: sessionID,
	}, nil
}

// ListAgents lists all agents.
func (s *StructuredStore) ListAgents() ([]*types.AgentEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, name, manifest, state, created_at, updated_at, session_id
		FROM agents ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer rows.Close()

	var agents []*types.AgentEntry
	for rows.Next() {
		var id, name string
		var manifestBytes, stateBytes []byte
		var createdAtStr, updatedAtStr, sessionIDStr string

		if err := rows.Scan(
			&id, &name, &manifestBytes, &stateBytes,
			&createdAtStr, &updatedAtStr, &sessionIDStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		agentID, err := types.ParseAgentID(id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse agent_id: %w", err)
		}

		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}

		var manifest types.AgentManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}

		var state map[string]interface{}
		if err := json.Unmarshal(stateBytes, &state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state: %w", err)
		}

		sessionID, err := types.ParseSessionID(sessionIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse session_id: %w", err)
		}

		agents = append(agents, &types.AgentEntry{
			ID:        agentID,
			Name:      name,
			Manifest:  manifest,
			State:     state,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			SessionID: sessionID,
		})
	}

	return agents, nil
}

// DeleteAgent deletes an agent.
func (s *StructuredStore) DeleteAgent(agentID types.AgentID) error {
	_, err := s.db.Exec("DELETE FROM agents WHERE id = ?", agentID.String())
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}
	return nil
}
