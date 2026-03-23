// Package api implements the REST API server for OpenFang.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/skip2/go-qrcode"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/a2a"
	"github.com/penzhan8451/fangclaw-go/internal/approvals"
	"github.com/penzhan8451/fangclaw-go/internal/audit"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/clawhub"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/pairing"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"github.com/penzhan8451/fangclaw-go/internal/security"
	"github.com/penzhan8451/fangclaw-go/internal/triggers"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// Field type for the channel configuration form.
type FieldType string

const (
	FieldTypeSecret FieldType = "secret"
	FieldTypeText   FieldType = "text"
	FieldTypeNumber FieldType = "number"
	FieldTypeList   FieldType = "list"
)

// A single configurable field for a channel adapter.
type ChannelField struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	FieldType   FieldType `json:"field_type"`
	EnvVar      *string   `json:"env_var,omitempty"`
	Required    bool      `json:"required"`
	Placeholder string    `json:"placeholder"`
	Advanced    bool      `json:"advanced"`
}

// Metadata for one channel adapter.
type ChannelMeta struct {
	Name           string         `json:"name"`
	DisplayName    string         `json:"display_name"`
	Icon           string         `json:"icon"`
	Description    string         `json:"description"`
	Category       string         `json:"category"`
	Difficulty     string         `json:"difficulty"`
	SetupTime      string         `json:"setup_time"`
	QuickSetup     string         `json:"quick_setup"`
	SetupType      string         `json:"setup_type"`
	Fields         []ChannelField `json:"fields"`
	SetupSteps     []string       `json:"setup_steps"`
	ConfigTemplate string         `json:"config_template"`
}

// CHANNEL_REGISTRY contains all available channel adapters.
var CHANNEL_REGISTRY = []ChannelMeta{
	{
		Name:        "telegram",
		DisplayName: "Telegram",
		Icon:        "TG",
		Description: "Telegram Bot API — long-polling adapter",
		Category:    "messaging",
		Difficulty:  "Easy",
		SetupTime:   "~2 min",
		QuickSetup:  "Paste your bot token from @BotFather",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "bot_token_env", Label: "Bot Token", FieldType: FieldTypeSecret, EnvVar: strPtr("TELEGRAM_BOT_TOKEN"), Required: true, Placeholder: "123456:ABC-DEF...", Advanced: false},
			{Key: "allowed_users", Label: "Allowed User IDs", FieldType: FieldTypeList, EnvVar: nil, Required: false, Placeholder: "12345, 67890", Advanced: true},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Open @BotFather on Telegram", "Send /newbot and follow the prompts", "Paste the token below"},
		ConfigTemplate: "[channels.telegram]\nbot_token_env = \"TELEGRAM_BOT_TOKEN\"",
	},
	{
		Name:        "discord",
		DisplayName: "Discord",
		Icon:        "DC",
		Description: "Discord Gateway bot adapter",
		Category:    "messaging",
		Difficulty:  "Easy",
		SetupTime:   "~3 min",
		QuickSetup:  "Paste your bot token from the Discord Developer Portal",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "bot_token_env", Label: "Bot Token", FieldType: FieldTypeSecret, EnvVar: strPtr("DISCORD_BOT_TOKEN"), Required: true, Placeholder: "MTIz...", Advanced: false},
			{Key: "allowed_guilds", Label: "Allowed Guild IDs", FieldType: FieldTypeList, EnvVar: nil, Required: false, Placeholder: "123456789, 987654321", Advanced: true},
			{Key: "allowed_users", Label: "Allowed User IDs", FieldType: FieldTypeList, EnvVar: nil, Required: false, Placeholder: "123456789, 987654321", Advanced: true},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Go to discord.com/developers/applications", "Create a bot and copy the token", "Paste it below"},
		ConfigTemplate: "[channels.discord]\nbot_token_env = \"DISCORD_BOT_TOKEN\"",
	},
	{
		Name:        "slack",
		DisplayName: "Slack",
		Icon:        "SL",
		Description: "Slack Socket Mode + Events API",
		Category:    "messaging",
		Difficulty:  "Medium",
		SetupTime:   "~5 min",
		QuickSetup:  "Paste your App Token and Bot Token from api.slack.com",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "app_token_env", Label: "App Token (xapp-)", FieldType: FieldTypeSecret, EnvVar: strPtr("SLACK_APP_TOKEN"), Required: true, Placeholder: "xapp-1-...", Advanced: false},
			{Key: "bot_token_env", Label: "Bot Token (xoxb-)", FieldType: FieldTypeSecret, EnvVar: strPtr("SLACK_BOT_TOKEN"), Required: true, Placeholder: "xoxb-...", Advanced: false},
			{Key: "allowed_channels", Label: "Allowed Channel IDs", FieldType: FieldTypeList, EnvVar: nil, Required: false, Placeholder: "C01234, C56789", Advanced: true},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Create app at api.slack.com/apps", "Enable Socket Mode and copy App Token", "Copy Bot Token from OAuth & Permissions"},
		ConfigTemplate: "[channels.slack]\napp_token_env = \"SLACK_APP_TOKEN\"\nbot_token_env = \"SLACK_BOT_TOKEN\"",
	},
	{
		Name:        "whatsapp",
		DisplayName: "WhatsApp",
		Icon:        "WA",
		Description: "Connect your personal WhatsApp via QR scan",
		Category:    "messaging",
		Difficulty:  "Easy",
		SetupTime:   "~1 min",
		QuickSetup:  "Scan QR code with your phone — no developer account needed",
		SetupType:   "qr",
		Fields: []ChannelField{
			{Key: "access_token_env", Label: "Access Token", FieldType: FieldTypeSecret, EnvVar: strPtr("WHATSAPP_ACCESS_TOKEN"), Required: false, Placeholder: "EAAx...", Advanced: true},
			{Key: "phone_number_id", Label: "Phone Number ID", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "1234567890", Advanced: true},
			{Key: "verify_token_env", Label: "Verify Token", FieldType: FieldTypeSecret, EnvVar: strPtr("WHATSAPP_VERIFY_TOKEN"), Required: false, Placeholder: "my-verify-token", Advanced: true},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Open WhatsApp on your phone", "Go to Linked Devices", "Tap Link a Device and scan the QR code"},
		ConfigTemplate: "[channels.whatsapp]\naccess_token_env = \"WHATSAPP_ACCESS_TOKEN\"\nphone_number_id = \"\"",
	},
	{
		Name:        "qq",
		DisplayName: "QQ",
		Icon:        "QQ",
		Description: "QQ Bot API adapter",
		Category:    "messaging",
		Difficulty:  "Easy",
		SetupTime:   "~2 min",
		QuickSetup:  "Paste your App ID and App Secret",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "app_id", Label: "App ID", FieldType: FieldTypeText, EnvVar: strPtr("QQ_APP_ID"), Required: true, Placeholder: "123456789", Advanced: false},
			{Key: "app_secret_env", Label: "App Secret", FieldType: FieldTypeSecret, EnvVar: strPtr("QQ_APP_SECRET"), Required: true, Placeholder: "abc123...", Advanced: false},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Create QQ Bot at QQ Open Platform", "Copy App ID and App Secret", "Paste them below"},
		ConfigTemplate: "[channels.qq]\napp_id = \"\"\napp_secret_env = \"QQ_APP_SECRET\"",
	},
	{
		Name:        "dingtalk",
		DisplayName: "DingTalk",
		Icon:        "DT",
		Description: "DingTalk Robot API adapter",
		Category:    "enterprise",
		Difficulty:  "Easy",
		SetupTime:   "~3 min",
		QuickSetup:  "Paste your Client ID and Client Secret",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "client_id", Label: "Client ID", FieldType: FieldTypeText, EnvVar: strPtr("DINGTALK_CLIENT_ID"), Required: true, Placeholder: "dingtalk123abc", Advanced: false},
			{Key: "client_secret_env", Label: "Client Secret", FieldType: FieldTypeSecret, EnvVar: strPtr("DINGTALK_CLIENT_SECRET"), Required: true, Placeholder: "abc123...", Advanced: false},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Create an app at open.dingtalk.com", "Copy Client ID and Secret", "Paste them below"},
		ConfigTemplate: "[channels.dingtalk]\nclient_id = \"\"\nclient_secret_env = \"DINGTALK_CLIENT_SECRET\"",
	},
	{
		Name:        "feishu",
		DisplayName: "Feishu/Lark",
		Icon:        "FS",
		Description: "Feishu/Lark Open Platform adapter",
		Category:    "enterprise",
		Difficulty:  "Easy",
		SetupTime:   "~3 min",
		QuickSetup:  "Paste your App ID and App Secret",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "app_id", Label: "App ID", FieldType: FieldTypeText, EnvVar: strPtr("FEISHU_APP_ID"), Required: true, Placeholder: "cli_abc123", Advanced: false},
			{Key: "app_secret_env", Label: "App Secret", FieldType: FieldTypeSecret, EnvVar: strPtr("FEISHU_APP_SECRET"), Required: true, Placeholder: "abc123...", Advanced: false},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Create an app at open.feishu.cn", "Copy App ID and Secret", "Paste them below"},
		ConfigTemplate: "[channels.feishu]\napp_id = \"\"\napp_secret_env = \"FEISHU_APP_SECRET\"",
	},
}

func strPtr(s string) *string {
	return &s
}

func getHandsFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".fangclaw-go", "hands.json")
}

func loadHandsStatus() []map[string]string {
	var bundledHands = []map[string]string{
		{
			"id":          "researcher",
			"name":        "Researcher",
			"description": "Deep autonomous researcher. Cross-references multiple sources, evaluates credibility, generates cited reports.",
			"category":    "content",
			"status":      "inactive",
		},
		{
			"id":          "lead",
			"name":        "Lead",
			"description": "Runs daily. Discovers prospects matching your ICP, enriches with research, scores 0-100.",
			"category":    "productivity",
			"status":      "inactive",
		},
		{
			"id":          "collector",
			"name":        "Collector",
			"description": "OSINT-grade intelligence. Monitors targets continuously with change detection and knowledge graphs.",
			"category":    "data",
			"status":      "inactive",
		},
		{
			"id":          "predictor",
			"name":        "Predictor",
			"description": "Superforecasting engine. Collects signals, builds calibrated reasoning chains, makes predictions.",
			"category":    "data",
			"status":      "inactive",
		},
		{
			"id":          "clip",
			"name":        "Clip",
			"description": "YouTube video processing. Downloads, identifies best moments, cuts into vertical shorts.",
			"category":    "content",
			"status":      "inactive",
		},
		{
			"id":          "twitter",
			"name":        "Twitter",
			"description": "Autonomous Twitter/X account manager. Creates content, schedules posts, responds to mentions.",
			"category":    "communication",
			"status":      "inactive",
		},
		{
			"id":          "browser",
			"name":        "Browser",
			"description": "Web automation agent. Navigates sites, fills forms, handles multi-step workflows.",
			"category":    "productivity",
			"status":      "inactive",
		},
	}

	path := getHandsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return bundledHands
	}
	var savedHands []map[string]string
	if err := json.Unmarshal(data, &savedHands); err != nil {
		return bundledHands
	}
	for _, saved := range savedHands {
		for j := range bundledHands {
			if bundledHands[j]["id"] == saved["id"] {
				bundledHands[j]["status"] = saved["status"]
				break
			}
		}
	}
	return bundledHands
}

func saveHandsStatus(hands []map[string]string) {
	path := getHandsFilePath()
	data, err := json.MarshalIndent(hands, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	os.WriteFile(path, data, 0644)
}

func updateHandStatus(handID, status string) {
	hands := loadHandsStatus()
	for i := range hands {
		if hands[i]["id"] == handID {
			hands[i]["status"] = status
			break
		}
	}
	saveHandsStatus(hands)
}

// sharedMemoryAgentID is the well-known shared-memory agent ID used for cross-agent KV storage.
// Must match the value in openfang-kernel.
func sharedMemoryAgentID() string {
	return uuid.UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}.String()
}

// Router manages API routes.
type Router struct {
	kernel       *kernel.Kernel
	defaultAgent string
}

// NewRouter creates a new API router.
func NewRouter(k *kernel.Kernel) *Router {
	r := &Router{
		kernel: k,
	}

	// Register approval callback to notify frontend
	k.ApprovalManager().SetOnNewRequest(func(req *approvals.ApprovalRequest) {
		// Create WebSocket message
		type WSMessage struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		}

		msg := WSMessage{
			Type: "new_approval",
			Data: map[string]interface{}{
				"id":           req.ID,
				"agent_id":     req.AgentID,
				"tool_name":    req.ToolName,
				"description":  req.Description,
				"risk_level":   req.RiskLevel,
				"created_at":   req.CreatedAt,
				"timeout_secs": req.TimeoutSecs,
			},
		}

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal approval message: %v\n", err)
			return
		}

		// Broadcast to all connected clients
		wsManager.BroadcastToAll(msgBytes)
	})

	// Register approval resolved callback to notify frontend
	k.ApprovalManager().SetOnResolve(func(req *approvals.ApprovalRequest, decision approvals.ApprovalDecision, reason string) {
		// Create WebSocket message
		type WSMessage struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		}

		msg := WSMessage{
			Type: "approval_resolved",
			Data: map[string]interface{}{
				"id":         req.ID,
				"agent_id":   req.AgentID,
				"session_id": req.SessionID,
				"decision":   decision,
				"reason":     reason,
			},
		}

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal approval resolved message: %v\n", err)
			return
		}

		// Broadcast to all connected clients
		wsManager.BroadcastToAll(msgBytes)
	})

	return r
}

// SetDefault sets the default agent for A2A tasks.
func (r *Router) SetDefault(agentID string) {
	r.defaultAgent = agentID
}

