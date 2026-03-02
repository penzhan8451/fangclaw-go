package kernel

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

const (
	DefaultTimeoutSecs = 120
	MaxRetainedRuns    = 200
)

type WorkflowEngine struct {
	mu        sync.RWMutex
	workflows map[types.WorkflowID]types.Workflow
	runs      map[types.WorkflowRunID]types.WorkflowRun
}

func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{
		workflows: make(map[types.WorkflowID]types.Workflow),
		runs:      make(map[types.WorkflowRunID]types.WorkflowRun),
	}
}

func (e *WorkflowEngine) Register(workflow types.Workflow) types.WorkflowID {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[workflow.ID] = workflow
	return workflow.ID
}

func (e *WorkflowEngine) ListWorkflows() []types.Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	workflows := make([]types.Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		workflows = append(workflows, wf)
	}
	return workflows
}

func (e *WorkflowEngine) GetWorkflow(id types.WorkflowID) *types.Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if wf, ok := e.workflows[id]; ok {
		return &wf
	}
	return nil
}

func (e *WorkflowEngine) RemoveWorkflow(id types.WorkflowID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.workflows[id]; ok {
		delete(e.workflows, id)
		return true
	}
	return false
}

func (e *WorkflowEngine) CreateRun(workflowID types.WorkflowID, input string) *types.WorkflowRunID {
	e.mu.Lock()
	defer e.mu.Unlock()

	workflow, ok := e.workflows[workflowID]
	if !ok {
		return nil
	}

	runID := types.WorkflowRunID(fmt.Sprintf("wf-run-%d", time.Now().UnixNano()))
	run := types.WorkflowRun{
		ID:           runID,
		WorkflowID:   workflowID,
		WorkflowName: workflow.Name,
		Input:        input,
		State:        types.WorkflowRunStatePending,
		StepResults:  []types.StepResult{},
		StartedAt:    time.Now(),
	}

	e.runs[runID] = run
	e.evictOldRuns()

	return &runID
}

func (e *WorkflowEngine) evictOldRuns() {
	if len(e.runs) <= MaxRetainedRuns {
		return
	}

	evictable := make([]struct {
		id     types.WorkflowRunID
		start  time.Time
	}, 0)

	for id, run := range e.runs {
		if run.State == types.WorkflowRunStateCompleted || run.State == types.WorkflowRunStateFailed {
			evictable = append(evictable, struct {
				id     types.WorkflowRunID
				start  time.Time
			}{id, run.StartedAt})
		}
	}

	if len(evictable) == 0 {
		return
	}

	for i := 0; i < len(evictable)-1; i++ {
		for j := i + 1; j < len(evictable); j++ {
			if evictable[i].start.After(evictable[j].start) {
				evictable[i], evictable[j] = evictable[j], evictable[i]
			}
		}
	}

	toRemove := len(e.runs) - MaxRetainedRuns
	for i := 0; i < toRemove && i < len(evictable); i++ {
		delete(e.runs, evictable[i].id)
	}
}

func (e *WorkflowEngine) GetRun(id types.WorkflowRunID) *types.WorkflowRun {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if run, ok := e.runs[id]; ok {
		return &run
	}
	return nil
}

func (e *WorkflowEngine) ListRuns(stateFilter *string) []types.WorkflowRun {
	e.mu.RLock()
	defer e.mu.RUnlock()
	runs := make([]types.WorkflowRun, 0, len(e.runs))
	for _, run := range e.runs {
		if stateFilter == nil {
			runs = append(runs, run)
			continue
		}
		switch *stateFilter {
		case "pending":
			if run.State == types.WorkflowRunStatePending {
				runs = append(runs, run)
			}
		case "running":
			if run.State == types.WorkflowRunStateRunning {
				runs = append(runs, run)
			}
		case "completed":
			if run.State == types.WorkflowRunStateCompleted {
				runs = append(runs, run)
			}
		case "failed":
			if run.State == types.WorkflowRunStateFailed {
				runs = append(runs, run)
			}
		default:
			runs = append(runs, run)
		}
	}
	return runs
}

