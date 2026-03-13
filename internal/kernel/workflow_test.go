package kernel

import (
	"fmt"
	"testing"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/stretchr/testify/assert"
)

func testWorkflow(name string, steps []types.WorkflowStep) types.Workflow {
	return types.Workflow{
		ID:          types.WorkflowID("test-wf-" + name),
		Name:        name,
		Description: "Test workflow",
		Steps:       steps,
		CreatedAt:   time.Now(),
	}
}

func stepAgentByName(name string) types.StepAgent {
	return types.StepAgent{Name: &name}
}

func TestRegisterWorkflow(t *testing.T) {
	engine := NewWorkflowEngine()
	wf := testWorkflow("test-pipeline", []types.WorkflowStep{})
	id := engine.Register(wf)
	assert.Equal(t, wf.ID, id)

	retrieved := engine.GetWorkflow(id)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-pipeline", retrieved.Name)
}

func TestListWorkflows(t *testing.T) {
	engine := NewWorkflowEngine()
	wf1 := testWorkflow("wf1", []types.WorkflowStep{})
	wf2 := testWorkflow("wf2", []types.WorkflowStep{})
	engine.Register(wf1)
	engine.Register(wf2)

	list := engine.ListWorkflows()
	assert.Len(t, list, 2)
}

func TestRemoveWorkflow(t *testing.T) {
	engine := NewWorkflowEngine()
	wf := testWorkflow("test-remove", []types.WorkflowStep{})
	id := engine.Register(wf)

	assert.True(t, engine.RemoveWorkflow(id))
	assert.Nil(t, engine.GetWorkflow(id))
}

func TestCreateRun(t *testing.T) {
	engine := NewWorkflowEngine()
	wf := testWorkflow("test-run", []types.WorkflowStep{})
	wfID := engine.Register(wf)

	runID := engine.CreateRun(wfID, "test input")
	assert.NotNil(t, runID)

	run := engine.GetRun(*runID)
	assert.NotNil(t, run)
	assert.Equal(t, "test input", run.Input)
	assert.Equal(t, types.WorkflowRunStatePending, run.State)
}

