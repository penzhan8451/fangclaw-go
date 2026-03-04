// Package channels provides channel adapters for FangClaw.
package channels

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// BaseAdapter provides common functionality for channel adapters.
type BaseAdapter struct {
	Channel       *Channel
	State         ChannelState
	healthMonitor *HealthMonitor
}

// NewBaseAdapter creates a new base adapter.
func NewBaseAdapter(channel *Channel) *BaseAdapter {
	return &BaseAdapter{
		Channel:       channel,
		State:         ChannelStateIdle,
		healthMonitor: NewHealthMonitor(),
	}
}

// GetState returns the current state of the channel.
func (a *BaseAdapter) GetState() ChannelState {
	return a.State
}

// GetInfo returns information about the channel.
func (a *BaseAdapter) GetInfo() map[string]interface{} {
	return map[string]interface{}{
		"channel_id":    a.Channel.ID,
		"channel_name":  a.Channel.Name,
		"channel_type":  a.Channel.Type,
		"channel_state": a.State,
	}
}

// Connect connects to the channel.
func (a *BaseAdapter) Connect() error {
	a.State = ChannelStateConnected
	a.Channel.State = ChannelStateConnected
	a.Channel.UpdatedAt = time.Now()
	a.healthMonitor.Start()
	return nil
}

// Disconnect disconnects from the channel.
func (a *BaseAdapter) Disconnect() error {
	a.State = ChannelStateDisconnected
	a.Channel.State = ChannelStateDisconnected
	a.Channel.UpdatedAt = time.Now()
	a.healthMonitor.Stop()
	return nil
}

// GetHealthStatus returns the health status of the channel adapter.
func (a *BaseAdapter) GetHealthStatus() *types.ChannelStatus {
	return a.healthMonitor.GetHealthStatus()
}

// RecordMessageSent records that a message was sent.
func (a *BaseAdapter) RecordMessageSent() {
	a.healthMonitor.RecordMessageSent()
}

// RecordMessageReceived records that a message was received.
func (a *BaseAdapter) RecordMessageReceived() {
	a.healthMonitor.RecordMessageReceived()
}

// GetChannel returns the channel associated with this adapter.
func (a *BaseAdapter) GetChannel() *Channel {
	return a.Channel
}

// Start starts the channel adapter.
func (a *BaseAdapter) Start() error {
	return nil
}

// Stop stops the channel adapter.
func (a *BaseAdapter) Stop() error {
	return nil
}

// Send sends a message to the channel.
func (a *BaseAdapter) Send(msg *Message) error {
	return fmt.Errorf("send not implemented for %s channel", a.Channel.Type)
}

// Receive receives messages from the channel.
func (a *BaseAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	ch := make(chan *Message)
	close(ch)
	return ch, nil
}

// SignalAdapter implements the Adapter interface for Signal.
type SignalAdapter struct {
	*BaseAdapter
}

