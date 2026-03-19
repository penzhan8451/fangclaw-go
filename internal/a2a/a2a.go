// Package a2a provides Agent-to-Agent protocol support.
package a2a

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/approvals"
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

// A2aTaskStatus represents the status of a task.
type A2aTaskStatus string

const (
	A2aTaskStatusSubmitted     A2aTaskStatus = "submitted"
	A2aTaskStatusWorking       A2aTaskStatus = "working"
	A2aTaskStatusInputRequired A2aTaskStatus = "inputRequired"
	A2aTaskStatusCompleted     A2aTaskStatus = "completed"
	A2aTaskStatusFailed        A2aTaskStatus = "failed"
	A2aTaskStatusCancelled     A2aTaskStatus = "cancelled"
)

// A2aTaskStatusWrapper accepts either a bare status string ("completed")
// or the object form ({"state": "completed", "message": null})
// used by some A2A implementations.
type A2aTaskStatusWrapper struct {
	State   A2aTaskStatus   `json:"state"`
	Message json.RawMessage `json:"message,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for A2aTaskStatusWrapper.
func (w *A2aTaskStatusWrapper) UnmarshalJSON(data []byte) error {
	type objectForm struct {
		State   A2aTaskStatus   `json:"state"`
		Message json.RawMessage `json:"message,omitempty"`
	}

	var obj objectForm
	if err := json.Unmarshal(data, &obj); err == nil {
		w.State = obj.State
		w.Message = obj.Message
		return nil
	}

	var status A2aTaskStatus
	if err := json.Unmarshal(data, &status); err == nil {
		w.State = status
		w.Message = nil
		return nil
	}

	return nil
}

// MarshalJSON implements custom marshaling for A2aTaskStatusWrapper.
func (w A2aTaskStatusWrapper) MarshalJSON() ([]byte, error) {
	if w.Message != nil {
		type objectForm struct {
			State   A2aTaskStatus   `json:"state"`
			Message json.RawMessage `json:"message,omitempty"`
		}
		return json.Marshal(objectForm{
			State:   w.State,
			Message: w.Message,
		})
	}
	return json.Marshal(w.State)
}

// A2aPart represents a message content part.
type A2aPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Name     string          `json:"name,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// A2aMessage represents a message in a task.
type A2aMessage struct {
	Role  string    `json:"role"`
	Parts []A2aPart `json:"parts"`
}

// A2aArtifact represents an artifact produced by a task.
type A2aArtifact struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Index       *uint32         `json:"index,omitempty"`
	LastChunk   *bool           `json:"lastChunk,omitempty"`
	Parts       []A2aPart       `json:"parts"`
}

