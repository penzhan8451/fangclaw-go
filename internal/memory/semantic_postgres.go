// Package memory provides PostgreSQL semantic storage implementation.
package memory

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// PostgresSemanticStore implements semantic memory using PostgreSQL with pgvector.
type PostgresSemanticStore struct {
	db *PostgresDB
}

// NewPostgresSemanticStore creates a new PostgreSQL semantic store.
func NewPostgresSemanticStore(db *PostgresDB) (*PostgresSemanticStore, error) {
	return &PostgresSemanticStore{db: db}, nil
}

// Remember stores a new memory fragment.
func (p *PostgresSemanticStore) Remember(
	agentID types.AgentID,
	content string,
	source types.MemorySource,
	scope string,
	metadata map[string]interface{},
) (types.MemoryID, error) {
	return p.RememberWithEmbedding(agentID, content, source, scope, metadata, nil)
}

// RememberWithEmbedding stores a memory with an optional embedding.
func (p *PostgresSemanticStore) RememberWithEmbedding(
	agentID types.AgentID,
	content string,
	source types.MemorySource,
	scope string,
	metadata map[string]interface{},
	embedding []float32,
) (types.MemoryID, error) {
	id := types.NewMemoryID()
	now := time.Now().UTC()

	sourceBytes, err := json.Marshal(source)
	if err != nil {
		return types.MemoryID{}, fmt.Errorf("failed to marshal source: %w", err)
	}

	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return types.MemoryID{}, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Build the query
	sql := `
		INSERT INTO memories (id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count, deleted)
		VALUES ($1, $2, $3, $4, $5, 1.0, $6, $7, $8, 0, false)
	`
	args := []interface{}{
		id.String(),
		agentID.String(),
		content,
		string(sourceBytes),
		scope,
		string(metaBytes),
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	}

	// If embedding is provided, include it
	if embedding != nil {
		sql = `
			INSERT INTO memories (id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count, deleted, embedding)
			VALUES ($1, $2, $3, $4, $5, 1.0, $6, $7, $8, 0, false, $9::vector)
		`
		args = append(args, vectorToPostgresArray(embedding))
	}

	_, err = p.db.db.Exec(sql, args...)
	if err != nil {
		return types.MemoryID{}, fmt.Errorf("failed to insert memory: %w", err)
	}

	return id, nil
}

// Recall searches for memories using text matching.
func (p *PostgresSemanticStore) Recall(
	query string,
	limit int,
	filter *types.MemoryFilter,
) ([]types.MemoryFragment, error) {
	return p.RecallWithEmbedding(query, limit, filter, nil)
}

// RecallWithEmbedding searches for memories using vector similarity or text matching.
func (p *PostgresSemanticStore) RecallWithEmbedding(
	query string,
	limit int,
	filter *types.MemoryFilter,
	queryEmbedding []float32,
) ([]types.MemoryFragment, error) {
	// If we have an embedding, use vector search
	if queryEmbedding != nil && len(queryEmbedding) > 0 {
		return p.vectorSearch(queryEmbedding, limit, filter)
	}

	// Otherwise use text search
	return p.textSearch(query, limit, filter)
}

