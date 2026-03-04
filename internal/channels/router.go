package channels

import (
	"sync"
)

// AgentRouter routes incoming messages to the correct agent.
type AgentRouter struct {
	mu              sync.RWMutex
	userDefaults    map[string]string
	channelDefaults map[ChannelType]string
	defaultAgent    string
}

// NewAgentRouter creates a new agent router.
func NewAgentRouter() *AgentRouter {
	return &AgentRouter{
		userDefaults:    make(map[string]string),
		channelDefaults: make(map[ChannelType]string),
	}
}

// SetDefault sets the system-wide default agent.
func (r *AgentRouter) SetDefault(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultAgent = agentID
}

// SetChannelDefault sets a default agent for a channel type.
func (r *AgentRouter) SetChannelDefault(channelType ChannelType, agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelDefaults[channelType] = agentID
}

// SetUserDefault sets a default agent for a user.
func (r *AgentRouter) SetUserDefault(userKey string, agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userDefaults[userKey] = agentID
}

// Route finds the appropriate agent for a message.
// Returns (agentID, found) where found is false if no agent is configured.
func (r *AgentRouter) Route(channelType ChannelType, userID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if agentID, ok := r.userDefaults[userID]; ok && agentID != "" {
		return agentID, true
	}

	if agentID, ok := r.channelDefaults[channelType]; ok && agentID != "" {
		return agentID, true
	}

	if r.defaultAgent != "" {
		return r.defaultAgent, true
	}

	return "", false
}
