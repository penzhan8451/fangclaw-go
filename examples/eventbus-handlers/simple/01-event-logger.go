package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

func runEventLoggerDemo() {
	eb := eventbus.NewEventBus()

	logFile, err := os.OpenFile("event-log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags)

	eventLogger := func(event *eventbus.Event) {
		logEntry := fmt.Sprintf(
			"[%s] ID: %s | Type: %s | Source: %s | Target: %s | AgentID: %s",
			event.Timestamp.Format(time.RFC3339),
			event.ID,
			event.Type,
			event.Source,
			event.Target,
			event.AgentID,
		)
		logger.Println(logEntry)
		fmt.Println("Logged event:", event.Type)
	}

	eb.SubscribeAll(eventLogger)

	fmt.Println("Event logger started. All events will be logged to event-log.txt")
	fmt.Println("Press Ctrl+C to exit")

	exampleEvent := eventbus.NewEvent(
		eventbus.EventTypeSystem,
		"example-source",
		eventbus.EventTargetBroadcast,
	).WithPayload(map[string]interface{}{
		"message": "Hello, EventBus!",
	})

	eb.Publish(exampleEvent)

	select {}
}