func (e *WorkflowEngine) ExpandVariables(template, input string, vars map[string]string) string {
	result := strings.ReplaceAll(template, "{{input}}", input)
	for key, value := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}
	return result
}

type AgentResolver func(agent types.StepAgent) (string, string, bool)
type MessageSender func(agentID, prompt string) (string, uint64, uint64, error)

func (e *WorkflowEngine) ExecuteRun(
	runID types.WorkflowRunID,
	resolver AgentResolver,
	sender MessageSender,
) (string, error) {
	e.mu.Lock()
	run, ok := e.runs[runID]
	if !ok {
		e.mu.Unlock()
		return "", fmt.Errorf("workflow run not found")
	}
	run.State = types.WorkflowRunStateRunning
	e.runs[runID] = run

	workflow, ok := e.workflows[run.WorkflowID]
	if !ok {
		e.mu.Unlock()
		return "", fmt.Errorf("workflow definition not found")
	}
	input := run.Input
	e.mu.Unlock()

	currentInput := input
	allOutputs := []string{}
	variables := make(map[string]string)
	i := 0

	for i < len(workflow.Steps) {
		step := workflow.Steps[i]

		switch step.Mode.Type {
		case "sequential":
			agentID, agentName, ok := resolver(step.Agent)
			if !ok {
				return e.failRun(runID, fmt.Sprintf("agent not found for step '%s'", step.Name))
			}

			prompt := e.ExpandVariables(step.PromptTemplate, currentInput, variables)
			output, inputTokens, outputTokens, durationMS, err := e.executeStepWithErrorMode(step, agentID, prompt, sender)
			if err != nil {
				return e.failRun(runID, err.Error())
			}

			if output != nil {
				stepResult := types.StepResult{
					StepName:     step.Name,
					AgentID:      agentID,
					AgentName:    agentName,
					Output:       *output,
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
					DurationMS:   durationMS,
				}
				e.addStepResult(runID, stepResult)

				if step.OutputVar != nil {
					variables[*step.OutputVar] = *output
				}
				allOutputs = append(allOutputs, *output)
				currentInput = *output
			}

		case "conditional":
			prevLower := strings.ToLower(currentInput)
			condLower := strings.ToLower(*step.Mode.Condition)
			if !strings.Contains(prevLower, condLower) {
				i++
				continue
			}

			agentID, agentName, ok := resolver(step.Agent)
			if !ok {
				return e.failRun(runID, fmt.Sprintf("agent not found for step '%s'", step.Name))
			}

			prompt := e.ExpandVariables(step.PromptTemplate, currentInput, variables)
			output, inputTokens, outputTokens, durationMS, err := e.executeStepWithErrorMode(step, agentID, prompt, sender)
			if err != nil {
				return e.failRun(runID, err.Error())
			}

			if output != nil {
				stepResult := types.StepResult{
					StepName:     step.Name,
					AgentID:      agentID,
					AgentName:    agentName,
					Output:       *output,
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
					DurationMS:   durationMS,
				}
				e.addStepResult(runID, stepResult)

				if step.OutputVar != nil {
					variables[*step.OutputVar] = *output
				}
				allOutputs = append(allOutputs, *output)
				currentInput = *output
			}

		case "loop":
			agentID, agentName, ok := resolver(step.Agent)
			if !ok {
				return e.failRun(runID, fmt.Sprintf("agent not found for step '%s'", step.Name))
			}

			untilLower := strings.ToLower(*step.Mode.Until)
			maxIter := *step.Mode.MaxIterations

			for loopIter := uint32(0); loopIter < maxIter; loopIter++ {
				prompt := e.ExpandVariables(step.PromptTemplate, currentInput, variables)
				output, inputTokens, outputTokens, durationMS, err := e.executeStepWithErrorMode(step, agentID, prompt, sender)
				if err != nil {
					return e.failRun(runID, err.Error())
				}

				if output != nil {
					stepResult := types.StepResult{
						StepName:     fmt.Sprintf("%s (iter %d)", step.Name, loopIter+1),
						AgentID:      agentID,
						AgentName:    agentName,
						Output:       *output,
						InputTokens:  inputTokens,
						OutputTokens: outputTokens,
						DurationMS:   durationMS,
					}
					e.addStepResult(runID, stepResult)
					currentInput = *output

					if strings.Contains(strings.ToLower(*output), untilLower) {
						break
					}
				} else {
					break
				}
			}

			if step.OutputVar != nil {
				variables[*step.OutputVar] = currentInput
			}
			allOutputs = append(allOutputs, currentInput)

		case "collect":
			currentInput = strings.Join(allOutputs, "\n\n---\n\n")
			allOutputs = []string{currentInput}
			if step.OutputVar != nil {
				variables[*step.OutputVar] = currentInput
			}
		}

		i++
	}

	return e.completeRun(runID, currentInput)
}

