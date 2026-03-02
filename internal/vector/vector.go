// Package vector provides vector memory functionality for OpenFang.
package vector

import (
	"math"
	"sync"
)

// Vector represents a vector embedding.
type Vector []float32

// MemoryItem represents an item in vector memory.
type MemoryItem struct {
	ID       string  `json:"id"`
	Content  string  `json:"content"`
	Vector   Vector  `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryStore is an interface for vector memory storage.
type MemoryStore interface {
	Add(item *MemoryItem) error
	Get(id string) (*MemoryItem, error)
	Delete(id string) error
	Search(query Vector, topK int) ([]*MemoryItem, error)
	List() ([]*MemoryItem, error)
	Clear() error
}

// InMemoryStore is an in-memory implementation of MemoryStore.
type InMemoryStore struct {
	mu      sync.RWMutex
	items   map[string]*MemoryItem
}

// NewInMemoryStore creates a new in-memory vector store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		items: make(map[string]*MemoryItem),
	}
}

// Add adds an item to the vector store.
func (s *InMemoryStore) Add(item *MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

// Get retrieves an item from the vector store by ID.
func (s *InMemoryStore) Get(id string) (*MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return nil, nil
	}
	return item, nil
}

// Delete removes an item from the vector store by ID.
func (s *InMemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

// Search performs a similarity search on the vector store.
func (s *InMemoryStore) Search(query Vector, topK int) ([]*MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type result struct {
		item       *MemoryItem
		similarity float32
	}

	results := make([]result, 0, len(s.items))
	for _, item := range s.items {
		sim := cosineSimilarity(query, item.Vector)
		results = append(results, result{item, sim})
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].similarity > results[i].similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > len(results) {
		topK = len(results)
	}

	items := make([]*MemoryItem, topK)
	for i := 0; i < topK; i++ {
		items[i] = results[i].item
	}

	return items, nil
}

// List returns all items in the vector store.
func (s *InMemoryStore) List() ([]*MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*MemoryItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}

// Clear removes all items from the vector store.
func (s *InMemoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*MemoryItem)
	return nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b Vector) float32 {
	if len(a) != len(b) {
		return 0
	}

	dotProduct := float32(0)
	normA := float32(0)
	normB := float32(0)

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
