# FangGo Cron 功能示例

本目录包含 fanggo 中 cron 定时任务功能的各种示例。

## 目录结构

```
examples/cron-examples/
├── simple/          # 简单示例
│   ├── 01-at-single-time.json
│   ├── 02-every-10-minutes.json
│   ├── 03-cron-daily-morning.json
│   ├── 04-every-hour-system-event.json
│   └── 05-cron-weekly-report.json
├── complex/         # 复杂示例
│   ├── 01-execute-shell-daily-backup.json
│   ├── 02-agent-turn-with-model-override.json
│   ├── 03-health-check-with-webhook.json
│   ├── 04-midnight-cleanup-task.json
│   ├── 05-weekly-summary-to-email.json
│   └── 06-monthly-billing-report.json
└── README.md        # 本文档
```

## 功能概述

fanggo 的 cron 功能支持三种调度方式：

### 1. Schedule Kind: "at" - 一次性定时任务
在指定的时间点执行一次任务。

**示例：**
```json
{
  "schedule": {
    "kind": "at",
    "at": "2026-04-02T10:00:00+08:00"
  }
}
```

### 2. Schedule Kind: "every" - 周期性定时任务
每隔指定秒数重复执行任务。

**示例：**
```json
{
  "schedule": {
    "kind": "every",
    "every_secs": 600
  }
}
```

### 3. Schedule Kind: "cron" - Cron 表达式
使用标准 cron 表达式定义调度任务。

**示例：**
```json
{
  "schedule": {
    "kind": "cron",
    "expr": "0 9 * * *"
  }
}
```

## Action 类型

### 1. Action Kind: "agent_turn"
触发 agent 对话。

```json
{
  "action": {
    "kind": "agent_turn",
    "message": "请提醒我今天的会议安排",
    "model_override": "gpt-4",
    "timeout_secs": 300
  }
}
```

### 2. Action Kind: "system_event"
发送系统事件。

```json
{
  "action": {
    "kind": "system_event",
    "text": "hourly_check_event"
  }
}
```

### 3. Action Kind: "execute_shell"
执行 shell 命令。

```json
{
  "action": {
    "kind": "execute_shell",
    "command": "date",
    "args": ["+%Y-%m-%d %H:%M:%S"],
    "timeout_secs": 30
  }
}
```

## Delivery 类型

### 1. Delivery Kind: "none"
不进行任何消息投递。

```json
{
  "delivery": {
    "kind": "none"
  }
}
```

### 2. Delivery Kind: "last_channel"
投递到最近使用的频道。

```json
{
  "delivery": {
    "kind": "last_channel"
  }
}
```

### 3. Delivery Kind: "channel"
投递到指定频道。

```json
{
  "delivery": {
    "kind": "channel",
    "channel_name": "slack",
    "recipient": "#daily-report"
  }
}
```

### 4. Delivery Kind: "webhook"
通过 Webhook 投递。

```json
{
  "delivery": {
    "kind": "webhook",
    "url": "https://your-webhook-url.com"
  }
}
```

## 使用方法

You can create a cron task through dashboard or CLI:
`go build -o fanggo ./cmd/fangclaw-go`

### 1. 创建定时任务

```bash
# 确保 daemon 正在运行
fanggo start

# 创建定时任务
fanggo cron create examples/cron-examples/simple/03-cron-daily-morning.json
```

### 2. 列出所有定时任务

```bash
fanggo cron list
```

### 3. 启用/禁用定时任务

```bash
# 启用
fanggo cron enable <job-id>

# 禁用
fanggo cron disable <job-id>
```

### 4. 查看任务状态

```bash
fanggo cron status <job-id>
```

### 5. 删除定时任务

```bash
fanggo cron delete <job-id>
```

## Cron 表达式说明

标准 cron 表达式格式：`分 时 日 月 周`

| 字段 | 允许值 |
|------|--------|
| 分 | 0-59 |
| 时 | 0-23 |
| 日 | 1-31 |
| 月 | 1-12 |
| 周 | 0-6 (0=周日) |

**常用示例：**
- `* * * * *` - 每分钟执行
- `0 * * * *` - 每小时执行
- `0 9 * * *` - 每天上午9点执行
- `0 9 * * 1-5` - 工作日上午9点执行
- `0 18 * * 5` - 每周五下午6点执行
- `0 2 * * *` - 每天凌晨2点执行
- `0 0 * * 0` - 每周日午夜执行
- `0 8 1 * *` - 每月1号上午8点执行

## 重要提示

1. **agent_id**：使用前请将示例中的 `YOUR-AGENT-ID-HERE` 替换为实际的 agent ID
2. **时间格式**：使用 RFC3339 格式，例如：`2026-04-02T10:00:00+08:00`
3. **时区**：cron 表达式使用 UTC 时间
4. **最大任务数**：每个 agent 最多 50 个任务
5. **最大执行时间**：execute_shell 命令建议设置合理的 timeout_secs
6. **错误处理**：连续 5 次执行失败会自动禁用任务

## 更多信息

查看代码实现：
- `/internal/cron/cron.go` - Cron 调度器实现
- `/internal/types/scheduler.go` - 数据类型定义
- `/cmd/fangclaw-go/commands/cron.go` - CLI 命令实现
