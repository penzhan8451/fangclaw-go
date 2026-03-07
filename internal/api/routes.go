// Package api implements the REST API server for OpenFang.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/approvals"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/clawhub"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/cron"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
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
			{Key: "poll_interval_secs", Label: "Poll Interval (sec)", FieldType: FieldTypeNumber, EnvVar: nil, Required: false, Placeholder: "1", Advanced: true},
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
			{Key: "intents", Label: "Intents Bitmask", FieldType: FieldTypeNumber, EnvVar: nil, Required: false, Placeholder: "37376", Advanced: true},
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
			{Key: "webhook_port", Label: "Webhook Port", FieldType: FieldTypeNumber, EnvVar: nil, Required: false, Placeholder: "8443", Advanced: true},
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
		QuickSetup:  "Paste your webhook token and signing secret",
		SetupType:   "form",
		Fields: []ChannelField{
			{Key: "access_token_env", Label: "Access Token", FieldType: FieldTypeSecret, EnvVar: strPtr("DINGTALK_ACCESS_TOKEN"), Required: true, Placeholder: "abc123...", Advanced: false},
			{Key: "secret_env", Label: "Signing Secret", FieldType: FieldTypeSecret, EnvVar: strPtr("DINGTALK_SECRET"), Required: true, Placeholder: "SEC...", Advanced: false},
			{Key: "webhook_port", Label: "Webhook Port", FieldType: FieldTypeNumber, EnvVar: nil, Required: false, Placeholder: "8457", Advanced: true},
			{Key: "default_agent", Label: "Default Agent", FieldType: FieldTypeText, EnvVar: nil, Required: false, Placeholder: "assistant", Advanced: true},
		},
		SetupSteps:     []string{"Create a robot in your DingTalk group", "Copy the token and signing secret", "Paste them below"},
		ConfigTemplate: "[channels.dingtalk]\naccess_token_env = \"DINGTALK_ACCESS_TOKEN\"\nsecret_env = \"DINGTALK_SECRET\"",
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
			{Key: "webhook_port", Label: "Webhook Port", FieldType: FieldTypeNumber, EnvVar: nil, Required: false, Placeholder: "8453", Advanced: true},
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
	kernel *kernel.Kernel
}

// NewRouter creates a new API router.
func NewRouter(k *kernel.Kernel) *Router {
	return &Router{
		kernel: k,
	}
}

// RegisterRoutes registers all API routes.
func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	// Health and status endpoints
	mux.HandleFunc("GET /api/health", r.handleHealth)
	mux.HandleFunc("GET /api/status", r.handleStatus)
	mux.HandleFunc("GET /api/version", r.handleVersion)

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
	mux.HandleFunc("GET /api/a2a/agents", r.handleA2AAgents)
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
	mux.HandleFunc("POST /api/workflows/{id}/run", r.handleRunWorkflow)
	mux.HandleFunc("GET /api/workflows/{id}/runs", r.handleListWorkflowRuns)

	// Triggers endpoints
	mux.HandleFunc("POST /api/triggers", r.handleCreateTrigger)
	mux.HandleFunc("GET /api/triggers", r.handleListTriggers)
	mux.HandleFunc("DELETE /api/triggers/{id}", r.handleDeleteTrigger)

	// Shutdown endpoint
	mux.HandleFunc("POST /api/shutdown", r.handleShutdown)
}

