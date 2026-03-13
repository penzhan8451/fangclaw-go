// Package channels provides channel adapters for OpenFang.
package channels

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Registry manages channel adapters.
type Registry struct {
	mu        sync.RWMutex
	channels  map[string]*Channel
	adapters  map[string]Adapter
	factories map[ChannelType]AdapterFactory
}

// NewRegistry creates a new channel registry.
func NewRegistry() *Registry {
	r := &Registry{
		channels:  make(map[string]*Channel),
		adapters:  make(map[string]Adapter),
		factories: make(map[ChannelType]AdapterFactory),
	}
	r.registerDefaultFactories()
	return r
}

// registerDefaultFactories registers default channel adapter factories.
func (r *Registry) registerDefaultFactories() {
	// Register core channels
	r.RegisterFactory(ChannelTypeTelegram, NewTelegramAdapter)
	r.RegisterFactory(ChannelTypeDiscord, NewDiscordAdapter)
	r.RegisterFactory(ChannelTypeSlack, NewSlackAdapter)
	r.RegisterFactory(ChannelTypeWhatsApp, NewWhatsAppAdapter)
	r.RegisterFactory(ChannelTypeSignal, NewSignalAdapter)
	r.RegisterFactory(ChannelTypeMatrix, NewMatrixAdapter)
	r.RegisterFactory(ChannelTypeEmail, NewEmailAdapter)

	// Register enterprise channels
	r.RegisterFactory(ChannelTypeTeams, NewTeamsAdapter)
	r.RegisterFactory(ChannelTypeMattermost, NewMattermostAdapter)
	r.RegisterFactory(ChannelTypeGoogleChat, NewGoogleChatAdapter)
	r.RegisterFactory(ChannelTypeWebex, NewWebexAdapter)
	r.RegisterFactory(ChannelTypeFeishu, NewFeishuAdapter)
	r.RegisterFactory(ChannelTypeZulip, NewZulipAdapter)

	// Register social channels
	r.RegisterFactory(ChannelTypeLINE, NewLINEAdapter)
	r.RegisterFactory(ChannelTypeViber, NewViberAdapter)
	r.RegisterFactory(ChannelTypeMessenger, NewMessengerAdapter)
	r.RegisterFactory(ChannelTypeMastodon, NewMastodonAdapter)
	r.RegisterFactory(ChannelTypeBluesky, NewBlueskyAdapter)
	r.RegisterFactory(ChannelTypeReddit, NewRedditAdapter)
	r.RegisterFactory(ChannelTypeLinkedIn, NewLinkedInAdapter)
	r.RegisterFactory(ChannelTypeTwitch, NewTwitchAdapter)

	// Register community channels
	r.RegisterFactory(ChannelTypeIRC, NewIRCAdapter)
	r.RegisterFactory(ChannelTypeXMPP, NewXMPPAdapter)
	r.RegisterFactory(ChannelTypeGuilded, NewGuildedAdapter)
	r.RegisterFactory(ChannelTypeRevolt, NewRevoltAdapter)
	r.RegisterFactory(ChannelTypeKeybase, NewKeybaseAdapter)
	r.RegisterFactory(ChannelTypeDiscourse, NewDiscourseAdapter)
	r.RegisterFactory(ChannelTypeGitter, NewGitterAdapter)

	// Register privacy channels
	r.RegisterFactory(ChannelTypeThreema, NewThreemaAdapter)
	r.RegisterFactory(ChannelTypeNostr, NewNostrAdapter)
	r.RegisterFactory(ChannelTypeMumble, NewMumbleAdapter)
	r.RegisterFactory(ChannelTypeNextcloud, NewNextcloudAdapter)
	r.RegisterFactory(ChannelTypeRocketChat, NewRocketChatAdapter)
	r.RegisterFactory(ChannelTypeNtfy, NewNtfyAdapter)
	r.RegisterFactory(ChannelTypeGotify, NewGotifyAdapter)

	// Register workplace channels
	r.RegisterFactory(ChannelTypePumble, NewPumbleAdapter)
	r.RegisterFactory(ChannelTypeFlock, NewFlockAdapter)
	r.RegisterFactory(ChannelTypeTwist, NewTwistAdapter)
	r.RegisterFactory(ChannelTypeDingTalk, NewDingTalkAdapter)
	r.RegisterFactory(ChannelTypeQQ, NewQQAdapter)
	r.RegisterFactory(ChannelTypeZalo, NewZaloAdapter)
	r.RegisterFactory(ChannelTypeWebhook, NewWebhookAdapter)
}

