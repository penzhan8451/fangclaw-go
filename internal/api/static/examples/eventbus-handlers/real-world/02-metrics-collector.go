package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

type Metrics struct {
	mu sync.RWMutex

	AgentMetrics    map[string]*AgentMetrics
	MessageMetrics  *MessageMetrics
	WorkflowMetrics *WorkflowMetrics
	HandMetrics     map[string]*HandMetrics
	StartTime       time.Time
}

type AgentMetrics struct {
	CreatedCount int
	StartedCount int
	StoppedCount int
	DeletedCount int
	TotalUptime  time.Duration
	LastActive   time.Time
}

type MessageMetrics struct {
	ReceivedCount  int
	SentCount      int
	TotalTokensIn  uint64
	TotalTokensOut uint64
	Channels       map[string]int
}

type WorkflowMetrics struct {
	StartedCount   int
	CompletedCount int
	FailedCount    int
	TotalDuration  time.Duration
}

type HandMetrics struct {
	ActivatedCount int
	CompletedCount int
	ErrorCount     int
}

func NewMetrics() *Metrics {
	return &Metrics{
		AgentMetrics:    make(map[string]*AgentMetrics),
		MessageMetrics:  &MessageMetrics{Channels: make(map[string]int)},
		WorkflowMetrics: &WorkflowMetrics{},
		HandMetrics:     make(map[string]*HandMetrics),
		StartTime:       time.Now(),
	}
}

func (m *Metrics) TrackAgentCreated(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.AgentMetrics[agentID]; !exists {
		m.AgentMetrics[agentID] = &AgentMetrics{}
	}
	m.AgentMetrics[agentID].CreatedCount++
	m.AgentMetrics[agentID].LastActive = time.Now()
}

func (m *Metrics) TrackAgentStarted(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.AgentMetrics[agentID]; !exists {
		m.AgentMetrics[agentID] = &AgentMetrics{}
	}
	m.AgentMetrics[agentID].StartedCount++
	m.AgentMetrics[agentID].LastActive = time.Now()
}

func (m *Metrics) TrackAgentStopped(agentID string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.AgentMetrics[agentID]; !exists {
		m.AgentMetrics[agentID] = &AgentMetrics{}
	}
	m.AgentMetrics[agentID].StoppedCount++
	m.AgentMetrics[agentID].TotalUptime += duration
}

func (m *Metrics) TrackMessageReceived(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessageMetrics.ReceivedCount++
	m.MessageMetrics.Channels[channel]++
}

func (m *Metrics) TrackMessageSent(channel string, tokensIn, tokensOut uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessageMetrics.SentCount++
	m.MessageMetrics.TotalTokensIn += tokensIn
	m.MessageMetrics.TotalTokensOut += tokensOut
	m.MessageMetrics.Channels[channel]++
}

func (m *Metrics) TrackWorkflowStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.WorkflowMetrics.StartedCount++
}

func (m *Metrics) TrackWorkflowCompleted(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.WorkflowMetrics.CompletedCount++
	m.WorkflowMetrics.TotalDuration += duration
}

func (m *Metrics) TrackHandActivated(handID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.HandMetrics[handID]; !exists {
		m.HandMetrics[handID] = &HandMetrics{}
	}
	m.HandMetrics[handID].ActivatedCount++
}

func (m *Metrics) TrackHandCompleted(handID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.HandMetrics[handID]; !exists {
		m.HandMetrics[handID] = &HandMetrics{}
	}
	m.HandMetrics[handID].CompletedCount++
}

func (m *Metrics) TrackHandError(handID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.HandMetrics[handID]; !exists {
		m.HandMetrics[handID] = &HandMetrics{}
	}
	m.HandMetrics[handID].ErrorCount++
}

func (m *Metrics) ExportToJSON(filename string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := map[string]interface{}{
		"uptime_seconds": time.Since(m.StartTime).Seconds(),
		"agents":         m.AgentMetrics,
		"messages":       m.MessageMetrics,
		"workflows":      m.WorkflowMetrics,
		"hands":          m.HandMetrics,
		"exported_at":    time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, jsonData, 0644)
}

