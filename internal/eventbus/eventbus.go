// Package eventbus provides event bus functionality for OpenFang.
package eventbus

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of an event.
type EventType string

const (
	EventTypeAgentCreated        EventType = "agent.created"
	EventTypeAgentStarted        EventType = "agent.started"
	EventTypeAgentStopped        EventType = "agent.stopped"
	EventTypeAgentDeleted        EventType = "agent.deleted"
	EventTypeMessageReceived     EventType = "message.received"
	EventTypeMessageSent         EventType = "message.sent"
	EventTypeHandActivated       EventType = "hand.activated"
	EventTypeHandDeactivated     EventType = "hand.deactivated"
	EventTypeHandCompleted       EventType = "hand.completed"
	EventTypeHandError           EventType = "hand.error"
	EventTypeChannelConnected    EventType = "channel.connected"
	EventTypeChannelDisconnected EventType = "channel.disconnected"
	EventTypeWorkflowStarted     EventType = "workflow.started"
	EventTypeWorkflowCompleted   EventType = "workflow.completed"
	EventTypeTriggerFired        EventType = "trigger.fired"
	EventTypeSystem              EventType = "system"
)

// EventTarget represents the target of an event.
type EventTarget string

const (
	EventTargetBroadcast EventTarget = "broadcast"
	EventTargetSystem    EventTarget = "system"
	EventTargetAgent     EventTarget = "agent"
)

// Event represents a system event.
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Target    EventTarget            `json:"target"`
	AgentID   string                 `json:"agent_id,omitempty"`
	HandID    string                 `json:"hand_id,omitempty"`
	ChannelID string                 `json:"channel_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewEvent creates a new event.
func NewEvent(eventType EventType, source string, target EventTarget) *Event {
	return &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Target:    target,
		Timestamp: time.Now(),
	}
}

// WithAgentID sets the agent ID for the event.
func (e *Event) WithAgentID(agentID string) *Event {
	e.AgentID = agentID
	return e
}

// WithHandID sets the hand ID for the event.
func (e *Event) WithHandID(handID string) *Event {
	e.HandID = handID
	return e
}

// WithChannelID sets the channel ID for the event.
func (e *Event) WithChannelID(channelID string) *Event {
	e.ChannelID = channelID
	return e
}

// WithPayload sets the payload for the event.
func (e *Event) WithPayload(payload map[string]interface{}) *Event {
	e.Payload = payload
	return e
}

// EventHandler is a function that handles events.
type EventHandler func(event *Event)

// handlerEntry wraps an EventHandler with a unique ID for proper unsubscription.
type handlerEntry struct {
	id      string
	handler EventHandler
}

// EventBus is a pub/sub system for events.
type EventBus struct {
	mu          sync.RWMutex
	handlers    map[EventType][]handlerEntry
	allHandlers []handlerEntry
	history     *list.List
	historySize int
}

const defaultHistorySize = 1000

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers:    make(map[EventType][]handlerEntry),
		allHandlers: make([]handlerEntry, 0),
		history:     list.New(),
		historySize: defaultHistorySize,
	}
}

// Subscribe subscribes to a specific event type and returns a subscription ID for unsubscription.
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) string {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := uuid.New().String()
	entry := handlerEntry{id: id, handler: handler}
	eb.handlers[eventType] = append(eb.handlers[eventType], entry)
	return id
}

// SubscribeAll subscribes to all events and returns a subscription ID for unsubscription.
func (eb *EventBus) SubscribeAll(handler EventHandler) string {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := uuid.New().String()
	entry := handlerEntry{id: id, handler: handler}
	eb.allHandlers = append(eb.allHandlers, entry)
	return id
}

// Unsubscribe unsubscribes a handler from an event type using the subscription ID.
func (eb *EventBus) Unsubscribe(eventType EventType, subscriptionID string) bool {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	handlers, ok := eb.handlers[eventType]
	if !ok {
		return false
	}

	for i, entry := range handlers {
		if entry.id == subscriptionID {
			eb.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			return true
		}
	}
	return false
}

// UnsubscribeAll unsubscribes a handler from all events using the subscription ID.
func (eb *EventBus) UnsubscribeAll(subscriptionID string) bool {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for i, entry := range eb.allHandlers {
		if entry.id == subscriptionID {
			eb.allHandlers = append(eb.allHandlers[:i], eb.allHandlers[i+1:]...)
			return true
		}
	}
	return false
}

// Publish publishes an event to the bus.
func (eb *EventBus) Publish(event *Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.addToHistory(event)

	fmt.Printf("EventBus Publish to all handlers: %v\n", event.Payload)
	for _, entry := range eb.allHandlers {
		go entry.handler(event)
	}

	fmt.Printf("EventBus Publish type to specific handlers: %s, %v\n", event.Type, event.Payload)
	if handlers, ok := eb.handlers[event.Type]; ok {
		for _, entry := range handlers {
			go entry.handler(event)
		}
	}
}

// addToHistory adds an event to the history.
func (eb *EventBus) addToHistory(event *Event) {
	eb.history.PushBack(event)
	if eb.history.Len() > eb.historySize {
		eb.history.Remove(eb.history.Front())
	}
}

// GetHistory returns recent events from history.
func (eb *EventBus) GetHistory(limit int) []*Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if limit <= 0 {
		limit = eb.historySize
	}

	events := make([]*Event, 0, limit)
	count := 0

	for e := eb.history.Back(); e != nil && count < limit; e = e.Prev() {
		events = append(events, e.Value.(*Event))
		count++
	}

	return events
}

// ClearHistory clears the event history.
func (eb *EventBus) ClearHistory() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.history = list.New()
}
