// Package triggers provides event-driven agent triggers.
package triggers

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
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

// String returns the string representation of the TriggerID.
func (t TriggerID) String() string {
	return uuid.UUID(t).String()
}

// MarshalJSON implements json.Marshaler for TriggerID.
func (t TriggerID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(t).String())
}

// UnmarshalJSON implements json.Unmarshaler for TriggerID.
func (t *TriggerID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	*t = TriggerID(id)
	return nil
}

// TriggerPatternType represents the type of trigger pattern.
type TriggerPatternType string

const (
	TriggerPatternTypeAll              TriggerPatternType = "all"
	TriggerPatternTypeLifecycle        TriggerPatternType = "lifecycle"
	TriggerPatternTypeAgentSpawned     TriggerPatternType = "agent_spawned"
	TriggerPatternTypeAgentTerminated  TriggerPatternType = "agent_terminated"
	TriggerPatternTypeSystem           TriggerPatternType = "system"
	TriggerPatternTypeSystemKeyword    TriggerPatternType = "system_keyword"
	TriggerPatternTypeMemoryUpdate     TriggerPatternType = "memory_update"
	TriggerPatternTypeMemoryKeyPattern TriggerPatternType = "memory_key_pattern"
	TriggerPatternTypeContentMatch     TriggerPatternType = "content_match"
)

// TriggerPattern defines what events a trigger matches.
// Uses a type-safe interface-based approach similar to Rust enums.
type TriggerPattern interface {
	Type() TriggerPatternType
	Matches(event *eventbus.Event, eventDesc string) bool
	json.Marshaler
	json.Unmarshaler
}

// BaseTriggerPattern provides the base implementation for all pattern types.
type BaseTriggerPattern struct {
	patternType TriggerPatternType
}

func (b *BaseTriggerPattern) Type() TriggerPatternType {
	return b.patternType
}

// AllPattern matches all events.
type AllPattern struct {
	BaseTriggerPattern
}

func NewAllPattern() *AllPattern {
	return &AllPattern{BaseTriggerPattern{patternType: TriggerPatternTypeAll}}
}

func (p *AllPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	return true
}

func (p *AllPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{"type": p.Type()})
}

func (p *AllPattern) UnmarshalJSON(data []byte) error {
	p.patternType = TriggerPatternTypeAll
	return nil
}

// LifecyclePattern matches any lifecycle event.
type LifecyclePattern struct {
	BaseTriggerPattern
}

func NewLifecyclePattern() *LifecyclePattern {
	return &LifecyclePattern{BaseTriggerPattern{patternType: TriggerPatternTypeLifecycle}}
}

func (p *LifecyclePattern) Matches(event *eventbus.Event, eventDesc string) bool {
	return strings.HasPrefix(string(event.Type), "agent.")
}

func (p *LifecyclePattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{"type": p.Type()})
}

func (p *LifecyclePattern) UnmarshalJSON(data []byte) error {
	p.patternType = TriggerPatternTypeLifecycle
	return nil
}

// AgentSpawnedPattern matches when an agent is spawned.
type AgentSpawnedPattern struct {
	BaseTriggerPattern
	NamePattern string `json:"name_pattern"`
}

func NewAgentSpawnedPattern(namePattern string) *AgentSpawnedPattern {
	return &AgentSpawnedPattern{
		BaseTriggerPattern: BaseTriggerPattern{patternType: TriggerPatternTypeAgentSpawned},
		NamePattern:        namePattern,
	}
}

func (p *AgentSpawnedPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	if event.Type != eventbus.EventTypeAgentCreated {
		return false
	}
	if p.NamePattern == "" || p.NamePattern == "*" {
		return true
	}
	if name, ok := event.Payload["name"].(string); ok {
		return strings.Contains(strings.ToLower(name), strings.ToLower(p.NamePattern))
	}
	return false
}

func (p *AgentSpawnedPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":         p.Type(),
		"name_pattern": p.NamePattern,
	})
}

