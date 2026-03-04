package channels

import "context"

// ChannelBridgeHandle defines the interface for kernel operations needed by channel adapters.
type ChannelBridgeHandle interface {
	// SendMessage sends a message to an agent and gets the response.
	SendMessage(ctx context.Context, agentID string, message string) (string, error)

	// FindAgentByName finds an agent by name.
	FindAgentByName(ctx context.Context, name string) (string, bool)

	// ListAgents lists all running agents.
	ListAgents(ctx context.Context) ([]AgentInfo, error)

	// SpawnAgentByName spawns an agent by manifest name.
	SpawnAgentByName(ctx context.Context, manifestName string) (string, error)
}

// AgentInfo contains basic information about an agent.
type AgentInfo struct {
	ID   string
	Name string
}