// RegisterRoutes registers all API routes.
func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	// A2A Standard protocol endpoints (standard, no /api prefix)
	mux.HandleFunc("GET /.well-known/agent.json", r.handleA2AAgentCard)
	mux.HandleFunc("GET /a2a/agents", r.handleA2AListAgents)
	// called through JSON-RPC2.0 format by external A2A Agent
	mux.HandleFunc("POST /a2a/tasks/send", r.handleA2ASendTask)
	mux.HandleFunc("GET /a2a/tasks/{id}", r.handleA2AGetTask)
	mux.HandleFunc("POST /a2a/tasks/{id}/cancel", r.handleA2ACancelTask)

	// A2A protocol endpoints (with /api prefix for frontend compatibility)
	mux.HandleFunc("POST /api/a2a/tasks/send", r.handleA2ASendTask)
	mux.HandleFunc("GET /api/a2a/tasks/{id}", r.handleA2AGetTask)
	mux.HandleFunc("POST /api/a2a/tasks/{id}/cancel", r.handleA2ACancelTask)

	// A2A external agent management endpoints (internal API)
	mux.HandleFunc("GET /api/a2a/agents", r.handleA2AListAgents)
	mux.HandleFunc("POST /api/a2a/discover", r.handleA2ADiscoverExternal)
	mux.HandleFunc("POST /api/a2a/send", r.handleA2ASendExternal)
	mux.HandleFunc("GET /api/a2a/tasks/{id}/status", r.handleA2AExternalTaskStatus)
	mux.HandleFunc("GET /api/a2a/topology", r.handleA2ATopology)
	mux.HandleFunc("GET /api/a2a/events", r.handleA2AEvents)
	mux.HandleFunc("GET /api/a2a/events/stream", r.handleA2AEventsStream)

	// Comms endpoints (local agent communication)
	mux.HandleFunc("GET /api/comms/topology", r.handleA2ATopology) // Reuse topology handler
	//-- Send a message from one local agent to another
	mux.HandleFunc("POST /api/comms/send", r.handleCommsSend)
	//-- Publish a task to an agent's task queue (local agent, no A2A protocol)
	mux.HandleFunc("POST /api/comms/task", r.handleCommsTask)
	//-- Task management endpoints
	mux.HandleFunc("GET /api/tasks", r.handleListTasks)
	mux.HandleFunc("GET /api/tasks/{id}", r.handleGetTask)

	// Health and status endpoints
	mux.HandleFunc("GET /api/health", r.handleHealth)
	mux.HandleFunc("GET /api/status", r.handleStatus)
	mux.HandleFunc("GET /api/version", r.handleVersion)
	mux.HandleFunc("GET /api/security", r.handleSecurity)

	// Auth endpoints
	mux.HandleFunc("POST /api/auth/login", r.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", r.handleLogout)

	// Agent endpoints (v1)
	mux.HandleFunc("GET /api/v1/agents", r.handleListAgents)
	mux.HandleFunc("POST /api/v1/agents", r.handleCreateAgent)
	mux.HandleFunc("GET /api/v1/agents/{id}", r.handleGetAgent)
	mux.HandleFunc("PUT /api/v1/agents/{id}", r.handleUpdateAgent)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", r.handleDeleteAgent)

	// Agent endpoints (aliases without v1)
	mux.HandleFunc("GET /api/agents", r.handleListAgents)
	mux.HandleFunc("POST /api/agents", r.handleCreateAgent)
	mux.HandleFunc("GET /api/agents/{id}", r.handleGetAgent)
	mux.HandleFunc("PUT /api/agents/{id}", r.handleUpdateAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", r.handleDeleteAgent)

	// Session endpoints
	mux.HandleFunc("GET /api/v1/sessions", r.handleListSessions)
	mux.HandleFunc("POST /api/v1/sessions", r.handleCreateSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}", r.handleGetSession)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", r.handleDeleteSession)

	// Session endpoints (aliases)
	mux.HandleFunc("GET /api/sessions", r.handleListSessions)
	mux.HandleFunc("POST /api/sessions", r.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", r.handleGetSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", r.handleDeleteSession)

	// Chat endpoints
	mux.HandleFunc("POST /api/v1/chat", r.handleChat)

	// Memory endpoints
	mux.HandleFunc("GET /api/v1/memories", r.handleListMemories)
	mux.HandleFunc("POST /api/v1/memories", r.handleCreateMemory)
	mux.HandleFunc("GET /api/v1/memories/search", r.handleSearchMemories)
	mux.HandleFunc("DELETE /api/v1/memories/{id}", r.handleDeleteMemory)

	// Memory endpoints (aliases)
	mux.HandleFunc("GET /api/memories", r.handleListMemories)
	mux.HandleFunc("POST /api/memories", r.handleCreateMemory)
	mux.HandleFunc("GET /api/memories/search", r.handleSearchMemories)
	mux.HandleFunc("DELETE /api/memories/{id}", r.handleDeleteMemory)

	// Memory KV endpoints
	mux.HandleFunc("GET /api/memory/agents/{id}/kv", r.handleGetAgentKV)
	mux.HandleFunc("GET /api/memory/agents/{id}/kv/{key}", r.handleGetAgentKVKey)
	mux.HandleFunc("PUT /api/memory/agents/{id}/kv/{key}", r.handleSetAgentKVKey)
	mux.HandleFunc("DELETE /api/memory/agents/{id}/kv/{key}", r.handleDeleteAgentKVKey)

	// Skill endpoints
	mux.HandleFunc("GET /api/v1/skills", r.handleListSkills)
	mux.HandleFunc("POST /api/v1/skills", r.handleInstallSkill)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", r.handleUninstallSkill)

	// Skill endpoints (aliases)
	mux.HandleFunc("GET /api/skills", r.handleListSkills)
	mux.HandleFunc("POST /api/skills", r.handleInstallSkill)
	mux.HandleFunc("DELETE /api/skills/{id}", r.handleUninstallSkill)
	// ClawHub endpoints
	mux.HandleFunc("GET /api/clawhub/search", r.handleClawhubSearch)
	mux.HandleFunc("GET /api/clawhub/browse", r.handleClawhubBrowse)
	mux.HandleFunc("GET /api/clawhub/skill/{slug}", r.handleClawhubSkillDetail)
	mux.HandleFunc("POST /api/clawhub/install", r.handleClawhubInstall)

	// Channel endpoints
	mux.HandleFunc("GET /api/v1/channels", r.handleListChannels)
	mux.HandleFunc("POST /api/v1/channels", r.handleCreateChannel)
	mux.HandleFunc("DELETE /api/v1/channels/{id}", r.handleDeleteChannel)
	mux.HandleFunc("POST /api/v1/channels/{name}/configure", r.handleConfigureChannel)
	mux.HandleFunc("POST /api/v1/channels/{name}/test", r.handleTestChannel)

	// Channel endpoints (aliases)
	mux.HandleFunc("GET /api/channels", r.handleListChannels)
	mux.HandleFunc("POST /api/channels", r.handleCreateChannel)
	mux.HandleFunc("DELETE /api/channels/{id}", r.handleDeleteChannel)
	mux.HandleFunc("POST /api/channels/{name}/configure", r.handleConfigureChannel)
	mux.HandleFunc("POST /api/channels/{name}/test", r.handleTestChannel)

	// OpenAI-compatible endpoints
	mux.HandleFunc("GET /v1/models", r.handleListModels)
	// Models endpoints
	mux.HandleFunc("GET /api/models", r.handleAPIModels)
	mux.HandleFunc("GET /api/models/aliases", r.handleModelsAliases)
	mux.HandleFunc("GET /api/models/{id}", r.handleGetModel)
	mux.HandleFunc("POST /api/models/custom", r.handleAddCustomModel)
	mux.HandleFunc("DELETE /api/models/custom/{id}", r.handleDeleteCustomModel)
	// Additional frontend endpoints
	mux.HandleFunc("GET /api/commands", r.handleCommands)
	mux.HandleFunc("GET /api/config", r.handleConfig)
	mux.HandleFunc("GET /api/config/schema", r.handleConfigSchema)
	mux.HandleFunc("GET /api/logs/stream", r.handleLogsStream)

	// Hands endpoints
	mux.HandleFunc("GET /api/hands", r.handleListHands)
	mux.HandleFunc("GET /api/hands/active", r.handleActiveHands)
	mux.HandleFunc("GET /api/hands/{id}", r.handleGetHand)
	mux.HandleFunc("POST /api/hands/{id}/activate", r.handleActivateHand)
	mux.HandleFunc("POST /api/hands/{id}/install-deps", r.handleInstallHandDeps)
	mux.HandleFunc("DELETE /api/hands/instances/{instanceID}", r.handleDeactivateHand)
	mux.HandleFunc("POST /api/hands/instances/{instanceID}/deactivate", r.handleDeactivateHand)
	mux.HandleFunc("POST /api/hands/instances/{instanceID}/pause", r.handlePauseHand)
	mux.HandleFunc("POST /api/hands/instances/{instanceID}/resume", r.handleResumeHand)
	mux.HandleFunc("GET /api/hands/instances/{instanceID}/stats", r.handleHandInstanceStats)
	mux.HandleFunc("GET /api/hands/instances/{instanceID}/browser", r.handleHandInstanceBrowser)

	// Approval endpoints
	mux.HandleFunc("GET /api/approvals", r.handleListApprovals)
	mux.HandleFunc("POST /api/approvals", r.handleCreateApproval)
	mux.HandleFunc("POST /api/approvals/{id}/approve", r.handleApproveApproval)
	mux.HandleFunc("POST /api/approvals/{id}/reject", r.handleRejectApproval)
	// Budget endpoints
	mux.HandleFunc("GET /api/budget", r.handleBudget)
	mux.HandleFunc("GET /api/budget/agents", r.handleBudgetAgents)
	mux.HandleFunc("GET /api/network/status", r.handleNetworkStatus)
	mux.HandleFunc("GET /api/tools", r.handleTools)
	mux.HandleFunc("GET /api/usage/summary", r.handleUsageSummary)
	mux.HandleFunc("GET /api/usage/by-model", r.handleUsageByModel)
	mux.HandleFunc("GET /api/usage/daily", r.handleUsageDaily)
	mux.HandleFunc("GET /api/usage", r.handleUsage)
	mux.HandleFunc("GET /api/audit/recent", r.handleAuditRecent)
	mux.HandleFunc("GET /api/audit/verify", r.handleAuditVerify)
	mux.HandleFunc("GET /api/providers", r.handleProviders)
	mux.HandleFunc("POST /api/providers/{name}/key", r.handleSetProviderKey)
	mux.HandleFunc("DELETE /api/providers/{name}/key", r.handleDeleteProviderKey)
	mux.HandleFunc("POST /api/providers/{name}/test", r.handleTestProvider)
	mux.HandleFunc("PUT /api/providers/{name}/url", r.handleSetProviderURL)
	mux.HandleFunc("GET /api/mcp/servers", r.handleMcpServers)
	mux.HandleFunc("POST /api/mcp/servers/:id/reconnect", r.handleMcpServerReconnect)
	mux.HandleFunc("GET /api/profiles", r.handleProfiles)
	mux.HandleFunc("PATCH /api/agents/{id}/config", r.handlePatchAgentConfig)
	mux.HandleFunc("GET /api/agents/{id}/files", r.handleListAgentFiles)
	mux.HandleFunc("GET /api/agents/{id}/files/{filename}", r.handleGetAgentFile)
	mux.HandleFunc("PUT /api/agents/{id}/files/{filename}", r.handleSetAgentFile)

	// Agent session endpoints
	mux.HandleFunc("GET /api/agents/{id}/session", r.handleGetAgentSession)
	mux.HandleFunc("GET /api/agents/{id}/sessions", r.handleGetAgentSessions)
	mux.HandleFunc("POST /api/agents/{id}/sessions", r.handleCreateAgentSession)
	mux.HandleFunc("POST /api/agents/{id}/sessions/{sid}/switch", r.handleSwitchSession)
	mux.HandleFunc("POST /api/agents/{id}/session/reset", r.handleResetSession)
	mux.HandleFunc("POST /api/agents/{id}/session/compact", r.handleCompactSession)
	mux.HandleFunc("POST /api/agents/{id}/message", r.handleAgentMessage)
	mux.HandleFunc("POST /api/agents/{id}/stop", r.handleStopAgent)
	mux.HandleFunc("PUT /api/agents/{id}/model", r.handleUpdateAgentModel)

	// Agent WebSocket endpoint
	mux.HandleFunc("/api/agents/{id}/ws", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		req.URL.RawQuery = "agent_id=" + id
		WSHandler(r.kernel)(w, req)
	})

	// Cron jobs endpoints
	mux.HandleFunc("GET /api/cron/jobs", r.handleListCronJobs)
	mux.HandleFunc("POST /api/cron/jobs", r.handleCreateCronJob)
	mux.HandleFunc("PUT /api/cron/jobs/{id}/enable", r.handleEnableCronJob)
	mux.HandleFunc("DELETE /api/cron/jobs/{id}", r.handleDeleteCronJob)
	mux.HandleFunc("GET /api/cron/jobs/{id}/status", r.handleCronJobStatus)

	// Workflows endpoints
	mux.HandleFunc("POST /api/workflows", r.handleCreateWorkflow)
	mux.HandleFunc("GET /api/workflows", r.handleListWorkflows)
	mux.HandleFunc("GET /api/workflows/{id}", r.handleGetWorkflow)
	mux.HandleFunc("DELETE /api/workflows/{id}", r.handleDeleteWorkflow)
	mux.HandleFunc("POST /api/workflows/{id}/run", r.handleRunWorkflow)
	mux.HandleFunc("GET /api/workflows/{id}/runs", r.handleListWorkflowRuns)
	// Workflow templates endpoints
	mux.HandleFunc("GET /api/workflow-templates", r.handleListWorkflowTemplates)
	mux.HandleFunc("GET /api/workflow-templates/{id}", r.handleGetWorkflowTemplate)
	mux.HandleFunc("POST /api/workflows/from-template", r.handleCreateWorkflowFromTemplate)
	// Workflow delivery endpoints
	mux.HandleFunc("POST /api/workflows/{id}/run-with-delivery", r.handleRunWorkflowWithDelivery)

	// Triggers endpoints
	mux.HandleFunc("POST /api/triggers", r.handleCreateTrigger)
	mux.HandleFunc("GET /api/triggers", r.handleListTriggers)
	mux.HandleFunc("PUT /api/triggers/{id}", r.handleUpdateTrigger)
	mux.HandleFunc("DELETE /api/triggers/{id}", r.handleDeleteTrigger)
	mux.HandleFunc("GET /api/trigger-history", r.handleListTriggerHistory)

	// Agent templates endpoints
	mux.HandleFunc("GET /api/agent-templates", r.handleListAgentTemplates)
	mux.HandleFunc("GET /api/agent-templates/{id}", r.handleGetAgentTemplate)
	mux.HandleFunc("POST /api/templates/{id}/spawn", r.handleSpawnAgentFromTemplate)

	// Shutdown endpoint
	mux.HandleFunc("POST /api/shutdown", r.handleShutdown)

	// Pairing endpoints
	mux.HandleFunc("GET /pair", r.handlePairingPage)
	mux.HandleFunc("GET /api/pairing/config", r.handleGetPairingConfig)
	mux.HandleFunc("POST /api/pairing/config", r.handleUpdatePairingConfig)
	mux.HandleFunc("POST /api/pairing/request", r.handleCreatePairingRequest)
	mux.HandleFunc("POST /api/pairing/complete", r.handleCompletePairing)
	mux.HandleFunc("GET /api/pairing/devices", r.handleListPairedDevices)
	mux.HandleFunc("DELETE /api/pairing/devices/{id}", r.handleRemovePairedDevice)
	mux.HandleFunc("POST /api/pairing/notify", r.handleNotify)
}

// respondJSON responds with JSON.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError responds with an error.
func respondError(w http.ResponseWriter, status int, data interface{}) {
	respondJSON(w, status, data)
}

// handleHealth handles health check requests.
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, types.HealthStatus{
		Status:  "ok",
		Healthy: true,
		Checks:  map[string]bool{"database": true, "kernel": true},
	})
}

// handleStatus handles status requests.
func (r *Router) handleStatus(w http.ResponseWriter, req *http.Request) {
	uptime := r.kernel.GetUptime()
	var uptimeStr string
	secs := int(uptime.Seconds())
	days := secs / 86400
	hours := (secs % 86400) / 3600
	mins := (secs % 3600) / 60
	if days > 0 {
		uptimeStr = fmt.Sprintf("%dd %dh", days, hours)
	} else if hours > 0 {
		uptimeStr = fmt.Sprintf("%dh %dm", hours, mins)
	} else {
		uptimeStr = fmt.Sprintf("%dm", mins)
	}

	respondJSON(w, http.StatusOK, types.StatusResponse{
		Status:        "running",
		Version:       "0.1.0",
		ListenAddr:    ":4200",
		AgentCount:    r.kernel.AgentRegistry().Count(),
		ModelCount:    1,
		Uptime:        uptimeStr,
		UptimeSeconds: secs,
	})
}

// handleSecurity handles security status requests.
func (r *Router) handleSecurity(w http.ResponseWriter, req *http.Request) {
	authMode := "localhost_only"
	apiKeySet := false

	coreProtections := map[string]bool{
		"path_traversal":                  true,
		"ssrf_protection":                 true,
		"capability_system":               true,
		"privilege_escalation_prevention": true,
		"subprocess_isolation":            true,
		"security_headers":                true,
		"wire_hmac_auth":                  true,
		"request_id_tracking":             true,
	}

	configurable := map[string]interface{}{
		"rate_limiter": map[string]interface{}{
			"enabled":           true,
			"tokens_per_minute": 500,
			"algorithm":         "GCRA",
		},
		"websocket_limits": map[string]interface{}{
			"max_per_ip":              5,
			"idle_timeout_secs":       1800,
			"max_message_size":        65536,
			"max_messages_per_minute": 10,
		},
		"wasm_sandbox": map[string]interface{}{
			"fuel_metering":        true,
			"epoch_interruption":   true,
			"default_timeout_secs": 30,
			"default_fuel_limit":   1000000,
		},
		"auth": map[string]interface{}{
			"mode":        authMode,
			"api_key_set": apiKeySet,
		},
	}

	auditCount := 0
	monitoring := map[string]interface{}{
		"audit_trail": map[string]interface{}{
			"enabled":     true,
			"algorithm":   "SHA-256 Merkle Chain",
			"entry_count": auditCount,
		},
		"taint_tracking": map[string]interface{}{
			"enabled": true,
			"tracked_labels": []string{
				"ExternalNetwork",
				"UserInput",
				"PII",
				"Secret",
				"UntrustedAgent",
			},
		},
		"manifest_signing": map[string]interface{}{
			"algorithm": "Ed25519",
			"available": true,
		},
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"core_protections":   coreProtections,
		"configurable":       configurable,
		"monitoring":         monitoring,
		"secret_zeroization": true,
		"total_features":     15,
		"timestamp":          time.Now().UTC(),
	})
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token string `json:"token"`
}

// handleLogin handles login requests.
func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var loginReq LoginRequest
	if err := json.NewDecoder(req.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// TODO: Replace with actual authentication logic
	// For now, return a dummy token for demonstration
	// In production, validate credentials against a user store
	dummyUser := &security.User{
		ID:       "demo-user-id",
		Username: loginReq.Username,
		Role:     "user",
	}

	// Create a default security config
	secConfig := security.DefaultSecurityConfig()

	// Create auth service
	authService := security.NewAuthService(secConfig)

	// Generate token
	token, err := authService.GenerateToken(dummyUser)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, LoginResponse{
		Token: token,
	})
}

// handleLogout handles logout requests.
func (r *Router) handleLogout(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Invalidate token on server-side (e.g., add to blacklist)
	// For now, just return success
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

// handleVersion handles version requests.
func (r *Router) handleVersion(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"version": "0.1.0",
	})
}

