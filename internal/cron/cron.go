// Package cron provides cron scheduling functionality for OpenFang.
package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// CronJob represents a scheduled cron job.
type CronJob struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schedule    string                 `json:"schedule"`
	AgentID     string                 `json:"agent_id,omitempty"`
	HandID      string                 `json:"hand_id,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastRun     *time.Time             `json:"last_run,omitempty"`
	NextRun     *time.Time             `json:"next_run,omitempty"`
}

// CronJobStatus represents the status of a cron job execution.
type CronJobStatus string

const (
	CronJobStatusPending CronJobStatus = "pending"
	CronJobStatusRunning CronJobStatus = "running"
	CronJobStatusSuccess CronJobStatus = "success"
	CronJobStatusFailed  CronJobStatus = "failed"
)

// CronJobExecution represents a single execution of a cron job.
type CronJobExecution struct {
	ID        string        `json:"id"`
	JobID     string        `json:"job_id"`
	Status    CronJobStatus `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Error     string        `json:"error,omitempty"`
	Output    string        `json:"output,omitempty"`
}

// JobHandler is a function that handles cron job execution.
type JobHandler func(job *CronJob) error

// CronScheduler manages cron jobs.
type CronScheduler struct {
	mu         sync.RWMutex
	jobs       map[string]*CronJob
	executions map[string]*CronJobExecution
	cron       *cron.Cron
	entries    map[string]cron.EntryID
	handler    JobHandler
	persistDir string
	running    bool
}

// NewCronScheduler creates a new cron scheduler.
func NewCronScheduler(persistDir string, handler JobHandler) *CronScheduler {
	return &CronScheduler{
		jobs:       make(map[string]*CronJob),
		executions: make(map[string]*CronJobExecution),
		cron:       cron.New(),
		entries:    make(map[string]cron.EntryID),
		handler:    handler,
		persistDir: persistDir,
	}
}

// Start starts the cron scheduler.
func (cs *CronScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.running {
		return nil
	}

	if err := cs.load(); err != nil {
		return err
	}

	for _, job := range cs.jobs {
		if job.Enabled {
			cs.scheduleJob(job)
		}
	}

	cs.cron.Start()
	cs.running = true
	return nil
}

// Stop stops the cron scheduler.
func (cs *CronScheduler) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.running {
		return
	}

	cs.cron.Stop()
	cs.running = false
}

// AddJob adds a new cron job.
func (cs *CronScheduler) AddJob(job *CronJob) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	job.Enabled = true

	cs.jobs[job.ID] = job

	if cs.running && job.Enabled {
		cs.scheduleJob(job)
	}

	if err := cs.save(); err != nil {
		return err
	}

	return nil
}

// GetJob gets a cron job by ID.
func (cs *CronScheduler) GetJob(id string) (*CronJob, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	job, ok := cs.jobs[id]
	return job, ok
}

// ListJobs lists all cron jobs.
func (cs *CronScheduler) ListJobs() []*CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	jobs := make([]*CronJob, 0, len(cs.jobs))
	for _, job := range cs.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// UpdateJob updates a cron job.
func (cs *CronScheduler) UpdateJob(job *CronJob) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	existing, ok := cs.jobs[job.ID]
	if !ok {
		return fmt.Errorf("job not found")
	}

	if entryID, ok := cs.entries[job.ID]; ok {
		cs.cron.Remove(entryID)
		delete(cs.entries, job.ID)
	}

	job.CreatedAt = existing.CreatedAt
	job.UpdatedAt = time.Now()
	cs.jobs[job.ID] = job

	if cs.running && job.Enabled {
		cs.scheduleJob(job)
	}

	if err := cs.save(); err != nil {
		return err
	}

	return nil
}

// DeleteJob deletes a cron job.
func (cs *CronScheduler) DeleteJob(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if entryID, ok := cs.entries[id]; ok {
		cs.cron.Remove(entryID)
		delete(cs.entries, id)
	}

	delete(cs.jobs, id)

	if err := cs.save(); err != nil {
		return err
	}

	return nil
}

// EnableJob enables a cron job.
func (cs *CronScheduler) EnableJob(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	job, ok := cs.jobs[id]
	if !ok {
		return fmt.Errorf("job not found")
	}

	job.Enabled = true
	job.UpdatedAt = time.Now()

	if cs.running {
		cs.scheduleJob(job)
	}

	if err := cs.save(); err != nil {
		return err
	}

	return nil
}

// DisableJob disables a cron job.
func (cs *CronScheduler) DisableJob(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	job, ok := cs.jobs[id]
	if !ok {
		return fmt.Errorf("job not found")
	}

	job.Enabled = false
	job.UpdatedAt = time.Now()

	if entryID, ok := cs.entries[id]; ok {
		cs.cron.Remove(entryID)
		delete(cs.entries, id)
	}

	if err := cs.save(); err != nil {
		return err
	}

	return nil
}

// RunJob runs a cron job immediately.
func (cs *CronScheduler) RunJob(id string) error {
	cs.mu.RLock()
	job, ok := cs.jobs[id]
	if !ok {
		cs.mu.RUnlock()
		return fmt.Errorf("job not found")
	}
	cs.mu.RUnlock()

	execution := &CronJobExecution{
		ID:        uuid.New().String(),
		JobID:     job.ID,
		Status:    CronJobStatusRunning,
		StartedAt: time.Now(),
	}

	cs.mu.Lock()
	cs.executions[execution.ID] = execution
	cs.mu.Unlock()

	now := time.Now()
	job.LastRun = &now
	cs.updateNextRun(job)

	if cs.handler != nil {
		err := cs.handler(job)
		endedAt := time.Now()
		execution.EndedAt = &endedAt
		job.UpdatedAt = time.Now()

		if err != nil {
			execution.Status = CronJobStatusFailed
			execution.Error = err.Error()
		} else {
			execution.Status = CronJobStatusSuccess
		}
	}

	if err := cs.save(); err != nil {
		return err
	}

	return nil
}

// ListExecutions lists all cron job executions.
func (cs *CronScheduler) ListExecutions() []*CronJobExecution {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	executions := make([]*CronJobExecution, 0, len(cs.executions))
	for _, execution := range cs.executions {
		executions = append(executions, execution)
	}
	return executions
}

// scheduleJob schedules a cron job.
func (cs *CronScheduler) scheduleJob(job *CronJob) {
	entryID, err := cs.cron.AddFunc(job.Schedule, func() {
		cs.RunJob(job.ID)
	})

	if err == nil {
		cs.entries[job.ID] = entryID
		cs.updateNextRun(job)
	}
}

// updateNextRun updates the next run time for a job.
func (cs *CronScheduler) updateNextRun(job *CronJob) {
	if entryID, ok := cs.entries[job.ID]; ok {
		entry := cs.cron.Entry(entryID)
		next := entry.Next
		job.NextRun = &next
	}
}

// save saves the cron jobs to disk.
func (cs *CronScheduler) save() error {
	if cs.persistDir == "" {
		return nil
	}

	if err := os.MkdirAll(cs.persistDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cs.jobs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(cs.persistDir, "cron_jobs.json"), data, 0644)
}

// load loads the cron jobs from disk.
func (cs *CronScheduler) load() error {
	if cs.persistDir == "" {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(cs.persistDir, "cron_jobs.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var jobs map[string]*CronJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}

	cs.jobs = jobs
	return nil
}
