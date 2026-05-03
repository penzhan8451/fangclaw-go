package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TelegramAdapter implements the Adapter interface for Telegram.
type TelegramAdapter struct {
	*BaseAdapter
	client       *http.Client
	allowedUsers []int64
	pollInterval time.Duration
	lastUpdateID int64
	shutdown     chan struct{}
	msgChan      chan *Message
}

// NewTelegramAdapter creates a new Telegram adapter.
func NewTelegramAdapter(channel *Channel) (Adapter, error) {
	return &TelegramAdapter{
		BaseAdapter: NewBaseAdapter(channel),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		allowedUsers: []int64{},
		pollInterval: 1 * time.Second,
		lastUpdateID: 0,
		shutdown:     make(chan struct{}),
		msgChan:      make(chan *Message, 100),
	}, nil
}

// Connect connects to the channel.
func (a *TelegramAdapter) Connect() error {
	return a.Start()
}

// Disconnect disconnects from the channel.
func (a *TelegramAdapter) Disconnect() error {
	return a.Stop()
}

// Receive receives messages from the channel.
func (a *TelegramAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message to Telegram.
func (a *TelegramAdapter) Send(msg *Message) error {
	if a.Channel.Config.Telegram == nil || a.Channel.Config.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	chatID, err := strconv.ParseInt(msg.Recipient, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	return a.apiSendMessage(chatID, msg.Content)
}

// Start starts the Telegram adapter.
func (a *TelegramAdapter) Start() error {
	// Validate token first
	botName, err := a.validateToken()
	if err != nil {
		return err
	}
	log.Info().Str("bot", botName).Msg("Telegram bot connected")

	// Start polling in a goroutine
	go a.pollLoop()
	return nil
}

// Stop stops the Telegram adapter.
func (a *TelegramAdapter) Stop() error {
	close(a.shutdown)
	return nil
}

// validateToken validates the bot token by calling getMe.
func (a *TelegramAdapter) validateToken() (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", a.Channel.Config.Telegram.BotToken)
	resp, err := a.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if ok, _ := result["ok"].(bool); !ok {
		desc, _ := result["description"].(string)
		return "", fmt.Errorf("telegram getMe failed: %s", desc)
	}

	resultMap, _ := result["result"].(map[string]interface{})
	username, _ := resultMap["username"].(string)
	return username, nil
}

// apiSendMessage calls sendMessage on the Telegram API.
func (a *TelegramAdapter) apiSendMessage(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.Channel.Config.Telegram.BotToken)

	// Split message if needed (Telegram limit is 4096 chars)
	chunks := splitMessage(text, 4096)
	for _, chunk := range chunks {
		payload := map[string]interface{}{
			"chat_id": chatID,
			"text":    chunk,
		}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("telegram sendMessage failed: %s", string(body))
		}
	}
	return nil
}

// pollLoop polls for updates from Telegram.
func (a *TelegramAdapter) pollLoop() {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-a.shutdown:
			log.Info().Msg("Telegram poll loop stopped")
			return
		default:
			updates, err := a.getUpdates()
			if err != nil {
				errMsg := err.Error()
				log.Warn().Err(err).Msg("Telegram poll error")

				if errMsg == "conflict: another bot instance is running" {
					log.Error().Msg("Stopping due to conflict with another bot instance")
					return
				}

				var sleepDuration time.Duration
				if len(errMsg) > 25 && errMsg[:25] == "rate limited, retry after" {
					var retryAfter int
					fmt.Sscanf(errMsg, "rate limited, retry after %d seconds", &retryAfter)
					sleepDuration = time.Duration(retryAfter) * time.Second
				} else {
					sleepDuration = backoff
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}

				select {
				case <-a.shutdown:
					log.Info().Msg("Telegram poll loop stopped")
					return
				case <-time.After(sleepDuration):
				}
				continue
			}
			backoff = 1 * time.Second

			for _, update := range updates {
				a.handleUpdate(update)
			}

			select {
			case <-a.shutdown:
				log.Info().Msg("Telegram poll loop stopped")
				return
			case <-time.After(a.pollInterval):
			}
		}
	}
}

// getUpdates gets updates from Telegram.
func (a *TelegramAdapter) getUpdates() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", a.Channel.Config.Telegram.BotToken)

	payload := map[string]interface{}{
		"offset":          a.lastUpdateID + 1,
		"timeout":         30,
		"allowed_updates": []string{"message", "edited_message"},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle rate limiting (429)
	if resp.StatusCode == 429 {
		var result map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &result)
		if params, ok := result["parameters"].(map[string]interface{}); ok {
			if retryAfter, ok := params["retry_after"].(float64); ok {
				return nil, fmt.Errorf("rate limited, retry after %d seconds", int(retryAfter))
			}
		}
		return nil, fmt.Errorf("rate limited")
	}

	// Handle conflict (409 - another bot instance polling)
	if resp.StatusCode == 409 {
		return nil, fmt.Errorf("conflict: another bot instance is running")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if ok, _ := result["ok"].(bool); !ok {
		desc, _ := result["description"].(string)
		return nil, fmt.Errorf("telegram getUpdates failed: %s", desc)
	}

	resultArr, _ := result["result"].([]interface{})
	updates := make([]map[string]interface{}, 0, len(resultArr))
	for _, u := range resultArr {
		update, _ := u.(map[string]interface{})
		updates = append(updates, update)

		updateID, _ := update["update_id"].(float64)
		if int64(updateID) > a.lastUpdateID {
			a.lastUpdateID = int64(updateID)
		}
	}

	return updates, nil
}

// handleUpdate handles a single Telegram update.
func (a *TelegramAdapter) handleUpdate(update map[string]interface{}) {
	// Try to get message, also support edited_message
	message, _ := update["message"].(map[string]interface{})
	if message == nil {
		message, _ = update["edited_message"].(map[string]interface{})
		if message == nil {
			return
		}
	}

	from, _ := message["from"].(map[string]interface{})
	if from == nil {
		return
	}

	userID, _ := from["id"].(float64)
	chat, _ := message["chat"].(map[string]interface{})
	if chat == nil {
		return
	}

	chatID, _ := chat["id"].(float64)
	text, _ := message["text"].(string)
	if text == "" {
		return
	}

	// Check allowed users
	if len(a.allowedUsers) > 0 {
		allowed := false
		for _, u := range a.allowedUsers {
			if u == int64(userID) {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}

	// Create message
	msg := &Message{
		ID:        generateMessageID(),
		Content:   text,
		Sender:    fmt.Sprintf("%d", int64(userID)),
		Recipient: fmt.Sprintf("%d", int64(chatID)),
		ChannelID: a.Channel.ID,
		CreatedAt: time.Now(),
	}

	// Send to message channel
	select {
	case a.msgChan <- msg:
	case <-time.After(1 * time.Second):
		log.Warn().Msg("Message channel full, dropped message")
	}
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return uuid.NewString()
}

// splitMessage splits a message into chunks of max length.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for i := 0; i < len(text); i += maxLen {
		end := i + maxLen
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])
	}
	return chunks
}
