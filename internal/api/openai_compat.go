package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
)

// OpenAICompatibleHandler implements OpenAI-compatible API endpoints.
type OpenAICompatibleHandler struct {
	kernel *kernel.Kernel
}

// NewOpenAICompatibleHandler creates a new OpenAI-compatible handler.
func NewOpenAICompatibleHandler(k *kernel.Kernel) *OpenAICompatibleHandler {
	return &OpenAICompatibleHandler{kernel: k}
}

// ChatCompletionRequest represents an OpenAI chat completion request.
type ChatCompletionRequest struct {
	Model       string       `json:"model"`
	Messages    []OaiMessage `json:"messages"`
	Stream      bool         `json:"stream"`
	MaxTokens   *int         `json:"max_tokens,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
	TopP        *float64     `json:"top_p,omitempty"`
}

// OaiMessage represents a message in the OpenAI chat format.
type OaiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ChatCompletionResponse represents an OpenAI chat completion response.
type ChatCompletionResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   UsageInfo `json:"usage"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int           `json:"index"`
	Message      ChoiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// ChoiceMessage represents the message in a choice.
type ChoiceMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// UsageInfo represents token usage information.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents a streaming chat completion chunk.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice represents a streaming completion choice.
type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason,omitempty"`
}

// ChunkDelta represents the delta in a streaming completion.
type ChunkDelta struct {
	Role    *string `json:"role,omitempty"`
	Content *string `json:"content,omitempty"`
}

// ModelListResponse represents a model list response.
type ModelListResponse struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// ModelObject represents a model object.
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// RegisterRoutes registers OpenAI-compatible routes.
func (h *OpenAICompatibleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/models", loggingMiddleware(corsMiddleware(h.ModelsHandler)))
	mux.HandleFunc("/v1/chat/completions", loggingMiddleware(corsMiddleware(h.ChatCompletionsHandler)))
}

// ModelsHandler handles the /v1/models endpoint.
func (h *OpenAICompatibleHandler) ModelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}

	agents := h.kernel.AgentRegistry().List()
	created := time.Now().Unix()

	models := []ModelObject{}
	for _, agent := range agents {
		models = append(models, ModelObject{
			ID:      fmt.Sprintf("fangclaw:%s", agent.Name),
			Object:  "model",
			Created: created,
			OwnedBy: "fangclaw",
		})
		models = append(models, ModelObject{
			ID:      agent.Name,
			Object:  "model",
			Created: created,
			OwnedBy: "fangclaw",
		})
	}

	if len(models) == 0 {
		models = []ModelObject{
			{
				ID:      "fangclaw:default",
				Object:  "model",
				Created: created,
				OwnedBy: "fangclaw",
			},
			{
				ID:      "default",
				Object:  "model",
				Created: created,
				OwnedBy: "fangclaw",
			},
		}
	}

	response := ModelListResponse{
		Object: "list",
		Data:   models,
	}

	WriteJSON(w, http.StatusOK, response)
}

// ChatCompletionsHandler handles the /v1/chat/completions endpoint.
func (h *OpenAICompatibleHandler) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}

	var req ChatCompletionRequest
	if err := ParseJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	// Extract the last user message
	var lastMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg.Role == "user" {
			switch v := msg.Content.(type) {
			case string:
				lastMessage = v
			}
			break
		}
	}

	if lastMessage == "" {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("no user message found"))
		return
	}

	// Resolve agent from model name
	_, agentName := h.resolveAgent(req.Model)

	driver, err := getLLMDriver()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var messages []llm.Message

	if hand, _ := hands.GetBundledHand(agentName); hand != nil {
		systemPrompt := getHandSystemPrompt(agentName)
		if systemPrompt != "" {
			messages = append(messages, llm.Message{
				Role:    "system",
				Content: systemPrompt,
			})
		}
	}

	// Add messages from request
	for _, msg := range req.Messages {
		var content string
		switch v := msg.Content.(type) {
		case string:
			content = v
		}
		if content == "" {
			continue
		}
		messages = append(messages, llm.Message{
			Role:    msg.Role,
			Content: content,
		})
	}

	// Handle streaming (暂时禁用，先实现非流式)
	// if req.Stream {
	// 	h.handleStreamingChat(w, r, driver, messages, agentName)
	// 	return
	// }

	// Non-streaming
	llmReq := &llm.Request{
		Messages:    messages,
		Temperature: 0.7,
	}

	if req.Temperature != nil {
		llmReq.Temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		llmReq.MaxTokens = *req.MaxTokens
	}
	if req.TopP != nil {
		llmReq.TopP = *req.TopP
	}

	ctx := context.Background()
	resp, err := driver.Chat(ctx, llmReq)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	// Build OpenAI-compatible response
	completionResp := ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: ChoiceMessage{
					Role:    "assistant",
					Content: resp.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: UsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	WriteJSON(w, http.StatusOK, completionResp)
}

func (h *OpenAICompatibleHandler) resolveAgent(model string) (string, string) {
	var agentName string

	if strings.HasPrefix(model, "fangclaw:") {
		agentName = strings.TrimPrefix(model, "fangclaw:")
	} else {
		agentName = model
	}

	agents := h.kernel.AgentRegistry().List()
	for _, agent := range agents {
		if agent.Name == agentName {
			return "", agent.Name
		}
	}

	if len(agents) > 0 {
		return "", agents[0].Name
	}

	return "", "default"
}

// handleStreamingChat handles streaming chat completions.
func (h *OpenAICompatibleHandler) handleStreamingChat(w http.ResponseWriter, r *http.Request, message, model string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	response := "This is a placeholder streaming response from FangClaw. The full LLM integration is coming soon!"

	// Send initial chunk with role
	initialChunk := ChatCompletionChunk{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChunkChoice{
			{
				Index: 0,
				Delta: ChunkDelta{
					Role: stringPtr("assistant"),
				},
			},
		},
	}
	initialBytes, _ := json.Marshal(initialChunk)
	fmt.Fprintf(w, "data: %s\n\n", initialBytes)
	w.(http.Flusher).Flush()

	// Send response in chunks
	chunkSize := 10
	for i := 0; i < len(response); i += chunkSize {
		end := i + chunkSize
		if end > len(response) {
			end = len(response)
		}
		chunkContent := response[i:end]

		chunk := ChatCompletionChunk{
			ID:      initialChunk.ID,
			Object:  "chat.completion.chunk",
			Created: initialChunk.Created,
			Model:   model,
			Choices: []ChunkChoice{
				{
					Index: 0,
					Delta: ChunkDelta{
						Content: &chunkContent,
					},
				},
			},
		}
		chunkBytes, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", chunkBytes)
		w.(http.Flusher).Flush()
		time.Sleep(30 * time.Millisecond)
	}

	// Send final chunk
	finalChunk := ChatCompletionChunk{
		ID:      initialChunk.ID,
		Object:  "chat.completion.chunk",
		Created: initialChunk.Created,
		Model:   model,
		Choices: []ChunkChoice{
			{
				Index:        0,
				Delta:        ChunkDelta{},
				FinishReason: stringPtr("stop"),
			},
		},
	}
	finalBytes, _ := json.Marshal(finalChunk)
	fmt.Fprintf(w, "data: %s\n\n", finalBytes)
	w.(http.Flusher).Flush()

	// Send [DONE] marker
	fmt.Fprintf(w, "data: [DONE]\n\n")
	w.(http.Flusher).Flush()
}

func stringPtr(s string) *string {
	return &s
}
