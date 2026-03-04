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
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Type      ChannelType   `json:"type"`
	State     ChannelState  `json:"state"`
	Config    ChannelConfig `json:"config"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ChannelConfig represents the configuration for a channel.
type ChannelConfig struct {
	APIKey              string            `json:"api_key,omitempty"`
	Token               string            `json:"token,omitempty"`
	TelegramBotToken    string            `json:"telegram_bot_token,omitempty"`
	SlackBotToken       string            `json:"slack_bot_token,omitempty"`
	DiscordBotToken     string            `json:"discord_bot_token,omitempty"`
	FeishuAppID         string            `json:"feishu_app_id,omitempty"`
	FeishuAppSecret     string            `json:"feishu_app_secret,omitempty"`
	DingTalkAppKey      string            `json:"dingtalk_app_key,omitempty"`
	DingTalkAppSecret   string            `json:"dingtalk_app_secret,omitempty"`
	DingTalkAgentID     string            `json:"dingtalk_agent_id,omitempty"`
	WhatsAppPhoneID     string            `json:"whatsapp_phone_id,omitempty"`
	WhatsAppBusinessID  string            `json:"whatsapp_business_id,omitempty"`
	WhatsAppAccessToken string            `json:"whatsapp_access_token,omitempty"`
	QQAppID             string            `json:"qq_app_id,omitempty"`
	QQAppSecret         string            `json:"qq_app_secret,omitempty"`
	QQGroupTrigger      string            `json:"qq_group_trigger,omitempty"`
	QQReasoningChannelID string           `json:"qq_reasoning_channel_id,omitempty"`
	QQAllowFrom         []string          `json:"qq_allow_from,omitempty"`
	Username            string            `json:"username,omitempty"`
	Password            string            `json:"password,omitempty"`
	Server              string            `json:"server,omitempty"`
	Port                int               `json:"port,omitempty"`
	ChannelID           string            `json:"channel_id,omitempty"`
	ChatID              string            `json:"chat_id,omitempty"`
	WebhookURL          string            `json:"webhook_url,omitempty"`
	Settings            map[string]string `json:"settings,omitempty"`
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
}

// AdapterFactory is a function that creates an adapter for a channel.
type AdapterFactory func(channel *Channel) (Adapter, error)
