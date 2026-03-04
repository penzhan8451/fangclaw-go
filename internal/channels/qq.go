package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
	"golang.org/x/oauth2"
)

// QQAdapter implements the Adapter interface for QQ.
type QQAdapter struct {
	*BaseAdapter
	api            openapi.OpenAPI
	tokenSource    oauth2.TokenSource
	ctx            context.Context
	cancel         context.CancelFunc
	sessionManager botgo.SessionManager
	processedIDs   map[string]bool
	msgChan        chan *Message
	mu             sync.RWMutex
}

// NewQQAdapter creates a new QQ adapter.
func NewQQAdapter(channel *Channel) (Adapter, error) {
	return &QQAdapter{
		BaseAdapter:  NewBaseAdapter(channel),
		processedIDs: make(map[string]bool),
		msgChan:      make(chan *Message, 100),
	}, nil
}

// Connect connects to QQ using WebSocket.
func (a *QQAdapter) Connect() error {
	return a.BaseAdapter.Connect()
}

// Disconnect disconnects from QQ.
func (a *QQAdapter) Disconnect() error {
	if err := a.Stop(); err != nil {
		return err
	}
	return a.BaseAdapter.Disconnect()
}

// Receive receives messages from QQ.
// QQ Bot receive message from the user or group.
func (a *QQAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

// Send sends a message via QQ Bot API.
// QQ Bot send back message to the user or group.
func (a *QQAdapter) Send(msg *Message) error {
	if a.Channel.Config.QQAppID == "" || a.Channel.Config.QQAppSecret == "" {
		return fmt.Errorf("qq app id or app secret not configured")
	}

	if a.api == nil {
		return fmt.Errorf("qq api not initialized")
	}

	if msg.Recipient == "" {
		return fmt.Errorf("recipient (group id or user id) required for QQ")
	}

	msgToCreate := &dto.MessageToCreate{
		Content: msg.Content,
	}

	_, err := a.api.PostC2CMessage(a.ctx, msg.Recipient, msgToCreate)
	if err != nil {
		return fmt.Errorf("qq send: %w", err)
	}

	return nil
}

// Start starts the QQ adapter with WebSocket.
// QQ Bot connect to QQ server with WebSocket.
func (a *QQAdapter) Start() error {
	if a.Channel.Config.QQAppID == "" || a.Channel.Config.QQAppSecret == "" {
		return fmt.Errorf("qq app id or app secret not configured")
	}

	credentials := &token.QQBotCredentials{
		AppID:     a.Channel.Config.QQAppID,
		AppSecret: a.Channel.Config.QQAppSecret,
	}
	a.tokenSource = token.NewQQBotTokenSource(credentials)

	a.ctx, a.cancel = context.WithCancel(context.Background())

	if err := token.StartRefreshAccessToken(a.ctx, a.tokenSource); err != nil {
		return fmt.Errorf("failed to start token refresh: %w", err)
	}

	a.api = botgo.NewOpenAPI(a.Channel.Config.QQAppID, a.tokenSource).WithTimeout(5 * time.Second)

	intent := event.RegisterHandlers(
		a.handleC2CMessage(),
		a.handleGroupATMessage(),
	)

	wsInfo, err := a.api.WS(a.ctx, nil, "")
	if err != nil {
		return fmt.Errorf("failed to get websocket info: %w", err)
	}

	a.sessionManager = botgo.NewSessionManager()

	go func() {
		if err := a.sessionManager.Start(wsInfo, a.tokenSource, &intent); err != nil {
			fmt.Printf("QQ WebSocket session error: %v\n", err)
			a.Channel.State = ChannelStateError
		}
	}()

	a.Channel.State = ChannelStateConnected
	return nil
}

// Stop stops the QQ adapter.
func (a *QQAdapter) Stop() error {
	a.Channel.State = ChannelStateDisconnected
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

// handleC2CMessage handles QQ private messages.
// QQ Bot receive private message from the user.
func (a *QQAdapter) handleC2CMessage() event.C2CMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CMessageData) error {
		if a.isDuplicate(data.ID) {
			return nil
		}

		var senderID string
		if data.Author != nil && data.Author.ID != "" {
			senderID = data.Author.ID
		} else {
			return nil
		}

		content := data.Content
		if content == "" {
			return nil
		}

		if !a.isAllowedSender(senderID) {
			return nil
		}

		a.msgChan <- &Message{
			ID:        data.ID,
			ChannelID: a.Channel.ID,
			Content:   content,
			Sender:    senderID,
			Recipient: senderID,
			Metadata: map[string]interface{}{
				"message_type": "c2c",
			},
			CreatedAt: time.Now(),
		}

		return nil
	}
}

// handleGroupATMessage handles QQ group @ messages.
// QQ Bot receive group @ message from the group user.
func (a *QQAdapter) handleGroupATMessage() event.GroupATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		if a.isDuplicate(data.ID) {
			return nil
		}

		var senderID string
		if data.Author != nil && data.Author.ID != "" {
			senderID = data.Author.ID
		} else {
			return nil
		}

		content := data.Content
		if content == "" {
			return nil
		}

		if !a.isAllowedSender(senderID) {
			return nil
		}

		a.msgChan <- &Message{
			ID:        data.ID,
			ChannelID: a.Channel.ID,
			Content:   content,
			Sender:    senderID,
			Recipient: data.GroupID,
			Metadata: map[string]interface{}{
				"message_type": "group_at",
				"group_id":     data.GroupID,
			},
			CreatedAt: time.Now(),
		}

		return nil
	}
}

// isDuplicate checks if message is duplicate.
func (a *QQAdapter) isDuplicate(messageID string) bool {
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
func (a *QQAdapter) isAllowedSender(senderID string) bool {
	if len(a.Channel.Config.QQAllowFrom) == 0 {
		return true
	}

	for _, allowed := range a.Channel.Config.QQAllowFrom {
		if allowed == senderID {
			return true
		}
	}

	return false
}
