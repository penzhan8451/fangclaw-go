package types

import (
	"encoding/json"
)

type McpServerConfig struct {
	Name        string      `json:"name"`
	Transport   McpTransport `json:"transport"`
	TimeoutSecs uint64      `json:"timeout_secs,omitempty"`
	Env         []string    `json:"env,omitempty"`
}

type McpTransport struct {
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
}

func (t McpTransport) MarshalJSON() ([]byte, error) {
	switch t.Type {
	case "stdio":
		return json.Marshal(struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args,omitempty"`
		}{t.Type, t.Command, t.Args})
	case "sse":
		return json.Marshal(struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		}{t.Type, t.URL})
	default:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{t.Type})
	}
}

type JsonRpcRequest struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      uint64                 `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type JsonRpcResponse struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      *uint64                `json:"id,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *JsonRpcError          `json:"error,omitempty"`
}

type JsonRpcError struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e JsonRpcError) Error() string {
	return e.Message
}

type McpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type McpToolCallResult struct {
	Content []McpContent `json:"content"`
}

type McpContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type McpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type McpInitializeResult struct {
	ProtocolVersion string        `json:"protocolVersion"`
	Server          McpServerInfo `json:"server"`
	Capabilities    interface{}   `json:"capabilities"`
}

type McpListToolsResult struct {
	Tools      []McpTool `json:"tools"`
	NextCursor *string   `json:"nextCursor,omitempty"`
}
