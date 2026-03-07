package embedding

import (
	"context"

	"github.com/penzhan8451/fangclaw-go/internal/vector"
)

// VectorEmbedderAdapter adapts vector.Embedder to embedding.Embedder
type VectorEmbedderAdapter struct {
	embedder vector.Embedder
}

// NewVectorEmbedderAdapter creates a new adapter
func NewVectorEmbedderAdapter(embedder vector.Embedder) *VectorEmbedderAdapter {
	return &VectorEmbedderAdapter{
		embedder: embedder,
	}
}

// EmbedText implements embedding.Embedder
func (a *VectorEmbedderAdapter) EmbedText(ctx context.Context, texts []string) ([][]float32, error) {
	vectors, err := a.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(vectors))
	for i, v := range vectors {
		embeddings[i] = []float32(v)
	}

	return embeddings, nil
}

// EmbedImage implements embedding.Embedder
func (a *VectorEmbedderAdapter) EmbedImage(ctx context.Context, imageData []byte) ([]float32, error) {
	return nil, &EmbeddingError{Message: "image embedding not supported"}
}

// Name implements embedding.Embedder
func (a *VectorEmbedderAdapter) Name() string {
	return a.embedder.Name()
}
