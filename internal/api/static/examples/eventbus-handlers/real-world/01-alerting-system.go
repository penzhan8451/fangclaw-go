package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelError
	AlertLevelCritical
)

type Alert struct {
	ID        string
	Level     AlertLevel
	Message   string
	Source    string
	Timestamp time.Time
	Handled   bool
}

type AlertingSystem struct {
	mu         sync.RWMutex
	alerts     []*Alert
	alertCount map[AlertLevel]int
	handlers   map[AlertLevel][]func(*Alert)
}

func NewAlertingSystem() *AlertingSystem {
	return &AlertingSystem{
		alerts:     make([]*Alert, 0),
		alertCount: make(map[AlertLevel]int),
		handlers:   make(map[AlertLevel][]func(*Alert)),
	}
}

func (as *AlertingSystem) RegisterHandler(level AlertLevel, handler func(*Alert)) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.handlers[level] = append(as.handlers[level], handler)
}

func (as *AlertingSystem) AddAlert(level AlertLevel, message, source string) {
	as.mu.Lock()
	defer as.mu.Unlock()

	alert := &Alert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Level:     level,
		Message:   message,
		Source:    source,
		Timestamp: time.Now(),
		Handled:   false,
	}

	as.alerts = append(as.alerts, alert)
	as.alertCount[level]++

	if handlers, exists := as.handlers[level]; exists {
		for _, handler := range handlers {
			go handler(alert)
		}
	}
}

func (as *AlertingSystem) PrintSummary() {
	as.mu.RLock()
	defer as.mu.RUnlock()

	fmt.Println()
	fmt.Println("=== Alert Summary ===")
	fmt.Printf("Info:        %d\n", as.alertCount[AlertLevelInfo])
	fmt.Printf("Warning:     %d\n", as.alertCount[AlertLevelWarning])
	fmt.Printf("Error:       %d\n", as.alertCount[AlertLevelError])
	fmt.Printf("Critical:    %d\n", as.alertCount[AlertLevelCritical])
	fmt.Printf("Total:       %d\n", len(as.alerts))
}

func main() {
	eb := eventbus.NewEventBus()
	alertSystem := NewAlertingSystem()

	logFile, err := os.OpenFile("alerts.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	alertLogger := log.New(logFile, "[ALERT] ", log.LstdFlags)

	consoleAlertHandler := func(alert *Alert) {
		levelIcon := "ℹ️"
		switch alert.Level {
		case AlertLevelWarning:
			levelIcon = "⚠️"
		case AlertLevelError:
			levelIcon = "❌"
		case AlertLevelCritical:
			levelIcon = "🚨"
		}
		fmt.Printf("%s [%s] %s (Source: %s)\n", levelIcon, alert.Level.String(), alert.Message, alert.Source)
	}

	logAlertHandler := func(alert *Alert) {
		alertLogger.Printf("[%s] %s (Source: %s)", alert.Level.String(), alert.Message, alert.Source)
	}

	alertSystem.RegisterHandler(AlertLevelInfo, consoleAlertHandler)
	alertSystem.RegisterHandler(AlertLevelInfo, logAlertHandler)
	alertSystem.RegisterHandler(AlertLevelWarning, consoleAlertHandler)
	alertSystem.RegisterHandler(AlertLevelWarning, logAlertHandler)
	alertSystem.RegisterHandler(AlertLevelError, consoleAlertHandler)
	alertSystem.RegisterHandler(AlertLevelError, logAlertHandler)
	alertSystem.RegisterHandler(AlertLevelCritical, consoleAlertHandler)
	alertSystem.RegisterHandler(AlertLevelCritical, logAlertHandler)

	agentErrorHandler := func(event *eventbus.Event) {
		name, _ := event.Payload["name"].(string)
		reason, _ := event.Payload["reason"].(string)
		if reason == "error" {
			alertSystem.AddAlert(AlertLevelError, fmt.Sprintf("Agent '%s' stopped due to error", name), event.Source)
		}
	}

	handErrorHandler := func(event *eventbus.Event) {
		handID := event.HandID
		alertSystem.AddAlert(AlertLevelWarning, fmt.Sprintf("Hand '%s' encountered an error", handID), event.Source)
	}

	// workflowErrorHandler := func(event *eventbus.Event) {
	// 	workflowID, _ := event.Payload["workflow_id"].(string)
	// 	errorMsg, _ := event.Payload["error"].(string)
	// 	alertSystem.AddAlert(AlertLevelError, fmt.Sprintf("Workflow '%s' failed: %s", workflowID, errorMsg), event.Source)
	// }

	agentStartHandler := func(event *eventbus.Event) {
		name, _ := event.Payload["name"].(string)
		alertSystem.AddAlert(AlertLevelInfo, fmt.Sprintf("Agent '%s' started", name), event.Source)
	}

	eb.Subscribe(eventbus.EventTypeAgentStarted, agentStartHandler)
	eb.Subscribe(eventbus.EventTypeAgentStopped, agentErrorHandler)
	eb.Subscribe(eventbus.EventTypeHandError, handErrorHandler)

	fmt.Println("Alerting System started")
	fmt.Println("Monitoring system events...")
	fmt.Println("Alerts will be logged to alerts.log")
	fmt.Println()

	simulateEvents(eb)

	alertSystem.PrintSummary()
}

func (a AlertLevel) String() string {
	switch a {
	case AlertLevelInfo:
		return "INFO"
	case AlertLevelWarning:
		return "WARNING"
	case AlertLevelError:
		return "ERROR"
	case AlertLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func simulateEvents(eb *eventbus.EventBus) {
	testAgents := []struct {
		id   string
		name string
	}{
		{"agent-001", "Data Processor"},
		{"agent-002", "Content Writer"},
		{"agent-003", "Code Reviewer"},
	}

	for _, agent := range testAgents {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeAgentStarted,
			"simulator",
			eventbus.EventTargetAgent,
		).WithAgentID(agent.id).WithPayload(map[string]interface{}{
			"name": agent.name,
		}))
		time.Sleep(200 * time.Millisecond)
	}

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeHandError,
		"simulator",
		eventbus.EventTargetSystem,
	).WithHandID("hand-file-reader"))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentStopped,
		"simulator",
		eventbus.EventTargetAgent,
	).WithAgentID("agent-002").WithPayload(map[string]interface{}{
		"name":   "Content Writer",
		"reason": "error",
	}))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentStopped,
		"simulator",
		eventbus.EventTargetAgent,
	).WithAgentID("agent-001").WithPayload(map[string]interface{}{
		"name":   "Data Processor",
		"reason": "shutdown",
	}))
}
