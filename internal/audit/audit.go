// Package audit provides Merkle hash-chain audit logging.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Action represents an audit action type.
type Action string

const (
	ActionAgentSpawn         Action = "agent.spawn"
	ActionAgentKill          Action = "agent.kill"
	ActionAgentMessage       Action = "agent.message"
	ActionToolCall           Action = "tool.call"
	ActionConfigChange       Action = "config.change"
	ActionAuthAttempt        Action = "auth.attempt"
	ActionAPIKeyUse          Action = "apikey.use"
	ActionChannelSend        Action = "channel.send"
	ActionBudgetAlert        Action = "budget.alert"
	ActionApproval           Action = "approval"
	ActionShutdown           Action = "system.shutdown"
	ActionLogin              Action = "auth.login"
	ActionLogout             Action = "auth.logout"
	ActionA2ATaskCreated     Action = "a2a.task.created"
	ActionA2ATaskUpdated     Action = "a2a.task.updated"
	ActionA2ATaskCompleted   Action = "a2a.task.completed"
	ActionA2ATaskFailed      Action = "a2a.task.failed"
	ActionA2ATaskCancelled   Action = "a2a.task.cancelled"
	ActionA2AAgentDiscovered Action = "a2a.agent.discovered"
	ActionA2ATaskSent        Action = "a2a.task.sent"
)

// Entry represents a single audit log entry.
type Entry struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Action    Action          `json:"action"`
	Actor     string          `json:"actor"`
	Target    string          `json:"target"`
	Details   string          `json:"details"`
	Result    string          `json:"result"`
	PrevHash  string          `json:"prev_hash"`
	Hash      string          `json:"hash"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// AuditLog provides tamper-evident audit logging using a Merkle hash chain.
type AuditLog struct {
	mu      sync.RWMutex
	entries []*Entry
	chain   []byte // Current chain hash
}

// NewAuditLog creates a new audit log.
func NewAuditLog() *AuditLog {
	return &AuditLog{
		entries: make([]*Entry, 0),
		chain:   []byte{},
	}
}

// Record records an audit event.
func (al *AuditLog) Record(actor, target string, action Action, details, result string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	entry := &Entry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Action:    action,
		Actor:     actor,
		Target:    target,
		Details:   details,
		Result:    result,
		PrevHash:  hex.EncodeToString(al.chain),
	}

	// Compute hash
	entry.Hash = al.computeHash(entry)

	// Update chain
	al.chain = []byte(entry.Hash)

	al.entries = append(al.entries, entry)
}

// GetEntries returns all audit entries, optionally filtered.
func (al *AuditLog) GetEntries(limit int, action Action) []*Entry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	entries := al.entries
	if action != "" {
		var filtered []*Entry
		for _, e := range entries {
			if e.Action == action {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if limit > 0 && limit < len(entries) {
		return entries[len(entries)-limit:]
	}

	return entries
}

// Verify checks the integrity of the audit log.
func (al *AuditLog) Verify() (bool, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var prevHash string

	for i, entry := range al.entries {
		// Verify previous hash link
		if entry.PrevHash != prevHash {
			return false, fmt.Errorf("chain broken at entry %d", i)
		}

		// Verify entry hash
		expectedHash := al.computeHash(entry)
		if entry.Hash != expectedHash {
			return false, fmt.Errorf("hash mismatch at entry %d", i)
		}

		prevHash = entry.Hash
	}

	return true, nil
}

// GetChainHash returns the current chain hash.
func (al *AuditLog) GetChainHash() string {
	al.mu.RLock()
	defer al.mu.RUnlock()

	return hex.EncodeToString(al.chain)
}

// Count returns the number of entries.
func (al *AuditLog) Count() int {
	al.mu.RLock()
	defer al.mu.RUnlock()

	return len(al.entries)
}

// computeHash computes the hash of an entry.
func (al *AuditLog) computeHash(entry *Entry) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		entry.ID,
		entry.Timestamp.Format(time.RFC3339Nano),
		string(entry.Action),
		entry.Actor,
		entry.Target,
		entry.Details,
		entry.Result,
		entry.PrevHash,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SearchResult represents a search result.
type SearchResult struct {
	Entries []*Entry `json:"entries"`
	Total   int      `json:"total"`
}

// Search searches audit logs with filters.
func (al *AuditLog) Search(query string, limit int) *SearchResult {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var results []*Entry

	for _, e := range al.entries {
		if contains(e.Actor, query) || contains(e.Target, query) ||
			contains(e.Details, query) || contains(e.Result, query) {
			results = append(results, e)
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[len(results)-limit:]
	}

	return &SearchResult{
		Entries: results,
		Total:   len(results),
	}
}

// FilterByTimeRange filters entries by time range.
func (al *AuditLog) FilterByTimeRange(start, end time.Time) []*Entry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var results []*Entry

	for _, e := range al.entries {
		if (start.IsZero() || e.Timestamp.After(start)) &&
			(end.IsZero() || e.Timestamp.Before(end)) {
			results = append(results, e)
		}
	}

	return results
}

// FilterByActor filters entries by actor.
func (al *AuditLog) FilterByActor(actor string) []*Entry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var results []*Entry

	for _, e := range al.entries {
		if e.Actor == actor {
			results = append(results, e)
		}
	}

	return results
}

// GetRecent returns the most recent entries.
func (al *AuditLog) GetRecent(count int) []*Entry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	if count > len(al.entries) {
		count = len(al.entries)
	}

	start := len(al.entries) - count
	return al.entries[start:]
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MarshalJSON implements custom JSON marshaling.
func (e *Entry) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        e.ID,
		"timestamp": e.Timestamp.Format(time.RFC3339),
		"action":    e.Action,
		"actor":     e.Actor,
		"target":    e.Target,
		"details":   e.Details,
		"result":    e.Result,
		"prev_hash": e.PrevHash,
		"hash":      e.Hash,
	})
}