func (p *AgentSpawnedPattern) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type        TriggerPatternType `json:"type"`
		NamePattern string             `json:"name_pattern"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	p.patternType = temp.Type
	p.NamePattern = temp.NamePattern
	return nil
}

// AgentTerminatedPattern matches when any agent is terminated.
type AgentTerminatedPattern struct {
	BaseTriggerPattern
}

func NewAgentTerminatedPattern() *AgentTerminatedPattern {
	return &AgentTerminatedPattern{BaseTriggerPattern{patternType: TriggerPatternTypeAgentTerminated}}
}

func (p *AgentTerminatedPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	return event.Type == eventbus.EventTypeAgentStopped || event.Type == eventbus.EventTypeAgentDeleted
}

func (p *AgentTerminatedPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{"type": p.Type()})
}

func (p *AgentTerminatedPattern) UnmarshalJSON(data []byte) error {
	p.patternType = TriggerPatternTypeAgentTerminated
	return nil
}

// SystemPattern matches any system event.
type SystemPattern struct {
	BaseTriggerPattern
}

func NewSystemPattern() *SystemPattern {
	return &SystemPattern{BaseTriggerPattern{patternType: TriggerPatternTypeSystem}}
}

func (p *SystemPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	return event.Type == eventbus.EventTypeSystem
}

func (p *SystemPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{"type": p.Type()})
}

func (p *SystemPattern) UnmarshalJSON(data []byte) error {
	p.patternType = TriggerPatternTypeSystem
	return nil
}

// SystemKeywordPattern matches a specific system event by keyword.
type SystemKeywordPattern struct {
	BaseTriggerPattern
	Keyword string `json:"keyword"`
}

func NewSystemKeywordPattern(keyword string) *SystemKeywordPattern {
	return &SystemKeywordPattern{
		BaseTriggerPattern: BaseTriggerPattern{patternType: TriggerPatternTypeSystemKeyword},
		Keyword:            keyword,
	}
}

func (p *SystemKeywordPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	if event.Type != eventbus.EventTypeSystem {
		return false
	}
	if p.Keyword == "" {
		return true
	}
	return strings.Contains(strings.ToLower(eventDesc), strings.ToLower(p.Keyword))
}

func (p *SystemKeywordPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    p.Type(),
		"keyword": p.Keyword,
	})
}

func (p *SystemKeywordPattern) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type    TriggerPatternType `json:"type"`
		Keyword string             `json:"keyword"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	p.patternType = temp.Type
	p.Keyword = temp.Keyword
	return nil
}

// MemoryUpdatePattern matches any memory update event.
type MemoryUpdatePattern struct {
	BaseTriggerPattern
}

func NewMemoryUpdatePattern() *MemoryUpdatePattern {
	return &MemoryUpdatePattern{BaseTriggerPattern{patternType: TriggerPatternTypeMemoryUpdate}}
}

func (p *MemoryUpdatePattern) Matches(event *eventbus.Event, eventDesc string) bool {
	if event.Payload == nil {
		return false
	}
	_, hasKey := event.Payload["key"]
	return hasKey
}

func (p *MemoryUpdatePattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{"type": p.Type()})
}

func (p *MemoryUpdatePattern) UnmarshalJSON(data []byte) error {
	p.patternType = TriggerPatternTypeMemoryUpdate
	return nil
}

// MemoryKeyPattern matches memory updates for a specific key pattern.
type MemoryKeyPattern struct {
	BaseTriggerPattern
	KeyPattern string `json:"key_pattern"`
}

func NewMemoryKeyPattern(keyPattern string) *MemoryKeyPattern {
	return &MemoryKeyPattern{
		BaseTriggerPattern: BaseTriggerPattern{patternType: TriggerPatternTypeMemoryKeyPattern},
		KeyPattern:         keyPattern,
	}
}

