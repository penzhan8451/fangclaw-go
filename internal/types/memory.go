// Package types provides core data structures for OpenFang.
package types

import (
	"time"

	"github.com/google/uuid"
)

// MemoryID is a unique identifier for a memory.
type MemoryID = uuid.UUID

// NewMemoryID creates a new memory ID.
func NewMemoryID() MemoryID {
	return uuid.New()
}

// ParseMemoryID parses a string into a MemoryID.
func ParseMemoryID(s string) (MemoryID, error) {
	return uuid.Parse(s)
}

// MemorySource represents the source of a memory.
type MemorySource string

const (
	MemorySourceUser         MemorySource = "user"
	MemorySourceSystem       MemorySource = "system"
	MemorySourceAgent        MemorySource = "agent"
	MemorySourceTool         MemorySource = "tool"
	MemorySourceWeb          MemorySource = "web"
	MemorySourceSkill        MemorySource = "skill"
	MemorySourceImported     MemorySource = "imported"
	MemorySourceConversation MemorySource = "conversation"
)

// MemoryFilter filters memory queries.
type MemoryFilter struct {
	AgentID       *AgentID      `json:"agent_id,omitempty"`
	Scope         *string       `json:"scope,omitempty"`
	MinConfidence *float64      `json:"min_confidence,omitempty"`
	Source        *MemorySource `json:"source,omitempty"`
	Since         *time.Time    `json:"since,omitempty"`
	Until         *time.Time    `json:"until,omitempty"`
}

// MemoryFragment represents a single memory fragment.
type MemoryFragment struct {
	ID          MemoryID               `json:"id"`
	AgentID     AgentID                `json:"agent_id"`
	Content     string                 `json:"content"`
	Source      MemorySource           `json:"source"`
	Scope       string                 `json:"scope"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	AccessedAt  time.Time              `json:"accessed_at"`
	AccessCount int                    `json:"access_count"`
	Embedding   []float32              `json:"embedding,omitempty"`
}

// Memory represents a memory entry.
type Memory struct {
	Fragment MemoryFragment `json:"fragment"`
	Score    float64        `json:"score,omitempty"`
}

// Entity represents an entity in the knowledge graph.
type Entity struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// Relation represents a relation between two entities in the knowledge graph.
type Relation struct {
	ID         string                 `json:"id"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// GraphQuery represents a query for the knowledge graph.
type GraphQuery struct {
	StartEntity *string `json:"start_entity,omitempty"`
	EndEntity   *string `json:"end_entity,omitempty"`
	Relation    *string `json:"relation,omitempty"`
	EntityType  *string `json:"entity_type,omitempty"`
	Limit       int     `json:"limit"`
}

// ConsolidationReport represents the result of a memory consolidation cycle.
type ConsolidationReport struct {
	MemoriesMerged  uint64 `json:"memories_merged"`
	MemoriesDecayed uint64 `json:"memories_decayed"`
	DurationMS      uint64 `json:"duration_ms"`
}
