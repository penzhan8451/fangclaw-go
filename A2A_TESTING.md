# A2A 功能测试指南

## 概述

本文档介绍如何测试 fangclaw-go 的 A2A（Agent-to-Agent）功能，包括本地 Agent 测试和与外部 Agent 通讯。

## 前置条件

1. 确保 fangclaw-go 服务已启动并运行
2. 确保至少有一个 Agent 已注册
3. 对于 Bash 脚本，需要安装 `jq` 工具
4. 对于 Python 脚本，需要安装 `requests` 和 `websocket-client` 库
    pip install requests websocket-client

## 快速开始

### 1. 启动 fangclaw-go 服务

```bash
cd /Users/lipeng/Downloads/AI/openfang-mig/fangclaw-go
go run cmd/fangclaw-go/main.go
```

服务默认监听 `http://localhost:8080`

### 2. 使用 Bash 脚本测试

```bash
# 给脚本添加执行权限
chmod +x test_a2a.sh

# 运行测试脚本
./test_a2a.sh
```

### 3. 使用 Python 脚本测试（推荐）

```bash
# 安装依赖
pip install requests websocket-client

# 运行完整测试
python test_a2a.py

# 测试外部 Agent 通讯
python test_a2a.py --external-url http://other-agent:8080

# 指定自定义服务地址
python test_a2a.py --url http://your-host:8080
```

## API 端点说明

### A2A 协议端点

#### 1. 获取 Agent Card
- **端点**: `GET /.well-known/agent.json`
- **说明**: 获取当前 Agent 的能力描述卡片
- **示例**:
```bash
curl http://localhost:8080/.well-known/agent.json
```

#### 2. 列出本地 Agents
- **端点**: `GET /a2a/agents`
- **说明**: 列出所有可用的外部 Agent Card
- **示例**:
```bash
curl http://localhost:8080/a2a/agents
```

#### 3. 发送任务
- **端点**: `POST /a2a/tasks/send`
- **说明**: 向本地 Agent 发送任务
- **请求体**:
```json
{
  "params": {
    "message": {
      "role": "user",
      "parts": [
        {
          "type": "text",
          "text": "你好，请介绍一下自己。"
        }
      ]
    },
    "sessionId": "test-session-001"
  }
}
```
- **示例**:
```bash
curl -X POST http://localhost:8080/a2a/tasks/send \
  -H "Content-Type: application/json" \
  -d @- <<EOF
{
  "params": {
    "message": {
      "role": "user",
      "parts": [
        {
          "type": "text",
          "text": "你好，请介绍一下自己。"
        }
      ]
    }
  }
}
EOF
```

#### 4. 获取任务状态
- **端点**: `GET /a2a/tasks/{id}`
- **说明**: 查询指定任务的状态和结果
- **示例**:
```bash
curl http://localhost:8080/a2a/tasks/your-task-id
```

#### 5. 取消任务
- **端点**: `POST /a2a/tasks/{id}/cancel`
- **说明**: 取消正在执行的任务
- **示例**:
```bash
curl -X POST http://localhost:8080/a2a/tasks/your-task-id/cancel
```

### 外部 Agent 通讯端点

#### 6. 发现外部 Agent
- **端点**: `POST /api/a2a/discover`
- **说明**: 发现并获取外部 Agent 的 Agent Card
- **请求体**:
```json
{
  "url": "http://external-agent:8080"
}
```
- **示例**:
```bash
curl -X POST http://localhost:8080/api/a2a/discover \
  -H "Content-Type: application/json" \
  -d '{"url": "http://external-agent:8080"}'
```

#### 7. 向外部 Agent 发送任务
- **端点**: `POST /api/a2a/send`
- **说明**: 向外部 Agent 发送任务
- **请求体**:
```json
{
  "agentUrl": "http://external-agent:8080",
  "params": {
    "message": {
      "role": "user",
      "parts": [
        {
          "type": "text",
          "text": "你好，外部 Agent！"
        }
      ]
    }
  }
}
```
- **示例**:
```bash
curl -X POST http://localhost:8080/api/a2a/send \
  -H "Content-Type: application/json" \
  -d @- <<EOF
{
  "agentUrl": "http://external-agent:8080",
  "params": {
    "message": {
      "role": "user",
      "parts": [
        {
          "type": "text",
          "text": "你好，外部 Agent！"
        }
      ]
    }
  }
}
EOF
```

#### 8. 查询外部任务状态
- **端点**: `GET /api/a2a/tasks/{id}/status`
- **说明**: 查询发送给外部 Agent 的任务状态
- **示例**:
```bash
curl http://localhost:8080/api/a2a/tasks/your-external-task-id/status
```

## WebSocket 实时推送

### 连接方式

#### 1. 连接所有任务
```
ws://localhost:8080/ws/a2a/tasks
```

#### 2. 按任务 ID 过滤
```
ws://localhost:8080/ws/a2a/tasks?taskId=your-task-id
```

#### 3. 按 Agent ID 过滤
```
ws://localhost:8080/ws/a2a/tasks?agentId=your-agent-id
```

### WebSocket 消息格式

消息推送格式参考代码中的实现，包含任务状态更新等信息。

### Python 示例

```python
import websocket
import json

def on_message(ws, message):
    print(f"收到消息: {message}")
    data = json.loads(message)
    print(f"任务状态: {data.get('status', {}).get('state')}")

def on_error(ws, error):
    print(f"错误: {error}")

def on_close(ws, close_status_code, close_msg):
    print("连接已关闭")

def on_open(ws):
    print("连接已建立")

ws = websocket.WebSocketApp(
    "ws://localhost:8080/ws/a2a/tasks",
    on_open=on_open,
    on_message=on_message,
    on_error=on_error,
    on_close=on_close
)

ws.run_forever()
```

## 测试流程

### 本地 Agent 测试流程

1. 启动 fangclaw-go 服务
2. 获取 Agent Card 验证服务正常
3. 列出本地 Agents 确认有可用的 Agent
4. 发送测试任务
5. 查询任务状态获取结果
6. （可选）使用 WebSocket 监听实时更新

### 外部 Agent 通讯测试流程

1. 确保两个 fangclaw-go 实例都在运行（或一个实例和一个兼容的外部 Agent）
2. 使用发现 API 获取外部 Agent 的 Agent Card
3. 向外部 Agent 发送任务
4. 查询外部任务状态
5. （可选）使用 WebSocket 监听实时更新

## 常见问题

### Q: 测试时提示 "No agents available"
A: 确保已创建并注册至少一个 Agent。

### Q: WebSocket 连接失败
A: 检查服务是否正常启动，确认 WebSocket 端点配置正确。

### Q: 外部 Agent 通讯失败
A: 确认外部 Agent URL 可访问，且外部 Agent 也实现了 A2A 协议。

### Q: 任务一直处于 working 状态
A: 检查 Agent 是否正常工作，查看日志确认任务处理情况。

## 下一步

- 查看代码中的 A2A 实现以了解更多细节
- 尝试实现自己的 A2A 兼容 Agent
- 探索更多 A2A 协议的高级功能
