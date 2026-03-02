# OpenFang-Go Codebase Map

This document provides a structured map of the OpenFang-Go codebase, focusing on modules, entry points, and configuration/data models relevant to parity guarantees.

## 1. Module boundaries
- Root module: github.com/rightnow-ai/openfang (from /go.mod)
- Primary directories:
  - cmd/        : CLI entry points for OpenFang
  - internal/   : Core runtime, kernel, API, config, memory, etc.

## 2. Entry points
- CLI entry point
  - /Users/lipeng/Downloads/AI/openfang-go/cmd/openfang/main.go
  - Role: Bootstraps the CLI, defines root command and help/usage; calls commands.Register to wire subcommands
- Daemon/kernel core
  - /Users/lipeng/Downloads/AI/openfang-go/internal/kernel/kernel.go
  - Role: OpenFangKernel struct and boot lifecycle; manages config, DB, model catalog, tools, runtime
- API server
  - /Users/lipeng/Downloads/AI/openfang-go/internal/api/server.go
  - Role: HTTP API server with health/status endpoints; routes likely wired in internal/api/routes.go
- Configuration
  - /Users/lipeng/Downloads/AI/openfang-go/internal/config/config.go
  - Role: DefaultConfig, Load, Save, Get/Set/Unset helpers; core configuration model

## 3. Core data models (representing parity touchpoints)
- Agent and related types
  - /Users/lipeng/Downloads/AI/openfang-go/internal/types/agent.go
  - Key types: AgentID, AgentState, Agent struct, AgentManifest, Model, Provider, etc.
- Configuration model
  - /Users/lipeng/Downloads/AI/openfang-go/internal/config/config.go
  - Key types: Config, ModelSettings, MemorySettings, SecuritySettings, LogSettings

## 4. Key subsystems and interactions
- Persistence
  - /Users/lipeng/Downloads/AI/openfang-go/internal/memory/db.go
  - In-memory DB layer; used by kernel for state and migrations
- Process management
  - /Users/lipeng/Downloads/AI/openfang-go/internal/process/manager.go
  - Manages child processes with restart-on-crash policy
- Channel and eventing
  - /Users/lipeng/Downloads/AI/openfang-go/internal/channels/adapters.go
  - Boundary adapters for channels
  - /Users/lipeng/Downloads/AI/openfang-go/internal/eventbus/eventbus.go
  - Intra-system event bus
- API routing and health
  - /Users/lipeng/Downloads/AI/openfang-go/internal/api/routes.go
  - API route wiring (look for health/status) – routes likely wired into server.go

## 5. Parity touchpoints (high-level hypotheses)
- Boot and initialization parity
  - OpenFangKernel.New and Boot orchestrate initialization of DB, drivers, and runtime; parity guarantees rely on consistent startup ordering
- Model/provider wiring parity
  - Kernel.ModelCatalog initialized and providers/models wired via llm drivers (internal/kernel/kernel.go interactions with llm model catalog)
- Persistence parity
  - Memory DB migrations and consistency checks (db.go) ensure deterministic state
- API parity and health
  - API server exposes health/status and ensures stable routes; parity endpoints help validate runtime state

## 6. Verification plan (quick commands)
- Build and smoke test
  - go build ./...
- Entry point checks
  - grep -R "package main" -n /Users/lipeng/Downloads/AI/openfang-go/cmd/openfang/main.go
  - grep -R "Register(" -n /Users/lipeng/Downloads/AI/openfang-go/cmd/openfang/commands
- Config sanity
  - go run -e "package main" test (or small Go snippet) to call default config load via internal/config
- API routing sanity
  - Inspect internal/api/server.go for RunServer/Start usage and health route logic

## 7. Quick map reference (selected files)
- /Users/lipeng/Downloads/AI/openfang-go/go.mod
- /Users/lipeng/Downloads/AI/openfang-go/cmd/openfang/main.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/kernel/kernel.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/config/config.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/api/server.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/memory/db.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/types/agent.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/process/manager.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/channels/adapters.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/eventbus/eventbus.go
- /Users/lipeng/Downloads/AI/openfang-go/cmd/openfang/commands/commands.go
- /Users/lipeng/Downloads/AI/openfang-go/internal/api/routes.go
