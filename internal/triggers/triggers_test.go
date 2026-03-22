package triggers

import (
	"testing"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/stretchr/testify/assert"
)

var db, err = memory.NewDB("/tmp/triggers.db")

func TestNewTriggerEngine(t *testing.T) {
	assert.NoError(t, err, "Failed to create database")
	engine := NewTriggerEngine(db)
	assert.NotNil(t, engine)
}

func TestRegisterAndGetTrigger(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
		FireCount:      0,
		MaxFires:       0,
	}

	err := engine.Register(trigger)
	assert.NoError(t, err)

	retrieved, ok := engine.Get(triggerID)
	assert.True(t, ok)
	assert.Equal(t, triggerID, retrieved.ID)
	assert.Equal(t, "agent-123", retrieved.AgentID)
}

func TestListTriggers(t *testing.T) {
	engine := NewTriggerEngine(db)

	trigger1 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        "agent-1",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event 1",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}
	trigger2 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        "agent-2",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event 2",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger1)
	engine.Register(trigger2)

	allTriggers := engine.List("")
	assert.Len(t, allTriggers, 2)

	agent1Triggers := engine.List("agent-1")
	assert.Len(t, agent1Triggers, 1)
	assert.Equal(t, "agent-1", agent1Triggers[0].AgentID)
}

func TestDeleteTrigger(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger)
	assert.True(t, engine.Delete(triggerID))
	assert.False(t, engine.Delete(triggerID))

	_, ok := engine.Get(triggerID)
	assert.False(t, ok)
}

func TestEnableDisableTrigger(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger)
	assert.True(t, engine.Disable(triggerID))

	retrieved, ok := engine.Get(triggerID)
	assert.True(t, ok)
	assert.False(t, retrieved.Enabled)

	assert.True(t, engine.Enable(triggerID))
	retrieved, ok = engine.Get(triggerID)
	assert.True(t, ok)
	assert.True(t, retrieved.Enabled)
}

func TestEvaluateAllPattern(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
		FireCount:      0,
		MaxFires:       0,
	}

	engine.Register(trigger)
	event := eventbus.NewEvent(eventbus.EventTypeSystem, "test", eventbus.EventTargetSystem)
	matches := engine.Evaluate(event)

	assert.Len(t, matches, 1)
	assert.Equal(t, "agent-123", matches[0].AgentID)
	assert.Contains(t, matches[0].Message, "System event")
}

func TestEvaluateLifecyclePattern(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewLifecyclePattern(),
		PromptTemplate: "Lifecycle: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger)

	lifecycleEvent := eventbus.NewEvent(eventbus.EventTypeAgentCreated, "test", eventbus.EventTargetBroadcast)
	matches := engine.Evaluate(lifecycleEvent)
	assert.Len(t, matches, 1)

	systemEvent := eventbus.NewEvent(eventbus.EventTypeSystem, "test", eventbus.EventTargetSystem)
	matches = engine.Evaluate(systemEvent)
	assert.Len(t, matches, 0)
}

func TestEvaluateAgentSpawnedPattern(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewAgentSpawnedPattern("coder"),
		PromptTemplate: "Coder spawned: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger)

	matchingEvent := eventbus.NewEvent(eventbus.EventTypeAgentCreated, "test", eventbus.EventTargetBroadcast).
		WithPayload(map[string]interface{}{"name": "my-coder-agent"})
	matches := engine.Evaluate(matchingEvent)
	assert.Len(t, matches, 1)

	nonMatchingEvent := eventbus.NewEvent(eventbus.EventTypeAgentCreated, "test", eventbus.EventTargetBroadcast).
		WithPayload(map[string]interface{}{"name": "researcher"})
	matches = engine.Evaluate(nonMatchingEvent)
	assert.Len(t, matches, 0)
}

func TestMaxFires(t *testing.T) {
	engine := NewTriggerEngine(db)
	triggerID := NewTriggerID()

	trigger := &Trigger{
		ID:             triggerID,
		AgentID:        "agent-123",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event",
		Enabled:        true,
		CreatedAt:      time.Now(),
		FireCount:      0,
		MaxFires:       2,
	}

	engine.Register(trigger)
	event := eventbus.NewEvent(eventbus.EventTypeSystem, "test", eventbus.EventTargetSystem)

	assert.Len(t, engine.Evaluate(event), 1)
	assert.Len(t, engine.Evaluate(event), 1)
	assert.Len(t, engine.Evaluate(event), 0)
}

