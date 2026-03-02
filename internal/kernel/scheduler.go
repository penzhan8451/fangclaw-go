// Package kernel implements the OpenFang kernel core.
package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a scheduled task.
type Task struct {
	ID       string
	Interval time.Duration
	Handler  func(context.Context)
	StopChan chan struct{}
}

// Scheduler implements a task scheduler.
type Scheduler struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

// NewScheduler creates a new scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks: make(map[string]*Task),
	}
}

// Schedule schedules a task to run at regular intervals.
func (s *Scheduler) Schedule(id string, interval time.Duration, handler func(context.Context)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with id %s already exists", id)
	}

	task := &Task{
		ID:       id,
		Interval: interval,
		Handler:  handler,
		StopChan: make(chan struct{}),
	}

	s.tasks[id] = task
	go s.runTask(task)

	return nil
}

// ScheduleOnce schedules a task to run once after a delay.
func (s *Scheduler) ScheduleOnce(id string, delay time.Duration, handler func(context.Context)) {
	go func() {
		time.Sleep(delay)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		handler(ctx)
	}()
}

// runTask runs a scheduled task.
func (s *Scheduler) runTask(task *Task) {
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), task.Interval)
			task.Handler(ctx)
			cancel()
		case <-task.StopChan:
			return
		}
	}
}

// Cancel cancels a scheduled task.
func (s *Scheduler) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	close(task.StopChan)
	delete(s.tasks, id)

	return nil
}

// List lists all scheduled tasks.
func (s *Scheduler) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		ids = append(ids, id)
	}
	return ids
}

// Shutdown shuts down the scheduler and cancels all tasks.
func (s *Scheduler) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		close(task.StopChan)
	}

	s.tasks = make(map[string]*Task)
}
