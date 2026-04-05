# Scheduler Configuration Guide

## Overview

The Scheduler (AgentScheduler) in FangClaw manages resource quotas for agents, including token usage, tool calls, and cost limits, enforced with hourly rolling windows.

## Key Features

- **Hourly Rolling Windows**: All quotas reset hourly from the first usage
- **Agent-specific Quotas**: Configure global defaults and per-agent overrides
- **Non-persistent Tracking**: Quotas reset on service restart (no persistence)
- **Quota Exceeded Notifications**: Sends approval messages to dashboard when limits are reached

## Configuration

Configure quotas in your `config.toml` file:

```toml
[quotas]
  [quotas.default]
  max_tokens_per_hour = 100000
  max_tool_calls_per_hour = 100
  max_cost_per_hour_usd = 1.0

  [quotas.agents]
    [quotas.agents."agent-id-1"]
    max_tokens_per_hour = 50000
    max_tool_calls_per_hour = 50
    max_cost_per_hour_usd = 0.5

    [quotas.agents."agent-id-2"]
    max_tokens_per_hour = 200000
    max_tool_calls_per_hour = 200
    max_cost_per_hour_usd = 2.0
```

## Configuration Fields

### QuotasConfig
- `default`: Global default resource quotas (applies to all agents without specific configuration)
- `agents`: Map of agent IDs to their specific resource quotas

### ResourceQuota
Each quota entry supports these fields:

| Field | Type | Description |
|-------|------|-------------|
| `max_tokens_per_hour` | int | Maximum total tokens (prompt + completion) per hour |
| `max_tool_calls_per_hour` | int | Maximum number of tool calls per hour |
| `max_cost_per_hour_usd` | float64 | Maximum USD cost per hour (not yet fully enforced) |

## How It Works

1. **Quota Check**: Before executing an agent, the system checks if the agent has remaining quota
2. **Usage Recording**: After each agent execution, token usage and tool calls are recorded
3. **Rolling Window**: Each agent has its own hourly window starting from first usage
4. **Notifications**: When quotas are exceeded, a notification is sent to the dashboard's approval queue

## Default Configuration

If no quotas are configured, the following defaults apply:
- `max_tokens_per_hour`: 100,000
- `max_tool_calls_per_hour`: 100
- `max_cost_per_hour_usd`: 1.0

## Disabling Quotas

To disable quotas, set values to 0 or remove the `quotas` section from config.
