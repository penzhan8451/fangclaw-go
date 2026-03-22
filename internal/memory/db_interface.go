// Package memory provides storage interface abstraction for FangClaw.
package memory

import (
	"encoding/json"
	"fmt"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// Storage defines the unified data storage interface.
type Storage interface {
	// Connection management
	Close() error

	// Agent operations
	SaveAgent(agent *AgentRecord) error
	GetAgent(id string) (*AgentRecord, error)
	ListAgents() ([]*AgentRecord, error)
	DeleteAgent(id string) error

	// Session operations
	SaveSession(session *SessionRecord) error
	GetSession(id string) (*SessionRecord, error)
	ListAllSessions() ([]*SessionRecord, error)
	DeleteSession(id string) error

	// Memory operations (legacy KV)
	SetMemory(agentID, key, value string) error
	GetMemory(agentID, key string) (*MemoryRecord, error)
	ListMemory(agentID string) ([]*MemoryRecord, error)
	DeleteMemory(agentID, key string) error

	// KV store operations (binary values)
	SetKV(agentID, key string, value []byte) error
	GetKV(agentID, key string) (*KVRecord, error)
	ListKV(agentID string) ([]*KVRecord, error)
	DeleteKV(agentID, key string) error

	// Trigger operations
	SaveTrigger(trigger *TriggerRecord) error
	GetTrigger(id string) (*TriggerRecord, error)
	ListTriggers() ([]*TriggerRecord, error)
	DeleteTrigger(id string) error

	// Trigger history
	SaveTriggerHistory(record *TriggerHistoryRecord) error
	ListTriggerHistory(triggerID, agentID string, limit int) ([]*TriggerHistoryRecord, error)

	// Workflow operations
	SaveWorkflow(workflow *WorkflowRecord) error
	GetWorkflow(id string) (*WorkflowRecord, error)
	ListWorkflows() ([]*WorkflowRecord, error)
	DeleteWorkflow(id string) error

	// Cron job operations
	SaveCronJob(job *CronJobRecord) error
	GetCronJob(id string) (*CronJobRecord, error)
	ListCronJobs() ([]*CronJobRecord, error)
	DeleteCronJob(id string) error

	// Audit operations
	AddAudit(action, agentID, details string) error
	ListAudit(limit int) ([]*AuditRecord, error)

	// Migration
	Migrate() error
}

// VectorStorage defines vector storage capabilities (optional).
type VectorStorage interface {
	// Store embedding with metadata
	StoreEmbedding(id string, embedding []float32, metadata json.RawMessage) error

	// Search similar embeddings using cosine similarity
	SearchSimilar(embedding []float32, limit int, filter *types.MemoryFilter) ([]SearchResult, error)

	// Delete embedding
	DeleteEmbedding(id string) error

	// Distance metric used
	DistanceMetric() string
}

// SemanticStorage defines semantic memory operations.
type SemanticStorage interface {
	// Remember stores a new memory fragment
	Remember(agentID types.AgentID, content string, source types.MemorySource, scope string, metadata map[string]interface{}) (types.MemoryID, error)

	// RememberWithEmbedding stores a memory with embedding
	RememberWithEmbedding(agentID types.AgentID, content string, source types.MemorySource, scope string, metadata map[string]interface{}, embedding []float32) (types.MemoryID, error)

	// Recall searches memories
	Recall(query string, limit int, filter *types.MemoryFilter) ([]types.MemoryFragment, error)

	// RecallWithEmbedding searches with embedding
	RecallWithEmbedding(query string, limit int, filter *types.MemoryFilter, queryEmbedding []float32) ([]types.MemoryFragment, error)

	// Forget marks memory as deleted
	Forget(id types.MemoryID) error
}

// SearchResult represents a vector search result.
type SearchResult struct {
	ID       string
	Score    float64
	Metadata json.RawMessage
	Distance float64
}

// StorageType defines supported storage backends.
type StorageType string

const (
	StorageSQLite   StorageType = "sqlite"
	StoragePostgres StorageType = "postgres"
)

// StorageConfig holds storage configuration.
type StorageConfig struct {
	Type StorageType

	// SQLite configuration
	SQLitePath string

	// PostgreSQL configuration
	Postgres PostgresConfig
}

// PostgresConfig holds PostgreSQL connection parameters.
type PostgresConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string // disable, require, verify-ca, verify-full
}

// NewStorage creates a storage instance based on configuration.
func NewStorage(config StorageConfig) (Storage, error) {
	switch config.Type {
	case StorageSQLite:
		return NewDB(config.SQLitePath)
	case StoragePostgres:
		return NewPostgresDB(config.Postgres)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", config.Type)
	}
}

// NewSemanticStorage creates a semantic storage instance.
func NewSemanticStorage(storage Storage) (SemanticStorage, error) {
	switch s := storage.(type) {
	case *DB:
		// SQLite uses existing semantic store
		return NewSemanticStore(s.Path)
	case *PostgresDB:
		return NewPostgresSemanticStore(s)
	default:
		return nil, fmt.Errorf("unsupported storage type for semantic operations")
	}
}
