// Package background provides background agent execution for OpenFang.
package background

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ScheduleMode represents the scheduling mode for an agent.
type ScheduleMode string

const (
	ScheduleModeReactive   ScheduleMode = "reactive"
	ScheduleModeContinuous ScheduleMode = "continuous"
	ScheduleModePeriodic   ScheduleMode = "periodic"
	ScheduleModeProactive  ScheduleMode = "proactive"
)

// ContinuousSchedule represents a continuous schedule configuration.
type ContinuousSchedule struct {
	CheckIntervalSecs int
}

// PeriodicSchedule represents a periodic schedule configuration.
type PeriodicSchedule struct {
	CronExpression string
}

// ProactiveSchedule represents a proactive schedule configuration.
type ProactiveSchedule struct {
	TriggerPatterns []string
}

// AgentSchedule represents the complete schedule configuration for an agent.
type AgentSchedule struct {
	Mode       ScheduleMode
	Continuous *ContinuousSchedule
	Periodic   *PeriodicSchedule
	Proactive  *ProactiveSchedule
}

// SendMessageFunc is a function that sends a message to an agent.
type SendMessageFunc func(agentID, message string)

// BackgroundExecutor manages background task loops for autonomous agents.
type BackgroundExecutor struct {
	mu             sync.RWMutex
	tasks          map[string]context.CancelFunc
	llmSemaphore   chan struct{}
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

const maxConcurrentBGLLM = 5

// NewBackgroundExecutor creates a new background executor.
func NewBackgroundExecutor() *BackgroundExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackgroundExecutor{
		tasks:          make(map[string]context.CancelFunc),
		llmSemaphore:   make(chan struct{}, maxConcurrentBGLLM),
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}
}

// StartAgent starts a background loop for an agent based on its schedule mode.
func (be *BackgroundExecutor) StartAgent(agentID, agentName string, schedule AgentSchedule, sendMessage SendMessageFunc) error {
	be.mu.Lock()
	defer be.mu.Unlock()

	if _, exists := be.tasks[agentID]; exists {
		return fmt.Errorf("agent %s already has a background task running", agentID)
	}

	ctx, cancel := context.WithCancel(be.shutdownCtx)
	be.tasks[agentID] = cancel

	switch schedule.Mode {
	case ScheduleModeContinuous:
		if schedule.Continuous != nil {
			go be.runContinuous(ctx, agentID, agentName, schedule.Continuous.CheckIntervalSecs, sendMessage)
		}
	case ScheduleModePeriodic:
		if schedule.Periodic != nil {
			go be.runPeriodic(ctx, agentID, agentName, schedule.Periodic.CronExpression, sendMessage)
		}
	case ScheduleModeProactive:
		if schedule.Proactive != nil {
			go be.runProactive(ctx, agentID, agentName, schedule.Proactive.TriggerPatterns, sendMessage)
		}
	case ScheduleModeReactive:
	default:
		cancel()
		delete(be.tasks, agentID)
		return fmt.Errorf("unknown schedule mode: %s", schedule.Mode)
	}

	return nil
}

// StopAgent stops the background task for an agent.
func (be *BackgroundExecutor) StopAgent(agentID string) error {
	be.mu.Lock()
	defer be.mu.Unlock()

	cancel, exists := be.tasks[agentID]
	if !exists {
		return fmt.Errorf("agent %s has no background task running", agentID)
	}

	cancel()
	delete(be.tasks, agentID)
	return nil
}

// StopAll stops all background tasks.
func (be *BackgroundExecutor) StopAll() {
	be.mu.Lock()
	defer be.mu.Unlock()

	for agentID, cancel := range be.tasks {
		cancel()
		delete(be.tasks, agentID)
	}

	be.shutdownCancel()
}

// IsRunning checks if an agent has a background task running.
func (be *BackgroundExecutor) IsRunning(agentID string) bool {
	be.mu.RLock()
	defer be.mu.RUnlock()

	_, exists := be.tasks[agentID]
	return exists
}

// ListRunning returns a list of agents with running background tasks.
func (be *BackgroundExecutor) ListRunning() []string {
	be.mu.RLock()
	defer be.mu.RUnlock()

	agentIDs := make([]string, 0, len(be.tasks))
	for agentID := range be.tasks {
		agentIDs = append(agentIDs, agentID)
	}
	return agentIDs
}

// AcquireLLM acquires a slot for LLM execution.
func (be *BackgroundExecutor) AcquireLLM() {
	be.llmSemaphore <- struct{}{}
}

// ReleaseLLM releases a slot for LLM execution.
func (be *BackgroundExecutor) ReleaseLLM() {
	<-be.llmSemaphore
}

// runContinuous runs the continuous mode background loop.
func (be *BackgroundExecutor) runContinuous(ctx context.Context, agentID, agentName string, intervalSecs int, sendMessage SendMessageFunc) {
	if intervalSecs <= 0 {
		intervalSecs = 60
	}

	ticker := time.NewTicker(time.Duration(intervalSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			be.AcquireLLM()
			go func() {
				defer be.ReleaseLLM()
				sendMessage(agentID, "Continue with your autonomous task")
			}()
		}
	}
}

// runPeriodic runs the periodic mode background loop.
func (be *BackgroundExecutor) runPeriodic(ctx context.Context, agentID, agentName, cronExpr string, sendMessage SendMessageFunc) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			be.AcquireLLM()
			go func() {
				defer be.ReleaseLLM()
				sendMessage(agentID, "Time to wake up and perform your scheduled task")
			}()
		}
	}
}

// runProactive runs the proactive mode background loop.
func (be *BackgroundExecutor) runProactive(ctx context.Context, agentID, agentName string, triggerPatterns []string, sendMessage SendMessageFunc) {
	<-ctx.Done()
}