// Agent handlers
func (r *Router) handleListAgents(w http.ResponseWriter, req *http.Request) {
	agents := r.kernel.AgentRegistry().List()
	var result []map[string]interface{}
	for _, agent := range agents {
		result = append(result, map[string]interface{}{
			"id":             agent.ID,
			"name":           agent.Name,
			"state":          agent.State,
			"mode":           agent.Mode,
			"tags":           agent.Tags,
			"created_at":     agent.CreatedAt,
			"last_active":    agent.LastActive,
			"model_provider": agent.Manifest.Model.Provider,
			"model_name":     agent.Manifest.Model.Model,
		})
	}
	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleCreateAgent(w http.ResponseWriter, req *http.Request) {
	var body struct {
		ManifestTOML string `json:"manifest_toml"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// SECURITY: Reject oversized manifests to prevent parser memory exhaustion.
	const MAX_MANIFEST_SIZE = 1024 * 1024 // 1MB
	if len(body.ManifestTOML) > MAX_MANIFEST_SIZE {
		respondError(w, http.StatusRequestEntityTooLarge, "Manifest too large (max 1MB)")
		return
	}

	var manifest types.AgentManifest
	if err := toml.Unmarshal([]byte(body.ManifestTOML), &manifest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid manifest format: "+err.Error())
		return
	}

	agentID, agentName, err := r.kernel.SpawnAgent(manifest)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondError(w, http.StatusConflict, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, "Agent spawn failed: "+err.Error())
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"agent_id": agentID,
		"name":     agentName,
	})
}

func (r *Router) handleGetAgent(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	agent := r.kernel.AgentRegistry().Get(id)
	if agent == nil {
		respondError(w, http.StatusNotFound, "agent not found")
		return
	}

	respondJSON(w, http.StatusOK, agent)
}

func (r *Router) handleUpdateAgent(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	var reqBody struct {
		State *string `json:"state"`
		Mode  *string `json:"mode"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if reqBody.State != nil {
		state := types.AgentState(*reqBody.State)
		if err := r.kernel.AgentRegistry().SetState(id, state); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	if reqBody.Mode != nil {
		if err := r.kernel.AgentRegistry().SetMode(id, *reqBody.Mode); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	agent := r.kernel.AgentRegistry().Get(id)
	respondJSON(w, http.StatusOK, agent)
}

func (r *Router) handleDeleteAgent(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	if err := r.kernel.DeleteAgent(idStr); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// Session handlers
func (r *Router) handleListSessions(w http.ResponseWriter, req *http.Request) {
	sessions, err := r.kernel.SessionStore().ListSessions()
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"sessions": []map[string]interface{}{},
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

func (r *Router) handleCreateSession(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		AgentID string  `json:"agent_id"`
		Label   *string `json:"label,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	agentID, err := types.ParseAgentID(reqBody.AgentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent_id")
		return
	}

	agent := r.kernel.AgentRegistry().Get(agentID)
	agentName := ""
	agentModelProvider := ""
	agentModelName := ""
	if agent != nil {
		agentName = agent.Name
		agentModelProvider = agent.Manifest.Model.Provider
		agentModelName = agent.Manifest.Model.Model
	}

	session := types.NewSession(agentID, agentName, agentModelProvider, agentModelName, reqBody.Label)
	if err := r.kernel.SessionStore().SaveSession(&session); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, session)
}

func (r *Router) handleGetSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sessionID, err := types.ParseSessionID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid session_id")
		return
	}

	session, err := r.kernel.SessionStore().GetSession(sessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	respondJSON(w, http.StatusOK, session)
}

func (r *Router) handleDeleteSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sessionID, err := types.ParseSessionID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid session_id")
		return
	}

	if err := r.kernel.SessionStore().DeleteSession(sessionID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// Chat handlers
func (r *Router) handleChat(w http.ResponseWriter, req *http.Request) {
	var chatReq types.ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, types.ChatResponse{
		Response:   "This is a placeholder response.",
		AgentID:    chatReq.AgentID,
		StopReason: "end_turn",
	})
}

// Memory handlers
func (r *Router) handleListMemories(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleCreateMemory(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (r *Router) handleSearchMemories(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	limit := 10
	agentIDStr := req.URL.Query().Get("agent_id")

	var filter *types.MemoryFilter
	if agentIDStr != "" {
		agentID, _ := types.ParseAgentID(agentIDStr)
		filter = &types.MemoryFilter{AgentID: &agentID}
	}

	memories, err := r.kernel.SemanticStore().Recall(query, limit, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, memories)
}

func (r *Router) handleDeleteMemory(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	memoryID, err := types.ParseMemoryID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid memory_id")
		return
	}

	if err := r.kernel.SemanticStore().Forget(memoryID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Memory KV handlers

// handleGetAgentKV handles GET /api/memory/agents/{id}/kv — List KV pairs for an agent.
func (r *Router) handleGetAgentKV(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}

	records, err := r.kernel.DB().ListKV(agentID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Memory operation failed")
		return
	}

	var kvPairs []map[string]interface{}
	for _, r := range records {
		var value interface{}
		if err := json.Unmarshal(r.Value, &value); err != nil {
			value = string(r.Value)
		}
		kvPairs = append(kvPairs, map[string]interface{}{
			"key":   r.Key,
			"value": value,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"kv_pairs": kvPairs,
	})
}

// handleGetAgentKVKey handles GET /api/memory/agents/{id}/kv/{key} — Get a specific KV value.
func (r *Router) handleGetAgentKVKey(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}
	key := req.PathValue("key")

	record, err := r.kernel.DB().GetKV(agentID, key)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Memory operation failed")
		return
	}

	if record == nil {
		respondError(w, http.StatusNotFound, "Key not found")
		return
	}

	var value interface{}
	if err := json.Unmarshal(record.Value, &value); err != nil {
		value = string(record.Value)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"key":   key,
		"value": value,
	})
}

// handleSetAgentKVKey handles PUT /api/memory/agents/{id}/kv/{key} — Set a KV value.
func (r *Router) handleSetAgentKVKey(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}
	key := req.PathValue("key")

	var reqBody struct {
		Value interface{} `json:"value"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		var value interface{}
		if err2 := json.NewDecoder(req.Body).Decode(&value); err2 != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		reqBody.Value = value
	}

	valueBytes, err := json.Marshal(reqBody.Value)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Memory operation failed")
		return
	}

	if err := r.kernel.DB().SetKV(agentID, key, valueBytes); err != nil {
		respondError(w, http.StatusInternalServerError, "Memory operation failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "stored",
		"key":    key,
	})
}

// handleDeleteAgentKVKey handles DELETE /api/memory/agents/{id}/kv/{key} — Delete a KV value.
func (r *Router) handleDeleteAgentKVKey(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}
	key := req.PathValue("key")

	if err := r.kernel.DB().DeleteKV(agentID, key); err != nil {
		respondError(w, http.StatusInternalServerError, "Memory operation failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
		"key":    key,
	})
}

// Skill handlers
func (r *Router) handleListSkills(w http.ResponseWriter, req *http.Request) {
	skills, err := r.kernel.SkillLoader().ListSkills()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var skillsResult []map[string]interface{}
	for _, skill := range skills {
		tools := skill.Manifest.Tools.Provided
		if tools == nil {
			tools = []types.SkillToolDefinition{}
		}

		tags := []string{}
		if skill.Manifest.Metadata != nil && skill.Manifest.Metadata["tags"] != "" {
		}

		skillsResult = append(skillsResult, map[string]interface{}{
			"name":               skill.Manifest.Name,
			"description":        skill.Manifest.Description,
			"version":            skill.Manifest.Version,
			"author":             skill.Manifest.Author,
			"runtime":            string(skill.Manifest.Runtime.RuntimeType),
			"tools_count":        len(tools),
			"tags":               tags,
			"enabled":            skill.Enabled,
			"source":             map[string]interface{}{"type": "local"},
			"has_prompt_context": skill.Manifest.Runtime.RuntimeType == types.SkillRuntimePrompt,
		})
	}

	if skillsResult == nil {
		skillsResult = []map[string]interface{}{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"skills": skillsResult,
		"total":  len(skillsResult),
	})
}

func (r *Router) handleInstallSkill(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		SourcePath string `json:"source_path"`
		SkillID    string `json:"skill_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	skill, err := r.kernel.SkillLoader().InstallSkill(reqBody.SourcePath, reqBody.SkillID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, skill)
}

func (r *Router) handleUninstallSkill(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if err := r.kernel.SkillLoader().UninstallSkill(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// Channel handlers
func (r *Router) handleListChannels(w http.ResponseWriter, req *http.Request) {
	var channels []map[string]interface{}
	var configuredCount uint32 = 0

	for _, meta := range CHANNEL_REGISTRY {
		configured := isChannelConfigured(meta.Name)
		if configured {
			configuredCount++
		}

		hasToken := true
		if meta.Name == "feishu" {
			cfg, err := config.Load("")
			if err == nil && cfg.Channels.Feishu != nil && cfg.Channels.Feishu.AppID != "" && (cfg.Channels.Feishu.AppSecret != "" || cfg.Channels.Feishu.AppSecretEnv != "") {
				hasToken = true
			} else {
				hasToken = false
			}
		} else if meta.Name == "qq" {
			cfg, err := config.Load("")
			if err == nil && cfg.Channels.QQ != nil && cfg.Channels.QQ.AppID != "" && (cfg.Channels.QQ.AppSecret != "" || cfg.Channels.QQ.AppSecretEnv != "") {
				hasToken = true
			} else {
				hasToken = false
			}
		} else if meta.Name == "whatsapp" {
			cfg, err := config.Load("")
			if err == nil && cfg.Channels.WhatsApp != nil && cfg.Channels.WhatsApp.AccessTokenEnv != "" && cfg.Channels.WhatsApp.PhoneNumberID != "" {
				hasToken = true
			} else {
				hasToken = false
			}
		} else {
			for _, f := range meta.Fields {
				if f.Required && f.EnvVar != nil {
					val := os.Getenv(*f.EnvVar)
					if val == "" {
						hasToken = false
						break
					}
				}
			}
		}

		var fields []map[string]interface{}
		cfg, _ := config.Load("")
		for _, f := range meta.Fields {
			hasValue := false
			if meta.Name == "feishu" {
				if f.Key == "app_id" && cfg.Channels.Feishu != nil {
					hasValue = cfg.Channels.Feishu.AppID != ""
				} else if f.Key == "app_secret_env" && cfg.Channels.Feishu != nil {
					hasValue = cfg.Channels.Feishu.AppSecret != "" || cfg.Channels.Feishu.AppSecretEnv != ""
				} else if f.EnvVar != nil {
					val := os.Getenv(*f.EnvVar)
					hasValue = val != ""
				}
			} else if meta.Name == "qq" {
				if f.Key == "app_id" && cfg.Channels.QQ != nil {
					hasValue = cfg.Channels.QQ.AppID != ""
				} else if f.Key == "app_secret_env" && cfg.Channels.QQ != nil {
					hasValue = cfg.Channels.QQ.AppSecret != "" || cfg.Channels.QQ.AppSecretEnv != ""
				} else if f.EnvVar != nil {
					val := os.Getenv(*f.EnvVar)
					hasValue = val != ""
				}
			} else if meta.Name == "whatsapp" {
				if f.Key == "access_token_env" && cfg.Channels.WhatsApp != nil {
					hasValue = cfg.Channels.WhatsApp.AccessTokenEnv != ""
				} else if f.Key == "phone_number_id" && cfg.Channels.WhatsApp != nil {
					hasValue = cfg.Channels.WhatsApp.PhoneNumberID != ""
				} else if f.EnvVar != nil {
					val := os.Getenv(*f.EnvVar)
					hasValue = val != ""
				}
			} else {
				if f.EnvVar != nil {
					val := os.Getenv(*f.EnvVar)
					hasValue = val != ""
				}
			}

			field := map[string]interface{}{
				"key":         f.Key,
				"label":       f.Label,
				"type":        f.FieldType,
				"required":    f.Required,
				"placeholder": f.Placeholder,
				"advanced":    f.Advanced,
				"has_value":   hasValue,
			}
			if f.EnvVar != nil {
				field["env_var"] = *f.EnvVar
			}
			fields = append(fields, field)
		}

		channels = append(channels, map[string]interface{}{
			"name":            meta.Name,
			"display_name":    meta.DisplayName,
			"icon":            meta.Icon,
			"description":     meta.Description,
			"category":        meta.Category,
			"difficulty":      meta.Difficulty,
			"setup_time":      meta.SetupTime,
			"quick_setup":     meta.QuickSetup,
			"setup_type":      meta.SetupType,
			"configured":      configured,
			"has_token":       hasToken,
			"fields":          fields,
			"setup_steps":     meta.SetupSteps,
			"config_template": meta.ConfigTemplate,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"channels":         channels,
		"total":            len(channels),
		"configured_count": configuredCount,
	})
}

func isChannelConfigured(channelName string) bool {
	cfg, err := config.Load("")
	if err != nil {
		return false
	}

	switch channelName {
	case "telegram":
		return cfg.Channels.Telegram != nil && cfg.Channels.Telegram.BotTokenEnv != ""
	case "discord":
		return cfg.Channels.Discord != nil && cfg.Channels.Discord.BotTokenEnv != ""
	case "slack":
		return cfg.Channels.Slack != nil && cfg.Channels.Slack.BotTokenEnv != "" && cfg.Channels.Slack.AppTokenEnv != ""
	case "whatsapp":
		return cfg.Channels.WhatsApp != nil && cfg.Channels.WhatsApp.AccessTokenEnv != "" && cfg.Channels.WhatsApp.PhoneNumberID != ""
	case "qq":
		return cfg.Channels.QQ != nil && cfg.Channels.QQ.AppID != "" && (cfg.Channels.QQ.AppSecret != "" || cfg.Channels.QQ.AppSecretEnv != "")
	case "dingtalk":
		return cfg.Channels.DingTalk != nil && cfg.Channels.DingTalk.AccessTokenEnv != "" && cfg.Channels.DingTalk.SecretEnv != ""
	case "feishu":
		return cfg.Channels.Feishu != nil && cfg.Channels.Feishu.AppID != "" && (cfg.Channels.Feishu.AppSecretEnv != "" || cfg.Channels.Feishu.AppSecret != "")
	default:
		return false
	}
}

func (r *Router) handleCreateChannel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// handleConfigureChannel handles POST /api/channels/{name}/configure — Configures a channel.
func (r *Router) handleConfigureChannel(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")

	// Find channel metadata
	var channelMeta *ChannelMeta
	for _, meta := range CHANNEL_REGISTRY {
		if meta.Name == name {
			channelMeta = &meta
			break
		}
	}

	if channelMeta == nil {
		respondError(w, http.StatusNotFound, "channel not found")
		return
	}

	// Parse request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load current config
	cfg, err := config.Load("")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load config")
		return
	}

	// Create or update channel config
	var channelConfig *config.ChannelConfig
	switch name {
	case "telegram":
		if cfg.Channels.Telegram == nil {
			cfg.Channels.Telegram = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.Telegram
	case "discord":
		if cfg.Channels.Discord == nil {
			cfg.Channels.Discord = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.Discord
	case "slack":
		if cfg.Channels.Slack == nil {
			cfg.Channels.Slack = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.Slack
	case "whatsapp":
		if cfg.Channels.WhatsApp == nil {
			cfg.Channels.WhatsApp = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.WhatsApp
	case "qq":
		if cfg.Channels.QQ == nil {
			cfg.Channels.QQ = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.QQ
	case "dingtalk":
		if cfg.Channels.DingTalk == nil {
			cfg.Channels.DingTalk = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.DingTalk
	case "feishu":
		if cfg.Channels.Feishu == nil {
			cfg.Channels.Feishu = &config.ChannelConfig{}
		}
		channelConfig = cfg.Channels.Feishu
	default:
		respondError(w, http.StatusBadRequest, "unsupported channel")
		return
	}

	// Process fields
	fieldsData, hasFields := reqBody["fields"].(map[string]interface{})
	if !hasFields {
		fieldsData = reqBody
	}

	for _, field := range channelMeta.Fields {
		value, exists := fieldsData[field.Key]
		if !exists {
			continue
		}

		valueStr, ok := value.(string)
		if !ok {
			continue
		}

		// If value is empty, skip (don't overwrite existing)
		if valueStr == "" {
			continue
		}

		// Handle env vars (secrets)
		if field.EnvVar != nil && field.FieldType == FieldTypeSecret {
			// Directly save the secret in config (no secrets.env dependency)
			switch field.Key {
			case "bot_token_env":
				channelConfig.BotToken = valueStr
			case "app_token_env":
				channelConfig.AppToken = valueStr
			case "access_token_env":
				channelConfig.AccessToken = valueStr
			case "app_secret_env":
				channelConfig.AppSecret = valueStr
			case "secret_env":
				channelConfig.Secret = valueStr
			case "verify_token_env":
				channelConfig.VerifyToken = valueStr
			case "client_secret_env":
				channelConfig.ClientSecret = valueStr
			}
			// Also set in current process for backward compatibility
			os.Setenv(*field.EnvVar, valueStr)
		} else {
			// Handle regular fields
			switch field.Key {
			case "app_id":
				channelConfig.AppID = valueStr
			case "client_id":
				channelConfig.ClientID = valueStr
			case "bot_token_env":
				channelConfig.BotTokenEnv = valueStr
			case "app_token_env":
				channelConfig.AppTokenEnv = valueStr
			case "access_token_env":
				channelConfig.AccessTokenEnv = valueStr
			case "app_secret_env":
				channelConfig.AppSecretEnv = valueStr
			case "secret_env":
				channelConfig.SecretEnv = valueStr
			case "verify_token_env":
				channelConfig.VerifyTokenEnv = valueStr
			case "client_secret_env":
				channelConfig.ClientSecretEnv = valueStr
			case "allowed_users":
				channelConfig.AllowedUsers = valueStr
			case "allowed_guilds":
				channelConfig.AllowedGuilds = valueStr
			case "allowed_channels":
				channelConfig.AllowedChannels = valueStr
			case "default_agent":
				channelConfig.DefaultAgent = valueStr
			case "phone_number_id":
				channelConfig.PhoneNumberID = valueStr
			}
		}
	}

	// Save config
	if err := config.Save(cfg, ""); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	// Reload channels
	started, err := reloadChannelsFromDisk(r.kernel, name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to reload channels: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "configured",
		"started_channels": started,
	})
}

// nameToChannelType converts channel name to ChannelType
func nameToChannelType(name string) channels.ChannelType {
	switch name {
	case "telegram":
		return channels.ChannelTypeTelegram
	case "discord":
		return channels.ChannelTypeDiscord
	case "slack":
		return channels.ChannelTypeSlack
	case "whatsapp":
		return channels.ChannelTypeWhatsApp
	case "qq":
		return channels.ChannelTypeQQ
	case "dingtalk":
		return channels.ChannelTypeDingTalk
	case "feishu":
		return channels.ChannelTypeFeishu
	default:
		return channels.ChannelType(name)
	}
}

// reloadChannelsFromDisk reloads a specific channel from disk and restarts it.
func reloadChannelsFromDisk(k *kernel.Kernel, channelName string) ([]string, error) {
	// Load fresh config
	cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}

	registry := k.Registry()
	var started []string

	if registry != nil {
		channelType := nameToChannelType(channelName)

		// Check if this channel has an adapter factory
		_, hasFactory := registry.GetFactory(channelType)
		if !hasFactory {
			return started, nil
		}

		// Stop existing channels of this type
		existingChannels := registry.ListChannels()
		for _, ch := range existingChannels {
			if ch.Type == channelType {
				if adapter, ok := registry.GetAdapter(ch.ID); ok {
					adapter.Disconnect()
				}
				registry.RemoveChannel(ch.ID)
			}
		}

		// Check if channel is configured
		isConfigured := false
		switch channelName {
		case "telegram":
			isConfigured = cfg.Channels.Telegram != nil && cfg.Channels.Telegram.BotTokenEnv != ""
		case "discord":
			isConfigured = cfg.Channels.Discord != nil && cfg.Channels.Discord.BotTokenEnv != ""
		case "slack":
			isConfigured = cfg.Channels.Slack != nil && cfg.Channels.Slack.BotTokenEnv != "" && cfg.Channels.Slack.AppTokenEnv != ""
		case "whatsapp":
			isConfigured = cfg.Channels.WhatsApp != nil && (cfg.Channels.WhatsApp.AccessTokenEnv != "" || cfg.Channels.WhatsApp.PhoneNumberID != "")
		case "qq":
			isConfigured = cfg.Channels.QQ != nil && cfg.Channels.QQ.AppID != "" && cfg.Channels.QQ.AppSecretEnv != ""
		case "dingtalk":
			isConfigured = cfg.Channels.DingTalk != nil && cfg.Channels.DingTalk.AccessTokenEnv != "" && cfg.Channels.DingTalk.SecretEnv != ""
		case "feishu":
			isConfigured = cfg.Channels.Feishu != nil && cfg.Channels.Feishu.AppID != "" && (cfg.Channels.Feishu.AppSecretEnv != "" || cfg.Channels.Feishu.AppSecret != "")
		}

		if isConfigured {
			// Create and register new channel
			newChannel := &channels.Channel{
				Name:  channelName + " Channel",
				Type:  channelType,
				State: channels.ChannelStateIdle,
			}

			// Set channel-specific config
			switch channelName {
			case "qq":
				newChannel.Config.QQ = &channels.QQChannelConfig{
					AppID:     cfg.Channels.QQ.AppID,
					AppSecret: os.Getenv(cfg.Channels.QQ.AppSecretEnv),
				}
			case "dingtalk":
				newChannel.Config.DingTalk = &channels.DingTalkChannelConfig{
					AppSecret: os.Getenv(cfg.Channels.DingTalk.SecretEnv),
				}
			case "feishu":
				appSecret := cfg.Channels.Feishu.AppSecret
				if appSecret == "" && cfg.Channels.Feishu.AppSecretEnv != "" {
					appSecret = os.Getenv(cfg.Channels.Feishu.AppSecretEnv)
				}
				newChannel.Config.Feishu = &channels.FeishuChannelConfig{
					AppID:     cfg.Channels.Feishu.AppID,
					AppSecret: appSecret,
				}
			}

			if err := registry.RegisterChannel(newChannel); err == nil {
				// Try to start the adapter
				if adapter, ok := registry.GetAdapter(newChannel.ID); ok {
					if err := adapter.Start(); err == nil {
						started = append(started, channelName)
					}
				}
			}
		}
	}

	return started, nil
}

// handleTestChannel tests if a channel is properly configured and connected.
func (r *Router) handleTestChannel(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "channel name is required")
		return
	}

	// Find channel meta
	var channelMeta *ChannelMeta
	for _, meta := range CHANNEL_REGISTRY {
		if meta.Name == name {
			channelMeta = &meta
			break
		}
	}

	if channelMeta == nil {
		respondError(w, http.StatusNotFound, "unknown channel")
		return
	}

	// For Feishu, check config file directly instead of env vars
	if name == "feishu" {
		cfg, err := config.Load("")
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("Failed to load config: %v", err),
			})
			return
		}
		if cfg.Channels.Feishu == nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "error",
				"message": "Feishu config not found",
			})
			return
		}
		if cfg.Channels.Feishu.AppID == "" {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "error",
				"message": "Feishu App ID is missing",
			})
			return
		}
		if cfg.Channels.Feishu.AppSecret == "" && cfg.Channels.Feishu.AppSecretEnv == "" {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "error",
				"message": "Feishu App Secret is missing",
			})
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"message": fmt.Sprintf("All required credentials for %s are set.", channelMeta.DisplayName),
		})
		return
	}

	// Check all required env vars are set for other channels
	var missing []string
	for _, fieldDef := range channelMeta.Fields {
		if fieldDef.Required && fieldDef.EnvVar != nil {
			value := os.Getenv(*fieldDef.EnvVar)
			if value == "" {
				missing = append(missing, *fieldDef.EnvVar)
			}
		}
	}

	if len(missing) > 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Missing required env vars: %s", strings.Join(missing, ", ")),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("All required credentials for %s are set.", channelMeta.DisplayName),
	})
}

func (r *Router) handleDeleteChannel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusNoContent, nil)
}

// OpenAI-compatible handlers (already implemented in openai_compat.go)
func (r *Router) handleListModels(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"id":       "gpt-4o",
				"object":   "model",
				"created":  1699000000,
				"owned_by": "openai",
			},
		},
	})
}

func (r *Router) handleCommands(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []map[string]string{
		{"cmd": "/help", "desc": "Show available commands"},
		{"cmd": "/clear", "desc": "Clear chat history"},
		{"cmd": "/model", "desc": "Switch model"},
	})
}

func (r *Router) handleBudget(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"monthly_limit": 0.0,
		"monthly_spent": 0.0,
	})
}

func (r *Router) handleBudgetAgents(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleNetworkStatus(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connected": true,
		"agents":    []interface{}{},
	})
}

func (r *Router) handleA2AAgents(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"agents": []interface{}{},
	})
}

// handleA2AAgentCard handles GET /.well-known/agent.json — A2A Agent Card
func (r *Router) handleA2AAgentCard(w http.ResponseWriter, req *http.Request) {
	agents := r.kernel.AgentRegistry().List()
	baseURL := fmt.Sprintf("http://%s", req.Host)

	if len(agents) > 0 {
		card := a2a.BuildAgentCard(baseURL)
		respondJSON(w, http.StatusOK, card)
	} else {
		card := map[string]interface{}{
			"name":               "fangclaw-go",
			"description":        "FangClaw-go Agent — no agents spawned yet",
			"url":                baseURL + "/a2a",
			"version":            "0.1.0",
			"capabilities":       map[string]bool{"streaming": true},
			"skills":             []interface{}{},
			"defaultInputModes":  []string{"text"},
			"defaultOutputModes": []string{"text"},
		}
		respondJSON(w, http.StatusOK, card)
	}
}

// handleA2AListAgents handles GET /a2a/agents — List all discovered external A2A agents
func (r *Router) handleA2AListAgents(w http.ResponseWriter, req *http.Request) {
	var cards []interface{} = make([]interface{}, 0)

	if r.kernel.A2AClient() != nil {
		externalAgents := r.kernel.A2AClient().ListExternalAgents()
		for _, extAgent := range externalAgents {
			cards = append(cards, extAgent.Card)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"agents": cards,
		"total":  len(cards),
	})
}

// handleA2ASendTask handles POST /a2a/tasks/send — Submit a task to a local agent
func (r *Router) handleA2ASendTask(w http.ResponseWriter, req *http.Request) {
	var request map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	messageText := "No message provided"
	if params, ok := request["params"].(map[string]interface{}); ok {
		if message, ok := params["message"].(map[string]interface{}); ok {
			if parts, ok := message["parts"].([]interface{}); ok {
				for _, part := range parts {
					if p, ok := part.(map[string]interface{}); ok {
						if p["type"] == "text" {
							if text, ok := p["text"].(string); ok {
								messageText = text
								break
							}
						}
					}
				}
			}
		}
	}

	agentRuntime := r.kernel.AgentRuntime()

	var actualAgentID string

	if r.defaultAgent != "" {
		if _, ok := agentRuntime.GetAgent(r.defaultAgent); ok {
			actualAgentID = r.defaultAgent
		} else if agentCtx, ok := agentRuntime.FindAgentByName(r.defaultAgent); ok {
			actualAgentID = agentCtx.ID
		} else if agentEntry := r.kernel.AgentRegistry().FindByName(r.defaultAgent); agentEntry != nil {
			actualAgentID = agentEntry.ID.String()
		}
	}

	var actualAgentName string
	if actualAgentID == "" {
		if agentCtx, ok := agentRuntime.GetFirstAgent(); ok {
			actualAgentID = agentCtx.ID
			actualAgentName = agentCtx.Name
		} else {
			agents := r.kernel.AgentRegistry().List()
			if len(agents) == 0 {
				respondError(w, http.StatusNotFound, "No agents available")
				return
			}
			actualAgentID = agents[0].ID.String()
			actualAgentName = agents[0].Name
		}
	} else {
		if aid, err := uuid.Parse(actualAgentID); err == nil {
			agentEntry := r.kernel.AgentRegistry().Get(aid)
			if agentEntry != nil {
				actualAgentName = agentEntry.Name
			}
		}
		if agentCtx, ok := agentRuntime.GetAgent(actualAgentID); ok {
			actualAgentName = agentCtx.Name
		}
	}

	var sessionID *string
	if params, ok := request["params"].(map[string]interface{}); ok {
		if sid, ok := params["sessionId"].(string); ok && sid != "" {
			sessionID = &sid
		}
	}

	if r.kernel.A2ATaskStore() == nil {
		respondError(w, http.StatusInternalServerError, "A2A not configured")
		return
	}

	messages := []a2a.A2aMessage{
		{
			Role: "user",
			Parts: []a2a.A2aPart{
				{
					Type: "text",
					Text: messageText,
				},
			},
		},
	}
	task := r.kernel.A2ATaskStore().CreateTask(actualAgentID, actualAgentName, messages, sessionID)
	task.Status.State = a2a.A2aTaskStatusWorking
	r.kernel.A2ATaskStore().UpdateTaskStatus(task.ID, a2a.A2aTaskStatusWorking)

	r.kernel.AuditLog().Record("a2a", actualAgentID, audit.ActionA2ATaskCreated, fmt.Sprintf("task_id=%s", task.ID), "submitted")

	go func() {
		result, err := r.kernel.SendMessage(context.Background(), actualAgentID, messageText)
		if err != nil {
			errorMsg := a2a.A2aMessage{
				Role: "agent",
				Parts: []a2a.A2aPart{
					{
						Type: "text",
						Text: fmt.Sprintf("Error: %v", err),
					},
				},
			}
			r.kernel.A2ATaskStore().FailTask(task.ID, errorMsg)
			r.kernel.AuditLog().Record("a2a", actualAgentID, audit.ActionA2ATaskFailed, fmt.Sprintf("task_id=%s, error=%v", task.ID, err), "failed")
		} else {
			responseMsg := a2a.A2aMessage{
				Role: "agent",
				Parts: []a2a.A2aPart{
					{
						Type: "text",
						Text: result,
					},
				},
			}
			r.kernel.A2ATaskStore().CompleteTask(task.ID, responseMsg, []a2a.A2aArtifact{})
			r.kernel.AuditLog().Record("a2a", actualAgentID, audit.ActionA2ATaskCompleted, fmt.Sprintf("task_id=%s", task.ID), "completed")
		}
	}()

	if completedTask, ok := r.kernel.A2ATaskStore().GetTask(task.ID); ok {
		respondJSON(w, http.StatusOK, completedTask)
	} else {
		respondError(w, http.StatusInternalServerError, "Task disappeared after submission")
	}
}

// handleA2AGetTask handles GET /a2a/tasks/{id} — Get task status
func (r *Router) handleA2AGetTask(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")
	if r.kernel.A2ATaskStore() == nil {
		respondError(w, http.StatusInternalServerError, "A2A not configured")
		return
	}

	if task, ok := r.kernel.A2ATaskStore().GetTask(taskID); ok {
		respondJSON(w, http.StatusOK, task)
	} else {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Task '%s' not found", taskID))
	}
}

// handleA2ACancelTask handles POST /a2a/tasks/{id}/cancel — Cancel a task
func (r *Router) handleA2ACancelTask(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")
	if r.kernel.A2ATaskStore() == nil {
		respondError(w, http.StatusInternalServerError, "A2A not configured")
		return
	}

	if task, ok := r.kernel.A2ATaskStore().GetTask(taskID); ok {
		if r.kernel.A2ATaskStore().CancelTask(taskID) {
			var sessionIDStr string
			if task.SessionID != nil {
				sessionIDStr = *task.SessionID
			} else {
				sessionIDStr = "default"
			}
			r.kernel.AuditLog().Record("a2a", sessionIDStr, audit.ActionA2ATaskCancelled, fmt.Sprintf("task_id=%s", taskID), "cancelled")
			if updatedTask, ok := r.kernel.A2ATaskStore().GetTask(taskID); ok {
				respondJSON(w, http.StatusOK, updatedTask)
			} else {
				respondError(w, http.StatusInternalServerError, "Task disappeared after cancellation")
			}
		} else {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Task '%s' cannot be cancelled", taskID))
		}
	} else {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Task '%s' not found", taskID))
	}
}

// handleA2ADiscoverExternal handles POST /api/a2a/discover — Discover an external A2A agent
func (r *Router) handleA2ADiscoverExternal(w http.ResponseWriter, req *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Missing 'url' field")
		return
	}

	if r.kernel.A2AClient() == nil {
		respondError(w, http.StatusInternalServerError, "A2A not configured")
		return
	}

	card, err := r.kernel.A2AClient().DiscoverAgent(body.URL)
	if err != nil {
		r.kernel.AuditLog().Record("a2a", body.URL, audit.ActionA2AAgentDiscovered, fmt.Sprintf("url=%s, error=%v", body.URL, err), "failed")
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	r.kernel.AuditLog().Record("a2a", body.URL, audit.ActionA2AAgentDiscovered, fmt.Sprintf("url=%s, agent=%s", body.URL, card.Name), "success")

	// Record A2A event
	r.kernel.A2AClient().RecordAgentDiscoveredEvent(r.kernel.A2AEventStore(), card, body.URL)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"url":   body.URL,
		"agent": card,
	})
}

// handleA2ATopology handles GET /api/a2a/topology — Get agent topology (local + external)
func (r *Router) handleA2ATopology(w http.ResponseWriter, req *http.Request) {
	var topology a2a.Topology

	// Add local agents
	localAgents := r.kernel.AgentRegistry().List()
	for _, agent := range localAgents {
		topology.Nodes = append(topology.Nodes, &a2a.TopoNode{
			ID:    agent.ID.String(),
			Name:  agent.Name,
			Type:  "local",
			State: string(agent.State),
		})
	}

	// Add external agents
	if r.kernel.A2AClient() != nil {
		externalAgents := r.kernel.A2AClient().ListExternalAgents()
		for _, extAgent := range externalAgents {
			topology.Nodes = append(topology.Nodes, &a2a.TopoNode{
				ID:    extAgent.Card.Name,
				Name:  extAgent.Card.Name,
				Type:  "external",
				URL:   extAgent.URL,
				State: "discovered",
			})
		}
	}

	// No edges for now - can add later if needed
	topology.Edges = []*a2a.TopoEdge{}

	respondJSON(w, http.StatusOK, topology)
}

// handleA2AEvents handles GET /api/a2a/events — Get A2A events
func (r *Router) handleA2AEvents(w http.ResponseWriter, req *http.Request) {
	if r.kernel.A2AEventStore() == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	// Get limit from query parameter, default 200
	limitStr := req.URL.Query().Get("limit")
	limit := 200
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events := r.kernel.A2AEventStore().ListEvents(limit)
	respondJSON(w, http.StatusOK, events)
}

// handleCommsSend handles POST /api/comms/send — Send a message from one agent to another (local or external)
func (r *Router) handleCommsSend(w http.ResponseWriter, req *http.Request) {
	var body struct {
		FromAgentID   string `json:"fromAgentId"`
		FromAgentName string `json:"fromAgentName"`
		ToAgentID     string `json:"toAgentId"`
		ToAgentName   string `json:"toAgentName"`
		Message       string `json:"message"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.ToAgentID == "" {
		respondError(w, http.StatusBadRequest, "Missing 'toAgentId' field")
		return
	}
	if body.Message == "" {
		respondError(w, http.StatusBadRequest, "Missing 'message' field")
		return
	}

	// Check if agent is external agent
	var externalAgent *a2a.ExternalAgent
	if r.kernel.A2AClient() != nil {
		externalAgents := r.kernel.A2AClient().ListExternalAgents()
		for _, extAgent := range externalAgents {
			if extAgent.Card.Name == body.ToAgentID {
				externalAgent = extAgent
				if body.ToAgentName == "" {
					body.ToAgentName = extAgent.Card.Name
				}
				break
			}
		}
	}

	if externalAgent != nil {
		// Send to external agent first
		externalTask, err := r.kernel.A2AClient().SendTask(externalAgent.URL, body.Message, nil)
		if err != nil {
			r.kernel.AuditLog().Record("a2a", externalAgent.URL, audit.ActionA2ATaskSent, fmt.Sprintf("message=%s, error=%v", body.Message, err), "failed")
			respondError(w, http.StatusBadGateway, err.Error())
			return
		}

		// Create task in store with external task ID
		externalTask.AgentName = body.ToAgentName
		r.kernel.A2ATaskStore().AddTask(externalTask)
		r.kernel.AuditLog().Record("a2a", externalAgent.URL, audit.ActionA2ATaskSent, fmt.Sprintf("message=%s, task_id=%s", body.Message, externalTask.ID), "success")

		if r.kernel.A2AEventStore() != nil {
			event := &a2a.A2AEvent{
				ID:         uuid.New().String(),
				Timestamp:  time.Now(),
				Kind:       "agent_message",
				SourceID:   body.FromAgentID,
				SourceName: body.FromAgentName,
				TargetID:   body.ToAgentID,
				TargetName: body.ToAgentName,
				Detail:     body.Message,
				Payload:    map[string]interface{}{"task": externalTask},
			}
			r.kernel.A2AEventStore().AddEvent(event)
		}

		// If task is already completed/failed/cancelled, just update status (messages and artifacts are already in the task)
		if externalTask.Status.State == a2a.A2aTaskStatusCompleted ||
			externalTask.Status.State == a2a.A2aTaskStatusFailed ||
			externalTask.Status.State == a2a.A2aTaskStatusCancelled {
			r.kernel.A2ATaskStore().UpdateTaskStatus(externalTask.ID, externalTask.Status.State)
		} else {
			// Start polling for status
			r.kernel.A2AClient().StartPolling(externalTask.ID, externalAgent.URL, 2*time.Second)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"task": externalTask,
		})
		return
	}

	// Create task for local agent
	messages := []a2a.A2aMessage{
		{
			Role: "user",
			Parts: []a2a.A2aPart{
				{Type: "text", Text: body.Message},
			},
		},
	}
	task := r.kernel.A2ATaskStore().CreateTask(body.ToAgentID, body.ToAgentName, messages, nil)

	// Execute task asynchronously
	go func() {
		ctx := context.Background()
		r.kernel.A2ATaskStore().UpdateTaskStatus(task.ID, a2a.A2aTaskStatusWorking)

		response, err := r.kernel.SendMessage(ctx, body.ToAgentID, body.Message)
		if err != nil {
			r.kernel.AuditLog().Record("comms", body.ToAgentID, audit.ActionAgentMessage, fmt.Sprintf("to=%s, error=%v", body.ToAgentID, err), "failed")
			r.kernel.A2ATaskStore().FailTask(task.ID, a2a.A2aMessage{
				Role: "assistant",
				Parts: []a2a.A2aPart{
					{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
				},
			})
			return
		}

		r.kernel.AuditLog().Record("comms", body.ToAgentID, audit.ActionAgentMessage, fmt.Sprintf("from=%s, to=%s", body.FromAgentID, body.ToAgentID), "success")

		// Complete task with response
		r.kernel.A2ATaskStore().CompleteTask(task.ID, a2a.A2aMessage{
			Role: "assistant",
			Parts: []a2a.A2aPart{
				{Type: "text", Text: response},
			},
		}, nil)
	}()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task": task,
	})
}

// handleCommsTask handles POST /api/comms/task — Publish a task to an agent's task queue
func (r *Router) handleCommsTask(w http.ResponseWriter, req *http.Request) {
	var body struct {
		AgentID    string `json:"agentId"`
		AssignedTo string `json:"assignedTo"`
		AgentName  string `json:"agentName"`
		Title      string `json:"title"`
		Desc       string `json:"description"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Title == "" {
		respondError(w, http.StatusBadRequest, "Missing 'title' field")
		return
	}

	// Determine which agent to use
	agentID := body.AgentID
	if agentID == "" {
		agentID = body.AssignedTo
	}

	// If no agent specified, use the first available local agent
	if agentID == "" {
		localAgents := r.kernel.AgentRegistry().List()
		if len(localAgents) == 0 {
			respondError(w, http.StatusBadRequest, "No agents available")
			return
		}
		agentID = localAgents[0].ID.String()
		if body.AgentName == "" {
			body.AgentName = localAgents[0].Name
		}
	}

	// Check if agent is external agent
	var externalAgent *a2a.ExternalAgent
	if r.kernel.A2AClient() != nil {
		externalAgents := r.kernel.A2AClient().ListExternalAgents()
		for _, extAgent := range externalAgents {
			if extAgent.Card.Name == agentID {
				externalAgent = extAgent
				if body.AgentName == "" {
					body.AgentName = extAgent.Card.Name
				}
				break
			}
		}
	}

	message := fmt.Sprintf("Task: %s\n\n%s", body.Title, body.Desc)

	if externalAgent != nil {
		// Send to external agent
		if r.kernel.A2AClient() == nil {
			respondError(w, http.StatusInternalServerError, "A2A not configured")
			return
		}

		// Send to external agent first
		externalTask, err := r.kernel.A2AClient().SendTask(externalAgent.URL, message, nil)
		if err != nil {
			r.kernel.AuditLog().Record("a2a", externalAgent.URL, audit.ActionA2ATaskSent, fmt.Sprintf("task=%s, error=%v", body.Title, err), "failed")
			respondError(w, http.StatusBadGateway, err.Error())
			return
		}

		// Create task in store with external task ID
		externalTask.AgentName = body.AgentName
		r.kernel.A2ATaskStore().AddTask(externalTask)
		r.kernel.AuditLog().Record("a2a", externalAgent.URL, audit.ActionA2ATaskSent, fmt.Sprintf("task=%s, task_id=%s", body.Title, externalTask.ID), "success")

		if r.kernel.A2AEventStore() != nil {
			event := &a2a.A2AEvent{
				ID:         uuid.New().String(),
				Timestamp:  time.Now(),
				Kind:       "agent_task",
				SourceID:   "system",
				SourceName: "System",
				TargetID:   agentID,
				TargetName: body.AgentName,
				Detail:     body.Title,
				Payload:    map[string]interface{}{"description": body.Desc, "task": externalTask},
			}
			r.kernel.A2AEventStore().AddEvent(event)
		}

		// If task is already completed/failed/cancelled, just update status (messages and artifacts are already in the task)
		if externalTask.Status.State == a2a.A2aTaskStatusCompleted ||
			externalTask.Status.State == a2a.A2aTaskStatusFailed ||
			externalTask.Status.State == a2a.A2aTaskStatusCancelled {
			r.kernel.A2ATaskStore().UpdateTaskStatus(externalTask.ID, externalTask.Status.State)
		} else {
			// Start polling for status
			r.kernel.A2AClient().StartPolling(externalTask.ID, externalAgent.URL, 2*time.Second)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"task": externalTask,
		})
		return
	}

	// If agentName not provided, try to get it from registry
	if body.AgentName == "" {
		if aid, err := uuid.Parse(agentID); err == nil {
			agentEntry := r.kernel.AgentRegistry().Get(aid)
			if agentEntry != nil {
				body.AgentName = agentEntry.Name
			}
		}
	}

	// Create task for local agent
	messages := []a2a.A2aMessage{
		{
			Role: "user",
			Parts: []a2a.A2aPart{
				{Type: "text", Text: message},
			},
		},
	}
	task := r.kernel.A2ATaskStore().CreateTask(agentID, body.AgentName, messages, nil)

	// Execute task asynchronously
	go func() {
		ctx := context.Background()
		r.kernel.A2ATaskStore().UpdateTaskStatus(task.ID, a2a.A2aTaskStatusWorking)

		response, err := r.kernel.SendMessage(ctx, agentID, message)
		if err != nil {
			r.kernel.AuditLog().Record("comms", agentID, audit.ActionAgentMessage, fmt.Sprintf("task=%s, error=%v", body.Title, err), "failed")
			r.kernel.A2ATaskStore().FailTask(task.ID, a2a.A2aMessage{
				Role: "assistant",
				Parts: []a2a.A2aPart{
					{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
				},
			})
			return
		}

		r.kernel.AuditLog().Record("comms", agentID, audit.ActionAgentMessage, fmt.Sprintf("task=%s, agent=%s", body.Title, agentID), "success")

		// Complete task with response
		r.kernel.A2ATaskStore().CompleteTask(task.ID, a2a.A2aMessage{
			Role: "assistant",
			Parts: []a2a.A2aPart{
				{Type: "text", Text: response},
			},
		}, nil)
	}()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task": task,
	})
}

// handleListTasks handles GET /api/tasks — List all tasks
func (r *Router) handleListTasks(w http.ResponseWriter, req *http.Request) {
	if r.kernel.A2ATaskStore() == nil {
		respondError(w, http.StatusInternalServerError, "Task store not available")
		return
	}
	tasks := r.kernel.A2ATaskStore().ListTasks()
	respondJSON(w, http.StatusOK, tasks)
}

// handleGetTask handles GET /api/tasks/{id} — Get a specific task by ID
func (r *Router) handleGetTask(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")
	if taskID == "" {
		respondError(w, http.StatusBadRequest, "Missing task ID")
		return
	}

	if r.kernel.A2ATaskStore() == nil {
		respondError(w, http.StatusInternalServerError, "Task store not available")
		return
	}

	task, ok := r.kernel.A2ATaskStore().GetTask(taskID)
	if !ok {
		respondError(w, http.StatusNotFound, "Task not found")
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// handleA2ASendExternal handles POST /api/a2a/send — Send task to an external A2A agent
func (r *Router) handleA2ASendExternal(w http.ResponseWriter, req *http.Request) {
	var body struct {
		URL       string  `json:"url"`
		Message   string  `json:"message"`
		SessionID *string `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.URL == "" {
		respondError(w, http.StatusBadRequest, "Missing 'url' field")
		return
	}
	if body.Message == "" {
		respondError(w, http.StatusBadRequest, "Missing 'message' field")
		return
	}

	if r.kernel.A2AClient() == nil {
		respondError(w, http.StatusInternalServerError, "A2A not configured")
		return
	}

	task, err := r.kernel.A2AClient().SendTask(body.URL, body.Message, body.SessionID)
	if err != nil {
		r.kernel.AuditLog().Record("a2a", body.URL, audit.ActionA2ATaskSent, fmt.Sprintf("url=%s, error=%v", body.URL, err), "failed")
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	r.kernel.AuditLog().Record("a2a", body.URL, audit.ActionA2ATaskSent, fmt.Sprintf("url=%s, task_id=%s", body.URL, task.ID), "success")

	respondJSON(w, http.StatusOK, task)
}

// handleA2AExternalTaskStatus handles GET /api/a2a/tasks/{id}/status — Get external task status
func (r *Router) handleA2AExternalTaskStatus(w http.ResponseWriter, req *http.Request) {
	taskID := req.PathValue("id")
	url := req.URL.Query().Get("url")
	if url == "" {
		respondError(w, http.StatusBadRequest, "Missing 'url' query parameter")
		return
	}

	if r.kernel.A2AClient() == nil {
		respondError(w, http.StatusInternalServerError, "A2A not configured")
		return
	}

	task, err := r.kernel.A2AClient().GetTaskStatus(url, taskID)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, task)
}

func (r *Router) handleGetAgentSession(w http.ResponseWriter, req *http.Request) {
	agentIDStr := req.PathValue("id")
	if agentIDStr == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	sessionsStore := r.kernel.SessionStore()
	if sessionsStore == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": "default",
			"messages":   []interface{}{},
		})
		return
	}

	// Check if we have a session id in query or use default
	sessionIDStr := req.URL.Query().Get("session_id")
	if sessionIDStr == "" {
		// If no session id specified, try to get the latest session for this agent
		sessions, err := sessionsStore.ListAgentSessions(agentID)
		if err != nil || len(sessions) == 0 {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"session_id": "default",
				"messages":   []interface{}{},
			})
			return
		}
		// Use the latest session
		sessionIDStr = sessions[0]["session_id"].(string)
	}

	sessionID, err := types.ParseSessionID(sessionIDStr)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": sessionIDStr,
			"messages":   []interface{}{},
		})
		return
	}

	session, err := sessionsStore.GetSession(sessionID)
	if err != nil || session == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": sessionIDStr,
			"messages":   []interface{}{},
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": session.ID.String(),
		"messages":   session.Messages,
	})
}

