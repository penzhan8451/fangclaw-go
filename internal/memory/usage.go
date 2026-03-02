package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// UsageStore tracks token usage and costs.
type UsageStore struct {
	db *DB
}

// NewUsageStore creates a new usage store.
func NewUsageStore(db *DB) *UsageStore {
	return &UsageStore{db: db}
}

// RecordUsage records a token usage event.
func (s *UsageStore) RecordUsage(record *types.UsageRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}

	usageBytes, err := json.Marshal(record.Usage)
	if err != nil {
		return fmt.Errorf("failed to marshal usage: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO usage (id, agent_id, session_id, model, provider, usage, cost_usd, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.ID,
		record.AgentID.String(),
		record.SessionID.String(),
		record.Model,
		record.Provider,
		usageBytes,
		record.CostUSD,
		record.CreatedAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	return nil
}

// GetUsage retrieves a usage record by ID.
func (s *UsageStore) GetUsage(id string) (*types.UsageRecord, error) {
	var agentIDStr, sessionIDStr, model, provider string
	var usageBytes []byte
	var costUSD float64
	var createdAtStr string

	err := s.db.QueryRow(`
		SELECT agent_id, session_id, model, provider, usage, cost_usd, created_at
		FROM usage WHERE id = ?
	`, id).Scan(
		&agentIDStr, &sessionIDStr, &model, &provider,
		&usageBytes, &costUSD, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent_id: %w", err)
	}

	sessionID, err := types.ParseSessionID(sessionIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse session_id: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	var usage types.TokenUsage
	if err := json.Unmarshal(usageBytes, &usage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal usage: %w", err)
	}

	return &types.UsageRecord{
		ID:        id,
		AgentID:   agentID,
		SessionID: sessionID,
		Model:     model,
		Provider:  provider,
		Usage:     usage,
		CostUSD:   costUSD,
		CreatedAt: createdAt,
	}, nil
}

// ListUsage lists usage records with optional filters.
func (s *UsageStore) ListUsage(agentID *types.AgentID, start, end *time.Time, limit int) ([]*types.UsageRecord, error) {
	query := `
		SELECT id, agent_id, session_id, model, provider, usage, cost_usd, created_at
		FROM usage WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if agentID != nil {
		query += fmt.Sprintf(" AND agent_id = ?%d", argIndex)
		args = append(args, agentID.String())
		argIndex++
	}

	if start != nil {
		query += fmt.Sprintf(" AND created_at >= ?%d", argIndex)
		args = append(args, start.Format(time.RFC3339))
		argIndex++
	}

	if end != nil {
		query += fmt.Sprintf(" AND created_at <= ?%d", argIndex)
		args = append(args, end.Format(time.RFC3339))
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list usage: %w", err)
	}
	defer rows.Close()

	var records []*types.UsageRecord
	for rows.Next() {
		var id, agentIDStr, sessionIDStr, model, provider string
		var usageBytes []byte
		var costUSD float64
		var createdAtStr string

		if err := rows.Scan(
			&id, &agentIDStr, &sessionIDStr, &model, &provider,
			&usageBytes, &costUSD, &createdAtStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan usage: %w", err)
		}

		agentID, err := types.ParseAgentID(agentIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse agent_id: %w", err)
		}

		sessionID, err := types.ParseSessionID(sessionIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse session_id: %w", err)
		}

		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		var usage types.TokenUsage
		if err := json.Unmarshal(usageBytes, &usage); err != nil {
			return nil, fmt.Errorf("failed to unmarshal usage: %w", err)
		}

		records = append(records, &types.UsageRecord{
			ID:        id,
			AgentID:   agentID,
			SessionID: sessionID,
			Model:     model,
			Provider:  provider,
			Usage:     usage,
			CostUSD:   costUSD,
			CreatedAt: createdAt,
		})
	}

	return records, nil
}

// GetUsageSummary gets a summary of usage over a period.
func (s *UsageStore) GetUsageSummary(agentID *types.AgentID, start, end *time.Time) (*types.UsageSummary, error) {
	query := `
		SELECT COUNT(*), COALESCE(SUM(json_extract(usage, '$.prompt_tokens')), 0),
		       COALESCE(SUM(json_extract(usage, '$.completion_tokens')), 0),
		       COALESCE(SUM(json_extract(usage, '$.total_tokens')), 0),
		       COALESCE(SUM(cost_usd), 0)
		FROM usage WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if agentID != nil {
		query += fmt.Sprintf(" AND agent_id = ?%d", argIndex)
		args = append(args, agentID.String())
		argIndex++
	}

	if start != nil {
		query += fmt.Sprintf(" AND created_at >= ?%d", argIndex)
		args = append(args, start.Format(time.RFC3339))
		argIndex++
	}

	if end != nil {
		query += fmt.Sprintf(" AND created_at <= ?%d", argIndex)
		args = append(args, end.Format(time.RFC3339))
		argIndex++
	}

	var recordCount int
	var totalPrompt, totalCompletion, totalTotal int
	var totalCost float64

	err := s.db.QueryRow(query, args...).Scan(
		&recordCount, &totalPrompt, &totalCompletion, &totalTotal, &totalCost,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get usage summary: %w", err)
	}

	return &types.UsageSummary{
		TotalPromptTokens:     totalPrompt,
		TotalCompletionTokens: totalCompletion,
		TotalTokens:           totalTotal,
		TotalCostUSD:          totalCost,
		RecordCount:           recordCount,
	}, nil
}

// DeleteOldUsage deletes usage records older than the given time.
func (s *UsageStore) DeleteOldUsage(before time.Time) (int64, error) {
	result, err := s.db.Exec(`
		DELETE FROM usage WHERE created_at < ?
	`, before.Format(time.RFC3339))

	if err != nil {
		return 0, fmt.Errorf("failed to delete old usage: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}
