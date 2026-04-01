package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type UserIDFilter struct {
	UserID string
}

func NewUserIDFilter(userID string) *UserIDFilter {
	return &UserIDFilter{UserID: userID}
}

func (f *UserIDFilter) WhereClause() string {
	if f.UserID == "" {
		return ""
	}
	return "user_id = ?"
}

func (f *UserIDFilter) Args() []interface{} {
	if f.UserID == "" {
		return nil
	}
	return []interface{}{f.UserID}
}

func (db *DB) SaveAgentWithUser(agent *AgentRecord, userID string) error {
	metadata, _ := json.Marshal(agent.Metadata)
	_, err := db.Exec(`
		INSERT OR REPLACE INTO agents (id, name, state, model_provider, model_name, created_at, metadata, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, agent.ID, agent.Name, agent.State, agent.ModelProvider, agent.ModelName, agent.CreatedAt.Format(time.RFC3339), string(metadata), userID)
	return err
}

func (db *DB) GetAgentWithUser(id string, userID string) (*AgentRecord, error) {
	var agent AgentRecord
	var metadata []byte
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, name, state, model_provider, model_name, created_at, metadata, user_id FROM agents WHERE id = ? AND user_id = ?`
		args = []interface{}{id, userID}
	} else {
		query = `SELECT id, name, state, model_provider, model_name, created_at, metadata, user_id FROM agents WHERE id = ?`
		args = []interface{}{id}
	}

	err := db.QueryRow(query, args...).Scan(&agent.ID, &agent.Name, &agent.State, &agent.ModelProvider, &agent.ModelName, &agent.CreatedAt, &metadata, &agent.UserID)

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

func (db *DB) ListAgentsByUser(userID string) ([]*AgentRecord, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, name, state, model_provider, model_name, created_at, metadata, user_id FROM agents WHERE user_id = ?`
		args = []interface{}{userID}
	} else {
		query = `SELECT id, name, state, model_provider, model_name, created_at, metadata, user_id FROM agents`
		args = nil
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*AgentRecord
	for rows.Next() {
		var agent AgentRecord
		var metadata []byte
		if err := rows.Scan(&agent.ID, &agent.Name, &agent.State, &agent.ModelProvider, &agent.ModelName, &agent.CreatedAt, &metadata, &agent.UserID); err != nil {
			return nil, err
		}
		if len(metadata) > 0 {
			json.Unmarshal(metadata, &agent.Metadata)
		}
		agents = append(agents, &agent)
	}

	return agents, nil
}

func (db *DB) DeleteAgentWithUser(id string, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `DELETE FROM agents WHERE id = ? AND user_id = ?`
		args = []interface{}{id, userID}
	} else {
		query = `DELETE FROM agents WHERE id = ?`
		args = []interface{}{id}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) SetMemoryWithUser(agentID, key, value, userID string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO memory (agent_id, key, value, updated_at, user_id)
		VALUES (?, ?, ?, ?, ?)
	`, agentID, key, value, time.Now().Format(time.RFC3339), userID)
	return err
}

func (db *DB) GetMemoryWithUser(agentID, key, userID string) (*MemoryRecord, error) {
	var record MemoryRecord
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT agent_id, key, value, updated_at FROM memory WHERE agent_id = ? AND key = ? AND user_id = ?`
		args = []interface{}{agentID, key, userID}
	} else {
		query = `SELECT agent_id, key, value, updated_at FROM memory WHERE agent_id = ? AND key = ?`
		args = []interface{}{agentID, key}
	}

	err := db.QueryRow(query, args...).Scan(&record.AgentID, &record.Key, &record.Value, &record.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (db *DB) ListMemoryByUser(agentID, userID string) ([]*MemoryRecord, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT agent_id, key, value, updated_at FROM memory WHERE agent_id = ? AND user_id = ?`
		args = []interface{}{agentID, userID}
	} else {
		query = `SELECT agent_id, key, value, updated_at FROM memory WHERE agent_id = ?`
		args = []interface{}{agentID}
	}

	rows, err := db.Query(query, args...)
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

func (db *DB) DeleteMemoryWithUser(agentID, key, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `DELETE FROM memory WHERE agent_id = ? AND key = ? AND user_id = ?`
		args = []interface{}{agentID, key, userID}
	} else {
		query = `DELETE FROM memory WHERE agent_id = ? AND key = ?`
		args = []interface{}{agentID, key}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) SaveTriggerWithUser(triggerID, agentID, pattern, promptTemplate string, enabled bool, userID string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO triggers (id, agent_id, pattern, prompt_template, enabled, created_at, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, triggerID, agentID, pattern, promptTemplate, enabled, time.Now().Format(time.RFC3339), userID)
	return err
}