func TestRemoveAgentTriggers(t *testing.T) {
	engine := NewTriggerEngine(db)

	trigger1 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        "agent-1",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event 1",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}
	trigger2 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        "agent-1",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event 2",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}
	trigger3 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        "agent-2",
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event 3",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger1)
	engine.Register(trigger2)
	engine.Register(trigger3)

	assert.Len(t, engine.List("agent-1"), 2)
	engine.RemoveAgentTriggers("agent-1")
	assert.Len(t, engine.List("agent-1"), 0)
	assert.Len(t, engine.List("agent-2"), 1)
}

func TestTakeAndRestoreTriggers(t *testing.T) {
	engine := NewTriggerEngine(db)
	oldAgentID := "old-agent"
	newAgentID := "new-agent"

	trigger := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        oldAgentID,
		Pattern:        NewContentMatchPattern("deploy"),
		PromptTemplate: "Deploy alert: {{event}}",
		Enabled:        true,
		CreatedAt:      time.Now(),
		FireCount:      0,
		MaxFires:       5,
	}

	engine.Register(trigger)

	taken := engine.TakeAgentTriggers(oldAgentID)
	assert.Len(t, taken, 1)
	assert.Len(t, engine.List(oldAgentID), 0)

	restored := engine.RestoreTriggers(newAgentID, taken)
	assert.Equal(t, 1, restored)
	assert.Len(t, engine.List(newAgentID), 1)

	restoredTriggers := engine.List(newAgentID)
	assert.Equal(t, uint64(5), restoredTriggers[0].MaxFires)
}

func TestReassignAgentTriggers(t *testing.T) {
	engine := NewTriggerEngine(db)
	oldAgentID := "old-agent"
	newAgentID := "new-agent"

	trigger1 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        oldAgentID,
		Pattern:        NewAllPattern(),
		PromptTemplate: "Event 1",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}
	trigger2 := &Trigger{
		ID:             NewTriggerID(),
		AgentID:        oldAgentID,
		Pattern:        NewSystemPattern(),
		PromptTemplate: "Event 2",
		Enabled:        true,
		CreatedAt:      time.Now(),
	}

	engine.Register(trigger1)
	engine.Register(trigger2)

	count := engine.ReassignAgentTriggers(oldAgentID, newAgentID)
	assert.Equal(t, 2, count)
	assert.Len(t, engine.List(oldAgentID), 0)
	assert.Len(t, engine.List(newAgentID), 2)
}

func TestTriggerJSONSerialization(t *testing.T) {
	tests := []struct {
		name    string
		pattern TriggerPattern
	}{
		{"AllPattern", NewAllPattern()},
		{"LifecyclePattern", NewLifecyclePattern()},
		{"AgentSpawnedPattern", NewAgentSpawnedPattern("test")},
		{"AgentTerminatedPattern", NewAgentTerminatedPattern()},
		{"SystemPattern", NewSystemPattern()},
		{"SystemKeywordPattern", NewSystemKeywordPattern("keyword")},
		{"MemoryUpdatePattern", NewMemoryUpdatePattern()},
		{"MemoryKeyPattern", NewMemoryKeyPattern("key*")},
		{"ContentMatchPattern", NewContentMatchPattern("substring")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := &Trigger{
				ID:             NewTriggerID(),
				AgentID:        "agent-123",
				Pattern:        tt.pattern,
				PromptTemplate: "Test",
				Enabled:        true,
				CreatedAt:      time.Now(),
			}

			data, err := trigger.MarshalJSON()
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			var unmarshaled Trigger
			err = unmarshaled.UnmarshalJSON(data)
			assert.NoError(t, err)
			assert.Equal(t, trigger.ID, unmarshaled.ID)
			assert.Equal(t, trigger.Pattern.Type(), unmarshaled.Pattern.Type())
		})
	}
}
