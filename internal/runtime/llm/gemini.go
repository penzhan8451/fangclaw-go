// Package llm provides LLM provider integrations.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// GeminiProvider implements Driver for Google Gemini.
type GeminiProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGemini creates a new Gemini provider.
func NewGemini(apiKey, model string) *GeminiProvider {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if model == "" {
		model = "gemini-1.5-pro"
	}
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// SupportsStreaming returns true if the provider supports streaming.
func (p *GeminiProvider) SupportsStreaming() bool {
	return true
}

type geminiRequest struct {
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		MaxTokens   int     `json:"maxOutputTokens"`
		Temperature float64 `json:"temperature,omitempty"`
		TopP        float64 `json:"topP,omitempty"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// Chat sends a chat completion request.
func (p *GeminiProvider) Chat(ctx context.Context, req *Request) (*Response, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("gemini API key not configured")
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	// Convert model name to gemini format
	if !strings.HasPrefix(model, "gemini-") {
		model = "gemini-" + model
	}

	geminiReq := geminiRequest{}
	geminiReq.GenerationConfig.MaxTokens = req.MaxTokens
	if geminiReq.GenerationConfig.MaxTokens == 0 {
		geminiReq.GenerationConfig.MaxTokens = 4096
	}
	geminiReq.GenerationConfig.Temperature = req.Temperature
	geminiReq.GenerationConfig.TopP = req.TopP

	for _, msg := range req.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		geminiReq.Contents = append(geminiReq.Contents, struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{role, []struct {
			Text string `json:"text"`
		}{{msg.Content}}})
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		url,
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error: %d", resp.StatusCode)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, ErrNoResponse
	}

	var content string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		content += part.Text
	}

	return &Response{
		Model:      model,
		Content:    content,
		StopReason: geminiResp.Candidates[0].FinishReason,
		Usage: Usage{
			InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
			OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  geminiResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}