func (m *Metrics) PrintReport() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fmt.Println()
	fmt.Println("=== System Metrics Report ===")
	fmt.Printf("Uptime: %v\n\n", time.Since(m.StartTime).Round(time.Second))

	fmt.Println("--- Agent Metrics ---")
	totalAgents := len(m.AgentMetrics)
	totalCreated := 0
	totalStarted := 0
	for _, am := range m.AgentMetrics {
		totalCreated += am.CreatedCount
		totalStarted += am.StartedCount
	}
	fmt.Printf("Total Agents:    %d\n", totalAgents)
	fmt.Printf("Created:         %d\n", totalCreated)
	fmt.Printf("Started:         %d\n", totalStarted)

	fmt.Println("\n--- Message Metrics ---")
	fmt.Printf("Received:        %d\n", m.MessageMetrics.ReceivedCount)
	fmt.Printf("Sent:            %d\n", m.MessageMetrics.SentCount)
	fmt.Printf("Tokens In:       %d\n", m.MessageMetrics.TotalTokensIn)
	fmt.Printf("Tokens Out:      %d\n", m.MessageMetrics.TotalTokensOut)
	fmt.Printf("Channels:        %d\n", len(m.MessageMetrics.Channels))

	fmt.Println("\n--- Workflow Metrics ---")
	fmt.Printf("Started:         %d\n", m.WorkflowMetrics.StartedCount)
	fmt.Printf("Completed:       %d\n", m.WorkflowMetrics.CompletedCount)
	fmt.Printf("Total Duration:  %v\n", m.WorkflowMetrics.TotalDuration.Round(time.Millisecond))

	fmt.Println("\n--- Hand Metrics ---")
	totalHands := len(m.HandMetrics)
	totalActivated := 0
	totalErrors := 0
	for _, hm := range m.HandMetrics {
		totalActivated += hm.ActivatedCount
		totalErrors += hm.ErrorCount
	}
	fmt.Printf("Total Hands:     %d\n", totalHands)
	fmt.Printf("Activated:       %d\n", totalActivated)
	fmt.Printf("Errors:          %d\n", totalErrors)
}

func runMetricsDemo() {
	eb := eventbus.NewEventBus()
	metrics := NewMetrics()
	agentStartTimes := make(map[string]time.Time)

	agentCreatedHandler := func(event *eventbus.Event) {
		metrics.TrackAgentCreated(event.AgentID)
		name, _ := event.Payload["name"].(string)
		fmt.Printf("📊 Agent created: %s\n", name)
	}

	agentStartedHandler := func(event *eventbus.Event) {
		metrics.TrackAgentStarted(event.AgentID)
		agentStartTimes[event.AgentID] = time.Now()
		name, _ := event.Payload["name"].(string)
		fmt.Printf("📊 Agent started: %s\n", name)
	}

	agentStoppedHandler := func(event *eventbus.Event) {
		if startTime, exists := agentStartTimes[event.AgentID]; exists {
			duration := time.Since(startTime)
			metrics.TrackAgentStopped(event.AgentID, duration)
			delete(agentStartTimes, event.AgentID)
		}
		name, _ := event.Payload["name"].(string)
		fmt.Printf("📊 Agent stopped: %s\n", name)
	}

	messageReceivedHandler := func(event *eventbus.Event) {
		channel, _ := event.Payload["channel"].(string)
		metrics.TrackMessageReceived(channel)
	}

	messageSentHandler := func(event *eventbus.Event) {
		channel, _ := event.Payload["channel"].(string)
		tokensIn, _ := event.Payload["tokens_in"].(uint64)
		tokensOut, _ := event.Payload["tokens_out"].(uint64)
		metrics.TrackMessageSent(channel, tokensIn, tokensOut)
	}

	workflowStartedHandler := func(event *eventbus.Event) {
		metrics.TrackWorkflowStarted()
		name, _ := event.Payload["workflow_name"].(string)
		fmt.Printf("📊 Workflow started: %s\n", name)
	}

	workflowCompletedHandler := func(event *eventbus.Event) {
		metrics.TrackWorkflowCompleted(100 * time.Millisecond)
		name, _ := event.Payload["workflow_name"].(string)
		fmt.Printf("📊 Workflow completed: %s\n", name)
	}

	handActivatedHandler := func(event *eventbus.Event) {
		metrics.TrackHandActivated(event.HandID)
	}

	handCompletedHandler := func(event *eventbus.Event) {
		metrics.TrackHandCompleted(event.HandID)
	}

	handErrorHandler := func(event *eventbus.Event) {
		metrics.TrackHandError(event.HandID)
		fmt.Printf("📊 Hand error: %s\n", event.HandID)
	}

	eb.Subscribe(eventbus.EventTypeAgentCreated, agentCreatedHandler)
	eb.Subscribe(eventbus.EventTypeAgentStarted, agentStartedHandler)
	eb.Subscribe(eventbus.EventTypeAgentStopped, agentStoppedHandler)
	eb.Subscribe(eventbus.EventTypeMessageReceived, messageReceivedHandler)
	eb.Subscribe(eventbus.EventTypeMessageSent, messageSentHandler)
	eb.Subscribe(eventbus.EventTypeWorkflowStarted, workflowStartedHandler)
	eb.Subscribe(eventbus.EventTypeWorkflowCompleted, workflowCompletedHandler)
	eb.Subscribe(eventbus.EventTypeHandActivated, handActivatedHandler)
	eb.Subscribe(eventbus.EventTypeHandCompleted, handCompletedHandler)
	eb.Subscribe(eventbus.EventTypeHandError, handErrorHandler)

	fmt.Println("Metrics Collector started")
	fmt.Println("Collecting system metrics...")
	fmt.Println()

	simulateActivity(eb)

	metrics.PrintReport()
	fmt.Println()
	if err := metrics.ExportToJSON("metrics.json"); err != nil {
		fmt.Printf("Failed to export metrics: %v\n", err)
	} else {
		fmt.Println("Metrics exported to metrics.json")
	}
}

