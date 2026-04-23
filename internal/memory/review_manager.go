package memory

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ReviewTask is the interface that all review tasks must implement
type ReviewTask interface {
	// Name returns the name of the task
	Name() string
	// Interval returns the interval at which the task should run
	Interval() time.Duration
	// Run executes the task
	Run(ctx context.Context) error
}

// ReviewManager manages background review tasks
type ReviewManager struct {
	mu          sync.RWMutex
	tasks       map[string]ReviewTask
	cancelFuncs map[string]context.CancelFunc
	isRunning   bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewReviewManager creates a new ReviewManager
func NewReviewManager() *ReviewManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReviewManager{
		tasks:       make(map[string]ReviewTask),
		cancelFuncs: make(map[string]context.CancelFunc),
		isRunning:   false,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// AddTask adds a review task to the manager
func (rm *ReviewManager) AddTask(task ReviewTask) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	name := task.Name()
	if _, exists := rm.tasks[name]; exists {
		return fmt.Errorf("task with name '%s' already exists", name)
	}

	rm.tasks[name] = task

	// If manager is running, start the task immediately
	if rm.isRunning {
		rm.startTask(task)
	}

	log.Printf("Review task '%s' added (interval: %v)", name, task.Interval())
	return nil
}

// Start starts all review tasks
func (rm *ReviewManager) Start() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isRunning {
		log.Println("ReviewManager already running")
		return
	}

	log.Println("Starting ReviewManager...")
	rm.isRunning = true

	for _, task := range rm.tasks {
		rm.startTask(task)
	}

	log.Printf("ReviewManager started with %d tasks", len(rm.tasks))
}

// Stop stops all review tasks
func (rm *ReviewManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.isRunning {
		log.Println("ReviewManager not running")
		return
	}

	log.Println("Stopping ReviewManager...")

	// Cancel all running tasks
	for name, cancelFunc := range rm.cancelFuncs {
		cancelFunc()
		delete(rm.cancelFuncs, name)
		log.Printf("Task '%s' stopped", name)
	}

	// Cancel the manager context
	rm.cancel()

	rm.isRunning = false
	log.Println("ReviewManager stopped")
}

// startTask starts a single task in a goroutine
func (rm *ReviewManager) startTask(task ReviewTask) {
	name := task.Name()

	// Create a context for this task
	taskCtx, cancel := context.WithCancel(rm.ctx)
	rm.cancelFuncs[name] = cancel

	// Start the task in a goroutine
	go func() {
		ticker := time.NewTicker(task.Interval())
		defer ticker.Stop()

		log.Printf("Task '%s' started (interval: %v)", name, task.Interval())

		// Run immediately on start
		if err := task.Run(taskCtx); err != nil {
			log.Printf("Task '%s' initial run failed: %v", name, err)
		}

		for {
			select {
			case <-ticker.C:
				log.Printf("Running task '%s'...", name)
				if err := task.Run(taskCtx); err != nil {
					log.Printf("Task '%s' failed: %v", name, err)
				}
			case <-taskCtx.Done():
				log.Printf("Task '%s' stopped", name)
				return
			}
		}
	}()
}

// IsRunning returns whether the ReviewManager is running
func (rm *ReviewManager) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.isRunning
}

// ListTasks returns all registered task names
func (rm *ReviewManager) ListTasks() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	names := make([]string, 0, len(rm.tasks))
	for name := range rm.tasks {
		names = append(names, name)
	}
	return names
}
