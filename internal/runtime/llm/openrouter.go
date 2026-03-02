// Package llm provides LLM provider integrations.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// OpenRouterProvider implements Driver for OpenRouter.
type OpenRouterProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenRouter creates a new OpenRouter provider.
func NewOpenRouter(apiKey, model string) *OpenRouterProvider {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if model == "" {
		model = "openai/gpt-4o"
	}
	return &OpenRouterProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the provider name.
func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

// SupportsStreaming returns true if the provider supports streaming.
func (p *OpenRouterProvider) SupportsStreaming() bool {
	return true
}

type openrouterRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type openrouterResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message Message `json:"message"`
		Finish  string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat sends a chat completion request.
func (p *OpenRouterProvider) Chat(ctx context.Context, req *Request) (*Response, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("openrouter API key not configured")
	}

	openrouterReq := openrouterRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	if openrouterReq.Model == "" {
		openrouterReq.Model = p.model
	}

	body, err := json.Marshal(openrouterReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/penzhan8451/fangclaw-go")
	httpReq.Header.Set("X-Title", "OpenFang")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter API error: %d", resp.StatusCode)
	}

	var openrouterResp openrouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openrouterResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openrouterResp.Choices) == 0 {
		return nil, ErrNoResponse
	}

	return &Response{
		Model:      openrouterResp.Model,
		Content:    openrouterResp.Choices[0].Message.Content,
		StopReason: openrouterResp.Choices[0].Finish,
		Usage: Usage{
			InputTokens:  openrouterResp.Usage.PromptTokens,
			OutputTokens: openrouterResp.Usage.CompletionTokens,
			TotalTokens:  openrouterResp.Usage.TotalTokens,
		},
	}, nil
}
