package types

import (
	"time"
)

// ChannelType represents the type of messaging channel.
type ChannelType string

const (
	ChannelTypeTelegram  ChannelType = "telegram"
	ChannelTypeWhatsApp  ChannelType = "whatsapp"
	ChannelTypeSlack     ChannelType = "slack"
	ChannelTypeDiscord   ChannelType = "discord"
	ChannelTypeSignal    ChannelType = "signal"
	ChannelTypeMatrix    ChannelType = "matrix"
	ChannelTypeEmail     ChannelType = "email"
	ChannelTypeTeams     ChannelType = "teams"
	ChannelTypeMattermost ChannelType = "mattermost"
	ChannelTypeWebChat   ChannelType = "webchat"
	ChannelTypeCLI       ChannelType = "cli"
)

// ChannelUser represents a user on a messaging platform.
type ChannelUser struct {
	PlatformID   string `json:"platform_id"`
	DisplayName  string `json:"display_name"`
	OpenfangUser string `json:"openfang_user,omitempty"`
}

// AgentPhase represents the agent lifecycle phase for UX indicators.
type AgentPhase string

const (
	AgentPhaseQueued    AgentPhase = "queued"
	AgentPhaseThinking  AgentPhase = "thinking"
	AgentPhaseToolUse   AgentPhase = "tool_use"
	AgentPhaseStreaming AgentPhase = "streaming"
	AgentPhaseDone      AgentPhase = "done"
	AgentPhaseError     AgentPhase = "error"
)

// AgentPhaseWithToolName includes the tool name for tool use phase.
type AgentPhaseWithToolName struct {
	Phase     AgentPhase `json:"phase"`
	ToolName  string     `json:"tool_name,omitempty"`
}

// LifecycleReaction represents a reaction to show in a channel.
type LifecycleReaction struct {
	Phase           AgentPhase `json:"phase"`
	Emoji           string     `json:"emoji"`
	RemovePrevious  bool       `json:"remove_previous"`
}

// DeliveryStatus represents the delivery status for outbound messages.
type DeliveryStatus string

const (
	DeliveryStatusSent       DeliveryStatus = "sent"
	DeliveryStatusDelivered  DeliveryStatus = "delivered"
	DeliveryStatusFailed     DeliveryStatus = "failed"
	DeliveryStatusBestEffort DeliveryStatus = "best_effort"
)

// DeliveryReceipt tracks outbound message delivery.
type DeliveryReceipt struct {
	MessageID  string         `json:"message_id"`
	Channel    string         `json:"channel"`
	Recipient  string         `json:"recipient"`
	Status     DeliveryStatus `json:"status"`
	Timestamp  time.Time      `json:"timestamp"`
	Error      string         `json:"error,omitempty"`
}

// ChannelStatus represents the health status for a channel adapter.
type ChannelStatus struct {
	Connected         bool       `json:"connected"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	LastMessageAt     *time.Time `json:"last_message_at,omitempty"`
	MessagesReceived  uint64     `json:"messages_received"`
	MessagesSent      uint64     `json:"messages_sent"`
}

// Default phase emojis.
var DefaultPhaseEmojis = map[AgentPhase]string{
	AgentPhaseQueued:    "⏳",
	AgentPhaseThinking:  "🤔",
	AgentPhaseToolUse:   "⚙️",
	AgentPhaseStreaming: "✍️",
	AgentPhaseDone:      "✅",
	AgentPhaseError:     "❌",
}

// AllowedReactionEmojis is the list of allowed reaction emojis.
var AllowedReactionEmojis = []string{
	"🤔", "⚙️", "✍️", "✅", "❌", "⏳", "🔄", "👀",
}
