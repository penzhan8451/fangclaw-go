// Package embedding provides vector embedding support.
package embedding

import (
	"context"
	"sync"
)

// Embedding represents a vector embedding.
type Embedding []float32

// Embedder is the interface for embedding providers.
type Embedder interface {
	// EmbedText returns embeddings for text
	EmbedText(ctx context.Context, texts []string) ([][]float32, error)

	// EmbedImage returns embeddings for images
	EmbedImage(ctx context.Context, imageData []byte) ([]float32, error)

	// Name returns the provider name
	Name() string
}

// EmbeddingDriver is the main driver for embeddings.
type EmbeddingDriver struct {
	mu        sync.RWMutex
	primary   Embedder
	fallback  Embedder
	embedders map[string]Embedder
}

// NewEmbeddingDriver creates a new embedding driver.
func NewEmbeddingDriver() *EmbeddingDriver {
	return &EmbeddingDriver{
		embedders: make(map[string]Embedder),
	}
}

// Register registers an embedding provider.
func (d *EmbeddingDriver) Register(name string, embedder Embedder) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.embedders[name] = embedder
	if d.primary == nil {
		d.primary = embedder
	}
}

// SetPrimary sets the primary embedding provider.
func (d *EmbeddingDriver) SetPrimary(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	embedder, ok := d.embedders[name]
	if !ok {
		return &EmbeddingError{Message: "provider not found: " + name}
	}

	d.primary = embedder
	return nil
}

// SetFallback sets the fallback embedding provider.
func (d *EmbeddingDriver) SetFallback(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	embedder, ok := d.embedders[name]
	if !ok {
		return &EmbeddingError{Message: "provider not found: " + name}
	}

	d.fallback = embedder
	return nil
}

// EmbedText returns embeddings for text using the primary provider.
func (d *EmbeddingDriver) EmbedText(ctx context.Context, texts []string) ([][]float32, error) {
	d.mu.RLock()
	primary := d.primary
	d.mu.RUnlock()

	if primary == nil {
		// Return dummy embeddings if no provider configured
		dummy := make([][]float32, len(texts))
		for i := range texts {
			dummy[i] = make([]float32, 384) // Standard dimension
		}
		return dummy, nil
	}

	embeddings, err := primary.EmbedText(ctx, texts)
	if err != nil && d.fallback != nil {
		return d.fallback.EmbedText(ctx, texts)
	}

	return embeddings, err
}

// Similarity computes cosine similarity between two embeddings.
func Similarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct float32
	var normA, normB float32

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32)(normA*normB)
}

// FindMostSimilar finds the most similar text in a corpus.
func (d *EmbeddingDriver) FindMostSimilar(ctx context.Context, query string, corpus []string) (int, float32, error) {
	queryEmbeds, err := d.EmbedText(ctx, []string{query})
	if err != nil {
		return -1, 0, err
	}

	corpusEmbeds, err := d.EmbedText(ctx, corpus)
	if err != nil {
		return -1, 0, err
	}

	bestIdx := -1
	bestScore := float32(-1)

	for i, corpusEmbed := range corpusEmbeds {
		score := Similarity(queryEmbeds[0], corpusEmbed)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx, bestScore, nil
}

// EmbeddingError represents an embedding error.
type EmbeddingError struct {
	Message string
}

func (e *EmbeddingError) Error() string {
	return e.Message
}