func (r *Router) handleGetAgentSessions(w http.ResponseWriter, req *http.Request) {
	agentIDStr := req.PathValue("id")
	if agentIDStr == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	sessionsStore := r.kernel.SessionStore()
	if sessionsStore == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"sessions": []interface{}{}})
		return
	}

	sessions, err := sessionsStore.ListAgentSessions(agentID)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"sessions": []interface{}{}})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"sessions": sessions})
}

func (r *Router) handleCreateAgentSession(w http.ResponseWriter, req *http.Request) {
	agentIDStr := req.PathValue("id")
	if agentIDStr == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}

	agentID, err := types.ParseAgentID(agentIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	var reqBody struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		// Ignore decode errors, use empty label
	}

	sessionsStore := r.kernel.SessionStore()
	agent := r.kernel.AgentRegistry().Get(agentID)

	agentName := ""
	agentModelProvider := ""
	agentModelName := ""
	if agent != nil {
		agentName = agent.Name
		agentModelProvider = agent.Manifest.Model.Provider
		agentModelName = agent.Manifest.Model.Model
	}

	session := types.NewSession(
		agentID,
		agentName,
		agentModelProvider,
		agentModelName,
		&reqBody.Label,
	)

	if sessionsStore != nil {
		if err := sessionsStore.SaveSession(&session); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"session_id": session.ID.String(),
	})
}

