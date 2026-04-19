# 快速开始指南

## 概述

欢迎使用 FangClaw！本指南将帮助你快速上手。

## 系统要求

- Go 1.21 或更高版本
- 支持的操作系统：macOS、Linux、Windows
- 至少 2GB 可用内存

## 安装

### 方式一：从源码编译

```bash
git clone https://github.com/your-org/fangclaw-go
cd fangclaw-go
go build ./cmd/fangclaw-go
```

### 方式二：使用预编译版本

下载对应平台的预编译版本。

## 首次启动

### 1. 启动服务

```bash
./fangclaw-go
```

服务默认在 `http://localhost:8080` 启动。

### 2. 访问首页

打开浏览器访问 `http://localhost:8080`，你会看到 FangClaw 首页。

### 3. 登录 Dashboard

点击首页的 "立即开始" 或 "进入 Dashboard" 按钮。

## 配置

### 配置文件

FangClaw 使用 TOML 配置文件。首次启动会在当前目录生成 `config.toml`。

```toml
[api]
host = "0.0.0.0"
port = 8080
// 或者； api_listen = "0.0.0.0:8080"

[model]
provider = "openai"
api_key = "your-api-key"
```

### 配置模型提供商, 支持当下市场大多数AI提供商

支持的模型提供商：
- OpenAI
- Anthropic
- Groq
- Ollama
- OpenRouter
- zhipu
- volcengine
- Moonshot
- qwen

在 Dashboard 的 Settings 页面中配置 API 密钥。

## 创建第一个 Agent

### 1. 进入 Agents 页面

登录后，点击左侧菜单的 "Agents"。

### 2. 创建 Agent

点击 "New Agent"，选择一个模板或从头创建：

- 填写 Agent 名称
- 编写系统提示词
- 选择模型
- 配置技能（可选）

### 3. 测试 Agent

在 Chat 页面选择你的 Agent，开始对话！

## 下一步

- 了解 [工作流编排](./workflow-configuration.md)
- 探索 [Agent 模板](./agent-templates.md)
- 配置 [Scheduler](./scheduler-configuration.md)
- 学习 [A2A 功能](./A2A_TESTING.md)
- 配对 [微信扫码配对](./WECHAT_PAIRING_IMPLEMENTATION.md)
