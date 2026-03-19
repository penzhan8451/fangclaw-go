// Package a2a provides Agent-to-Agent protocol support.
package a2a

import (
	"sync"
	"time"
)

// A2AMetrics tracks metrics for A2A operations.
type A2AMetrics struct {
	mu                sync.RWMutex
	tasksCreated      uint64
	tasksCompleted    uint64
	tasksFailed       uint64
	tasksCancelled    uint64
	agentsDiscovered  uint64
	tasksSentExternally uint64
	totalTaskDuration time.Duration
	taskDurations     []time.Duration
}

// NewA2AMetrics creates a new metrics collector.
func NewA2AMetrics() *A2AMetrics {
	return &A2AMetrics{
		taskDurations: make([]time.Duration, 0, 1000),
	}
}

// RecordTaskCreated records a task creation.
func (m *A2AMetrics) RecordTaskCreated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasksCreated++
}

// RecordTaskCompleted records a task completion.
func (m *A2AMetrics) RecordTaskCompleted(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasksCompleted++
	m.totalTaskDuration += duration
	m.taskDurations = append(m.taskDurations, duration)
	if len(m.taskDurations) > 1000 {
		m.taskDurations = m.taskDurations[len(m.taskDurations)-1000:]
	}
}

// RecordTaskFailed records a task failure.
func (m *A2AMetrics) RecordTaskFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasksFailed++
}

// RecordTaskCancelled records a task cancellation.
func (m *A2AMetrics) RecordTaskCancelled() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasksCancelled++
}

// RecordAgentDiscovered records an agent discovery.
func (m *A2AMetrics) RecordAgentDiscovered() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentsDiscovered++
}

// RecordTaskSentExternally records an external task send.
func (m *A2AMetrics) RecordTaskSentExternally() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasksSentExternally++
}

// GetMetrics returns the current metrics.
func (m *A2AMetrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var avgDuration time.Duration
	if len(m.taskDurations) > 0 {
		avgDuration = m.totalTaskDuration / time.Duration(len(m.taskDurations))
	}

	return map[string]interface{}{
		"tasks_created":        m.tasksCreated,
		"tasks_completed":      m.tasksCompleted,
		"tasks_failed":         m.tasksFailed,
		"tasks_cancelled":      m.tasksCancelled,
		"agents_discovered":    m.agentsDiscovered,
		"tasks_sent_externally": m.tasksSentExternally,
		"total_tasks":          m.tasksCreated,
		"avg_task_duration_ms": avgDuration.Milliseconds(),
	}
}
