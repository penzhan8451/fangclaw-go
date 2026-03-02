package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EntityType represents the type of an entity.
type EntityType string

const (
	EntityTypePerson       EntityType = "person"
	EntityTypePlace        EntityType = "place"
	EntityTypeThing        EntityType = "thing"
	EntityTypeConcept      EntityType = "concept"
	EntityTypeEvent        EntityType = "event"
	EntityTypeOrganization EntityType = "organization"
	EntityTypeCustom       EntityType = "custom"
)

// Entity represents an entity in the knowledge graph.
type Entity struct {
	ID         string                 `json:"id"`
	EntityType EntityType             `json:"entity_type"`
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// RelationType represents the type of a relation.
type RelationType string

const (
	RelationTypeIsA       RelationType = "is_a"
	RelationTypeHas       RelationType = "has"
	RelationTypeLocatedIn RelationType = "located_in"
	RelationTypeWorksFor  RelationType = "works_for"
	RelationTypeKnows     RelationType = "knows"
	RelationTypeLikes     RelationType = "likes"
	RelationTypeCreated   RelationType = "created"
	RelationTypePartOf    RelationType = "part_of"
	RelationTypeRelatedTo RelationType = "related_to"
	RelationTypeCustom    RelationType = "custom"
)

// Relation represents a relation between two entities.
type Relation struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`
	Relation   RelationType           `json:"relation"`
	Target     string                 `json:"target"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Confidence float64                `json:"confidence"`
	CreatedAt  time.Time              `json:"created_at"`
}

// GraphPattern represents a pattern for querying the knowledge graph.
type GraphPattern struct {
	Source   *string       `json:"source,omitempty"`
	Relation *RelationType `json:"relation,omitempty"`
	Target   *string       `json:"target,omitempty"`
}

// GraphMatch represents a matched graph pattern.
type GraphMatch struct {
	SourceEntity Entity   `json:"source_entity"`
	Relation     Relation `json:"relation"`
	TargetEntity Entity   `json:"target_entity"`
}

// KnowledgeStore provides knowledge graph storage and query capabilities.
type KnowledgeStore struct {
	db *DB
}

// NewKnowledgeStore creates a new knowledge store.
func NewKnowledgeStore(db *DB) *KnowledgeStore {
	return &KnowledgeStore{db: db}
}

// Init initializes the knowledge store tables.
func (k *KnowledgeStore) Init() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS entities (
			id TEXT PRIMARY KEY,
			entity_type TEXT NOT NULL,
			name TEXT NOT NULL,
			properties TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS relations (
			id TEXT PRIMARY KEY,
			source_entity TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			target_entity TEXT NOT NULL,
			properties TEXT,
			confidence REAL NOT NULL DEFAULT 1.0,
			created_at TEXT NOT NULL,
			FOREIGN KEY (source_entity) REFERENCES entities(id) ON DELETE CASCADE,
			FOREIGN KEY (target_entity) REFERENCES entities(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name)`,
		`CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(entity_type)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_source ON relations(source_entity)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_target ON relations(target_entity)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_type ON relations(relation_type)`,
	}

	for _, m := range migrations {
		if _, err := k.db.Exec(m); err != nil {
			return fmt.Errorf("knowledge store migration failed: %w", err)
		}
	}

	return nil
}

// AddEntity adds an entity to the knowledge graph.
func (k *KnowledgeStore) AddEntity(entity Entity) (string, error) {
	if entity.ID == "" {
		entity.ID = uuid.NewString()
	}

	propsStr, err := json.Marshal(entity.Properties)
	if err != nil {
		return "", fmt.Errorf("failed to marshal properties: %w", err)
	}

	now := time.Now()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	entity.UpdatedAt = now

	_, err = k.db.Exec(`
		INSERT OR REPLACE INTO entities (id, entity_type, name, properties, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, entity.ID, string(entity.EntityType), entity.Name, string(propsStr),
		entity.CreatedAt.Format(time.RFC3339), entity.UpdatedAt.Format(time.RFC3339))

	if err != nil {
		return "", fmt.Errorf("failed to add entity: %w", err)
	}

	return entity.ID, nil
}

// GetEntity retrieves an entity by ID.
func (k *KnowledgeStore) GetEntity(id string) (*Entity, error) {
	var entity Entity
	var propsStr, createdAtStr, updatedAtStr string

	err := k.db.QueryRow(`
		SELECT id, entity_type, name, properties, created_at, updated_at
		FROM entities WHERE id = ?
	`, id).Scan(&entity.ID, &entity.EntityType, &entity.Name, &propsStr,
		&createdAtStr, &updatedAtStr)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	entity.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	entity.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)

	if propsStr != "" {
		json.Unmarshal([]byte(propsStr), &entity.Properties)
	}

	return &entity, nil
}

