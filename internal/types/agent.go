// Package types provides core data structures for OpenFang.
package types

import (
	"time"

	"github.com/google/uuid"
)

// AgentID is a unique identifier for an agent.
type AgentID = uuid.UUID

// NewAgentID creates a new agent ID.
func NewAgentID() AgentID {
	return uuid.New()
}

// ParseAgentID parses a string into an AgentID.
func ParseAgentID(s string) (AgentID, error) {
	return uuid.Parse(s)
}

// AgentState represents the current state of an agent.
type AgentState string

const (
	AgentStatePending  AgentState = "pending"
	AgentStateRunning  AgentState = "running"
	AgentStateIdle     AgentState = "idle"
	AgentStateThinking AgentState = "thinking"
	AgentStateWaiting  AgentState = "waiting"
	AgentStateError    AgentState = "error"
	AgentStateStopped  AgentState = "stopped"
)

// Agent represents an agent in the system.
type Agent struct {
	ID        AgentID           `json:"id"`
	Name      string            `json:"name"`
	State     AgentState        `json:"state"`
	Model     string            `json:"model_provider"`
	ModelName string            `json:"model_name"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AgentManifest defines an agent's configuration.
type AgentManifest struct {
	Name         string            `toml:"name" json:"name"`
	Description  string            `toml:"description" json:"description,omitempty"`
	SystemPrompt string            `toml:"system_prompt" json:"system_prompt,omitempty"`
	Model        ModelConfig       `toml:"model" json:"model,omitempty"`
	Tools        []string          `toml:"tools" json:"tools,omitempty"`
	Skills       []string          `toml:"skills" json:"skills,omitempty"`
	Metadata     map[string]string `toml:"metadata" json:"metadata,omitempty"`
}

// ModelConfig defines the LLM configuration for an agent.
type ModelConfig struct {
	Provider  string `toml:"provider" json:"provider"`
	Model     string `toml:"model" json:"model"`
	APIKeyEnv string `toml:"api_key_env" json:"api_key_env,omitempty"`
}

// Provider represents an LLM provider.
type Provider struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	APIKeyEnv      string `json:"api_key_env"`
	APIBaseURL     string `json:"api_base_url,omitempty"`
	SupportsStream bool   `json:"supports_stream"`
}

// Model represents an available LLM model.
type Model struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	ContextSize    int    `json:"context_size"`
	SupportsVision bool   `json:"supports_vision"`
	SupportsTools  bool   `json:"supports_tools"`
}

// DaemonInfo contains information about a running daemon.
type DaemonInfo struct {
	PID        int       `json:"pid"`
	ListenAddr string    `json:"listen_addr"`
	StartedAt  time.Time `json:"started_at"`
	Version    string    `json:"version"`
}

// HealthStatus represents the health of the system.
type HealthStatus struct {
	Status  string          `json:"status"`
	Healthy bool            `json:"healthy"`
	Checks  map[string]bool `json:"checks,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// StatusResponse represents the daemon status.
type StatusResponse struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	ListenAddr string `json:"listen_addr"`
	AgentCount int    `json:"agent_count"`
	ModelCount int    `json:"model_count"`
	Uptime     string `json:"uptime"`
}

// ReplyDirective controls how the agent loop responds.
type ReplyDirective struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Stop      bool       `json:"stop"`
}

// LoopConfig configures the agent loop behavior.
type LoopConfig struct {
	MaxIterations int     `json:"max_iterations"`
	MaxTokens     int     `json:"max_tokens"`
	Temperature   float64 `json:"temperature"`
	TopP          float64 `json:"top_p"`
}

// ChatRequest represents a chat message request.
type ChatRequest struct {
	Message string `json:"message"`
	AgentID string `json:"agent_id,omitempty"`
}

// ChatResponse represents a chat message response.
type ChatResponse struct {
	Response   string `json:"response"`
	AgentID    string `json:"agent_id"`
	StopReason string `json:"stop_reason,omitempty"`
	Error      string `json:"error,omitempty"`
}