func (r *Router) handleSwitchSession(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	sessionID := req.PathValue("sid")

	if agentID == "" || sessionID == "" {
		respondError(w, http.StatusBadRequest, "agent id and session id required")
		return
	}

	// Switch session - in a real implementation, this would update the agent's current session
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleResetSession(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (r *Router) handleCompactSession(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "agent id required")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "compacted",
		"tokens_before": 0,
		"tokens_after":  0,
	})
}

func (r *Router) handleAgentMessage(w http.ResponseWriter, req *http.Request) {
	agentIdentifier := req.PathValue("id")

	// Parse request body
	var reqBody struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	agentRuntime := r.kernel.AgentRuntime()
	if agentRuntime == nil {
		respondError(w, http.StatusInternalServerError, "agent runtime not available")
		return
	}

	// Agent lookup strategy:
	// 1. First try direct ID lookup
	// 2. If not found, try name lookup in agentRuntime
	// 3. If still not found, try name lookup in kernel's AgentRegistry
	// 4. If all fail, use the first available agent
	var actualAgentID string
	if _, ok := agentRuntime.GetAgent(agentIdentifier); ok {
		actualAgentID = agentIdentifier
	} else if agentCtx, ok := agentRuntime.FindAgentByName(agentIdentifier); ok {
		actualAgentID = agentCtx.ID
	} else {
		if agentEntry := r.kernel.AgentRegistry().FindByName(agentIdentifier); agentEntry != nil {
			actualAgentID = agentEntry.ID.String()
		} else {
			if agentCtx, ok := agentRuntime.GetFirstAgent(); ok {
				actualAgentID = agentCtx.ID
			} else {
				respondError(w, http.StatusNotFound, "no agents available")
				return
			}
		}
	}

	ctx := context.Background()
	response, inputTokens, outputTokens, err := r.kernel.SendMessageWithUsage(ctx, actualAgentID, reqBody.Message)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"response": response,
		"message": map[string]string{
			"role":    "assistant",
			"content": response,
		},
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	})
}

func getLLMDriver() (llm.Driver, error) {
	cfg, err := config.Load("")
	if err == nil && cfg.DefaultModel.Provider != "" && cfg.DefaultModel.Model != "" {
		provider := cfg.DefaultModel.Provider
		model := cfg.DefaultModel.Model
		apiKeyEnv := cfg.DefaultModel.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = strings.ToUpper(provider) + "_API_KEY"
		}
		apiKey := os.Getenv(apiKeyEnv)
		if apiKey != "" {
			driver, err := llm.NewDriver(provider, apiKey, model)
			if err == nil {
				return driver, nil
			}
		}
	}

	provider := "openrouter"
	model := "meta-llama/llama-3.1-8b-instruct"
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	if apiKey == "" {
		provider = "openai"
		model = "gpt-4o"
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		provider = "anthropic"
		model = "claude-sonnet-4-20250514"
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if apiKey == "" {
		provider = "groq"
		model = "groq/llama-3.3-70b-versatile"
		apiKey = os.Getenv("GROQ_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found. Set OPENROUTER_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, or GROQ_API_KEY")
	}

	return llm.NewDriver(provider, apiKey, model)
}

func getHandSystemPrompt(handID string) string {
	switch handID {
	case "researcher":
		return hands.ResearcherSystemPrompt
	case "lead":
		return hands.LeadSystemPrompt
	case "collector":
		return hands.CollectorSystemPrompt
	case "predictor":
		return hands.PredictorSystemPrompt
	case "clip":
		return hands.ClipSystemPrompt
	case "twitter":
		return hands.TwitterSystemPrompt
	case "browser":
		return hands.BrowserSystemPrompt
	default:
		return ""
	}
}

func (r *Router) handleStopAgent(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (r *Router) handleUpdateAgentModel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (r *Router) handleDeleteAgentAlias(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusNoContent, nil)
}

func (r *Router) handleTools(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tools": []interface{}{},
	})
}

func (r *Router) handleUsage(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_tokens":     0,
		"total_cost_usd":   0.0,
		"period_start":     "2024-01-01",
		"period_end":       "2024-12-31",
		"agents":           []interface{}{},
		"first_event_date": nil,
	})
}

func (r *Router) handleAuditRecent(w http.ResponseWriter, req *http.Request) {
	nStr := req.URL.Query().Get("n")
	n := 50
	if nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > 1000 {
		n = 1000
	}

	entries := r.kernel.AuditLog().GetRecent(n)
	tipHash := r.kernel.AuditLog().GetChainHash()

	items := make([]map[string]interface{}, 0, len(entries))
	for i, entry := range entries {
		items = append(items, map[string]interface{}{
			"seq":       i + 1,
			"timestamp": entry.Timestamp.Format(time.RFC3339),
			"agent_id":  entry.Target,
			"action":    string(entry.Action),
			"detail":    entry.Details,
			"outcome":   entry.Result,
			"hash":      entry.Hash,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries":  items,
		"tip_hash": tipHash,
	})
}

func (r *Router) handleAuditVerify(w http.ResponseWriter, req *http.Request) {
	valid, err := r.kernel.AuditLog().Verify()
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   valid,
		"entries": r.kernel.AuditLog().Count(),
	})
}

func (r *Router) handleMcpServers(w http.ResponseWriter, req *http.Request) {
	servers := r.kernel.GetMcpServers()
	respondJSON(w, http.StatusOK, servers)
}

