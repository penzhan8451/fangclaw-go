// Package channels provides channel adapters for FangClaw.
package channels

import (
	"context"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// HealthMonitor provides channel health status tracking.
type HealthMonitor struct {
	mu               sync.Mutex
	connected        bool
	startedAt        *time.Time
	lastMessageAt    *time.Time
	messagesReceived uint64
	messagesSent     uint64
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{}
}

// Start marks the channel as started.
func (h *HealthMonitor) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	h.startedAt = &now
	h.connected = true
}

// Stop marks the channel as stopped.
func (h *HealthMonitor) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connected = false
}

// RecordMessageSent records that a message was sent.
func (h *HealthMonitor) RecordMessageSent() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	h.lastMessageAt = &now
	h.messagesSent++
}

// RecordMessageReceived records that a message was received.
func (h *HealthMonitor) RecordMessageReceived() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	h.lastMessageAt = &now
	h.messagesReceived++
}

// GetHealthStatus returns the current health status.
func (h *HealthMonitor) GetHealthStatus() *types.ChannelStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return &types.ChannelStatus{
		Connected:        h.connected,
		StartedAt:        h.startedAt,
		LastMessageAt:    h.lastMessageAt,
		MessagesReceived: h.messagesReceived,
		MessagesSent:     h.messagesSent,
	}
}

// ChannelType represents the type of a channel.
type ChannelType string

// ChannelState represents the state of a channel.
type ChannelState string

const (
	// Channel states
	ChannelStateIdle         ChannelState = "idle"
	ChannelStateConnected    ChannelState = "connected"
	ChannelStateDisconnected ChannelState = "disconnected"
	ChannelStateError        ChannelState = "error"

	// Core channels
	ChannelTypeTelegram ChannelType = "telegram"
	ChannelTypeDiscord  ChannelType = "discord"
	ChannelTypeSlack    ChannelType = "slack"
	ChannelTypeWhatsApp ChannelType = "whatsapp"
	ChannelTypeSignal   ChannelType = "signal"
	ChannelTypeMatrix   ChannelType = "matrix"
	ChannelTypeEmail    ChannelType = "email"

	// Enterprise channels
	ChannelTypeTeams      ChannelType = "teams"
	ChannelTypeMattermost ChannelType = "mattermost"
	ChannelTypeGoogleChat ChannelType = "google_chat"
	ChannelTypeWebex      ChannelType = "webex"
	ChannelTypeFeishu     ChannelType = "feishu"
	ChannelTypeZulip      ChannelType = "zulip"

	// Social channels
	ChannelTypeLINE      ChannelType = "line"
	ChannelTypeViber     ChannelType = "viber"
	ChannelTypeMessenger ChannelType = "messenger"
	ChannelTypeMastodon  ChannelType = "mastodon"
	ChannelTypeBluesky   ChannelType = "bluesky"
	ChannelTypeReddit    ChannelType = "reddit"
	ChannelTypeLinkedIn  ChannelType = "linkedin"
	ChannelTypeTwitch    ChannelType = "twitch"

	// Community channels
	ChannelTypeIRC       ChannelType = "irc"
	ChannelTypeXMPP      ChannelType = "xmpp"
	ChannelTypeGuilded   ChannelType = "guilded"
	ChannelTypeRevolt    ChannelType = "revolt"
	ChannelTypeKeybase   ChannelType = "keybase"
	ChannelTypeDiscourse ChannelType = "discourse"
	ChannelTypeGitter    ChannelType = "gitter"

	// Privacy channels
	ChannelTypeThreema    ChannelType = "threema"
	ChannelTypeNostr      ChannelType = "nostr"
	ChannelTypeMumble     ChannelType = "mumble"
	ChannelTypeNextcloud  ChannelType = "nextcloud"
	ChannelTypeRocketChat ChannelType = "rocketchat"
	ChannelTypeNtfy       ChannelType = "ntfy"
	ChannelTypeGotify     ChannelType = "gotify"

	// Workplace channels
	ChannelTypePumble   ChannelType = "pumble"
	ChannelTypeFlock    ChannelType = "flock"
	ChannelTypeTwist    ChannelType = "twist"
	ChannelTypeDingTalk ChannelType = "dingtalk"
	ChannelTypeQQ       ChannelType = "qq"
	ChannelTypeZalo     ChannelType = "zalo"
	ChannelTypeWebhook  ChannelType = "webhook"
)

