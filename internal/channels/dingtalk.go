package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DingTalkAdapter implements the Adapter interface for DingTalk.
type DingTalkAdapter struct {
	*BaseAdapter
	client       *http.Client
	pollInterval time.Duration
	shutdown     chan struct{}
	msgChan      chan *Message
	tokenCache   string
	tokenExpiry  time.Time
}

// NewDingTalkAdapter creates a new DingTalk adapter.
func NewDingTalkAdapter(channel *Channel) (Adapter, error) {
	return &DingTalkAdapter{
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
func (a *DingTalkAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from the channel.
func (a *DingTalkAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from the channel.
func (a *DingTalkAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message to DingTalk.
func (a *DingTalkAdapter) Send(msg *Message) error {
	if a.Channel.Config.DingTalkAppKey == "" || a.Channel.Config.DingTalkAppSecret == "" {
		return fmt.Errorf("dingtalk app key or secret not configured")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (user id) required for DingTalk")
	}

	apiBase := "https://oapi.dingtalk.com"

	accessToken, err := a.getAccessToken(apiBase)
	if err != nil {
		return err
	}

	// DingTalk text message limit
	const chunkSize = 4000
	chunks := splitMessage(msg.Content, chunkSize)

	for i, chunk := range chunks {
		err := a.sendChunk(apiBase, accessToken, msg.Recipient, chunk)
		if err != nil {
			return err
		}
		// Add delay between chunks
		if i < len(chunks)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}

	return nil
}

// sendChunk sends a single message chunk to DingTalk.
func (a *DingTalkAdapter) sendChunk(apiBase, token, recipient, chunk string) error {
	url := fmt.Sprintf("%s/topapi/message/corpconversation/asyncsend_v2?access_token=%s", apiBase, token)
	payload := map[string]interface{}{
		"agent_id":    a.Channel.Config.DingTalkAgentID,
		"userid_list": recipient,
		"msg": map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": chunk,
			},
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

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response for error details
	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if result.Errcode != 0 {
			return fmt.Errorf("dingtalk api error: %s (code: %d)", result.Errmsg, result.Errcode)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk api error: %s", resp.Status)
	}

	return nil
}

// getAccessToken gets DingTalk access token with caching.
func (a *DingTalkAdapter) getAccessToken(apiBase string) (string, error) {
	// Check if token is still valid (cache for 1 hour 50 minutes)
	if a.tokenCache != "" && time.Now().Before(a.tokenExpiry) {
		return a.tokenCache, nil
	}

	url := fmt.Sprintf("%s/gettoken?appkey=%s&appsecret=%s", apiBase, a.Channel.Config.DingTalkAppKey, a.Channel.Config.DingTalkAppSecret)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Errcode     int    `json:"errcode"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Errmsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("dingtalk auth error: %s", result.Errmsg)
	}

	// Cache the token
	a.tokenCache = result.AccessToken
	expireSeconds := result.ExpiresIn
	if expireSeconds == 0 {
		expireSeconds = 7200 // Default to 2 hours
	}
	a.tokenExpiry = time.Now().Add(time.Duration(expireSeconds-600) * time.Second)

	return result.AccessToken, nil
}

// Start starts the DingTalk adapter.
func (a *DingTalkAdapter) Start() error {
	go a.pollLoop()
	return nil
}

// Stop stops the DingTalk adapter.
func (a *DingTalkAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// pollLoop polls for updates.
func (a *DingTalkAdapter) pollLoop() {
	for {
		select {
		case <-a.shutdown:
			return
		case <-time.After(a.pollInterval):
			// DingTalk typically uses Webhook or Event Callback
			// This is a placeholder - full implementation would use webhook
		}
	}
}