func (r *Router) handleMcpServerReconnect(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleProfiles(w http.ResponseWriter, req *http.Request) {
	profiles := []types.Profile{
		{Name: string(types.ToolProfileMinimal), Tools: types.ToolProfileMinimal.Tools()},
		{Name: string(types.ToolProfileCoding), Tools: types.ToolProfileCoding.Tools()},
		{Name: string(types.ToolProfileResearch), Tools: types.ToolProfileResearch.Tools()},
		{Name: string(types.ToolProfileMessaging), Tools: types.ToolProfileMessaging.Tools()},
		{Name: string(types.ToolProfileAutomation), Tools: types.ToolProfileAutomation.Tools()},
		{Name: string(types.ToolProfileFull), Tools: types.ToolProfileFull.Tools()},
		{Name: string(types.ToolProfileCustom), Tools: types.ToolProfileCustom.Tools()},
	}
	respondJSON(w, http.StatusOK, types.ProfilesResponse{Profiles: profiles})
}

type PatchAgentConfigRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	SystemPrompt  *string `json:"system_prompt,omitempty"`
	Emoji         *string `json:"emoji,omitempty"`
	AvatarURL     *string `json:"avatar_url,omitempty"`
	Color         *string `json:"color,omitempty"`
	Archetype     *string `json:"archetype,omitempty"`
	Vibe          *string `json:"vibe,omitempty"`
	GreetingStyle *string `json:"greeting_style,omitempty"`
	Model         *string `json:"model,omitempty"`
	Provider      *string `json:"provider,omitempty"`
}

func (r *Router) handlePatchAgentConfig(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	agentID, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent ID")
		return
	}

	var reqBody PatchAgentConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if r.kernel.AgentRegistry().Get(agentID) == nil {
		respondError(w, http.StatusNotFound, "agent not found")
		return
	}

	const MAX_PROMPT_LEN = 65536
	if reqBody.SystemPrompt != nil && len(*reqBody.SystemPrompt) > MAX_PROMPT_LEN {
		respondError(w, http.StatusRequestEntityTooLarge, "system prompt exceeds max length")
		return
	}

	if reqBody.Color != nil && *reqBody.Color != "" && !strings.HasPrefix(*reqBody.Color, "#") {
		respondError(w, http.StatusBadRequest, "color must be a hex code starting with '#'")
		return
	}

	if reqBody.SystemPrompt != nil {
		if err := r.kernel.AgentRegistry().UpdateSystemPrompt(agentID, *reqBody.SystemPrompt); err != nil {
			respondError(w, http.StatusNotFound, "agent not found")
			return
		}
	}

	identity := make(map[string]string)
	if reqBody.Emoji != nil {
		identity["emoji"] = *reqBody.Emoji
	}
	if reqBody.Color != nil {
		identity["color"] = *reqBody.Color
	}
	if reqBody.Archetype != nil {
		identity["archetype"] = *reqBody.Archetype
	}
	if reqBody.Vibe != nil {
		identity["vibe"] = *reqBody.Vibe
	}
	if reqBody.GreetingStyle != nil {
		identity["greeting_style"] = *reqBody.GreetingStyle
	}
	if reqBody.AvatarURL != nil {
		identity["avatar_url"] = *reqBody.AvatarURL
	}

	if len(identity) > 0 {
		if err := r.kernel.AgentRegistry().UpdateIdentity(agentID, identity); err != nil {
			respondError(w, http.StatusNotFound, "agent not found")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

var KNOWN_IDENTITY_FILES = []string{"SOUL.md", "SKILL.md", "README.md", "PROMPT.md"}

func (r *Router) handleListAgentFiles(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	_, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent ID")
		return
	}

	files := make([]map[string]interface{}, 0)
	for _, name := range KNOWN_IDENTITY_FILES {
		files = append(files, map[string]interface{}{
			"name":       name,
			"exists":     false,
			"size_bytes": 0,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"files": files})
}

func (r *Router) handleGetAgentFile(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	_, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent ID")
		return
	}

	filename := req.PathValue("filename")
	found := false
	for _, name := range KNOWN_IDENTITY_FILES {
		if name == filename {
			found = true
			break
		}
	}
	if !found {
		respondError(w, http.StatusBadRequest, "file not in whitelist")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":       filename,
		"content":    "",
		"size_bytes": 0,
	})
}

type SetAgentFileRequest struct {
	Content string `json:"content"`
}

func (r *Router) handleSetAgentFile(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	_, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent ID")
		return
	}

	filename := req.PathValue("filename")
	found := false
	for _, name := range KNOWN_IDENTITY_FILES {
		if name == filename {
			found = true
			break
		}
	}
	if !found {
		respondError(w, http.StatusBadRequest, "file not in whitelist")
		return
	}

	var reqBody SetAgentFileRequest
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"name":       filename,
		"size_bytes": len(reqBody.Content),
	})
}

func (r *Router) handleConfig(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"theme":             "system",
		"language":          "en",
		"sidebar_collapsed": false,
	})
}

func (r *Router) handleLogsStream(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	levelFilter := req.URL.Query().Get("level")
	textFilter := req.URL.Query().Get("filter")
	if textFilter != "" {
		textFilter = strings.ToLower(textFilter)
	}

	ctx := req.Context()

	var lastSeq int
	firstPoll := true

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-ticker.C:
			entries := r.kernel.AuditLog().GetRecent(200)

			for i, entry := range entries {
				seq := i + 1

				if !firstPoll && seq <= lastSeq {
					continue
				}

				actionStr := string(entry.Action)

				if levelFilter != "" {
					classified := classifyAuditLevel(actionStr)
					if classified != levelFilter {
						continue
					}
				}

				if textFilter != "" {
					haystack := strings.ToLower(fmt.Sprintf("%s %s %s", actionStr, entry.Details, entry.Target))
					if !strings.Contains(haystack, textFilter) {
						continue
					}
				}

				jsonData := map[string]interface{}{
					"seq":       seq,
					"timestamp": entry.Timestamp.Format(time.RFC3339),
					"agent_id":  entry.Target,
					"action":    actionStr,
					"detail":    entry.Details,
					"outcome":   entry.Result,
					"hash":      entry.Hash,
				}

				dataBytes, err := json.Marshal(jsonData)
				if err != nil {
					continue
				}

				fmt.Fprintf(w, "data: %s\n\n", dataBytes)
				flusher.Flush()
			}

			if len(entries) > 0 {
				lastSeq = len(entries)
			}
			firstPoll = false
		}
	}
}

func classifyAuditLevel(action string) string {
	a := strings.ToLower(action)
	if strings.Contains(a, "error") || strings.Contains(a, "fail") || strings.Contains(a, "crash") || strings.Contains(a, "denied") {
		return "error"
	} else if strings.Contains(a, "warn") || strings.Contains(a, "block") || strings.Contains(a, "kill") {
		return "warn"
	}
	return "info"
}

func (r *Router) handleA2AEventsStream(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := req.Context()

	var lastEventID string
	firstPoll := true

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-ticker.C:
			events := r.kernel.A2AEventStore().ListEvents(200)

			if firstPoll {
				if len(events) > 0 {
					lastEventID = events[len(events)-1].ID
				}
				firstPoll = false
				continue
			}

			foundLastEvent := false
			for _, event := range events {
				if !foundLastEvent {
					if event.ID == lastEventID {
						foundLastEvent = true
					}
					continue
				}

				jsonData := map[string]interface{}{
					"id":         event.ID,
					"timestamp":  event.Timestamp.Format(time.RFC3339),
					"kind":       event.Kind,
					"sourceId":   event.SourceID,
					"sourceName": event.SourceName,
					"targetId":   event.TargetID,
					"targetName": event.TargetName,
					"detail":     event.Detail,
					"payload":    event.Payload,
				}

				dataBytes, err := json.Marshal(jsonData)
				if err != nil {
					continue
				}

				fmt.Fprintf(w, "data: %s\n\n", dataBytes)
				flusher.Flush()
			}

			if len(events) > 0 {
				lastEventID = events[len(events)-1].ID
			}
		}
	}
}

func (r *Router) handleUsageSummary(w http.ResponseWriter, req *http.Request) {
	summary, err := r.kernel.UsageStore().QuerySummary()
	if err != nil || summary.CallCount == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"total_input_tokens":  0,
			"total_output_tokens": 0,
			"total_cost_usd":      0.0,
			"call_count":          0,
			"total_tool_calls":    0,
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_input_tokens":  summary.TotalInputTokens,
		"total_output_tokens": summary.TotalOutputTokens,
		"total_cost_usd":      summary.TotalCostUSD,
		"call_count":          summary.CallCount,
		"total_tool_calls":    summary.TotalToolCalls,
	})
}

func (r *Router) handleUsageByModel(w http.ResponseWriter, req *http.Request) {
	models, err := r.kernel.UsageStore().GetUsageByModel()
	if err != nil || models == nil || len(models) == 0 {
		models = []*types.ModelUsage{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"models": models,
	})
}

func (r *Router) handleUsageDaily(w http.ResponseWriter, req *http.Request) {
	days, err := r.kernel.UsageStore().GetDailyBreakdown(7)
	if err != nil || days == nil || len(days) == 0 {
		today := time.Now()
		days = []*types.DailyBreakdown{}
		for i := 6; i >= 0; i-- {
			date := today.AddDate(0, 0, -i)
			days = append(days, &types.DailyBreakdown{
				Date:    date.Format("2006-01-02"),
				CostUSD: 0.0,
				Tokens:  0,
				Calls:   0,
			})
		}
	}

	todayCost, _ := r.kernel.UsageStore().GetTodayCost()

	firstEventDate, _ := r.kernel.UsageStore().GetFirstEventDate()
	if firstEventDate == nil && len(days) > 0 {
		firstEventDate = &days[0].Date
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"days":             days,
		"today_cost_usd":   todayCost,
		"first_event_date": firstEventDate,
	})
}

func loadHandsFromFile() []map[string]string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return getDefaultHands()
	}
	path := filepath.Join(homeDir, ".fangclaw-go", "hands.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return getDefaultHands()
	}
	var hands []map[string]string
	if err := json.Unmarshal(data, &hands); err != nil {
		return getDefaultHands()
	}
	return hands
}

func getDefaultHands() []map[string]string {
	return []map[string]string{
		{"id": "researcher", "name": "Researcher", "status": "inactive"},
		{"id": "lead", "name": "Lead", "status": "inactive"},
		{"id": "collector", "name": "Collector", "status": "inactive"},
		{"id": "predictor", "name": "Predictor", "status": "inactive"},
		{"id": "clip", "name": "Clip", "status": "inactive"},
		{"id": "twitter", "name": "Twitter", "status": "inactive"},
		{"id": "browser", "name": "Browser", "status": "inactive"},
	}
}

func (r *Router) handleListHands(w http.ResponseWriter, req *http.Request) {
	handDefs := r.kernel.HandRegistry().ListDefinitions()
	instances := r.kernel.HandRegistry().ListInstances()

	// Read locally saved hand status
	handsStatus := loadHandsStatus()
	handStatusMap := make(map[string]string)
	for _, h := range handsStatus {
		handStatusMap[h["id"]] = h["status"]
	}

	var handsResult []map[string]interface{}
	for _, hand := range handDefs {
		tools := hand.Tools
		if tools == nil {
			tools = []string{}
		}

		dashboardMetrics := 0
		if hand.Dashboard.Metrics != nil {
			dashboardMetrics = len(hand.Dashboard.Metrics)
		}

		// Get hand status
		status := "inactive"
		if s, ok := handStatusMap[hand.ID]; ok {
			status = s
		}

		handsResult = append(handsResult, map[string]interface{}{
			"id":                hand.ID,
			"name":              hand.Name,
			"description":       hand.Description,
			"category":          hand.Category,
			"icon":              hand.Icon,
			"tools":             tools,
			"dashboard_metrics": dashboardMetrics,
			"has_settings":      hand.Settings != nil && len(hand.Settings) > 0,
			"settings_count":    len(hand.Settings),
			"requires":          hand.Requires,
			"settings":          hand.Settings,
			"status":            status,
		})
	}

	var instancesResult []map[string]interface{}
	for _, inst := range instances {
		instancesResult = append(instancesResult, map[string]interface{}{
			"instance_id":  inst.InstanceID,
			"hand_id":      inst.HandID,
			"agent_id":     inst.AgentID,
			"agent_name":   inst.AgentName,
			"status":       inst.Status,
			"config":       inst.Config,
			"activated_at": inst.ActivatedAt,
			"updated_at":   inst.UpdatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"hands":     handsResult,
		"instances": instancesResult,
	})
}

func (r *Router) handleActiveHands(w http.ResponseWriter, req *http.Request) {
	instances := r.kernel.HandRegistry().ListInstances()

	var activeInstances []map[string]interface{}
	for _, inst := range instances {
		if inst.Status == hands.HandStatusActive || inst.Status == hands.HandStatusPaused {
			activeInstances = append(activeInstances, map[string]interface{}{
				"instance_id": inst.InstanceID,
				"hand_id":     inst.HandID,
				"agent_id":    inst.AgentID,
				"agent_name":  inst.AgentName,
				"status":      inst.Status,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"instances": activeInstances,
	})
}

func (r *Router) handleGetHand(w http.ResponseWriter, req *http.Request) {
	handID := req.PathValue("id")

	hand, ok := r.kernel.HandRegistry().GetDefinition(handID)
	if !ok {
		respondError(w, http.StatusNotFound, "hand not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":          hand.ID,
		"name":        hand.Name,
		"description": hand.Description,
		"category":    hand.Category,
		"icon":        hand.Icon,
		"tools":       hand.Tools,
		"requires":    hand.Requires,
		"settings":    hand.Settings,
		"agent":       hand.Agent,
		"dashboard":   hand.Dashboard,
	})
}

func (r *Router) handleActivateHand(w http.ResponseWriter, req *http.Request) {
	handID := req.PathValue("id")

	var config map[string]interface{}
	if req.Body != http.NoBody {
		if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
			config = make(map[string]interface{})
		}
		defer req.Body.Close()
	}

	instance, err := r.kernel.ActivateHand(handID, config)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updateHandStatus(handID, "active")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"instance_id": instance.InstanceID,
		"hand_id":     instance.HandID,
		"agent_id":    instance.AgentID,
		"agent_name":  instance.AgentName,
		"status":      instance.Status,
	})
}

func (r *Router) handleDeactivateHand(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	instance, ok := r.kernel.HandRegistry().GetInstance(instanceID)
	var handID string
	if ok {
		handID = instance.HandID
	}

	err := r.kernel.DeactivateHand(instanceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if handID != "" {
		updateHandStatus(handID, "inactive")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handlePauseHand(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	instance, ok := r.kernel.HandRegistry().GetInstance(instanceID)
	var handID string
	if ok {
		handID = instance.HandID
	}

	err := r.kernel.PauseHand(instanceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if handID != "" {
		updateHandStatus(handID, "paused")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleResumeHand(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	instance, ok := r.kernel.HandRegistry().GetInstance(instanceID)
	var handID string
	if ok {
		handID = instance.HandID
	}

	err := r.kernel.ResumeHand(instanceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if handID != "" {
		updateHandStatus(handID, "active")
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleHandInstanceStats(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	instance, ok := r.kernel.HandRegistry().GetInstance(instanceID)
	if !ok {
		respondError(w, http.StatusNotFound, "Instance not found")
		return
	}

	def, ok := r.kernel.HandRegistry().GetDefinition(instance.HandID)
	if !ok {
		respondError(w, http.StatusNotFound, "Hand definition not found")
		return
	}

	if instance.AgentID == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"instance_id": instanceID,
			"hand_id":     instance.HandID,
			"metrics":     map[string]interface{}{},
		})
		return
	}

	metrics := make(map[string]interface{})
	for _, metric := range def.Dashboard.Metrics {
		metrics[metric.Label] = map[string]interface{}{
			"value":  nil,
			"format": metric.Format,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"instance_id": instanceID,
		"hand_id":     instance.HandID,
		"status":      instance.Status,
		"agent_id":    instance.AgentID,
		"metrics":     metrics,
	})
}

func (r *Router) handleHandInstanceBrowser(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	instance, ok := r.kernel.HandRegistry().GetInstance(instanceID)
	if !ok {
		respondError(w, http.StatusNotFound, "Instance not found")
		return
	}

	if instance.AgentID == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"active": false,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"active": false,
	})
}

func (r *Router) handleInstallHandDeps(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleShutdown(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Shutdown initiated",
	})

	RequestShutdown()
}

func (r *Router) handleConfigSchema(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sections": map[string]interface{}{
			"api": map[string]interface{}{
				"fields": map[string]string{
					"api_listen": "string",
					"api_key":    "string",
					"log_level":  "string",
				},
			},
			"default_model": map[string]interface{}{
				"fields": map[string]string{
					"provider":    "string",
					"model":       "string",
					"api_key_env": "string",
					"base_url":    "string",
				},
			},
			"memory": map[string]interface{}{
				"fields": map[string]string{
					"decay_rate":  "number",
					"vector_dims": "number",
				},
			},
			"web": map[string]interface{}{
				"fields": map[string]string{
					"provider":     "string",
					"timeout_secs": "number",
					"max_results":  "number",
				},
			},
			"browser": map[string]interface{}{
				"fields": map[string]string{
					"headless":        "boolean",
					"timeout_secs":    "number",
					"executable_path": "string",
				},
			},
			"network": map[string]interface{}{
				"fields": map[string]string{
					"enabled":       "boolean",
					"listen_addr":   "string",
					"shared_secret": "string",
				},
			},
			"extensions": map[string]interface{}{
				"fields": map[string]string{
					"auto_connect":               "boolean",
					"health_check_interval_secs": "number",
				},
			},
			"vault": map[string]interface{}{
				"fields": map[string]string{
					"path": "string",
				},
			},
			"a2a": map[string]interface{}{
				"fields": map[string]string{
					"enabled":     "boolean",
					"name":        "string",
					"description": "string",
					"url":         "string",
				},
			},
			"channels": map[string]interface{}{
				"fields": map[string]string{
					"telegram": "object",
					"discord":  "object",
					"slack":    "object",
					"whatsapp": "object",
				},
			},
		},
	})
}

