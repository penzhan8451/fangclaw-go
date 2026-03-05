package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WhatsAppAdapter implements the Adapter interface for WhatsApp Business API.
type WhatsAppAdapter struct {
	*BaseAdapter
	client        *http.Client
	pollInterval  time.Duration
	shutdown      chan struct{}
	msgChan       chan *Message
}

// NewWhatsAppAdapter creates a new WhatsApp adapter.
func NewWhatsAppAdapter(channel *Channel) (Adapter, error) {
	return &WhatsAppAdapter{
		BaseAdapter: NewBaseAdapter(channel),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		pollInterval: 1 * time.Second,
		shutdown: make(chan struct{}),
		msgChan: make(chan *Message, 100),
	}, nil
}

// Connect connects to WhatsApp.
func (a *WhatsAppAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from WhatsApp.
func (a *WhatsAppAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from WhatsApp.
func (a *WhatsAppAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message via WhatsApp Business API.
func (a *WhatsAppAdapter) Send(msg *Message) error {
	if a.Channel.Config.WhatsApp == nil || a.Channel.Config.WhatsApp.PhoneID == "" || a.Channel.Config.WhatsApp.AccessToken == "" {
		return fmt.Errorf("whatsapp phone id or access token not configured")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (phone number) required for WhatsApp")
	}

	apiBase := "https://graph.facebook.com/v18.0"
	url := fmt.Sprintf("%s/%s/messages", apiBase, a.Channel.Config.WhatsApp.PhoneID)
	
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                msg.Recipient,
		"text": map[string]string{
			"body": msg.Content,
		},
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.Channel.Config.WhatsApp.AccessToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp api error: %s", resp.Status)
	}

	return nil
}

// Start starts the WhatsApp adapter.
func (a *WhatsAppAdapter) Start() error {
	go a.pollLoop()
	return nil
}

// Stop stops the WhatsApp adapter.
func (a *WhatsAppAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// pollLoop polls for updates (placeholder - WhatsApp uses webhooks).
func (a *WhatsAppAdapter) pollLoop() {
	for {
		select {
		case <-a.shutdown:
			return
		case <-time.After(a.pollInterval):
			// WhatsApp typically uses Webhooks for real-time updates
			// This is a placeholder - full implementation would use webhook
		}
	}
}
