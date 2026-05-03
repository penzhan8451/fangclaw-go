package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

const (
	MAX_CONSECUTIVE_ERRORS = 5
)

var (
	globalScheduler *CronScheduler
	schedulerMu     sync.RWMutex
)

func SetGlobalScheduler(s *CronScheduler) {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()
	globalScheduler = s
}

func GetGlobalScheduler() *CronScheduler {
	schedulerMu.RLock()
	defer schedulerMu.RUnlock()
	return globalScheduler
}

type JobMeta struct {
	Job               types.CronJob `json:"job"`
	OneShot           bool          `json:"one_shot"`
	LastStatus        *string       `json:"last_status,omitempty"`
	ConsecutiveErrors uint32        `json:"consecutive_errors"`
}

func NewJobMeta(job types.CronJob, oneShot bool) JobMeta {
	return JobMeta{
		Job:               job,
		OneShot:           oneShot,
		LastStatus:        nil,
		ConsecutiveErrors: 0,
	}
}

type CronScheduler struct {
	mu           sync.RWMutex
	jobs         map[types.CronJobID]JobMeta
	persistPath  string
	maxTotalJobs *atomic.Uint32
	watcher      struct {
		mu       sync.Mutex
		stopChan chan struct{}
		running  bool
	}
}

func NewCronScheduler(homeDir string, maxTotalJobs uint32) *CronScheduler {
	scheduler := &CronScheduler{
		jobs:         make(map[types.CronJobID]JobMeta),
		persistPath:  filepath.Join(homeDir, "cron_jobs.json"),
		maxTotalJobs: &atomic.Uint32{},
	}
	scheduler.maxTotalJobs.Store(maxTotalJobs)
	return scheduler
}

func (cs *CronScheduler) SetMaxTotalJobs(newMax uint32) {
	cs.maxTotalJobs.Store(newMax)
}

func (cs *CronScheduler) Load() (int, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, err := os.Stat(cs.persistPath); os.IsNotExist(err) {
		return 0, nil
	}

	data, err := os.ReadFile(cs.persistPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read cron jobs file, starting with empty")
		return 0, nil
	}

	// 检查文件是否为空或只有空白字符
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		log.Info().Msg("Cron jobs file is empty, starting with empty")
		return 0, nil
	}

	// 检查是否是空数组 []
	if trimmed == "[]" {
		log.Info().Msg("Cron jobs file has empty array, starting with empty")
		return 0, nil
	}

	var metas []JobMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		log.Warn().Err(err).Str("file_content", string(data)).Msg("Failed to parse cron jobs file, backing up and starting fresh")
		backupPath := cs.persistPath + ".corrupted." + time.Now().Format("20060102-150405")
		if err := os.Rename(cs.persistPath, backupPath); err != nil {
			log.Warn().Err(err).Str("backup_path", backupPath).Msg("Failed to backup corrupted cron jobs file")
		} else {
			log.Info().Str("backup_path", backupPath).Msg("Backed up corrupted cron jobs file")
		}
		return 0, nil
	}

	newJobs := make(map[types.CronJobID]JobMeta)
	count := len(metas)
	for _, meta := range metas {
		newJobs[meta.Job.ID] = meta
	}

	cs.jobs = newJobs

	log.Info().Int("count", count).Msg("Loaded cron jobs from disk")
	return count, nil
}

func (cs *CronScheduler) Persist() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	metas := make([]JobMeta, 0, len(cs.jobs))
	for _, meta := range cs.jobs {
		metas = append(metas, meta)
	}

	data, err := json.MarshalIndent(metas, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize cron jobs")
		return fmt.Errorf("failed to serialize cron jobs: %w", err)
	}

	dir := filepath.Dir(cs.persistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("Failed to create directory for cron jobs")
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(cs.persistPath, data, 0644); err != nil {
		log.Error().Err(err).Str("path", cs.persistPath).Msg("Failed to write cron jobs file")
		return fmt.Errorf("failed to write cron jobs file: %w", err)
	}

	log.Info().Int("count", len(metas)).Str("path", cs.persistPath).Msg("Persisted cron jobs successfully")
	return nil
}

