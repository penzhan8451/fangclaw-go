package delivery

import (
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type DeliveryReceipt struct {
	MessageID  string
	Status     string
	DeliveredAt time.Time
}

type DeliveryTracker struct {
	mu              sync.RWMutex
	receipts        map[types.AgentID][]DeliveryReceipt
	maxPerAgent     int
	maxTotal        int
}

func NewDeliveryTracker() *DeliveryTracker {
	return &DeliveryTracker{
		receipts:    make(map[types.AgentID][]DeliveryReceipt),
		maxPerAgent: 500,
		maxTotal:    10000,
	}
}

func (t *DeliveryTracker) Record(agentID types.AgentID, receipt DeliveryReceipt) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.receipts[agentID] = append(t.receipts[agentID], receipt)

	if len(t.receipts[agentID]) > t.maxPerAgent {
		excess := len(t.receipts[agentID]) - t.maxPerAgent
		t.receipts[agentID] = t.receipts[agentID][excess:]
	}

	total := 0
	for _, receipts := range t.receipts {
		total += len(receipts)
	}

	if total > t.maxTotal {
		for id, receipts := range t.receipts {
			if len(receipts) > 0 {
				remove := total - t.maxTotal
				if remove > len(receipts) {
					remove = len(receipts)
				}
				t.receipts[id] = receipts[remove:]
				total -= remove
				if total <= t.maxTotal {
					break
				}
			}
		}
	}
}

func (t *DeliveryTracker) Get(agentID types.AgentID) []DeliveryReceipt {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if receipts, ok := t.receipts[agentID]; ok {
		result := make([]DeliveryReceipt, len(receipts))
		copy(result, receipts)
		return result
	}
	return nil
}

func (t *DeliveryTracker) Clear(agentID types.AgentID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.receipts, agentID)
}
