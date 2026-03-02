// Package workflow provides workflow engine for multi-step agent pipeline execution.
package workflow

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowID represents a unique workflow identifier.
type WorkflowID uuid.UUID

// NewWorkflowID creates a new workflow ID.
func NewWorkflowID() WorkflowID {
	return WorkflowID(uuid.New())
}

// ParseWorkflowID parses a string into a WorkflowID.
func ParseWorkflowID(s string) (WorkflowID, error) {
	id, err := uuid.Parse(s)
	return WorkflowID(id), err
}

// WorkflowRunID represents a unique workflow execution instance.
type WorkflowRunID uuid.UUID

// NewWorkflowRunID creates a new workflow run ID.
func NewWorkflowRunID() WorkflowRunID {
	return WorkflowRunID(uuid.New())
}

// StepMode defines how a workflow step executes.
type StepMode string

const (
	StepModePipeline StepMode = "pipeline" // Sequential execution
	StepModeFanOut   StepMode = "fan_out"  // Parallel execution
)

// ErrorMode defines how errors are handled in a step.
type ErrorMode string

const (
	ErrorModeFail     ErrorMode = "fail"     // Stop on error
	ErrorModeContinue ErrorMode = "continue" // Continue despite error
	ErrorModeRetry    ErrorMode = "retry"    // Retry up to N times
)

// StepAgent defines which agent handles a workflow step.
type StepAgent struct {
	Type  string `json:"type"`  // "name" or "id"
	Value string `json:"value"` // Agent name or ID
}

// WorkflowStep represents a single step in a workflow.
type WorkflowStep struct {
	Name           string    `json:"name"`            // Step name for logging
	Agent          StepAgent `json:"agent"`           // Which agent to route to
	PromptTemplate string    `json:"prompt_template"` // Prompt with {{input}} or {{var_name}}
	Mode           StepMode  `json:"mode"`            // Execution mode
	TimeoutSecs    uint64    `json:"timeout_secs"`    // Max execution time
	ErrorMode      ErrorMode `json:"error_mode"`      // Error handling
	OutputVar      string    `json:"output_var"`      // Variable name to store output
}

// Workflow represents a named sequence of steps.
type Workflow struct {
	ID          WorkflowID     `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	CreatedAt   time.Time      `json:"created_at"`
}

// WorkflowRun represents a running workflow instance.
type WorkflowRun struct {
	ID         WorkflowRunID     `json:"id"`
	WorkflowID WorkflowID        `json:"workflow_id"`
	AgentID    string            `json:"agent_id"`
	State      string            `json:"state"` // pending, running, completed, failed
	Input      string            `json:"input"`
	Output     string            `json:"output"`
	Variables  map[string]string `json:"variables"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// WorkflowRegistry manages workflows.
type WorkflowRegistry struct {
	workflows map[WorkflowID]*Workflow
	runs      map[WorkflowRunID]*WorkflowRun
}

// NewWorkflowRegistry creates a new workflow registry.
func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		workflows: make(map[WorkflowID]*Workflow),
		runs:      make(map[WorkflowRunID]*WorkflowRun),
	}
}

// Add adds a workflow to the registry.
func (r *WorkflowRegistry) Add(w *Workflow) {
	r.workflows[w.ID] = w
}

// Get returns a workflow by ID.
func (r *WorkflowRegistry) Get(id WorkflowID) (*Workflow, bool) {
	w, ok := r.workflows[id]
	return w, ok
}

// List returns all workflows.
func (r *WorkflowRegistry) List() []*Workflow {
	workflows := make([]*Workflow, 0, len(r.workflows))
	for _, w := range r.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

// Remove removes a workflow by ID.
func (r *WorkflowRegistry) Remove(id WorkflowID) bool {
	if _, ok := r.workflows[id]; ok {
		delete(r.workflows, id)
		return true
	}
	return false
}

// AddRun adds a workflow run to the registry.
func (r *WorkflowRegistry) AddRun(run *WorkflowRun) {
	r.runs[run.ID] = run
}

// GetRun returns a workflow run by ID.
func (r *WorkflowRegistry) GetRun(id WorkflowRunID) (*WorkflowRun, bool) {
	run, ok := r.runs[id]
	return run, ok
}

// ListRuns returns all runs for a workflow.
func (r *WorkflowRegistry) ListRuns(workflowID WorkflowID) []*WorkflowRun {
	runs := make([]*WorkflowRun, 0)
	for _, run := range r.runs {
		if run.WorkflowID == workflowID {
			runs = append(runs, run)
		}
	}
	return runs
}
