package kernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTimeoutSecs = 120
	MaxRetainedRuns    = 200
)

type WorkflowEngine struct {
	mu            sync.RWMutex
	workflows     map[types.WorkflowID]types.Workflow
	runs          map[types.WorkflowRunID]types.WorkflowRun
	templates     map[types.WorkflowTemplateID]types.WorkflowTemplate
	dataDir       string
	channelSender func(channelName, recipient, message string) error
}

func NewWorkflowEngine(dataDir ...string) *WorkflowEngine {
	var dir string
	if len(dataDir) > 0 {
		dir = dataDir[0]
	}
	fmt.Printf("[WorkflowEngine] NewWorkflowEngine called with dataDir: '%s'\n", dir)
	e := &WorkflowEngine{
		workflows: make(map[types.WorkflowID]types.Workflow),
		runs:      make(map[types.WorkflowRunID]types.WorkflowRun),
		templates: make(map[types.WorkflowTemplateID]types.WorkflowTemplate),
		dataDir:   dir,
	}
	e.loadDefaultTemplates()
	if dir != "" {
		fmt.Printf("[WorkflowEngine] Calling loadFromDisk...\n")
		e.loadFromDisk()
		fmt.Printf("[WorkflowEngine] loadFromDisk completed. Total workflows: %d\n", len(e.workflows))
	} else {
		fmt.Printf("[WorkflowEngine] dataDir is empty, skipping loadFromDisk\n")
	}
	return e
}

func (e *WorkflowEngine) SetChannelSender(sender func(channelName, recipient, message string) error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.channelSender = sender
}

func (e *WorkflowEngine) loadFromDisk() {
	if e.dataDir == "" {
		fmt.Printf("[WorkflowEngine] dataDir is empty, skipping loadFromDisk\n")
		return
	}

	e.loadWorkflowsFromDisk()
	e.loadWorkflowRunsFromDisk()
}

func (e *WorkflowEngine) loadWorkflowsFromDisk() {
	workflowsDir := filepath.Join(e.dataDir, "workflows")
	fmt.Printf("[WorkflowEngine] Loading workflows from: %s\n", workflowsDir)

	if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
		fmt.Printf("[WorkflowEngine] Workflows directory does not exist: %s\n", workflowsDir)
		return
	}

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		fmt.Printf("[WorkflowEngine] Failed to read workflows directory: %v\n", err)
		return
	}

	fmt.Printf("[WorkflowEngine] Found %d entries in workflows directory\n", len(entries))

	e.mu.Lock()
	defer e.mu.Unlock()

	loadedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(workflowsDir, entry.Name())
			fmt.Printf("[WorkflowEngine] Loading workflow file: %s\n", path)
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("[WorkflowEngine] Failed to read workflow file %s: %v\n", path, err)
				continue
			}
			var wf types.Workflow
			if err := json.Unmarshal(data, &wf); err != nil {
				fmt.Printf("[WorkflowEngine] Failed to unmarshal workflow file %s: %v\n", path, err)
				continue
			}
			fmt.Printf("[WorkflowEngine] Loaded workflow: %s (ID: %s)\n", wf.Name, wf.ID)
			e.workflows[wf.ID] = wf
			loadedCount++
		}
	}
	fmt.Printf("[WorkflowEngine] Total workflows loaded: %d\n", loadedCount)
}

func (e *WorkflowEngine) loadWorkflowRunsFromDisk() {
	runsDir := filepath.Join(e.dataDir, "workflow_runs")
	fmt.Printf("[WorkflowEngine] Loading workflow runs from: %s\n", runsDir)

	if _, err := os.Stat(runsDir); os.IsNotExist(err) {
		fmt.Printf("[WorkflowEngine] Workflow runs directory does not exist: %s\n", runsDir)
		return
	}

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		fmt.Printf("[WorkflowEngine] Failed to read workflow runs directory: %v\n", err)
		return
	}

	fmt.Printf("[WorkflowEngine] Found %d entries in workflow runs directory\n", len(entries))

	e.mu.Lock()
	defer e.mu.Unlock()

	loadedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(runsDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("[WorkflowEngine] Failed to read workflow run file %s: %v\n", path, err)
				continue
			}
			var run types.WorkflowRun
			if err := json.Unmarshal(data, &run); err != nil {
				fmt.Printf("[WorkflowEngine] Failed to unmarshal workflow run file %s: %v\n", path, err)
				continue
			}
			fmt.Printf("[WorkflowEngine] Loaded workflow run: %s (Workflow: %s)\n", run.ID, run.WorkflowName)
			e.runs[run.ID] = run
			loadedCount++
		}
	}
	fmt.Printf("[WorkflowEngine] Total workflow runs loaded: %d\n", loadedCount)
}

