package kernel

import (
	"testing"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
	"github.com/penzhan8451/fangclaw-go/internal/triggers"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestPublishEventWithTrigger(t *testing.T) {
	config := types.KernelConfig{
		DataDir: t.TempDir(),
	}

	kernel, err := NewKernel(config)
	assert.NoError(t, err)
	assert.NotNil(t, kernel)

	agentID := "test-agent-123"

	triggerID := triggers.NewTriggerID()
	trigger := &triggers.Trigger{
		ID:             triggerID,
		AgentID:        agentID,
		Pattern:        triggers.NewAllPattern(),
		PromptTemplate: "Event occurred: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	kernel.triggerEngine.Register(trigger)

	event := eventbus.NewEvent(
		eventbus.EventTypeSystem,
		"test-source",
		eventbus.EventTargetBroadcast,
	).WithPayload(map[string]interface{}{
		"test": "payload",
	})

	result := kernel.PublishEvent(event)

	assert.Len(t, result, 1)
	assert.Equal(t, agentID, result[0].AgentID)
	assert.Contains(t, result[0].Message, "Event occurred")
}

func TestMultipleTriggers(t *testing.T) {
	config := types.KernelConfig{
		DataDir: t.TempDir(),
	}

	kernel, err := NewKernel(config)
	assert.NoError(t, err)
	assert.NotNil(t, kernel)

	agent1 := "agent-1"
	agent2 := "agent-2"

	trigger1 := &triggers.Trigger{
		ID:             triggers.NewTriggerID(),
		AgentID:        agent1,
		Pattern:        triggers.NewAllPattern(),
		PromptTemplate: "Agent 1: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	trigger2 := &triggers.Trigger{
		ID:             triggers.NewTriggerID(),
		AgentID:        agent2,
		Pattern:        triggers.NewAllPattern(),
		PromptTemplate: "Agent 2: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	kernel.triggerEngine.Register(trigger1)
	kernel.triggerEngine.Register(trigger2)

	event := eventbus.NewEvent(
		eventbus.EventTypeAgentCreated,
		"test",
		eventbus.EventTargetBroadcast,
	)

	result := kernel.PublishEvent(event)

	assert.Len(t, result, 2)

	agentIDs := make([]string, 2)
	for i, r := range result {
		agentIDs[i] = r.AgentID
	}
	assert.Contains(t, agentIDs, agent1)
	assert.Contains(t, agentIDs, agent2)
}

func TestDisabledTriggerDoesNotFire(t *testing.T) {
	config := types.KernelConfig{
		DataDir: t.TempDir(),
	}

	kernel, err := NewKernel(config)
	assert.NoError(t, err)
	assert.NotNil(t, kernel)

	trigger := &triggers.Trigger{
		ID:             triggers.NewTriggerID(),
		AgentID:        "test-agent",
		Pattern:        triggers.NewAllPattern(),
		PromptTemplate: "Should not fire",
		Enabled:        false,
		CreatedAt:      time.Now(),
	}

	kernel.triggerEngine.Register(trigger)

	event := eventbus.NewEvent(
		eventbus.EventTypeSystem,
		"test",
		eventbus.EventTargetBroadcast,
	)

	result := kernel.PublishEvent(event)

	assert.Len(t, result, 0)
}

func TestLifecyclePatternTrigger(t *testing.T) {
	config := types.KernelConfig{
		DataDir: t.TempDir(),
	}

	kernel, err := NewKernel(config)
	assert.NoError(t, err)
	assert.NotNil(t, kernel)

	trigger := &triggers.Trigger{
		ID:             triggers.NewTriggerID(),
		AgentID:        "test-agent",
		Pattern:        triggers.NewLifecyclePattern(),
		PromptTemplate: "Lifecycle event: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	kernel.triggerEngine.Register(trigger)

	lifecycleEvent := eventbus.NewEvent(
		eventbus.EventTypeAgentStarted,
		"test",
		eventbus.EventTargetBroadcast,
	).WithPayload(map[string]interface{}{
		"name": "TestAgent",
	})

	result := kernel.PublishEvent(lifecycleEvent)
	assert.Len(t, result, 1)
	assert.Contains(t, result[0].Message, "Lifecycle event")
	assert.Contains(t, result[0].Message, "TestAgent")
}

func TestEventBusIntegration(t *testing.T) {
	config := types.KernelConfig{
		DataDir: t.TempDir(),
	}

	kernel, err := NewKernel(config)
	assert.NoError(t, err)
	assert.NotNil(t, kernel)

	eventReceived := false
	var receivedEvent *eventbus.Event

	subscriptionID := kernel.eventBus.SubscribeAll(func(e *eventbus.Event) {
		eventReceived = true
		receivedEvent = e
	})

	event := eventbus.NewEvent(
		eventbus.EventTypeSystem,
		"test-source",
		eventbus.EventTargetSystem,
	)

	kernel.PublishEvent(event)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, eventReceived)
	assert.NotNil(t, receivedEvent)
	assert.Equal(t, event.ID, receivedEvent.ID)
	assert.Equal(t, eventbus.EventTypeSystem, receivedEvent.Type)

	kernel.eventBus.UnsubscribeAll(subscriptionID)
}
