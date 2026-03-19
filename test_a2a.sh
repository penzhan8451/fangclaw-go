#!/bin/bash

# A2A 功能测试脚本
# 测试 fangclaw-go 的 A2A 功能

BASE_URL="http://localhost:8080"

echo "========================================="
echo "  A2A 功能测试"
echo "========================================="

# 1. 测试获取 Agent Card
echo ""
echo "1. 测试获取 Agent Card (GET /.well-known/agent.json)"
echo "--------------------------------------------------------"
curl -s "${BASE_URL}/.well-known/agent.json" | jq .
if [ $? -eq 0 ]; then
    echo "✓ 成功获取 Agent Card"
else
    echo "✗ 获取 Agent Card 失败"
fi

echo ""
read -p "按回车继续..."

# 2. 测试列出外部发现的Agents
echo ""
echo "2. List all discovered external A2A agents: (GET /a2a/agents)"
echo "-------------------------------------------"
curl -s "${BASE_URL}/a2a/agents" | jq .
if [ $? -eq 0 ]; then
    echo "✓ 成功列出 Agents"
else
    echo "✗ 列出 Agents 失败"
fi

echo ""
read -p "按回车继续..."

# 3. 测试发送任务
echo ""
echo "3. 测试发送任务 (POST /a2a/tasks/send)"
echo "----------------------------------------"
TASK_RESPONSE=$(curl -s -X POST "${BASE_URL}/a2a/tasks/send" \
    -H "Content-Type: application/json" \
    -d '{
        "params": {
            "message": {
                "role": "user",
                "parts": [
                    {
                        "type": "text",
                        "text": "你好！请介绍一下自己。"
                    }
                ]
            },
            "sessionId": "test-session-001"
        }
    }')

echo "$TASK_RESPONSE" | jq .

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.id // empty')
if [ -n "$TASK_ID" ] && [ "$TASK_ID" != "null" ]; then
    echo "✓ 任务发送成功，任务 ID: $TASK_ID"
else
    echo "✗ 任务发送失败"
    exit 1
fi

echo ""
read -p "按回车继续..."

# 4. 测试查询任务状态
echo ""
echo "4. 测试查询任务状态 (GET /a2a/tasks/${TASK_ID})"
echo "--------------------------------------------------"

echo "等待任务处理..."
sleep 3

curl -s "${BASE_URL}/a2a/tasks/${TASK_ID}" | jq .
if [ $? -eq 0 ]; then
    echo "✓ 成功获取任务状态"
else
    echo "✗ 获取任务状态失败"
fi

echo ""
read -p "按回车继续..."

# 5. 测试发现外部 Agent
echo ""
echo "5. 测试发现外部 Agent (POST /api/a2a/discover)"
echo "------------------------------------------------"
echo "测试外部 Agent 的方式："
echo "  1. 使用 mock server: 运行 'python3 mock_a2a_server.py'（默认端口 9000）"
echo "  2. 运行另一个 fangclaw-go 实例（修改 config.toml 中的端口）"
echo ""
read -p "输入外部 Agent URL (例如: http://127.0.0.1:9000): " EXTERNAL_URL

if [ -n "$EXTERNAL_URL" ]; then
    DISCOVER_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/a2a/discover" \
        -H "Content-Type: application/json" \
        -d "{\"url\": \"${EXTERNAL_URL}\"}")
    
    echo "$DISCOVER_RESPONSE" | jq .
    
    if [ $? -eq 0 ]; then
        echo "✓ 外部 Agent 发现请求已发送"
    else
        echo "✗ 外部 Agent 发现失败"
    fi
else
    echo "跳过外部 Agent 发现测试"
fi

echo ""
read -p "按回车继续..."

# 6. 测试向外部 Agent 发送任务
echo ""
echo "6. 测试向外部 Agent 发送任务 (POST /api/a2a/send)"
echo "-----------------------------------------------------"
echo "测试外部 Agent 的方式："
echo "  1. 使用 mock server: 运行 'python3 mock_a2a_server.py'（默认端口 9000）"
echo "  2. 运行另一个 fangclaw-go 实例（修改 config.toml 中的端口）"
echo ""
read -p "输入外部 Agent URL (例如: http://127.0.0.1:9000): " EXTERNAL_URL2

if [ -n "$EXTERNAL_URL2" ]; then
    EXTERNAL_TASK_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/a2a/send" \
        -H "Content-Type: application/json" \
        -d "{
            \"agentUrl\": \"${EXTERNAL_URL2}\",
            \"params\": {
                \"message\": {
                    \"role\": \"user\",
                    \"parts\": [
                        {
                            \"type\": \"text\",
                            \"text\": \"你好，外部 Agent！\"
                        }
                    ]
                }
            }
        }")
    
    echo "$EXTERNAL_TASK_RESPONSE" | jq .
    
    EXTERNAL_TASK_ID=$(echo "$EXTERNAL_TASK_RESPONSE" | jq -r '.taskId // empty')
    if [ -n "$EXTERNAL_TASK_ID" ] && [ "$EXTERNAL_TASK_ID" != "null" ]; then
        echo "✓ 外部任务发送成功，任务 ID: $EXTERNAL_TASK_ID"
        
        echo ""
        read -p "按回车查询外部任务状态..."
        
        curl -s "${BASE_URL}/api/a2a/tasks/${EXTERNAL_TASK_ID}/status" | jq .
    else
        echo "✗ 外部任务发送失败"
    fi
else
    echo "跳过外部任务发送测试"
fi

echo ""
echo "========================================="
echo "  测试完成！"
echo "========================================="
echo ""
echo "WebSocket 实时推送："
echo "  连接：ws://localhost:8080/ws/a2a/tasks"
echo "  按任务ID过滤：ws://localhost:8080/ws/a2a/tasks?taskId=${TASK_ID}"
echo "  按Agent过滤：ws://localhost:8080/ws/a2a/tasks?agentId=your-agent-id"