// A2aTask represents an A2A task.
type A2aTask struct {
	ID        string               `json:"id"`
	AgentID   string               `json:"agentId"`
	AgentName string               `json:"agentName,omitempty"`
	SessionID *string              `json:"sessionId,omitempty"`
	Status    A2aTaskStatusWrapper `json:"status"`
	Messages  []A2aMessage         `json:"messages"`
	Artifacts []A2aArtifact        `json:"artifacts"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

// ---------------------------------------------------------------------------
// A2A Task Store
// ---------------------------------------------------------------------------

// A2ATaskStatusCallback is called when a task's status changes.
type A2ATaskStatusCallback func(task *A2aTask, oldStatus A2aTaskStatus, newStatus A2aTaskStatus)

// A2ATaskStore stores A2A tasks.
type A2ATaskStore struct {
	mu             sync.RWMutex
	tasks          map[string]*A2aTask
	maxTasks       int
	metrics        *A2AMetrics
	approvalMgr    *approvals.ApprovalManager
	rateLimiter    *A2ARateLimiter
	statusCallback A2ATaskStatusCallback
	eventStore     *A2AEventStore
}

// NewA2ATaskStore creates a new task store with a capacity limit.
func NewA2ATaskStore(maxTasks int) *A2ATaskStore {
	if maxTasks <= 0 {
		maxTasks = 1000
	}
	return &A2ATaskStore{
		tasks:    make(map[string]*A2aTask),
		maxTasks: maxTasks,
		metrics:  NewA2AMetrics(),
	}
}

// NewA2ATaskStoreWithOptions creates a new task store with all options.
func NewA2ATaskStoreWithOptions(maxTasks int, metrics *A2AMetrics, approvalMgr *approvals.ApprovalManager, rateLimiter *A2ARateLimiter, eventStore *A2AEventStore) *A2ATaskStore {
	if maxTasks <= 0 {
		maxTasks = 1000
	}
	if metrics == nil {
		metrics = NewA2AMetrics()
	}
	return &A2ATaskStore{
		tasks:       make(map[string]*A2aTask),
		maxTasks:    maxTasks,
		metrics:     metrics,
		approvalMgr: approvalMgr,
		rateLimiter: rateLimiter,
		eventStore:  eventStore,
	}
}

// SetEventStore sets the event store.
func (s *A2ATaskStore) SetEventStore(eventStore *A2AEventStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventStore = eventStore
}

// addEvent adds an event to the event store if set.
func (s *A2ATaskStore) addEvent(kind A2AEventKind, sourceID, sourceName, targetID, targetName, detail string, payload map[string]interface{}) {
	s.mu.RLock()
	eventStore := s.eventStore
	s.mu.RUnlock()

	if eventStore != nil {
		event := &A2AEvent{
			ID:         uuid.New().String(),
			Timestamp:  time.Now(),
			Kind:       kind,
			SourceID:   sourceID,
			SourceName: sourceName,
			TargetID:   targetID,
			TargetName: targetName,
			Detail:     detail,
			Payload:    payload,
		}
		eventStore.AddEvent(event)
	}
}

// SetMetrics sets the metrics collector.
func (s *A2ATaskStore) SetMetrics(metrics *A2AMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = metrics
}

// SetApprovalManager sets the approval manager.
func (s *A2ATaskStore) SetApprovalManager(approvalMgr *approvals.ApprovalManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvalMgr = approvalMgr
}

// SetRateLimiter sets the rate limiter.
func (s *A2ATaskStore) SetRateLimiter(rateLimiter *A2ARateLimiter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rateLimiter = rateLimiter
}

// Metrics returns the metrics collector.
func (s *A2ATaskStore) Metrics() *A2AMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

// SetStatusCallback sets the callback for task status changes.
func (s *A2ATaskStore) SetStatusCallback(callback A2ATaskStatusCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCallback = callback
}

// notifyStatusChange notifies the status callback if set.
func (s *A2ATaskStore) notifyStatusChange(task *A2aTask, oldStatus A2aTaskStatus, newStatus A2aTaskStatus) {
	s.mu.RLock()
	callback := s.statusCallback
	s.mu.RUnlock()

	if callback != nil {
		callback(task, oldStatus, newStatus)
	}
}

// CreateTask creates a new task.
func (s *A2ATaskStore) CreateTask(agentID string, agentName string, messages []A2aMessage, sessionID *string) *A2aTask {
	s.mu.Lock()

	if len(s.tasks) >= s.maxTasks {
		for id, task := range s.tasks {
			if task.Status.State == A2aTaskStatusCompleted ||
				task.Status.State == A2aTaskStatusFailed ||
				task.Status.State == A2aTaskStatusCancelled {
				delete(s.tasks, id)
				break
			}
		}
	}

	task := &A2aTask{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		AgentName: agentName,
		SessionID: sessionID,
		Status: A2aTaskStatusWrapper{
			State: A2aTaskStatusSubmitted,
		},
		Messages:  messages,
		Artifacts: []A2aArtifact{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.tasks[task.ID] = task
	taskID := task.ID

	if s.metrics != nil {
		s.metrics.RecordTaskCreated()
	}

	s.mu.Unlock()
	s.addEvent(A2AEventKindTaskCreated, "system", "System", agentID, agentName, "Task created", map[string]interface{}{"task_id": taskID})

	s.notifyStatusChange(task, task.Status.State, task.Status.State)

	return task
}

// AddTask adds an existing task to the store.
func (s *A2ATaskStore) AddTask(task *A2aTask) {
	s.mu.Lock()

	// Check if task already exists
	_, exists := s.tasks[task.ID]

	if len(s.tasks) >= s.maxTasks {
		for id, t := range s.tasks {
			if t.Status.State == A2aTaskStatusCompleted ||
				t.Status.State == A2aTaskStatusFailed ||
				t.Status.State == A2aTaskStatusCancelled {
				delete(s.tasks, id)
				break
			}
		}
	}

	// Ensure CreatedAt and UpdatedAt are valid
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = time.Now()
	}

	s.tasks[task.ID] = task

	if s.metrics != nil {
		s.metrics.RecordTaskCreated()
	}

	s.mu.Unlock()

	agentName := task.AgentName
	if agentName == "" {
		agentName = task.AgentID
	}

	// Only add event and notify if it's a new task
	if !exists {
		s.addEvent(A2AEventKindTaskCreated, "system", "System", task.AgentID, agentName, "Task created", map[string]interface{}{"task_id": task.ID})
		s.notifyStatusChange(task, task.Status.State, task.Status.State)
	} else {
		// If task existed, send a status update notification to refresh frontend
		s.notifyStatusChange(task, task.Status.State, task.Status.State)
	}
}

// GetTask returns a task by ID.
func (s *A2ATaskStore) GetTask(id string) (*A2aTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	return task, ok
}

// UpdateTaskStatus updates a task's status.
func (s *A2ATaskStore) UpdateTaskStatus(id string, status A2aTaskStatus) bool {
	s.mu.Lock()

	task, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return false
	}

	oldStatus := task.Status.State
	task.Status.State = status
	task.UpdatedAt = time.Now()

	s.mu.Unlock()

	if oldStatus != status {
		s.notifyStatusChange(task, oldStatus, status)
		s.addEvent(A2AEventKindTaskStatusUpdate, "system", "System", task.AgentID, "", fmt.Sprintf("Status changed from %s to %s", oldStatus, status), map[string]interface{}{"task_id": id, "old_status": oldStatus, "new_status": status})
	}
	return true
}

// CompleteTask completes a task with a response message and optional artifacts.
func (s *A2ATaskStore) CompleteTask(id string, response A2aMessage, artifacts []A2aArtifact) {
	s.mu.Lock()

	task, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return
	}

	oldStatus := task.Status.State
	// Only append response if task doesn't already have messages from external agent
	if len(task.Messages) == 0 || task.Messages[len(task.Messages)-1].Role != response.Role {
		task.Messages = append(task.Messages, response)
	}
	// Only append artifacts if not already present
	if len(artifacts) > 0 {
		task.Artifacts = append(task.Artifacts, artifacts...)
	}
	task.Status.State = A2aTaskStatusCompleted
	task.UpdatedAt = time.Now()

	if s.metrics != nil {
		duration := task.UpdatedAt.Sub(task.CreatedAt)
		s.metrics.RecordTaskCompleted(duration)
	}

	s.mu.Unlock()

	if oldStatus != A2aTaskStatusCompleted {
		s.notifyStatusChange(task, oldStatus, A2aTaskStatusCompleted)
		s.addEvent(A2AEventKindTaskCompleted, "system", "System", task.AgentID, "", "Task completed", map[string]interface{}{"task_id": id})
	}
}

// FailTask fails a task with an error message.
func (s *A2ATaskStore) FailTask(id string, errorMessage A2aMessage) {
	s.mu.Lock()

	task, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return
	}

	oldStatus := task.Status.State
	// Only append error message if task doesn't already have it
	if len(task.Messages) == 0 || task.Messages[len(task.Messages)-1].Role != errorMessage.Role {
		task.Messages = append(task.Messages, errorMessage)
	}
	task.Status.State = A2aTaskStatusFailed
	task.UpdatedAt = time.Now()

	if s.metrics != nil {
		s.metrics.RecordTaskFailed()
	}

	s.mu.Unlock()

	if oldStatus != A2aTaskStatusFailed {
		s.notifyStatusChange(task, oldStatus, A2aTaskStatusFailed)
		s.addEvent(A2AEventKindTaskFailed, "system", "System", task.AgentID, "", "Task failed", map[string]interface{}{"task_id": id})
	}
}

// CancelTask cancels a task.
func (s *A2ATaskStore) CancelTask(id string) bool {
	s.mu.Lock()

	task, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return false
	}

	if task.Status.State == A2aTaskStatusCompleted ||
		task.Status.State == A2aTaskStatusFailed ||
		task.Status.State == A2aTaskStatusCancelled {
		s.mu.Unlock()
		return false
	}

	oldStatus := task.Status.State
	task.Status.State = A2aTaskStatusCancelled
	task.UpdatedAt = time.Now()

	if s.metrics != nil {
		s.metrics.RecordTaskCancelled()
	}

	s.mu.Unlock()

	if oldStatus != A2aTaskStatusCancelled {
		s.notifyStatusChange(task, oldStatus, A2aTaskStatusCancelled)
		s.addEvent(A2AEventKindTaskCancelled, "system", "System", task.AgentID, "", "Task cancelled", map[string]interface{}{"task_id": id})
	}

	return true
}

// ListTasks returns all tasks for an agent, sorted by creation time (newest first).
func (s *A2ATaskStore) ListTasks() []*A2aTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*A2aTask
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	// Sort tasks by creation time in descending order (newest first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// Len returns the number of tasks in the store.
func (s *A2ATaskStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

// IsEmpty returns true if the store is empty.
func (s *A2ATaskStore) IsEmpty() bool {
	return s.Len() == 0
}

// ---------------------------------------------------------------------------
// External Agent Support
// ---------------------------------------------------------------------------

// A2AClient manages communication with external A2A agents.
type A2AClient struct {
	mu             sync.RWMutex
	agents         map[string]*ExternalAgent
	httpClient     *http.Client
	metrics        *A2AMetrics
	taskStore      *A2ATaskStore
	eventStore     *A2AEventStore
	pollingTasks   map[string]pollingTask
	shutdownPoller chan struct{}
}

// ExternalAgent represents a discovered external A2A agent.
type ExternalAgent struct {
	Card         AgentCard
	URL          string
	DiscoveredAt time.Time
}

type pollingTask struct {
	taskID   string
	agentURL string
	interval time.Duration
	stopChan chan struct{}
}

// SetTaskStore sets the task store for polling.
func (c *A2AClient) SetTaskStore(store *A2ATaskStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskStore = store
}

// StartPolling starts polling an external agent task for status updates.
func (c *A2AClient) StartPolling(taskID, agentURL string, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	c.mu.Lock()
	if c.shutdownPoller == nil {
		c.shutdownPoller = make(chan struct{})
		c.pollingTasks = make(map[string]pollingTask)
	}
	if _, exists := c.pollingTasks[taskID]; exists {
		c.mu.Unlock()
		return
	}

	pt := pollingTask{
		taskID:   taskID,
		agentURL: agentURL,
		interval: interval,
		stopChan: make(chan struct{}),
	}
	c.pollingTasks[taskID] = pt
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.pollTaskStatus(taskID, agentURL)
			case <-pt.stopChan:
				return
			case <-c.shutdownPoller:
				return
			}
		}
	}()
}

// StopPolling stops polling a specific task.
func (c *A2AClient) StopPolling(taskID string) {
	c.mu.Lock()
	pt, exists := c.pollingTasks[taskID]
	if exists {
		delete(c.pollingTasks, taskID)
		close(pt.stopChan)
	}
	c.mu.Unlock()
}

// ShutdownPoller stops all polling.
func (c *A2AClient) ShutdownPoller() {
	c.mu.Lock()
	if c.shutdownPoller != nil {
		close(c.shutdownPoller)
		c.shutdownPoller = nil
		for _, pt := range c.pollingTasks {
			close(pt.stopChan)
		}
		c.pollingTasks = make(map[string]pollingTask)
	}
	c.mu.Unlock()
}

func (c *A2AClient) pollTaskStatus(taskID, agentURL string) {
	task, err := c.GetTaskStatus(agentURL, taskID)
	if err != nil {
		return
	}

	c.mu.RLock()
	store := c.taskStore
	c.mu.RUnlock()

	if store == nil {
		return
	}

	internalTask, ok := store.GetTask(taskID)
	if !ok {
		c.StopPolling(taskID)
		return
	}

	oldStatus := internalTask.Status.State

	if task.Status.State != oldStatus {
		store.UpdateTaskStatus(taskID, task.Status.State)
	}

	if task.Status.State == A2aTaskStatusCompleted ||
		task.Status.State == A2aTaskStatusFailed ||
		task.Status.State == A2aTaskStatusCancelled {
		c.StopPolling(taskID)

		if len(task.Messages) > 0 {
			if task.Status.State == A2aTaskStatusCompleted {
				store.CompleteTask(taskID, task.Messages[len(task.Messages)-1], task.Artifacts)
			} else if task.Status.State == A2aTaskStatusFailed {
				store.FailTask(taskID, task.Messages[len(task.Messages)-1])
			}
		}
	}
}

// NewA2AClient creates a new A2A client.
func NewA2AClient() *A2AClient {
	return NewA2AClientWithMetrics(nil)
}

// NewA2AClientWithMetrics creates a new A2A client with metrics.
func NewA2AClientWithMetrics(metrics *A2AMetrics) *A2AClient {
	if metrics == nil {
		metrics = NewA2AMetrics()
	}
	return &A2AClient{
		agents:       make(map[string]*ExternalAgent),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		metrics:      metrics,
		pollingTasks: make(map[string]pollingTask),
	}
}

// SetMetrics sets the metrics collector for the client.
func (c *A2AClient) SetMetrics(metrics *A2AMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = metrics
}

// DiscoverAgent discovers an A2A agent at the given URL.
func (c *A2AClient) DiscoverAgent(url string) (*AgentCard, error) {
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

	c.mu.Lock()
	c.agents[card.Name] = &ExternalAgent{
		Card:         card,
		URL:          url,
		DiscoveredAt: time.Now(),
	}
	c.mu.Unlock()

	if c.metrics != nil {
		c.metrics.RecordAgentDiscovered()
	}

	return &card, nil
}

// SetEventStore sets the event store for the client.
func (c *A2AClient) SetEventStore(eventStore *A2AEventStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// We don't store the event store here, but we'll add events via external calls
}

// RecordAgentDiscoveredEvent records an agent discovered event.
func (c *A2AClient) RecordAgentDiscoveredEvent(eventStore *A2AEventStore, card *AgentCard, url string) {
	if eventStore != nil {
		event := &A2AEvent{
			ID:         uuid.New().String(),
			Timestamp:  time.Now(),
			Kind:       A2AEventKindAgentDiscovered,
			SourceID:   "system",
			SourceName: "System",
			TargetID:   card.Name,
			TargetName: card.Name,
			Detail:     fmt.Sprintf("Discovered agent at %s", url),
			Payload:    map[string]interface{}{"url": url, "card": card},
		}
		eventStore.AddEvent(event)
	}
}

// SendTask sends a task to an external A2A agent.
func (c *A2AClient) SendTask(agentURL string, message string, sessionID *string) (*A2aTask, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tasks/send",
		"params": map[string]interface{}{
			"message": map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"type": "text",
						"text": message,
					},
				},
			},
			"sessionId": sessionID,
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, &A2AError{Message: "failed to marshal request: " + err.Error()}
	}

	targetURL := agentURL + "/a2a/tasks/send"
	resp, err := c.httpClient.Post(targetURL, "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, &A2AError{Message: "A2A send_task failed: " + err.Error()}
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, &A2AError{Message: "Invalid A2A response: " + err.Error()}
	}

	if result, ok := body["result"]; ok {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, &A2AError{Message: "failed to marshal result: " + err.Error()}
		}
		var task A2aTask
		if err := json.Unmarshal(resultJSON, &task); err != nil {
			return nil, &A2AError{Message: "Invalid A2A task response: " + err.Error()}
		}

		for i := range task.Messages {
			if task.Messages[i].Role == "agent" {
				task.Messages[i].Role = "assistant"
			}
		}

		if c.metrics != nil {
			c.metrics.RecordTaskSentExternally()
		}

		c.mu.RLock()
		defer c.mu.RUnlock()
		for _, agent := range c.agents {
			if agent.URL == agentURL {
				task.AgentName = agent.Card.Name
				break
			}
		}

		return &task, nil
	} else if errorVal, ok := body["error"]; ok {
		errorJSON, err := json.Marshal(errorVal)
		if err != nil {
			return nil, &A2AError{Message: "A2A error: " + err.Error()}
		}
		return nil, &A2AError{Message: "A2A error: " + string(errorJSON)}
	}

	return nil, &A2AError{Message: "Empty A2A response"}
}

// GetTaskStatus gets the status of a task from an external A2A agent.
func (c *A2AClient) GetTaskStatus(agentURL, taskID string) (*A2aTask, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tasks/get",
		"params": map[string]interface{}{
			"id": taskID,
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, &A2AError{Message: "failed to marshal request: " + err.Error()}
	}

	targetURL := agentURL + "/a2a/tasks/" + taskID
	resp, err := c.httpClient.Post(targetURL, "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, &A2AError{Message: "A2A get_task failed: " + err.Error()}
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, &A2AError{Message: "Invalid A2A response: " + err.Error()}
	}

	if result, ok := body["result"]; ok {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, &A2AError{Message: "failed to marshal result: " + err.Error()}
		}
		var task A2aTask
		if err := json.Unmarshal(resultJSON, &task); err != nil {
			return nil, &A2AError{Message: "Invalid A2A task: " + err.Error()}
		}

		for i := range task.Messages {
			if task.Messages[i].Role == "agent" {
				task.Messages[i].Role = "assistant"
			}
		}

		c.mu.RLock()
		defer c.mu.RUnlock()
		for _, agent := range c.agents {
			if agent.URL == agentURL {
				task.AgentName = agent.Card.Name
				break
			}
		}

		return &task, nil
	}

	return nil, &A2AError{Message: "Empty A2A response"}
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
		Name:        "FangClaw-Go",
		Description: "FangClaw-Go Agents - Autonomous Agent Application",
		URL:         url,
		Version:     "0.1.0",
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

// ---------------------------------------------------------------------------
// A2A Events
// ---------------------------------------------------------------------------

// A2AEventKind represents the kind of A2A event.
type A2AEventKind string

const (
	A2AEventKindAgentDiscovered  A2AEventKind = "agent_discovered"
	A2AEventKindTaskCreated      A2AEventKind = "task_created"
	A2AEventKindTaskStatusUpdate A2AEventKind = "task_status_update"
	A2AEventKindTaskCompleted    A2AEventKind = "task_completed"
	A2AEventKindTaskFailed       A2AEventKind = "task_failed"
	A2AEventKindTaskCancelled    A2AEventKind = "task_cancelled"
)

// A2AEvent represents an A2A event.
type A2AEvent struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Kind       A2AEventKind           `json:"kind"`
	SourceID   string                 `json:"sourceId"`
	SourceName string                 `json:"sourceName"`
	TargetID   string                 `json:"targetId,omitempty"`
	TargetName string                 `json:"targetName,omitempty"`
	Detail     string                 `json:"detail,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// A2AEventStore stores A2A events.
type A2AEventStore struct {
	mu        sync.RWMutex
	events    []*A2AEvent
	maxEvents int
}

// NewA2AEventStore creates a new A2A event store.
func NewA2AEventStore(maxEvents int) *A2AEventStore {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	return &A2AEventStore{
		events:    make([]*A2AEvent, 0),
		maxEvents: maxEvents,
	}
}

// AddEvent adds an event to the store.
func (s *A2AEventStore) AddEvent(event *A2AEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)

	// Trim if over max
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
}