// vectorSearch performs similarity search using pgvector.
func (p *PostgresSemanticStore) vectorSearch(
	embedding []float32,
	limit int,
	filter *types.MemoryFilter,
) ([]types.MemoryFragment, error) {
	vecStr := vectorToPostgresArray(embedding)

	// Build the query with dynamic filters
	sql := `
		SELECT id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count,
		       embedding <=> $1::vector as distance
		FROM memories
		WHERE deleted = false
	`
	args := []interface{}{vecStr}
	argIdx := 2

	// Apply filters
	if filter != nil {
		if filter.AgentID != nil {
			sql += fmt.Sprintf(" AND agent_id = $%d", argIdx)
			args = append(args, filter.AgentID.String())
			argIdx++
		}
		if filter.Scope != nil {
			sql += fmt.Sprintf(" AND scope = $%d", argIdx)
			args = append(args, *filter.Scope)
			argIdx++
		}
		if filter.MinConfidence != nil {
			sql += fmt.Sprintf(" AND confidence >= $%d", argIdx)
			args = append(args, *filter.MinConfidence)
			argIdx++
		}
		if filter.Source != nil {
			sourceBytes, _ := json.Marshal(*filter.Source)
			sql += fmt.Sprintf(" AND source = $%d", argIdx)
			args = append(args, string(sourceBytes))
			argIdx++
		}
		if filter.Since != nil {
			sql += fmt.Sprintf(" AND created_at >= $%d", argIdx)
			args = append(args, filter.Since.Format(time.RFC3339))
			argIdx++
		}
	}

	// Order by similarity (smaller distance = more similar)
	sql += fmt.Sprintf(" ORDER BY embedding <=> $1::vector LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := p.db.db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	var fragments []types.MemoryFragment
	for rows.Next() {
		var frag types.MemoryFragment
		var idStr, agentIDStr, sourceStr, metaStr, createdAt, accessedAt string
		var distance float64

		err := rows.Scan(
			&idStr, &agentIDStr, &frag.Content, &sourceStr, &frag.Scope,
			&frag.Confidence, &metaStr, &createdAt, &accessedAt,
			&frag.AccessCount, &distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		frag.ID, _ = types.ParseMemoryID(idStr)
		frag.AgentID, _ = types.ParseAgentID(agentIDStr)

		var source types.MemorySource
		json.Unmarshal([]byte(sourceStr), &source)
		frag.Source = source

		json.Unmarshal([]byte(metaStr), &frag.Metadata)

		frag.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		frag.AccessedAt, _ = time.Parse(time.RFC3339, accessedAt)

		// Convert distance to similarity score (1 - distance for cosine)
		// Note: Score is not part of MemoryFragment, but could be added to Metadata

		fragments = append(fragments, frag)
	}

	return fragments, nil
}

// textSearch performs text-based search.
func (p *PostgresSemanticStore) textSearch(
	query string,
	limit int,
	filter *types.MemoryFilter,
) ([]types.MemoryFragment, error) {
	sql := `
		SELECT id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count
		FROM memories
		WHERE deleted = false
	`
	var args []interface{}
	argIdx := 1

	// Text search using ILIKE for case-insensitive matching
	if query != "" {
		sql += fmt.Sprintf(" AND content ILIKE $%d", argIdx)
		args = append(args, "%"+query+"%")
		argIdx++
	}

	// Apply filters
	if filter != nil {
		if filter.AgentID != nil {
			sql += fmt.Sprintf(" AND agent_id = $%d", argIdx)
			args = append(args, filter.AgentID.String())
			argIdx++
		}
		if filter.Scope != nil {
			sql += fmt.Sprintf(" AND scope = $%d", argIdx)
			args = append(args, *filter.Scope)
			argIdx++
		}
		if filter.MinConfidence != nil {
			sql += fmt.Sprintf(" AND confidence >= $%d", argIdx)
			args = append(args, *filter.MinConfidence)
			argIdx++
		}
		if filter.Source != nil {
			sourceBytes, _ := json.Marshal(*filter.Source)
			sql += fmt.Sprintf(" AND source = $%d", argIdx)
			args = append(args, string(sourceBytes))
			argIdx++
		}
		if filter.Since != nil {
			sql += fmt.Sprintf(" AND created_at >= $%d", argIdx)
			args = append(args, filter.Since.Format(time.RFC3339))
			argIdx++
		}
	}

	sql += " ORDER BY created_at DESC"
	sql += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := p.db.db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	var fragments []types.MemoryFragment
	for rows.Next() {
		var frag types.MemoryFragment
		var idStr, agentIDStr, sourceStr, metaStr, createdAt, accessedAt string

		err := rows.Scan(
			&idStr, &agentIDStr, &frag.Content, &sourceStr, &frag.Scope,
			&frag.Confidence, &metaStr, &createdAt, &accessedAt,
			&frag.AccessCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		frag.ID, _ = types.ParseMemoryID(idStr)
		frag.AgentID, _ = types.ParseAgentID(agentIDStr)

		var source types.MemorySource
		json.Unmarshal([]byte(sourceStr), &source)
		frag.Source = source

		json.Unmarshal([]byte(metaStr), &frag.Metadata)

		frag.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		frag.AccessedAt, _ = time.Parse(time.RFC3339, accessedAt)

		fragments = append(fragments, frag)
	}

	return fragments, nil
}

// Forget marks a memory as deleted.
func (p *PostgresSemanticStore) Forget(id types.MemoryID) error {
	_, err := p.db.db.Exec("UPDATE memories SET deleted = true WHERE id = $1", id.String())
	return err
}

// StoreEmbedding stores an embedding with metadata.
func (p *PostgresSemanticStore) StoreEmbedding(
	id string,
	embedding []float32,
	metadata json.RawMessage,
) error {
	vecStr := vectorToPostgresArray(embedding)
	_, err := p.db.db.Exec(`
		INSERT INTO memories (id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count, deleted, embedding)
		VALUES ($1, '', '{}', 'vector', 1.0, $2, NOW(), NOW(), 0, false, $3::vector)
		ON CONFLICT (id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata
	`, id, string(metadata), vecStr)
	return err
}

// SearchSimilar searches for similar embeddings.
func (p *PostgresSemanticStore) SearchSimilar(
	embedding []float32,
	limit int,
	filter *types.MemoryFilter,
) ([]SearchResult, error) {
	vecStr := vectorToPostgresArray(embedding)

	sql := `
		SELECT id, metadata, embedding <=> $1::vector as distance
		FROM memories
		WHERE deleted = false AND embedding IS NOT NULL
	`
	args := []interface{}{vecStr}
	argIdx := 2

	if filter != nil && filter.AgentID != nil {
		sql += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, filter.AgentID.String())
		argIdx++
	}

	sql += fmt.Sprintf(" ORDER BY embedding <=> $1::vector LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := p.db.db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var distance float64
		if err := rows.Scan(&r.ID, &r.Metadata, &distance); err != nil {
			return nil, err
		}
		r.Distance = distance
		r.Score = 1.0 - distance // Convert distance to similarity
		results = append(results, r)
	}

	return results, nil
}

// DeleteEmbedding deletes an embedding.
func (p *PostgresSemanticStore) DeleteEmbedding(id string) error {
	_, err := p.db.db.Exec("DELETE FROM memories WHERE id = $1", id)
	return err
}

// DistanceMetric returns the distance metric used.
func (p *PostgresSemanticStore) DistanceMetric() string {
	return "cosine"
}

// Note: vectorToPostgresArray is defined in db_postgres.go

// Ensure PostgresSemanticStore implements interfaces
var _ SemanticStorage = (*PostgresSemanticStore)(nil)
var _ VectorStorage = (*PostgresSemanticStore)(nil)
