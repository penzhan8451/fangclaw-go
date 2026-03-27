package channels

import (
	"context"

	"github.com/penzhan8451/fangclaw-go/internal/autoreply"
)

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

	// GetAutoReplyEngine returns the auto-reply engine.
	GetAutoReplyEngine() *autoreply.AutoReplyEngine

	// RecordDelivery records the outcome of sending a message reply back to a channel.
	// agentID is the agent that produced the response; channel is the channel type name;
	// recipient is the platform user identifier; success indicates whether the send succeeded;
	// errMsg is the error message on failure (empty on success).
	RecordDelivery(ctx context.Context, agentID, channel, recipient string, success bool, errMsg string)
}

// AgentInfo contains basic information about an agent.
type AgentInfo struct {
	ID   string
	Name string
}
