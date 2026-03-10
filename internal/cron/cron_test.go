package cron

import (
	"os"
	"testing"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/stretchr/testify/assert"
)

func makeTestJob(agentID types.AgentID) types.CronJob {
	return types.NewCronJob(
		agentID,
		"test-job",
		true,
		types.NewCronScheduleEvery(3600),
		types.NewCronActionSystemEvent("ping"),
		types.NewCronDeliveryNone(),
	)
}

func makeTestScheduler(t *testing.T) (*CronScheduler, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "cron-test-*")
	assert.NoError(t, err)
	scheduler := NewCronScheduler(tmpDir, 100)
	return scheduler, tmpDir
}

func TestAddJobAndList(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()
	job := makeTestJob(agentID)

	id, err := scheduler.AddJob(job, false)
	assert.NoError(t, err)

	jobs := scheduler.ListJobs(agentID)
	assert.Len(t, jobs, 1)
	assert.Equal(t, jobs[0].ID, id)
	assert.Equal(t, jobs[0].Name, "test-job")

	allJobs := scheduler.ListAllJobs()
	assert.Len(t, allJobs, 1)

	fetchedJob := scheduler.GetJob(id)
	assert.NotNil(t, fetchedJob)
	assert.Equal(t, fetchedJob.AgentID, agentID)

	assert.NotNil(t, fetchedJob.NextRun)
	assert.Equal(t, scheduler.TotalJobs(), 1)
}

func TestRemoveJob(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()
	job := makeTestJob(agentID)
	id, err := scheduler.AddJob(job, false)
	assert.NoError(t, err)

	removed, err := scheduler.RemoveJob(id)
	assert.NoError(t, err)
	assert.Equal(t, removed.Name, "test-job")
	assert.Equal(t, scheduler.TotalJobs(), 0)

	_, err = scheduler.RemoveJob(id)
	assert.Error(t, err)
}

func TestAddJobGlobalLimit(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()
	scheduler.SetMaxTotalJobs(2)

	j1 := makeTestJob(agentID)
	j2 := makeTestJob(agentID)
	j3 := makeTestJob(agentID)

	_, err := scheduler.AddJob(j1, false)
	assert.NoError(t, err)
	_, err = scheduler.AddJob(j2, false)
	assert.NoError(t, err)

	_, err = scheduler.AddJob(j3, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "limit")
}

func TestRecordSuccessRemovesOneShot(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()
	job := makeTestJob(agentID)
	id, err := scheduler.AddJob(job, true)
	assert.NoError(t, err)
	assert.Equal(t, scheduler.TotalJobs(), 1)

	scheduler.RecordSuccess(id)
	assert.Equal(t, scheduler.TotalJobs(), 0)
	assert.Nil(t, scheduler.GetJob(id))
}

func TestRecordSuccessKeepsRecurring(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()
	job := makeTestJob(agentID)
	id, err := scheduler.AddJob(job, false)
	assert.NoError(t, err)

	scheduler.RecordSuccess(id)
	assert.Equal(t, scheduler.TotalJobs(), 1)
	meta := scheduler.GetMeta(id)
	assert.NotNil(t, meta)
	assert.Equal(t, *meta.LastStatus, "ok")
	assert.Equal(t, meta.ConsecutiveErrors, uint32(0))
	assert.NotNil(t, meta.Job.LastRun)
}

func TestRecordFailureAutoDisable(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()
	job := makeTestJob(agentID)
	id, err := scheduler.AddJob(job, false)
	assert.NoError(t, err)

	for i := 0; i < MAX_CONSECUTIVE_ERRORS-1; i++ {
		scheduler.RecordFailure(id, "error")
		meta := scheduler.GetMeta(id)
		assert.NotNil(t, meta)
		assert.True(t, meta.Job.Enabled)
		assert.Equal(t, meta.ConsecutiveErrors, uint32(i+1))
	}

	scheduler.RecordFailure(id, "final error")
	meta := scheduler.GetMeta(id)
	assert.NotNil(t, meta)
	assert.False(t, meta.Job.Enabled)
	assert.Equal(t, meta.ConsecutiveErrors, uint32(MAX_CONSECUTIVE_ERRORS))
	assert.Contains(t, *meta.LastStatus, "error:")
}