func (cs *CronScheduler) AddJob(job types.CronJob, oneShot bool) (types.CronJobID, error) {
	cs.mu.Lock()

	log.Info().Str("job_name", job.Name).Str("agent_id", job.AgentID.String()).Msg("[AddJob]-Cron: adding new job")

	maxJobs := cs.maxTotalJobs.Load()
	if uint32(len(cs.jobs)) >= maxJobs {
		cs.mu.Unlock()
		return "", fmt.Errorf("global cron job limit reached (%d)", maxJobs)
	}

	agentCount := 0
	for _, meta := range cs.jobs {
		if meta.Job.AgentID == job.AgentID {
			agentCount++
		}
	}

	if err := job.Validate(agentCount); err != nil {
		cs.mu.Unlock()
		return "", err
	}

	nextRun := ComputeNextRun(&job.Schedule)
	job.NextRun = &nextRun

	log.Info().Str("job_name", job.Name).Time("next_run", nextRun).Msg("Cron: computed next run time")

	id := job.ID
	cs.jobs[id] = NewJobMeta(job, oneShot)

	log.Info().Str("job_id", id.String()).Str("job_name", job.Name).Int("total_jobs", len(cs.jobs)).Msg("Cron: job added successfully")

	cs.mu.Unlock()

	if err := cs.Persist(); err != nil {
		log.Warn().Err(err).Msg("Failed to persist after adding job")
	} else {
		log.Info().Str("job_id", id.String()).Msg("Cron: job persisted to disk")
	}

	return id, nil
}

func (cs *CronScheduler) RemoveJob(id types.CronJobID) (types.CronJob, error) {
	cs.mu.Lock()

	meta, exists := cs.jobs[id]
	if !exists {
		cs.mu.Unlock()
		return types.CronJob{}, fmt.Errorf("cron job %s not found", id)
	}

	delete(cs.jobs, id)

	cs.mu.Unlock()

	if err := cs.Persist(); err != nil {
		log.Warn().Err(err).Msg("Failed to persist after removing job")
	}

	return meta.Job, nil
}

func (cs *CronScheduler) SetEnabled(id types.CronJobID, enabled bool) error {
	cs.mu.Lock()

	meta, exists := cs.jobs[id]
	if !exists {
		cs.mu.Unlock()
		return fmt.Errorf("cron job %s not found", id)
	}

	meta.Job.Enabled = enabled
	if enabled {
		meta.ConsecutiveErrors = 0
		nextRun := ComputeNextRun(&meta.Job.Schedule)
		meta.Job.NextRun = &nextRun
	}

	cs.jobs[id] = meta

	cs.mu.Unlock()

	if err := cs.Persist(); err != nil {
		log.Warn().Err(err).Msg("Failed to persist after setting enabled")
	}

	return nil
}

func (cs *CronScheduler) UpdateJob(id types.CronJobID, job types.CronJob) error {
	cs.mu.Lock()

	_, exists := cs.jobs[id]
	if !exists {
		cs.mu.Unlock()
		return fmt.Errorf("cron job %s not found", id)
	}

	agentCount := 0
	for _, meta := range cs.jobs {
		if meta.Job.AgentID == job.AgentID && meta.Job.ID != id {
			agentCount++
		}
	}

	if err := job.Validate(agentCount); err != nil {
		cs.mu.Unlock()
		return err
	}

	nextRun := ComputeNextRun(&job.Schedule)
	job.NextRun = &nextRun
	job.ID = id

	oldMeta := cs.jobs[id]
	oldMeta.Job = job
	oldMeta.ConsecutiveErrors = 0
	cs.jobs[id] = oldMeta

	log.Info().Str("job_id", id.String()).Str("job_name", job.Name).Msg("Cron: job updated successfully")

	cs.mu.Unlock()

	if err := cs.Persist(); err != nil {
		log.Warn().Err(err).Msg("Failed to persist after updating job")
	} else {
		log.Info().Str("job_id", id.String()).Msg("Cron: job persisted to disk")
	}

	return nil
}

func (cs *CronScheduler) GetJob(id types.CronJobID) *types.CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	meta, exists := cs.jobs[id]
	if !exists {
		return nil
	}
	job := meta.Job
	return &job
}

