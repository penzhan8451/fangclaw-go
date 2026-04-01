package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func init() {
	RegisterAutoRegister(autoRegisterFeishu)
}

func autoRegisterFeishu(registry *Registry, getSecret SecretGetter) error {
	feishuAppID := getSecret("FEISHU_APP_ID")
	feishuAppSecret := getSecret("FEISHU_APP_SECRET")

	if feishuAppID != "" && feishuAppSecret != "" {
		fmt.Println("Auto-registering Feishu channel...")
		feishuChannel := &Channel{
			Name:  "Feishu Bot",
			Type:  ChannelTypeFeishu,
			State: ChannelStateIdle,
			Config: ChannelAdapterConfig{
				Feishu: &FeishuChannelConfig{
					AppID:     feishuAppID,
					AppSecret: feishuAppSecret,
				},
			},
		}

		if err := registry.RegisterChannel(feishuChannel); err != nil {
			fmt.Printf("Warning: Failed to auto-register Feishu channel: %v\n", err)
			return err
		}
		fmt.Println("Feishu channel auto-registered successfully")
	}
	return nil
}

type FeishuAdapter struct {
	*BaseAdapter
	client       *lark.Client
	wsClient     *larkws.Client
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	msgChan      chan *Message
	processedIDs map[string]bool
}

func NewFeishuAdapter(channel *Channel) (Adapter, error) {
	return &FeishuAdapter{
		BaseAdapter:  NewBaseAdapter(channel),
		msgChan:      make(chan *Message, 100),
		processedIDs: make(map[string]bool),
	}, nil
}

func (a *FeishuAdapter) Disconnect() error {
	if err := a.Stop(); err != nil {
		return err
	}
	return a.BaseAdapter.Disconnect()
}

func (a *FeishuAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

func (a *FeishuAdapter) Send(msg *Message) error {
	if a.client == nil {
		return fmt.Errorf("feishu client not initialized")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (chat id) required for Feishu")
	}

	cardContent, err := buildMarkdownCard(msg.Content)
	if err != nil {
		return fmt.Errorf("feishu send: card build failed: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(msg.Recipient).
			MsgType(larkim.MsgTypeInteractive).
			Content(cardContent).
			Build()).
		Build()

	resp, err := a.client.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("feishu send: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu send api error (code=%d msg=%s)", resp.Code, resp.Msg)
	}

	return nil
}

func (a *FeishuAdapter) Start() error {
	appID := ""
	appSecret := ""

	if a.Channel.Config.Feishu != nil {
		appID = a.Channel.Config.Feishu.AppID
		appSecret = a.Channel.Config.Feishu.AppSecret
	}

	if appID == "" || appSecret == "" {
		return fmt.Errorf("feishu app id or app secret not configured (app_id=%q)", appID)
	}

	fmt.Printf("Starting Feishu adapter with app_id=%q\n", appID)

	a.client = lark.NewClient(appID, appSecret)

	dispatcher := larkdispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(a.handleMessageReceive)

	a.ctx, a.cancel = context.WithCancel(context.Background())

	a.wsClient = larkws.NewClient(
		appID,
		appSecret,
		larkws.WithEventHandler(dispatcher),
	)

	a.Channel.State = ChannelStateConnected

	go func() {
		if err := a.wsClient.Start(a.ctx); err != nil {
			fmt.Printf("Feishu WebSocket session error: %v\n", err)
			a.Channel.State = ChannelStateError
		}
	}()

	return nil
}

func (a *FeishuAdapter) Stop() error {
	a.Channel.State = ChannelStateDisconnected
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.ctx = nil
	a.wsClient = nil
	a.client = nil
	a.mu.Unlock()
	return nil
}

func (a *FeishuAdapter) isDuplicate(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.processedIDs[id] {
		return true
	}
	a.processedIDs[id] = true
	return false
}

// handleMessageReceive handles Feishu message receive events.
func (a *FeishuAdapter) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}

	message := event.Event.Message
	sender := event.Event.Sender

	messageID := stringValue(message.MessageId)
	if messageID == "" || a.isDuplicate(messageID) {
		return nil
	}

	chatID := stringValue(message.ChatId)
	if chatID == "" {
		return nil
	}

	senderID := extractFeishuSenderID(sender)
	if senderID == "" {
		senderID = "unknown"
	}

	messageType := stringValue(message.MessageType)
	rawContent := stringValue(message.Content)

	content := extractContent(messageType, rawContent)
	if content == "" {
		content = "[empty message]"
	}

	a.msgChan <- &Message{
		ID:        messageID,
		ChannelID: a.Channel.ID,
		Content:   content,
		Sender:    senderID,
		Recipient: chatID,
		CreatedAt: time.Now(),
	}

	return nil
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func extractFeishuSenderID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}

	if sender.SenderId.UserId != nil && *sender.SenderId.UserId != "" {
		return *sender.SenderId.UserId
	}
	if sender.SenderId.OpenId != nil && *sender.SenderId.OpenId != "" {
		return *sender.SenderId.OpenId
	}
	if sender.SenderId.UnionId != nil && *sender.SenderId.UnionId != "" {
		return *sender.SenderId.UnionId
	}

	return ""
}

func extractContent(messageType, rawContent string) string {
	if rawContent == "" {
		return ""
	}

	switch messageType {
	case larkim.MsgTypeText:
		var textPayload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(rawContent), &textPayload); err == nil {
			return textPayload.Text
		}
		return rawContent
	default:
		return rawContent
	}
}

func buildMarkdownCard(content string) (string, error) {
	card := map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": content,
				},
			},
		},
	}

	data, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
