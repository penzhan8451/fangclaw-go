package memory

import (
	"fmt"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// ConsolidationEngine handles memory consolidation and decay.
type ConsolidationEngine struct {
	db         *DB
	decayRate  float64
	cutoffDays int
}

// NewConsolidationEngine creates a new consolidation engine.
func NewConsolidationEngine(db *DB) *ConsolidationEngine {
	return &ConsolidationEngine{
		db:         db,
		decayRate:  0.1,
		cutoffDays: 7,
	}
}

// SetDecayRate sets the decay rate for memories.
func (e *ConsolidationEngine) SetDecayRate(rate float64) {
	if rate >= 0 && rate <= 1 {
		e.decayRate = rate
	}
}

// SetCutoffDays sets the cutoff days for memory decay.
func (e *ConsolidationEngine) SetCutoffDays(days int) {
	if days > 0 {
		e.cutoffDays = days
	}
}

// Consolidate runs a memory consolidation cycle.
func (e *ConsolidationEngine) Consolidate() (*types.ConsolidationReport, error) {
	start := time.Now()

	cutoff := time.Now().AddDate(0, 0, -e.cutoffDays).Format(time.RFC3339)
	decayFactor := 1.0 - e.decayRate

	result, err := e.db.Exec(`
		UPDATE memories 
		SET confidence = MAX(0.1, confidence * ?)
		WHERE deleted = 0 
		  AND accessed_at < ? 
		  AND confidence > 0.1
	`, decayFactor, cutoff)

	if err != nil {
		return nil, fmt.Errorf("failed to decay memories: %w", err)
	}

	memoriesDecayed, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	duration := time.Since(start)

	return &types.ConsolidationReport{
		MemoriesMerged: 0,
		MemoriesDecayed: uint64(memoriesDecayed),
		DurationMS:     uint64(duration.Milliseconds()),
	}, nil
}

// MergeSimilarMemories merges similar memories (placeholder for future implementation).
func (e *ConsolidationEngine) MergeSimilarMemories() (uint64, error) {
	return 0, nil
}

// CleanupDeletedMemories permanently removes soft-deleted memories.
func (e *ConsolidationEngine) CleanupDeletedMemories(before time.Time) (uint64, error) {
	result, err := e.db.Exec(`
		DELETE FROM memories 
		WHERE deleted = 1 
		  AND updated_at < ?
	`, before.Format(time.RFC3339))

	if err != nil {
		return 0, fmt.Errorf("failed to cleanup deleted memories: %w", err)
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows deleted: %w", err)
	}

	return uint64(rowsDeleted), nil
}
