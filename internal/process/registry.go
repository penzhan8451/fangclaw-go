package process

import (
	"sync"
	"time"
)

type ProcessStatus string

const (
	PSRunning ProcessStatus = "running"
	PSStopped ProcessStatus = "stopped"
)

type ProcessEntry struct {
	ID             string
	Name           string
	Cmd            []string
	Env            map[string]string
	PID            int
	StartTime      time.Time
	EndTime        time.Time
	Status         ProcessStatus
	RestartOnCrash bool
	RestartCount   int
	RestartLimit   int
}

type ProcessRegistry struct {
	mu        sync.RWMutex
	processes map[string]*ProcessEntry
}

func NewProcessRegistry() *ProcessRegistry {
	return &ProcessRegistry{processes: make(map[string]*ProcessEntry)}
}

func (r *ProcessRegistry) get(id string) (*ProcessEntry, bool) {
	r.mu.RLock()
	p, ok := r.processes[id]
	r.mu.RUnlock()
	return p, ok
}

func (r *ProcessRegistry) set(p *ProcessEntry) {
	r.mu.Lock()
	r.processes[p.ID] = p
	r.mu.Unlock()
}

func (r *ProcessRegistry) list() []*ProcessEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ProcessEntry, 0, len(r.processes))
	for _, p := range r.processes {
		out = append(out, p)
	}
	return out
}
