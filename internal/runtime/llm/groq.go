// Package llm provides LLM provider integrations.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// GroqProvider implements Driver for Groq.
type GroqProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGroq creates a new Groq provider.
func NewGroq(apiKey, model string) *GroqProvider {
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	if model == "" {
		model = "llama-3.1-70b-versatile"
	}
	return &GroqProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the provider name.
func (p *GroqProvider) Name() string {
	return "groq"
}

// SupportsStreaming returns true if the provider supports streaming.
func (p *GroqProvider) SupportsStreaming() bool {
	return true
}

// Chat sends a chat completion request.
func (p *GroqProvider) Chat(ctx context.Context, req *Request) (*Response, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("groq API key not configured")
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	groqReq := openaiRequest{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	body, err := json.Marshal(groqReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq API error: %d", resp.StatusCode)
	}

	var groqResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return nil, ErrNoResponse
	}

	return &Response{
		Model:      groqResp.Model,
		Content:    groqResp.Choices[0].Message.Content,
		StopReason: groqResp.Choices[0].Finish,
		Usage: Usage{
			InputTokens:  groqResp.Usage.PromptTokens,
			OutputTokens: groqResp.Usage.CompletionTokens,
			TotalTokens:  groqResp.Usage.TotalTokens,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *GroqProvider) ChatStream(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("groq API key not configured")
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	groqReq := openaiRequest{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      true,
	}

	body, err := json.Marshal(groqReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("groq API error: %d", resp.StatusCode)
	}

	eventChan := make(chan StreamEvent)

	go func() {
		defer resp.Body.Close()
		defer close(eventChan)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var streamResp openaiStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				if choice.Delta.Content != "" {
					eventChan <- StreamEvent{
						Type: StreamEventTextDelta,
						Text: choice.Delta.Content,
					}
				}
				if choice.FinishReason != "" {
					eventChan <- StreamEvent{
						Type: StreamEventContentComplete,
					}
				}
			}
		}
	}()

	return eventChan, nil
}