// RegisterFactory registers a channel adapter factory.
func (r *Registry) RegisterFactory(channelType ChannelType, factory AdapterFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[channelType] = factory
}

// GetFactory returns a channel adapter factory by channel type.
func (r *Registry) GetFactory(channelType ChannelType) (AdapterFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[channelType]
	return factory, ok
}

// RegisterChannel registers a channel.
func (r *Registry) RegisterChannel(channel *Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if channel.ID == "" {
		channel.ID = uuid.New().String()
	}

	channel.CreatedAt = time.Now()
	channel.UpdatedAt = time.Now()

	r.channels[channel.ID] = channel

	// Create and connect adapter
	factory, ok := r.factories[channel.Type]
	if ok {
		adapter, err := factory(channel)
		if err == nil {
			r.adapters[channel.ID] = adapter
			_ = adapter.Connect() // Ignore error for now
		}
	}

	return nil
}

// GetChannel returns a channel by ID.
func (r *Registry) GetChannel(id string) (*Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	channel, ok := r.channels[id]
	return channel, ok
}

// ListChannels returns all channels.
func (r *Registry) ListChannels() []*Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	channels := make([]*Channel, 0, len(r.channels))
	for _, channel := range r.channels {
		channels = append(channels, channel)
	}
	return channels
}

// GetChannelByName returns a channel by name.
func (r *Registry) GetChannelByName(name string) (*Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, channel := range r.channels {
		if channel.Name == name {
			return channel, true
		}
	}
	return nil, false
}

// RemoveChannel removes a channel by ID.
func (r *Registry) RemoveChannel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Disconnect adapter if it exists
	if adapter, ok := r.adapters[id]; ok {
		_ = adapter.Disconnect() // Ignore error for now
		delete(r.adapters, id)
	}

	// Remove channel
	delete(r.channels, id)
	return nil
}

// GetAdapter returns an adapter for a channel by ID.
func (r *Registry) GetAdapter(channelID string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[channelID]
	return adapter, ok
}

// ListAdapters returns all registered adapters.
func (r *Registry) ListAdapters() map[string]Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Adapter, len(r.adapters))
	for id, adapter := range r.adapters {
		result[id] = adapter
	}
	return result
}

// UpdateChannel updates a channel.
func (r *Registry) UpdateChannel(channel *Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.channels[channel.ID]
	if !ok {
		return nil
	}

	// Update fields
	existing.Name = channel.Name
	existing.Config = channel.Config
	existing.UpdatedAt = time.Now()

	// Reconnect adapter if needed
	if adapter, ok := r.adapters[channel.ID]; ok {
		_ = adapter.Disconnect() // Ignore error for now
		factory, ok := r.factories[existing.Type]
		if ok {
			newAdapter, err := factory(existing)
			if err == nil {
				r.adapters[channel.ID] = newAdapter
				_ = newAdapter.Connect() // Ignore error for now
			}
		}
	}

	return nil
}

// ConnectAll connects all channels.
func (r *Registry) ConnectAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for channelID, adapter := range r.adapters {
		if err := adapter.Connect(); err != nil {
			// Update channel state
			if channel, ok := r.channels[channelID]; ok {
				channel.State = ChannelStateError
				channel.UpdatedAt = time.Now()
			}
		} else {
			// Update channel state
			if channel, ok := r.channels[channelID]; ok {
				channel.State = ChannelStateConnected
				channel.UpdatedAt = time.Now()
			}
		}
	}

	return nil
}

// DisconnectAll disconnects all channels.
func (r *Registry) DisconnectAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for channelID, adapter := range r.adapters {
		if err := adapter.Disconnect(); err != nil {
			// Update channel state
			if channel, ok := r.channels[channelID]; ok {
				channel.State = ChannelStateError
				channel.UpdatedAt = time.Now()
			}
		} else {
			// Update channel state
			if channel, ok := r.channels[channelID]; ok {
				channel.State = ChannelStateDisconnected
				channel.UpdatedAt = time.Now()
			}
		}
	}

	return nil
}
