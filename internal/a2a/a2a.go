// Package a2a provides Agent-to-Agent protocol support.
package a2a

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// A2A Types
// ---------------------------------------------------------------------------

// AgentCard describes an agent's capabilities to external systems.
type AgentCard struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	URL                string            `json:"url"`
	Version            string            `json:"version"`
	Capabilities       AgentCapabilities `json:"capabilities"`
	Skills             []AgentSkill      `json:"skills"`
	DefaultInputModes  []string          `json:"defaultInputModes"`
	DefaultOutputModes []string          `json:"defaultOutputModes"`
}

// AgentCapabilities describes agent capabilities.
type AgentCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// AgentSkill describes a capability.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples"`
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusSubmitted TaskStatus = "submitted"
	TaskStatusWorking   TaskStatus = "working"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCanceled  TaskStatus = "canceled"
)

// Message represents a message in a task.
type Message struct {
	Role    string `json:"role"` // "user" or "agent"
	Content string `json:"content"`
}

// Task represents an A2A task.
type Task struct {
	ID        string     `json:"id"`
	AgentID   string     `json:"agentId"`
	Status    TaskStatus `json:"status"`
	Messages  []Message  `json:"messages"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Result    string     `json:"result,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// A2ATaskStore stores A2A tasks.
type A2ATaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewA2ATaskStore creates a new task store.
func NewA2ATaskStore() *A2ATaskStore {
	return &A2ATaskStore{
		tasks: make(map[string]*Task),
	}
}

// CreateTask creates a new task.
func (s *A2ATaskStore) CreateTask(agentID string, messages []Message) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := &Task{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Status:    TaskStatusSubmitted,
		Messages:  messages,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.tasks[task.ID] = task
	return task
}

// GetTask returns a task by ID.
func (s *A2ATaskStore) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	return task, ok
}

// UpdateTaskStatus updates a task's status.
func (s *A2ATaskStore) UpdateTaskStatus(id string, status TaskStatus, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return &A2AError{Message: "task not found"}
	}

	task.Status = status
	task.Result = result
	task.UpdatedAt = time.Now()

	return nil
}

// SetTaskError sets an error on a task.
func (s *A2ATaskStore) SetTaskError(id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return &A2AError{Message: "task not found"}
	}

	task.Status = TaskStatusFailed
	task.Error = errMsg
	task.UpdatedAt = time.Now()

	return nil
}

// CancelTask cancels a task.
func (s *A2ATaskStore) CancelTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return &A2AError{Message: "task not found"}
	}

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		return &A2AError{Message: "cannot cancel completed or failed task"}
	}

	task.Status = TaskStatusCanceled
	task.UpdatedAt = time.Now()

	return nil
}

// ListTasks returns all tasks for an agent.
func (s *A2ATaskStore) ListTasks(agentID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*Task
	for _, task := range s.tasks {
		if task.AgentID == agentID {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// ---------------------------------------------------------------------------
// External Agent Support
// ---------------------------------------------------------------------------

// ExternalAgent represents a discovered external A2A agent.
type ExternalAgent struct {
	Card         AgentCard
	URL          string
	DiscoveredAt time.Time
}

// A2AClient provides methods to interact with external A2A agents.
type A2AClient struct {
	mu         sync.RWMutex
	agents     map[string]*ExternalAgent
	httpClient *http.Client
}

// NewA2AClient creates a new A2A client.
func NewA2AClient() *A2AClient {
	return &A2AClient{
		agents:     make(map[string]*ExternalAgent),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// DiscoverAgent discovers an A2A agent at the given URL.
func (c *A2AClient) DiscoverAgent(url string) (*AgentCard, error) {
	// Fetch agent card from well-known endpoint
	cardURL := url + "/.well-known/agent.json"

	resp, err := c.httpClient.Get(cardURL)
	if err != nil {
		return nil, &A2AError{Message: "failed to discover agent: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &A2AError{Message: "agent card not found"}
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, &A2AError{Message: "failed to parse agent card: " + err.Error()}
	}

	// Cache the agent
	c.mu.Lock()
	c.agents[card.Name] = &ExternalAgent{
		Card:         card,
		URL:          url,
		DiscoveredAt: time.Now(),
	}
	c.mu.Unlock()

	return &card, nil
}

// SendTask sends a task to an external agent.
func (c *A2AClient) SendTask(agentName string, message string) (string, error) {
	c.mu.RLock()
	agent, ok := c.agents[agentName]
	c.mu.RUnlock()

	if !ok {
		return "", &A2AError{Message: "agent not found: " + agentName}
	}

	// taskReq := map[string]interface{}{
	// 	"jsonrpc": "2.0",
	// 	"id":      uuid.New().String(),
	// 	"method":  "tasks/send",
	// 	"params": map[string]interface{}{
	// 		"message": map[string]string{
	// 			"role":    "user",
	// 			"content": message,
	// 		},
	// 	},
	// }



	resp, err := c.httpClient.Post(agent.URL+"/rpc", "application/json", nil)
	if err != nil {
		return "", &A2AError{Message: "failed to send task: " + err.Error()}
	}
	defer resp.Body.Close()

	// Parse response
	var rpcResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}

	// Extract task ID from response
	if result, ok := rpcResp["result"].(map[string]interface{}); ok {
		if id, ok := result["id"].(string); ok {
			return id, nil
		}
	}

	return "", &A2AError{Message: "failed to parse task response"}
}

// GetTaskStatus gets the status of an external task.
func (c *A2AClient) GetTaskStatus(agentName, taskID string) (*Task, error) {
	c.mu.RLock()
	agent, ok := c.agents[agentName]
	c.mu.RUnlock()

	if !ok {
		return nil, &A2AError{Message: "agent not found"}
	}
	_ = agent

	// 	"id": taskID,
	// }

	// For now, return a placeholder
	return &Task{
		ID:        taskID,
		AgentID:   agentName,
		Status:    TaskStatusWorking,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// ListExternalAgents returns all discovered external agents.
func (c *A2AClient) ListExternalAgents() []*ExternalAgent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agents := make([]*ExternalAgent, 0, len(c.agents))
	for _, a := range c.agents {
		agents = append(agents, a)
	}
	return agents
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// BuildAgentCard creates an agent card for OpenFang.
func BuildAgentCard(url string) *AgentCard {
	return &AgentCard{
		Name:        "OpenFang",
		Description: "OpenFang Agent OS - Autonomous agent operating system",
		URL:         url,
		Version:     "0.2.0",
		Capabilities: AgentCapabilities{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		Skills: []AgentSkill{
			{
				ID:          "chat",
				Name:        "Chat",
				Description: "General conversation and question answering",
				Tags:        []string{"chat", "qa"},
				Examples:    []string{"Hello", "What is the weather?"},
			},
			{
				ID:          "code",
				Name:        "Code",
				Description: "Programming and code assistance",
				Tags:        []string{"coding", "programming"},
				Examples:    []string{"Write a function", "Fix this bug"},
			},
			{
				ID:          "research",
				Name:        "Research",
				Description: "Information gathering and analysis",
				Tags:        []string{"research", "analysis"},
				Examples:    []string{"Research topic X", "Find information about Y"},
			},
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}
}

// A2AError represents an A2A error.
type A2AError struct {
	Message string
}

func (e *A2AError) Error() string {
	return e.Message
}