func TestExecuteSequentialPipeline(t *testing.T) {
	engine := NewWorkflowEngine()

	step1Name := "analyze"
	step2Name := "summarize"
	agentName := "test-agent"

	wf := testWorkflow("sequential-test", []types.WorkflowStep{
		{
			Name:           step1Name,
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Analyze this: {{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    30,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
		{
			Name:           step2Name,
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Summarize this analysis: {{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    30,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "raw data")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return "Processed: " + prompt, 100, 50, nil
	}

	result, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)
	assert.Contains(t, result, "Processed:")

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 2)
	assert.NotNil(t, run.Output)
}

func TestConditionalSkip(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"
	condition := "ERROR"

	wf := testWorkflow("conditional-skip-test", []types.WorkflowStep{
		{
			Name:           "first",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
		{
			Name:           "only-if-error",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Fix: {{input}}",
			Mode:           types.StepMode{Type: "conditional", Condition: &condition},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "all good")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return "OK: " + prompt, 10, 5, nil
	}

	_, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 1)
}

func TestConditionalExecutes(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"
	condition := "ERROR"

	wf := testWorkflow("conditional-exec-test", []types.WorkflowStep{
		{
			Name:           "first",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
		{
			Name:           "only-if-error",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Fix: {{input}}",
			Mode:           types.StepMode{Type: "conditional", Condition: &condition},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "data")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	callCount := 0
	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		callCount++
		if callCount == 1 {
			return "Found an ERROR in the data", 10, 5, nil
		}
		return "Fixed: " + prompt, 10, 5, nil
	}

	_, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 2)
}

func TestLoopUntilCondition(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"
	until := "DONE"
	maxIter := uint32(5)

	wf := testWorkflow("loop-test", []types.WorkflowStep{
		{
			Name:           "refine",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Refine: {{input}}",
			Mode:           types.StepMode{Type: "loop", MaxIterations: &maxIter, Until: &until},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "draft")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	callCount := 0
	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		callCount++
		if callCount >= 3 {
			return "Result: DONE", 10, 5, nil
		}
		return "Still working...", 10, 5, nil
	}

	result, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)
	assert.Contains(t, result, "DONE")
	assert.Equal(t, 3, callCount)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 3)
}

func TestLoopMaxIterations(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"
	until := "NEVER_MATCH"
	maxIter := uint32(3)

	wf := testWorkflow("loop-max-test", []types.WorkflowStep{
		{
			Name:           "refine",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "loop", MaxIterations: &maxIter, Until: &until},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "data")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	callCount := 0
	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		callCount++
		return "iteration output", 10, 5, nil
	}

	_, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 3)
}

func TestFanOutAndCollect(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"

	wf := testWorkflow("fanout-test", []types.WorkflowStep{
		{
			Name:           "fan1",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Task 1: {{input}}",
			Mode:           types.StepMode{Type: "fan_out"},
			TimeoutSecs:    30,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
		{
			Name:           "fan2",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Task 2: {{input}}",
			Mode:           types.StepMode{Type: "fan_out"},
			TimeoutSecs:    30,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
		{
			Name:           "collect",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Combine: {{input}}",
			Mode:           types.StepMode{Type: "collect"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "initial input")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return "Result for: " + prompt, 100, 50, nil
	}

	_, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 2)
	assert.Contains(t, *run.Output, "---")
}

func TestErrorModeSkip(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"

	wf := testWorkflow("error-skip-test", []types.WorkflowStep{
		{
			Name:           "failing-step",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "skip"},
		},
		{
			Name:           "success-step",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "Continue with: {{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "test data")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	callCount := 0
	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		callCount++
		if callCount == 1 {
			return "", 0, 0, assert.AnError
		}
		return "Success: " + prompt, 10, 5, nil
	}

	_, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
	assert.Len(t, run.StepResults, 1)
}

func TestErrorModeRetry(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"
	maxRetries := uint32(2)

	wf := testWorkflow("error-retry-test", []types.WorkflowStep{
		{
			Name:           "retry-step",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "retry", MaxRetries: &maxRetries},
		},
	})

	wfID := engine.Register(wf)
	runID := engine.CreateRun(wfID, "test data")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	callCount := 0
	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		callCount++
		if callCount <= 2 {
			return "", 0, 0, assert.AnError
		}
		return "Success after retry", 10, 5, nil
	}

	_, err := engine.ExecuteRun(*runID, resolver, sender)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)

	run := engine.GetRun(*runID)
	assert.Equal(t, types.WorkflowRunStateCompleted, run.State)
}

func TestListRunsWithStateFilter(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"

	wf := testWorkflow("filter-test", []types.WorkflowStep{
		{
			Name:           "step1",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)

	runID1 := engine.CreateRun(wfID, "input1")

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return "output", 10, 5, nil
	}

	_, err := engine.ExecuteRun(*runID1, resolver, sender)
	assert.NoError(t, err)

	_ = engine.CreateRun(wfID, "input2")

	allRuns := engine.ListRuns(nil, nil)
	assert.Len(t, allRuns, 2)

	pendingFilter := "pending"
	pendingRuns := engine.ListRuns(&pendingFilter, &wfID)
	assert.Len(t, pendingRuns, 1)

	completedFilter := "completed"
	completedRuns := engine.ListRuns(&completedFilter, &wfID)
	assert.Len(t, completedRuns, 1)
}

func TestEvictOldRuns(t *testing.T) {
	engine := NewWorkflowEngine()

	agentName := "test-agent"

	wf := testWorkflow("evict-test", []types.WorkflowStep{
		{
			Name:           "step1",
			Agent:          stepAgentByName(agentName),
			PromptTemplate: "{{input}}",
			Mode:           types.StepMode{Type: "sequential"},
			TimeoutSecs:    10,
			ErrorMode:      types.ErrorMode{Type: "fail"},
		},
	})

	wfID := engine.Register(wf)

	resolver := func(agent types.StepAgent) (string, string, bool) {
		return "agent-id-123", agentName, true
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return "output", 10, 5, nil
	}

	for i := 0; i < 250; i++ {
		runID := engine.CreateRun(wfID, fmt.Sprintf("input-%d", i))
		_, err := engine.ExecuteRun(*runID, resolver, sender)
		assert.NoError(t, err)
	}

	allRuns := engine.ListRuns(nil, &wfID)
	assert.LessOrEqual(t, len(allRuns), MaxRetainedRuns)
}
