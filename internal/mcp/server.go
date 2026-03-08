package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

const (
	MaxMcpMessageSize = 10 * 1024 * 1024
	McpServerName     = "fangclaw-go"
	McpServerVersion  = "0.1.0"
)

type McpServerBackend interface {
	ListAgents() ([]*AgentInfo, error)
	SendMessage(agentID, message string) (string, error)
}

type AgentInfo struct {
	ID          string
	Name        string
	Description string
}

type McpServer struct {
	backend McpServerBackend
}

func NewMcpServer(backend McpServerBackend) *McpServer {
	return &McpServer{
		backend: backend,
	}
}

func (s *McpServer) HandleRequest(request map[string]interface{}) (map[string]interface{}, error) {
	method, ok := request["method"].(string)
	if !ok {
		method = ""
	}
	id, _ := request["id"]

	switch method {
	case "initialize":
		return makeResponse(id, map[string]interface{}{
			"protocolVersion": McpProtocolVersion,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    McpServerName,
				"version": McpServerVersion,
			},
		}), nil

	case "notifications/initialized":
		return nil, nil

	case "tools/list":
		agents, err := s.backend.ListAgents()
		if err != nil {
			return makeError(id, -32603, fmt.Sprintf("Failed to list agents: %v", err)), nil
		}

		tools := make([]map[string]interface{}, 0, len(agents))
		for _, agent := range agents {
			toolName := formatMcpServerToolName(agent.Name)
			desc := agent.Description
			if desc == "" {
				desc = fmt.Sprintf("Send a message to fangclaw-go agent '%s'", agent.Name)
			}

			tools = append(tools, map[string]interface{}{
				"name":        toolName,
				"description": desc,
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Message to send to the agent",
						},
					},
					"required": []string{"message"},
				},
			})
		}

		return makeResponse(id, map[string]interface{}{
			"tools": tools,
		}), nil

	case "tools/call":
		params, ok := request["params"].(map[string]interface{})
		if !ok {
			params = map[string]interface{}{}
		}

		toolName, _ := params["name"].(string)
		arguments, _ := params["arguments"].(map[string]interface{})
		if arguments == nil {
			arguments = map[string]interface{}{}
		}

		message, _ := arguments["message"].(string)
		if message == "" {
			return makeError(id, -32602, "Missing 'message' argument"), nil
		}

		agentID, ok := s.resolveToolAgent(toolName)
		if !ok {
			return makeError(id, -32602, fmt.Sprintf("Unknown tool: %s", toolName)), nil
		}

		response, err := s.backend.SendMessage(agentID, message)
		if err != nil {
			return makeResponse(id, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Error: %v", err),
					},
				},
				"isError": true,
			}), nil
		}

		return makeResponse(id, map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": response,
				},
			},
		}), nil

	default:
		return makeError(id, -32601, fmt.Sprintf("Method not found: %s", method)), nil
	}
}

func (s *McpServer) resolveToolAgent(toolName string) (string, bool) {
	agentName, ok := parseMcpServerToolName(toolName)
	if !ok {
		return "", false
	}

	agents, err := s.backend.ListAgents()
	if err != nil {
		return "", false
	}

	for _, agent := range agents {
		if normalizeAgentName(agent.Name) == normalizeAgentName(agentName) {
			return agent.ID, true
		}
	}

	return "", false
}

func formatMcpServerToolName(agentName string) string {
	normalized := strings.Map(func(r rune) rune {
		if r == '-' || r == '.' || r == ' ' {
			return '_'
		}
		return r
	}, agentName)

	return fmt.Sprintf("fangclawgo_agent_%s", normalized)
}

func parseMcpServerToolName(toolName string) (string, bool) {
	if !strings.HasPrefix(toolName, "fangclawgo_agent_") {
		return "", false
	}

	agentName := strings.TrimPrefix(toolName, "fangclawgo_agent_")
	return agentName, true
}

func normalizeAgentName(name string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == '.' || r == ' ' {
			return '_'
		}
		return r
	}, name))
}

func makeResponse(id interface{}, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
}

func makeError(id interface{}, code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
}

type KernelMcpBackend struct {
	kernel KernelInterface
}

type KernelInterface interface {
	ListAgentEntries() []*AgentEntry
	SendMessage(ctx context.Context, agentID string, message string) (string, error)
}

type AgentEntry struct {
	ID       string
	Name     string
	Manifest types.AgentManifest
}

func NewKernelMcpBackend(kernel KernelInterface) *KernelMcpBackend {
	return &KernelMcpBackend{
		kernel: kernel,
	}
}

func (b *KernelMcpBackend) ListAgents() ([]*AgentInfo, error) {
	entries := b.kernel.ListAgentEntries()
	agents := make([]*AgentInfo, 0, len(entries))
	for _, entry := range entries {
		agents = append(agents, &AgentInfo{
			ID:          entry.ID,
			Name:        entry.Name,
			Description: entry.Manifest.Description,
		})
	}
	return agents, nil
}

func (b *KernelMcpBackend) SendMessage(agentID, message string) (string, error) {
	return b.kernel.SendMessage(context.Background(), agentID, message)
}

func RunStdioServer(server *McpServer) {
	stdin := os.Stdin
	stdout := os.Stdout

	reader := bufio.NewReader(stdin)
	writer := bufio.NewWriter(stdout)

	for {
		msg, err := readMessage(reader)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "MCP read error: %v\n", err)
			}
			break
		}
		if msg == nil {
			break
		}

		response, err := server.HandleRequest(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP handle error: %v\n", err)
			continue
		}

		if response != nil {
			if err := writeMessage(writer, response); err != nil {
				fmt.Fprintf(os.Stderr, "MCP write error: %v\n", err)
			}
		}
	}
}

func readMessage(reader *bufio.Reader) (map[string]interface{}, error) {
	var contentLength int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}

		if strings.HasPrefix(trimmed, "Content-Length: ") {
			lenStr := strings.TrimPrefix(trimmed, "Content-Length: ")
			_, err := fmt.Sscanf(lenStr, "%d", &contentLength)
			if err != nil {
				contentLength = 0
			}
		}
	}

	if contentLength == 0 {
		return nil, nil
	}

	if contentLength > MaxMcpMessageSize {
		discard := make([]byte, 4096)
		remaining := contentLength
		for remaining > 0 {
			toRead := remaining
			if toRead > 4096 {
				toRead = 4096
			}
			_, err := reader.Read(discard[:toRead])
			if err != nil {
				break
			}
			remaining -= toRead
		}
		return nil, fmt.Errorf("MCP message too large: %d bytes (max %d)", contentLength, MaxMcpMessageSize)
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func writeMessage(writer *bufio.Writer, msg map[string]interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}

	if _, err := writer.Write(body); err != nil {
		return err
	}

	return writer.Flush()
}