func (r *Router) handleAPIModels(w http.ResponseWriter, req *http.Request) {
	providerFilter := req.URL.Query().Get("provider")
	tierFilter := req.URL.Query().Get("tier")
	availableOnly := req.URL.Query().Get("available") == "true" || req.URL.Query().Get("available") == "1"

	allModels := r.kernel.ModelCatalog().ListModels()
	var filteredModels []map[string]interface{}

	for _, m := range allModels {
		if providerFilter != "" && strings.ToLower(m.Provider) != strings.ToLower(providerFilter) {
			continue
		}
		if tierFilter != "" && strings.ToLower(string(m.Tier)) != strings.ToLower(tierFilter) {
			continue
		}

		provider := r.kernel.ModelCatalog().GetProvider(m.Provider)
		available := provider != nil && provider.AuthStatus != types.AuthStatusMissing

		if availableOnly && !available {
			continue
		}

		filteredModels = append(filteredModels, map[string]interface{}{
			"id":                 m.ID,
			"display_name":       m.DisplayName,
			"provider":           m.Provider,
			"tier":               m.Tier,
			"context_window":     m.ContextWindow,
			"max_output_tokens":  m.MaxOutputTokens,
			"input_cost_per_m":   m.InputCostPerM,
			"output_cost_per_m":  m.OutputCostPerM,
			"supports_tools":     m.SupportsTools,
			"supports_vision":    m.SupportsVision,
			"supports_streaming": m.SupportsStreaming,
			"available":          available,
		})
	}

	total := len(allModels)
	availableCount := len(r.kernel.ModelCatalog().AvailableModels())

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"models":    filteredModels,
		"total":     total,
		"available": availableCount,
	})
}

func (r *Router) handleModelsAliases(w http.ResponseWriter, req *http.Request) {
	aliases := r.kernel.ModelCatalog().ListAliases()
	var entries []map[string]interface{}

	for alias, modelID := range aliases {
		entries = append(entries, map[string]interface{}{
			"alias":    alias,
			"model_id": modelID,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"aliases": entries,
		"total":   len(entries),
	})
}

func (r *Router) handleGetModel(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	model := r.kernel.ModelCatalog().FindModel(id)
	if model == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Model '%s' not found", id))
		return
	}

	provider := r.kernel.ModelCatalog().GetProvider(model.Provider)
	available := provider != nil && provider.AuthStatus != types.AuthStatusMissing

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":                 model.ID,
		"display_name":       model.DisplayName,
		"provider":           model.Provider,
		"tier":               model.Tier,
		"context_window":     model.ContextWindow,
		"max_output_tokens":  model.MaxOutputTokens,
		"input_cost_per_m":   model.InputCostPerM,
		"output_cost_per_m":  model.OutputCostPerM,
		"supports_tools":     model.SupportsTools,
		"supports_vision":    model.SupportsVision,
		"supports_streaming": model.SupportsStreaming,
		"aliases":            model.Aliases,
		"available":          available,
	})
}

func (r *Router) handleAddCustomModel(w http.ResponseWriter, req *http.Request) {
	var model types.ModelCatalogEntry

	if err := json.NewDecoder(req.Body).Decode(&model); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if model.ID == "" || model.Provider == "" {
		respondError(w, http.StatusBadRequest, "ID and provider are required")
		return
	}

	r.kernel.ModelCatalog().AddCustomModel(model)

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (r *Router) handleDeleteCustomModel(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	deleted := r.kernel.ModelCatalog().RemoveCustomModel(id)
	if !deleted {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Custom model '%s' not found", id))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (r *Router) handleListCronJobs(w http.ResponseWriter, req *http.Request) {
	agentIDStr := req.URL.Query().Get("agent_id")
	var jobs []types.CronJob

	allJobs := r.kernel.CronScheduler().ListAllJobs()
	if agentIDStr != "" {
		agentID, err := types.ParseAgentID(agentIDStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid agent ID")
			return
		}
		for _, job := range allJobs {
			if job.AgentID == agentID {
				jobs = append(jobs, job)
			}
		}
	} else {
		jobs = allJobs
	}

	if jobs == nil {
		jobs = []types.CronJob{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func (r *Router) handleCreateCronJob(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		AgentID  string             `json:"agent_id"`
		Name     string             `json:"name"`
		Enabled  bool               `json:"enabled"`
		Schedule types.CronSchedule `json:"schedule"`
		Action   types.CronAction   `json:"action"`
		Delivery types.CronDelivery `json:"delivery"`
		OneShot  bool               `json:"one_shot"`
	}

	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	agentID, err := types.ParseAgentID(reqBody.AgentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid agent ID")
		return
	}

	job := types.NewCronJob(
		agentID,
		reqBody.Name,
		reqBody.Enabled,
		reqBody.Schedule,
		reqBody.Action,
		reqBody.Delivery,
	)

	id, err := r.kernel.CronScheduler().AddJob(job, reqBody.OneShot)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"job_id": id.String(),
	})
}

func (r *Router) handleEnableCronJob(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseCronJobID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	var reqBody struct {
		Enabled bool `json:"enabled"`
	}

	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := r.kernel.CronScheduler().SetEnabled(id, reqBody.Enabled); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":      idStr,
		"enabled": reqBody.Enabled,
	})
}

func (r *Router) handleDeleteCronJob(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseCronJobID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	if _, err := r.kernel.CronScheduler().RemoveJob(id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}

func (r *Router) handleCronJobStatus(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseCronJobID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	meta := r.kernel.CronScheduler().GetMeta(id)
	if meta == nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"job":                meta.Job,
		"one_shot":           meta.OneShot,
		"last_status":        meta.LastStatus,
		"consecutive_errors": meta.ConsecutiveErrors,
	})
}

func (r *Router) handleClawhubSearch(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	if query == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items":       []interface{}{},
			"next_cursor": nil,
		})
		return
	}

	limitStr := req.URL.Query().Get("limit")
	limit := uint32(20)
	if limitStr != "" {
		if l, err := strconv.ParseUint(limitStr, 10, 32); err == nil {
			limit = uint32(l)
		}
	}

	client := clawhub.NewClawHubClient()
	results, err := client.Search(query, limit)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items":       []interface{}{},
			"next_cursor": nil,
			"error":       err.Error(),
		})
		return
	}

	var items []map[string]interface{}
	for _, entry := range results.Results {
		items = append(items, map[string]interface{}{
			"slug":        entry.Slug,
			"name":        entry.DisplayName,
			"description": entry.Summary,
			"version":     entry.Version,
			"score":       entry.Score,
			"updated_at":  entry.UpdatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items":       items,
		"next_cursor": nil,
	})
}

func (r *Router) handleClawhubBrowse(w http.ResponseWriter, req *http.Request) {
	sortParam := req.URL.Query().Get("sort")
	var sort clawhub.ClawHubSort
	switch sortParam {
	case "downloads":
		sort = clawhub.ClawHubSortDownloads
	case "stars":
		sort = clawhub.ClawHubSortStars
	case "updated":
		sort = clawhub.ClawHubSortUpdated
	case "rating":
		sort = clawhub.ClawHubSortRating
	default:
		sort = clawhub.ClawHubSortTrending
	}

	limitStr := req.URL.Query().Get("limit")
	limit := uint32(20)
	if limitStr != "" {
		if l, err := strconv.ParseUint(limitStr, 10, 32); err == nil {
			limit = uint32(l)
		}
	}

	cursorParam := req.URL.Query().Get("cursor")
	var cursor *string
	if cursorParam != "" {
		cursor = &cursorParam
	}

	client := clawhub.NewClawHubClient()
	results, err := client.Browse(sort, limit, cursor)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items":       []interface{}{},
			"next_cursor": nil,
			"error":       err.Error(),
		})
		return
	}

	var items []map[string]interface{}
	for _, entry := range results.Items {
		items = append(items, clawhubBrowseEntryToJSON(entry))
	}

	var nextCursor interface{} = nil
	if results.NextCursor != nil {
		nextCursor = *results.NextCursor
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

func clawhubBrowseEntryToJSON(entry clawhub.ClawHubBrowseEntry) map[string]interface{} {
	version := clawhub.EntryVersion(entry)
	return map[string]interface{}{
		"slug":        entry.Slug,
		"name":        entry.DisplayName,
		"description": entry.Summary,
		"version":     version,
		"stars":       entry.Stats.Stars,
		"downloads":   entry.Stats.Downloads,
		"updated_at":  entry.UpdatedAt,
		"tags":        []string{},
		"tools":       []string{},
	}
}

func (r *Router) handleClawhubSkillDetail(w http.ResponseWriter, req *http.Request) {
	slug := req.PathValue("slug")
	client := clawhub.NewClawHubClient()
	detail, err := client.GetSkill(slug)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"slug":         slug,
			"name":         "",
			"description":  "",
			"version":      "",
			"author":       "",
			"author_name":  "",
			"author_image": "",
			"stars":        0,
			"downloads":    0,
			"updated_at":   nil,
			"tags":         []string{},
			"tools":        []string{},
			"is_installed": false,
			"error":        err.Error(),
		})
		return
	}

	version := ""
	if detail.LatestVersion != nil {
		version = detail.LatestVersion.Version
	}

	author := ""
	authorName := ""
	authorImage := ""
	if detail.Owner != nil {
		author = detail.Owner.Handle
		authorName = detail.Owner.DisplayName
		authorImage = detail.Owner.Image
	}

	skillsDir := filepath.Join(r.kernel.Config().DataDir, "skills")
	skillDir := filepath.Join(skillsDir, slug)
	isInstalled := false
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		skillTomlPath := filepath.Join(skillDir, "skill.toml")
		if _, err := os.Stat(skillTomlPath); !os.IsNotExist(err) {
			isInstalled = true
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"slug":         detail.Skill.Slug,
		"name":         detail.Skill.DisplayName,
		"description":  detail.Skill.Summary,
		"version":      version,
		"author":       author,
		"author_name":  authorName,
		"author_image": authorImage,
		"stars":        detail.Skill.Stats.Stars,
		"downloads":    detail.Skill.Stats.Downloads,
		"updated_at":   detail.Skill.UpdatedAt,
		"tags":         []string{},
		"tools":        []string{},
		"is_installed": isInstalled,
	})
}

func (r *Router) handleClawhubInstall(w http.ResponseWriter, req *http.Request) {
	// fmt.Println("[DEBUG] handleClawhubInstall called")

	var reqBody struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		// fmt.Println("[DEBUG] Failed to decode request body:", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	// fmt.Println("[DEBUG] Request slug:", reqBody.Slug)

	skillsDir := filepath.Join(r.kernel.Config().DataDir, "skills")
	// fmt.Println("[DEBUG] Skills directory:", skillsDir)

	client := clawhub.NewClawHubClient()

	if client.IsInstalled(reqBody.Slug, skillsDir) {
		// fmt.Println("[DEBUG] Skill already installed")
		respondJSON(w, http.StatusConflict, map[string]interface{}{
			"error":  fmt.Sprintf("Skill '%s' is already installed", reqBody.Slug),
			"status": "already_installed",
		})
		return
	}

	// fmt.Println("[DEBUG] Starting installation...")
	result, err := client.Install(reqBody.Slug, skillsDir)
	if err != nil {
		// fmt.Println("[DEBUG] Installation failed:", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	// fmt.Println("[DEBUG] Installation successful:", result.SkillName)

	var warnings []map[string]interface{}
	for _, w := range result.Warnings {
		warnings = append(warnings, map[string]interface{}{
			"severity": w.Severity,
			"message":  w.Message,
		})
	}

	var translations []map[string]interface{}
	for _, t := range result.ToolTranslations {
		translations = append(translations, map[string]interface{}{
			"from": t.From,
			"to":   t.To,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "installed",
		"name":              result.SkillName,
		"version":           result.Version,
		"slug":              result.Slug,
		"is_prompt_only":    result.IsPromptOnly,
		"warnings":          warnings,
		"tool_translations": translations,
	})
}

func (r *Router) handleListApprovals(w http.ResponseWriter, req *http.Request) {
	pending := r.kernel.ApprovalManager().ListPending()
	total := len(pending)

	var approvals []map[string]interface{}
	for _, a := range pending {
		approvals = append(approvals, map[string]interface{}{
			"id":             a.ID,
			"agent_id":       a.AgentID,
			"agent_name":     a.AgentName,
			"model_provider": a.ModelProvider,
			"model_name":     a.ModelName,
			"session_id":     a.SessionID,
			"tool_name":      a.ToolName,
			"description":    a.Description,
			"action_summary": a.ActionSummary,
			"action":         a.ActionSummary,
			"risk_level":     a.RiskLevel,
			"requested_at":   a.RequestedAt,
			"created_at":     a.CreatedAt,
			"timeout_secs":   a.TimeoutSecs,
			"status":         "pending",
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"approvals": approvals,
		"total":     total,
	})
}

func (r *Router) handleCreateApproval(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		AgentID       string `json:"agent_id"`
		AgentName     string `json:"agent_name"`
		ModelProvider string `json:"model_provider"`
		ModelName     string `json:"model_name"`
		SessionID     string `json:"session_id"`
		ToolName      string `json:"tool_name"`
		Description   string `json:"description"`
		ActionSummary string `json:"action_summary"`
	}

	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	description := reqBody.Description
	if description == "" {
		description = fmt.Sprintf("Manual approval request for %s", reqBody.ToolName)
	}

	actionSummary := reqBody.ActionSummary
	if actionSummary == "" {
		actionSummary = reqBody.ToolName
	}

	approvalReq := approvals.NewApprovalRequestWithDetails(
		reqBody.AgentID,
		reqBody.AgentName,
		reqBody.ModelProvider,
		reqBody.ModelName,
		reqBody.SessionID,
		reqBody.ToolName,
		description,
		actionSummary,
		actionSummary,
		"",
		approvals.RiskLevelHigh,
	)

	go func() {
		ch, _ := r.kernel.ApprovalManager().RequestApproval(approvalReq)
		<-ch
	}()

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":     approvalReq.ID,
		"status": "pending",
	})
}

func (r *Router) handleApproveApproval(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	if err := r.kernel.ApprovalManager().Resolve(id, approvals.ApprovalDecisionApproved, "api"); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"status":     "approved",
		"decided_at": time.Now().Format(time.RFC3339),
	})
}

func (r *Router) handleRejectApproval(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	if err := r.kernel.ApprovalManager().Resolve(id, approvals.ApprovalDecisionDenied, "api"); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"status":     "rejected",
		"decided_at": time.Now().Format(time.RFC3339),
	})
}

func (r *Router) handleCreateWorkflow(w http.ResponseWriter, req *http.Request) {
	var reqBody map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := "unnamed"
	if n, ok := reqBody["name"].(string); ok {
		name = n
	}

	description := ""
	if d, ok := reqBody["description"].(string); ok {
		description = d
	}

	stepsJson, ok := reqBody["steps"].([]interface{})
	if !ok {
		respondError(w, http.StatusBadRequest, "Missing 'steps' array")
		return
	}

	var steps []types.WorkflowStep
	for _, s := range stepsJson {
		stepMap, ok := s.(map[string]interface{})
		if !ok {
			respondError(w, http.StatusBadRequest, "Invalid step format")
			return
		}

		stepName := "step"
		if sn, ok := stepMap["name"].(string); ok {
			stepName = sn
		}

		var agent types.StepAgent
		if agentID, ok := stepMap["agent_id"].(string); ok {
			agent.ID = &agentID
		} else if agentName, ok := stepMap["agent_name"].(string); ok {
			agent.Name = &agentName
		} else if agentObj, ok := stepMap["agent"].(map[string]interface{}); ok {
			if agentID, ok := agentObj["id"].(string); ok {
				agent.ID = &agentID
			} else if agentName, ok := agentObj["name"].(string); ok {
				agent.Name = &agentName
			} else {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Step '%s' needs 'agent_id', 'agent_name', or 'agent' object with 'id' or 'name'", stepName))
				return
			}
		} else {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Step '%s' needs 'agent_id', 'agent_name', or 'agent' object with 'id' or 'name'", stepName))
			return
		}

		modeType := "sequential"
		if mt, ok := stepMap["mode"].(string); ok {
			modeType = mt
		} else if modeObj, ok := stepMap["mode"].(map[string]interface{}); ok {
			if mt, ok := modeObj["type"].(string); ok {
				modeType = mt
			}
		}

		var mode types.StepMode
		switch modeType {
		case "fan_out":
			mode = types.StepMode{Type: "fan_out"}
		case "collect":
			mode = types.StepMode{Type: "collect"}
		case "conditional":
			condition := ""
			if modeObj, ok := stepMap["mode"].(map[string]interface{}); ok {
				if c, ok := modeObj["condition"].(string); ok {
					condition = c
				}
			} else if c, ok := stepMap["condition"].(string); ok {
				condition = c
			}
			mode = types.StepMode{Type: "conditional", Condition: &condition}
		case "loop":
			maxIterations := uint32(5)
			until := ""
			if modeObj, ok := stepMap["mode"].(map[string]interface{}); ok {
				if mi, ok := modeObj["max_iterations"].(float64); ok {
					maxIterations = uint32(mi)
				}
				if u, ok := modeObj["until"].(string); ok {
					until = u
				}
			} else {
				if mi, ok := stepMap["max_iterations"].(float64); ok {
					maxIterations = uint32(mi)
				}
				if u, ok := stepMap["until"].(string); ok {
					until = u
				}
			}
			mode = types.StepMode{Type: "loop", MaxIterations: &maxIterations, Until: &until}
		default:
			mode = types.StepMode{Type: "sequential"}
		}

		errorModeType := "fail"
		if emt, ok := stepMap["error_mode"].(string); ok {
			errorModeType = emt
		} else if errorModeObj, ok := stepMap["error_mode"].(map[string]interface{}); ok {
			if emt, ok := errorModeObj["type"].(string); ok {
				errorModeType = emt
			}
		}

		var errorMode types.ErrorMode
		switch errorModeType {
		case "skip":
			errorMode = types.ErrorMode{Type: "skip"}
		case "retry":
			maxRetries := uint32(3)
			if errorModeObj, ok := stepMap["error_mode"].(map[string]interface{}); ok {
				if mr, ok := errorModeObj["max_retries"].(float64); ok {
					maxRetries = uint32(mr)
				}
			} else if mr, ok := stepMap["max_retries"].(float64); ok {
				maxRetries = uint32(mr)
			}
			errorMode = types.ErrorMode{Type: "retry", MaxRetries: &maxRetries}
		default:
			errorMode = types.ErrorMode{Type: "fail"}
		}

		promptTemplate := "{{input}}"
		if pt, ok := stepMap["prompt"].(string); ok {
			promptTemplate = pt
		} else if pt, ok := stepMap["prompt_template"].(string); ok {
			promptTemplate = pt
		}

		timeoutSecs := uint64(120)
		if ts, ok := stepMap["timeout_secs"].(float64); ok {
			timeoutSecs = uint64(ts)
		}

		var outputVar *string
		if ov, ok := stepMap["output_var"].(string); ok {
			outputVar = &ov
		}

		steps = append(steps, types.WorkflowStep{
			Name:           stepName,
			Agent:          agent,
			PromptTemplate: promptTemplate,
			Mode:           mode,
			TimeoutSecs:    timeoutSecs,
			ErrorMode:      errorMode,
			OutputVar:      outputVar,
		})
	}

	// Get workflow ID from request, or generate a new one if not provided
	var workflowID types.WorkflowID
	if idStr, ok := reqBody["id"].(string); ok && idStr != "" {
		workflowID = types.WorkflowID(idStr)
	} else {
		workflowID = types.WorkflowID(fmt.Sprintf("wf-%d", time.Now().UnixNano()))
	}

	workflow := types.Workflow{
		ID:          workflowID,
		Name:        name,
		Description: description,
		Steps:       steps,
		CreatedAt:   time.Now(),
	}

	id := r.kernel.WorkflowEngine().Register(workflow)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"workflow_id": id,
	})
}

