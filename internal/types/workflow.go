package types

import (
	"encoding/json"
	"time"
)

type WorkflowID string
type WorkflowRunID string

type Workflow struct {
	ID          WorkflowID       `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Steps       []WorkflowStep   `json:"steps"`
	CreatedAt   time.Time        `json:"created_at"`
}

type WorkflowStep struct {
	Name           string        `json:"name"`
	Agent          StepAgent     `json:"agent"`
	PromptTemplate string        `json:"prompt_template"`
	Mode           StepMode      `json:"mode"`
	TimeoutSecs    uint64        `json:"timeout_secs"`
	ErrorMode      ErrorMode     `json:"error_mode"`
	OutputVar      *string       `json:"output_var,omitempty"`
}

type StepAgent struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

func (a StepAgent) MarshalJSON() ([]byte, error) {
	if a.ID != nil {
		return json.Marshal(struct {
			ID string `json:"id"`
		}{*a.ID})
	}
	if a.Name != nil {
		return json.Marshal(struct {
			Name string `json:"name"`
		}{*a.Name})
	}
	return json.Marshal(nil)
}

type StepMode struct {
	Type         string  `json:"type"`
	Condition    *string `json:"condition,omitempty"`
	MaxIterations *uint32 `json:"max_iterations,omitempty"`
	Until        *string `json:"until,omitempty"`
}

func (m StepMode) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case "sequential":
		return json.Marshal("sequential")
	case "fan_out":
		return json.Marshal("fan_out")
	case "collect":
		return json.Marshal("collect")
	case "conditional":
		return json.Marshal(struct {
			Condition string `json:"condition"`
		}{*m.Condition})
	case "loop":
		return json.Marshal(struct {
			MaxIterations uint32 `json:"max_iterations"`
			Until         string `json:"until"`
		}{*m.MaxIterations, *m.Until})
	default:
		return json.Marshal("sequential")
	}
}

type ErrorMode struct {
	Type        string  `json:"type"`
	MaxRetries  *uint32 `json:"max_retries,omitempty"`
}

func (m ErrorMode) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case "fail":
		return json.Marshal("fail")
	case "skip":
		return json.Marshal("skip")
	case "retry":
		return json.Marshal(struct {
			MaxRetries uint32 `json:"max_retries"`
		}{*m.MaxRetries})
	default:
		return json.Marshal("fail")
	}
}

type WorkflowRunState string

const (
	WorkflowRunStatePending   WorkflowRunState = "pending"
	WorkflowRunStateRunning   WorkflowRunState = "running"
	WorkflowRunStateCompleted WorkflowRunState = "completed"
	WorkflowRunStateFailed    WorkflowRunState = "failed"
)

type WorkflowRun struct {
	ID            WorkflowRunID   `json:"id"`
	WorkflowID    WorkflowID      `json:"workflow_id"`
	WorkflowName  string          `json:"workflow_name"`
	Input         string          `json:"input"`
	State         WorkflowRunState `json:"state"`
	StepResults   []StepResult    `json:"step_results"`
	Output        *string         `json:"output,omitempty"`
	Error         *string         `json:"error,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
}

type StepResult struct {
	StepName      string `json:"step_name"`
	AgentID       string `json:"agent_id"`
	AgentName     string `json:"agent_name"`
	Output        string `json:"output"`
	InputTokens   uint64 `json:"input_tokens"`
	OutputTokens  uint64 `json:"output_tokens"`
	DurationMS    uint64 `json:"duration_ms"`
}
