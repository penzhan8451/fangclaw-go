package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/penzhan8451/fangclaw-go/internal/autoreply"
	"github.com/rs/zerolog/log"
)

type KernelResolver interface {
	GetKernelByOwner(owner string) (ChannelBridgeHandle, bool)
}

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

func (m *BridgeManager) SetKernelResolver(resolver KernelResolver) {
	m.kernelResolver = resolver
}

func (m *BridgeManager) RegisterAdapter(name string, adapter Adapter) error {
	m.adaptersMu.Lock()
	defer m.adaptersMu.Unlock()

	if _, exists := m.adapters[name]; exists {
		return fmt.Errorf("adapter %s already registered", name)
	}

	m.adapters[name] = adapter

	select {
	case <-m.ctx.Done():
	default:
		log.Info().Str("adapter", name).Msg("Registering and starting new channel adapter")

		if adapter.GetState() != ChannelStateConnected {
			if err := adapter.Connect(); err != nil {
				log.Warn().Str("adapter", name).Err(err).Msg("Failed to connect adapter")
			}
		}

		if adapter.GetState() != ChannelStateConnected {
			if err := adapter.Start(); err != nil {
				log.Warn().Str("adapter", name).Err(err).Msg("Failed to start adapter")
			}
		}

		msgChan, err := adapter.Receive(m.ctx)
		if err == nil {
			m.wg.Add(1)
			go m.dispatchMessages(name, adapter, msgChan)
		} else {
			log.Warn().Str("adapter", name).Err(err).Msg("Failed to get receive channel")
		}
	}

	return nil
}

