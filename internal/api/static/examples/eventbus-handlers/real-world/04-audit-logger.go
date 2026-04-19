package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

type AuditLogEntry struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Target    string                 `json:"target"`
	AgentID   string                 `json:"agent_id,omitempty"`
	HandID    string                 `json:"hand_id,omitempty"`
	ChannelID string                 `json:"channel_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

type AuditLogger struct {
	mu        sync.RWMutex
	entries   []*AuditLogEntry
	jsonFile  *os.File
	csvFile   *os.File
	csvWriter *csv.Writer
	logDir    string
}

func NewAuditLogger(logDir string) (*AuditLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	jsonPath := fmt.Sprintf("%s/audit_%s.jsonl", logDir, time.Now().Format("2006-01-02"))
	jsonFile, err := os.OpenFile(jsonPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	csvPath := fmt.Sprintf("%s/audit_%s.csv", logDir, time.Now().Format("2006-01-02"))
	csvFile, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		jsonFile.Close()
		return nil, err
	}

	csvWriter := csv.NewWriter(csvFile)

	stat, _ := csvFile.Stat()
	if stat.Size() == 0 {
		header := []string{
			"id", "type", "source", "target", "agent_id",
			"hand_id", "channel_id", "timestamp",
		}
		csvWriter.Write(header)
		csvWriter.Flush()
	}

	return &AuditLogger{
		entries:   make([]*AuditLogEntry, 0),
		jsonFile:  jsonFile,
		csvFile:   csvFile,
		csvWriter: csvWriter,
		logDir:    logDir,
	}, nil
}

func (al *AuditLogger) LogEvent(event *eventbus.Event) {
	al.mu.Lock()
	defer al.mu.Unlock()

	entry := &AuditLogEntry{
		ID:        event.ID,
		Type:      string(event.Type),
		Source:    event.Source,
		Target:    string(event.Target),
		AgentID:   event.AgentID,
		HandID:    event.HandID,
		ChannelID: event.ChannelID,
		Payload:   event.Payload,
		Timestamp: event.Timestamp.Format(time.RFC3339),
	}

	al.entries = append(al.entries, entry)

	jsonLine, _ := json.Marshal(entry)
	al.jsonFile.Write(jsonLine)
	al.jsonFile.WriteString("\n")

	csvRow := []string{
		entry.ID,
		entry.Type,
		entry.Source,
		entry.Target,
		entry.AgentID,
		entry.HandID,
		entry.ChannelID,
		entry.Timestamp,
	}
	al.csvWriter.Write(csvRow)
	al.csvWriter.Flush()

	al.printEntry(entry)
}

func (al *AuditLogger) printEntry(entry *AuditLogEntry) {
	var icon string
	switch entry.Type {
	case "agent.created":
		icon = "🤖"
	case "agent.started":
		icon = "▶️"
	case "agent.stopped":
		icon = "⏹️"
	case "agent.deleted":
		icon = "🗑️"
	case "message.received":
		icon = "📥"
	case "message.sent":
		icon = "📤"
	case "hand.activated":
		icon = "✋"
	case "hand.completed":
		icon = "✅"
	case "hand.error":
		icon = "❌"
	case "workflow.started":
		icon = "🚀"
	case "workflow.completed":
		icon = "🏁"
	default:
		icon = "📌"
	}

	fmt.Printf("%s [%s] %s (Source: %s)\n", icon, entry.Type, entry.ID[:8], entry.Source)
}

func (al *AuditLogger) Close() {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.jsonFile.Sync()
	al.jsonFile.Close()
	al.csvWriter.Flush()
	al.csvFile.Sync()
	al.csvFile.Close()
}

func (al *AuditLogger) GetStats() map[string]int {
	al.mu.RLock()
	defer al.mu.RUnlock()

	stats := make(map[string]int)
	for _, entry := range al.entries {
		stats[entry.Type]++
		stats["total"]++
	}
	return stats
}

func (al *AuditLogger) PrintReport() {
	al.mu.RLock()
	defer al.mu.RUnlock()

	fmt.Println()
	fmt.Println("=== Audit Log Summary ===")
	fmt.Printf("Total Entries:  %d\n", len(al.entries))
	fmt.Printf("Log Directory:  %s\n", al.logDir)
	fmt.Println()

	stats := al.GetStats()
	fmt.Println("Event Type Counts:")
	for eventType, count := range stats {
		if eventType != "total" {
			fmt.Printf("  - %-25s %d\n", eventType, count)
		}
	}
}

func runAuditLoggerDemo() {
	logDir := "./audit-logs"
	auditLogger, err := NewAuditLogger(logDir)
	if err != nil {
		fmt.Printf("Failed to create audit logger: %v\n", err)
		return
	}
	defer auditLogger.Close()

	eb := eventbus.NewEventBus()

	auditHandler := func(event *eventbus.Event) {
		auditLogger.LogEvent(event)
	}

	eb.SubscribeAll(auditHandler)

	fmt.Println("Audit Logger started")
	fmt.Println("All events will be logged to:")
	fmt.Printf("  - %s/audit_*.jsonl (JSON Lines)\n", logDir)
	fmt.Printf("  - %s/audit_*.csv (CSV)\n", logDir)
	fmt.Println()
	fmt.Println("Logging events...")
	fmt.Println()

	simulateEvents(eb)

	auditLogger.PrintReport()
	fmt.Println()
	fmt.Println("Audit logging complete!")
}

func runAuditSimulateEvents(eb *eventbus.EventBus) {
	agents := []struct {
		id   string
		name string
	}{
		{"agent-001", "Research Assistant"},
		{"agent-002", "Content Creator"},
	}

	for _, agent := range agents {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentCreated,
			"system",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{
			"name": agent.name,
		}))
		time.Sleep(100 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentStarted,
			"system",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{
			"name": agent.name,
		}))
		time.Sleep(100 * time.Millisecond)
	}

	channels := []string{"slack", "discord", "web"}
	for i, channel := range channels {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeMessageReceived,
			"user",
			eventbus.EventTargetBroadcast,
		).WithPayload(map[string]interface{}{
			"channel": channel,
			"content": fmt.Sprintf("Test message %d", i+1),
		}))
		time.Sleep(100 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeMessageSent,
			"agent",
			eventbus.EventTargetBroadcast,
		).WithPayload(map[string]interface{}{
			"channel": channel,
			"content": fmt.Sprintf("Response %d", i+1),
		}))
		time.Sleep(100 * time.Millisecond)
	}

	hands := []string{"hand-search", "hand-file", "hand-browser"}
	for _, hand := range hands {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeHandActivated,
			"agent",
			eventbus.EventTargetSystem,
		).WithHandID(hand))
		time.Sleep(100 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeHandCompleted,
			"agent",
			eventbus.EventTargetSystem,
		).WithHandID(hand))
		time.Sleep(100 * time.Millisecond)
	}

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeHandError,
		"agent",
		eventbus.EventTargetSystem,
	).WithHandID("hand-unstable"))
	time.Sleep(100 * time.Millisecond)

	workflows := []struct {
		id   string
		name string
	}{
		{"wf-001", "Data Analysis"},
		{"wf-002", "Report Generation"},
	}

	for _, wf := range workflows {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeWorkflowStarted,
			"system",
			eventbus.EventTargetSystem,
		).WithPayload(map[string]interface{}{
			"workflow_id":   wf.id,
			"workflow_name": wf.name,
		}))
		time.Sleep(150 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeWorkflowCompleted,
			"system",
			eventbus.EventTargetSystem,
		).WithPayload(map[string]interface{}{
			"workflow_id": wf.id,
			"output":      "Success",
		}))
		time.Sleep(100 * time.Millisecond)
	}

	for _, agent := range agents {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentStopped,
			"system",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{
			"name":   agent.name,
			"reason": "shutdown",
		}))
		time.Sleep(100 * time.Millisecond)
	}
}
