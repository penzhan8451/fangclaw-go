// Package scheduler provides agent resource quota management.
package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// UsageTracker tracks resource usage for an agent with a rolling hourly window.
type UsageTracker struct {
	TotalTokens int       `json:"total_tokens"`
	ToolCalls   int       `json:"tool_calls"`
	WindowStart time.Time `json:"window_start"`
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		WindowStart: time.Now(),
	}
}

// ResetIfExpired resets counters if the window has expired (1 hour).
func (u *UsageTracker) ResetIfExpired() {
	if time.Since(u.WindowStart) >= time.Hour {
		u.TotalTokens = 0
		u.ToolCalls = 0
		u.WindowStart = time.Now()
	}
}

// AgentScheduler manages agent resource quotas.
type AgentScheduler struct {
	mu     sync.RWMutex
	quotas map[string]ResourceQuota
	usage  map[string]*UsageTracker
}

// NewAgentScheduler creates a new scheduler.
func NewAgentScheduler() *AgentScheduler {
	return &AgentScheduler{
		quotas: make(map[string]ResourceQuota),
		usage:  make(map[string]*UsageTracker),
	}
}

// ResourceQuota defines spending limits for an agent.
type ResourceQuota struct {
	MaxTokensPerHour    int     `json:"max_tokens_per_hour" toml:"max_tokens_per_hour"`
	MaxToolCallsPerHour int     `json:"max_tool_calls_per_hour" toml:"max_tool_calls_per_hour"`
	MaxCostPerHourUSD   float64 `json:"max_cost_per_hour_usd" toml:"max_cost_per_hour_usd"`
}

// DefaultQuota returns default resource quotas.
func DefaultQuota() ResourceQuota {
	return ResourceQuota{
		MaxTokensPerHour:    100000,
		MaxToolCallsPerHour: 100,
		MaxCostPerHourUSD:   10.0,
	}
}

// Register registers an agent with its resource quota.
func (s *AgentScheduler) Register(agentID string, quota ResourceQuota) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotas[agentID] = quota
	s.usage[agentID] = NewUsageTracker()
}

// Unregister removes an agent from the scheduler.
func (s *AgentScheduler) Unregister(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.quotas, agentID)
	delete(s.usage, agentID)
}

// RecordUsage records token usage for an agent.
func (s *AgentScheduler) RecordUsage(agentID string, usage types.TokenUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tracker, ok := s.usage[agentID]; ok {
		tracker.ResetIfExpired()
		tracker.TotalTokens += usage.TotalTokens
	}
}

// RecordToolCall records a tool call for an agent.
func (s *AgentScheduler) RecordToolCall(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tracker, ok := s.usage[agentID]; ok {
		tracker.ResetIfExpired()
		tracker.ToolCalls++
	}
}

// GetUsage returns current usage for an agent.
func (s *AgentScheduler) GetUsage(agentID string) (*UsageTracker, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tracker, ok := s.usage[agentID]
	if !ok {
		return nil, false
	}
	return &UsageTracker{
		TotalTokens: tracker.TotalTokens,
		ToolCalls:   tracker.ToolCalls,
		WindowStart: tracker.WindowStart,
	}, true
}

// GetQuota returns the quota for an agent.
func (s *AgentScheduler) GetQuota(agentID string) (ResourceQuota, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	quota, ok := s.quotas[agentID]
	return quota, ok
}

// CheckQuota checks if an agent has exceeded its quota.
func (s *AgentScheduler) CheckQuota(agentID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	quota, ok := s.quotas[agentID]
	if !ok {
		return nil // No quota set
	}

	tracker, ok := s.usage[agentID]
	if !ok {
		return nil
	}

	tracker.ResetIfExpired()

	if quota.MaxTokensPerHour > 0 && tracker.TotalTokens >= quota.MaxTokensPerHour {
		return &QuotaExceededError{
			Resource: "tokens",
			Limit:    quota.MaxTokensPerHour,
			Used:     tracker.TotalTokens,
		}
	}

	if quota.MaxToolCallsPerHour > 0 && tracker.ToolCalls >= quota.MaxToolCallsPerHour {
		return &QuotaExceededError{
			Resource: "tool_calls",
			Limit:    quota.MaxToolCallsPerHour,
			Used:     tracker.ToolCalls,
		}
	}

	return nil
}

// TokenHeadroom returns remaining token headroom before quota is hit.
// Returns 0 if no token quota is configured (unlimited).
func (s *AgentScheduler) TokenHeadroom(agentID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	quota, ok := s.quotas[agentID]
	if !ok || quota.MaxTokensPerHour == 0 {
		return 0
	}

	tracker, ok := s.usage[agentID]
	if !ok {
		return quota.MaxTokensPerHour
	}

	tracker.ResetIfExpired()
	if tracker.TotalTokens >= quota.MaxTokensPerHour {
		return 0
	}
	return quota.MaxTokensPerHour - tracker.TotalTokens
}

// ResetUsage resets usage tracking for an agent.
func (s *AgentScheduler) ResetUsage(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tracker, ok := s.usage[agentID]; ok {
		tracker.TotalTokens = 0
		tracker.ToolCalls = 0
		tracker.WindowStart = time.Now()
	}
}

// QuotaExceededError represents a quota exceeded error.
type QuotaExceededError struct {
	Resource string
	Limit    int
	Used     int
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded: %s (used: %d, limit: %d)", e.Resource, e.Used, e.Limit)
}