func (cs *CronScheduler) GetMeta(id types.CronJobID) *JobMeta {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	meta, exists := cs.jobs[id]
	if !exists {
		return nil
	}
	result := meta
	return &result
}

func (cs *CronScheduler) ListJobs(agentID types.AgentID) []types.CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var jobs []types.CronJob
	for _, meta := range cs.jobs {
		if meta.Job.AgentID == agentID {
			jobs = append(jobs, meta.Job)
		}
	}
	return jobs
}

func (cs *CronScheduler) TakeAgentJobs(agentID types.AgentID) []types.CronJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var jobs []types.CronJob
	for id, meta := range cs.jobs {
		if meta.Job.AgentID == agentID {
			jobs = append(jobs, meta.Job)
			delete(cs.jobs, id)
		}
	}
	return jobs
}

func (cs *CronScheduler) RemoveAgentJobs(agentID types.AgentID) int {
	cs.mu.Lock()
	count := 0
	for id, meta := range cs.jobs {

		if meta.Job.AgentID == agentID {
			log.Info().Str("job_id", id.String()).Msg("Cron: removing job")
			delete(cs.jobs, id)
			count++
		}
	}

	// 先释放写锁，再调用 Persist
	cs.mu.Unlock()

	if count > 0 {
		log.Info().Str("agent", agentID.String()).Int("count", count).Msg("Cron: removed jobs for agent")
		if err := cs.Persist(); err != nil {
			log.Warn().Err(err).Msg("Failed to persist after removing agent jobs")
		}
	}
	return count
}

func (cs *CronScheduler) SetAgentJobsEnabled(agentID types.AgentID, enabled bool) int {
	cs.mu.Lock()

	count := 0
	for id, meta := range cs.jobs {
		if meta.Job.AgentID == agentID {
			if meta.Job.Enabled != enabled {
				meta.Job.Enabled = enabled
				if enabled {
					meta.ConsecutiveErrors = 0
					nextRun := ComputeNextRun(&meta.Job.Schedule)
					meta.Job.NextRun = &nextRun
				}
				cs.jobs[id] = meta
				count++
			}
		}
	}

	// 先释放写锁，再调用 Persist
	cs.mu.Unlock()

	if count > 0 {
		if err := cs.Persist(); err != nil {
			log.Warn().Err(err).Msg("Failed to persist after setting agent jobs enabled")
		}
	}
	return count
}

func (cs *CronScheduler) ListAllJobs() []types.CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var jobs []types.CronJob
	for _, meta := range cs.jobs {
		jobs = append(jobs, meta.Job)
	}
	return jobs
}

func (cs *CronScheduler) TotalJobs() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return len(cs.jobs)
}

func (cs *CronScheduler) DueJobs() []types.CronJob {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now().UTC()
	var due []types.CronJob

	log.Info().Time("now", now).Int("total_jobs", len(cs.jobs)).Msg("Cron: checking due jobs")

	for id, meta := range cs.jobs {
		job := meta.Job
		log.Info().
			Str("job_id", id.String()).
			Str("job_name", job.Name).
			Bool("enabled", job.Enabled).
			Time("next_run", *job.NextRun).
			Bool("is_due", job.Enabled && job.NextRun != nil && !job.NextRun.After(now)).
			Msg("Cron: checking job")

		if job.Enabled && job.NextRun != nil && !job.NextRun.After(now) {
			due = append(due, job)
			nextRun := ComputeNextRunAfter(&job.Schedule, now)
			meta.Job.NextRun = &nextRun
			cs.jobs[id] = meta
			log.Info().Str("job_name", job.Name).Time("next_run", nextRun).Msg("Cron: job fired, updated next run")
		}
	}

	return due
}