// respondJSON responds with JSON.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError responds with an error.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
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
	respondJSON(w, http.StatusOK, types.StatusResponse{
		Status:     "running",
		Version:    "0.1.0",
		ListenAddr: ":4200",
		AgentCount: r.kernel.AgentRegistry().Count(),
		ModelCount: 1,
		Uptime:     "0s",
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
		respondError(w, http.StatusInternalServerError, "Agent spawn failed: "+err.Error())
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

	session := types.NewSession(agentID, reqBody.Label)
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

	respondJSON(w, http.StatusNoContent, nil)
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
// Note: memory_store tool writes to a shared namespace, so we read from that
// same namespace regardless of which agent ID is in the URL.
func (r *Router) handleGetAgentKV(w http.ResponseWriter, req *http.Request) {
	agentID := sharedMemoryAgentID()

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
	agentID := sharedMemoryAgentID()
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
// Note: memory_store tool writes to a shared namespace, so we write to that
// same namespace regardless of which agent ID is in the URL.
func (r *Router) handleSetAgentKVKey(w http.ResponseWriter, req *http.Request) {
	agentID := sharedMemoryAgentID()
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
	agentID := sharedMemoryAgentID()
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
		return cfg.Channels.WhatsApp != nil && (cfg.Channels.WhatsApp.AccessTokenEnv != "" || cfg.Channels.WhatsApp.PhoneNumberID != "")
	case "qq":
		return cfg.Channels.QQ != nil && cfg.Channels.QQ.AppID != "" && cfg.Channels.QQ.AppSecretEnv != ""
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
			// For Feishu, directly save the secret in config (no secrets.env dependency)
			if name == "feishu" && field.Key == "app_secret_env" {
				channelConfig.AppSecret = valueStr
				// Also set in current process for backward compatibility
				os.Setenv(*field.EnvVar, valueStr)
			} else {
				// Try to write secret to file (non-fatal if fails)
				if err := config.WriteSecretEnv(*field.EnvVar, valueStr); err != nil {
					fmt.Printf("Warning: failed to write secret to file: %v\n", err)
				}
				// Always set in current process
				os.Setenv(*field.EnvVar, valueStr)
				// Store env var name in config
				switch field.Key {
				case "bot_token_env":
					channelConfig.BotTokenEnv = *field.EnvVar
				case "app_token_env":
					channelConfig.AppTokenEnv = *field.EnvVar
				case "access_token_env":
					channelConfig.AccessTokenEnv = *field.EnvVar
				case "app_secret_env":
					channelConfig.AppSecretEnv = *field.EnvVar
				case "secret_env":
					channelConfig.SecretEnv = *field.EnvVar
				case "verify_token_env":
					channelConfig.VerifyTokenEnv = *field.EnvVar
				}
			}
		} else {
			// Handle regular fields
			switch field.Key {
			case "app_id":
				channelConfig.AppID = valueStr
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
			case "poll_interval_secs":
				if i, err := strconv.Atoi(valueStr); err == nil {
					channelConfig.PollIntervalSecs = i
				}
			case "intents":
				if i, err := strconv.Atoi(valueStr); err == nil {
					channelConfig.Intents = i
				}
			case "webhook_port":
				if i, err := strconv.Atoi(valueStr); err == nil {
					channelConfig.WebhookPort = i
				}
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

func (r *Router) handleGetAgentSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": "default",
		"messages":   []interface{}{},
	})
}

func (r *Router) handleGetAgentSessions(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleCreateAgentSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusCreated, map[string]string{
		"session_id": "new-session",
	})
}

func (r *Router) handleSwitchSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleResetSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (r *Router) handleCompactSession(w http.ResponseWriter, req *http.Request) {
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

	runner := agent.NewAgentRunner(agentRuntime)

	ctx := context.Background()
	result, err := runner.RunAgent(ctx, actualAgentID, reqBody.Message, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"response": result.Response,
		"message": map[string]string{
			"role":    "assistant",
			"content": result.Response,
		},
		"usage": map[string]interface{}{
			"input_tokens":  result.TotalUsage.PromptTokens,
			"output_tokens": result.TotalUsage.CompletionTokens,
			"total_tokens":  result.TotalUsage.TotalTokens,
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
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": []interface{}{},
	})
}

func (r *Router) handleAuditVerify(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"verified":    true,
		"merkle_root": "0000000000000000000000000000000000000000000000000000000000000000",
	})
}

func (r *Router) handleMcpServers(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"servers": []interface{}{},
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

	flusher.Flush()
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

	// 读取本地保存的hand状态
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

		// 获取hand的状态
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
	agentID := req.URL.Query().Get("agent_id")
	var jobs []*cron.CronJob

	if agentID != "" {
		allJobs := r.kernel.CronScheduler().ListJobs()
		for _, job := range allJobs {
			if job.AgentID == agentID {
				jobs = append(jobs, job)
			}
		}
	} else {
		jobs = r.kernel.CronScheduler().ListJobs()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func (r *Router) handleCreateCronJob(w http.ResponseWriter, req *http.Request) {
	var job cron.CronJob
	if err := json.NewDecoder(req.Body).Decode(&job); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := r.kernel.CronScheduler().AddJob(&job); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"result": job,
	})
}