func simulateActivity(eb *eventbus.EventBus) {
	agents := []struct{ id, name string }{
		{"agent-001", "Data Analyst"},
		{"agent-002", "Content Writer"},
	}

	for _, agent := range agents {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentCreated,
			"sim",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{"name": agent.name}))
		time.Sleep(50 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentStarted,
			"sim",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{"name": agent.name}))
		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < 5; i++ {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeMessageReceived,
			"sim",
			eventbus.EventTargetBroadcast,
		).WithPayload(map[string]interface{}{
			"channel": "slack",
			"content": fmt.Sprintf("Message %d", i),
		}))
		time.Sleep(50 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeMessageSent,
			"sim",
			eventbus.EventTargetBroadcast,
		).WithPayload(map[string]interface{}{
			"channel":    "slack",
			"content":    fmt.Sprintf("Response %d", i),
			"tokens_in":  uint64(100 + i*10),
			"tokens_out": uint64(50 + i*5),
		}))
		time.Sleep(50 * time.Millisecond)
	}

	for i := 0; i < 3; i++ {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeWorkflowStarted,
			"sim",
			eventbus.EventTargetSystem,
		).WithPayload(map[string]interface{}{
			"workflow_id":   fmt.Sprintf("wf-%d", i),
			"workflow_name": fmt.Sprintf("Workflow %d", i),
		}))
		time.Sleep(150 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeWorkflowCompleted,
			"sim",
			eventbus.EventTargetSystem,
		).WithPayload(map[string]interface{}{
			"workflow_id":   fmt.Sprintf("wf-%d", i),
			"workflow_name": fmt.Sprintf("Workflow %d", i),
		}))
		time.Sleep(50 * time.Millisecond)
	}

	hands := []string{"hand-search", "hand-file", "hand-browser"}
	for _, hand := range hands {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeHandActivated,
			"sim",
			eventbus.EventTargetSystem,
		).WithHandID(hand))
		time.Sleep(50 * time.Millisecond)
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeHandCompleted,
			"sim",
			eventbus.EventTargetSystem,
		).WithHandID(hand))
		time.Sleep(50 * time.Millisecond)
	}

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeHandError,
		"sim",
		eventbus.EventTargetSystem,
	).WithHandID("hand-unstable"))

	for _, agent := range agents {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentStopped,
			"sim",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{
			"name":   agent.name,
			"reason": "shutdown",
		}))
		time.Sleep(50 * time.Millisecond)
	}
}
