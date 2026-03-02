// Package eventbus provides event bus functionality for OpenFang.
package eventbus

import (
	"container/list"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of an event.
type EventType string

const (
	EventTypeAgentCreated     EventType = "agent.created"
	EventTypeAgentStarted     EventType = "agent.started"
	EventTypeAgentStopped     EventType = "agent.stopped"
	EventTypeAgentDeleted     EventType = "agent.deleted"
	EventTypeMessageReceived  EventType = "message.received"
	EventTypeMessageSent      EventType = "message.sent"
	EventTypeHandActivated    EventType = "hand.activated"
	EventTypeHandDeactivated  EventType = "hand.deactivated"
	EventTypeHandCompleted    EventType = "hand.completed"
	EventTypeHandError        EventType = "hand.error"
	EventTypeChannelConnected EventType = "channel.connected"
	EventTypeChannelDisconnected EventType = "channel.disconnected"
	EventTypeWorkflowStarted  EventType = "workflow.started"
	EventTypeWorkflowCompleted EventType = "workflow.completed"
	EventTypeTriggerFired     EventType = "trigger.fired"
	EventTypeSystem           EventType = "system"
)

// EventTarget represents the target of an event.
type EventTarget string

const (
	EventTargetBroadcast EventTarget = "broadcast"
	EventTargetSystem    EventTarget = "system"
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

// EventBus is a pub/sub system for events.
type EventBus struct {
	mu          sync.RWMutex
	handlers    map[EventType][]EventHandler
	allHandlers []EventHandler
	history     *list.List
	historySize int
}

const defaultHistorySize = 1000

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers:    make(map[EventType][]EventHandler),
		allHandlers: make([]EventHandler, 0),
		history:     list.New(),
		historySize: defaultHistorySize,
	}
}

// Subscribe subscribes to a specific event type.
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// SubscribeAll subscribes to all events.
func (eb *EventBus) SubscribeAll(handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.allHandlers = append(eb.allHandlers, handler)
}

// Unsubscribe unsubscribes a handler from an event type.
func (eb *EventBus) Unsubscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	handlers, ok := eb.handlers[eventType]
	if !ok {
		return
	}

	for i, h := range handlers {
		if &h == &handler {
			eb.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// Publish publishes an event to the bus.
func (eb *EventBus) Publish(event *Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.addToHistory(event)

	for _, handler := range eb.allHandlers {
		go handler(event)
	}

	if handlers, ok := eb.handlers[event.Type]; ok {
		for _, handler := range handlers {
			go handler(event)
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
