package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

type WebhookConfig struct {
	URL        string
	Secret     string
	EventTypes []eventbus.EventType
	Active     bool
}

type WebhookIntegrator struct {
	mu       sync.RWMutex
	webhooks map[string]*WebhookConfig
	client   *http.Client
}

func NewWebhookIntegrator() *WebhookIntegrator {
	return &WebhookIntegrator{
		webhooks: make(map[string]*WebhookConfig),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (wi *WebhookIntegrator) RegisterWebhook(id string, config *WebhookConfig) {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	wi.webhooks[id] = config
	fmt.Printf("🔗 Webhook registered: %s -> %s\n", id, config.URL)
}

func (wi *WebhookIntegrator) SendToWebhook(config *WebhookConfig, event *eventbus.Event) error {
	payload := map[string]interface{}{
		"id":         event.ID,
		"type":       event.Type,
		"source":     event.Source,
		"target":     event.Target,
		"agent_id":   event.AgentID,
		"hand_id":    event.HandID,
		"channel_id": event.ChannelID,
		"payload":    event.Payload,
		"timestamp":  event.Timestamp.Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", config.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if config.Secret != "" {
		req.Header.Set("X-Webhook-Secret", config.Secret)
	}

	resp, err := wi.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (wi *WebhookIntegrator) CreateHandler(config *WebhookConfig) eventbus.EventHandler {
	return func(event *eventbus.Event) {
		if !config.Active {
			return
		}

		shouldSend := len(config.EventTypes) == 0
		if !shouldSend {
			for _, et := range config.EventTypes {
				if event.Type == et {
					shouldSend = true
					break
				}
			}
		}

		if shouldSend {
			go func() {
				if err := wi.SendToWebhook(config, event); err != nil {
					fmt.Printf("❌ Webhook failed: %v\n", err)
				} else {
					fmt.Printf("✅ Webhook sent: %s\n", event.Type)
				}
			}()
		}
	}
}

func runWebhookDemo() {
	eb := eventbus.NewEventBus()
	integrator := NewWebhookIntegrator()

	fmt.Println("Starting mock webhook server...")
	go startMockWebhookServer()
	time.Sleep(500 * time.Millisecond)

	slackWebhook := &WebhookConfig{
		URL:    "http://localhost:8080/webhook/slack",
		Secret: "slack-secret-123",
		EventTypes: []eventbus.EventType{
			eventbus.EventTypeAgentCreated,
			eventbus.EventTypeAgentStarted,
			eventbus.EventTypeAgentStopped,
		},
		Active: true,
	}

	analyticsWebhook := &WebhookConfig{
		URL:    "http://localhost:8080/webhook/analytics",
		Secret: "analytics-secret-456",
		EventTypes: []eventbus.EventType{
			eventbus.EventTypeMessageReceived,
			eventbus.EventTypeMessageSent,
			eventbus.EventTypeWorkflowStarted,
			eventbus.EventTypeWorkflowCompleted,
		},
		Active: true,
	}

	alertWebhook := &WebhookConfig{
		URL:    "http://localhost:8080/webhook/alerts",
		Secret: "alert-secret-789",
		EventTypes: []eventbus.EventType{
			eventbus.EventTypeHandError,
		},
		Active: true,
	}

	integrator.RegisterWebhook("slack-notifications", slackWebhook)
	integrator.RegisterWebhook("analytics-tracking", analyticsWebhook)
	integrator.RegisterWebhook("alerting-system", alertWebhook)

	eb.SubscribeAll(integrator.CreateHandler(slackWebhook))
	eb.SubscribeAll(integrator.CreateHandler(analyticsWebhook))
	eb.SubscribeAll(integrator.CreateHandler(alertWebhook))

	fmt.Println()
	fmt.Println("Webhook Integrator started")
	fmt.Println("Registered webhooks:")
	fmt.Println("  - slack-notifications (agent lifecycle events)")
	fmt.Println("  - analytics-tracking (message and workflow events)")
	fmt.Println("  - alerting-system (error events)")
	fmt.Println()
	fmt.Println("Sending test events...")
	fmt.Println()

	simulateEvents(eb)

	fmt.Println()
	fmt.Println("Demonstration complete!")
}

func startMockWebhookServer() {
	http.HandleFunc("/webhook/slack", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		eventType, _ := payload["type"].(string)
		fmt.Printf("📨 [Slack] Received: %s\n", eventType)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/webhook/analytics", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		eventType, _ := payload["type"].(string)
		fmt.Printf("📊 [Analytics] Received: %s\n", eventType)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/webhook/alerts", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		eventType, _ := payload["type"].(string)
		fmt.Printf("🚨 [Alerts] Received: %s\n", eventType)
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}

func runSimulateEvents(eb *eventbus.EventBus) {
	testAgentID := "agent-001"
	testAgentName := "Test Agent"

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentCreated,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name": testAgentName,
	}))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentStarted,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name": testAgentName,
	}))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeMessageReceived,
		"test",
		eventbus.EventTargetBroadcast,
	).WithPayload(map[string]interface{}{
		"channel": "slack",
		"content": "Hello, world!",
	}))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeWorkflowStarted,
		"test",
		eventbus.EventTargetSystem,
	).WithPayload(map[string]interface{}{
		"workflow_id":   "wf-001",
		"workflow_name": "Test Workflow",
	}))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeHandError,
		"test",
		eventbus.EventTargetSystem,
	).WithHandID("hand-test"))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeWorkflowCompleted,
		"test",
		eventbus.EventTargetSystem,
	).WithPayload(map[string]interface{}{
		"workflow_id": "wf-001",
		"output":      "Done!",
	}))
	time.Sleep(200 * time.Millisecond)

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentStopped,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name":   testAgentName,
		"reason": "shutdown",
	}))
}
