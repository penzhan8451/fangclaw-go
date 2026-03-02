// Package scheduler provides agent scheduling and resource management.
package scheduler

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// UsageTracker tracks resource usage for an agent with a rolling hourly window.
type UsageTracker struct {
	TotalTokens int       `json:"total_tokens"`
	ToolCalls   int       `json:"tool_calls"`
	WindowStart time.Time `json:"window_start_start"`
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

// AgentScheduler manages agent execution ordering and resource quotas.
type AgentScheduler struct {
	mu     sync.RWMutex
	quotas map[string]ResourceQuota
	usage  map[string]*UsageTracker
	tasks  map[string]interface{} // JoinHandle placeholder
}

// NewAgentScheduler creates a new scheduler.
func NewAgentScheduler() *AgentScheduler {
	return &AgentScheduler{
		quotas: make(map[string]ResourceQuota),
		usage:  make(map[string]*UsageTracker),
		tasks:  make(map[string]interface{}),
	}
}

// ResourceQuota defines spending limits for an agent.
type ResourceQuota struct {
	MaxCostPerHourUSD   float64 `json:"max_cost_per_hour_usd"`
	MaxCostPerDayUSD    float64 `json:"max_cost_per_day_usd"`
	MaxCostPerMonthUSD  float64 `json:"max_cost_per_month_usd"`
	MaxTokensPerHour    int     `json:"max_tokens_per_hour"`
	MaxToolCallsPerHour int     `json:"max_tool_calls_per_hour"`
}

// DefaultQuota returns default resource quotas.
func DefaultQuota() ResourceQuota {
	return ResourceQuota{
		MaxCostPerHourUSD:   10.0,
		MaxCostPerDayUSD:    100.0,
		MaxCostPerMonthUSD:  1000.0,
		MaxTokensPerHour:    100000,
		MaxToolCallsPerHour: 100,
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
func (s *AgentScheduler) RecordUsage(agentID string, tokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tracker, ok := s.usage[agentID]; ok {
		tracker.ResetIfExpired()
		tracker.TotalTokens += tokens
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
func (s *AgentScheduler) GetUsage(agentID string) *UsageTracker {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tracker, ok := s.usage[agentID]; ok {
		return &UsageTracker{
			TotalTokens: tracker.TotalTokens,
			ToolCalls:   tracker.ToolCalls,
			WindowStart: tracker.WindowStart,
		}
	}
	return &UsageTracker{WindowStart: time.Now()}
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
		return &QuotaExceededError{Resource: "tokens", Limit: quota.MaxTokensPerHour}
	}

	if quota.MaxToolCallsPerHour > 0 && tracker.ToolCalls >= quota.MaxToolCallsPerHour {
		return &QuotaExceededError{Resource: "tool_calls", Limit: quota.MaxToolCallsPerHour}
	}

	return nil
}

// QuotaExceededError represents a quota exceeded error.
type QuotaExceededError struct {
	Resource string
	Limit    int
}

func (e *QuotaExceededError) Error() string {
	return "quota exceeded: " + e.Resource
}

// ScheduleInfo represents scheduling information for an agent.
type ScheduleInfo struct {
	AgentID     string        `json:"agent_id"`
	ScheduledAt time.Time     `json:"scheduled_at"`
	Interval    time.Duration `json:"interval"`
	Enabled     bool          `json:"enabled"`
	LastRun     time.Time     `json:"last_run,omitempty"`
	NextRun     time.Time     `json:"next_run,omitempty"`
}

// ScheduleManager manages scheduled agent runs.
type ScheduleManager struct {
	mu        sync.RWMutex
	schedules map[string]*ScheduleInfo
}

// NewScheduleManager creates a new schedule manager.
func NewScheduleManager() *ScheduleManager {
	return &ScheduleManager{
		schedules: make(map[string]*ScheduleInfo),
	}
}

// AddSchedule adds a new schedule.
func (m *ScheduleManager) AddSchedule(agentID string, interval time.Duration) *ScheduleInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := &ScheduleInfo{
		AgentID:     agentID,
		Interval:    interval,
		Enabled:     true,
		ScheduledAt: time.Now(),
		NextRun:     time.Now().Add(interval),
	}
	m.schedules[agentID] = info
	return info
}

// RemoveSchedule removes a schedule.
func (m *ScheduleManager) RemoveSchedule(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.schedules, agentID)
}

// GetSchedules returns all schedules.
func (m *ScheduleManager) GetSchedules() []*ScheduleInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*ScheduleInfo, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// GetDueSchedules returns schedules that are due to run.
func (m *ScheduleManager) GetDueSchedules() []*ScheduleInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var due []*ScheduleInfo
	for _, s := range m.schedules {
		if s.Enabled && now.After(s.NextRun) {
			due = append(due, s)
		}
	}
	return due
}

// MarkRun marks a schedule as run and calculates next run time.
func (m *ScheduleManager) MarkRun(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.schedules[agentID]; ok {
		s.LastRun = time.Now()
		s.NextRun = time.Now().Add(s.Interval)
	}
}

// ToggleSchedule enables or disables a schedule.
func (m *ScheduleManager) ToggleSchedule(agentID string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.schedules[agentID]; ok {
		s.Enabled = enabled
	}
}

// GenerateScheduleID generates a unique schedule ID.
func GenerateScheduleID() string {
	return uuid.New().String()
}
