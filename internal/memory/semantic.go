// Package memory implements the memory substrate for OpenFang.
package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// SemanticStore implements semantic memory with vector embedding support.
type SemanticStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSemanticStore creates a new semantic memory store.
func NewSemanticStore(dbPath string) (*SemanticStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SemanticStore{
		db: db,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return store, nil
}

// initSchema initializes the database schema.
func (s *SemanticStore) initSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
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
		);
		CREATE INDEX IF NOT EXISTS idx_memories_agent_id ON memories(agent_id);
		CREATE INDEX IF NOT EXISTS idx_memories_source ON memories(source);
		CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope);
		CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
		CREATE INDEX IF NOT EXISTS idx_memories_deleted ON memories(deleted);
	`)
	return err
}

// Remember stores a new memory fragment.
func (s *SemanticStore) Remember(
	agentID types.AgentID,
	content string,
	source types.MemorySource,
	scope string,
	metadata map[string]interface{},
) (types.MemoryID, error) {
	return s.RememberWithEmbedding(agentID, content, source, scope, metadata, nil)
}

// RememberWithEmbedding stores a memory with an optional embedding.
func (s *SemanticStore) RememberWithEmbedding(
	agentID types.AgentID,
	content string,
	source types.MemorySource,
	scope string,
	metadata map[string]interface{},
	embedding []float32,
) (types.MemoryID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := types.NewMemoryID()
	now := time.Now().UTC().Format(time.RFC3339)

	sourceBytes, err := json.Marshal(source)
	if err != nil {
		return types.MemoryID{}, fmt.Errorf("failed to marshal source: %w", err)
	}

	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return types.MemoryID{}, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var embeddingBytes []byte
	if embedding != nil {
		embeddingBytes, err = json.Marshal(embedding)
		if err != nil {
			return types.MemoryID{}, fmt.Errorf("failed to marshal embedding: %w", err)
		}
	}

	_, err = s.db.Exec(`
		INSERT INTO memories (id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count, deleted, embedding)
		VALUES (?, ?, ?, ?, ?, 1.0, ?, ?, ?, 0, 0, ?)
	`, id.String(), agentID.String(), content, string(sourceBytes), scope, string(metaBytes), now, now, embeddingBytes)

	if err != nil {
		return types.MemoryID{}, fmt.Errorf("failed to insert memory: %w", err)
	}

	return id, nil
}

// Recall searches for memories using text matching.
func (s *SemanticStore) Recall(
	query string,
	limit int,
	filter *types.MemoryFilter,
) ([]types.MemoryFragment, error) {
	return s.RecallWithEmbedding(query, limit, filter, nil)
}

// RecallWithEmbedding searches for memories using vector similarity or text matching.
func (s *SemanticStore) RecallWithEmbedding(
	query string,
	limit int,
	filter *types.MemoryFilter,
	queryEmbedding []float32,
) ([]types.MemoryFragment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fetchLimit := limit
	if queryEmbedding != nil {
		fetchLimit = (limit * 10)
		if fetchLimit < 100 {
			fetchLimit = 100
		}
	}

	sql := `
		SELECT id, agent_id, content, source, scope, confidence, metadata, created_at, accessed_at, access_count, embedding
		FROM memories WHERE deleted = 0
	`
	var args []interface{}
	paramIdx := 1

	if queryEmbedding == nil && query != "" {
		sql += fmt.Sprintf(" AND content LIKE ?%d", paramIdx)
		args = append(args, "%"+query+"%")
		paramIdx++
	}

	if filter != nil {
		if filter.AgentID != nil {
			sql += fmt.Sprintf(" AND agent_id = ?%d", paramIdx)
			args = append(args, filter.AgentID.String())
			paramIdx++
		}
		if filter.Scope != nil {
			sql += fmt.Sprintf(" AND scope = ?%d", paramIdx)
			args = append(args, *filter.Scope)
			paramIdx++
		}
		if filter.MinConfidence != nil {
			sql += fmt.Sprintf(" AND confidence >= ?%d", paramIdx)
			args = append(args, *filter.MinConfidence)
			paramIdx++
		}
		if filter.Source != nil {
			sql += fmt.Sprintf(" AND source = ?%d", paramIdx)
			sourceBytes, _ := json.Marshal(*filter.Source)
			args = append(args, string(sourceBytes))
			paramIdx++
		}
		if filter.Since != nil {
			sql += fmt.Sprintf(" AND created_at >= ?%d", paramIdx)
			args = append(args, filter.Since.Format(time.RFC3339))
			paramIdx++
		}
	}

	sql += " ORDER BY created_at DESC"
	sql += fmt.Sprintf(" LIMIT ?%d", paramIdx)
	args = append(args, fetchLimit)

	rows, err := s.db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	var fragments []types.MemoryFragment
	for rows.Next() {
		var frag types.MemoryFragment
		var idStr, agentIDStr, sourceStr, metaStr, createdAt, accessedAt string
		var embeddingBytes []byte

		err := rows.Scan(
			&idStr, &agentIDStr, &frag.Content, &sourceStr, &frag.Scope,
			&frag.Confidence, &metaStr, &createdAt, &accessedAt,
			&frag.AccessCount, &embeddingBytes,
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

		if embeddingBytes != nil {
			json.Unmarshal(embeddingBytes, &frag.Embedding)
		}

		fragments = append(fragments, frag)
	}

	if queryEmbedding != nil && len(fragments) > 0 {
		fragments = s.reRankBySimilarity(fragments, queryEmbedding, limit)
	} else if len(fragments) > limit {
		fragments = fragments[:limit]
	}

	return fragments, nil
}

// reRankBySimilarity re-ranks memories by cosine similarity.
func (s *SemanticStore) reRankBySimilarity(
	fragments []types.MemoryFragment,
	queryEmbedding []float32,
	limit int,
) []types.MemoryFragment {
	type scoredFragment struct {
		fragment types.MemoryFragment
		score    float64
	}

	var scored []scoredFragment
	for _, frag := range fragments {
		score := 0.0
		if len(frag.Embedding) == len(queryEmbedding) {
			score = cosineSimilarity(queryEmbedding, frag.Embedding)
		}
		scored = append(scored, scoredFragment{fragment: frag, score: score})
	}

	for i := range scored {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].score < scored[j].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	result := make([]types.MemoryFragment, 0, limit)
	for i := 0; i < limit && i < len(scored); i++ {
		result = append(result, scored[i].fragment)
	}
	return result
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Forget marks a memory as deleted.
func (s *SemanticStore) Forget(id types.MemoryID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE memories SET deleted = 1 WHERE id = ?", id.String())
	return err
}

// Close closes the database.
func (s *SemanticStore) Close() error {
	return s.db.Close()
}
