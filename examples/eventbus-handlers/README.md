# EventBus Handler Examples

This directory contains practical examples of EventBus handlers for fangclaw-go. These examples demonstrate how to use the EventBus system for various real-world scenarios.

## Overview

The EventBus system in fangclaw-go provides a publish-subscribe mechanism for event-driven communication. Handlers can subscribe to specific event types or all events and react accordingly.

## Directory Structure

```
eventbus-handlers/
├── simple/                    # Simple, basic examples
│   ├── 01-event-logger.go       # Basic event logging
│   ├── 02-agent-lifecycle-monitor.go  # Agent lifecycle tracking
│   ├── 03-message-counter.go    # Message statistics
│   └── 04-workflow-tracker.go   # Workflow execution tracking
├── real-world/               # Production-ready examples
│   ├── 01-alerting-system.go    # Alerting and monitoring
│   ├── 02-metrics-collector.go  # Metrics collection
│   ├── 03-webhook-integrator.go # Webhook integrations
│   └── 04-audit-logger.go       # Audit logging
└── README.md                  # This file
```

## Event Types

The EventBus supports the following event types:

- `agent.created` - Agent created
- `agent.started` - Agent started
- `agent.stopped` - Agent stopped
- `agent.deleted` - Agent deleted
- `message.received` - Message received
- `message.sent` - Message sent
- `hand.activated` - Hand activated
- `hand.completed` - Hand completed
- `hand.error` - Hand error
- `workflow.started` - Workflow started
- `workflow.completed` - Workflow completed
- `system` - System events

## Simple Examples

### 01-event-logger.go
A basic event logger that writes all events to a file. Useful for debugging and understanding event flow.

```bash
cd simple
go run 01-event-logger.go
```

### 02-agent-lifecycle-monitor.go
Monitors agent lifecycle events (created, started, stopped, deleted) and displays them with emojis.

```bash
go run 02-agent-lifecycle-monitor.go
```

### 03-message-counter.go
Tracks message statistics including count, tokens, and channels.

```bash
go run 03-message-counter.go
```

### 04-workflow-tracker.go
Monitors workflow execution and generates execution reports.

```bash
go run 04-workflow-tracker.go
```

## Real-World Examples

### 01-alerting-system.go
A complete alerting system with different severity levels (info, warning, error, critical). Features:
- Multiple alert handlers (console, log file)
- Alert level filtering
- Alert summary reporting

```bash
cd real-world
go run 01-alerting-system.go
```

### 02-metrics-collector.go
Comprehensive metrics collector that tracks:
- Agent metrics (creation, startup, uptime)
- Message metrics (count, tokens, channels)
- Workflow metrics (execution count, duration)
- Hand metrics (activation, completion, errors)
- JSON export for metrics

```bash
go run 02-metrics-collector.go
```

### 03-webhook-integrator.go
Webhook integration system that sends events to external services. Features:
- Multiple webhook configurations
- Event type filtering per webhook
- Secret-based authentication
- Mock webhook server for testing

```bash
go run 03-webhook-integrator.go
```

### 04-audit-logger.go
Production-grade audit logger with:
- JSON Lines format for easy parsing
- CSV format for spreadsheet analysis
- Daily log rotation
- Summary statistics

```bash
go run 04-audit-logger.go
```

## Usage Patterns

### Subscribe to All Events
```go
eb.SubscribeAll(func(event *eventbus.Event) {
    fmt.Printf("Event: %s\n", event.Type)
})
```

### Subscribe to Specific Event Type
```go
eb.Subscribe(eventbus.EventTypeAgentCreated, func(event *eventbus.Event) {
    fmt.Printf("Agent created: %s\n", event.AgentID)
})
```

### Create Event with Payload
```go
event := eventbus.NewEvent(
    eventbus.EventTypeMessageSent,
    "my-source",
    eventbus.EventTargetBroadcast,
).WithPayload(map[string]interface{}{
    "channel": "slack",
    "content": "Hello!",
})
eb.Publish(event)
```

## Best Practices

1. **Use Goroutines for Long-Running Handlers**: If your handler does I/O or heavy processing, use goroutines.
2. **Handle Errors Gracefully**: Always handle errors in handlers to prevent panics.
3. **Filter Events Early**: Subscribe to specific event types instead of all events when possible.
4. **Use Timeouts**: For network operations (like webhooks), use timeouts.
5. **Log Context**: Include sufficient context in logs for debugging.

## Integration with fangclaw-go

To use these handlers in your fangclaw-go application:

1. Import the eventbus package
2. Create handlers for the events you're interested in
3. Subscribe to the events
4. The EventBus will call your handlers when events are published

See the individual example files for complete implementations.