func (p *MemoryKeyPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	if event.Payload == nil {
		return false
	}
	key, ok := event.Payload["key"].(string)
	if !ok {
		return false
	}
	if p.KeyPattern == "" || p.KeyPattern == "*" {
		return true
	}
	return strings.Contains(strings.ToLower(key), strings.ToLower(p.KeyPattern))
}

func (p *MemoryKeyPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":        p.Type(),
		"key_pattern": p.KeyPattern,
	})
}

func (p *MemoryKeyPattern) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type       TriggerPatternType `json:"type"`
		KeyPattern string             `json:"key_pattern"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	p.patternType = temp.Type
	p.KeyPattern = temp.KeyPattern
	return nil
}

// ContentMatchPattern matches custom events by content substring.
type ContentMatchPattern struct {
	BaseTriggerPattern
	Substring string `json:"substring"`
}

func NewContentMatchPattern(substring string) *ContentMatchPattern {
	return &ContentMatchPattern{
		BaseTriggerPattern: BaseTriggerPattern{patternType: TriggerPatternTypeContentMatch},
		Substring:          substring,
	}
}

func (p *ContentMatchPattern) Matches(event *eventbus.Event, eventDesc string) bool {
	if p.Substring == "" {
		return true
	}
	return strings.Contains(strings.ToLower(eventDesc), strings.ToLower(p.Substring))
}

func (p *ContentMatchPattern) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":      p.Type(),
		"substring": p.Substring,
	})
}

func (p *ContentMatchPattern) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type      TriggerPatternType `json:"type"`
		Substring string             `json:"substring"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	p.patternType = temp.Type
	p.Substring = temp.Substring
	return nil
}