// AddRelation adds a relation between two entities.
func (k *KnowledgeStore) AddRelation(relation Relation) (string, error) {
	if relation.ID == "" {
		relation.ID = uuid.NewString()
	}

	propsStr, err := json.Marshal(relation.Properties)
	if err != nil {
		return "", fmt.Errorf("failed to marshal properties: %w", err)
	}

	if relation.CreatedAt.IsZero() {
		relation.CreatedAt = time.Now()
	}

	_, err = k.db.Exec(`
		INSERT INTO relations (id, source_entity, relation_type, target_entity, properties, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, relation.ID, relation.Source, string(relation.Relation), relation.Target, string(propsStr),
		relation.Confidence, relation.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return "", fmt.Errorf("failed to add relation: %w", err)
	}

	return relation.ID, nil
}

// QueryGraph queries the knowledge graph with a pattern.
func (k *KnowledgeStore) QueryGraph(pattern GraphPattern) ([]GraphMatch, error) {
	var args []interface{}
	whereClauses := []string{"1=1"}

	if pattern.Source != nil {
		whereClauses = append(whereClauses, "(s.id = ? OR s.name = ?)")
		args = append(args, *pattern.Source, *pattern.Source)
	}

	if pattern.Relation != nil {
		whereClauses = append(whereClauses, "r.relation_type = ?")
		args = append(args, string(*pattern.Relation))
	}

	if pattern.Target != nil {
		whereClauses = append(whereClauses, "(t.id = ? OR t.name = ?)")
		args = append(args, *pattern.Target, *pattern.Target)
	}

	whereStr := "WHERE " + strings.Join(whereClauses, " AND ")

	query := fmt.Sprintf(`
		SELECT
			s.id, s.entity_type, s.name, s.properties, s.created_at, s.updated_at,
			r.id, r.source_entity, r.relation_type, r.target_entity, r.properties, r.confidence, r.created_at,
			t.id, t.entity_type, t.name, t.properties, t.created_at, t.updated_at
		FROM relations r
		JOIN entities s ON r.source_entity = s.id
		JOIN entities t ON r.target_entity = t.id
		%s
		LIMIT 100
	`, whereStr)

	rows, err := k.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query graph: %w", err)
	}
	defer rows.Close()

	var matches []GraphMatch
	for rows.Next() {
		var sID, sType, sName, sProps, sCreated, sUpdated string
		var rID, rSource, rType, rTarget, rProps, rCreated string
		var rConfidence float64
		var tID, tType, tName, tProps, tCreated, tUpdated string

		err := rows.Scan(
			&sID, &sType, &sName, &sProps, &sCreated, &sUpdated,
			&rID, &rSource, &rType, &rTarget, &rProps, &rConfidence, &rCreated,
			&tID, &tType, &tName, &tProps, &tCreated, &tUpdated,
		)
		if err != nil {
			return nil, err
		}

		sEntity := Entity{
			ID:         sID,
			EntityType: EntityType(sType),
			Name:       sName,
		}
		sEntity.CreatedAt, _ = time.Parse(time.RFC3339, sCreated)
		sEntity.UpdatedAt, _ = time.Parse(time.RFC3339, sUpdated)
		if sProps != "" {
			json.Unmarshal([]byte(sProps), &sEntity.Properties)
		}

		rel := Relation{
			ID:         rID,
			Source:     rSource,
			Relation:   RelationType(rType),
			Target:     rTarget,
			Confidence: rConfidence,
		}
		rel.CreatedAt, _ = time.Parse(time.RFC3339, rCreated)
		if rProps != "" {
			json.Unmarshal([]byte(rProps), &rel.Properties)
		}

		tEntity := Entity{
			ID:         tID,
			EntityType: EntityType(tType),
			Name:       tName,
		}
		tEntity.CreatedAt, _ = time.Parse(time.RFC3339, tCreated)
		tEntity.UpdatedAt, _ = time.Parse(time.RFC3339, tUpdated)
		if tProps != "" {
			json.Unmarshal([]byte(tProps), &tEntity.Properties)
		}

		matches = append(matches, GraphMatch{
			SourceEntity: sEntity,
			Relation:     rel,
			TargetEntity: tEntity,
		})
	}

	return matches, nil
}

// DeleteEntity deletes an entity and all its relations.
func (k *KnowledgeStore) DeleteEntity(id string) error {
	_, err := k.db.Exec("DELETE FROM entities WHERE id = ?", id)
	return err
}

// DeleteRelation deletes a relation.
func (k *KnowledgeStore) DeleteRelation(id string) error {
	_, err := k.db.Exec("DELETE FROM relations WHERE id = ?", id)
	return err
}

// ListEntities lists all entities.
func (k *KnowledgeStore) ListEntities() ([]Entity, error) {
	rows, err := k.db.Query(`
		SELECT id, entity_type, name, properties, created_at, updated_at
		FROM entities ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var entity Entity
		var propsStr, createdAtStr, updatedAtStr string
		if err := rows.Scan(&entity.ID, &entity.EntityType, &entity.Name,
			&propsStr, &createdAtStr, &updatedAtStr); err != nil {
			return nil, err
		}
		entity.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		entity.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		if propsStr != "" {
			json.Unmarshal([]byte(propsStr), &entity.Properties)
		}
		entities = append(entities, entity)
	}
	return entities, nil
}
