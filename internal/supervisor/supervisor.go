// Package supervisor provides process supervision for OpenFang.
package supervisor

import (
	"sync"
	"sync/atomic"
)

// Supervisor manages shutdown signals and health monitoring.
type Supervisor struct {
	shutdownChan   chan struct{}
	shutdownOnce   sync.Once
	restartCount   atomic.Uint64
	panicCount     atomic.Uint64
	agentRestarts  sync.Map
	mu             sync.RWMutex
}

// NewSupervisor creates a new supervisor.
func NewSupervisor() *Supervisor {
	return &Supervisor{
		shutdownChan: make(chan struct{}),
	}
}

// ShutdownChan returns a channel that will be closed when shutdown is requested.
func (s *Supervisor) ShutdownChan() <-chan struct{} {
	return s.shutdownChan
}

// Shutdown triggers a graceful shutdown.
func (s *Supervisor) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownChan)
	})
}

// IsShuttingDown checks if shutdown has been requested.
func (s *Supervisor) IsShuttingDown() bool {
	select {
	case <-s.shutdownChan:
		return true
	default:
		return false
	}
}

// RecordPanic records that a panic was caught during agent execution.
func (s *Supervisor) RecordPanic() {
	s.panicCount.Add(1)
}

// GetPanicCount returns the total number of panics recorded.
func (s *Supervisor) GetPanicCount() uint64 {
	return s.panicCount.Load()
}

// RecordRestart records an agent restart.
func (s *Supervisor) RecordRestart(agentID string) {
	s.restartCount.Add(1)
	
	val, _ := s.agentRestarts.LoadOrStore(agentID, uint32(0))
	count := val.(uint32)
	s.agentRestarts.Store(agentID, count+1)
}

// GetRestartCount returns the total number of restarts recorded.
func (s *Supervisor) GetRestartCount() uint64 {
	return s.restartCount.Load()
}

// GetAgentRestartCount returns the number of restarts for a specific agent.
func (s *Supervisor) GetAgentRestartCount(agentID string) uint32 {
	val, ok := s.agentRestarts.Load(agentID)
	if !ok {
		return 0
	}
	return val.(uint32)
}

// ResetAgentRestarts resets the restart count for a specific agent.
func (s *Supervisor) ResetAgentRestarts(agentID string) {
	s.agentRestarts.Store(agentID, uint32(0))
}

// ResetAll resets all counters.
func (s *Supervisor) ResetAll() {
	s.restartCount.Store(0)
	s.panicCount.Store(0)
	s.agentRestarts = sync.Map{}
}
