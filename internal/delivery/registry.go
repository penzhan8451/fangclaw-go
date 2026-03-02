package delivery

import (
	"sync"
	"time"
)

type DeliveryStatus string

const (
	DeliveryStatusPending    DeliveryStatus = "pending"
	DeliveryStatusInProgress DeliveryStatus = "in_progress"
	DeliveryStatusDone       DeliveryStatus = "done"
	DeliveryStatusFailed     DeliveryStatus = "failed"
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
	id := randomID()
	r.deliveries[id] = &DeliveryEntry{ID: id, Name: name, Payload: payload, Status: DeliveryStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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

func randomID() string {
	b := make([]byte, 4)
	t := time.Now().UnixNano()
	for i := 0; i < len(b); i++ {
		b[i] = byte((t >> (i * 8)) & 0xff)
	}
	// simple hex encoding
	hex := "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i := 0; i < len(b); i++ {
		out[i*2] = hex[(b[i]>>4)&0x0f]
		out[i*2+1] = hex[b[i]&0x0f]
	}
	return string(out)
}
