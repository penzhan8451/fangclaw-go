<h1 align="center">FangClaw-Go</h1>
<h3 align="center">The Go Implementation based on OpenFang</h3>

<p align="center">
  A feature-complete, Go-based Agent Operating Platform based on the OpenFang project. Open-source, production-ready, and battle-tested.<br/>
  <strong>One binary. Agents that actually work for you.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-blue?style=flat-square" alt="Go" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT" />
  <img src="https://img.shields.io/badge/version-0.2.0-green?style=flat-square" alt="v0.2.0" />
</p>

***

## What is FangClaw-Go?

FangClaw-go is a **Go language implementation based on the OpenFang project**, (<https://github.com/RightNow-AI/openfang>)

Traditional agent frameworks wait for you to type something. FangClaw-go runs **autonomous agents that work for you** — on schedules, 24/7, building knowledge graphs, monitoring targets, generating leads, managing your social media, and reporting results to your dashboard. **It is multiuser supported，and dashboard can do almost everything that CLI can do through dashboard.**

## The entire system compiles to a single binary. One install, one command, your agents are live.

## Features

### Core Features

- 🤖 **Smart Agent Management** - Complete Agent lifecycle management
- 💬 **Multi-Channel Communication** - Supports DingTalk, Feishu, QQ, WeChat Personal
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
- ⏰ **Scheduler System** - Cron scheduling and background task execution

***

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
│   ├── multi_tenant_server/ # Multi-tenant server
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

***

## Quick Start

### Installation

```bash
git clone https://github.com/your-username/fangclaw-go.git
cd fangclaw-go
go build -o fangclaw-go ./cmd/fangclaw-go
```

If you have binary already compiled for your OS platform, you can skip this step.

### First-time Setup

This step is **Only** for the case you can access CLI, mostly it is owner role user.
For **user** role, you can use dashboard to configure your provider and model for the first time (Settings -> Provider and Config tab).

Run the setup wizard:

```bash
./fangclaw-go init
```

The wizard will generate some default files, you can modify in the config.toml file located by default at `~/.fangclaw-go/config.toml`:

1. LLM provider (OpenAI, Anthropic, Groq, Ollama, OpenRouter, etc.)
2. Configuring Model (e.g., openai/gpt-34o, openai/gpt-4o, etc.)
3. Selecting default model

(After daemon started, you can also configure them by visit dashboard: <http://127.0.0.1:4200/>, Settings->Provider and Config tab.)

Then, start the daemon:

```bash
./fangclaw-go start
```

Check System Status

```bash
./fangclaw-go status
```

### Next Steps After Starting

Once the daemon is running, you have several options: (For CLI user):

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

If you don't want to use CLI or cannot access CLI, you can use dashboard to manage your agents. You can create, activate, chat with agents or hands in the dashboard. Open your browser to:

- **Dashboard**: <http://127.0.0.1:4200/>
- **API Status**: <http://127.0.0.1:4200/api/health>

You can spawn and chat with agents in the dashboard.

![Chat with an agent](image.png)

***

### Example Configuration in config.toml

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

Set your API key in `~/.fangclaw-go/secrets.env`:

```bash
OPENROUTER_API_KEY=sk-...
# or
OPENAI_API_KEY=sk-...
# or
ANTHROPIC_API_KEY=sk-ant-...
```

***

## Skills System

FangClaw-go supports external skills that can extend agent capabilities. Skills can be loaded from directories and provide tools that agents can use.

### Skill Directory Structure

For owner role user (CLI user), Skills are stored in `~/.fangclaw-go/skills/` directory by default. Each skill should be in its own subdirectory:

```
~/.fangclaw-go/skills/
├── my-skill/
│   ├── manifest.json    # Skill manifest (JSON format)
│   ├── skill.toml       # OR Skill manifest (TOML format - coming soon)
│   ├── SKILL.md         # OR Skill manifest with YAML frontmatter
│   ├── main.py          # Python skill entry point if has
│   └── main.js          # Node.js skill entry point if has
```

### Skill Manifest Formats

FangClaw-go supports three manifest formats:

#### 1. **manifest.json (JSON Format)**

```json
{
  "version": "1.0.0",
  "name": "My Skill",
  "description": "A sample skill that provides useful tools",
  "author": "Your Name",
  "runtime": {
    "runtime_type": "python",
    "entry": "main.py",
    "version": "3.8+"
  },
  "tools": {
    "provided": [
      {
        "name": "my_tool",
        "description": "A useful tool provided by this skill",
        "parameters": {
          "type": "object",
          "properties": {
            "param1": {
              "type": "string",
              "description": "First parameter"
            }
          },
          "required": ["param1"]
        }
      }
    ]
  },
  "requirements": {
    "python": ["requests", "beautifulsoup4"],
    "node": [],
    "system": []
  }
}
```

#### 2. **SKILL.md (Markdown with YAML Frontmatter)**

````markdown
---
name: My Skill
description: A sample skill that provides useful tools
version: 1.0.0
author: Your Name
tags: ["utility", "tools"]
runtime:
  runtime_type: python
  entry: main.py
  version: 3.8+
tools:
  provided:
    - name: my_tool
      description: A useful tool provided by this skill
      parameters:
        type: object
        properties:
          param1:
            type: string
            description: First parameter
        required:
          - param1
requirements:
  python:
    - requests
    - beautifulsoup4
  node: []
  system: []
---

# My Skill

This is a skill that provides useful tools. The content here becomes the prompt context for the agent.

## How to use

Agents can use the `my_tool` function to perform actions.


### Supported Runtime Types

- **`prompt`** (Default) - Prompt-only skill, adds context to agent's system prompt
- **`python`** - Python skill, executes Python scripts
- **`node`** - Node.js skill, executes Node.js scripts
- **`wasm`** - WebAssembly skill (coming soon)
- **`builtin`** - Built-in skills handled by the kernel

### Installing and Loading Skills (For CLI user)

#### Install a Skill from Directory

```bash
./fangclaw-go skill install /path/to/skill-directory my-skill
````

#### List Installed Skills

```bash
./fangclaw-go skill list
```

#### Loading Skills

**Built-in Hands** are automatically loaded when the daemon starts. These are the predefined capability packages (like Researcher, Lead, Collector, etc.).

**External Skills** are loaded on-demand when an Agent uses them. When an Agent specifies skills in its configuration, those skills are automatically loaded from `~/.fangclaw-go/skills/{skillID}/` directory when the Agent first runs.

**Important Note**: Skills are configured per-Agent, not in the global `~/.fangclaw-go/config.toml`. You cannot set `skills = ["..."]` in the global config file. See the example below for how to configure skills in an Agent's configuration.

##### Example: Using an External GitHub Skill

1. **Create the GitHub Skill Directory:**

```bash
mkdir -p ~/.fangclaw-go/skills/github/
```

2. **Create** **`~/.fangclaw-go/skills/github/skill.md`:**

```markdown
---
name: GitHub
version: 1.0.0
description: Interact with GitHub repositories, issues, and PRs
prompt_context: |
  You are a GitHub assistant. You can help users with GitHub-related tasks.

  Available tools:
  - github_search: Search GitHub repositories
  - github_issues: List and manage issues
  - github_prs: List and manage pull requests

  Always be concise and helpful.
---
# GitHub Skill

This skill provides GitHub integration capabilities.
```

3. **Configure an Agent to Use the GitHub Skill:**

Create `github-agent.json`:

```json
{
  "id": "devops",
  "name": "DevOps Engineer",
  "description": "A systems-focused agent for CI/CD, infrastructure, Docker, and deployment troubleshooting.",
  "category": "Development",
  "icon": "DO",
  "provider": "deepseek",
  "model": "deepseek-chat",
  "profile": "precise",
  "system_prompt": "You are a DevOps engineer. Help with CI/CD pipelines, Docker, Kubernetes, infrastructure as code, and deployment. Prioritize reliability and security.",
  "tools": [
    "file_read",
    "web_search",
    "shell_exec"
  ],
  "skills": ["github"],   # Specify the skill to use!
  "mcp_servers": []
}
```

### Installing and Loading Skills (For Dashboard User)

You can install skills from ClawHub or upload your own skills fromn the dashboard -> Skills sidebar.

![Install Skill from ClawHub or Upload your Skill (SKILL.md or zip)](image-1.png)

## **Agent Configuration**

1. **Create the Agent:**

![Use setup wizard](image-3.png)


After creation, you can configure the Agent's skills in the Agent Details window to add external skills.

![Agent Details](image-4.png)

***

## Channel Configuration

FangClaw-go supports multiple communication channels including QQ, Feishu (Lark), and DingTalk. Channels can be configured via environment variables or through the dashboard.

### QQ Channel Configuration

#### Getting QQ Credentials

1. Go to [QQ Open Platform](https://app.open.qq.com/)
2. Create a new bot application
3. Get your App ID and App Secret from the application dashboard

#### Configuration in Config File (For CLI user)

You can also configure in `~/.fangclaw-go/config.toml`:

```toml
[channels]
  [channels.qq]
    app_id = "102876188"
    app_secret = "your_qq_app_secret"
    allow_from = ["user_id_1", "user_id_2"]  # Optional: restrict to specific users
```

For **Dashboard** User:
Click sidebar menu Channels, then find QQ card:

![Setup QQ Channel](/image-2.png)

### Feishu (Lark) Channel Configuration

#### Getting Feishu Credentials

1. Go to [Feishu Open Platform](https://open.feishu.cn/)
2. Create a custom app
3. Get App ID and App Secret from "Credentials & Basic Info"
4. Enable necessary permissions (im:message, im:resource)

#### Via Config File (For CLI user)

```toml
[channels]
  [channels.feishu]
    app_id = "your_feishu_app_id"
    app_secret = "your_feishu_app_secret"
```

For **Dashboard** User:
Click sidebar menu Channels, then find Feishu card to setup your Feishu channel.

### DingTalk Channel Configuration

#### Getting DingTalk Credentials

1. Go to [DingTalk Open Platform](https://open.dingtalk.com/)
2. Create a H5 micro-app or robot
3. Get App Key and App Secret

#### Configuration in Config File (For CLI user)

```toml
[channels]
  [channels.dingtalk]
    app_key = "your_dingtalk_app_key"
    app_secret = "your_dingtalk_app_secret"
```

For **Dashboard** User:
Click sidebar menu Channels, then find DingTalk card to setup your DingTalk channel.

## About FangClaw-go

FangClaw-go is a Go language reimplementation based on the [OpenFang](https://github.com/RightNow-AI/openfang) project, which is a feature-complete Agent Operating System built in Rust, with 137K+ lines of code, 14 crates, and 1,767+ tests.

***

## Contributing

Contributions are welcome! Please submit Issues or Pull Requests.

***

## License

This project is based on the OpenFang project and uses the MIT license.

***

## Links

- [OpenFang (Original Rust Project)](https://github.com/RightNow-AI/openfang)
- [OpenFang Documentation](https://openfang.sh/docs)

***

<p align="center">
  <strong>Built with Go. Based on OpenFang. Agents that actually work for you.</strong>
</p>