// NewSignalAdapter creates a new Signal adapter.
func NewSignalAdapter(channel *Channel) (Adapter, error) {
	return &SignalAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// MatrixAdapter implements the Adapter interface for Matrix.
type MatrixAdapter struct {
	*BaseAdapter
}

// NewMatrixAdapter creates a new Matrix adapter.
func NewMatrixAdapter(channel *Channel) (Adapter, error) {
	return &MatrixAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// EmailAdapter implements the Adapter interface for Email.
type EmailAdapter struct {
	*BaseAdapter
}

// NewEmailAdapter creates a new Email adapter.
func NewEmailAdapter(channel *Channel) (Adapter, error) {
	return &EmailAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// TeamsAdapter implements the Adapter interface for Microsoft Teams.
type TeamsAdapter struct {
	*BaseAdapter
}

// NewTeamsAdapter creates a new Teams adapter.
func NewTeamsAdapter(channel *Channel) (Adapter, error) {
	return &TeamsAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// MattermostAdapter implements the Adapter interface for Mattermost.
type MattermostAdapter struct {
	*BaseAdapter
}

// NewMattermostAdapter creates a new Mattermost adapter.
func NewMattermostAdapter(channel *Channel) (Adapter, error) {
	return &MattermostAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// GoogleChatAdapter implements the Adapter interface for Google Chat.
type GoogleChatAdapter struct {
	*BaseAdapter
}

// NewGoogleChatAdapter creates a new Google Chat adapter.
func NewGoogleChatAdapter(channel *Channel) (Adapter, error) {
	return &GoogleChatAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// WebexAdapter implements the Adapter interface for Webex.
type WebexAdapter struct {
	*BaseAdapter
}

// NewWebexAdapter creates a new Webex adapter.
func NewWebexAdapter(channel *Channel) (Adapter, error) {
	return &WebexAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// ZulipAdapter implements the Adapter interface for Zulip.
type ZulipAdapter struct {
	*BaseAdapter
}

// NewZulipAdapter creates a new Zulip adapter.
func NewZulipAdapter(channel *Channel) (Adapter, error) {
	return &ZulipAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// LINEAdapter implements the Adapter interface for LINE.
type LINEAdapter struct {
	*BaseAdapter
}

// NewLINEAdapter creates a new LINE adapter.
func NewLINEAdapter(channel *Channel) (Adapter, error) {
	return &LINEAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// ViberAdapter implements the Adapter interface for Viber.
type ViberAdapter struct {
	*BaseAdapter
}

// NewViberAdapter creates a new Viber adapter.
func NewViberAdapter(channel *Channel) (Adapter, error) {
	return &ViberAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// MessengerAdapter implements the Adapter interface for Facebook Messenger.
type MessengerAdapter struct {
	*BaseAdapter
}

// NewMessengerAdapter creates a new Messenger adapter.
func NewMessengerAdapter(channel *Channel) (Adapter, error) {
	return &MessengerAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// MastodonAdapter implements the Adapter interface for Mastodon.
type MastodonAdapter struct {
	*BaseAdapter
}

// NewMastodonAdapter creates a new Mastodon adapter.
func NewMastodonAdapter(channel *Channel) (Adapter, error) {
	return &MastodonAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// BlueskyAdapter implements the Adapter interface for Bluesky.
type BlueskyAdapter struct {
	*BaseAdapter
}

// NewBlueskyAdapter creates a new Bluesky adapter.
func NewBlueskyAdapter(channel *Channel) (Adapter, error) {
	return &BlueskyAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// RedditAdapter implements the Adapter interface for Reddit.
type RedditAdapter struct {
	*BaseAdapter
}

// NewRedditAdapter creates a new Reddit adapter.
func NewRedditAdapter(channel *Channel) (Adapter, error) {
	return &RedditAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// LinkedInAdapter implements the Adapter interface for LinkedIn.
type LinkedInAdapter struct {
	*BaseAdapter
}

// NewLinkedInAdapter creates a new LinkedIn adapter.
func NewLinkedInAdapter(channel *Channel) (Adapter, error) {
	return &LinkedInAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// TwitchAdapter implements the Adapter interface for Twitch.
type TwitchAdapter struct {
	*BaseAdapter
}

// NewTwitchAdapter creates a new Twitch adapter.
func NewTwitchAdapter(channel *Channel) (Adapter, error) {
	return &TwitchAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// IRCAdapter implements the Adapter interface for IRC.
type IRCAdapter struct {
	*BaseAdapter
}

// NewIRCAdapter creates a new IRC adapter.
func NewIRCAdapter(channel *Channel) (Adapter, error) {
	return &IRCAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// XMPPAdapter implements the Adapter interface for XMPP.
type XMPPAdapter struct {
	*BaseAdapter
}

// NewXMPPAdapter creates a new XMPP adapter.
func NewXMPPAdapter(channel *Channel) (Adapter, error) {
	return &XMPPAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// GuildedAdapter implements the Adapter interface for Guilded.
type GuildedAdapter struct {
	*BaseAdapter
}

// NewGuildedAdapter creates a new Guilded adapter.
func NewGuildedAdapter(channel *Channel) (Adapter, error) {
	return &GuildedAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// RevoltAdapter implements the Adapter interface for Revolt.
type RevoltAdapter struct {
	*BaseAdapter
}

// NewRevoltAdapter creates a new Revolt adapter.
func NewRevoltAdapter(channel *Channel) (Adapter, error) {
	return &RevoltAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// KeybaseAdapter implements the Adapter interface for Keybase.
type KeybaseAdapter struct {
	*BaseAdapter
}

// NewKeybaseAdapter creates a new Keybase adapter.
func NewKeybaseAdapter(channel *Channel) (Adapter, error) {
	return &KeybaseAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// DiscourseAdapter implements the Adapter interface for Discourse.
type DiscourseAdapter struct {
	*BaseAdapter
}

// NewDiscourseAdapter creates a new Discourse adapter.
func NewDiscourseAdapter(channel *Channel) (Adapter, error) {
	return &DiscourseAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// GitterAdapter implements the Adapter interface for Gitter.
type GitterAdapter struct {
	*BaseAdapter
}

// NewGitterAdapter creates a new Gitter adapter.
func NewGitterAdapter(channel *Channel) (Adapter, error) {
	return &GitterAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// ThreemaAdapter implements the Adapter interface for Threema.
type ThreemaAdapter struct {
	*BaseAdapter
}

// NewThreemaAdapter creates a new Threema adapter.
func NewThreemaAdapter(channel *Channel) (Adapter, error) {
	return &ThreemaAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// NostrAdapter implements the Adapter interface for Nostr.
type NostrAdapter struct {
	*BaseAdapter
}

// NewNostrAdapter creates a new Nostr adapter.
func NewNostrAdapter(channel *Channel) (Adapter, error) {
	return &NostrAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// MumbleAdapter implements the Adapter interface for Mumble.
type MumbleAdapter struct {
	*BaseAdapter
}

// NewMumbleAdapter creates a new Mumble adapter.
func NewMumbleAdapter(channel *Channel) (Adapter, error) {
	return &MumbleAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// NextcloudAdapter implements the Adapter interface for Nextcloud.
type NextcloudAdapter struct {
	*BaseAdapter
}

// NewNextcloudAdapter creates a new Nextcloud adapter.
func NewNextcloudAdapter(channel *Channel) (Adapter, error) {
	return &NextcloudAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// RocketChatAdapter implements the Adapter interface for Rocket.Chat.
type RocketChatAdapter struct {
	*BaseAdapter
}

// NewRocketChatAdapter creates a new Rocket.Chat adapter.
func NewRocketChatAdapter(channel *Channel) (Adapter, error) {
	return &RocketChatAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// NtfyAdapter implements the Adapter interface for Ntfy.
type NtfyAdapter struct {
	*BaseAdapter
}

// NewNtfyAdapter creates a new Ntfy adapter.
func NewNtfyAdapter(channel *Channel) (Adapter, error) {
	return &NtfyAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// GotifyAdapter implements the Adapter interface for Gotify.
type GotifyAdapter struct {
	*BaseAdapter
}

// NewGotifyAdapter creates a new Gotify adapter.
func NewGotifyAdapter(channel *Channel) (Adapter, error) {
	return &GotifyAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// PumbleAdapter implements the Adapter interface for Pumble.
type PumbleAdapter struct {
	*BaseAdapter
}

// NewPumbleAdapter creates a new Pumble adapter.
func NewPumbleAdapter(channel *Channel) (Adapter, error) {
	return &PumbleAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// FlockAdapter implements the Adapter interface for Flock.
type FlockAdapter struct {
	*BaseAdapter
}

// NewFlockAdapter creates a new Flock adapter.
func NewFlockAdapter(channel *Channel) (Adapter, error) {
	return &FlockAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// TwistAdapter implements the Adapter interface for Twist.
type TwistAdapter struct {
	*BaseAdapter
}

// NewTwistAdapter creates a new Twist adapter.
func NewTwistAdapter(channel *Channel) (Adapter, error) {
	return &TwistAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// ZaloAdapter implements the Adapter interface for Zalo.
type ZaloAdapter struct {
	*BaseAdapter
}

// NewZaloAdapter creates a new Zalo adapter.
func NewZaloAdapter(channel *Channel) (Adapter, error) {
	return &ZaloAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// WebhookAdapter implements the Adapter interface for Webhook.
type WebhookAdapter struct {
	*BaseAdapter
}

// NewWebhookAdapter creates a new Webhook adapter.
func NewWebhookAdapter(channel *Channel) (Adapter, error) {
	return &WebhookAdapter{
		BaseAdapter: NewBaseAdapter(channel),
	}, nil
}

// SendMessage sends a message through the specified channel.
func SendMessage(registry *Registry, channelID string, content string, metadata interface{}) (*Message, error) {
	_, ok := registry.GetChannel(channelID)
	if !ok {
		return nil, fmt.Errorf("channel not found")
	}

	adapter, ok := registry.GetAdapter(channelID)
	if !ok {
		return nil, fmt.Errorf("adapter not found for channel")
	}

	message := &Message{
		ID:        uuid.New().String(),
		ChannelID: channelID,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	err := adapter.Send(message)
	if err != nil {
		return nil, err
	}

	return message, nil
}
