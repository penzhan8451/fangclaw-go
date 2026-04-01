package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/penzhan8451/fangclaw-go/internal/autoreply"
)

// KernelResolver resolves a kernel by owner username.
type KernelResolver interface {
	GetKernelByOwner(owner string) (ChannelBridgeHandle, bool)
}

// BridgeManager manages all running channel adapters and dispatches messages.
type BridgeManager struct {
	handle         ChannelBridgeHandle
	router         *AgentRouter
	adapters       map[string]Adapter
	adaptersMu     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	kernelResolver KernelResolver
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

// SetKernelResolver sets the kernel resolver for multi-tenant message routing.
func (m *BridgeManager) SetKernelResolver(resolver KernelResolver) {
	m.kernelResolver = resolver
}

// RegisterAdapter registers a channel adapter with the bridge manager.
func (m *BridgeManager) RegisterAdapter(name string, adapter Adapter) error {
	m.adaptersMu.Lock()
	defer m.adaptersMu.Unlock()

	if _, exists := m.adapters[name]; exists {
		return fmt.Errorf("adapter %s already registered", name)
	}

	m.adapters[name] = adapter

	// If bridge manager is already running, start dispatching for this new adapter
	select {
	case <-m.ctx.Done():
		// Bridge manager is stopping, don't start
	default:
		// Check if we're already started (by seeing if cancel func is still valid)
		// Start dispatching for this new adapter
		fmt.Printf("Registering and starting new channel adapter: %s\n", name)

		// Check if adapter is already connected and started - if not, do it
		if adapter.GetState() != ChannelStateConnected {
			if err := adapter.Connect(); err != nil {
				fmt.Printf("Warning: Failed to connect %s: %v\n", name, err)
			}
		}

		if adapter.GetState() != ChannelStateConnected {
			if err := adapter.Start(); err != nil {
				fmt.Printf("Warning: Failed to start %s: %v\n", name, err)
			}
		}

		msgChan, err := adapter.Receive(m.ctx)
		if err == nil {
			m.wg.Add(1)
			go m.dispatchMessages(name, adapter, msgChan)
		} else {
			fmt.Printf("Warning: Failed to get receive channel for %s: %v\n", name, err)
		}
	}

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

			channel := adapter.GetChannel()
			owner := "unknown"
			if channel != nil {
				owner = channel.Owner
			}
			fmt.Printf("[BridgeManager] Received message from %s (owner: %s): %s\n", name, owner, msg.Content)
			m.handleMessage(adapter, msg)
		}
	}
}

