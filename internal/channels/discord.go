package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordAdapter implements the Adapter interface for Discord.
type DiscordAdapter struct {
	*BaseAdapter
	client       *http.Client
	pollInterval time.Duration
	shutdown     chan struct{}
	msgChan      chan *Message
}

// NewDiscordAdapter creates a new Discord adapter.
func NewDiscordAdapter(channel *Channel) (Adapter, error) {
	return &DiscordAdapter{
		BaseAdapter: NewBaseAdapter(channel),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		pollInterval: 1 * time.Second,
		shutdown:     make(chan struct{}),
		msgChan:      make(chan *Message, 100),
	}, nil
}

// Connect connects to the channel.
func (a *DiscordAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from the channel.
func (a *DiscordAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from the channel.
func (a *DiscordAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message to Discord.
func (a *DiscordAdapter) Send(msg *Message) error {
	if a.Channel.Config.Discord == nil || a.Channel.Config.Discord.BotToken == "" {
		return fmt.Errorf("discord bot token not configured")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (channel ID) required for Discord")
	}

	apiBase := "https://discord.com/api/v10"
	url := fmt.Sprintf("%s/channels/%s/messages", apiBase, msg.Recipient)

	chunks := splitMessage(msg.Content, 2000)
	for i, chunk := range chunks {
		err := a.sendChunk(url, chunk)
		if err != nil {
			return err
		}
		// Add delay between chunks to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

// sendChunk sends a single message chunk to Discord.
func (a *DiscordAdapter) sendChunk(url, chunk string) error {
	payload := map[string]interface{}{
		"content": chunk,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", a.Channel.Config.Discord.BotToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response for error details
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errorResp struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		if errorResp.Message != "" {
			return fmt.Errorf("discord api error: %s (code: %d)", errorResp.Message, errorResp.Code)
		}
		return fmt.Errorf("discord api error: %s", resp.Status)
	}

	return nil
}

// Start starts the Discord adapter.
func (a *DiscordAdapter) Start() error {
	go a.pollLoop()
	return nil
}

// Stop stops the Discord adapter.
func (a *DiscordAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// pollLoop polls for updates (placeholder - Discord uses WebSocket).
func (a *DiscordAdapter) pollLoop() {
	for {
		select {
		case <-a.shutdown:
			return
		case <-time.After(a.pollInterval):
			// Discord typically uses WebSocket Gateway for real-time updates
			// This is a placeholder - full implementation would use websocket
		}
	}
}