func (r *Router) handleEnableCronJob(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	var reqBody struct {
		Enabled bool `json:"enabled"`
	}

	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var err error
	if reqBody.Enabled {
		err = r.kernel.CronScheduler().EnableJob(id)
	} else {
		err = r.kernel.CronScheduler().DisableJob(id)
	}

	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"enabled": reqBody.Enabled,
	})
}

func (r *Router) handleDeleteCronJob(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	if err := r.kernel.CronScheduler().DeleteJob(id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}

func (r *Router) handleCronJobStatus(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	job, ok := r.kernel.CronScheduler().GetJob(id)
	if !ok {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
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

	agents := r.kernel.AgentRegistry().List()

	var approvals []map[string]interface{}
	for _, a := range pending {
		agentName := a.AgentID
		for _, agent := range agents {
			if agent.ID.String() == a.AgentID || agent.Name == a.AgentID {
				agentName = agent.Name
				break
			}
		}

		approvals = append(approvals, map[string]interface{}{
			"id":             a.ID,
			"agent_id":       a.AgentID,
			"agent_name":     agentName,
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

	approvalReq := approvals.NewApprovalRequest(
		reqBody.AgentID,
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
		} else {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Step '%s' needs 'agent_id' or 'agent_name'", stepName))
			return
		}

		modeType := "sequential"
		if mt, ok := stepMap["mode"].(string); ok {
			modeType = mt
		}

		var mode types.StepMode
		switch modeType {
		case "fan_out":
			mode = types.StepMode{Type: "fan_out"}
		case "collect":
			mode = types.StepMode{Type: "collect"}
		case "conditional":
			condition := ""
			if c, ok := stepMap["condition"].(string); ok {
				condition = c
			}
			mode = types.StepMode{Type: "conditional", Condition: &condition}
		case "loop":
			maxIterations := uint32(5)
			if mi, ok := stepMap["max_iterations"].(float64); ok {
				maxIterations = uint32(mi)
			}
			until := ""
			if u, ok := stepMap["until"].(string); ok {
				until = u
			}
			mode = types.StepMode{Type: "loop", MaxIterations: &maxIterations, Until: &until}
		default:
			mode = types.StepMode{Type: "sequential"}
		}

		errorModeType := "fail"
		if emt, ok := stepMap["error_mode"].(string); ok {
			errorModeType = emt
		}

		var errorMode types.ErrorMode
		switch errorModeType {
		case "skip":
			errorMode = types.ErrorMode{Type: "skip"}
		case "retry":
			maxRetries := uint32(3)
			if mr, ok := stepMap["max_retries"].(float64); ok {
				maxRetries = uint32(mr)
			}
			errorMode = types.ErrorMode{Type: "retry", MaxRetries: &maxRetries}
		default:
			errorMode = types.ErrorMode{Type: "fail"}
		}

		promptTemplate := "{{input}}"
		if pt, ok := stepMap["prompt"].(string); ok {
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

	workflowID := types.WorkflowID(fmt.Sprintf("wf-%d", time.Now().UnixNano()))
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

	resolver := func(agent types.StepAgent) (string, string, bool) {
		if agent.ID != nil {
			return *agent.ID, "Agent", true
		}
		if agent.Name != nil {
			return "agent-id", *agent.Name, true
		}
		return "", "", false
	}

	sender := func(agentID, prompt string) (string, uint64, uint64, error) {
		return "Sample output", 100, 50, nil
	}

	output, err := r.kernel.WorkflowEngine().ExecuteRun(*runID, resolver, sender)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Workflow execution failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"run_id": *runID,
		"output": output,
		"status": "completed",
	})
}

func (r *Router) handleListWorkflowRuns(w http.ResponseWriter, req *http.Request) {
	runs := r.kernel.WorkflowEngine().ListRuns(nil)
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

	var pattern triggers.TriggerPattern
	patternBytes, err := json.Marshal(patternData)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid trigger pattern")
		return
	}

	if err := json.Unmarshal(patternBytes, &pattern); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid trigger pattern")
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
		result = append(result, map[string]interface{}{
			"id":              t.ID,
			"agent_id":        t.AgentID,
			"pattern":         t.Pattern,
			"prompt_template": t.PromptTemplate,
			"enabled":         t.Enabled,
			"fire_count":      t.FireCount,
			"max_fires":       t.MaxFires,
			"created_at":      t.CreatedAt.Format(time.RFC3339),
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
