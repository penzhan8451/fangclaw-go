package eventbus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEventBus(t *testing.T) {
	eb := NewEventBus()
	assert.NotNil(t, eb)
}

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventTypeSystem, "test-source", EventTargetSystem)
	assert.NotNil(t, event)
	assert.NotEmpty(t, event.ID)
	assert.Equal(t, EventTypeSystem, event.Type)
	assert.Equal(t, "test-source", event.Source)
	assert.Equal(t, EventTargetSystem, event.Target)
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
}

func TestEventWithAgentID(t *testing.T) {
	event := NewEvent(EventTypeAgentCreated, "test", EventTargetBroadcast).WithAgentID("agent-123")
	assert.Equal(t, "agent-123", event.AgentID)
}

func TestEventWithPayload(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	event := NewEvent(EventTypeSystem, "test", EventTargetSystem).WithPayload(payload)
	assert.Equal(t, "value", event.Payload["key"])
}

func TestPublishAndSubscribe(t *testing.T) {
	eb := NewEventBus()
	received := make(chan *Event, 1)

	subID := eb.Subscribe(EventTypeSystem, func(event *Event) {
		received <- event
	})

	event := NewEvent(EventTypeSystem, "test", EventTargetSystem)
	eb.Publish(event)

	select {
	case receivedEvent := <-received:
		assert.Equal(t, event.ID, receivedEvent.ID)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	assert.True(t, eb.Unsubscribe(EventTypeSystem, subID))
}

func TestSubscribeAll(t *testing.T) {
	eb := NewEventBus()
	received := make(chan *Event, 2)

	subID := eb.SubscribeAll(func(event *Event) {
		received <- event
	})

	eb.Publish(NewEvent(EventTypeSystem, "test1", EventTargetSystem))
	eb.Publish(NewEvent(EventTypeAgentCreated, "test2", EventTargetBroadcast))

	for i := 0; i < 2; i++ {
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for event")
		}
	}

	assert.True(t, eb.UnsubscribeAll(subID))
}

func TestHistory(t *testing.T) {
	eb := NewEventBus()

	for i := 0; i < 5; i++ {
		eb.Publish(NewEvent(EventTypeSystem, "test", EventTargetSystem))
	}

	history := eb.GetHistory(3)
	assert.Len(t, history, 3)

	allHistory := eb.GetHistory(0)
	assert.Len(t, allHistory, 5)
}

func TestClearHistory(t *testing.T) {
	eb := NewEventBus()
	eb.Publish(NewEvent(EventTypeSystem, "test", EventTargetSystem))

	assert.Len(t, eb.GetHistory(10), 1)
	eb.ClearHistory()
	assert.Len(t, eb.GetHistory(10), 0)
}

func TestUnsubscribeNonExistent(t *testing.T) {
	eb := NewEventBus()
	assert.False(t, eb.Unsubscribe(EventTypeSystem, "non-existent-id"))
	assert.False(t, eb.UnsubscribeAll("non-existent-id"))
}
