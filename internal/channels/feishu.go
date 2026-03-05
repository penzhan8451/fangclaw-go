package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// FeishuAdapter implements the Adapter interface for Feishu.
type FeishuAdapter struct {
	*BaseAdapter
	client       *http.Client
	pollInterval time.Duration
	shutdown     chan struct{}
	msgChan      chan *Message
	tokenCache   string
	tokenExpiry  time.Time
}

func init() {
	RegisterAutoRegister(autoRegisterFEISHU)
}

func autoRegisterFEISHU(registry *Registry) error {
	FeishuAppID := os.Getenv("FEISHU_APP_ID")
	FeishuAppSecret := os.Getenv("FEISHU_APP_SECRET")

	if FeishuAppID != "" && FeishuAppSecret != "" {
		fmt.Println("Auto Register Fei Shu Channel...")
		if err := registry.RegisterChannel(&Channel{
			Name:  "Feishu Bot",
			Type:  ChannelTypeFeishu,
			State: ChannelStateIdle,
			Config: ChannelConfig{
				FeishuAppID:     FeishuAppID,
				FeishuAppSecret: FeishuAppSecret,
			},
		}); err != nil {
			fmt.Printf("Warning: Failed to auto-register Feishu channel: %v\n", err)
			return err
		}

		fmt.Println("Feishu channel auto-registered successfully")
	}

	return nil
}

// NewFeishuAdapter creates a new Feishu adapter.
func NewFeishuAdapter(channel *Channel) (Adapter, error) {
	return &FeishuAdapter{
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
func (a *FeishuAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from the channel.
func (a *FeishuAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from the channel.
func (a *FeishuAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message to Feishu.
func (a *FeishuAdapter) Send(msg *Message) error {
	if a.Channel.Config.FeishuAppID == "" || a.Channel.Config.FeishuAppSecret == "" {
		return fmt.Errorf("feishu app id or secret not configured")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (open id) required for Feishu")
	}

	apiBase := "https://open.feishu.cn/open-apis"

	tenantAccessToken, err := a.getTenantAccessToken(apiBase)
	if err != nil {
		return err
	}

	// Feishu text message limit is about 5000 characters
	const chunkSize = 4500
	chunks := splitMessage(msg.Content, chunkSize)

	for i, chunk := range chunks {
		err := a.sendChunk(apiBase, tenantAccessToken, msg.Recipient, chunk)
		if err != nil {
			return err
		}
		// Add small delay between chunks to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return nil
}

// sendChunk sends a single message chunk.
func (a *FeishuAdapter) sendChunk(apiBase, token, recipient, chunk string) error {
	url := fmt.Sprintf("%s/im/v1/messages", apiBase)
	payload := map[string]interface{}{
		"receive_id": recipient,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, escapeJSON(chunk)),
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response body for error details
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || result.Code != 0 {
		if result.Msg != "" {
			return fmt.Errorf("feishu api error: %s (code: %d)", result.Msg, result.Code)
		}
		return fmt.Errorf("feishu api error: %s", resp.Status)
	}

	return nil
}

// getTenantAccessToken gets Feishu tenant access token with caching.
func (a *FeishuAdapter) getTenantAccessToken(apiBase string) (string, error) {
	// Check if token is still valid (cache for 1 hour 50 minutes to be safe)
	if a.tokenCache != "" && time.Now().Before(a.tokenExpiry) {
		return a.tokenCache, nil
	}

	url := fmt.Sprintf("%s/auth/v3/tenant_access_token/internal", apiBase)
	payload := map[string]string{
		"app_id":     a.Channel.Config.FeishuAppID,
		"app_secret": a.Channel.Config.FeishuAppSecret,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
		Msg               string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("feishu auth error: %s", result.Msg)
	}

	// Cache the token
	a.tokenCache = result.TenantAccessToken
	expireSeconds := result.Expire
	if expireSeconds == 0 {
		expireSeconds = 7200 // Default to 2 hours
	}
	// Set expiry to 10 minutes before actual expiry for safety
	a.tokenExpiry = time.Now().Add(time.Duration(expireSeconds-600) * time.Second)

	return result.TenantAccessToken, nil
}

// Start starts the Feishu adapter.
func (a *FeishuAdapter) Start() error {
	go a.pollLoop()
	return nil
}

// Stop stops the Feishu adapter.
func (a *FeishuAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// pollLoop polls for updates.
func (a *FeishuAdapter) pollLoop() {
	for {
		select {
		case <-a.shutdown:
			return
		case <-time.After(a.pollInterval):
			// Feishu typically uses Webhook or Event Callback
			// This is a placeholder - full implementation would use webhook
		}
	}
}

// escapeJSON escapes JSON special characters.
func escapeJSON(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteRune('\\')
			buf.WriteRune('"')
		case '\\':
			buf.WriteRune('\\')
			buf.WriteRune('\\')
		case '\n':
			buf.WriteRune('\\')
			buf.WriteRune('n')
		case '\r':
			buf.WriteRune('\\')
			buf.WriteRune('r')
		case '\t':
			buf.WriteRune('\\')
			buf.WriteRune('t')
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
