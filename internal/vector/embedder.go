// Package vector provides vector memory functionality for OpenFang.
package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Embedder is an interface for generating vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) (Vector, error)
	EmbedBatch(ctx context.Context, texts []string) ([]Vector, error)
	Name() string
}

// OpenAIEmbedder is an embedder that uses OpenAI's embedding API.
type OpenAIEmbedder struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIEmbedder creates a new OpenAI embedder.
func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the embedder name.
func (e *OpenAIEmbedder) Name() string {
	return "openai"
}

type embeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed generates an embedding for a single text.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) (Vector, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for a batch of texts.
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]Vector, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("openai API key not configured")
	}

	reqBody := embeddingRequest{
		Model: e.model,
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/embeddings",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embedding API error: %d", resp.StatusCode)
	}

	var respBody embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	embeddings := make([]Vector, len(respBody.Data))
	for i, data := range respBody.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// VectorMemory is a high-level interface for vector memory operations.
type VectorMemory struct {
	store    MemoryStore
	embedder Embedder
}

// NewVectorMemory creates a new vector memory instance.
func NewVectorMemory(store MemoryStore, embedder Embedder) *VectorMemory {
	return &VectorMemory{
		store:    store,
		embedder: embedder,
	}
}

// Add adds a text item to vector memory.
func (vm *VectorMemory) Add(ctx context.Context, id, content string, metadata map[string]interface{}) error {
	vector, err := vm.embedder.Embed(ctx, content)
	if err != nil {
		return err
	}

	item := &MemoryItem{
		ID:       id,
		Content:  content,
		Vector:   vector,
		Metadata: metadata,
	}

	return vm.store.Add(item)
}

// Search searches vector memory for similar items.
func (vm *VectorMemory) Search(ctx context.Context, query string, topK int) ([]*MemoryItem, error) {
	vector, err := vm.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	return vm.store.Search(vector, topK)
}

// Get retrieves an item from vector memory by ID.
func (vm *VectorMemory) Get(id string) (*MemoryItem, error) {
	return vm.store.Get(id)
}

// Delete removes an item from vector memory by ID.
func (vm *VectorMemory) Delete(id string) error {
	return vm.store.Delete(id)
}

// List returns all items in vector memory.
func (vm *VectorMemory) List() ([]*MemoryItem, error) {
	return vm.store.List()
}

// Clear removes all items from vector memory.
func (vm *VectorMemory) Clear() error {
	return vm.store.Clear()
}
