// Package capabilities provides capability-based security for FangClaw-Go.
package capabilities

import (
	"fmt"
	"strings"
	"sync"
)

// Capability represents a specific permission granted to an agent.
type Capability struct {
	Type     CapabilityType `json:"type"`
	Resource string         `json:"resource,omitempty"`
}

// CapabilityType represents the type of a capability.
type CapabilityType string

const (
	// File system capabilities
	CapFileRead  CapabilityType = "file_read"
	CapFileWrite CapabilityType = "file_write"

	// Network capabilities
	CapNetConnect CapabilityType = "net_connect"
	CapNetListen  CapabilityType = "net_listen"

	// Tool capabilities
	CapToolInvoke CapabilityType = "tool_invoke"
	CapToolAll    CapabilityType = "tool_all"

	// LLM capabilities
	CapLlmQuery     CapabilityType = "llm_query"
	CapLlmMaxTokens CapabilityType = "llm_max_tokens"

	// Agent interaction capabilities
	CapAgentSpawn   CapabilityType = "agent_spawn"
	CapAgentMessage CapabilityType = "agent_message"
	CapAgentKill    CapabilityType = "agent_kill"

	// Memory capabilities
	CapMemoryRead  CapabilityType = "memory_read"
	CapMemoryWrite CapabilityType = "memory_write"

	// Shell capabilities
	CapShellExec CapabilityType = "shell_exec"
	CapEnvRead   CapabilityType = "env_read"

	// Channel capabilities
	CapChannelSend CapabilityType = "channel_send"
	CapChannelRead CapabilityType = "channel_read"

	// Schedule capabilities
	CapSchedule CapabilityType = "schedule"
)

// CapabilityCheckResult represents the result of a capability check.
type CapabilityCheckResult string

const (
	CapabilityCheckGranted CapabilityCheckResult = "granted"
	CapabilityCheckDenied  CapabilityCheckResult = "denied"
)

// Check returns true if the capability is granted.
func (r CapabilityCheckResult) Granted() bool {
	return r == CapabilityCheckGranted
}

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
		if CapabilityMatches(granted, required) {
			return CapabilityCheckGranted, ""
		}
	}

	return CapabilityCheckDenied, fmt.Sprintf("Agent %s does not have capability: %s", agentID, required.Type)
}

// CheckWithDefault checks capability with backward compatibility.
// If agent has no capabilities registered, returns granted (backward compatible).
func (cm *CapabilityManager) CheckWithDefault(agentID string, required Capability) CapabilityCheckResult {
	cm.mu.RLock()
	grants, hasGrants := cm.grants[agentID]
	cm.mu.RUnlock()

	if !hasGrants || len(grants) == 0 {
		return CapabilityCheckGranted
	}

	result, _ := cm.Check(agentID, required)
	return result
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

// HasCapabilities returns true if agent has any capabilities registered.
func (cm *CapabilityManager) HasCapabilities(agentID string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	grants, ok := cm.grants[agentID]
	return ok && len(grants) > 0
}

// RevokeAll revokes all capabilities for an agent.
func (cm *CapabilityManager) RevokeAll(agentID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.grants, agentID)
}

// CapabilityMatches checks if a granted capability matches a required capability.
func CapabilityMatches(granted, required Capability) bool {
	if granted.Type != required.Type {
		if granted.Type != CapToolAll {
			return false
		}
		if required.Type == CapToolInvoke {
			return true
		}
		return false
	}

	switch granted.Type {
	case CapFileRead, CapFileWrite, CapNetConnect, CapToolInvoke,
		CapLlmQuery, CapAgentMessage, CapAgentKill,
		CapMemoryRead, CapMemoryWrite, CapShellExec, CapEnvRead,
		CapChannelSend, CapChannelRead:
		return globMatches(granted.Resource, required.Resource)

	case CapNetListen:
		return granted.Resource == required.Resource

	case CapLlmMaxTokens:
		return parseUint(granted.Resource) >= parseUint(required.Resource)

	case CapAgentSpawn, CapSchedule, CapToolAll:
		return true

	default:
		return globMatches(granted.Resource, required.Resource)
	}
}

// ValidateCapabilityInheritance validates that child capabilities are a subset of parent capabilities.
// This prevents privilege escalation: a restricted parent cannot create an unrestricted child.
func ValidateCapabilityInheritance(parentCaps, childCaps []Capability) error {
	for _, childCap := range childCaps {
		isCovered := false
		for _, parentCap := range parentCaps {
			if CapabilityMatches(parentCap, childCap) {
				isCovered = true
				break
			}
		}
		if !isCovered {
			return fmt.Errorf("privilege escalation denied: child requests %s but parent does not have a matching grant", childCap.Type)
		}
	}
	return nil
}

// globMatches performs glob pattern matching.
// Supports: "*" (match all), "prefix*" (prefix match), "*suffix" (suffix match), "prefix*suffix" (middle wildcard).
func globMatches(pattern, value string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if value == "" {
		return pattern == ""
	}
	if pattern == value {
		return true
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(value, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(value, prefix)
	}

	if strings.Contains(pattern, "*") {
		starIdx := strings.Index(pattern, "*")
		prefix := pattern[:starIdx]
		suffix := pattern[starIdx+1:]

		return strings.HasPrefix(value, prefix) &&
			strings.HasSuffix(value, suffix) &&
			len(value) >= len(prefix)+len(suffix)
	}

	return pattern == value
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		} else {
			break
		}
	}
	return n
}

// DefaultCapabilities returns a set of default capabilities for agents without explicit capabilities.
func DefaultCapabilities() []Capability {
	return []Capability{
		{Type: CapToolAll},
		{Type: CapFileRead, Resource: "*"},
		{Type: CapFileWrite, Resource: "*"},
		{Type: CapNetConnect, Resource: "*"},
		{Type: CapMemoryRead, Resource: "*"},
		{Type: CapMemoryWrite, Resource: "*"},
		{Type: CapShellExec, Resource: "*"},
		{Type: CapAgentSpawn},
		{Type: CapAgentMessage, Resource: "*"},
		{Type: CapChannelSend, Resource: "*"},
		{Type: CapChannelRead, Resource: "*"},
		{Type: CapSchedule},
	}
}

// MergeCapabilities merges two capability sets, removing duplicates.
func MergeCapabilities(base, extra []Capability) []Capability {
	seen := make(map[string]bool)
	result := make([]Capability, 0, len(base)+len(extra))

	for _, cap := range base {
		key := string(cap.Type) + ":" + cap.Resource
		if !seen[key] {
			seen[key] = true
			result = append(result, cap)
		}
	}

	for _, cap := range extra {
		key := string(cap.Type) + ":" + cap.Resource
		if !seen[key] {
			seen[key] = true
			result = append(result, cap)
		}
	}

	return result
}