func TestDueJobsOnlyEnabled(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()

	j1 := makeTestJob(agentID)
	j1.Name = "enabled-due"
	id1, err := scheduler.AddJob(j1, false)
	assert.NoError(t, err)

	j2 := makeTestJob(agentID)
	j2.Name = "disabled-job"
	id2, err := scheduler.AddJob(j2, false)
	assert.NoError(t, err)
	err = scheduler.SetEnabled(id2, false)
	assert.NoError(t, err)

	scheduler.mu.Lock()
	pastTime := time.Now().UTC().Add(-10 * time.Second)
	meta1 := scheduler.jobs[id1]
	meta1.Job.NextRun = &pastTime
	scheduler.jobs[id1] = meta1
	meta2 := scheduler.jobs[id2]
	meta2.Job.NextRun = &pastTime
	scheduler.jobs[id2] = meta2
	scheduler.mu.Unlock()

	due := scheduler.DueJobs()
	assert.Len(t, due, 1)
	assert.Equal(t, due[0].Name, "enabled-due")
}

func TestSetEnabled(t *testing.T) {
	scheduler, tmpDir := makeTestScheduler(t)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()

	job := makeTestJob(agentID)
	id, err := scheduler.AddJob(job, false)
	assert.NoError(t, err)

	err = scheduler.SetEnabled(id, false)
	assert.NoError(t, err)
	meta := scheduler.GetMeta(id)
	assert.NotNil(t, meta)
	assert.False(t, meta.Job.Enabled)

	err = scheduler.SetEnabled(id, true)
	assert.NoError(t, err)
	meta = scheduler.GetMeta(id)
	assert.NotNil(t, meta)
	assert.True(t, meta.Job.Enabled)
	assert.Equal(t, meta.ConsecutiveErrors, uint32(0))
	assert.NotNil(t, meta.Job.NextRun)

	fakeID := types.NewCronJobID()
	err = scheduler.SetEnabled(fakeID, true)
	assert.Error(t, err)
}

func TestPersistAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cron-persist-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	agentID := types.NewAgentID()

	{
		scheduler := NewCronScheduler(tmpDir, 100)
		j1 := makeTestJob(agentID)
		j1.Name = "persist-a"
		j2 := makeTestJob(agentID)
		j2.Name = "persist-b"

		_, err := scheduler.AddJob(j1, false)
		assert.NoError(t, err)
		_, err = scheduler.AddJob(j2, true)
		assert.NoError(t, err)

		err = scheduler.Persist()
		assert.NoError(t, err)
	}

	{
		scheduler := NewCronScheduler(tmpDir, 100)
		count, err := scheduler.Load()
		assert.NoError(t, err)
		assert.Equal(t, count, 2)
		assert.Equal(t, scheduler.TotalJobs(), 2)

		jobs := scheduler.ListJobs(agentID)
		assert.Len(t, jobs, 2)

		names := make([]string, 0, 2)
		for _, job := range jobs {
			names = append(names, job.Name)
		}
		assert.Contains(t, names, "persist-a")
		assert.Contains(t, names, "persist-b")

		var bID types.CronJobID
		for _, job := range jobs {
			if job.Name == "persist-b" {
				bID = job.ID
			}
		}
		assert.NotEmpty(t, bID)
		meta := scheduler.GetMeta(bID)
		assert.NotNil(t, meta)
		assert.True(t, meta.OneShot)
	}
}

func TestLoadNoFileReturnsZero(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cron-empty-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	scheduler := NewCronScheduler(tmpDir, 100)
	count, err := scheduler.Load()
	assert.NoError(t, err)
	assert.Equal(t, count, 0)
}

func TestComputeNextRunAt(t *testing.T) {
	target := time.Now().UTC().Add(2 * time.Hour)
	schedule := types.NewCronScheduleAt(target)
	next := ComputeNextRun(&schedule)
	assert.Equal(t, next, target)
}

func TestComputeNextRunEvery(t *testing.T) {
	before := time.Now().UTC()
	schedule := types.NewCronScheduleEvery(300)
	next := ComputeNextRun(&schedule)
	after := time.Now().UTC()

	assert.True(t, next.After(before.Add(299*time.Second)) || next.Equal(before.Add(299*time.Second)))
	assert.True(t, next.Before(after.Add(301*time.Second)) || next.Equal(after.Add(301*time.Second)))
}

func TestComputeNextRunCronDaily(t *testing.T) {
	now := time.Now().UTC()
	schedule := types.NewCronScheduleCron("0 9 * * *", nil)
	next := ComputeNextRun(&schedule)

	assert.True(t, next.After(now))
	assert.True(t, next.Before(now.Add(24*time.Hour)))
	assert.Equal(t, next.Minute(), 0)
	assert.Equal(t, next.Hour(), 9)
}

func TestComputeNextRunCronInvalidExpr(t *testing.T) {
	now := time.Now().UTC()
	schedule := types.NewCronScheduleCron("not a cron", nil)
	next := ComputeNextRun(&schedule)
	assert.True(t, next.After(now.Add(59*time.Minute)))
	assert.True(t, next.Before(now.Add(61*time.Minute)))
}
