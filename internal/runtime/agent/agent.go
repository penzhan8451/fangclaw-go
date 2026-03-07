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

	"github.com/penzhan8451/fangclaw-go/internal/embedding"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent/tools"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
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
	mu              sync.RWMutex
	drivers         map[string]llm.Driver
	tools           *ToolRegistry
	agents          map[string]*AgentContext
	semantic        *memory.SemanticStore
	sessions        *memory.SessionStore
	knowledge       *memory.KnowledgeStore
	usage           *memory.UsageStore
	skills          *skills.Loader
	embeddingDriver *embedding.EmbeddingDriver
}

func NewRuntime(semantic *memory.SemanticStore, sessions *memory.SessionStore, knowledge *memory.KnowledgeStore, usage *memory.UsageStore, skills *skills.Loader, embeddingDriver *embedding.EmbeddingDriver) *Runtime {
	return &Runtime{
		drivers:         make(map[string]llm.Driver),
		tools:           NewToolRegistry(),
		agents:          make(map[string]*AgentContext),
		semantic:        semantic,
		sessions:        sessions,
		knowledge:       knowledge,
		usage:           usage,
		skills:          skills,
		embeddingDriver: embeddingDriver,
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
	ID                 string
	Name               string
	Provider           string
	Model              string
	SystemPrompt       string
	SkillPromptContext string
	Messages           []types.Message
	Tools              []string
	Skills             []string
	Config             types.LoopConfig
	SessionID          types.SessionID
	AgentID            types.AgentID
	mu                 sync.Mutex
}

func NewAgentContext(id, name, provider, model, systemPrompt string, tools []string, skills []string, skillPromptContext string) *AgentContext {
	return &AgentContext{
		ID:                 id,
		Name:               name,
		Provider:           provider,
		Model:              model,
		SystemPrompt:       systemPrompt,       // agent definition，base system prompt
		SkillPromptContext: skillPromptContext, // agent Skill prompt context is added to the system prompt
		Tools:              tools,
		Skills:             skills, // skills to be used，e.g.["github", "calculator"] in ～/homedir/skills/{github,calculator}/skill.md
		Messages:           make([]types.Message, 0),
		Config:             types.LoopConfig{MaxIterations: 10, MaxTokens: 4096, Temperature: 0.7, TopP: 0.9},
		SessionID:          types.NewSessionID(),
		AgentID:            types.NewAgentID(),
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

// RunAgentLoop runs the agent execution loop.
// It handles the complete agent workflow: memory recall, LLM calls, tool execution, and session management.
func (r *Runtime) RunAgentLoop(ctx context.Context, agentCtx *AgentContext, onPhase PhaseCallback) (*AgentLoopResult, error) {
	// Get LLM driver for the agent's provider
	driver, err := r.GetDriver(agentCtx.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	// Recall relevant memories from semantic store
	memories, err := r.recallMemories(ctx, agentCtx)
	if err != nil {
		log.Printf("Warning: failed to recall memories: %v", err)
	}

	// Build system prompt with memories and skills
	systemPrompt := r.buildSystemPrompt(agentCtx.SystemPrompt, memories, agentCtx.Skills, agentCtx.SkillPromptContext)

	// Find user message from context
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

	// Trim message history if it exceeds maximum length
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

	// Get available tools for this agent
	availableTools := r.getAvailableTools(agentCtx.Tools, agentCtx.Skills)
	toolSchemas := make([]map[string]interface{}, len(availableTools))
	for i, tool := range availableTools {
		toolSchemas[i] = tool.Schema()
	}

	// Helper function to record token usage
	recordUsage := func(usage types.TokenUsage, iterations uint32) {
		if r.usage == nil {
			return
		}
		record := &types.UsageRecord{
			AgentID:   agentCtx.AgentID,
			SessionID: agentCtx.SessionID,
			Model:     agentCtx.Model,
			Provider:  agentCtx.Provider,
			Usage:     usage,
			CreatedAt: time.Now(),
		}
		if err := r.usage.RecordUsage(record); err != nil {
			log.Printf("Warning: failed to record usage: %v", err)
		}
	}

	// Main agent loop - iterate up to maxIterations
	for iteration := 0; iteration < maxIterations; iteration++ {
		if onPhase != nil {
			onPhase(PhaseThinking)
		}

		// Build messages for LLM including system prompt and history
		llmMessages := r.buildLLMMessages(systemPrompt, messages)

		// Call LLM with retry mechanism
		req := &llm.Request{
			Model:       agentCtx.Model,
			Messages:    llmMessages,
			Tools:       toolSchemas,
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

		// Check if we have tool calls to execute
		if len(resp.ToolCalls) > 0 {
			// Execute tool calls
			if onPhase != nil {
				onPhase(PhaseToolUse)
			}
			consecutiveMaxTokens = 0

			toolCalls := resp.ToolCalls
			var toolResults []string
			for _, tc := range toolCalls {
				result, err := r.executeTool(ctx, tc.Name, tc.Input)
				if err != nil {
					toolResults = append(toolResults, fmt.Sprintf("Error executing %s: %v", tc.Name, err))
				} else {
					toolResults = append(toolResults, fmt.Sprintf("Result from %s:\n%s", tc.Name, result))
				}
			}

			// Add tool results as user message
			toolResultMsg := types.Message{
				ID:        fmt.Sprintf("msg_%d", len(agentCtx.GetMessages())),
				Role:      "user",
				Content:   strings.Join(toolResults, "\n\n"),
				Timestamp: time.Now(),
			}
			agentCtx.AddMessage(toolResultMsg)
			messages = append(messages, toolResultMsg)
			continue
		}

		// Check stop reason
		switch resp.StopReason {
		case "stop", "end_turn", "":
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

			// Record usage
			recordUsage(totalUsage, uint32(iteration+1))

			return &AgentLoopResult{
				Response:   finalResponse,
				TotalUsage: totalUsage,
				Iterations: uint32(iteration + 1),
				Silent:     false,
			}, nil

		case "tool_calls", "tool_use":
			// This case is handled above, but keep it for completeness
			continue

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

				// Record usage
				recordUsage(totalUsage, uint32(iteration+1))

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

	// Record usage
	recordUsage(totalUsage, uint32(maxIterations))

	return &AgentLoopResult{
		Response:   "Max iterations exceeded. Please try again with a more specific request.",
		TotalUsage: totalUsage,
		Iterations: uint32(maxIterations),
		Silent:     false,
	}, fmt.Errorf("max iterations exceeded")
}

// parseToolCalls parses tool calls from response content (fallback for models that don't support structured tool calls).
func (r *Runtime) parseToolCalls(content string) []llm.ToolCall {
	var calls []llm.ToolCall

	// Try to parse JSON-based tool calls from text
	// This is a simplified implementation - in production, you'd want more robust parsing
	content = strings.TrimSpace(content)

	// Look for patterns like {"name": "tool", "input": {...}}
	// This is a basic parser, real implementation would need to handle edge cases
	if strings.Contains(content, "{") && strings.Contains(content, "}") {
		// Try to find and parse JSON objects
		// For now, return empty to avoid complexity - rely on structured tool_calls
	}

	return calls
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

	var queryEmbedding []float32
	if r.embeddingDriver != nil {
		embeddings, err := r.embeddingDriver.EmbedText(ctx, []string{userMessage})
		if err == nil && len(embeddings) > 0 {
			queryEmbedding = embeddings[0]
		}
	}

	memories, err := r.semantic.RecallWithEmbedding(userMessage, 5, &types.MemoryFilter{
		AgentID: &agentCtx.AgentID,
	}, queryEmbedding)
	if err != nil {
		return nil, err
	}

	return memories, nil
}

// buildSystemPrompt builds the system prompt with memories and skills.
func (r *Runtime) buildSystemPrompt(basePrompt string, memories []types.MemoryFragment, skillIDs []string, skillPromptContext string) string {
	prompt := basePrompt

	// Add bundled hand skill prompt context
	if skillPromptContext != "" {
		prompt += "\n\n## Hand Skill Context\n"
		prompt += skillPromptContext
	}

	// Add skills
	if r.skills != nil && len(skillIDs) > 0 {
		var skillsSection strings.Builder
		skillsSection.WriteString("\n\nSkills:\n")
		for _, skillID := range skillIDs {
			skill, err := r.skills.LoadSkill(skillID)
			if err != nil {
				log.Printf("Warning: failed to load skill %s: %v", skillID, err)
				continue
			}
			if skill.Manifest.PromptContext != "" {
				skillsSection.WriteString(fmt.Sprintf("\n--- %s ---\n", skill.Manifest.Name))
				skillsSection.WriteString(skill.Manifest.PromptContext)
				skillsSection.WriteString("\n")
			}
		}
		if skillsSection.Len() > len("\n\nSkills:\n") {
			prompt += skillsSection.String()
		}
	}

	// Add memories
	if len(memories) > 0 {
		memoriesSection := "\n\nRelevant memories:\n"
		for i, mem := range memories {
			memoriesSection += fmt.Sprintf("%d. %s\n", i+1, mem.Content)
		}
		prompt += memoriesSection
	}

	return prompt
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
// It includes both built-in tools and tools from the specified skills.
func (r *Runtime) getAvailableTools(toolNames []string, skillIDs []string) []tools.Tool {
	available := make([]tools.Tool, 0)

	// Add built-in tools
	if len(toolNames) == 0 {
		available = append(available, r.tools.List()...)
	} else {
		for _, name := range toolNames {
			if tool, ok := r.tools.Get(name); ok {
				available = append(available, tool)
			}
		}
	}

	// Add tools from skills if skill loader is available
	if r.skills != nil {
		for _, skillID := range skillIDs {
			if skill, ok := r.skills.GetSkill(skillID); ok {
				for _, toolDef := range skill.Manifest.Tools.Provided {
					// Create a wrapper that implements tools.Tool
					skillTool := &skillToolWrapper{
						skill:   skill,
						toolDef: toolDef,
						loader:  r.skills,
					}
					available = append(available, skillTool)
				}
			}
		}
	}

	return available
}

// skillToolWrapper wraps a skill tool to implement tools.Tool interface.
type skillToolWrapper struct {
	skill   *types.Skill
	toolDef types.SkillToolDefinition
	loader  *skills.Loader
}

func (t *skillToolWrapper) Name() string {
	return t.toolDef.Name
}

func (t *skillToolWrapper) Description() string {
	return t.toolDef.Description
}

func (t *skillToolWrapper) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	result, err := t.loader.ExecuteTool(t.skill.ID, t.toolDef.Name, args)
	if err != nil {
		return "", err
	}

	if result.IsError {
		if outputStr, ok := result.Output.(string); ok {
			return "", fmt.Errorf("%s", outputStr)
		}
		outputJSON, err := json.Marshal(result.Output)
		if err != nil {
			return "", fmt.Errorf("tool error")
		}
		return "", fmt.Errorf("%s", string(outputJSON))
	}

	if outputStr, ok := result.Output.(string); ok {
		return outputStr, nil
	}
	outputJSON, err := json.Marshal(result.Output)
	if err != nil {
		return fmt.Sprintf("%v", result.Output), nil
	}
	return string(outputJSON), nil
}

func (t *skillToolWrapper) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        t.toolDef.Name,
			"description": t.toolDef.Description,
			"parameters":  t.toolDef.Parameters,
		},
	}
}

// executeTool executes a tool.
// It first tries built-in tools, then falls back to skill tools.
func (r *Runtime) executeTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	// First, try to find a built-in tool
	if tool, ok := r.tools.Get(name); ok {
		// Create a timeout context
		toolCtx, cancel := context.WithTimeout(ctx, TOOL_TIMEOUT_SECS*time.Second)
		defer cancel()
		return tool.Execute(toolCtx, args)
	}

	// If no built-in tool found, try to find a skill that provides this tool
	if r.skills != nil {
		if skill, ok := r.skills.FindToolProvider(name); ok {
			// Execute the skill tool
			result, err := r.skills.ExecuteTool(skill.ID, name, args)
			if err != nil {
				return "", err
			}

			if result.IsError {
				if outputStr, ok := result.Output.(string); ok {
					return "", fmt.Errorf("%s", outputStr)
				}
				outputJSON, err := json.Marshal(result.Output)
				if err != nil {
					return "", fmt.Errorf("tool error")
				}
				return "", fmt.Errorf("%s", string(outputJSON))
			}

			if outputStr, ok := result.Output.(string); ok {
				return outputStr, nil
			}
			outputJSON, err := json.Marshal(result.Output)
			if err != nil {
				return fmt.Sprintf("%v", result.Output), nil
			}
			return string(outputJSON), nil
		}
	}

	return "", fmt.Errorf("tool not found: %s", name)
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

// RunAgent runs the agent with the given input message.
// It looks up the agent by ID, adds the user message, and executes the AgentLoop.
func (r *AgentRunner) RunAgent(ctx context.Context, agentID, input string, onPhase PhaseCallback) (*AgentLoopResult, error) {
	// Look up agent in runtime
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

	return r.Runtime.RunAgentLoop(ctx, agentCtx, onPhase)
}

func (r *Runtime) RegisterAgent(ctx context.Context, id, name, provider, model, systemPrompt string, tools []string, skills []string, skillPromptContext string) (*AgentContext, error) {
	_, err := r.GetDriver(provider)
	if err != nil {
		return nil, err
	}

	agentCtx := NewAgentContext(
		id,
		name, provider, model, systemPrompt, tools, skills, skillPromptContext,
	)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agentCtx.ID] = agentCtx
	log.Printf("Agent registered: %s (%s)", name, agentCtx.ID)
	return agentCtx, nil
}

// GetAgent retrieves an agent by its ID.
func (r *Runtime) GetAgent(id string) (*AgentContext, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[id]
	return agent, ok
}

// FindAgentByName finds an agent by its name.
func (r *Runtime) FindAgentByName(name string) (*AgentContext, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, agent := range r.agents {
		if agent.Name == name {
			return agent, true
		}
	}
	return nil, false
}

// GetFirstAgent returns the first available agent.
func (r *Runtime) GetFirstAgent() (*AgentContext, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, agent := range r.agents {
		return agent, true
	}
	return nil, false
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
