// Package autoreply provides auto-reply functionality for OpenFang.
package autoreply

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AutoReplyChannel represents where to deliver the auto-reply result.
type AutoReplyChannel struct {
	ChannelType string
	PeerID      string
	ThreadID    *string
}

// AutoReplyConfig represents the configuration for auto-reply.
type AutoReplyConfig struct {
	Enabled          bool
	MaxConcurrent    int
	SuppressPatterns []string
}

// AutoReplyEngine is an auto-reply engine with concurrency limits and suppression patterns.
type AutoReplyEngine struct {
	config    AutoReplyConfig
	semaphore chan struct{}
	mu        sync.Mutex
}

// NewAutoReplyEngine creates a new auto-reply engine from configuration.
func NewAutoReplyEngine(config AutoReplyConfig) *AutoReplyEngine {
	permits := config.MaxConcurrent
	if permits < 1 {
		permits = 1
	}

	return &AutoReplyEngine{
		config:    config,
		semaphore: make(chan struct{}, permits),
	}
}

// ShouldReply checks if a message should trigger auto-reply.
// Returns the agent ID if should auto-reply, empty string if suppressed or disabled.
func (are *AutoReplyEngine) ShouldReply(message, channelType, agentID string) string {
	are.mu.Lock()
	defer are.mu.Unlock()

	if !are.config.Enabled {
		return ""
	}

	lowerMessage := strings.ToLower(message)
	for _, pattern := range are.config.SuppressPatterns {
		if strings.Contains(lowerMessage, strings.ToLower(pattern)) {
			return ""
		}
	}

	return agentID
}

// Acquire acquires a slot for concurrent execution.
func (are *AutoReplyEngine) Acquire() {
	are.semaphore <- struct{}{}
}

// Release releases a slot for concurrent execution.
func (are *AutoReplyEngine) Release() {
	<-are.semaphore
}

// GetConfig returns the current configuration.
func (are *AutoReplyEngine) GetConfig() AutoReplyConfig {
	are.mu.Lock()
	defer are.mu.Unlock()
	return are.config
}

// UpdateConfig updates the engine configuration.
func (are *AutoReplyEngine) UpdateConfig(config AutoReplyConfig) {
	are.mu.Lock()
	defer are.mu.Unlock()

	oldPermits := are.config.MaxConcurrent
	are.config = config

	newPermits := config.MaxConcurrent
	if newPermits < 1 {
		newPermits = 1
	}

	if newPermits != oldPermits {
		oldSem := are.semaphore
		are.semaphore = make(chan struct{}, newPermits)
		close(oldSem)
	}
}

// ExecuteReply executes an auto-reply in the background with concurrency control.
// Parameters:
//   - ctx: Context
//   - agentID: Agent ID to send message to
//   - message: User message
//   - channel: Where to send the reply
//   - sendFn: Function to send reply to user
//   - sendToAgentFn: Function to send message to agent (kernel.SendMessage)
func (are *AutoReplyEngine) ExecuteReply(
	ctx context.Context,
	agentID string,
	message string,
	channel AutoReplyChannel,
	sendFn func(response string, channel AutoReplyChannel) error,
	sendToAgentFn func(ctx context.Context, agentID string, message string) (string, error),
) error {
	log.Debug().Str("agentID", agentID).Msg("[AutoReply] ExecuteReply called with agentID")

	select {
	case are.semaphore <- struct{}{}:
	case <-ctx.Done():
		log.Debug().Err(ctx.Err()).Msg("[AutoReply] Context done, returning")
		return ctx.Err()
	default:
		log.Debug().Msg("[AutoReply] No semaphore available, skipping")
		return nil
	}

	go func() {
		defer func() {
			log.Debug().Msg("[AutoReply] Releasing semaphore")
			<-are.semaphore
		}()

		log.Debug().Msg("[AutoReply] Starting to send to agent...")
		replyCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		response, err := sendToAgentFn(replyCtx, agentID, message)
		if err != nil {
			log.Debug().Err(err).Msg("[AutoReply] Error sending message to agent")
			return
		}
		fmt.Printf("[AutoReply] Received response from agent, len=%d\n", len(response))

		fmt.Printf("[AutoReply] Calling sendFn...\n")
		sendErr := sendFn(response, channel)
		if sendErr != nil {
			fmt.Printf("[AutoReply] Error sending reply: %v\n", sendErr)
		} else {
			fmt.Printf("[AutoReply] Reply sent successfully\n")
		}
	}()

	return nil
}
