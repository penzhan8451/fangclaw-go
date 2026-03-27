package delivery

import (
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// DeliveryStatus represents the outcome of a delivery attempt.
type DeliveryStatus string

const (
	// Outbound delivery statuses (used by DeliveryReceipt / DeliveryTracker)
	DeliveryStatusSent       DeliveryStatus = "sent"
	DeliveryStatusDelivered  DeliveryStatus = "delivered"
	DeliveryStatusFailed     DeliveryStatus = "failed"
	DeliveryStatusBestEffort DeliveryStatus = "best_effort"

	// Task lifecycle statuses (used by DeliveryRegistry / DeliveryEntry)
	DeliveryStatusPending    DeliveryStatus = "pending"
	DeliveryStatusInProgress DeliveryStatus = "in_progress"
	DeliveryStatusDone       DeliveryStatus = "done"
)

// DeliveryReceipt tracks a single outbound message delivery attempt.
type DeliveryReceipt struct {
	MessageID   string         `json:"message_id"`
	AgentID     string         `json:"agent_id"`
	Channel     string         `json:"channel"`
	Recipient   string         `json:"recipient"` // sanitized, no PII
	Status      DeliveryStatus `json:"status"`
	DeliveredAt time.Time      `json:"delivered_at"`
	Error       *string        `json:"error,omitempty"` // sanitized, only on failure
}

const (
	defaultMaxPerAgent = 500
	defaultMaxTotal    = 10_000
	maxRecipientLen    = 64
	maxErrorLen        = 256
)

// DeliveryTracker is a bounded in-memory store of delivery receipts, keyed by AgentID.
type DeliveryTracker struct {
	mu          sync.RWMutex
	receipts    map[types.AgentID][]DeliveryReceipt
	maxPerAgent int
	maxTotal    int
}

// NewDeliveryTracker creates a new DeliveryTracker with default capacity limits.
func NewDeliveryTracker() *DeliveryTracker {
	return &DeliveryTracker{
		receipts:    make(map[types.AgentID][]DeliveryReceipt),
		maxPerAgent: defaultMaxPerAgent,
		maxTotal:    defaultMaxTotal,
	}
}

// SentReceipt creates a receipt for a successful delivery.
func SentReceipt(agentID types.AgentID, channel, recipient string) DeliveryReceipt {
	return DeliveryReceipt{
		MessageID:   uuid.New().String(),
		AgentID:     agentID.String(),
		Channel:     channel,
		Recipient:   sanitizeRecipient(recipient),
		Status:      DeliveryStatusSent,
		DeliveredAt: time.Now().UTC(),
	}
}

// FailedReceipt creates a receipt for a failed delivery.
func FailedReceipt(agentID types.AgentID, channel, recipient, errMsg string) DeliveryReceipt {
	sanitized := sanitizeError(errMsg)
	return DeliveryReceipt{
		MessageID:   uuid.New().String(),
		AgentID:     agentID.String(),
		Channel:     channel,
		Recipient:   sanitizeRecipient(recipient),
		Status:      DeliveryStatusFailed,
		DeliveredAt: time.Now().UTC(),
		Error:       &sanitized,
	}
}

// Record appends a receipt for the given agent, enforcing per-agent and global capacity limits.
// Oldest entries are evicted when limits are exceeded.
func (t *DeliveryTracker) Record(agentID types.AgentID, receipt DeliveryReceipt) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.receipts[agentID] = append(t.receipts[agentID], receipt)

	// Per-agent cap: drain oldest
	if len(t.receipts[agentID]) > t.maxPerAgent {
		excess := len(t.receipts[agentID]) - t.maxPerAgent
		t.receipts[agentID] = t.receipts[agentID][excess:]
	}

	// Global cap: evict from the first agent that still has records
	total := 0
	for _, rs := range t.receipts {
		total += len(rs)
	}
	if total > t.maxTotal {
		for id, rs := range t.receipts {
			if len(rs) == 0 {
				continue
			}
			remove := total - t.maxTotal
			if remove > len(rs) {
				remove = len(rs)
			}
			t.receipts[id] = rs[remove:]
			total -= remove
			if total <= t.maxTotal {
				break
			}
		}
	}
}

// Get returns up to limit receipts for the given agent, newest first.
func (t *DeliveryTracker) Get(agentID types.AgentID, limit int) []DeliveryReceipt {
	t.mu.RLock()
	defer t.mu.RUnlock()

	rs, ok := t.receipts[agentID]
	if !ok || len(rs) == 0 {
		return nil
	}

	// Return a reversed copy (newest first)
	n := len(rs)
	if limit > 0 && limit < n {
		n = limit
	}
	out := make([]DeliveryReceipt, n)
	for i := 0; i < n; i++ {
		out[i] = rs[len(rs)-1-i]
	}
	return out
}

// Clear removes all receipts for the given agent.
func (t *DeliveryTracker) Clear(agentID types.AgentID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.receipts, agentID)
}

// GetAll returns all delivery receipts from all agents, up to the given limit (newest first).
func (t *DeliveryTracker) GetAll(limit int) []DeliveryReceipt {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var allReceipts []DeliveryReceipt
	for _, rs := range t.receipts {
		allReceipts = append(allReceipts, rs...)
	}

	// Sort by delivered_at descending (newest first)
	for i := len(allReceipts) - 1; i > 0; i-- {
		for j := 0; j < i; j++ {
			if allReceipts[j].DeliveredAt.Before(allReceipts[j+1].DeliveredAt) {
				allReceipts[j], allReceipts[j+1] = allReceipts[j+1], allReceipts[j]
			}
		}
	}

	// Apply limit
	if limit > 0 && limit < len(allReceipts) {
		allReceipts = allReceipts[:limit]
	}

	return allReceipts
}

// sanitizeRecipient strips control characters and truncates to maxRecipientLen.
func sanitizeRecipient(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	runes := []rune(s)
	if len(runes) > maxRecipientLen {
		runes = runes[:maxRecipientLen]
	}
	return string(runes)
}

// sanitizeError strips control characters and truncates to maxErrorLen.
func sanitizeError(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	runes := []rune(s)
	if len(runes) > maxErrorLen {
		runes = runes[:maxErrorLen]
	}
	return string(runes)
}
