// Package types provides core data structures for OpenFang.
package types

// ToolDefinition defines a tool that can be called by the agent.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function call.
type ToolFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	ToolCallID string      `json:"tool_call_id"`
	Content    interface{} `json:"content"`
	IsError    bool        `json:"is_error"`
}
