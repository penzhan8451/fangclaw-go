package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackAdapter implements the Adapter interface for Slack.
type SlackAdapter struct {
	*BaseAdapter
	client       *http.Client
	pollInterval time.Duration
	shutdown     chan struct{}
	msgChan      chan *Message
}

// NewSlackAdapter creates a new Slack adapter.
func NewSlackAdapter(channel *Channel) (Adapter, error) {
	return &SlackAdapter{
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
func (a *SlackAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from the channel.
func (a *SlackAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from the channel.
func (a *SlackAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message to Slack.
func (a *SlackAdapter) Send(msg *Message) error {
	if a.Channel.Config.SlackBotToken == "" {
		return fmt.Errorf("slack bot token not configured")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (channel ID) required for Slack")
	}

	apiBase := "https://slack.com/api"
	url := fmt.Sprintf("%s/chat.postMessage", apiBase)

	chunks := splitMessage(msg.Content, 3000)
	for i, chunk := range chunks {
		err := a.sendChunk(url, msg.Recipient, chunk)
		if err != nil {
			return err
		}
		// Add delay between chunks to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}

	return nil
}

// sendChunk sends a single message chunk to Slack.
func (a *SlackAdapter) sendChunk(url, channel, chunk string) error {
	payload := map[string]interface{}{
		"channel": channel,
		"text":    chunk,
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.Channel.Config.SlackBotToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response for error details
	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		return fmt.Errorf("slack api decode error: %w", err)
	}

	if !slackResp.OK {
		return fmt.Errorf("slack api error: %s", slackResp.Error)
	}

	return nil
}

// Start starts the Slack adapter.
func (a *SlackAdapter) Start() error {
	go a.pollLoop()
	return nil
}

// Stop stops the Slack adapter.
func (a *SlackAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// pollLoop polls for updates.
func (a *SlackAdapter) pollLoop() {
	for {
		select {
		case <-a.shutdown:
			return
		case <-time.After(a.pollInterval):
			// Slack typically uses Socket Mode for real-time updates
			// This is a placeholder - full implementation would use websocket
		}
	}
}