// handleMessage processes a single message.
func (m *BridgeManager) handleMessage(adapter Adapter, msg *Message) {
	channelType := adapter.GetChannel().Type
	channel := adapter.GetChannel()
	owner := channel.Owner

	var handle ChannelBridgeHandle = m.handle
	fmt.Printf("[BridgeManager] Default handle type: %T, kernelResolver nil: %v\n", m.handle, m.kernelResolver == nil)
	if m.kernelResolver != nil && owner != "" {
		if kernelHandle, ok := m.kernelResolver.GetKernelByOwner(owner); ok {
			handle = kernelHandle
			fmt.Printf("[BridgeManager] Routing message to kernel for owner: %s, handle type: %T\n", owner, handle)
		} else {
			fmt.Printf("[BridgeManager] No kernel found for owner: %s, using default handle\n", owner)
		}
	}

	if cmd, args, isCmd := isCommand(msg.Content); isCmd {
		fmt.Printf("Handling command: /%s\n", cmd)
		response := m.handleCommand(adapter, msg, cmd, args)
		m.sendReplyWithHandle(adapter, msg, response, "", handle)
		return
	}

	// 直接从用户的 kernel 获取 agents，不用全局的 router
	agents, err := handle.ListAgents(m.ctx)
	if err == nil {
		fmt.Printf("Available agents in user kernel: %v\n", agents)
		for i, a := range agents {
			fmt.Printf("  Agent[%d]: ID=%q, Name=%q\n", i, a.ID, a.Name)
		}
	} else {
		fmt.Printf("Error listing agents in user kernel: %v\n", err)
	}

	var agentID string
	if len(agents) > 0 {
		// 直接用用户 kernel 里的第一个 agent
		agentID = agents[0].ID
		fmt.Printf("Using first available agent from user kernel: %s (%s)\n", agentID, agents[0].Name)
	} else {
		fmt.Printf("No agents available in user kernel!\n")
		helpMsg := "No agent assigned. Use /agents to list available agents, then /agent <name> to select one."
		m.sendReplyWithHandle(adapter, msg, helpMsg, "", handle)
		return
	}

	autoReplyEngine := handle.GetAutoReplyEngine()
	if autoReplyEngine != nil {
		replyAgentID := autoReplyEngine.ShouldReply(msg.Content, string(channelType), agentID)
		if replyAgentID == "" {
			fmt.Printf("Message suppressed by pattern: %s\n", msg.Content)
			return
		}

		fmt.Printf("AutoReply triggered for message: %s (agentID=%s, availableAgents=%v)\n", msg.Content, agentID, agents)
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
			sendErr := adapter.Send(replyMsg)
			if sendErr == nil {
				fmt.Printf("AutoReply successfully sent reply to QQ user: %s\n", msg.Sender)
				handle.RecordDelivery(m.ctx, replyAgentID, string(channelType), msg.Sender, true, "")
			} else {
				fmt.Printf("AutoReply failed to send reply to QQ user: %v\n", sendErr)
				handle.RecordDelivery(m.ctx, replyAgentID, string(channelType), msg.Sender, false, sendErr.Error())
			}
			return sendErr
		}

		err := autoReplyEngine.ExecuteReply(
			m.ctx,
			replyAgentID,
			msg.Content,
			replyChannel,
			sendFn,
			handle.SendMessage,
		)
		if err == nil {
			fmt.Printf("AutoReply ExecuteReply returned nil, will send reply asynchronously\n")
			return
		}
		fmt.Printf("AutoReply ExecuteReply failed: %v, falling back to sync path\n", err)
	}

	response, err := handle.SendMessage(m.ctx, agentID, msg.Content)
	fmt.Printf("[BridgeManager] SendMessage called with agentID=%s, handle type=%T, error=%v, response_len=%d\n", agentID, handle, err, len(response))
	if err != nil {
		fmt.Printf("Failed to send message to agent: %v\n", err)
		handle.RecordDelivery(m.ctx, agentID, string(channelType), msg.Sender, false, err.Error())
		errorMsg := "No agent found. Use /agents to list available agents, then /agent <name> or /agent default to select one."
		m.sendReplyWithHandle(adapter, msg, errorMsg, "", handle)
		return
	}

	if response != "" {
		m.sendReplyWithHandle(adapter, msg, response, agentID, handle)
	}
}

// sendReply sends a reply message to the user and records a delivery receipt if agentID is provided.
func (m *BridgeManager) sendReply(adapter Adapter, originalMsg *Message, content string, agentID string) {
	m.sendReplyWithHandle(adapter, originalMsg, content, agentID, m.handle)
}

// sendReplyWithHandle sends a reply using the specified handle.
func (m *BridgeManager) sendReplyWithHandle(adapter Adapter, originalMsg *Message, content string, agentID string, handle ChannelBridgeHandle) {
	fmt.Printf("[BridgeManager] sendReplyWithHandle called: Recipient=%s, content_len=%d\n", originalMsg.Sender, len(content))
	channelType := adapter.GetChannel().Type
	replyMsg := &Message{
		Content:   content,
		Recipient: originalMsg.Sender,
		Sender:    "bot",
	}

	fmt.Printf("[BridgeManager] Calling adapter.Send (QQ adapter)...\n")
	if err := adapter.Send(replyMsg); err != nil {
		fmt.Printf("Failed to send reply: %v\n", err)
		if agentID != "" {
			handle.RecordDelivery(m.ctx, agentID, string(channelType), originalMsg.Sender, false, err.Error())
		}
		return
	}
	fmt.Printf("[BridgeManager] Adapter.Send (QQ) successful!\n")

	if agentID != "" {
		handle.RecordDelivery(m.ctx, agentID, string(channelType), originalMsg.Sender, true, "")
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
