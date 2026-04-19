package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

type WorkflowRun struct {
	ID        string
	Name      string
	StartedAt time.Time
	EndedAt   *time.Time
	Status    string
	Output    string
}

type WorkflowTracker struct {
	mu     sync.RWMutex
	runs   map[string]*WorkflowRun
	counts map[string]int
}

func NewWorkflowTracker() *WorkflowTracker {
	return &WorkflowTracker{
		runs:   make(map[string]*WorkflowRun),
		counts: make(map[string]int),
	}
}

func (wt *WorkflowTracker) TrackStart(event *eventbus.Event) {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	workflowID, _ := event.Payload["workflow_id"].(string)
	workflowName, _ := event.Payload["workflow_name"].(string)

	run := &WorkflowRun{
		ID:        workflowID,
		Name:      workflowName,
		StartedAt: event.Timestamp,
		Status:    "running",
	}
	wt.runs[workflowID] = run
	wt.counts["started"]++

	fmt.Printf("🚀 Workflow Started: %s (ID: %s)\n", workflowName, workflowID)
}

func (wt *WorkflowTracker) TrackComplete(event *eventbus.Event) {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	workflowID, _ := event.Payload["workflow_id"].(string)
	output, _ := event.Payload["output"].(string)

	if run, exists := wt.runs[workflowID]; exists {
		now := time.Now()
		run.EndedAt = &now
		run.Status = "completed"
		run.Output = output
		wt.counts["completed"]++

		duration := run.EndedAt.Sub(run.StartedAt)
		fmt.Printf("✅ Workflow Completed: %s (Duration: %v)\n", run.Name, duration.Round(time.Millisecond))
	}
}

func (wt *WorkflowTracker) PrintReport() {
	wt.mu.RLock()
	defer wt.mu.RUnlock()

	fmt.Println()
	fmt.Println("=== Workflow Execution Report ===")
	fmt.Printf("Total Started:    %d\n", wt.counts["started"])
	fmt.Printf("Total Completed:  %d\n", wt.counts["completed"])
	fmt.Println()
	fmt.Println("Recent Workflows:")
	for _, run := range wt.runs {
		status := run.Status
		if run.EndedAt != nil {
			duration := run.EndedAt.Sub(run.StartedAt)
			fmt.Printf("  - %s [%s]: %v\n", run.Name, status, duration.Round(time.Millisecond))
		} else {
			fmt.Printf("  - %s [%s]: Running for %v\n", run.Name, status, time.Since(run.StartedAt).Round(time.Millisecond))
		}
	}
}

func main() {
	eb := eventbus.NewEventBus()
	tracker := NewWorkflowTracker()

	eb.Subscribe(eventbus.EventTypeWorkflowStarted, tracker.TrackStart)
	eb.Subscribe(eventbus.EventTypeWorkflowCompleted, tracker.TrackComplete)

	fmt.Println("Workflow Tracker started")
	fmt.Println("Monitoring workflow events...")
	fmt.Println()

	simulateWorkflows(eb)

	tracker.PrintReport()
}

func simulateWorkflows(eb *eventbus.EventBus) {
	workflows := []struct {
		id   string
		name string
	}{
		{"wf-001", "Data Processing Pipeline"},
		{"wf-002", "Content Generation"},
		{"wf-003", "Report Generation"},
	}

	for _, wf := range workflows {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeWorkflowStarted,
			"simulator",
			eventbus.EventTargetSystem,
		).WithPayload(map[string]interface{}{
			"workflow_id":   wf.id,
			"workflow_name": wf.name,
			"input":         "sample input",
		}))

		time.Sleep(200 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeWorkflowCompleted,
			"simulator",
			eventbus.EventTargetSystem,
		).WithPayload(map[string]interface{}{
			"workflow_id":   wf.id,
			"workflow_name": wf.name,
			"output":        "Workflow completed successfully",
		}))

		time.Sleep(100 * time.Millisecond)
	}
}
