# 示例集合

这里包含了 Fangclaw-Go 的各种使用示例，帮助你快速入门。

## 目录

- [定时任务示例](#定时任务示例)
- [工作流示例](#工作流示例)
- [事件总线处理器示例](#事件总线处理器示例)

## 定时任务示例

在 [cron-examples/](cron-examples/README.md) 目录下，包含了各种定时任务的配置示例。

### 简单示例

- [cron-examples/simple/01-at-single-time.json](cron-examples/simple/01-at-single-time.json) - 单次执行任务
- [cron-examples/simple/02-every-10-minutes.json](cron-examples/simple/02-every-10-minutes.json) - 每10分钟执行
- [cron-examples/simple/03-cron-daily-morning.json](cron-examples/simple/03-cron-daily-morning.json) - 每天早上执行
- [cron-examples/simple/04-every-hour-system-event.json](cron-examples/simple/04-every-hour-system-event.json) - 每小时系统事件
- [cron-examples/simple/05-cron-weekly-report.json](cron-examples/simple/05-cron-weekly-report.json) - 每周报告

### 复杂示例

- [cron-examples/complex/01-execute-shell-daily-backup.json](cron-examples/complex/01-execute-shell-daily-backup.json) - 每日备份
- [cron-examples/complex/02-agent-turn-with-model-override.json](cron-examples/complex/02-agent-turn-with-model-override.json) - Agent 切换和模型覆盖
- [cron-examples/complex/03-health-check-with-webhook.json](cron-examples/complex/03-health-check-with-webhook.json) - 健康检查和 Webhook
- [cron-examples/complex/04-midnight-cleanup-task.json](cron-examples/complex/04-midnight-cleanup-task.json) - 午夜清理任务
- [cron-examples/complex/05-weekly-summary-to-email.json](cron-examples/complex/05-weekly-summary-to-email.json) - 每周摘要邮件
- [cron-examples/complex/06-monthly-billing-report.json](cron-examples/complex/06-monthly-billing-report.json) - 月度账单报告

## 工作流示例

在 [workflow-examples/](workflow-examples/README.md) 目录下，包含了各种工作流配置示例。

### 简单示例

- [workflow-examples/simple/01-simple-sequential-pipeline.json](workflow-examples/simple/01-simple-sequential-pipeline.json) - 简单顺序管道
- [workflow-examples/simple/02-conditional-workflow.json](workflow-examples/simple/02-conditional-workflow.json) - 条件工作流
- [workflow-examples/simple/03-fan-out-parallel.json](workflow-examples/simple/03-fan-out-parallel.json) - 并行执行
- [workflow-examples/simple/04-error-handling-with-retry.json](workflow-examples/simple/04-error-handling-with-retry.json) - 错误处理和重试
- [workflow-examples/simple/05-variable-passing.json](workflow-examples/simple/05-variable-passing.json) - 变量传递

### 复杂示例

- [workflow-examples/complex/01-software-release-workflow.json](workflow-examples/complex/01-software-release-workflow.json) - 软件发布工作流
- [workflow-examples/complex/02-content-creation-pipeline.json](workflow-examples/complex/02-content-creation-pipeline.json) - 内容创建管道
- [workflow-examples/complex/03-market-research-report.json](workflow-examples/complex/03-market-research-report.json) - 市场研究报告
- [workflow-examples/complex/04-technical-documentation-workflow.json](workflow-examples/complex/04-technical-documentation-workflow.json) - 技术文档工作流
- [workflow-examples/complex/05-data-analysis-pipeline.json](workflow-examples/complex/05-data-analysis-pipeline.json) - 数据分析管道
- [workflow-examples/complex/06-customer-support-ticket-handler.json](workflow-examples/complex/06-customer-support-ticket-handler.json) - 客户支持票处理

## 事件总线处理器示例

在 [eventbus-handlers/](eventbus-handlers/README.md) 目录下，包含了各种事件总线处理器的 Go 代码示例。

### 简单示例

- [eventbus-handlers/simple/01-event-logger.go](eventbus-handlers/simple/01-event-logger.go) - 事件记录器
- [eventbus-handlers/simple/02-agent-lifecycle-monitor.go](eventbus-handlers/simple/02-agent-lifecycle-monitor.go) - Agent 生命周期监控
- [eventbus-handlers/simple/03-message-counter.go](eventbus-handlers/simple/03-message-counter.go) - 消息计数器
- [eventbus-handlers/simple/04-workflow-tracker.go](eventbus-handlers/simple/04-workflow-tracker.go) - 工作流追踪器

### 实际应用示例

- [eventbus-handlers/real-world/01-alerting-system.go](eventbus-handlers/real-world/01-alerting-system.go) - 告警系统
- [eventbus-handlers/real-world/02-metrics-collector.go](eventbus-handlers/real-world/02-metrics-collector.go) - 指标收集器
- [eventbus-handlers/real-world/03-webhook-integrator.go](eventbus-handlers/real-world/03-webhook-integrator.go) - Webhook 集成
- [eventbus-handlers/real-world/04-audit-logger.go](eventbus-handlers/real-world/04-audit-logger.go) - 审计记录器

## 如何使用

### 定时任务和工作流

JSON 配置文件可以通过 API 加载到系统中。详细请参考 [workflow-configuration.md](../docs/workflow-configuration.md)。

### 事件总线处理器

Go 代码示例展示了如何编写自定义的事件处理器。详细请参考相关文档。

---

更多示例请查看各个子目录的 README 文件！
