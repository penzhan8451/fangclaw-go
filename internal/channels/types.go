// Package channels provides channel adapters for FangClaw.
package channels

import (
	"context"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/config"
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
	ChannelTypeWeixin   ChannelType = "weixin"
)

// Channel represents a communication channel.
type Channel struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Type      ChannelType          `json:"type"`
	Owner     string               `json:"owner,omitempty"`
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
	AppToken string `json:"app_token,omitempty"`
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
	// Old config (maintained for backward compatibility)
	AppKey    string `json:"app_key,omitempty"`
	AppSecret string `json:"app_secret,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`

	// New config (Stream SDK mode)
	ClientID     string   `json:"client_id,omitempty"`     // DingTalk App Client ID
	ClientSecret string   `json:"client_secret,omitempty"` // DingTalk App Client Secret
	GroupTrigger string   `json:"group_trigger,omitempty"` // Group trigger words (optional)
	AllowFrom    []string `json:"allow_from,omitempty"`    // User whitelist (optional)
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

// WeixinChannelConfig represents the configuration for Weixin channel.
type WeixinChannelConfig struct {
	Token              string   `json:"token,omitempty"`
	TokenEnv           string   `json:"token_env,omitempty"`
	BaseURL            string   `json:"base_url,omitempty"`
	CDNBaseURL         string   `json:"cdn_base_url,omitempty"`
	Proxy              string   `json:"proxy,omitempty"`
	ReasoningChannelID string   `json:"reasoning_channel_id,omitempty"`
	AllowFrom          []string `json:"allow_from,omitempty"`
	DefaultAgent       string   `json:"default_agent,omitempty"`
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
	Weixin   *WeixinChannelConfig   `json:"weixin,omitempty"`
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

// SecretGetter is a function that retrieves a secret by key.
type SecretGetter func(key string) string

// AutoRegisterFunc is a function that auto-registers a channel.
type AutoRegisterFunc func(registry *Registry, getSecret SecretGetter) error

var autoRegisterFuncs []AutoRegisterFunc

// RegisterAutoRegister adds an auto-register function.
func RegisterAutoRegister(f AutoRegisterFunc) {
	autoRegisterFuncs = append(autoRegisterFuncs, f)
}

// AutoRegisterAll runs all registered auto-register functions.
func AutoRegisterAll(registry *Registry, getSecret SecretGetter) error {
	for _, f := range autoRegisterFuncs {
		if err := f(registry, getSecret); err != nil {
			return err
		}
	}
	return nil
}

// LoadConfiguredChannels loads all configured channels from the given config and registers them.
func LoadConfiguredChannels(registry *Registry, cfg *config.Config, getSecret SecretGetter) ([]string, error) {
	var started []string

	// Check each channel type
	channelTypes := []struct {
		name string
		typ  ChannelType
	}{
		{"telegram", ChannelTypeTelegram},
		{"discord", ChannelTypeDiscord},
		{"slack", ChannelTypeSlack},
		{"whatsapp", ChannelTypeWhatsApp},
		{"qq", ChannelTypeQQ},
		{"dingtalk", ChannelTypeDingTalk},
		{"feishu", ChannelTypeFeishu},
		{"weixin", ChannelTypeWeixin},
	}

	for _, ct := range channelTypes {
		// Check if this channel has an adapter factory
		_, hasFactory := registry.GetFactory(ct.typ)
		if !hasFactory {
			continue
		}

		// Check if this channel already exists (from env vars), if yes, skip
		existingChannels := registry.ListChannels()
		alreadyExists := false
		for _, ch := range existingChannels {
			if ch.Type == ct.typ {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			continue
		}

		// Check if channel is configured in config.toml
		isConfigured := false
		switch ct.name {
		case "telegram":
			if cfg.Channels.Telegram != nil {
				botToken := cfg.Channels.Telegram.BotToken
				if botToken == "" && cfg.Channels.Telegram.BotTokenEnv != "" {
					botToken = getSecret(cfg.Channels.Telegram.BotTokenEnv)
				}
				isConfigured = botToken != ""
			}
		case "discord":
			if cfg.Channels.Discord != nil {
				botToken := cfg.Channels.Discord.BotToken
				if botToken == "" && cfg.Channels.Discord.BotTokenEnv != "" {
					botToken = getSecret(cfg.Channels.Discord.BotTokenEnv)
				}
				isConfigured = botToken != ""
			}
		case "slack":
			if cfg.Channels.Slack != nil {
				botToken := cfg.Channels.Slack.BotToken
				if botToken == "" && cfg.Channels.Slack.BotTokenEnv != "" {
					botToken = getSecret(cfg.Channels.Slack.BotTokenEnv)
				}
				appToken := cfg.Channels.Slack.AppToken
				if appToken == "" && cfg.Channels.Slack.AppTokenEnv != "" {
					appToken = getSecret(cfg.Channels.Slack.AppTokenEnv)
				}
				isConfigured = botToken != "" && appToken != ""
			}
		case "whatsapp":
			if cfg.Channels.WhatsApp != nil {
				accessToken := cfg.Channels.WhatsApp.AccessToken
				if accessToken == "" && cfg.Channels.WhatsApp.AccessTokenEnv != "" {
					accessToken = getSecret(cfg.Channels.WhatsApp.AccessTokenEnv)
				}
				isConfigured = (accessToken != "" || cfg.Channels.WhatsApp.PhoneNumberID != "")
			}
		case "qq":
			if cfg.Channels.QQ != nil && cfg.Channels.QQ.AppID != "" {
				appSecret := cfg.Channels.QQ.AppSecret
				if appSecret == "" && cfg.Channels.QQ.AppSecretEnv != "" {
					appSecret = getSecret(cfg.Channels.QQ.AppSecretEnv)
				}
				isConfigured = appSecret != ""
			}
		case "dingtalk":
			if cfg.Channels.DingTalk != nil && cfg.Channels.DingTalk.ClientID != "" {
				clientSecret := cfg.Channels.DingTalk.ClientSecret
				if clientSecret == "" && cfg.Channels.DingTalk.ClientSecretEnv != "" {
					clientSecret = getSecret(cfg.Channels.DingTalk.ClientSecretEnv)
				}
				isConfigured = clientSecret != ""
			}
		case "feishu":
			if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.AppID != "" {
				appSecret := cfg.Channels.Feishu.AppSecret
				if appSecret == "" && cfg.Channels.Feishu.AppSecretEnv != "" {
					appSecret = getSecret(cfg.Channels.Feishu.AppSecretEnv)
				}
				isConfigured = appSecret != ""
			}
		case "weixin":
			if cfg.Channels.Weixin != nil {
				token := cfg.Channels.Weixin.Token
				if token == "" && cfg.Channels.Weixin.TokenEnv != "" {
					token = getSecret(cfg.Channels.Weixin.TokenEnv)
				}
				isConfigured = token != ""
			}
		}

		// Only proceed if channel is configured in config.toml
		if !isConfigured {
			continue
		}

		// Create and register new channel
		newChannel := &Channel{
			Name:  ct.name,
			Type:  ct.typ,
			State: ChannelStateIdle,
		}

		// Set channel-specific config
		switch ct.name {
		case "telegram":
			botToken := cfg.Channels.Telegram.BotToken
			if botToken == "" && cfg.Channels.Telegram.BotTokenEnv != "" {
				botToken = getSecret(cfg.Channels.Telegram.BotTokenEnv)
			}
			newChannel.Config.Telegram = &TelegramChannelConfig{
				BotToken: botToken,
			}
		case "discord":
			botToken := cfg.Channels.Discord.BotToken
			if botToken == "" && cfg.Channels.Discord.BotTokenEnv != "" {
				botToken = getSecret(cfg.Channels.Discord.BotTokenEnv)
			}
			newChannel.Config.Discord = &DiscordChannelConfig{
				BotToken: botToken,
			}
		case "slack":
			botToken := cfg.Channels.Slack.BotToken
			if botToken == "" && cfg.Channels.Slack.BotTokenEnv != "" {
				botToken = getSecret(cfg.Channels.Slack.BotTokenEnv)
			}
			appToken := cfg.Channels.Slack.AppToken
			if appToken == "" && cfg.Channels.Slack.AppTokenEnv != "" {
				appToken = getSecret(cfg.Channels.Slack.AppTokenEnv)
			}
			newChannel.Config.Slack = &SlackChannelConfig{
				BotToken: botToken,
				AppToken: appToken,
			}
		case "whatsapp":
			accessToken := cfg.Channels.WhatsApp.AccessToken
			if accessToken == "" && cfg.Channels.WhatsApp.AccessTokenEnv != "" {
				accessToken = getSecret(cfg.Channels.WhatsApp.AccessTokenEnv)
			}
			newChannel.Config.WhatsApp = &WhatsAppChannelConfig{
				AccessToken: accessToken,
				PhoneID:     cfg.Channels.WhatsApp.PhoneNumberID,
			}
		case "qq":
			appSecret := cfg.Channels.QQ.AppSecret
			if appSecret == "" && cfg.Channels.QQ.AppSecretEnv != "" {
				appSecret = getSecret(cfg.Channels.QQ.AppSecretEnv)
			}
			newChannel.Config.QQ = &QQChannelConfig{
				AppID:     cfg.Channels.QQ.AppID,
				AppSecret: appSecret,
			}
		case "dingtalk":
			clientID := cfg.Channels.DingTalk.ClientID
			if clientID == "" && cfg.Channels.DingTalk.ClientIDEnv != "" {
				clientID = getSecret(cfg.Channels.DingTalk.ClientIDEnv)
			}
			clientSecret := cfg.Channels.DingTalk.ClientSecret
			if clientSecret == "" && cfg.Channels.DingTalk.ClientSecretEnv != "" {
				clientSecret = getSecret(cfg.Channels.DingTalk.ClientSecretEnv)
			}
			newChannel.Config.DingTalk = &DingTalkChannelConfig{
				ClientID:     clientID,
				ClientSecret: clientSecret,
			}
		case "feishu":
			appSecret := cfg.Channels.Feishu.AppSecret
			if appSecret == "" && cfg.Channels.Feishu.AppSecretEnv != "" {
				appSecret = getSecret(cfg.Channels.Feishu.AppSecretEnv)
			}
			newChannel.Config.Feishu = &FeishuChannelConfig{
				AppID:     cfg.Channels.Feishu.AppID,
				AppSecret: appSecret,
			}
		case "weixin":
			token := cfg.Channels.Weixin.Token
			if token == "" && cfg.Channels.Weixin.TokenEnv != "" {
				token = getSecret(cfg.Channels.Weixin.TokenEnv)
			}
			newChannel.Config.Weixin = &WeixinChannelConfig{
				Token:              token,
				BaseURL:            cfg.Channels.Weixin.BaseURL,
				CDNBaseURL:         cfg.Channels.Weixin.CDNBaseURL,
				Proxy:              cfg.Channels.Weixin.Proxy,
				ReasoningChannelID: cfg.Channels.Weixin.ReasoningChannelID,
				DefaultAgent:       cfg.Channels.Weixin.DefaultAgent,
			}
		}

		if err := registry.RegisterChannel(newChannel); err == nil {
			// Try to start the adapter
			if adapter, ok := registry.GetAdapter(newChannel.ID); ok {
				if err := adapter.Start(); err == nil {
					started = append(started, ct.name)
				}
			}
		}
	}

	return started, nil
}

// LoadConfiguredChannelsWithOwner loads all configured channels from the given config and registers them with owner.
func LoadConfiguredChannelsWithOwner(registry *Registry, cfg *config.Config, getSecret SecretGetter, owner string) ([]string, error) {
	var started []string

	channelTypes := []struct {
		name string
		typ  ChannelType
	}{
		{"telegram", ChannelTypeTelegram},
		{"discord", ChannelTypeDiscord},
		{"slack", ChannelTypeSlack},
		{"whatsapp", ChannelTypeWhatsApp},
		{"qq", ChannelTypeQQ},
		{"dingtalk", ChannelTypeDingTalk},
		{"feishu", ChannelTypeFeishu},
		{"weixin", ChannelTypeWeixin},
	}

	for _, ct := range channelTypes {
		_, hasFactory := registry.GetFactory(ct.typ)
		if !hasFactory {
			continue
		}

		existingChannels := registry.ListChannels()
		alreadyExists := false
		for _, ch := range existingChannels {
			if ch.Type == ct.typ && ch.Owner == owner {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			continue
		}

		isConfigured := false
		switch ct.name {
		case "telegram":
			if cfg.Channels.Telegram != nil {
				botToken := cfg.Channels.Telegram.BotToken
				if botToken == "" && cfg.Channels.Telegram.BotTokenEnv != "" {
					botToken = getSecret(cfg.Channels.Telegram.BotTokenEnv)
				}
				isConfigured = botToken != ""
			}
		case "discord":
			if cfg.Channels.Discord != nil {
				botToken := cfg.Channels.Discord.BotToken
				if botToken == "" && cfg.Channels.Discord.BotTokenEnv != "" {
					botToken = getSecret(cfg.Channels.Discord.BotTokenEnv)
				}
				isConfigured = botToken != ""
			}
		case "slack":
			if cfg.Channels.Slack != nil {
				botToken := cfg.Channels.Slack.BotToken
				if botToken == "" && cfg.Channels.Slack.BotTokenEnv != "" {
					botToken = getSecret(cfg.Channels.Slack.BotTokenEnv)
				}
				appToken := cfg.Channels.Slack.AppToken
				if appToken == "" && cfg.Channels.Slack.AppTokenEnv != "" {
					appToken = getSecret(cfg.Channels.Slack.AppTokenEnv)
				}
				isConfigured = botToken != "" && appToken != ""
			}
		case "whatsapp":
			if cfg.Channels.WhatsApp != nil {
				accessToken := cfg.Channels.WhatsApp.AccessToken
				if accessToken == "" && cfg.Channels.WhatsApp.AccessTokenEnv != "" {
					accessToken = getSecret(cfg.Channels.WhatsApp.AccessTokenEnv)
				}
				isConfigured = (accessToken != "" || cfg.Channels.WhatsApp.PhoneNumberID != "")
			}
		case "qq":
			if cfg.Channels.QQ != nil && cfg.Channels.QQ.AppID != "" {
				appSecret := cfg.Channels.QQ.AppSecret
				if appSecret == "" && cfg.Channels.QQ.AppSecretEnv != "" {
					appSecret = getSecret(cfg.Channels.QQ.AppSecretEnv)
				}
				isConfigured = appSecret != ""
			}
		case "dingtalk":
			if cfg.Channels.DingTalk != nil && cfg.Channels.DingTalk.ClientID != "" {
				clientSecret := cfg.Channels.DingTalk.ClientSecret
				if clientSecret == "" && cfg.Channels.DingTalk.ClientSecretEnv != "" {
					clientSecret = getSecret(cfg.Channels.DingTalk.ClientSecretEnv)
				}
				isConfigured = clientSecret != ""
			}
		case "feishu":
			if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.AppID != "" {
				appSecret := cfg.Channels.Feishu.AppSecret
				if appSecret == "" && cfg.Channels.Feishu.AppSecretEnv != "" {
					appSecret = getSecret(cfg.Channels.Feishu.AppSecretEnv)
				}
				isConfigured = appSecret != ""
			}
		case "weixin":
			if cfg.Channels.Weixin != nil {
				token := cfg.Channels.Weixin.Token
				if token == "" && cfg.Channels.Weixin.TokenEnv != "" {
					token = getSecret(cfg.Channels.Weixin.TokenEnv)
				}
				isConfigured = token != ""
			}
		}

		if !isConfigured {
			continue
		}

		newChannel := &Channel{
			Name:  ct.name,
			Type:  ct.typ,
			Owner: owner,
			State: ChannelStateIdle,
		}

		switch ct.name {
		case "telegram":
			botToken := cfg.Channels.Telegram.BotToken
			if botToken == "" && cfg.Channels.Telegram.BotTokenEnv != "" {
				botToken = getSecret(cfg.Channels.Telegram.BotTokenEnv)
			}
			newChannel.Config.Telegram = &TelegramChannelConfig{
				BotToken: botToken,
			}
		case "discord":
			botToken := cfg.Channels.Discord.BotToken
			if botToken == "" && cfg.Channels.Discord.BotTokenEnv != "" {
				botToken = getSecret(cfg.Channels.Discord.BotTokenEnv)
			}
			newChannel.Config.Discord = &DiscordChannelConfig{
				BotToken: botToken,
			}
		case "slack":
			botToken := cfg.Channels.Slack.BotToken
			if botToken == "" && cfg.Channels.Slack.BotTokenEnv != "" {
				botToken = getSecret(cfg.Channels.Slack.BotTokenEnv)
			}
			appToken := cfg.Channels.Slack.AppToken
			if appToken == "" && cfg.Channels.Slack.AppTokenEnv != "" {
				appToken = getSecret(cfg.Channels.Slack.AppTokenEnv)
			}
			newChannel.Config.Slack = &SlackChannelConfig{
				BotToken: botToken,
				AppToken: appToken,
			}
		case "whatsapp":
			accessToken := cfg.Channels.WhatsApp.AccessToken
			if accessToken == "" && cfg.Channels.WhatsApp.AccessTokenEnv != "" {
				accessToken = getSecret(cfg.Channels.WhatsApp.AccessTokenEnv)
			}
			newChannel.Config.WhatsApp = &WhatsAppChannelConfig{
				AccessToken: accessToken,
				PhoneID:     cfg.Channels.WhatsApp.PhoneNumberID,
			}
		case "qq":
			appSecret := cfg.Channels.QQ.AppSecret
			if appSecret == "" && cfg.Channels.QQ.AppSecretEnv != "" {
				appSecret = getSecret(cfg.Channels.QQ.AppSecretEnv)
			}
			newChannel.Config.QQ = &QQChannelConfig{
				AppID:     cfg.Channels.QQ.AppID,
				AppSecret: appSecret,
			}
		case "dingtalk":
			clientID := cfg.Channels.DingTalk.ClientID
			if clientID == "" && cfg.Channels.DingTalk.ClientIDEnv != "" {
				clientID = getSecret(cfg.Channels.DingTalk.ClientIDEnv)
			}
			clientSecret := cfg.Channels.DingTalk.ClientSecret
			if clientSecret == "" && cfg.Channels.DingTalk.ClientSecretEnv != "" {
				clientSecret = getSecret(cfg.Channels.DingTalk.ClientSecretEnv)
			}
			newChannel.Config.DingTalk = &DingTalkChannelConfig{
				ClientID:     clientID,
				ClientSecret: clientSecret,
			}
		case "feishu":
			appSecret := cfg.Channels.Feishu.AppSecret
			if appSecret == "" && cfg.Channels.Feishu.AppSecretEnv != "" {
				appSecret = getSecret(cfg.Channels.Feishu.AppSecretEnv)
			}
			newChannel.Config.Feishu = &FeishuChannelConfig{
				AppID:     cfg.Channels.Feishu.AppID,
				AppSecret: appSecret,
			}
		case "weixin":
			token := cfg.Channels.Weixin.Token
			if token == "" && cfg.Channels.Weixin.TokenEnv != "" {
				token = getSecret(cfg.Channels.Weixin.TokenEnv)
			}
			newChannel.Config.Weixin = &WeixinChannelConfig{
				Token:              token,
				BaseURL:            cfg.Channels.Weixin.BaseURL,
				CDNBaseURL:         cfg.Channels.Weixin.CDNBaseURL,
				Proxy:              cfg.Channels.Weixin.Proxy,
				ReasoningChannelID: cfg.Channels.Weixin.ReasoningChannelID,
				DefaultAgent:       cfg.Channels.Weixin.DefaultAgent,
			}
		}

		if err := registry.RegisterChannel(newChannel); err == nil {
			if adapter, ok := registry.GetAdapter(newChannel.ID); ok {
				if err := adapter.Start(); err == nil {
					started = append(started, ct.name)
				}
			}
		}
	}

	return started, nil
}
