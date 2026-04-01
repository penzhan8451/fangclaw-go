package main

import (
	"fmt"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

func runAgentLifecycleMonitorDemo() {
	eb := eventbus.NewEventBus()

	agentCreatedHandler := func(event *eventbus.Event) {
		name, _ := event.Payload["name"].(string)
		fmt.Printf("🤖 Agent Created: %s (ID: %s)\n", name, event.AgentID)
	}

	agentStartedHandler := func(event *eventbus.Event) {
		name, _ := event.Payload["name"].(string)
		fmt.Printf("▶️  Agent Started: %s (ID: %s)\n", name, event.AgentID)
	}

	agentStoppedHandler := func(event *eventbus.Event) {
		name, _ := event.Payload["name"].(string)
		reason, _ := event.Payload["reason"].(string)
		if reason != "" {
			fmt.Printf("⏹️  Agent Stopped: %s (ID: %s, Reason: %s)\n", name, event.AgentID, reason)
		} else {
			fmt.Printf("⏹️  Agent Stopped: %s (ID: %s)\n", name, event.AgentID)
		}
	}

	agentDeletedHandler := func(event *eventbus.Event) {
		name, _ := event.Payload["name"].(string)
		fmt.Printf("🗑️  Agent Deleted: %s (ID: %s)\n", name, event.AgentID)
	}

	eb.Subscribe(eventbus.EventTypeAgentCreated, agentCreatedHandler)
	eb.Subscribe(eventbus.EventTypeAgentStarted, agentStartedHandler)
	eb.Subscribe(eventbus.EventTypeAgentStopped, agentStoppedHandler)
	eb.Subscribe(eventbus.EventTypeAgentDeleted, agentDeletedHandler)

	fmt.Println("Agent Lifecycle Monitor started")
	fmt.Println("Listening for agent lifecycle events...")
	fmt.Println()

	testAgentID := "agent-001"
	testAgentName := "Test Agent"

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentCreated,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name": testAgentName,
	}))

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentStarted,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name": testAgentName,
	}))

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentStopped,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name":   testAgentName,
		"reason": "user request",
	}))

	eb.Publish(eventbus.NewEvent(
		eventbus.EventTypeAgentDeleted,
		"test",
		eventbus.EventTargetAgent,
	).WithAgentID(testAgentID).WithPayload(map[string]interface{}{
		"name": testAgentName,
	}))

	fmt.Println()
	fmt.Println("Demonstration complete!")
}
