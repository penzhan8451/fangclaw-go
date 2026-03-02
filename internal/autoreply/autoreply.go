// Package autoreply provides auto-reply functionality for OpenFang.
package autoreply

import (
	"strings"
	"sync"
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
