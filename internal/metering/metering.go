// Package metering provides usage metering for OpenFang.
package metering

import (
	"sync"
	"time"
)

// MetricType represents the type of metric being measured.
type MetricType string

const (
	MetricTypeLLMTokens    MetricType = "llm_tokens"
	MetricTypeLLMRequests  MetricType = "llm_requests"
	MetricTypeToolCalls    MetricType = "tool_calls"
	MetricTypeMessagesSent MetricType = "messages_sent"
	MetricTypeMessagesRecv MetricType = "messages_received"
	MetricTypeAgentRuns    MetricType = "agent_runs"
	MetricTypeHandRuns     MetricType = "hand_runs"
)

// Metric represents a single usage metric.
type Metric struct {
	Type      MetricType
	AgentID   string
	Resource  string
	Value     int64
	Timestamp time.Time
	Metadata  map[string]string
}

// UsageRecord represents a period of usage.
type UsageRecord struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	Metrics     map[MetricType]int64
	PerAgent    map[string]map[MetricType]int64
}

// Meter tracks usage metrics.
type Meter struct {
	mu         sync.RWMutex
	metrics    []Metric
	currentDay time.Time
	dailyUsage *UsageRecord
}

// NewMeter creates a new usage meter.
func NewMeter() *Meter {
	now := time.Now().UTC()
	currentDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	return &Meter{
		metrics: make([]Metric, 0, 1000),
		currentDay: currentDay,
		dailyUsage: &UsageRecord{
			PeriodStart: currentDay,
			PeriodEnd:   currentDay.Add(24 * time.Hour),
			Metrics:     make(map[MetricType]int64),
			PerAgent:    make(map[string]map[MetricType]int64),
		},
	}
}

// Record records a usage metric.
func (m *Meter) Record(metricType MetricType, agentID, resource string, value int64, metadata map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if !today.Equal(m.currentDay) {
		m.rollOver(today)
	}

	metric := Metric{
		Type:      metricType,
		AgentID:   agentID,
		Resource:  resource,
		Value:     value,
		Timestamp: now,
		Metadata:  metadata,
	}

	m.metrics = append(m.metrics, metric)

	m.dailyUsage.Metrics[metricType] += value

	if agentID != "" {
		if _, ok := m.dailyUsage.PerAgent[agentID]; !ok {
			m.dailyUsage.PerAgent[agentID] = make(map[MetricType]int64)
		}
		m.dailyUsage.PerAgent[agentID][metricType] += value
	}
}

// RecordLLMTokens records LLM token usage.
func (m *Meter) RecordLLMTokens(agentID, model string, inputTokens, outputTokens int) {
	m.Record(MetricTypeLLMTokens, agentID, model, int64(inputTokens+outputTokens), map[string]string{
		"input_tokens":  string(inputTokens),
		"output_tokens": string(outputTokens),
		"model":         model,
	})
	m.Record(MetricTypeLLMRequests, agentID, model, 1, map[string]string{
		"model": model,
	})
}

// RecordToolCall records a tool call.
func (m *Meter) RecordToolCall(agentID, toolName string) {
	m.Record(MetricTypeToolCalls, agentID, toolName, 1, map[string]string{
		"tool": toolName,
	})
}

// RecordMessageSent records a sent message.
func (m *Meter) RecordMessageSent(agentID, channelType string) {
	m.Record(MetricTypeMessagesSent, agentID, channelType, 1, map[string]string{
		"channel": channelType,
	})
}

// RecordMessageReceived records a received message.
func (m *Meter) RecordMessageReceived(agentID, channelType string) {
	m.Record(MetricTypeMessagesRecv, agentID, channelType, 1, map[string]string{
		"channel": channelType,
	})
}

// RecordAgentRun records an agent run.
func (m *Meter) RecordAgentRun(agentID string) {
	m.Record(MetricTypeAgentRuns, agentID, "", 1, nil)
}

// RecordHandRun records a Hand run.
func (m *Meter) RecordHandRun(agentID, handID string) {
	m.Record(MetricTypeHandRuns, agentID, handID, 1, map[string]string{
		"hand_id": handID,
	})
}

// GetDailyUsage returns the current day's usage.
func (m *Meter) GetDailyUsage() *UsageRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dailyUsage
}

// GetMetric returns the total for a specific metric type.
func (m *Meter) GetMetric(metricType MetricType) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dailyUsage.Metrics[metricType]
}

// GetAgentMetric returns the total for a specific metric type per agent.
func (m *Meter) GetAgentMetric(agentID string, metricType MetricType) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if agentMetrics, ok := m.dailyUsage.PerAgent[agentID]; ok {
		return agentMetrics[metricType]
	}
	return 0
}

// GetMetrics returns all recorded metrics (up to the last 1000).
func (m *Meter) GetMetrics() []Metric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]Metric, len(m.metrics))
	copy(metrics, m.metrics)
	return metrics
}

// rollOver rolls over to a new day.
func (m *Meter) rollOver(newDay time.Time) {
	m.currentDay = newDay
	m.dailyUsage = &UsageRecord{
		PeriodStart: newDay,
		PeriodEnd:   newDay.Add(24 * time.Hour),
		Metrics:     make(map[MetricType]int64),
		PerAgent:    make(map[string]map[MetricType]int64),
	}

	if len(m.metrics) > 10000 {
		m.metrics = m.metrics[len(m.metrics)-10000:]
	}
}

// Reset resets all metrics.
func (m *Meter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = make([]Metric, 0, 1000)

	now := time.Now().UTC()
	currentDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	m.currentDay = currentDay
	m.dailyUsage = &UsageRecord{
		PeriodStart: currentDay,
		PeriodEnd:   currentDay.Add(24 * time.Hour),
		Metrics:     make(map[MetricType]int64),
		PerAgent:    make(map[string]map[MetricType]int64),
	}
}