func (e *WorkflowEngine) executeStepWithErrorMode(
	step types.WorkflowStep,
	agentID, prompt string,
	sender MessageSender,
) (*string, uint64, uint64, uint64, error) {
	timeoutSecs := step.TimeoutSecs
	if timeoutSecs == 0 {
		timeoutSecs = DefaultTimeoutSecs
	}

	switch step.ErrorMode.Type {
	case "fail":
		start := time.Now()
		output, inputTokens, outputTokens, err := sender(agentID, prompt)
		durationMS := uint64(time.Since(start).Milliseconds())
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("step '%s' failed: %w", step.Name, err)
		}
		return &output, inputTokens, outputTokens, durationMS, nil

	case "skip":
		start := time.Now()
		output, inputTokens, outputTokens, err := sender(agentID, prompt)
		durationMS := uint64(time.Since(start).Milliseconds())
		if err != nil {
			return nil, 0, 0, 0, nil
		}
		return &output, inputTokens, outputTokens, durationMS, nil

	case "retry":
		maxRetries := *step.ErrorMode.MaxRetries
		var lastErr error
		for attempt := uint32(0); attempt <= maxRetries; attempt++ {
			start := time.Now()
			output, inputTokens, outputTokens, err := sender(agentID, prompt)
			durationMS := uint64(time.Since(start).Milliseconds())
			if err == nil {
				return &output, inputTokens, outputTokens, durationMS, nil
			}
			lastErr = err
		}
		return nil, 0, 0, 0, fmt.Errorf("step '%s' failed after %d retries: %w", step.Name, maxRetries, lastErr)

	default:
		start := time.Now()
		output, inputTokens, outputTokens, err := sender(agentID, prompt)
		durationMS := uint64(time.Since(start).Milliseconds())
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("step '%s' failed: %w", step.Name, err)
		}
		return &output, inputTokens, outputTokens, durationMS, nil
	}
}

func (e *WorkflowEngine) addStepResult(runID types.WorkflowRunID, result types.StepResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if run, ok := e.runs[runID]; ok {
		run.StepResults = append(run.StepResults, result)
		e.runs[runID] = run
	}
}

func (e *WorkflowEngine) failRun(runID types.WorkflowRunID, err string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if run, ok := e.runs[runID]; ok {
		run.State = types.WorkflowRunStateFailed
		run.Error = &err
		now := time.Now()
		run.CompletedAt = &now
		e.runs[runID] = run
	}
	return "", fmt.Errorf("%s", err)
}

func (e *WorkflowEngine) completeRun(runID types.WorkflowRunID, output string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if run, ok := e.runs[runID]; ok {
		run.State = types.WorkflowRunStateCompleted
		run.Output = &output
		now := time.Now()
		run.CompletedAt = &now
		e.runs[runID] = run
	}
	return output, nil
}