// Channel represents a communication channel.
type Channel struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Type      ChannelType          `json:"type"`
	State     ChannelState         `json:"state"`
	Config    ChannelAdapterConfig `json:"config"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// TelegramChannelConfig represents the configuration for Telegram channel.
type TelegramChannelConfig struct {
	BotToken string `json:"bot_token,omitempty"`
}

// SlackChannelConfig represents the configuration for Slack channel.
type SlackChannelConfig struct {
	BotToken string `json:"bot_token,omitempty"`
}

// DiscordChannelConfig represents the configuration for Discord channel.
type DiscordChannelConfig struct {
	BotToken string `json:"bot_token,omitempty"`
}

// FeishuChannelConfig represents the configuration for Feishu channel.
type FeishuChannelConfig struct {
	AppID     string `json:"app_id,omitempty"`
	AppSecret string `json:"app_secret,omitempty"`
}

// DingTalkChannelConfig represents the configuration for DingTalk channel.
type DingTalkChannelConfig struct {
	// 旧配置（保持向后兼容）
	AppKey    string `json:"app_key,omitempty"`
	AppSecret string `json:"app_secret,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`

	// 新配置（Stream SDK 方式）
	ClientID     string   `json:"client_id,omitempty"`     // 钉钉应用 Client ID
	ClientSecret string   `json:"client_secret,omitempty"` // 钉钉应用 Client Secret
	GroupTrigger string   `json:"group_trigger,omitempty"` // 群组触发词（可选）
	AllowFrom    []string `json:"allow_from,omitempty"`    // 用户白名单（可选）
}

// WhatsAppChannelConfig represents the configuration for WhatsApp channel.
type WhatsAppChannelConfig struct {
	PhoneID     string `json:"phone_id,omitempty"`
	BusinessID  string `json:"business_id,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
}

// QQChannelConfig represents the configuration for QQ channel.
type QQChannelConfig struct {
	AppID              string   `json:"app_id,omitempty"`
	AppSecret          string   `json:"app_secret,omitempty"`
	GroupTrigger       string   `json:"group_trigger,omitempty"`
	ReasoningChannelID string   `json:"reasoning_channel_id,omitempty"`
	AllowFrom          []string `json:"allow_from,omitempty"`
}

// GenericChannelConfig represents the generic configuration for channels.
// It is used for channels that don't have a specific configuration.
type GenericChannelConfig struct {
	APIKey     string            `json:"api_key,omitempty"`
	Token      string            `json:"token,omitempty"`
	Username   string            `json:"username,omitempty"`
	Password   string            `json:"password,omitempty"`
	Server     string            `json:"server,omitempty"`
	Port       int               `json:"port,omitempty"`
	ChannelID  string            `json:"channel_id,omitempty"`
	ChatID     string            `json:"chat_id,omitempty"`
	WebhookURL string            `json:"webhook_url,omitempty"`
	Settings   map[string]string `json:"settings,omitempty"`
}

// ChannelAdapterConfig represents the configuration for a channel.
// It is used to store the configuration for different types of channels.
type ChannelAdapterConfig struct {
	Telegram *TelegramChannelConfig `json:"telegram,omitempty"`
	Slack    *SlackChannelConfig    `json:"slack,omitempty"`
	Discord  *DiscordChannelConfig  `json:"discord,omitempty"`
	Feishu   *FeishuChannelConfig   `json:"feishu,omitempty"`
	DingTalk *DingTalkChannelConfig `json:"dingtalk,omitempty"`
	WhatsApp *WhatsAppChannelConfig `json:"whatsapp,omitempty"`
	QQ       *QQChannelConfig       `json:"qq,omitempty"`
	Generic  *GenericChannelConfig  `json:"generic,omitempty"`
}

// Message represents a message sent through a channel.
type Message struct {
	ID        string      `json:"id"`
	ChannelID string      `json:"channel_id"`
	Content   string      `json:"content"`
	Sender    string      `json:"sender,omitempty"`
	Recipient string      `json:"recipient,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

// Adapter is an interface for channel adapters.
type Adapter interface {
	// Connect connects to the channel.
	Connect() error

	// Disconnect disconnects from the channel.
	Disconnect() error

	// Start starts the channel adapter.
	Start() error

	// Stop stops the channel adapter.
	Stop() error

	// Send sends a message to the channel.
	Send(msg *Message) error

	// Receive receives messages from the channel.
	Receive(ctx context.Context) (<-chan *Message, error)

	// GetState returns the current state of the channel.
	GetState() ChannelState

	// GetInfo returns information about the channel.
	GetInfo() map[string]interface{}

	// GetHealthStatus returns the health status of the channel adapter.
	GetHealthStatus() *types.ChannelStatus

	// RecordMessageSent records that a message was sent.
	RecordMessageSent()

	// RecordMessageReceived records that a message was received.
	RecordMessageReceived()

	// GetChannel returns the channel associated with this adapter.
	GetChannel() *Channel
}

// AdapterFactory is a function that creates an adapter for a channel.
type AdapterFactory func(channel *Channel) (Adapter, error)

// AutoRegisterFunc is a function that auto-registers a channel.
type AutoRegisterFunc func(registry *Registry) error

var autoRegisterFuncs []AutoRegisterFunc

// RegisterAutoRegister adds an auto-register function.
func RegisterAutoRegister(f AutoRegisterFunc) {
	autoRegisterFuncs = append(autoRegisterFuncs, f)
}

// AutoRegisterAll runs all registered auto-register functions.
func AutoRegisterAll(registry *Registry) error {
	for _, f := range autoRegisterFuncs {
		if err := f(registry); err != nil {
			return err
		}
	}
	return nil
}