// UnmarshalTriggerPattern unmarshals JSON into the appropriate TriggerPattern type.
func UnmarshalTriggerPattern(data []byte) (TriggerPattern, error) {
	var temp struct {
		Type TriggerPatternType `json:"type"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, err
	}

	var pattern TriggerPattern
	switch temp.Type {
	case TriggerPatternTypeAll:
		pattern = NewAllPattern()
	case TriggerPatternTypeLifecycle:
		pattern = NewLifecyclePattern()
	case TriggerPatternTypeAgentSpawned:
		pattern = &AgentSpawnedPattern{}
	case TriggerPatternTypeAgentTerminated:
		pattern = NewAgentTerminatedPattern()
	case TriggerPatternTypeSystem:
		pattern = NewSystemPattern()
	case TriggerPatternTypeSystemKeyword:
		pattern = &SystemKeywordPattern{}
	case TriggerPatternTypeMemoryUpdate:
		pattern = NewMemoryUpdatePattern()
	case TriggerPatternTypeMemoryKeyPattern:
		pattern = &MemoryKeyPattern{}
	case TriggerPatternTypeContentMatch:
		pattern = &ContentMatchPattern{}
	default:
		return nil, fmt.Errorf("unknown trigger pattern type: %s", temp.Type)
	}

	if err := pattern.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return pattern, nil
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

// triggerJSON is used for JSON serialization/deserialization.
type triggerJSON struct {
	ID             TriggerID       `json:"id"`
	AgentID        string          `json:"agent_id"`
	Pattern        json.RawMessage `json:"pattern"`
	PromptTemplate string          `json:"prompt_template"`
	Enabled        bool            `json:"enabled"`
	CreatedAt      time.Time       `json:"created_at"`
	FireCount      uint64          `json:"fire_count"`
	MaxFires       uint64          `json:"max_fires"`
}

func (t *Trigger) MarshalJSON() ([]byte, error) {
	patternJSON, err := t.Pattern.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(triggerJSON{
		ID:             t.ID,
		AgentID:        t.AgentID,
		Pattern:        patternJSON,
		PromptTemplate: t.PromptTemplate,
		Enabled:        t.Enabled,
		CreatedAt:      t.CreatedAt,
		FireCount:      t.FireCount,
		MaxFires:       t.MaxFires,
	})
}

func (t *Trigger) UnmarshalJSON(data []byte) error {
	var tj triggerJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return err
	}
	pattern, err := UnmarshalTriggerPattern(tj.Pattern)
	if err != nil {
		return err
	}
	t.ID = tj.ID
	t.AgentID = tj.AgentID
	t.Pattern = pattern
	t.PromptTemplate = tj.PromptTemplate
	t.Enabled = tj.Enabled
	t.CreatedAt = tj.CreatedAt
	t.FireCount = tj.FireCount
	t.MaxFires = tj.MaxFires
	return nil
}

// MatchResult represents a trigger match result.
type MatchResult struct {
	TriggerID TriggerID
	AgentID   string
	Message   string
}

// TriggerEngine manages event-to-agent routing.
type TriggerEngine struct {
	mu            sync.RWMutex
	triggers      map[TriggerID]*Trigger
	agentTriggers map[string][]TriggerID // agent_id -> trigger IDs
	db            *memory.DB
}

// NewTriggerEngine creates a new trigger engine.
func NewTriggerEngine(db *memory.DB) *TriggerEngine {
	return &TriggerEngine{
		triggers:      make(map[TriggerID]*Trigger),
		agentTriggers: make(map[string][]TriggerID),
		db:            db,
	}
}

// LoadFromDB loads triggers from database.
func (e *TriggerEngine) LoadFromDB() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	records, err := e.db.ListTriggers()
	if err != nil {
		return err
	}

	for _, record := range records {
		id, err := ParseTriggerID(record.ID)
		if err != nil {
			continue
		}

		pattern, err := UnmarshalTriggerPattern([]byte(record.Pattern))
		if err != nil {
			continue
		}

		trigger := &Trigger{
			ID:             id,
			AgentID:        record.AgentID,
			Pattern:        pattern,
			PromptTemplate: record.PromptTemplate,
			Enabled:        record.Enabled,
			CreatedAt:      record.CreatedAt,
			FireCount:      uint64(record.FireCount),
			MaxFires:       uint64(record.MaxFires),
		}

		e.triggers[id] = trigger
		if _, ok := e.agentTriggers[trigger.AgentID]; !ok {
			e.agentTriggers[trigger.AgentID] = []TriggerID{}
		}
		e.agentTriggers[trigger.AgentID] = append(e.agentTriggers[trigger.AgentID], id)
	}

	return nil
}

// saveToDB saves a trigger to database.
func (e *TriggerEngine) saveToDB(t *Trigger) error {
	if e.db == nil {
		return nil
	}

	patternBytes, err := t.Pattern.MarshalJSON()
	if err != nil {
		return err
	}

	record := &memory.TriggerRecord{
		ID:             t.ID.String(),
		AgentID:        t.AgentID,
		Pattern:        string(patternBytes),
		PromptTemplate: t.PromptTemplate,
		Enabled:        t.Enabled,
		CreatedAt:      t.CreatedAt,
		FireCount:      int(t.FireCount),
		MaxFires:       int(t.MaxFires),
	}

	return e.db.SaveTrigger(record)
}

// Register adds a new trigger.
func (e *TriggerEngine) Register(t *Trigger) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.triggers[t.ID] = t

	// Index by agent
	if _, ok := e.agentTriggers[t.AgentID]; !ok {
		e.agentTriggers[t.AgentID] = []TriggerID{}
	}
	e.agentTriggers[t.AgentID] = append(e.agentTriggers[t.AgentID], t.ID)

	// Save to database
	if err := e.saveToDB(t); err != nil {
		return err
	}

	return nil
}

// Get returns a trigger by ID.
func (e *TriggerEngine) Get(id TriggerID) (*Trigger, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	t, ok := e.triggers[id]
	return t, ok
}

// List returns all triggers, optionally filtered by agent ID.
func (e *TriggerEngine) List(agentID string) []*Trigger {
	e.mu.RLock()
	defer e.mu.RUnlock()

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
	e.mu.Lock()
	defer e.mu.Unlock()

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

	// Delete from database
	if e.db != nil {
		e.db.DeleteTrigger(id.String())
	}

	return true
}

// RemoveAgentTriggers removes all triggers for an agent.
func (e *TriggerEngine) RemoveAgentTriggers(agentID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if ids, ok := e.agentTriggers[agentID]; ok {
		for _, id := range ids {
			delete(e.triggers, id)
		}
		delete(e.agentTriggers, agentID)
	}
}

// TakeAgentTriggers takes all triggers for an agent, removing them from the engine.
func (e *TriggerEngine) TakeAgentTriggers(agentID string) []*Trigger {
	e.mu.Lock()
	defer e.mu.Unlock()

	ids, ok := e.agentTriggers[agentID]
	if !ok {
		return []*Trigger{}
	}

	taken := make([]*Trigger, 0, len(ids))
	for _, id := range ids {
		if t, ok := e.triggers[id]; ok {
			taken = append(taken, t)
			delete(e.triggers, id)
		}
	}
	delete(e.agentTriggers, agentID)

	return taken
}

// RestoreTriggers restores previously taken triggers under a new agent ID.
func (e *TriggerEngine) RestoreTriggers(newAgentID string, triggers []*Trigger) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := len(triggers)
	for _, old := range triggers {
		newID := NewTriggerID()
		newTrigger := &Trigger{
			ID:             newID,
			AgentID:        newAgentID,
			Pattern:        old.Pattern,
			PromptTemplate: old.PromptTemplate,
			Enabled:        old.Enabled,
			CreatedAt:      old.CreatedAt,
			FireCount:      old.FireCount,
			MaxFires:       old.MaxFires,
		}
		e.triggers[newID] = newTrigger
		e.agentTriggers[newAgentID] = append(e.agentTriggers[newAgentID], newID)
		// Save to database
		e.saveToDB(newTrigger)
	}

	return count
}

// ReassignAgentTriggers reassigns all triggers from one agent to another.
func (e *TriggerEngine) ReassignAgentTriggers(oldAgentID, newAgentID string) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	ids, ok := e.agentTriggers[oldAgentID]
	if !ok {
		return 0
	}

	count := len(ids)
	for _, id := range ids {
		if t, ok := e.triggers[id]; ok {
			t.AgentID = newAgentID
			// Save to database
			e.saveToDB(t)
		}
	}
	e.agentTriggers[newAgentID] = append(e.agentTriggers[newAgentID], ids...)
	delete(e.agentTriggers, oldAgentID)

	return count
}

// Enable enables a trigger.
func (e *TriggerEngine) Enable(id TriggerID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t, ok := e.triggers[id]; ok {
		t.Enabled = true
		// Save to database
		e.saveToDB(t)
		return true
	}
	return false
}

// Disable disables a trigger.
func (e *TriggerEngine) Disable(id TriggerID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t, ok := e.triggers[id]; ok {
		t.Enabled = false
		// Save to database
		e.saveToDB(t)
		return true
	}
	return false
}

// describeEvent generates a human-readable description of an event.
func describeEvent(event *eventbus.Event) string {
	switch event.Type {
	case eventbus.EventTypeAgentCreated:
		name, _ := event.Payload["name"].(string)
		return fmt.Sprintf("Agent created: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeAgentStarted:
		name, _ := event.Payload["name"].(string)
		return fmt.Sprintf("Agent started: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeAgentStopped:
		name, _ := event.Payload["name"].(string)
		reason, _ := event.Payload["reason"].(string)
		if reason != "" {
			return fmt.Sprintf("Agent stopped: %s (id: %s, reason: %s)", name, event.AgentID, reason)
		}
		return fmt.Sprintf("Agent stopped: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeAgentDeleted:
		name, _ := event.Payload["name"].(string)
		return fmt.Sprintf("Agent deleted: %s (id: %s)", name, event.AgentID)
	case eventbus.EventTypeMessageReceived:
		channel, _ := event.Payload["channel"].(string)
		content, _ := event.Payload["content"].(string)
		if len(content) > 200 {
			content = content[:197] + "..."
		}
		return fmt.Sprintf("Message received on %s: %s", channel, content)
	case eventbus.EventTypeMessageSent:
		channel, _ := event.Payload["channel"].(string)
		content, _ := event.Payload["content"].(string)
		if len(content) > 200 {
			content = content[:197] + "..."
		}
		return fmt.Sprintf("Message sent to %s: %s", channel, content)
	case eventbus.EventTypeSystem:
		subtype, _ := event.Payload["subtype"].(string)
		if subtype != "" {
			return fmt.Sprintf("System event: %s", subtype)
		}
		return "System event"
	case eventbus.EventTypeTriggerFired:
		triggerID, _ := event.Payload["trigger_id"].(string)
		return fmt.Sprintf("Trigger fired: %s", triggerID)
	case eventbus.EventTypeWorkflowStarted:
		workflowID, _ := event.Payload["workflow_id"].(string)
		return fmt.Sprintf("Workflow started: %s", workflowID)
	case eventbus.EventTypeWorkflowCompleted:
		workflowID, _ := event.Payload["workflow_id"].(string)
		return fmt.Sprintf("Workflow completed: %s", workflowID)
	default:
		return fmt.Sprintf("Event: %s", event.Type)
	}
}

// Evaluate evaluates an event against all triggers and returns matches.
func (e *TriggerEngine) Evaluate(event *eventbus.Event) []MatchResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	eventDesc := describeEvent(event)
	matches := make([]MatchResult, 0)

	for _, t := range e.triggers {
		if !t.Enabled {
			continue
		}

		// Check max fires
		if t.MaxFires > 0 && t.FireCount >= t.MaxFires {
			t.Enabled = false
			e.saveToDB(t)
			continue
		}

		if t.Pattern.Matches(event, eventDesc) {
			message := strings.ReplaceAll(t.PromptTemplate, "{{event}}", eventDesc)
			matches = append(matches, MatchResult{
				TriggerID: t.ID,
				AgentID:   t.AgentID,
				Message:   message,
			})
			t.FireCount++
			e.saveToDB(t)
		}
	}

	return matches
}

// Match checks if an event matches any trigger patterns (deprecated, use Evaluate instead).
func (e *TriggerEngine) Match(eventType string, data map[string]interface{}) []*Trigger {
	e.mu.RLock()
	defer e.mu.RUnlock()

	matched := make([]*Trigger, 0)
	for _, t := range e.triggers {
		if !t.Enabled {
			continue
		}

		// Check max fires
		if t.MaxFires > 0 && t.FireCount >= t.MaxFires {
			continue
		}

		// Simple matching logic for backward compatibility
		switch t.Pattern.Type() {
		case TriggerPatternTypeLifecycle:
			if strings.HasPrefix(eventType, "agent.") {
				matched = append(matched, t)
			}
		case TriggerPatternTypeAgentSpawned:
			if eventType == string(eventbus.EventTypeAgentCreated) {
				matched = append(matched, t)
			}
		case TriggerPatternTypeAgentTerminated:
			if eventType == string(eventbus.EventTypeAgentStopped) || eventType == string(eventbus.EventTypeAgentDeleted) {
				matched = append(matched, t)
			}
		case TriggerPatternTypeSystem:
			if eventType == string(eventbus.EventTypeSystem) {
				matched = append(matched, t)
			}
		case TriggerPatternTypeMemoryUpdate:
			if strings.Contains(eventType, "memory") {
				matched = append(matched, t)
			}
		case TriggerPatternTypeAll:
			matched = append(matched, t)
		}
	}

	return matched
}
