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

// AnthropicProvider implements Driver for Anthropic Claude.
type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropic creates a new Anthropic provider.
func NewAnthropic(apiKey, model string) *AnthropicProvider {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// SupportsStreaming returns true if the provider supports streaming.
func (p *AnthropicProvider) SupportsStreaming() bool {
	return true
}

type anthropicRequest struct {
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	Messages    []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type anthropicResponse struct {
	Type    string `json:"type"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat sends a chat completion request.
func (p *AnthropicProvider) Chat(ctx context.Context, req *Request) (*Response, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("anthropic API key not configured")
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthropicReq := anthropicRequest{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	for _, msg := range req.Messages {
		anthropicReq.Messages = append(anthropicReq.Messages, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{msg.Role, msg.Content})
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.anthropic.com/v1/messages",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error: %d", resp.StatusCode)
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, ErrNoResponse
	}

	return &Response{
		Model:      anthropicResp.Model,
		Content:    anthropicResp.Content[0].Text,
		StopReason: anthropicResp.StopReason,
		Usage: Usage{
			InputTokens:  anthropicResp.Usage.InputTokens,
			OutputTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:  anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request (simulated).
func (p *AnthropicProvider) ChatStream(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	eventChan := make(chan StreamEvent)

	go func() {
		defer close(eventChan)

		chunkSize := 5
		for i := 0; i < len(resp.Content); i += chunkSize {
			end := i + chunkSize
			if end > len(resp.Content) {
				end = len(resp.Content)
			}
			eventChan <- StreamEvent{
				Type: StreamEventTextDelta,
				Text: resp.Content[i:end],
			}
			time.Sleep(20 * time.Millisecond)
		}

		eventChan <- StreamEvent{
			Type: StreamEventContentComplete,
		}
	}()

	return eventChan, nil
}