func (cs *CronScheduler) RecordSuccess(id types.CronJobID) {
	cs.mu.Lock()

	meta, exists := cs.jobs[id]
	if !exists {
		cs.mu.Unlock()
		return
	}

	now := time.Now().UTC()
	meta.Job.LastRun = &now
	status := "ok"
	meta.LastStatus = &status
	meta.ConsecutiveErrors = 0

	var shouldPersist bool
	if meta.OneShot {
		delete(cs.jobs, id)
		shouldPersist = true
	} else {
		cs.jobs[id] = meta
		shouldPersist = true
	}

	cs.mu.Unlock()

	if shouldPersist {
		if err := cs.Persist(); err != nil {
			log.Warn().Err(err).Msg("Failed to persist after recording success")
		}
	}
}

func (cs *CronScheduler) RecordFailure(id types.CronJobID, errorMsg string) {
	cs.mu.Lock()

	meta, exists := cs.jobs[id]
	if !exists {
		cs.mu.Unlock()
		return
	}

	now := time.Now().UTC()
	meta.Job.LastRun = &now

	truncated := errorMsg
	if len(truncated) > 256 {
		truncated = truncated[:256]
	}
	status := fmt.Sprintf("error: %s", truncated)
	meta.LastStatus = &status
	meta.ConsecutiveErrors++

	if meta.ConsecutiveErrors >= MAX_CONSECUTIVE_ERRORS {
		log.Warn().Str("job_id", id.String()).Uint32("errors", meta.ConsecutiveErrors).Msg("Auto-disabling cron job after repeated failures")
		meta.Job.Enabled = false
	} else {
		nextRun := ComputeNextRunAfter(&meta.Job.Schedule, now)
		meta.Job.NextRun = &nextRun
	}

	cs.jobs[id] = meta

	cs.mu.Unlock()

	if err := cs.Persist(); err != nil {
		log.Warn().Err(err).Msg("Failed to persist after recording failure")
	}
}

func ComputeNextRun(schedule *types.CronSchedule) time.Time {
	return ComputeNextRunAfter(schedule, time.Now().UTC())
}

func ComputeNextRunAfter(schedule *types.CronSchedule, after time.Time) time.Time {
	switch schedule.Kind {
	case types.CronScheduleKindAt:
		if schedule.At != nil {
			return *schedule.At
		}
		return after.Add(1 * time.Hour)

	case types.CronScheduleKindEvery:
		if schedule.EverySecs != nil {
			return after.Add(time.Duration(*schedule.EverySecs) * time.Second)
		}
		return after.Add(1 * time.Hour)

	case types.CronScheduleKindCron:
		if schedule.Expr == nil {
			return after.Add(1 * time.Hour)
		}

		trimmed := strings.TrimSpace(*schedule.Expr)
		base := after.Add(1 * time.Second)

		sched, err := cron.ParseStandard(trimmed)
		if err != nil {
			log.Warn().Str("expr", *schedule.Expr).Err(err).Msg("Failed to parse cron expression")
			return after.Add(1 * time.Hour)
		}

		next := sched.Next(base)
		if next.IsZero() {
			return after.Add(1 * time.Hour)
		}
		return next

	default:
		return after.Add(1 * time.Hour)
	}
}

func (cs *CronScheduler) StartHotReload() {
	cs.watcher.mu.Lock()
	if cs.watcher.running {
		cs.watcher.mu.Unlock()
		return
	}

	cs.watcher.stopChan = make(chan struct{})
	cs.watcher.running = true
	cs.watcher.mu.Unlock()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		var lastModTime time.Time
		for {
			select {
			case <-ticker.C:
				info, err := os.Stat(cs.persistPath)
				if err == nil {
					modTime := info.ModTime()
					if modTime.After(lastModTime) {
						lastModTime = modTime
						log.Debug().Msg("Cron jobs file modified, reloading...")
						if _, err := cs.Load(); err != nil {
							log.Warn().Err(err).Msg("Failed to reload cron jobs")
						}
					}
				}
			case <-cs.watcher.stopChan:
				return
			}
		}
	}()

	log.Debug().Msg("Cron hot reload started")
}

func (cs *CronScheduler) StopHotReload() {
	cs.watcher.mu.Lock()
	defer cs.watcher.mu.Unlock()

	if !cs.watcher.running {
		return
	}

	close(cs.watcher.stopChan)
	cs.watcher.running = false

	log.Debug().Msg("Cron hot reload stopped")
}
