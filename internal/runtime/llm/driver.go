// Package llm provides LLM provider integrations.
package llm

import (
	"context"
	"errors"
)

// ErrNoResponse is returned when no response is received.
var ErrNoResponse = errors.New("no response received")

// StreamEventType represents the type of a stream event.
type StreamEventType string

const (
	StreamEventTextDelta       StreamEventType = "text_delta"
	StreamEventContentComplete StreamEventType = "content_complete"
)

// StreamEvent represents an event in the streaming response.
type StreamEvent struct {
	Type StreamEventType
	Text string
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // Message content
}

// Request represents a chat completion request.
type Request struct {
	Model       string                   `json:"model"`
	Messages    []Message                `json:"messages"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	TopP        float64                  `json:"top_p,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
}

// ToolCall represents a tool call in the response.
type ToolCall struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// Response represents a chat completion response.
type Response struct {
	Model            string     `json:"model"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	StopReason       string     `json:"stop_reason,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	Usage            Usage      `json:"usage,omitempty"`
}

// Usage represents token usage.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Driver interface for LLM providers.
type Driver interface {
	// Name returns the provider name.
	Name() string
	// Chat sends a chat completion request.
	Chat(ctx context.Context, req *Request) (*Response, error)
	// ChatStream sends a streaming chat completion request.
	ChatStream(ctx context.Context, req *Request) (<-chan StreamEvent, error)
	// SupportsStreaming returns true if the provider supports streaming.
	SupportsStreaming() bool
}

// Config holds LLM provider configuration.
type Config struct {
	Anthropic  AnthropicConfig  `json:"anthropic,omitempty"`
	OpenAI     OpenAIConfig     `json:"openai,omitempty"`
	Groq       GroqConfig       `json:"groq,omitempty"`
	Gemini     GeminiConfig     `json:"gemini,omitempty"`
	OpenRouter OpenRouterConfig `json:"openrouter,omitempty"`
	DeepSeek   DeepSeekConfig   `json:"deepseek,omitempty"`
	Qwen       QwenConfig       `json:"qwen,omitempty"`
	Zhipu      ZhipuConfig      `json:"zhipu,omitempty"`
	Moonshot   MoonshotConfig   `json:"moonshot,omitempty"`
	MiniMax    MiniMaxConfig    `json:"minimax,omitempty"`
	Qianfan    QianfanConfig    `json:"qianfan,omitempty"`
	VolcEngine VolcEngineConfig `json:"volcengine,omitempty"`
}

// AnthropicConfig holds Anthropic API configuration.
type AnthropicConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// OpenAIConfig holds OpenAI API configuration.
type OpenAIConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// GroqConfig holds Groq API configuration.
type GroqConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// GeminiConfig holds Gemini API configuration.
type GeminiConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// OpenRouterConfig holds OpenRouter API configuration.
type OpenRouterConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// DeepSeekConfig holds DeepSeek API configuration.
type DeepSeekConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// QwenConfig holds Qwen API configuration.
type QwenConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// ZhipuConfig holds Zhipu AI API configuration.
type ZhipuConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// MoonshotConfig holds Moonshot API configuration.
type MoonshotConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// MiniMaxConfig holds MiniMax API configuration.
type MiniMaxConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// QianfanConfig holds Qianfan API configuration.
type QianfanConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}

// VolcEngineConfig holds VolcEngine API configuration.
type VolcEngineConfig struct {
	APIKey string `json:"api_key,omitempty"`
	Model  string `json:"model,omitempty"`
}
