package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

func init() {
	RegisterAutoRegister(autoRegisterDingTalk)
}

func autoRegisterDingTalk(registry *Registry, getSecret SecretGetter) error {
	dingtalkClientID := getSecret("DINGTALK_CLIENT_ID")
	dingtalkClientSecret := getSecret("DINGTALK_CLIENT_SECRET")

	if dingtalkClientID != "" && dingtalkClientSecret != "" {
		fmt.Println("Auto-registering DingTalk channel...")
		dingtalkChannel := &Channel{
			Name:  "DingTalk Bot",
			Type:  ChannelTypeDingTalk,
			State: ChannelStateIdle,
			Config: ChannelAdapterConfig{
				DingTalk: &DingTalkChannelConfig{
					ClientID:     dingtalkClientID,
					ClientSecret: dingtalkClientSecret,
				},
			},
		}

		if err := registry.RegisterChannel(dingtalkChannel); err != nil {
			fmt.Printf("Warning: Failed to auto-register DingTalk channel: %v\n", err)
			return err
		}
		fmt.Println("DingTalk channel auto-registered successfully")
	}
	return nil
}

// DingTalkAdapter implements the Adapter interface for DingTalk.
type DingTalkAdapter struct {
	*BaseAdapter
	streamClient    *client.StreamClient
	sessionWebhooks sync.Map // chatID -> sessionWebhook
	ctx             context.Context
	cancel          context.CancelFunc
	msgChan         chan *Message
	processedIDs    map[string]bool
	mu              sync.RWMutex
}

// NewDingTalkAdapter creates a new DingTalk adapter.
func NewDingTalkAdapter(channel *Channel) (Adapter, error) {
	return &DingTalkAdapter{
		BaseAdapter:  NewBaseAdapter(channel),
		msgChan:      make(chan *Message, 100),
		processedIDs: make(map[string]bool),
	}, nil
}

// Connect connects to DingTalk.
func (a *DingTalkAdapter) Connect() error {
	return a.BaseAdapter.Connect()
}

// Disconnect disconnects from DingTalk.
func (a *DingTalkAdapter) Disconnect() error {
	if err := a.Stop(); err != nil {
		return err
	}
	return a.BaseAdapter.Disconnect()
}

