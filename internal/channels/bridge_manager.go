package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/penzhan8451/fangclaw-go/internal/autoreply"
)

// BridgeManager manages all running channel adapters and dispatches messages.
type BridgeManager struct {
	handle     ChannelBridgeHandle
	router     *AgentRouter
	adapters   map[string]Adapter
	adaptersMu sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewBridgeManager creates a new bridge manager.
func NewBridgeManager(handle ChannelBridgeHandle, router *AgentRouter) *BridgeManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BridgeManager{
		handle:   handle,
		router:   router,
		adapters: make(map[string]Adapter),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterAdapter registers a channel adapter with the bridge manager.
func (m *BridgeManager) RegisterAdapter(name string, adapter Adapter) error {
	m.adaptersMu.Lock()
	defer m.adaptersMu.Unlock()

	if _, exists := m.adapters[name]; exists {
		return fmt.Errorf("adapter %s already registered", name)
	}

	m.adapters[name] = adapter
	return nil
}

// Start starts all registered adapters and begins message dispatching.
func (m *BridgeManager) Start() error {
	m.adaptersMu.RLock()
	defer m.adaptersMu.RUnlock()

	for name, adapter := range m.adapters {
		fmt.Printf("Starting channel adapter: %s\n", name)

		if err := adapter.Connect(); err != nil {
			return fmt.Errorf("failed to connect %s: %w", name, err)
		}

		if err := adapter.Start(); err != nil {
			return fmt.Errorf("failed to start %s: %w", name, err)
		}

		msgChan, err := adapter.Receive(m.ctx)
		if err != nil {
			return fmt.Errorf("failed to get receive channel for %s: %w", name, err)
		}

		m.wg.Add(1)
		go m.dispatchMessages(name, adapter, msgChan)
	}

	return nil
}

// dispatchMessages dispatches messages from a channel adapter.
func (m *BridgeManager) dispatchMessages(name string, adapter Adapter, msgChan <-chan *Message) {
	defer m.wg.Done()

	fmt.Printf("Message dispatcher started for: %s\n", name)

	for {
		select {
		case <-m.ctx.Done():
			fmt.Printf("Message dispatcher stopped for: %s\n", name)
			return
		case msg, ok := <-msgChan:
			if !ok {
				fmt.Printf("Message channel closed for: %s\n", name)
				return
			}

			fmt.Printf("Received message from %s: %s\n", name, msg.Content)
			m.handleMessage(adapter, msg)
		}
	}
}

// handleMessage processes a single message.
func (m *BridgeManager) handleMessage(adapter Adapter, msg *Message) {
	channelType := adapter.GetChannel().Type

	// Check if it's a slash command
	if cmd, args, isCmd := isCommand(msg.Content); isCmd {
		fmt.Printf("Handling command: /%s\n", cmd)
		response := m.handleCommand(adapter, msg, cmd, args)
		m.sendReply(adapter, msg, response)
		return
	}

	// Route to agent
	agentID, found := m.router.Route(channelType, msg.Sender)
	if !found {
		// No agent configured - send help message
		helpMsg := "No agent assigned. Use /agents to list available agents, then /agent <name> to select one."
		m.sendReply(adapter, msg, helpMsg)
		return
	}

	fmt.Printf("Routing message to agent: %s\n", agentID)

	agents, err := m.handle.ListAgents(m.ctx)
	if err == nil {
		fmt.Printf("Available agents in registry: %v\n", agents)
	}

	// Check AutoReply
	autoReplyEngine := m.handle.GetAutoReplyEngine()
	if autoReplyEngine != nil {
		replyAgentID := autoReplyEngine.ShouldReply(msg.Content, string(channelType), agentID)
		if replyAgentID == "" {
			// 消息匹配抑制模式，完全阻止，不发送到 LLM
			fmt.Printf("Message suppressed by pattern: %s\n", msg.Content)
			return
		}

		// 消息不匹配抑制模式，使用 AutoReply
		fmt.Printf("AutoReply triggered for message: %s\n", msg.Content)
		fmt.Printf("AutoReply msg.sender: %s\n", msg.Sender)
		replyChannel := autoreply.AutoReplyChannel{
			ChannelType: string(channelType),
			PeerID:      msg.Sender,
			ThreadID:    nil,
		}

		sendFn := func(response string, ch autoreply.AutoReplyChannel) error {
			replyMsg := &Message{
				Content:   response,
				Recipient: msg.Sender,
				Sender:    "bot",
			}
			fmt.Printf("AutoReply Recipient in sendFn(): %s\n", msg.Sender)
			return adapter.Send(replyMsg)
		}

		err := autoReplyEngine.ExecuteReply(
			m.ctx,
			replyAgentID,
			msg.Content,
			replyChannel,
			sendFn,
			m.handle.SendMessage,
		)
		if err == nil {
			return // AutoReply 已处理，直接返回
		}
	}

	// 如果 AutoReply 没处理，走原有流程
	response, err := m.handle.SendMessage(m.ctx, agentID, msg.Content)
	if err != nil {
		fmt.Printf("Failed to send message to agent: %v\n", err)
		errorMsg := "No agent found. Use /agents to list available agents, then /agent <name> or /agent default to select one."
		m.sendReply(adapter, msg, errorMsg)
		return
	}

	if response != "" {
		m.sendReply(adapter, msg, response)
	}
}

// sendReply sends a reply message to the user.
func (m *BridgeManager) sendReply(adapter Adapter, originalMsg *Message, content string) {
	replyMsg := &Message{
		Content:   content,
		Recipient: originalMsg.Recipient,
		Sender:    "bot",
	}

	if err := adapter.Send(replyMsg); err != nil {
		fmt.Printf("Failed to send reply: %v\n", err)
	}
}

// Stop stops all adapters and the bridge manager.
func (m *BridgeManager) Stop() {
	fmt.Println("Stopping bridge manager...")
	m.cancel()

	m.adaptersMu.RLock()
	defer m.adaptersMu.RUnlock()

	for name, adapter := range m.adapters {
		fmt.Printf("Stopping channel adapter: %s\n", name)
		if err := adapter.Stop(); err != nil {
			fmt.Printf("Error stopping %s: %v\n", name, err)
		}
		if err := adapter.Disconnect(); err != nil {
			fmt.Printf("Error disconnecting %s: %v\n", name, err)
		}
	}

	m.wg.Wait()
	fmt.Println("Bridge manager stopped")
}