func (e *WorkflowEngine) saveToDisk(workflow types.Workflow) error {
	if e.dataDir == "" {
		return nil
	}

	workflowsDir := filepath.Join(e.dataDir, "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(workflowsDir, fmt.Sprintf("%s.json", workflow.ID))
	data, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (e *WorkflowEngine) deleteFromDisk(id types.WorkflowID) error {
	if e.dataDir == "" {
		return nil
	}

	path := filepath.Join(e.dataDir, "workflows", fmt.Sprintf("%s.json", id))
	return os.Remove(path)
}

func (e *WorkflowEngine) saveRunToDisk(run types.WorkflowRun) error {
	if e.dataDir == "" {
		return nil
	}

	runsDir := filepath.Join(e.dataDir, "workflow_runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(runsDir, fmt.Sprintf("%s.json", run.ID))
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (e *WorkflowEngine) deleteRunFromDisk(id types.WorkflowRunID) error {
	if e.dataDir == "" {
		return nil
	}

	path := filepath.Join(e.dataDir, "workflow_runs", fmt.Sprintf("%s.json", id))
	return os.Remove(path)
}

func uuid() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func (e *WorkflowEngine) Register(workflow types.Workflow) types.WorkflowID {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[workflow.ID] = workflow
	e.saveToDisk(workflow)
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
		e.deleteFromDisk(id)
		return true
	}
	return false
}

func (e *WorkflowEngine) CreateRun(workflowID types.WorkflowID, input string) *types.WorkflowRunID {
	e.mu.Lock()

	workflow, ok := e.workflows[workflowID]
	if !ok {
		e.mu.Unlock()
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

	e.mu.Unlock()

	if err := e.saveRunToDisk(run); err != nil {
		log.Warn().Err(err).Str("run_id", string(runID)).Msg("Failed to save workflow run to disk")
	}

	return &runID
}

func (e *WorkflowEngine) evictOldRuns() {
	if len(e.runs) <= MaxRetainedRuns {
		return
	}

	evictable := make([]struct {
		id    types.WorkflowRunID
		start time.Time
	}, 0)

	for id, run := range e.runs {
		if run.State == types.WorkflowRunStateCompleted || run.State == types.WorkflowRunStateFailed {
			evictable = append(evictable, struct {
				id    types.WorkflowRunID
				start time.Time
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
	idsToRemove := make([]types.WorkflowRunID, 0, toRemove)
	for i := 0; i < toRemove && i < len(evictable); i++ {
		delete(e.runs, evictable[i].id)
		idsToRemove = append(idsToRemove, evictable[i].id)
	}

	e.mu.Unlock()
	for _, id := range idsToRemove {
		if err := e.deleteRunFromDisk(id); err != nil {
			log.Warn().Err(err).Str("run_id", string(id)).Msg("Failed to delete workflow run from disk")
		}
	}
	e.mu.Lock()
}

func (e *WorkflowEngine) GetRun(id types.WorkflowRunID) *types.WorkflowRun {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if run, ok := e.runs[id]; ok {
		return &run
	}
	return nil
}

func (e *WorkflowEngine) ListRuns(stateFilter *string, workflowID *types.WorkflowID) []types.WorkflowRun {
	e.mu.RLock()
	defer e.mu.RUnlock()
	runs := make([]types.WorkflowRun, 0, len(e.runs))
	for _, run := range e.runs {
		if workflowID != nil && run.WorkflowID != *workflowID {
			continue
		}
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

func (e *WorkflowEngine) ExpandVariables(template, input string, vars map[string]string, stepResults []types.StepResult) string {
	result := strings.ReplaceAll(template, "{{input}}", input)
	for key, value := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}
	// 处理 {{steps.step_name.output}} 格式的变量
	for _, stepResult := range stepResults {
		placeholder := fmt.Sprintf("{{steps.%s.output}}", stepResult.StepName)
		result = strings.ReplaceAll(result, placeholder, stepResult.Output)
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
	stepResults := []types.StepResult{}
	i := 0

	for i < len(workflow.Steps) {
		step := workflow.Steps[i]

		switch step.Mode.Type {
		case "sequential":
			agentID, agentName, ok := resolver(step.Agent)
			if !ok {
				return e.failRun(runID, fmt.Sprintf("agent not found for step '%s'", step.Name))
			}

			prompt := e.ExpandVariables(step.PromptTemplate, currentInput, variables, stepResults)
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
				stepResults = append(stepResults, stepResult)

				if step.OutputVar != nil {
					variables[*step.OutputVar] = *output
				}
				allOutputs = append(allOutputs, *output)
				currentInput = *output
			}

		case "fan_out":
			fanOutSteps := []struct {
				index int
				step  types.WorkflowStep
			}{{i, step}}
			j := i + 1
			for j < len(workflow.Steps) {
				if workflow.Steps[j].Mode.Type == "fan_out" {
					fanOutSteps = append(fanOutSteps, struct {
						index int
						step  types.WorkflowStep
					}{j, workflow.Steps[j]})
					j++
				} else {
					break
				}
			}

			type stepResult struct {
				index        int
				stepName     string
				agentID      string
				agentName    string
				output       *string
				inputTokens  uint64
				outputTokens uint64
				durationMS   uint64
				err          error
			}

			resultsChan := make(chan stepResult, len(fanOutSteps))
			var wg sync.WaitGroup

			for _, fs := range fanOutSteps {
				wg.Add(1)
				// Create local variables to capture the current values
				localIdx := fs.index
				localStep := fs.step
				go func(idx int, s types.WorkflowStep) {
					defer wg.Done()

					agentID, agentName, ok := resolver(s.Agent)
					if !ok {
						resultsChan <- stepResult{
							index:    idx,
							stepName: s.Name,
							err:      fmt.Errorf("agent not found for step '%s'", s.Name),
						}
						return
					}

					prompt := e.ExpandVariables(s.PromptTemplate, currentInput, variables, stepResults)
					output, inputTokens, outputTokens, durationMS, err := e.executeStepWithErrorMode(s, agentID, prompt, sender)

					resultsChan <- stepResult{
						index:        idx,
						stepName:     s.Name,
						agentID:      agentID,
						agentName:    agentName,
						output:       output,
						inputTokens:  inputTokens,
						outputTokens: outputTokens,
						durationMS:   durationMS,
						err:          err,
					}
				}(localIdx, localStep)
			}

			go func() {
				wg.Wait()
				close(resultsChan)
			}()

			results := make(map[int]stepResult)
			for res := range resultsChan {
				results[res.index] = res
			}

			for _, fs := range fanOutSteps {
				res, ok := results[fs.index]
				if !ok {
					continue
				}

				if res.err != nil {
					return e.failRun(runID, res.err.Error())
				}

				if res.output != nil {
					stepResult := types.StepResult{
						StepName:     res.stepName,
						AgentID:      res.agentID,
						AgentName:    res.agentName,
						Output:       *res.output,
						InputTokens:  res.inputTokens,
						OutputTokens: res.outputTokens,
						DurationMS:   res.durationMS,
					}
					e.addStepResult(runID, stepResult)
					stepResults = append(stepResults, stepResult)

					if fs.step.OutputVar != nil {
						variables[*fs.step.OutputVar] = *res.output
					}
					allOutputs = append(allOutputs, *res.output)
					// Don't update currentInput in fan_out mode - multiple steps run in parallel
					// The next step should use the original input or collect mode
				}
			}

			i = j
			continue

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

			prompt := e.ExpandVariables(step.PromptTemplate, currentInput, variables, stepResults)
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
				stepResults = append(stepResults, stepResult)

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
				prompt := e.ExpandVariables(step.PromptTemplate, currentInput, variables, stepResults)
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
					stepResults = append(stepResults, stepResult)
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

	type result struct {
		output       string
		inputTokens  uint64
		outputTokens uint64
		durationMS   uint64
		err          error
	}

	resultChan := make(chan result, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		start := time.Now()
		output, inputTokens, outputTokens, err := sender(agentID, prompt)
		durationMS := uint64(time.Since(start).Milliseconds())
		resultChan <- result{
			output:       output,
			inputTokens:  inputTokens,
			outputTokens: outputTokens,
			durationMS:   durationMS,
			err:          err,
		}
	}()

	select {
	case res := <-resultChan:
		if res.err != nil {
			switch step.ErrorMode.Type {
			case "fail":
				return nil, 0, 0, 0, fmt.Errorf("step '%s' failed: %w", step.Name, res.err)
			case "skip":
				return nil, 0, 0, 0, nil
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
				return nil, 0, 0, 0, fmt.Errorf("step '%s' failed: %w", step.Name, res.err)
			}
		}

		return &res.output, res.inputTokens, res.outputTokens, res.durationMS, nil

	case <-time.After(time.Duration(timeoutSecs) * time.Second):
		switch step.ErrorMode.Type {
		case "fail":
			return nil, 0, 0, 0, fmt.Errorf("step '%s' timed out after %d seconds", step.Name, timeoutSecs)
		case "skip":
			return nil, 0, 0, 0, nil
		case "retry":
			maxRetries := *step.ErrorMode.MaxRetries
			for attempt := uint32(0); attempt <= maxRetries; attempt++ {
				attemptResultChan := make(chan result, 1)
				attemptDone := make(chan struct{})

				go func() {
					defer close(attemptDone)
					start := time.Now()
					output, inputTokens, outputTokens, err := sender(agentID, prompt)
					durationMS := uint64(time.Since(start).Milliseconds())
					attemptResultChan <- result{
						output:       output,
						inputTokens:  inputTokens,
						outputTokens: outputTokens,
						durationMS:   durationMS,
						err:          err,
					}
				}()

				select {
				case res := <-attemptResultChan:
					if res.err == nil {
						return &res.output, res.inputTokens, res.outputTokens, res.durationMS, nil
					}
				case <-time.After(time.Duration(timeoutSecs) * time.Second):
					if attempt == maxRetries {
						return nil, 0, 0, 0, fmt.Errorf("step '%s' timed out after %d seconds and %d retries", step.Name, timeoutSecs, maxRetries)
					}
				}
			}
			return nil, 0, 0, 0, fmt.Errorf("step '%s' timed out after %d seconds", step.Name, timeoutSecs)
		default:
			return nil, 0, 0, 0, fmt.Errorf("step '%s' timed out after %d seconds", step.Name, timeoutSecs)
		}
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
	var runToSave *types.WorkflowRun
	if run, ok := e.runs[runID]; ok {
		run.State = types.WorkflowRunStateFailed
		run.Error = &err
		now := time.Now()
		run.CompletedAt = &now
		e.runs[runID] = run
		runCopy := run
		runToSave = &runCopy
	}
	e.mu.Unlock()

	if runToSave != nil {
		if err := e.saveRunToDisk(*runToSave); err != nil {
			log.Warn().Err(err).Str("run_id", string(runID)).Msg("Failed to save failed workflow run to disk")
		}
	}

	return "", fmt.Errorf("%s", err)
}

func (e *WorkflowEngine) completeRun(runID types.WorkflowRunID, output string) (string, error) {
	e.mu.Lock()
	var runToSave *types.WorkflowRun
	if run, ok := e.runs[runID]; ok {
		run.State = types.WorkflowRunStateCompleted
		run.Output = &output
		now := time.Now()
		run.CompletedAt = &now
		e.runs[runID] = run
		runCopy := run
		runToSave = &runCopy
	}
	e.mu.Unlock()

	if runToSave != nil {
		if err := e.saveRunToDisk(*runToSave); err != nil {
			log.Warn().Err(err).Str("run_id", string(runID)).Msg("Failed to save completed workflow run to disk")
		}
	}

	return output, nil
}

func (e *WorkflowEngine) loadDefaultTemplates() {
	e.mu.Lock()
	defer e.mu.Unlock()

	analystName := "analyst"
	writerName := "writer"
	coderName := "coder"
	researcherName := "researcher"

	templates := []types.WorkflowTemplate{
		{
			ID:          "content-pipeline",
			Name:        "Content Pipeline",
			Description: "Analyze input, then summarize and write final content",
			Category:    "content",
			Workflow: types.Workflow{
				ID:          "",
				Name:        "Content Pipeline",
				Description: "Analyze input, then summarize and write final content",
				Steps: []types.WorkflowStep{
					{
						Name: "analyze",
						Agent: types.StepAgent{
							Name: &analystName,
						},
						PromptTemplate: "Analyze this content carefully: {{input}}",
						Mode: types.StepMode{
							Type: "sequential",
						},
						TimeoutSecs: 120,
						ErrorMode: types.ErrorMode{
							Type: "fail",
						},
					},
					{
						Name: "summarize",
						Agent: types.StepAgent{
							Name: &writerName,
						},
						PromptTemplate: "Summarize this analysis: {{input}}",
						Mode: types.StepMode{
							Type: "sequential",
						},
						TimeoutSecs: 120,
						ErrorMode: types.ErrorMode{
							Type: "fail",
						},
					},
				},
				CreatedAt: time.Now(),
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          "code-review-pipeline",
			Name:        "Code Review Pipeline",
			Description: "Analyze code, then review and write improvements",
			Category:    "coding",
			Workflow: types.Workflow{
				ID:          "",
				Name:        "Code Review Pipeline",
				Description: "Analyze code, then review and write improvements",
				Steps: []types.WorkflowStep{
					{
						Name: "analyze-code",
						Agent: types.StepAgent{
							Name: &analystName,
						},
						PromptTemplate: "Analyze this code: {{input}}",
						Mode: types.StepMode{
							Type: "sequential",
						},
						TimeoutSecs: 120,
						ErrorMode: types.ErrorMode{
							Type: "fail",
						},
					},
					{
						Name: "review-code",
						Agent: types.StepAgent{
							Name: &coderName,
						},
						PromptTemplate: "Review and improve this code: {{input}}",
						Mode: types.StepMode{
							Type: "sequential",
						},
						TimeoutSecs: 120,
						ErrorMode: types.ErrorMode{
							Type: "fail",
						},
					},
				},
				CreatedAt: time.Now(),
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          "research-report",
			Name:        "Research Report",
			Description: "Research a topic and write a comprehensive report",
			Category:    "research",
			Workflow: types.Workflow{
				ID:          "",
				Name:        "Research Report",
				Description: "Research a topic and write a comprehensive report",
				Steps: []types.WorkflowStep{
					{
						Name: "research",
						Agent: types.StepAgent{
							Name: &researcherName,
						},
						PromptTemplate: "Research this topic thoroughly: {{input}}",
						Mode: types.StepMode{
							Type: "sequential",
						},
						TimeoutSecs: 300,
						ErrorMode: types.ErrorMode{
							Type: "fail",
						},
					},
					{
						Name: "write-report",
						Agent: types.StepAgent{
							Name: &writerName,
						},
						PromptTemplate: "Write a comprehensive report based on this research: {{input}}",
						Mode: types.StepMode{
							Type: "sequential",
						},
						TimeoutSecs: 300,
						ErrorMode: types.ErrorMode{
							Type: "fail",
						},
					},
				},
				CreatedAt: time.Now(),
			},
			CreatedAt: time.Now(),
		},
	}

	for _, t := range templates {
		e.templates[t.ID] = t
	}
}

func (e *WorkflowEngine) ListTemplates() []types.WorkflowTemplate {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]types.WorkflowTemplate, 0, len(e.templates))
	for _, t := range e.templates {
		result = append(result, t)
	}
	return result
}

func (e *WorkflowEngine) GetTemplate(id types.WorkflowTemplateID) *types.WorkflowTemplate {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if t, ok := e.templates[id]; ok {
		return &t
	}
	return nil
}

func (e *WorkflowEngine) CreateFromTemplate(templateID types.WorkflowTemplateID, customName, customDescription string) (*types.Workflow, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	template, ok := e.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	wf := template.Workflow
	wf.ID = types.WorkflowID(fmt.Sprintf("workflow-%s", uuid()))
	if customName != "" {
		wf.Name = customName
	}
	if customDescription != "" {
		wf.Description = customDescription
	}
	wf.CreatedAt = time.Now()

	e.workflows[wf.ID] = wf
	if err := e.saveToDisk(wf); err != nil {
		return nil, err
	}

	return &wf, nil
}

func (e *WorkflowEngine) DeliverResult(workflowID types.WorkflowID, output string, delivery *types.DeliveryConfig) error {
	if delivery == nil {
		return nil
	}

	switch delivery.Type {
	case types.DeliveryTypeChannel:
		if delivery.ChannelConfig == nil {
			return fmt.Errorf("channel config is required for channel delivery")
		}
		return e.deliverToChannel(delivery.ChannelConfig.ChannelName, delivery.ChannelConfig.Recipient, output)
	case types.DeliveryTypeWebhook:
		if delivery.WebhookConfig == nil {
			return fmt.Errorf("webhook config is required for webhook delivery")
		}
		return e.deliverToWebhook(delivery.WebhookConfig.URL, delivery.WebhookConfig.Headers, output, workflowID)
	default:
		return fmt.Errorf("unknown delivery type: %s", delivery.Type)
	}
}

func (e *WorkflowEngine) deliverToChannel(channelName, recipient, message string) error {
	e.mu.RLock()
	sender := e.channelSender
	e.mu.RUnlock()

	if sender == nil {
		return fmt.Errorf("channel sender not configured")
	}

	return sender(channelName, recipient, message)
}

func (e *WorkflowEngine) deliverToWebhook(url string, headers map[string]string, output string, workflowID types.WorkflowID) error {
	payload := map[string]interface{}{
		"workflow_id": workflowID,
		"output":      output,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook request failed with status: %d", resp.StatusCode)
	}

	return nil
}
