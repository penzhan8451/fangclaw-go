// Package capabilities provides capability-based security for OpenFang.
package capabilities

import (
	"fmt"
	"strings"
	"sync"
)

// CapabilityType represents the type of a capability.
type CapabilityType string

const (
	CapabilityTypeToolInvoke    CapabilityType = "tool_invoke"
	CapabilityTypeChannelSend   CapabilityType = "channel_send"
	CapabilityTypeChannelRead   CapabilityType = "channel_read"
	CapabilityTypeFileRead      CapabilityType = "file_read"
	CapabilityTypeFileWrite     CapabilityType = "file_write"
	CapabilityTypeMemoryAccess  CapabilityType = "memory_access"
	CapabilityTypeNetworkAccess CapabilityType = "network_access"
	CapabilityTypeProcessSpawn  CapabilityType = "process_spawn"
	CapabilityTypeAgentControl  CapabilityType = "agent_control"
)

// Capability represents a granted capability.
type Capability struct {
	Type       CapabilityType `json:"type"`
	Resource   string         `json:"resource,omitempty"`
	Permission string         `json:"permission,omitempty"`
}

// CapabilityCheckResult represents the result of a capability check.
type CapabilityCheckResult string

const (
	CapabilityCheckGranted CapabilityCheckResult = "granted"
	CapabilityCheckDenied  CapabilityCheckResult = "denied"
)

// CapabilityManager manages capability grants for all agents.
type CapabilityManager struct {
	mu     sync.RWMutex
	grants map[string][]Capability
}

// NewCapabilityManager creates a new capability manager.
func NewCapabilityManager() *CapabilityManager {
	return &CapabilityManager{
		grants: make(map[string][]Capability),
	}
}

// Grant grants capabilities to an agent.
func (cm *CapabilityManager) Grant(agentID string, capabilities []Capability) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.grants[agentID] = capabilities
}

// Check checks whether an agent has a specific capability.
func (cm *CapabilityManager) Check(agentID string, required Capability) (CapabilityCheckResult, string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	grants, ok := cm.grants[agentID]
	if !ok {
		return CapabilityCheckDenied, fmt.Sprintf("No capabilities registered for agent %s", agentID)
	}

	for _, granted := range grants {
		if capabilityMatches(granted, required) {
			return CapabilityCheckGranted, ""
		}
	}

	return CapabilityCheckDenied, fmt.Sprintf("Agent %s does not have capability: %v", agentID, required)
}

// List lists all capabilities for an agent.
func (cm *CapabilityManager) List(agentID string) []Capability {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if grants, ok := cm.grants[agentID]; ok {
		return grants
	}
	return []Capability{}
}

// RevokeAll revokes all capabilities for an agent.
func (cm *CapabilityManager) RevokeAll(agentID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.grants, agentID)
}

// capabilityMatches checks if a granted capability matches a required capability.
func capabilityMatches(granted, required Capability) bool {
	if granted.Type != required.Type {
		return false
	}

	if granted.Resource == "*" || granted.Resource == "" {
		return true
	}

	if required.Resource == "" {
		return true
	}

	return strings.HasPrefix(required.Resource, granted.Resource) ||
		strings.Contains(granted.Resource, "*") && wildcardMatch(granted.Resource, required.Resource)
}

// wildcardMatch performs a simple wildcard match.
func wildcardMatch(pattern, str string) bool {
	patternParts := strings.Split(pattern, "*")
	if len(patternParts) == 1 {
		return pattern == str
	}

	if !strings.HasPrefix(str, patternParts[0]) {
		return false
	}

	str = str[len(patternParts[0]):]

	for i := 1; i < len(patternParts)-1; i++ {
		idx := strings.Index(str, patternParts[i])
		if idx == -1 {
			return false
		}
		str = str[idx+len(patternParts[i]):]
	}

	return strings.HasSuffix(str, patternParts[len(patternParts)-1]) || patternParts[len(patternParts)-1] == ""
}
