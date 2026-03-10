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

## What is FangClaw?

FangClaw-go is a **Go language implementation of the OpenFang project**, (https://github.com/RightNow-AI/openfang)

Traditional agent frameworks wait for you to type something. FangClaw-go runs **autonomous agents that work for you** — on schedules, 24/7, building knowledge graphs, monitoring targets, generating leads, managing your social media, and reporting results to your dashboard.

The entire system compiles to a single binary. One install, one command, your agents are live.
---

## Features

### Core Features

- 🤖 **Smart Agent Management** - Complete Agent lifecycle management
- 💬 **Multi-Channel Communication** - Supports DingTalk, Feishu, QQ
- 🎯 **Workflow Orchestration** - Flexible workflow definition and execution
- 🔧 **Skill System** - Extensible skill loading and execution
- 📊 **Event Bus** - Complete publish/subscribe event system
- 💾 **Memory Storage** - Structured and semantic memory support
- 🔐 **Security & Authentication** - Complete permission and authentication system

### Advanced Features

- 🔌 **WhatsApp Gateway** - Integrated WhatsApp Web gateway (requires Node.js) -- TBD
- 🧙 **Wizard System**
  - NL Wizard - Natural language generation of Agent configuration -- TBD
  - Setup Wizard - Interactive command-line setup wizard
- 🔄 **Hot Configuration Reload** - Supports runtime configuration updates
- 📱 **Device Pairing** - QR code device pairing management
- 📈 **Usage Metering** - Complete usage statistics and metering
- ⏰ **Scheduler System** - Cron scheduling and background task execution

---

## Project Structure

```
fangclaw-go/
├── cmd/fangclaw-go/          # Main program entry
├── internal/
│   ├── a2a/               # Agent-to-Agent communication
│   ├── api/               # HTTP API service
│   ├── approvals/         # Approval system
│   ├── audit/             # Audit logging
│   ├── auth/              # Authentication system
│   ├── autoreply/         # Auto-reply engine
│   ├── background/        # Background task execution
│   ├── browser/           # Browser automation
│   ├── capabilities/      # Capabilities system
│   ├── channels/          # Multi-channel adapters
│   ├── config/            # Configuration management
│   ├── configreload/      # Hot configuration reload
│   ├── cron/              # Cron scheduling
│   ├── delivery/          # Message delivery
│   ├── embedding/         # Embedding models
│   ├── error/             # Error handling
│   ├── eventbus/          # Event bus
│   ├── hands/             # Hands system
│   ├── heartbeat/         # Heartbeat detection
│   ├── kernel/            # Core Kernel
│   ├── mcp/               # Model Context Protocol
│   ├── memory/            # Memory storage
│   ├── metering/          # Usage metering
│   ├── oauth/             # OAuth authentication
│   ├── p2p/               # P2P network
│   ├── pairing/           # Device pairing
│   ├── process/           # Process management
│   ├── runtime/           # Agent/LLM runtime
│   ├── scheduler/         # Task scheduling
│   ├── security/          # Security system
│   ├── skills/            # Skills system
│   ├── supervisor/        # Supervisor system
│   ├── triggers/          # Triggers
│   ├── tui/               # Terminal UI
│   ├── types/             # Core type definitions
│   ├── vault/             # Secret vault
│   ├── vector/            # Vector storage
│   ├── workflow/          # Workflow system
│   ├── whatsappgateway/   # WhatsApp gateway
│   ├── wizard/            # NL Wizard
│   └── setupwizard/       # Setup Wizard
```

---

## Quick Start

### Installation

```bash
git clone https://github.com/your-username/fangclaw-go.git
cd fangclaw-go
go build -o fangclaw-go ./cmd/fangclaw-go
```

### Configuration Example
In `~/.fangclaw-go/config.toml`, you can find the default configuration:

```bash
api_listen = "127.0.0.1:4200"
default_agent = "browser"

[default_model]
  provider = "openrouter"
  model = "openrouter/auto"
  api_key_env = "OPENROUTER_API_KEY"

[memory]
  decay_rate = 0.05

[security]
  rate_limit_per_minute = 0

[log]
  level = ""
  file = ""
```

### First-time Setup

Run the setup wizard:

```bash
./fangclaw-go init
```

The wizard will guide you through:
1. Selecting an LLM provider (OpenAI, Anthropic, Groq, Ollama, OpenRouter)
2. Configuring API Key
3. Selecting default model
4. Setting data directory


### Next Steps After Starting

Once the daemon is running, you have several options:

#### 1. **Explore Available Hands**
First, list all 7 bundled autonomous capability packages:
```bash
./fangclaw-go hand list
```

#### 2. **Activate a Hand**
Activate autonomous capabilities that work for you 24/7:
```bash
# Activate the Researcher hand
./fangclaw-go hand activate researcher

# Check hand status
./fangclaw-go hand status researcher

# Pause a hand
./fangclaw-go hand pause researcher

# Deactivate a hand
./fangclaw-go hand deactivate researcher
```

#### 3. **Start Chatting**
Begin an interactive chat (doesn't require activating a hand first):
```bash
# Chat with default agent
./fangclaw-go chat

# Chat with a specific Hand (uses specialized system prompts)
./fangclaw-go chat researcher    # Deep research specialist
./fangclaw-go chat lead          # Sales prospecting
./fangclaw-go chat collector     # OSINT intelligence
./fangclaw-go chat predictor     # Superforecasting
./fangclaw-go chat clip          # YouTube processing
./fangclaw-go chat twitter       # Social media management
./fangclaw-go chat browser       # Web automation
```

#### 4. **Access the Dashboard**
Open your browser to:
- **Dashboard**: http://127.0.0.1:4200/
- **API Status**: http://127.0.0.1:4200/api/health

#### 5. **Check System Status**
```bash
./fangclaw-go status
```

#### 6. **View Logs**
```bash
./fangclaw-go logs
```
---

## Configuration

FangClaw-go uses TOML configuration files, located by default at `~/.fangclaw-go/config.toml`.

### Example Configuration

```toml
# API Server
api_listen = "127.0.0.1:4200"

# Default Model (API key is loaded from environment variable)
[default_model]
provider = "openrouter"
model = "openai/gpt-4o"
api_key_env = "OPENROUTER_API_KEY" # or "OPENAI_API_KEY" or "ANTHROPIC_API_KEY"

# Memory Settings
[memory]
decay_rate = 0.05

# Security Settings
[security]
rate_limit_per_minute = 60

# Logging
[log]
level = "info"
```

> **Note**: API keys should be stored in environment variables, not in the config file. 
> Set your API key in `~/.fangclaw-go/.fangclaw-go.env`:
> ```bash
> OPENROUTER_API_KEY=sk-...
> # or
> OPENAI_API_KEY=sk-...
> # or
> ANTHROPIC_API_KEY=sk-ant-...
> ```
or, in the console:
```bash
export OPENROUTER_API_KEY=sk-...
# or
export OPENAI_API_KEY=sk-...
# or
export ANTHROPIC_API_KEY=sk-ant-...
```
---

## About FangClaw-go

FangClaw-go is a Go language reimplementation based on the [OpenFang](https://github.com/RightNow-AI/openfang) project, which is a feature-complete Agent Operating System built in Rust, with 137K+ lines of code, 14 crates, and 1,767+ tests.

---

## Contributing

Contributions are welcome! Please submit Issues or Pull Requests.

---

## License

This project is based on the OpenFang project and uses the MIT license.

---

## Links

- [OpenFang (Original Rust Project)](https://github.com/RightNow-AI/openfang)
- [OpenFang Documentation](https://openfang.sh/docs)

---

<p align="center">
  <strong>Built with Go. Based on OpenFang. Agents that actually work for you.</strong>
</p>
