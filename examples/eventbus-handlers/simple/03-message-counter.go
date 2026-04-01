package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/eventbus"
)

type MessageStats struct {
	mu              sync.RWMutex
	receivedCount   int
	sentCount       int
	totalTokensIn   uint64
	totalTokensOut  uint64
	lastMessageTime time.Time
}

func (ms *MessageStats) IncrementReceived() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.receivedCount++
	ms.lastMessageTime = time.Now()
}

func (ms *MessageStats) IncrementSent(tokensIn, tokensOut uint64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.sentCount++
	ms.totalTokensIn += tokensIn
	ms.totalTokensOut += tokensOut
	ms.lastMessageTime = time.Now()
}

func (ms *MessageStats) GetStats() (int, int, uint64, uint64, time.Time) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.receivedCount, ms.sentCount, ms.totalTokensIn, ms.totalTokensOut, ms.lastMessageTime
}

func runMessageCounterDemo() {
	eb := eventbus.NewEventBus()
	stats := &MessageStats{}

	messageReceivedHandler := func(event *eventbus.Event) {
		channel, _ := event.Payload["channel"].(string)
		content, _ := event.Payload["content"].(string)
		stats.IncrementReceived()

		fmt.Printf("📥 Received [%s]: %s\n", channel, truncate(content, 50))
	}

	messageSentHandler := func(event *eventbus.Event) {
		channel, _ := event.Payload["channel"].(string)
		content, _ := event.Payload["content"].(string)
		tokensIn, _ := event.Payload["tokens_in"].(uint64)
		tokensOut, _ := event.Payload["tokens_out"].(uint64)
		stats.IncrementSent(tokensIn, tokensOut)

		fmt.Printf("📤 Sent [%s]: %s\n", channel, truncate(content, 50))
	}

	eb.Subscribe(eventbus.EventTypeMessageReceived, messageReceivedHandler)
	eb.Subscribe(eventbus.EventTypeMessageSent, messageSentHandler)

	fmt.Println("Message Counter started")
	fmt.Println("Tracking message statistics...")
	fmt.Println()

	simulateMessages(eb)

	fmt.Println()
	fmt.Println("=== Final Statistics ===")
	received, sent, tokensIn, tokensOut, lastTime := stats.GetStats()
	fmt.Printf("Messages Received:  %d\n", received)
	fmt.Printf("Messages Sent:      %d\n", sent)
	fmt.Printf("Total Tokens In:    %d\n", tokensIn)
	fmt.Printf("Total Tokens Out:   %d\n", tokensOut)
	fmt.Printf("Last Message:       %s\n", lastTime.Format(time.RFC3339))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func simulateMessages(eb *eventbus.EventBus) {
	messages := []struct {
		channel   string
		content   string
		tokensIn  uint64
		tokensOut uint64
	}{
		{"slack", "Hello, how can I help you?", 10, 15},
		{"discord", "What's the weather like today?", 8, 20},
		{"slack", "Please analyze this data...", 15, 100},
		{"web", "Thank you for your assistance!", 12, 10},
	}

	for _, msg := range messages {
		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeMessageReceived,
			"simulator",
			eventbus.EventTargetBroadcast,
		).WithPayload(map[string]interface{}{
			"channel": msg.channel,
			"content": msg.content,
		}))

		time.Sleep(100 * time.Millisecond)

		eb.Publish(eventbus.NewEvent(
			eventbus.EventTypeMessageSent,
			"simulator",
			eventbus.EventTargetBroadcast,
		).WithPayload(map[string]interface{}{
			"channel":    msg.channel,
			"content":    "Response to: " + msg.content,
			"tokens_in":  msg.tokensIn,
			"tokens_out": msg.tokensOut,
		}))

		time.Sleep(100 * time.Millisecond)
	}
}