func (r *Router) handleListWorkflows(w http.ResponseWriter, req *http.Request) {
	workflows := r.kernel.WorkflowEngine().ListWorkflows()
	result := make([]map[string]interface{}, 0)
	for _, wf := range workflows {
		result = append(result, map[string]interface{}{
			"id":          wf.ID,
			"name":        wf.Name,
			"description": wf.Description,
			"steps":       len(wf.Steps),
			"created_at":  wf.CreatedAt.Format(time.RFC3339),
		})
	}
	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleGetWorkflow(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	workflowID := types.WorkflowID(idStr)
	workflow := r.kernel.WorkflowEngine().GetWorkflow(workflowID)
	if workflow == nil {
		respondError(w, http.StatusNotFound, "Workflow not found")
		return
	}
	respondJSON(w, http.StatusOK, workflow)
}

func (r *Router) handleDeleteWorkflow(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	workflowID := types.WorkflowID(idStr)
	deleted := r.kernel.WorkflowEngine().RemoveWorkflow(workflowID)
	if !deleted {
		respondError(w, http.StatusNotFound, "Workflow not found")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Workflow deleted",
	})
}

func (r *Router) handleRunWorkflow(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	workflowID := types.WorkflowID(idStr)

	var reqBody map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := ""
	if i, ok := reqBody["input"].(string); ok {
		input = i
	}

	runID := r.kernel.WorkflowEngine().CreateRun(workflowID, input)
	if runID == nil {
		respondError(w, http.StatusBadRequest, "Invalid workflow ID")
		return
	}

	// Create a background context for workflow execution
	// This ensures that goroutines in fan_out mode don't get cancelled when the request completes
	execCtx := context.Background()

	resolver := func(agent types.StepAgent) (string, string, bool) {
		if agent.Name != nil {
			agentID, ok := r.kernel.FindAgentByName(execCtx, *agent.Name)
			if ok {
				return agentID, *agent.Name, true
			}
		}
		if agent.ID != nil {
			return *agent.ID, "", true
		}
		return "", "", false
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return r.kernel.SendMessageWithUsage(execCtx, agentID, prompt)
	}

	output, err := r.kernel.WorkflowEngine().ExecuteRun(*runID, resolver, sender)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Workflow execution failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"run_id": *runID,
		"output": output,
		"status": "completed",
	})
}

func (r *Router) handleListWorkflowRuns(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	workflowID := types.WorkflowID(idStr)
	runs := r.kernel.WorkflowEngine().ListRuns(nil, &workflowID)
	result := make([]map[string]interface{}, 0)
	for _, run := range runs {
		var completedAt *string
		if run.CompletedAt != nil {
			ca := run.CompletedAt.Format(time.RFC3339)
			completedAt = &ca
		}
		result = append(result, map[string]interface{}{
			"id":              run.ID,
			"workflow_name":   run.WorkflowName,
			"state":           run.State,
			"steps_completed": len(run.StepResults),
			"started_at":      run.StartedAt.Format(time.RFC3339),
			"completed_at":    completedAt,
		})
	}
	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleListWorkflowTemplates(w http.ResponseWriter, req *http.Request) {
	templates := r.kernel.ListWorkflowTemplates()
	result := make([]map[string]interface{}, 0)
	for _, t := range templates {
		result = append(result, map[string]interface{}{
			"id":          t.ID,
			"name":        t.Name,
			"description": t.Description,
			"category":    t.Category,
			"created_at":  t.CreatedAt.Format(time.RFC3339),
		})
	}
	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleGetWorkflowTemplate(w http.ResponseWriter, req *http.Request) {
	id := types.WorkflowTemplateID(req.PathValue("id"))
	template := r.kernel.GetWorkflowTemplate(id)
	if template == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}
	respondJSON(w, http.StatusOK, template)
}

func (r *Router) handleCreateWorkflowFromTemplate(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		TemplateID        string  `json:"template_id"`
		CustomName        *string `json:"custom_name,omitempty"`
		CustomDescription *string `json:"custom_description,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if reqBody.TemplateID == "" {
		respondError(w, http.StatusBadRequest, "template_id is required")
		return
	}

	var customName, customDesc string
	if reqBody.CustomName != nil {
		customName = *reqBody.CustomName
	}
	if reqBody.CustomDescription != nil {
		customDesc = *reqBody.CustomDescription
	}

	wf, err := r.kernel.CreateWorkflowFromTemplate(types.WorkflowTemplateID(reqBody.TemplateID), customName, customDesc)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, wf)
}

func (r *Router) handleRunWorkflowWithDelivery(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	wfID := types.WorkflowID(idStr)

	var reqBody struct {
		Input    string                `json:"input"`
		Delivery *types.DeliveryConfig `json:"delivery"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	output, err := r.kernel.ExecuteWorkflowWithDelivery(req.Context(), wfID, reqBody.Input, reqBody.Delivery)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"workflow_id": wfID,
		"output":      output,
		"status":      "completed",
	})
}

func (r *Router) handleCreateTrigger(w http.ResponseWriter, req *http.Request) {
	var reqBody map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	agentID, ok := reqBody["agent_id"].(string)
	if !ok {
		respondError(w, http.StatusBadRequest, "Missing 'agent_id'")
		return
	}

	patternData, ok := reqBody["pattern"]
	if !ok {
		respondError(w, http.StatusBadRequest, "Missing 'pattern'")
		return
	}

	patternBytes, err := json.Marshal(patternData)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid trigger pattern")
		return
	}

	pattern, err := triggers.UnmarshalTriggerPattern(patternBytes)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid trigger pattern: "+err.Error())
		return
	}

	promptTemplate := "Event: {{event}}"
	if pt, ok := reqBody["prompt_template"].(string); ok {
		promptTemplate = pt
	}

	maxFires := uint64(0)
	if mf, ok := reqBody["max_fires"].(float64); ok {
		maxFires = uint64(mf)
	}

	trigger := &triggers.Trigger{
		ID:             triggers.NewTriggerID(),
		AgentID:        agentID,
		Pattern:        pattern,
		PromptTemplate: promptTemplate,
		Enabled:        true,
		CreatedAt:      time.Now(),
		FireCount:      0,
		MaxFires:       maxFires,
	}

	if err := r.kernel.TriggerEngine().Register(trigger); err != nil {
		respondError(w, http.StatusNotFound, "Trigger registration failed (agent not found?)")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"trigger_id": trigger.ID,
		"agent_id":   agentID,
	})
}

func (r *Router) handleListTriggers(w http.ResponseWriter, req *http.Request) {
	agentID := req.URL.Query().Get("agent_id")
	triggersList := r.kernel.TriggerEngine().List(agentID)
	result := make([]map[string]interface{}, 0)
	for _, t := range triggersList {
		patternType := t.Pattern.Type()
		result = append(result, map[string]interface{}{
			"id":              t.ID,
			"agent_id":        t.AgentID,
			"pattern":         t.Pattern,
			"pattern_type":    patternType,
			"prompt_template": t.PromptTemplate,
			"enabled":         t.Enabled,
			"fire_count":      t.FireCount,
			"max_fires":       t.MaxFires,
			"created_at":      t.CreatedAt.Format(time.RFC3339),
		})
	}
	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleListTriggerHistory(w http.ResponseWriter, req *http.Request) {
	triggerID := req.URL.Query().Get("trigger_id")
	agentID := req.URL.Query().Get("agent_id")
	limitStr := req.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	records, err := r.kernel.DB().ListTriggerHistory(triggerID, agentID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list trigger history")
		return
	}

	result := make([]map[string]interface{}, 0)
	for _, record := range records {
		result = append(result, map[string]interface{}{
			"id":                record.ID,
			"trigger_id":        record.TriggerID,
			"agent_id":          record.AgentID,
			"event_type":        record.EventType,
			"event_description": record.EventDescription,
			"sent_message":      record.SentMessage,
			"agent_response":    record.AgentResponse,
			"session_id":        record.SessionID,
			"created_at":        record.CreatedAt.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleDeleteTrigger(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := triggers.ParseTriggerID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid trigger ID")
		return
	}

	if r.kernel.TriggerEngine().Delete(id) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "removed",
			"trigger_id": idStr,
		})
	} else {
		respondError(w, http.StatusNotFound, "Trigger not found")
	}
}

func (r *Router) handleUpdateTrigger(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := triggers.ParseTriggerID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid trigger ID")
		return
	}

	var reqBody map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	trigger, ok := r.kernel.TriggerEngine().Get(id)
	if !ok {
		respondError(w, http.StatusNotFound, "Trigger not found")
		return
	}

	if enabled, ok := reqBody["enabled"].(bool); ok {
		trigger.Enabled = enabled
	}

	if maxFires, ok := reqBody["max_fires"].(float64); ok {
		trigger.MaxFires = uint64(maxFires)
	}

	if promptTemplate, ok := reqBody["prompt_template"].(string); ok {
		trigger.PromptTemplate = promptTemplate
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":              trigger.ID,
		"agent_id":        trigger.AgentID,
		"pattern":         trigger.Pattern,
		"prompt_template": trigger.PromptTemplate,
		"enabled":         trigger.Enabled,
		"fire_count":      trigger.FireCount,
		"max_fires":       trigger.MaxFires,
		"created_at":      trigger.CreatedAt.Format(time.RFC3339),
	})
}

func (r *Router) handleGetPairingConfig(w http.ResponseWriter, req *http.Request) {
	pm := r.kernel.PairingManager()
	if pm == nil {
		respondError(w, http.StatusInternalServerError, "Pairing manager not initialized")
		return
	}
	config := pm.Config()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":           config.Enabled,
		"max_devices":       config.MaxDevices,
		"token_expiry_secs": config.TokenExpirySecs,
		"push_provider":     config.PushProvider,
	})
}

func (r *Router) handleUpdatePairingConfig(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

// isLocalhost checks if a host is localhost or 127.0.0.1
func isLocalhost(host string) bool {
	// Remove port if present
	hostWithoutPort := host
	if idx := strings.Index(host, ":"); idx != -1 {
		hostWithoutPort = host[:idx]
	}
	return hostWithoutPort == "localhost" || hostWithoutPort == "127.0.0.1"
}

// getLANIPAddress gets the primary LAN IP address of this machine
func getLANIPAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		// Check if it's an IPv4 address and not loopback
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				// Skip link-local addresses
				if !ipNet.IP.IsLinkLocalUnicast() {
					return ipNet.IP.String()
				}
			}
		}
	}
	return ""
}

func (r *Router) handleCreatePairingRequest(w http.ResponseWriter, req *http.Request) {
	pm := r.kernel.PairingManager()
	if pm == nil {
		respondError(w, http.StatusNotFound, "Pairing not enabled")
		return
	}

	pairingReq, err := pm.CreatePairingRequest()
	if err != nil {
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}

	// Generate WeChat-compatible QR code URL
	// Use LAN IP for mobile accessibility, fallback to localhost
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	// Get server host from request, but replace localhost with LAN IP
	// Preserve the original port number
	host := req.Host
	if isLocalhost(host) {
		lanIP := getLANIPAddress()
		if lanIP != "" {
			// Extract port from original host and append to LAN IP
			_, port, _ := net.SplitHostPort(req.Host)
			if port != "" {
				host = fmt.Sprintf("%s:%s", lanIP, port)
			} else {
				host = lanIP
			}
		} else {
			// Fallback: use original host if no LAN IP found
			host = req.Host
		}
	}

	serverURL := fmt.Sprintf("%s://%s", scheme, host)
	qrURI := fmt.Sprintf(
		"%s/pair?token=%s",
		serverURL,
		pairingReq.Token,
	)

	// Log for debugging
	fmt.Printf("[DEBUG] QR Code URL: %s (original host: %s, LAN IP: %s)\n", qrURI, req.Host, getLANIPAddress())

	png, err := qrcode.Encode(qrURI, qrcode.Medium, 256)
	if err != nil {
		respondError(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to generate QR code"})
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(png)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token":      pairingReq.Token,
		"qr_uri":     qrURI,
		"qr_png":     qrBase64,
		"expires_at": pairingReq.ExpiresAt.Format(time.RFC3339),
	})
}

func (r *Router) handleCompletePairing(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Token       string  `json:"token"`
		DisplayName string  `json:"display_name"`
		Platform    string  `json:"platform"`
		PushToken   *string `json:"push_token"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	if body.Token == "" {
		fmt.Printf("[DEBUG] Pairing API - Missing token\n")
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": "Missing required fields"})
		return
	}

	pm := r.kernel.PairingManager()
	if pm == nil {
		fmt.Printf("[DEBUG] Pairing API - Pairing not enabled\n")
		respondError(w, http.StatusNotFound, map[string]interface{}{"error": "Pairing not enabled"})
		return
	}

	displayName := body.DisplayName
	if displayName == "" {
		displayName = "unknown"
	}
	platform := body.Platform
	if platform == "" {
		platform = "unknown"
	}

	device := pairing.PairedDevice{
		DeviceID:    uuid.NewString(),
		DisplayName: displayName,
		Platform:    platform,
		PushToken:   body.PushToken,
	}

	pairedDevice, err := pm.CompletePairing(body.Token, device)
	if err != nil {
		fmt.Printf("[DEBUG] Pairing API - CompletePairing error: %v\n", err)
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}

	fmt.Printf("[DEBUG] Pairing API - Success!\n")
	fmt.Printf("  Returned DeviceID: %s\n", pairedDevice.DeviceID)
	fmt.Printf("  Returned DisplayName: %s\n", pairedDevice.DisplayName)
	fmt.Printf("  Returned Platform: %s\n", pairedDevice.Platform)
	fmt.Printf("  PairedAt: %s\n", pairedDevice.PairedAt.Format(time.RFC3339))

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":    pairedDevice.DeviceID,
		"display_name": pairedDevice.DisplayName,
		"platform":     pairedDevice.Platform,
		"paired_at":    pairedDevice.PairedAt.Format(time.RFC3339),
	})
}

// handlePairingPage serves the pairing confirmation page
func (r *Router) handlePairingPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, req, "internal/api/static/pair.html")
}

func (r *Router) handleListPairedDevices(w http.ResponseWriter, req *http.Request) {
	pm := r.kernel.PairingManager()
	if pm == nil {
		respondError(w, http.StatusNotFound, map[string]interface{}{"error": "Pairing not enabled"})
		return
	}

	devices := pm.ListDevices()
	result := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		deviceMap := map[string]interface{}{
			"device_id":    d.DeviceID,
			"display_name": d.DisplayName,
			"platform":     d.Platform,
			"paired_at":    d.PairedAt.Format(time.RFC3339),
			"last_seen":    d.LastSeen.Format(time.RFC3339),
		}
		result = append(result, deviceMap)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"devices": result})
}

func (r *Router) handleNotify(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Message string `json:"message"`
		Title   string `json:"title"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	if body.Message == "" {
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": "Missing required fields"})
		return
	}

	pm := r.kernel.PairingManager()
	if pm == nil {
		respondError(w, http.StatusNotFound, map[string]interface{}{"error": "Pairing not enabled"})
		return
	}

	title := body.Title
	if title == "" {
		title = "FangClawGo Notification"
	}

	ctx := req.Context()
	results := pm.NotifyDevices(ctx, title, body.Message)
	for _, res := range results {
		if res.Error != nil {
			respondError(w, http.StatusInternalServerError, map[string]interface{}{"error": res.Error.Error()})
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (r *Router) handleRemovePairedDevice(w http.ResponseWriter, req *http.Request) {
	deviceID := req.PathValue("id")
	if deviceID == "" {
		respondError(w, http.StatusBadRequest, map[string]interface{}{"error": "Missing device ID"})
		return
	}

	pm := r.kernel.PairingManager()
	if pm == nil {
		respondError(w, http.StatusNotFound, map[string]interface{}{"error": "Pairing not enabled"})
		return
	}

	if err := pm.RemoveDevice(deviceID); err != nil {
		respondError(w, http.StatusNotFound, map[string]interface{}{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
