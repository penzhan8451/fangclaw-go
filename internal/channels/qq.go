package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// QQAdapter implements the Adapter interface for QQ.
type QQAdapter struct {
	*BaseAdapter
	client        *http.Client
	pollInterval  time.Duration
	shutdown      chan struct{}
	msgChan       chan *Message
}

// NewQQAdapter creates a new QQ adapter.
func NewQQAdapter(channel *Channel) (Adapter, error) {
	return &QQAdapter{
		BaseAdapter: NewBaseAdapter(channel),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		pollInterval: 1 * time.Second,
		shutdown: make(chan struct{}),
		msgChan: make(chan *Message, 100),
	}, nil
}

// Connect connects to QQ.
func (a *QQAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from QQ.
func (a *QQAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from QQ.
func (a *QQAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message via QQ Bot API.
func (a *QQAdapter) Send(msg *Message) error {
	if a.Channel.Config.QQBotID == "" || a.Channel.Config.QQBotToken == "" {
		return fmt.Errorf("qq bot id or token not configured")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (group id or user id) required for QQ")
	}

	apiBase := "https://api.sgroup.qq.com"
	
	// QQ message limit
	const chunkSize = 2000
	chunks := splitMessage(msg.Content, chunkSize)

	for i, chunk := range chunks {
		err := a.sendChunk(apiBase, msg.Recipient, chunk)
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

// sendChunk sends a single message chunk via QQ Bot API.
func (a *QQAdapter) sendChunk(apiBase, recipient, chunk string) error {
	url := fmt.Sprintf("%s/v2/users/%s/messages", apiBase, recipient)
	
	payload := map[string]interface{}{
		"content": chunk,
		"msg_type": 0,
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
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s.%s", a.Channel.Config.QQBotID, a.Channel.Config.QQBotToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response for error details
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if result.Message != "" {
			return fmt.Errorf("qq api error: %s (code: %d)", result.Message, result.Code)
		}
		return fmt.Errorf("qq api error: %s", resp.Status)
	}

	return nil
}

// Start starts the QQ adapter.
func (a *QQAdapter) Start() error {
	go a.pollLoop()
	return nil
}

// Stop stops the QQ adapter.
func (a *QQAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// pollLoop polls for updates (placeholder - QQ uses WebSocket).
func (a *QQAdapter) pollLoop() {
	for {
		select {
		case <-a.shutdown:
			return
		case <-time.After(a.pollInterval):
			// QQ typically uses WebSocket Gateway for real-time updates
			// This is a placeholder - full implementation would use websocket
		}
	}
}
