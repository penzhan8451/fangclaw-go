package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type AgentEntry struct {
	ID         types.AgentID       `json:"id"`
	Name       string              `json:"name"`
	State      types.AgentState    `json:"state"`
	Mode       string              `json:"mode"`
	Tags       []string            `json:"tags"`
	Manifest   types.AgentManifest `json:"manifest"`
	CreatedAt  time.Time           `json:"created_at"`
	LastActive time.Time           `json:"last_active"`
	Children   []types.AgentID     `json:"children"`
}

// GetID returns the agent ID as a string.
func (e *AgentEntry) GetID() string {
	return e.ID.String()
}

// GetName returns the agent name.
func (e *AgentEntry) GetName() string {
	return e.Name
}

// GetTags returns the agent tags.
func (e *AgentEntry) GetTags() []string {
	return e.Tags
}

// GetCreatedAt returns the agent creation time.
func (e *AgentEntry) GetCreatedAt() time.Time {
	return e.CreatedAt
}

type AgentRegistry struct {
	mu        sync.RWMutex
	agents    map[types.AgentID]*AgentEntry
	nameIndex map[string]types.AgentID
	tagIndex  map[string][]types.AgentID
	dataDir   string
}

func NewAgentRegistry(dataDir string) *AgentRegistry {
	return &AgentRegistry{
		agents:    make(map[types.AgentID]*AgentEntry),
		nameIndex: make(map[string]types.AgentID),
		tagIndex:  make(map[string][]types.AgentID),
		dataDir:   dataDir,
	}
}

func (r *AgentRegistry) Register(entry *AgentEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nameIndex[entry.Name]; exists {
		return fmt.Errorf("agent with name '%s' already exists", entry.Name)
	}

	r.agents[entry.ID] = entry
	r.nameIndex[entry.Name] = entry.ID

	for _, tag := range entry.Tags {
		r.tagIndex[tag] = append(r.tagIndex[tag], entry.ID)
	}

	r.saveToDisk()

	return nil
}

func (r *AgentRegistry) Get(id types.AgentID) *AgentEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[id]
}

func (r *AgentRegistry) FindByName(name string) *AgentEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if id, exists := r.nameIndex[name]; exists {
		return r.agents[id]
	}
	return nil
}

func (r *AgentRegistry) SetState(id types.AgentID, state types.AgentState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.agents[id]
	if !exists {
		return fmt.Errorf("agent not found: %s", id)
	}

	entry.State = state
	entry.LastActive = time.Now()
	r.saveToDisk()
	return nil
}

func (r *AgentRegistry) SetMode(id types.AgentID, mode string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.agents[id]
	if !exists {
		return fmt.Errorf("agent not found: %s", id)
	}

	entry.Mode = mode
	entry.LastActive = time.Now()
	r.saveToDisk()
	return nil
}

func (r *AgentRegistry) Remove(id types.AgentID) (*AgentEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	delete(r.agents, id)
	delete(r.nameIndex, entry.Name)

	for tag := range r.tagIndex {
		ids := r.tagIndex[tag]
		newIds := make([]types.AgentID, 0, len(ids)-1)
		for _, aid := range ids {
			if aid != id {
				newIds = append(newIds, aid)
			}
		}
		if len(newIds) == 0 {
			delete(r.tagIndex, tag)
		} else {
			r.tagIndex[tag] = newIds
		}
	}

	r.saveToDisk()
	return entry, nil
}

func (r *AgentRegistry) List() []*AgentEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*AgentEntry, 0, len(r.agents))
	for _, entry := range r.agents {
		entries = append(entries, entry)
	}
	return entries
}

func (r *AgentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

func (r *AgentRegistry) AddChild(parentID, childID types.AgentID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.agents[parentID]; exists {
		entry.Children = append(entry.Children, childID)
		r.saveToDisk()
	}
}

func (r *AgentRegistry) saveToDisk() {
	if r.dataDir == "" {
		return
	}

	agentsFile := filepath.Join(r.dataDir, "agents.json")
	entries := make([]*AgentEntry, 0, len(r.agents))
	for _, entry := range r.agents {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling agents: %v\n", err)
		return
	}

	if err := os.WriteFile(agentsFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing agents file: %v\n", err)
	}
}

func (r *AgentRegistry) LoadFromDisk() {
	if r.dataDir == "" {
		return
	}

	agentsFile := filepath.Join(r.dataDir, "agents.json")
	if _, err := os.Stat(agentsFile); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(agentsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading agents file: %v\n", err)
		return
	}

	var entries []*AgentEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling agents: %v\n", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		r.agents[entry.ID] = entry
		r.nameIndex[entry.Name] = entry.ID
		for _, tag := range entry.Tags {
			r.tagIndex[tag] = append(r.tagIndex[tag], entry.ID)
		}
	}
}
