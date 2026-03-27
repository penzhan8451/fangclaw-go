package delivery

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type DeliveryEntry struct {
	ID        string
	Name      string
	Payload   map[string]interface{}
	Status    DeliveryStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DeliveryRegistry struct {
	mu         sync.RWMutex
	deliveries map[string]*DeliveryEntry
}

func NewDeliveryRegistry() *DeliveryRegistry {
	return &DeliveryRegistry{deliveries: make(map[string]*DeliveryEntry)}
}

func (r *DeliveryRegistry) Create(name string, payload map[string]interface{}) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := uuid.New().String()
	r.deliveries[id] = &DeliveryEntry{
		ID:        id,
		Name:      name,
		Payload:   payload,
		Status:    DeliveryStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return id
}

func (r *DeliveryRegistry) Update(id string, status DeliveryStatus) {
	r.mu.Lock()
	if d, ok := r.deliveries[id]; ok {
		d.Status = status
		d.UpdatedAt = time.Now()
	}
	r.mu.Unlock()
}

func (r *DeliveryRegistry) Get(id string) (*DeliveryEntry, bool) {
	r.mu.RLock()
	d, ok := r.deliveries[id]
	r.mu.RUnlock()
	return d, ok
}

func (r *DeliveryRegistry) List() []*DeliveryEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*DeliveryEntry, 0, len(r.deliveries))
	for _, d := range r.deliveries {
		out = append(out, d)
	}
	return out
}
