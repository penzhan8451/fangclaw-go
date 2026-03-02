// Package agent provides the agent runtime for OpenFang.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent/tools"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

const (
	MAX_ITERATIONS       = 50
	MAX_RETRIES          = 3
	BASE_RETRY_DELAY_MS  = 1000
	TOOL_TIMEOUT_SECS    = 120
	MAX_CONTINUATIONS    = 5
	MAX_HISTORY_MESSAGES = 20
)

// LoopPhase represents the agent lifecycle phase.
type LoopPhase string

const (
	PhaseThinking  LoopPhase = "thinking"
	PhaseToolUse   LoopPhase = "tool_use"
	PhaseStreaming LoopPhase = "streaming"
	PhaseDone      LoopPhase = "done"
	PhaseError     LoopPhase = "error"
)

// PhaseCallback is a callback for agent lifecycle phase changes.
type PhaseCallback func(phase LoopPhase)

// AgentLoopResult represents the result of an agent loop execution.
type AgentLoopResult struct {
	Response   string
	TotalUsage types.TokenUsage
	Iterations uint32
	CostUSD    *float64
	Silent     bool
	Directives types.ReplyDirectives
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]tools.Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]tools.Tool)}
}

func (r *ToolRegistry) Register(tool tools.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (tools.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *ToolRegistry) List() []tools.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	toolsList := make([]tools.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		toolsList = append(toolsList, tool)
	}
	return toolsList
}

func (r *ToolRegistry) GetSchema() []map[string]interface{} {
	toolList := r.List()
	schemas := make([]map[string]interface{}, len(toolList))
	for i, tool := range toolList {
		schemas[i] = tool.Schema()
	}
	return schemas
}

// Runtime is the agent runtime that manages agents and their execution.
type Runtime struct {
	mu        sync.RWMutex
	drivers   map[string]llm.Driver
	tools     *ToolRegistry
	agents    map[string]*AgentContext
	semantic  *memory.SemanticStore
	sessions  *memory.SessionStore
	knowledge *memory.KnowledgeStore
}

func NewRuntime(semantic *memory.SemanticStore, sessions *memory.SessionStore, knowledge *memory.KnowledgeStore) *Runtime {
	return &Runtime{
		drivers:   make(map[string]llm.Driver),
		tools:     NewToolRegistry(),
		agents:    make(map[string]*AgentContext),
		semantic:  semantic,
		sessions:  sessions,
		knowledge: knowledge,
	}
}

func (r *Runtime) RegisterDriver(provider string, driver llm.Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[provider] = driver
}

func (r *Runtime) GetDriver(provider string) (llm.Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	driver, ok := r.drivers[provider]
	if !ok {
		return nil, fmt.Errorf("no driver for provider: %s", provider)
	}
	return driver, nil
}

func (r *Runtime) RegisterTool(tool tools.Tool) {
	r.tools.Register(tool)
}

// AgentContext represents the context for a running agent.
type AgentContext struct {
	ID           string
	Name         string
	Provider     string
	Model        string
	SystemPrompt string
	Messages     []types.Message
	Tools        []string
	Config       types.LoopConfig
	SessionID    types.SessionID
	AgentID      types.AgentID
	mu           sync.Mutex
}

func NewAgentContext(id, name, provider, model, systemPrompt string, tools []string) *AgentContext {
	return &AgentContext{
		ID:           id,
		Name:         name,
		Provider:     provider,
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		Messages:     make([]types.Message, 0),
		Config:       types.LoopConfig{MaxIterations: 10, MaxTokens: 4096, Temperature: 0.7, TopP: 0.9},
		SessionID:    types.NewSessionID(),
		AgentID:      types.NewAgentID(),
	}
}

func (c *AgentContext) AddMessage(msg types.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Messages = append(c.Messages, msg)
}

func (c *AgentContext) GetMessages() []types.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Messages
}

