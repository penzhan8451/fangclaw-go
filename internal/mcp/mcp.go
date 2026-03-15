package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

const (
	McpProtocolVersion = "2024-11-05"
)

type IntegrationTemplate struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Logo        string    `json:"logo"`
	Config      McpConfig `json:"config"`
}

type McpConfig struct {
	Env       []string           `json:"env"`
	Transport types.McpTransport `json:"transport"`
}

type Integration struct {
	Template IntegrationTemplate `json:"template"`
	Config   map[string]string   `json:"config"`
}

type IntegrationRegistry struct {
	templates map[string]IntegrationTemplate
	mu        sync.RWMutex
}

func NewIntegrationRegistry() *IntegrationRegistry {
	return &IntegrationRegistry{
		templates: make(map[string]IntegrationTemplate),
	}
}

func (r *IntegrationRegistry) Register(template IntegrationTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[template.Name] = template
}

func (r *IntegrationRegistry) Get(name string) (IntegrationTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	template, ok := r.templates[name]
	return template, ok
}

func (r *IntegrationRegistry) List() []IntegrationTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	templates := make([]IntegrationTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		templates = append(templates, t)
	}
	return templates
}

func (r *IntegrationRegistry) CreateIntegration(name string, config map[string]string) (*Integration, error) {
	template, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("integration template %s not found", name)
	}
	return &Integration{
		Template: template,
		Config:   config,
	}, nil
}

type McpTransport interface {
	Send(ctx context.Context, req types.JsonRpcRequest) (*types.JsonRpcResponse, error)
	Close() error
}

type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
}

func NewStdioTransport(ctx context.Context, command string, args []string, env []string) (*StdioTransport, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

func (t *StdioTransport) Send(ctx context.Context, req types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqBytes = append(reqBytes, '\n')

	if _, err := t.stdin.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	scanner := bufio.NewScanner(t.stdout)

	responseChan := make(chan *types.JsonRpcResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var resp types.JsonRpcResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}

			if resp.ID != nil && *resp.ID == req.ID {
				responseChan <- &resp
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("scanner error: %w", err)
		} else {
			errChan <- io.EOF
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-responseChan:
		return resp, nil
	case err := <-errChan:
		return nil, err
	}
}

func (t *StdioTransport) Close() error {
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.stdout != nil {
		t.stdout.Close()
	}
	if t.stderr != nil {
		t.stderr.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
	return nil
}

type SseTransport struct {
	client  *http.Client
	baseURL string
}

func NewSseTransport(ctx context.Context, url string) (*SseTransport, error) {
	return &SseTransport{
		client:  &http.Client{},
		baseURL: url,
	}, nil
}

func (t *SseTransport) Send(ctx context.Context, req types.JsonRpcRequest) (*types.JsonRpcResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, strings.NewReader(string(reqBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var jsonResp types.JsonRpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &jsonResp, nil
}

func (t *SseTransport) Close() error {
	return nil
}

type McpConnection struct {
	config        types.McpServerConfig
	transport     McpTransport
	tools         []types.ToolDefinition
	originalNames map[string]string
	nextID        uint64
	mu            sync.Mutex
}

func Connect(ctx context.Context, config types.McpServerConfig) (*McpConnection, error) {
	var transport McpTransport
	var err error

	switch config.Transport.Type {
	case "stdio":
		transport, err = NewStdioTransport(ctx, config.Transport.Command, config.Transport.Args, config.Env)
	case "sse":
		transport, err = NewSseTransport(ctx, config.Transport.URL)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", config.Transport.Type)
	}

	if err != nil {
		return nil, err
	}

	conn := &McpConnection{
		config:        config,
		transport:     transport,
		tools:         make([]types.ToolDefinition, 0),
		originalNames: make(map[string]string),
		nextID:        1,
	}

	timeout := time.Duration(config.TimeoutSecs) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	initCtx, initCancel := context.WithTimeout(ctx, timeout)
	defer initCancel()

	if err := conn.initialize(initCtx); err != nil {
		transport.Close()
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	discoverCtx, discoverCancel := context.WithTimeout(ctx, timeout)
	defer discoverCancel()

	if err := conn.discoverTools(discoverCtx); err != nil {
		transport.Close()
		return nil, fmt.Errorf("discover tools failed: %w", err)
	}

	return conn, nil
}

func (c *McpConnection) nextRequestID() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextID
	c.nextID++
	return id
}

func (c *McpConnection) initialize(ctx context.Context) error {
	req := types.JsonRpcRequest{
		Jsonrpc: "2.0",
		ID:      c.nextRequestID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": McpProtocolVersion,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "fangclaw-go",
				"version": "0.1.0",
			},
		},
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return resp.Error
	}

	return nil
}

func (c *McpConnection) discoverTools(ctx context.Context) error {
	req := types.JsonRpcRequest{
		Jsonrpc: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return resp.Error
	}

	tools, ok := resp.Result["tools"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid tools response")
	}

	c.tools = make([]types.ToolDefinition, 0, len(tools))
	c.originalNames = make(map[string]string)

	for _, toolInterface := range tools {
		toolMap, ok := toolInterface.(map[string]interface{})
		if !ok {
			continue
		}

		originalName, _ := toolMap["name"].(string)
		description, _ := toolMap["description"].(string)
		inputSchema, _ := toolMap["inputSchema"].(map[string]interface{})

		if originalName == "" {
			continue
		}

		namespacedName := formatMcpToolName(c.config.Name, originalName)
		c.originalNames[namespacedName] = originalName

		tool := types.ToolDefinition{
			Name:        namespacedName,
			Description: description,
			Parameters:  inputSchema,
		}

		c.tools = append(c.tools, tool)
	}

	return nil
}

func (c *McpConnection) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*types.McpToolCallResult, error) {
	originalName, ok := c.originalNames[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown MCP tool: %s", toolName)
	}

	req := types.JsonRpcRequest{
		Jsonrpc: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      originalName,
			"arguments": arguments,
		},
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var result types.McpToolCallResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

func (c *McpConnection) Tools() []types.ToolDefinition {
	return c.tools
}

func (c *McpConnection) Close() error {
	return c.transport.Close()
}

func formatMcpToolName(serverName, toolName string) string {
	normalizedTool := strings.Map(func(r rune) rune {
		if r == '-' || r == '.' {
			return '_'
		}
		return r
	}, toolName)

	return fmt.Sprintf("mcp_%s_%s", serverName, normalizedTool)
}

func IsMcpTool(toolName string) bool {
	return strings.HasPrefix(toolName, "mcp_")
}

func ExtractMcpServer(toolName string) (string, bool) {
	if !IsMcpTool(toolName) {
		return "", false
	}
	parts := strings.SplitN(toolName, "_", 3)
	if len(parts) < 3 {
		return "", false
	}
	return parts[1], true
}