func (db *DB) ListTriggersByUser(userID string) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires FROM triggers WHERE user_id = ?`
		args = []interface{}{userID}
	} else {
		query = `SELECT id, agent_id, pattern, prompt_template, enabled, created_at, fire_count, max_fires FROM triggers`
		args = nil
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []map[string]interface{}
	for rows.Next() {
		var id, agentID, pattern, promptTemplate, createdAt string
		var enabled bool
		var fireCount, maxFires int
		if err := rows.Scan(&id, &agentID, &pattern, &promptTemplate, &enabled, &createdAt, &fireCount, &maxFires); err != nil {
			return nil, err
		}
		triggers = append(triggers, map[string]interface{}{
			"id":             id,
			"agent_id":       agentID,
			"pattern":        pattern,
			"prompt_template": promptTemplate,
			"enabled":        enabled,
			"created_at":     createdAt,
			"fire_count":     fireCount,
			"max_fires":      maxFires,
		})
	}

	return triggers, nil
}

func (db *DB) DeleteTriggerWithUser(triggerID, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `DELETE FROM triggers WHERE id = ? AND user_id = ?`
		args = []interface{}{triggerID, userID}
	} else {
		query = `DELETE FROM triggers WHERE id = ?`
		args = []interface{}{triggerID}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) SaveCronJobWithUser(jobID, agentID, spec, prompt string, enabled bool, userID string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO cron_jobs (id, agent_id, spec, prompt, enabled, user_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, jobID, agentID, spec, prompt, enabled, userID)
	return err
}

func (db *DB) ListCronJobsByUser(userID string) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, agent_id, spec, prompt, enabled, next_run, last_run FROM cron_jobs WHERE user_id = ?`
		args = []interface{}{userID}
	} else {
		query = `SELECT id, agent_id, spec, prompt, enabled, next_run, last_run FROM cron_jobs`
		args = nil
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []map[string]interface{}
	for rows.Next() {
		var id, agentID, spec, prompt string
		var enabled bool
		var nextRun, lastRun sql.NullString
		if err := rows.Scan(&id, &agentID, &spec, &prompt, &enabled, &nextRun, &lastRun); err != nil {
			return nil, err
		}
		job := map[string]interface{}{
			"id":        id,
			"agent_id":  agentID,
			"spec":      spec,
			"prompt":    prompt,
			"enabled":   enabled,
		}
		if nextRun.Valid {
			job["next_run"] = nextRun.String
		}
		if lastRun.Valid {
			job["last_run"] = lastRun.String
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (db *DB) DeleteCronJobWithUser(jobID, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `DELETE FROM cron_jobs WHERE id = ? AND user_id = ?`
		args = []interface{}{jobID, userID}
	} else {
		query = `DELETE FROM cron_jobs WHERE id = ?`
		args = []interface{}{jobID}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) SaveWorkflowWithUser(workflowID, name, description string, steps []map[string]interface{}, userID string) error {
	stepsJSON, _ := json.Marshal(steps)
	_, err := db.Exec(`
		INSERT OR REPLACE INTO workflows (id, name, description, steps, created_at, user_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, workflowID, name, description, string(stepsJSON), time.Now().Format(time.RFC3339), userID)
	return err
}

func (db *DB) ListWorkflowsByUser(userID string) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, name, description, steps, created_at FROM workflows WHERE user_id = ?`
		args = []interface{}{userID}
	} else {
		query = `SELECT id, name, description, steps, created_at FROM workflows`
		args = nil
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []map[string]interface{}
	for rows.Next() {
		var id, name, description, stepsJSON, createdAt string
		if err := rows.Scan(&id, &name, &description, &stepsJSON, &createdAt); err != nil {
			return nil, err
		}
		var steps []map[string]interface{}
		json.Unmarshal([]byte(stepsJSON), &steps)
		workflows = append(workflows, map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
			"steps":       steps,
			"created_at":  createdAt,
		})
	}

	return workflows, nil
}

func (db *DB) DeleteWorkflowWithUser(workflowID, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `DELETE FROM workflows WHERE id = ? AND user_id = ?`
		args = []interface{}{workflowID, userID}
	} else {
		query = `DELETE FROM workflows WHERE id = ?`
		args = []interface{}{workflowID}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) SaveSessionWithUser(sessionID, agentID, agentName, modelProvider, modelName, userID string, messages []interface{}) error {
	messagesJSON, _ := json.Marshal(messages)
	_, err := db.Exec(`
		INSERT OR REPLACE INTO sessions (id, agent_id, agent_name, agent_model_provider, agent_model_name, created_at, messages, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, agentID, agentName, modelProvider, modelName, time.Now().Format(time.RFC3339), string(messagesJSON), userID)
	return err
}

func (db *DB) ListSessionsByUser(userID string) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, agent_id, agent_name, agent_model_provider, agent_model_name, created_at, messages FROM sessions WHERE user_id = ?`
		args = []interface{}{userID}
	} else {
		query = `SELECT id, agent_id, agent_name, agent_model_provider, agent_model_name, created_at, messages FROM sessions`
		args = nil
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, agentID, agentName, modelProvider, modelName, createdAt, messagesJSON string
		if err := rows.Scan(&id, &agentID, &agentName, &modelProvider, &modelName, &createdAt, &messagesJSON); err != nil {
			return nil, err
		}
		var messages []interface{}
		json.Unmarshal([]byte(messagesJSON), &messages)
		sessions = append(sessions, map[string]interface{}{
			"id":                   id,
			"agent_id":             agentID,
			"agent_name":           agentName,
			"agent_model_provider": modelProvider,
			"agent_model_name":     modelName,
			"created_at":           createdAt,
			"messages":             messages,
		})
	}

	return sessions, nil
}

func (db *DB) DeleteSessionWithUser(sessionID, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `DELETE FROM sessions WHERE id = ? AND user_id = ?`
		args = []interface{}{sessionID, userID}
	} else {
		query = `DELETE FROM sessions WHERE id = ?`
		args = []interface{}{sessionID}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) SaveUsageWithUser(usageID, agentID, sessionID, model, provider string, usageData map[string]interface{}, costUSD float64, userID string) error {
	usageJSON, _ := json.Marshal(usageData)
	_, err := db.Exec(`
		INSERT INTO usage (id, agent_id, session_id, model, provider, usage, cost_usd, created_at, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, usageID, agentID, sessionID, model, provider, string(usageJSON), costUSD, time.Now().Format(time.RFC3339), userID)
	return err
}

func (db *DB) ListUsageByUser(userID string, limit int) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, agent_id, session_id, model, provider, usage, cost_usd, created_at FROM usage WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`
		args = []interface{}{userID, limit}
	} else {
		query = `SELECT id, agent_id, session_id, model, provider, usage, cost_usd, created_at FROM usage ORDER BY created_at DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usageRecords []map[string]interface{}
	for rows.Next() {
		var id, agentID, sessionID, model, provider, usageJSON, createdAt string
		var costUSD float64
		if err := rows.Scan(&id, &agentID, &sessionID, &model, &provider, &usageJSON, &costUSD, &createdAt); err != nil {
			return nil, err
		}
		var usageData map[string]interface{}
		json.Unmarshal([]byte(usageJSON), &usageData)
		usageRecords = append(usageRecords, map[string]interface{}{
			"id":         id,
			"agent_id":   agentID,
			"session_id": sessionID,
			"model":      model,
			"provider":   provider,
			"usage":      usageData,
			"cost_usd":   costUSD,
			"created_at": createdAt,
		})
	}

	return usageRecords, nil
}

func (db *DB) SaveMemoryRecordWithUser(memoryID, agentID, content, source, scope string, confidence float64, metadata map[string]interface{}, userID string) error {
	metadataJSON, _ := json.Marshal(metadata)
	_, err := db.Exec(`
		INSERT INTO memories (id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, memoryID, agentID, content, source, scope, confidence, string(metadataJSON), time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), userID)
	return err
}

func (db *DB) ListMemoriesByUser(agentID, userID string) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = `SELECT id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count FROM memories WHERE agent_id = ? AND user_id = ? AND deleted = 0`
		args = []interface{}{agentID, userID}
	} else {
		query = `SELECT id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count FROM memories WHERE agent_id = ? AND deleted = 0`
		args = []interface{}{agentID}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []map[string]interface{}
	for rows.Next() {
		var id, agentIDVal, content, source, scope, metadataJSON, createdAt, accessedAt string
		var confidence float64
		var accessCount int
		if err := rows.Scan(&id, &agentIDVal, &content, &source, &scope, &confidence, &metadataJSON, &createdAt, &accessedAt, &accessCount); err != nil {
			return nil, err
		}
		var metadata map[string]interface{}
		json.Unmarshal([]byte(metadataJSON), &metadata)
		memories = append(memories, map[string]interface{}{
			"id":           id,
			"agent_id":     agentIDVal,
			"content":      content,
			"source":       source,
			"scope":        scope,
			"confidence":   confidence,
			"metadata":     metadata,
			"created_at":   createdAt,
			"accessed_at":  accessedAt,
			"access_count": accessCount,
		})
	}

	return memories, nil
}

func (db *DB) DeleteMemoryRecordWithUser(memoryID, userID string) error {
	var query string
	var args []interface{}

	if userID != "" {
		query = `UPDATE memories SET deleted = 1 WHERE id = ? AND user_id = ?`
		args = []interface{}{memoryID, userID}
	} else {
		query = `UPDATE memories SET deleted = 1 WHERE id = ?`
		args = []interface{}{memoryID}
	}

	_, err := db.Exec(query, args...)
	return err
}

func (db *DB) CountRecordsByUser(tableName, userID string) (int, error) {
	var query string
	var args []interface{}

	if userID != "" {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = ?", tableName)
		args = []interface{}{userID}
	} else {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
		args = nil
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}