func (m *BridgeManager) Start() error {
	m.adaptersMu.RLock()
	defer m.adaptersMu.RUnlock()

	for name, adapter := range m.adapters {
		log.Info().Str("adapter", name).Msg("Starting channel adapter")

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

func (m *BridgeManager) dispatchMessages(name string, adapter Adapter, msgChan <-chan *Message) {
	defer m.wg.Done()

	log.Info().Str("adapter", name).Msg("Message dispatcher started")

	for {
		select {
		case <-m.ctx.Done():
			log.Info().Str("adapter", name).Msg("Message dispatcher stopped")
			return
		case msg, ok := <-msgChan:
			if !ok {
				log.Info().Str("adapter", name).Msg("Message channel closed")
				return
			}

			channel := adapter.GetChannel()
			owner := "unknown"
			if channel != nil {
				owner = channel.Owner
			}
			log.Debug().Str("adapter", name).Str("owner", owner).Str("content", msg.Content).Msg("Received message")
			m.handleMessage(adapter, msg)
		}
	}
}

func (m *BridgeManager) handleMessage(adapter Adapter, msg *Message) {
	channelType := adapter.GetChannel().Type
	channel := adapter.GetChannel()
	owner := channel.Owner

	var handle ChannelBridgeHandle = m.handle
	log.Debug().Str("handle_type", fmt.Sprintf("%T", m.handle)).Bool("kernel_resolver_nil", m.kernelResolver == nil).Msg("Handling message")
	if m.kernelResolver != nil && owner != "" {
		if kernelHandle, ok := m.kernelResolver.GetKernelByOwner(owner); ok {
			handle = kernelHandle
			log.Debug().Str("owner", owner).Str("handle_type", fmt.Sprintf("%T", handle)).Msg("Routing message to kernel for owner")
		} else {
			log.Debug().Str("owner", owner).Msg("No kernel found for owner, using default handle")
		}
	}

	if cmd, args, isCmd := isCommand(msg.Content); isCmd {
		log.Info().Str("command", cmd).Msg("Handling command")
		response := m.handleCommand(adapter, msg, cmd, args, handle)
		m.sendReplyWithHandle(adapter, msg, response, "", handle)
		return
	}

	agents, err := handle.ListAgents(m.ctx)
	if err == nil {
		log.Debug().Int("count", len(agents)).Msg("Available agents in user kernel")
		for i, a := range agents {
			log.Debug().Int("index", i).Str("id", a.ID).Str("name", a.Name).Msg("Agent")
		}
	} else {
		log.Warn().Err(err).Msg("Error listing agents in user kernel")
	}

	var agentID string
	if len(agents) > 0 {
		if userAgentID, ok := m.router.Route(channelType, msg.Sender); ok {
			found := false
			for _, a := range agents {
				if a.ID == userAgentID {
					found = true
					agentID = userAgentID
					log.Info().Str("agent_id", agentID).Msg("Using user-selected agent")
					break
				}
			}
			if !found {
				log.Warn().Str("agent_id", userAgentID).Msg("User-selected agent not available, trying channel config")
			}
		}

		if agentID == "" {
			var channelDefaultAgent string
			switch channelType {
			case ChannelTypeWeixin:
				if channel.Config.Weixin != nil {
					channelDefaultAgent = channel.Config.Weixin.DefaultAgent
				}
			}

			if channelDefaultAgent != "" {
				channelAgentID, found := handle.FindAgentByName(m.ctx, channelDefaultAgent)
				if found {
					available := false
					for _, a := range agents {
						if a.ID == channelAgentID {
							available = true
							agentID = channelAgentID
							log.Info().Str("agent_id", agentID).Str("agent_name", channelDefaultAgent).Msg("Using channel-configured agent")
							break
						}
					}
					if !available {
						log.Warn().Str("agent_name", channelDefaultAgent).Str("agent_id", channelAgentID).Msg("Channel-configured agent not in available list, using first agent")
					}
				} else {
					log.Warn().Str("agent_name", channelDefaultAgent).Msg("Channel-configured agent not found by name, using first agent")
				}
			}
		}

		if agentID == "" {
			agentID = agents[0].ID
			log.Info().Str("agent_id", agentID).Str("agent_name", agents[0].Name).Msg("Using first available agent from user kernel")
		}
	} else {
		log.Warn().Msg("No agents available in user kernel")
		helpMsg := "No agent assigned. Use /agents to list available agents, then /agent <name> to select one."
		m.sendReplyWithHandle(adapter, msg, helpMsg, "", handle)
		return
	}

	autoReplyEngine := handle.GetAutoReplyEngine()
	if autoReplyEngine != nil {
		replyAgentID := autoReplyEngine.ShouldReply(msg.Content, string(channelType), agentID)
		if replyAgentID == "" {
			log.Debug().Str("content", msg.Content).Msg("Message suppressed by pattern")
			return
		}

		log.Info().Str("content", msg.Content).Str("agent_id", agentID).Msg("AutoReply triggered for message")
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
			log.Debug().Str("recipient", msg.Sender).Msg("AutoReply sending reply")
			sendErr := adapter.Send(replyMsg)
			if sendErr == nil {
				log.Info().Str("recipient", msg.Sender).Msg("AutoReply successfully sent reply")
				handle.RecordDelivery(m.ctx, replyAgentID, string(channelType), msg.Sender, true, "")
			} else {
				log.Warn().Err(sendErr).Str("recipient", msg.Sender).Msg("AutoReply failed to send reply")
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
			log.Debug().Msg("AutoReply ExecuteReply returned nil, will send reply asynchronously")
			return
		}
		log.Warn().Err(err).Msg("AutoReply ExecuteReply failed, falling back to sync path")
	}

	response, err := handle.SendMessage(m.ctx, agentID, msg.Content)
	log.Debug().Str("agent_id", agentID).Err(err).Int("response_len", len(response)).Msg("SendMessage result")
	if err != nil {
		log.Warn().Err(err).Str("agent_id", agentID).Msg("Failed to send message to agent")
		handle.RecordDelivery(m.ctx, agentID, string(channelType), msg.Sender, false, err.Error())
		errorMsg := "No agent found. Use /agents to list available agents, then /agent <name> or /agent default to select one."
		m.sendReplyWithHandle(adapter, msg, errorMsg, "", handle)
		return
	}

	if response != "" {
		m.sendReplyWithHandle(adapter, msg, response, agentID, handle)
	}
}

func (m *BridgeManager) sendReply(adapter Adapter, originalMsg *Message, content string, agentID string) {
	m.sendReplyWithHandle(adapter, originalMsg, content, agentID, m.handle)
}

func (m *BridgeManager) sendReplyWithHandle(adapter Adapter, originalMsg *Message, content string, agentID string, handle ChannelBridgeHandle) {
	log.Debug().Str("recipient", originalMsg.Sender).Int("content_len", len(content)).Msg("Sending reply")
	channelType := adapter.GetChannel().Type
	replyMsg := &Message{
		Content:   content,
		Recipient: originalMsg.Sender,
		Sender:    "bot",
	}

	if err := adapter.Send(replyMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to send reply")
		if agentID != "" {
			handle.RecordDelivery(m.ctx, agentID, string(channelType), originalMsg.Sender, false, err.Error())
		}
		return
	}
	log.Debug().Msg("Reply sent successfully")

	if agentID != "" {
		handle.RecordDelivery(m.ctx, agentID, string(channelType), originalMsg.Sender, true, "")
	}
}

func (m *BridgeManager) Stop() {
	log.Info().Msg("Stopping bridge manager")
	m.cancel()

	m.adaptersMu.RLock()
	defer m.adaptersMu.RUnlock()

	for name, adapter := range m.adapters {
		log.Info().Str("adapter", name).Msg("Stopping channel adapter")
		if err := adapter.Stop(); err != nil {
			log.Warn().Str("adapter", name).Err(err).Msg("Error stopping adapter")
		}
		if err := adapter.Disconnect(); err != nil {
			log.Warn().Str("adapter", name).Err(err).Msg("Error disconnecting adapter")
		}
	}

	m.wg.Wait()
	log.Info().Msg("Bridge manager stopped")
}