// RunLoop runs the agent execution loop.
func (r *Runtime) RunLoop(ctx context.Context, agentCtx *AgentContext, onPhase PhaseCallback) (*AgentLoopResult, error) {
	driver, err := r.GetDriver(agentCtx.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Recall relevant memories
	memories, err := r.recallMemories(ctx, agentCtx)
	if err != nil {
		log.Printf("Warning: failed to recall memories: %v", err)
	}

	// Build system prompt with memories
	systemPrompt := r.buildSystemPrompt(agentCtx.SystemPrompt, memories)

	// Add user message to history
	var userMessage string
	for _, msg := range agentCtx.GetMessages() {
		if msg.Role == "user" {
			userMessage = msg.Content
			break
		}
	}

	if userMessage == "" {
		return nil, fmt.Errorf("no user message found")
	}

	// Trim message history if too long
	messages := agentCtx.GetMessages()
	if len(messages) > MAX_HISTORY_MESSAGES {
		trimCount := len(messages) - MAX_HISTORY_MESSAGES
		messages = messages[trimCount:]
	}

	totalUsage := types.TokenUsage{}
	var finalResponse string
	consecutiveMaxTokens := uint32(0)

	maxIterations := agentCtx.Config.MaxIterations
	if maxIterations == 0 {
		maxIterations = MAX_ITERATIONS
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		if onPhase != nil {
			onPhase(PhaseThinking)
		}

		// Build LLM messages
		llmMessages := r.buildLLMMessages(systemPrompt, messages)

		// Get available tools (for future use)
		_ = r.getAvailableTools(agentCtx.Tools)

		// Call LLM with retry
		req := &llm.Request{
			Model:       agentCtx.Model,
			Messages:    llmMessages,
			MaxTokens:   agentCtx.Config.MaxTokens,
			Temperature: agentCtx.Config.Temperature,
			TopP:        agentCtx.Config.TopP,
		}

		resp, err := r.callLLMWithRetry(ctx, driver, req)
		if err != nil {
			return nil, fmt.Errorf("LLM call failed after retries: %w", err)
		}

		// Update token usage
		totalUsage.PromptTokens += resp.Usage.InputTokens
		totalUsage.CompletionTokens += resp.Usage.OutputTokens
		totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens

		// Add assistant message to history
		assistantMsg := types.Message{
			ID:        fmt.Sprintf("msg_%d", len(agentCtx.GetMessages())),
			Role:      "assistant",
			Content:   resp.Content,
			Timestamp: time.Now(),
		}
		agentCtx.AddMessage(assistantMsg)
		messages = append(messages, assistantMsg)

		// Check stop reason
		switch resp.StopReason {
		case "stop", "end_turn":
			// LLM is done
			finalResponse = resp.Content

			// Save session
			if err := r.saveSession(ctx, agentCtx); err != nil {
				log.Printf("Warning: failed to save session: %v", err)
			}

			// Remember this interaction
			if err := r.rememberInteraction(ctx, agentCtx, userMessage, finalResponse); err != nil {
				log.Printf("Warning: failed to remember interaction: %v", err)
			}

			if onPhase != nil {
				onPhase(PhaseDone)
			}

			return &AgentLoopResult{
				Response:   finalResponse,
				TotalUsage: totalUsage,
				Iterations: uint32(iteration + 1),
				Silent:     false,
			}, nil

		case "tool_calls", "tool_use":
			// Reset max tokens counter
			consecutiveMaxTokens = 0

			if onPhase != nil {
				onPhase(PhaseToolUse)
			}

			// Parse tool calls (simplified - in real implementation, you'd parse structured tool calls)
			toolCalls := r.parseToolCalls(resp.Content)

			if len(toolCalls) > 0 {
				// Execute tool calls
				toolResults := make([]string, 0, len(toolCalls))
				for _, tc := range toolCalls {
					result, err := r.executeTool(ctx, tc.Name, tc.Args)
					if err != nil {
						toolResults = append(toolResults, fmt.Sprintf("Error: %v", err))
					} else {
						toolResults = append(toolResults, result)
					}
				}

				// Add tool results as user message
				toolResultMsg := types.Message{
					ID:        fmt.Sprintf("msg_%d", len(agentCtx.GetMessages())),
					Role:      "user",
					Content:   strings.Join(toolResults, "\n"),
					Timestamp: time.Now(),
				}
				agentCtx.AddMessage(toolResultMsg)
				messages = append(messages, toolResultMsg)
			} else {
				// No tool calls found, just continue with response
				finalResponse = resp.Content

				if err := r.saveSession(ctx, agentCtx); err != nil {
					log.Printf("Warning: failed to save session: %v", err)
				}

				if onPhase != nil {
					onPhase(PhaseDone)
				}

				return &AgentLoopResult{
					Response:   finalResponse,
					TotalUsage: totalUsage,
					Iterations: uint32(iteration + 1),
					Silent:     false,
				}, nil
			}

		case "length", "max_tokens":
			consecutiveMaxTokens++
			if consecutiveMaxTokens >= MAX_CONTINUATIONS {
				// Return partial response
				finalResponse = resp.Content
				if finalResponse == "" {
					finalResponse = "[Partial response — token limit reached]"
				}

				if err := r.saveSession(ctx, agentCtx); err != nil {
					log.Printf("Warning: failed to save session: %v", err)
				}

				if onPhase != nil {
					onPhase(PhaseDone)
				}

				return &AgentLoopResult{
					Response:   finalResponse,
					TotalUsage: totalUsage,
					Iterations: uint32(iteration + 1),
					Silent:     false,
				}, nil
			}

			// Continue generation
			continueMsg := types.Message{
				ID:        fmt.Sprintf("msg_%d", len(agentCtx.GetMessages())),
				Role:      "user",
				Content:   "Please continue.",
				Timestamp: time.Now(),
			}
			agentCtx.AddMessage(continueMsg)
			messages = append(messages, continueMsg)

		default:
			// Unknown stop reason, just return response
			finalResponse = resp.Content

			if err := r.saveSession(ctx, agentCtx); err != nil {
				log.Printf("Warning: failed to save session: %v", err)
			}

			if onPhase != nil {
				onPhase(PhaseDone)
			}

			return &AgentLoopResult{
				Response:   finalResponse,
				TotalUsage: totalUsage,
				Iterations: uint32(iteration + 1),
				Silent:     false,
			}, nil
		}
	}

	// Max iterations exceeded
	if onPhase != nil {
		onPhase(PhaseError)
	}

	return &AgentLoopResult{
		Response:   "Max iterations exceeded. Please try again with a more specific request.",
		TotalUsage: totalUsage,
		Iterations: uint32(maxIterations),
		Silent:     false,
	}, fmt.Errorf("max iterations exceeded")
}

// ToolCall represents a parsed tool call.
type ToolCall struct {
	Name string
	Args map[string]interface{}
}

// parseToolCalls parses tool calls from response content (simplified).
func (r *Runtime) parseToolCalls(content string) []ToolCall {
	// This is a simplified implementation.
	// In a real implementation, you'd parse structured tool calls from the LLM response.
	// For now, we'll return an empty slice to indicate no tool calls.
	return []ToolCall{}
}

// callLLMWithRetry calls the LLM with exponential backoff retry.
func (r *Runtime) callLLMWithRetry(ctx context.Context, driver llm.Driver, req *llm.Request) (*llm.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= MAX_RETRIES; attempt++ {
		resp, err := driver.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		log.Printf("LLM call failed (attempt %d/%d): %v", attempt+1, MAX_RETRIES+1, err)

		if attempt < MAX_RETRIES {
			delay := time.Duration(BASE_RETRY_DELAY_MS*(1<<attempt)) * time.Millisecond
			log.Printf("Retrying in %v...", delay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// recallMemories recalls relevant memories for the agent.
func (r *Runtime) recallMemories(ctx context.Context, agentCtx *AgentContext) ([]types.MemoryFragment, error) {
	if r.semantic == nil {
		return []types.MemoryFragment{}, nil
	}

	var userMessage string
	for _, msg := range agentCtx.GetMessages() {
		if msg.Role == "user" {
			userMessage = msg.Content
			break
		}
	}

	if userMessage == "" {
		return []types.MemoryFragment{}, nil
	}

	memories, err := r.semantic.Recall(userMessage, 5, &types.MemoryFilter{
		AgentID: &agentCtx.AgentID,
	})
	if err != nil {
		return nil, err
	}

	return memories, nil
}

// buildSystemPrompt builds the system prompt with memories.
func (r *Runtime) buildSystemPrompt(basePrompt string, memories []types.MemoryFragment) string {
	if len(memories) == 0 {
		return basePrompt
	}

	memoriesSection := "\n\nRelevant memories:\n"
	for i, mem := range memories {
		memoriesSection += fmt.Sprintf("%d. %s\n", i+1, mem.Content)
	}

	return basePrompt + memoriesSection
}

// buildLLMMessages builds the messages for the LLM.
func (r *Runtime) buildLLMMessages(systemPrompt string, messages []types.Message) []llm.Message {
	llmMsgs := make([]llm.Message, 0)
	if systemPrompt != "" {
		llmMsgs = append(llmMsgs, llm.Message{Role: "system", Content: systemPrompt})
	}
	for _, msg := range messages {
		llmMsgs = append(llmMsgs, llm.Message{Role: msg.Role, Content: msg.Content})
	}
	return llmMsgs
}

// getAvailableTools gets the available tools for the agent.
func (r *Runtime) getAvailableTools(toolNames []string) []tools.Tool {
	if len(toolNames) == 0 {
		return r.tools.List()
	}

	available := make([]tools.Tool, 0)
	for _, name := range toolNames {
		if tool, ok := r.tools.Get(name); ok {
			available = append(available, tool)
		}
	}
	return available
}

// executeTool executes a tool.
func (r *Runtime) executeTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	tool, ok := r.tools.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// Create a timeout context
	toolCtx, cancel := context.WithTimeout(ctx, TOOL_TIMEOUT_SECS*time.Second)
	defer cancel()

	return tool.Execute(toolCtx, args)
}

// saveSession saves the agent session.
func (r *Runtime) saveSession(ctx context.Context, agentCtx *AgentContext) error {
	if r.sessions == nil {
		return nil
	}

	session := &types.Session{
		ID:                  agentCtx.SessionID,
		AgentID:             agentCtx.AgentID,
		Messages:            agentCtx.GetMessages(),
		ContextWindowTokens: uint64(agentCtx.Config.MaxTokens),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	return r.sessions.SaveSession(session)
}

// rememberInteraction remembers the interaction in semantic memory.
func (r *Runtime) rememberInteraction(ctx context.Context, agentCtx *AgentContext, userMsg, assistantMsg string) error {
	if r.semantic == nil {
		return nil
	}

	interactionText := fmt.Sprintf("User asked: %s\nI responded: %s", userMsg, assistantMsg)
	_, err := r.semantic.Remember(agentCtx.AgentID, interactionText, types.MemorySourceConversation, "episodic", map[string]interface{}{})
	return err
}

// AgentRunner is a helper for running agents.
type AgentRunner struct {
	Runtime *Runtime
}

func NewAgentRunner(rt *Runtime) *AgentRunner {
	return &AgentRunner{Runtime: rt}
}

func (r *AgentRunner) RunAgent(ctx context.Context, agentID, input string, onPhase PhaseCallback) (*AgentLoopResult, error) {
	r.Runtime.mu.RLock()
	agentCtx, ok := r.Runtime.agents[agentID]
	r.Runtime.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	// Add user message
	userMsg := types.Message{
		ID:        fmt.Sprintf("msg_%d", len(agentCtx.GetMessages())),
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	}
	agentCtx.AddMessage(userMsg)

	return r.Runtime.RunLoop(ctx, agentCtx, onPhase)
}

func (r *Runtime) RegisterAgent(ctx context.Context, name, provider, model, systemPrompt string, tools []string) (*AgentContext, error) {
	_, err := r.GetDriver(provider)
	if err != nil {
		return nil, err
	}

	agentCtx := NewAgentContext(
		fmt.Sprintf("agent_%d", time.Now().Unix()),
		name, provider, model, systemPrompt, tools,
	)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agentCtx.ID] = agentCtx
	log.Printf("Agent registered: %s (%s)", name, agentCtx.ID)
	return agentCtx, nil
}

func (r *Runtime) GetAgent(id string) (*AgentContext, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[id]
	return agent, ok
}

func (r *Runtime) ListAgents() []*AgentContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agents := make([]*AgentContext, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (r *Runtime) DeleteAgent(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[id]; ok {
		delete(r.agents, id)
		return true
	}
	return false
}

func ParseToolArguments(jsonStr string) (map[string]interface{}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}
	return args, nil
}

func BuildToolSchema(name, description string, params map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        name,
			"description": description,
			"parameters":  map[string]interface{}{"type": "object", "properties": params, "required": []string{}},
		},
	}
}

func ContainsToolCall(content string) bool {
	return strings.Contains(content, "tool_calls") || strings.Contains(content, "function_call") || strings.Contains(content, "_fn {")
}