// Receive receives messages from DingTalk.
func (a *DingTalkAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message to DingTalk.
// Send back the message to the same recipient.
func (a *DingTalkAdapter) Send(msg *Message) error {
	if a.streamClient == nil {
		return fmt.Errorf("dingtalk stream client not initialized")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (chat id) required for DingTalk")
	}

	sessionWebhookRaw, ok := a.sessionWebhooks.Load(msg.Recipient)
	if !ok {
		return fmt.Errorf("no session_webhook found for chat %s, cannot send message", msg.Recipient)
	}

	sessionWebhook, ok := sessionWebhookRaw.(string)
	if !ok {
		return fmt.Errorf("invalid session_webhook type for chat %s", msg.Recipient)
	}

	const maxMessageLen = 20000
	chunks := splitMessage(msg.Content, maxMessageLen)

	for _, chunk := range chunks {
		if err := a.sendChunk(sessionWebhook, chunk); err != nil {
			return err
		}
		if len(chunks) > 1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return nil
}

// sendChunk sends a single message chunk to DingTalk.
func (a *DingTalkAdapter) sendChunk(sessionWebhook, content string) error {
	replier := chatbot.NewChatbotReplier()
	contentBytes := []byte(content)
	titleBytes := []byte("FangClaw")

	err := replier.SimpleReplyMarkdown(
		context.Background(),
		sessionWebhook,
		titleBytes,
		contentBytes,
	)
	if err != nil {
		return fmt.Errorf("dingtalk send: %w", err)
	}

	return nil
}

// Start starts the DingTalk adapter with Stream Mode.
func (a *DingTalkAdapter) Start() error {
	clientID := ""
	clientSecret := ""

	if a.Channel.Config.DingTalk != nil {
		clientID = a.Channel.Config.DingTalk.ClientID
		clientSecret = a.Channel.Config.DingTalk.ClientSecret
	}

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("dingtalk client_id or client_secret not configured (client_id=%q)", clientID)
	}

	fmt.Printf("Starting DingTalk adapter with client_id=%q\n", clientID)

	a.ctx, a.cancel = context.WithCancel(context.Background())

	cred := client.NewAppCredentialConfig(clientID, clientSecret)

	a.streamClient = client.NewStreamClient(
		client.WithAppCredential(cred),
		client.WithAutoReconnect(true),
	)

	a.streamClient.RegisterChatBotCallbackRouter(a.onChatBotMessageReceived)

	go func() {
		if err := a.streamClient.Start(a.ctx); err != nil {
			fmt.Printf("DingTalk WebSocket session error: %v\n", err)
			a.Channel.State = ChannelStateError
		}
	}()

	a.Channel.State = ChannelStateConnected
	return nil
}

// Stop stops the DingTalk adapter.
func (a *DingTalkAdapter) Stop() error {
	a.Channel.State = ChannelStateDisconnected
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.streamClient != nil {
		a.streamClient.Close()
		a.streamClient = nil
	}
	a.ctx = nil
	a.mu.Unlock()
	return nil
}

// onChatBotMessageReceived handles incoming chatbot messages from DingTalk.
func (a *DingTalkAdapter) onChatBotMessageReceived(
	ctx context.Context,
	data *chatbot.BotCallbackDataModel,
) ([]byte, error) {
	content := data.Text.Content
	if content == "" {
		if contentMap, ok := data.Content.(map[string]interface{}); ok {
			if textContent, ok := contentMap["content"].(string); ok {
				content = textContent
			}
		}
	}

	if content == "" {
		return nil, nil
	}

	senderID := data.SenderStaffId
	senderNick := data.SenderNick
	chatID := senderID
	isGroup := false
	if data.ConversationType != "1" {
		chatID = data.ConversationId
		isGroup = true
	}

	a.sessionWebhooks.Store(chatID, data.SessionWebhook)

	messageID := fmt.Sprintf("dt-%d", time.Now().UnixNano())
	if a.isDuplicate(messageID) {
		return nil, nil
	}

	if isGroup && a.Channel.Config.DingTalk != nil && a.Channel.Config.DingTalk.GroupTrigger != "" {
		if !containsTrigger(content, a.Channel.Config.DingTalk.GroupTrigger) {
			return nil, nil
		}
		content = removeTrigger(content, a.Channel.Config.DingTalk.GroupTrigger)
	}

	if !a.isAllowedSender(senderID) {
		return nil, nil
	}

	metadata := map[string]interface{}{
		"sender_name":       senderNick,
		"conversation_id":   data.ConversationId,
		"conversation_type": data.ConversationType,
		"platform":          "dingtalk",
		"session_webhook":   data.SessionWebhook,
	}

	a.msgChan <- &Message{
		ID:        messageID,
		ChannelID: a.Channel.ID,
		Content:   content,
		Sender:    senderID,
		Recipient: chatID,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	return nil, nil
}

// isDuplicate checks if message is duplicate.
func (a *DingTalkAdapter) isDuplicate(messageID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.processedIDs[messageID] {
		return true
	}

	a.processedIDs[messageID] = true

	if len(a.processedIDs) > 10000 {
		count := 0
		for id := range a.processedIDs {
			if count >= 5000 {
				break
			}
			delete(a.processedIDs, id)
			count++
		}
	}

	return false
}

// isAllowedSender checks if sender is allowed.
func (a *DingTalkAdapter) isAllowedSender(senderID string) bool {
	if a.Channel.Config.DingTalk == nil || len(a.Channel.Config.DingTalk.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range a.Channel.Config.DingTalk.AllowFrom {
		if allowed == senderID {
			return true
		}
	}

	return false
}

// containsTrigger checks if content contains the trigger.
func containsTrigger(content, trigger string) bool {
	return len(content) >= len(trigger) && content[:len(trigger)] == trigger
}

// removeTrigger removes the trigger from content.
func removeTrigger(content, trigger string) string {
	if len(content) > len(trigger) && content[len(trigger)] == ' ' {
		return content[len(trigger)+1:]
	}
	return content[len(trigger):]
}
