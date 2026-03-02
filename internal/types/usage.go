package types

import (
	"time"
)

// AgentEntry represents an agent stored in the database.
type AgentEntry struct {
	ID        AgentID                `json:"id"`
	Name      string                 `json:"name"`
	Manifest  AgentManifest          `json:"manifest"`
	State     map[string]interface{} `json:"state"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	SessionID SessionID              `json:"session_id"`
}

// UsageRecord represents a single usage record.
type UsageRecord struct {
	ID        string     `json:"id"`
	AgentID   AgentID    `json:"agent_id"`
	SessionID SessionID  `json:"session_id,omitempty"`
	Model     string     `json:"model"`
	Provider  string     `json:"provider"`
	Usage     TokenUsage `json:"usage"`
	CostUSD   float64    `json:"cost_usd,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// UsageSummary represents a summary of usage over a period.
type UsageSummary struct {
	TotalPromptTokens     int     `json:"total_prompt_tokens"`
	TotalCompletionTokens int     `json:"total_completion_tokens"`
	TotalTokens           int     `json:"total_tokens"`
	TotalCostUSD          float64 `json:"total_cost_usd"`
	RecordCount           int     `json:"record_count"`
}
