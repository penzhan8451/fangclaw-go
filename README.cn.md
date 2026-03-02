<h1 align="center">FangClaw-Go</h1>
<h3 align="center">The Go Implementation of OpenFang</h3>

<p align="center">
  A feature-complete, Go-based Agent Operating System built from the OpenFang project. Open-source, production-ready, and battle-tested.<br/>
  <strong>One binary. Agents that actually work for you.</strong>
</p>

<p align="center">
  <a href="https://github.com/RightNow-AI/openfang">OpenFang (Original Rust Project)</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-blue?style=flat-square" alt="Go" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT" />
  <img src="https://img.shields.io/badge/version-0.2.0-green?style=flat-square" alt="v0.2.0" />
</p>

---

## 什么是 FangCla-go？

FangClaw-go 是 **OpenFang 项目的 Go 语言实现**，(https://github.com/RightNow-AI/openfang)

传统的代理框架等待您输入内容。FangClaw 运行 **为您工作的自主代理** — 按计划、全天候、构建知识图谱、监控目标、生成线索、管理社交媒体，并将结果报告到您的仪表板。

整个系统编译为一个二进制文件。一次安装，一个命令，您的代理就上线了。

```bash
git clone https://github.com/your-username/fangclaw-go.git
cd fangclaw-go
go build -o fangclaw-go ./cmd/fangclaw-go
./fangclaw-go setup
./fangclaw-go start
```

---

## 功能特性

### 核心功能

- 🤖 **智能 Agent 管理** - 完整的 Agent 生命周期管理
- 💬 **多通道通信** - 支持 Telegram、Discord、Slack、DingTalk、Feishu、QQ、WhatsApp 等
- 🎯 **工作流编排** - 灵活的工作流定义和执行
- 🔧 **技能系统** - 可扩展的技能加载和执行
- 📊 **事件总线** - 完整的发布/订阅事件系统
- 💾 **内存存储** - 结构化和语义化内存支持
- 🔐 **安全认证** - 完整的权限和认证系统

### 高级功能

- 🔌 **WhatsApp Gateway** - 集成 WhatsApp Web 网关（需要 Node.js）
- 🧙 **Wizard 系统**
  - NL Wizard - 自然语言生成 Agent 配置
  - Setup Wizard - 交互式命令行设置向导
- 🔄 **配置热加载** - 支持运行时配置更新
- 📱 **设备配对** - QR 码设备配对管理
- 📈 **使用计量** - 完整的使用统计和计量
- ⏰ **调度系统** - Cron 调度和后台任务执行

---

## 项目结构

```
fangclaw-go/
├── cmd/fangclaw-go/          # 主程序入口
├── internal/
│   ├── a2a/               # Agent-to-Agent 通信
│   ├── api/               # HTTP API 服务
│   ├── approvals/         # 审批系统
│   ├── audit/             # 审计日志
│   ├── auth/              # 认证系统
│   ├── autoreply/         # 自动回复引擎
│   ├── background/        # 后台任务执行
│   ├── browser/           # 浏览器自动化
│   ├── capabilities/      # 能力系统
│   ├── channels/          # 多通道适配器
│   ├── config/            # 配置管理
│   ├── configreload/      # 配置热加载
│   ├── cron/              # Cron 调度
│   ├── delivery/          # 消息投递
│   ├── embedding/         # 嵌入模型
│   ├── error/             # 错误处理
│   ├── eventbus/          # 事件总线
│   ├── hands/             # Hands 系统
│   ├── heartbeat/         # 心跳检测
│   ├── kernel/            # 核心 Kernel
│   ├── mcp/               # Model Context Protocol
│   ├── memory/            # 内存存储
│   ├── metering/          # 使用计量
│   ├── oauth/             # OAuth 认证
│   ├── p2p/               # P2P 网络
│   ├── pairing/           # 设备配对
│   ├── process/           # 进程管理
│   ├── runtime/           # Agent/LLM 运行时
│   ├── scheduler/         # 任务调度
│   ├── security/          # 安全系统
│   ├── skills/            # 技能系统
│   ├── supervisor/        # 监督系统
│   ├── triggers/          # 触发器
│   ├── tui/               # 终端 UI
│   ├── types/             # 核心类型定义
│   ├── vault/             # 密钥存储
│   ├── vector/            # 向量存储
│   ├── workflow/          # 工作流系统
│   ├── whatsappgateway/   # WhatsApp 网关
│   ├── wizard/            # NL Wizard
│   └── setupwizard/       # Setup Wizard
```

---

## 快速开始

### 安装

```bash
git clone https://github.com/your-username/fangclaw-go.git
cd fangclaw-go
go build -o fangclaw-go ./cmd/fangclaw-go
```

### 首次设置

运行设置向导：

```bash
./fangclaw-go setup
```

向导将引导您完成：
1. 选择 LLM 提供商（OpenAI、Anthropic、Groq、Ollama）
2. 配置 API Key
3. 选择默认模型
4. 设置数据目录

### 启动服务

```bash
./fangclaw-go start
```

### 启动后的下一步操作

守护进程运行后，您有多个选择：

#### 1. **探索可用的 Hands**
首先，列出所有 7 个内置的自主能力包：
```bash
./fangclaw-go hand list
```

#### 2. **激活一个 Hand**
激活全天候为您工作的自主能力：
```bash
# 激活 Researcher hand
./fangclaw-go hand activate researcher

# 查看 hand 状态
./fangclaw-go hand status researcher

# 暂停一个 hand
./fangclaw-go hand pause researcher

# 停用一个 hand
./fangclaw-go hand deactivate researcher
```

#### 3. **开始聊天**
开始交互式聊天（不需要先激活 hand）：
```bash
# 与默认代理聊天
./fangclaw-go chat

# 与特定的 Hand 聊天（使用专门的系统提示）
./fangclaw-go chat researcher    # 深度研究专家
./fangclaw-go chat lead          # 销售线索发掘
./fangclaw-go chat collector     # OSINT 情报收集
./fangclaw-go chat predictor     # 超级预测
./fangclaw-go chat clip          # YouTube 处理
./fangclaw-go chat twitter       # 社交媒体管理
./fangclaw-go chat browser       # 网页自动化
```

#### 4. **访问仪表板**
在浏览器中打开：
- **仪表板**: http://127.0.0.1:4200/
- **API 状态**: http://127.0.0.1:4200/api/health

#### 5. **检查系统状态**
```bash
./fangclaw-go status
```

#### 6. **查看日志**
```bash
./fangclaw-go logs
```

---

## 配置

FangClaw-go 使用 TOML 配置文件，默认位于 `~/.fangclaw-go/config.toml`。

### 示例配置

```toml
# API 服务器
api_listen = "127.0.0.1:4200"

# 默认模型（API key 从环境变量加载）
[default_model]
provider = "openrouter"
model = "openai/gpt-4o"
api_key_env = "OPENROUTER_API_KEY"

# 内存设置
[memory]
decay_rate = 0.05

# 安全设置
[security]
rate_limit_per_minute = 60

# 日志
[log]
level = "info"
```

> **注意**：API 密钥应存储在环境变量中，而不是配置文件中。 
> 请在 `~/.fangclaw-go/.fangclaw-go.env` 中设置您的 API 密钥：
> ```bash
> OPENROUTER_API_KEY=sk-...
> # 或者
> OPENAI_API_KEY=sk-...
> # 或者
> ANTHROPIC_API_KEY=sk-ant-...
> ```

---

## 关于 FangClaw-go

FangClaw-go 是基于 [OpenFang](https://github.com/RightNow-AI/openfang) 项目的 Go 语言重实现。OpenFang 是一个用 Rust 构建的功能完整的 Agent 操作系统，拥有 137K+ 代码行、14 个 crate 和 1,767+ 个测试。

我们感谢 OpenFang 项目的所有贡献者！

---

## 贡献

欢迎贡献！请提交 Issue 或 Pull Request。

---

## 许可证

本项目基于 OpenFang 项目，采用 MIT 许可证。

---

## 链接

- [OpenFang (Original Rust Project)](https://github.com/RightNow-AI/openfang)
- [OpenFang Documentation](https://openfang.sh/docs)

---

<p align="center">
  <strong>Built with Go. Based on OpenFang. Agents that actually work for you.</strong>
</p>
