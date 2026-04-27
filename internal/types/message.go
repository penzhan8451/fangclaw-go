// Package types provides core data structures for OpenFang.
package types

import (
	"time"

	"github.com/google/uuid"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// StopReason represents why the agent loop stopped.
type StopReason string

const (
	StopReasonToolCalls     StopReason = "tool_calls"
	StopReasonEndTurn       StopReason = "end_turn"
	StopReasonStopSequence  StopReason = "stop_sequence"
	StopReasonMaxTokens     StopReason = "max_tokens"
	StopReasonMaxIterations StopReason = "max_iterations"
	StopReasonError         StopReason = "error"
)

// TokenUsage tracks token usage.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ToolCalls        int `json:"tool_calls"`
}

// Message represents a chat message.
type Message struct {
	ID               string     `json:"id"`
	Role             string     `json:"role"` // "system", "user", "assistant", "tool"
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"` // For tool responses
	Timestamp        time.Time  `json:"timestamp"`
}

// NewTextMessage creates a new text message.
func NewTextMessage(role Role, text string) Message {
	return Message{
		ID:        uuid.New().String(),
		Role:      string(role),
		Content:   text,
		Timestamp: time.Now(),
	}
}

// NewTextMessageWithReasoning creates a new text message with reasoning content.
func NewTextMessageWithReasoning(role Role, text, reasoning string) Message {
	return Message{
		ID:               uuid.New().String(),
		Role:             string(role),
		Content:          text,
		ReasoningContent: reasoning,
		Timestamp:        time.Now(),
	}
}

// NewToolMessage creates a new tool response message.
func NewToolMessage(toolCallID, name, content string, isError bool) Message {
	var text string
	if isError {
		text = "Error: " + content
	} else {
		text = content
	}
	return Message{
		ID:         uuid.New().String(),
		Role:       string(RoleTool),
		Content:    text,
		ToolCallID: toolCallID,
		Name:       name,
		Timestamp:  time.Now(),
	}
}

// ReplyDirectives contains directives for how the agent should reply.
type ReplyDirectives struct {
	Silent  bool     `json:"silent"`
	Targets []string `json:"targets,omitempty"`
	Format  string   `json:"format,omitempty"`
}

// SessionID is a unique identifier for a session.
type SessionID = uuid.UUID

// NewSessionID creates a new session ID.
func NewSessionID() SessionID {
	return uuid.New()
}

// ParseSessionID parses a string into a SessionID.
func ParseSessionID(s string) (SessionID, error) {
	return uuid.Parse(s)
}

// Session represents a conversation session with message history.
type Session struct {
	ID                  SessionID `json:"id"`
	AgentID             AgentID   `json:"agent_id"`
	AgentName           string    `json:"agent_name"`
	AgentModelProvider  string    `json:"agent_model_provider"`
	AgentModelName      string    `json:"agent_model_name"`
	Messages            []Message `json:"messages"`
	ContextWindowTokens uint64    `json:"context_window_tokens"`
	Label               *string   `json:"label,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// NewSession creates a new session.
func NewSession(agentID AgentID, agentName, agentModelProvider, agentModelName string, label *string) Session {
	now := time.Now()
	return Session{
		ID:                 NewSessionID(),
		AgentID:            agentID,
		AgentName:          agentName,
		AgentModelProvider: agentModelProvider,
		AgentModelName:     agentModelName,
		Messages:           []Message{},
		Label:              label,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetMessages returns messages, trimming to the limit if specified.
func (s *Session) GetMessages(limit int) []Message {
	if limit <= 0 || len(s.Messages) <= limit {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-limit:]
}