// ListEvents returns all events, optionally limited.
func (s *A2AEventStore) ListEvents(limit int) []*A2AEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit >= len(s.events) {
		return s.events
	}
	return s.events[len(s.events)-limit:]
}

// ---------------------------------------------------------------------------
// A2A Topology
// ---------------------------------------------------------------------------

// TopoNode represents a node in the topology.
type TopoNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`          // "local" or "external"
	State string `json:"state"`         // for local agents
	URL   string `json:"url,omitempty"` // for external agents
}

// TopoEdge represents an edge in the topology.
type TopoEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // "parent_child" or "peer"
}

// Topology represents the agent topology.
type Topology struct {
	Nodes []*TopoNode `json:"nodes"`
	Edges []*TopoEdge `json:"edges"`
}

// ---------------------------------------------------------------------------
// Backward Compatibility
// ---------------------------------------------------------------------------

// Task is an alias for A2aTask for backward compatibility.
type Task = A2aTask

// TaskStatus is an alias for A2aTaskStatus for backward compatibility.
type TaskStatus = A2aTaskStatus

// Message is an alias for A2aMessage for backward compatibility.
type Message = A2aMessage

// Backward compatibility constants
const (
	TaskStatusSubmitted TaskStatus = A2aTaskStatusSubmitted
	TaskStatusWorking   TaskStatus = A2aTaskStatusWorking
	TaskStatusCompleted TaskStatus = A2aTaskStatusCompleted
	TaskStatusFailed    TaskStatus = A2aTaskStatusFailed
	TaskStatusCanceled  TaskStatus = A2aTaskStatusCancelled
)
