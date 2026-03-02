// Package triggers provides event-driven agent triggers.
package triggers

import (
	"time"

	"github.com/google/uuid"
)

// TriggerID represents a unique trigger identifier.
type TriggerID uuid.UUID

// NewTriggerID creates a new trigger ID.
func NewTriggerID() TriggerID {
	return TriggerID(uuid.New())
}

// ParseTriggerID parses a string into a TriggerID.
func ParseTriggerID(s string) (TriggerID, error) {
	id, err := uuid.Parse(s)
	return TriggerID(id), err
}

// TriggerPattern defines what events a trigger matches.
type TriggerPattern struct {
	Type        string `json:"type"`                   // "lifecycle", "agent_spawned", "agent_terminated", "system", "memory_update", "all", "content_match"
	NamePattern string `json:"name_pattern,omitempty"` // For agent_spawned
	Keyword     string `json:"keyword,omitempty"`      // For system_keyword
	KeyPattern  string `json:"key_pattern,omitempty"`  // For memory_key_pattern
	Substring   string `json:"substring,omitempty"`    // For content_match
}

// Trigger represents a registered trigger definition.
type Trigger struct {
	ID             TriggerID      `json:"id"`
	AgentID        string         `json:"agent_id"`
	Pattern        TriggerPattern `json:"pattern"`
	PromptTemplate string         `json:"prompt_template"`
	Enabled        bool           `json:"enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	FireCount      uint64         `json:"fire_count"`
	MaxFires       uint64         `json:"max_fires"`
}

// TriggerEngine manages event-to-agent routing.
type TriggerEngine struct {
	triggers      map[TriggerID]*Trigger
	agentTriggers map[string][]TriggerID // agent_id -> trigger IDs
}

// NewTriggerEngine creates a new trigger engine.
func NewTriggerEngine() *TriggerEngine {
	return &TriggerEngine{
		triggers:      make(map[TriggerID]*Trigger),
		agentTriggers: make(map[string][]TriggerID),
	}
}

// Register adds a new trigger.
func (e *TriggerEngine) Register(t *Trigger) error {
	e.triggers[t.ID] = t

	// Index by agent
	if _, ok := e.agentTriggers[t.AgentID]; !ok {
		e.agentTriggers[t.AgentID] = []TriggerID{}
	}
	e.agentTriggers[t.AgentID] = append(e.agentTriggers[t.AgentID], t.ID)

	return nil
}

// Get returns a trigger by ID.
func (e *TriggerEngine) Get(id TriggerID) (*Trigger, bool) {
	t, ok := e.triggers[id]
	return t, ok
}

// List returns all triggers, optionally filtered by agent ID.
func (e *TriggerEngine) List(agentID string) []*Trigger {
	triggers := make([]*Trigger, 0)
	for _, t := range e.triggers {
		if agentID == "" || t.AgentID == agentID {
			triggers = append(triggers, t)
		}
	}
	return triggers
}

// Delete removes a trigger by ID.
func (e *TriggerEngine) Delete(id TriggerID) bool {
	t, ok := e.triggers[id]
	if !ok {
		return false
	}

	// Remove from agent index
	agentID := t.AgentID
	if ids, ok := e.agentTriggers[agentID]; ok {
		newIDs := make([]TriggerID, 0)
		for _, tid := range ids {
			if tid != id {
				newIDs = append(newIDs, tid)
			}
		}
		e.agentTriggers[agentID] = newIDs
	}

	delete(e.triggers, id)
	return true
}

// Enable enables a trigger.
func (e *TriggerEngine) Enable(id TriggerID) bool {
	if t, ok := e.triggers[id]; ok {
		t.Enabled = true
		return true
	}
	return false
}

// Disable disables a trigger.
func (e *TriggerEngine) Disable(id TriggerID) bool {
	if t, ok := e.triggers[id]; ok {
		t.Enabled = false
		return true
	}
	return false
}

// Match checks if an event matches any trigger patterns.
func (e *TriggerEngine) Match(eventType string, data map[string]interface{}) []*Trigger {
	matched := make([]*Trigger, 0)

	for _, t := range e.triggers {
		if !t.Enabled {
			continue
		}

		// Check max fires
		if t.MaxFires > 0 && t.FireCount >= t.MaxFires {
			continue
		}

		// Simple matching logic
		switch t.Pattern.Type {
		case "lifecycle":
			if eventType == "lifecycle" {
				matched = append(matched, t)
			}
		case "agent_spawned":
			if eventType == "agent_spawned" {
				matched = append(matched, t)
			}
		case "agent_terminated":
			if eventType == "agent_terminated" {
				matched = append(matched, t)
			}
		case "system":
			if eventType == "system" {
				matched = append(matched, t)
			}
		case "memory_update":
			if eventType == "memory_update" {
				matched = append(matched, t)
			}
		case "all":
			matched = append(matched, t)
		}
	}

	return matched
}
