package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// SemanticMemoryProvider is the memory provider that uses SemanticStore
type SemanticMemoryProvider struct {
	BaseMemoryProvider
	semanticStore *SemanticStore
	currentAgent  types.AgentID

	// Prefetch state
	prefetchLock sync.Mutex
	prefetchMap  map[string]string // sessionID -> cached prefetch result
}

func NewSemanticMemoryProvider(semanticStore *SemanticStore) *SemanticMemoryProvider {
	return &SemanticMemoryProvider{
		semanticStore: semanticStore,
		prefetchMap:   make(map[string]string),
	}
}

func (p *SemanticMemoryProvider) Name() string {
	return "semantic"
}

func (p *SemanticMemoryProvider) IsAvailable() bool {
	return p.semanticStore != nil
}

func (p *SemanticMemoryProvider) Initialize(sessionID string, ctx map[string]interface{}) error {
	// Get agent ID from context
	if agentIDStr, ok := ctx["agent_id"].(string); ok {
		p.currentAgent, _ = types.ParseAgentID(agentIDStr)
	}
	return nil
}

func (p *SemanticMemoryProvider) SystemPromptBlock() string {
	return "" // Semantic memory doesn't contribute to static system prompt
}

func (p *SemanticMemoryProvider) Prefetch(query string, sessionID string) string {
	p.prefetchLock.Lock()
	defer p.prefetchLock.Unlock()

	if result, ok := p.prefetchMap[sessionID]; ok {
		delete(p.prefetchMap, sessionID)
		return result
	}
	return ""
}

func (p *SemanticMemoryProvider) QueuePrefetch(query string, sessionID string) {
	if p.semanticStore == nil || query == "" || p.currentAgent == uuid.Nil {
		return
	}

	filter := &types.MemoryFilter{
		AgentID: &p.currentAgent,
	}
	memories, err := p.semanticStore.Recall(query, 10, filter)
	if err != nil || len(memories) == 0 {
		return
	}

	var blocks []string
	for _, mem := range memories {
		source := string(mem.Source)
		if mem.Source == "" {
			source = "unknown"
		}
		blocks = append(blocks, fmt.Sprintf("[%s] %s", source, mem.Content))
	}

	if len(blocks) > 0 {
		p.prefetchLock.Lock()
		p.prefetchMap[sessionID] = fmt.Sprintf("Semantic memory recall (relevant to \"%s\"):\n%s", query, JoinBlocks(blocks))
		p.prefetchLock.Unlock()
	}
}

func (p *SemanticMemoryProvider) GetToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "semantic_remember",
			"description": "Store a memory in semantic memory that can be recalled later based on content similarity.\n\nUse this for:\n- Important facts learned during conversations\n- Preferences and patterns that are not stable enough for long-term memory files\n- Context that might be relevant for future interactions",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The memory content to store (required)",
					},
					"source": map[string]interface{}{
						"type":        "string",
						"description": "Source of the memory (e.g., 'user_input', 'observation', 'tool_result')",
						"enum":        []string{"user_input", "observation", "tool_result", "inference", "other"},
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Scope of the memory (e.g., 'session', 'project', 'global')",
						"enum":        []string{"session", "project", "global"},
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Optional metadata key-value pairs",
					},
				},
				"required": []string{"content"},
			},
		},
		{
			"name":        "semantic_recall",
			"description": "Recall memories from semantic memory that are relevant to a query.\n\nUse this to find past context that might help with the current task.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query to find relevant memories (required)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of memories to recall (default 10, max 50)",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Optional scope filter",
						"enum":        []string{"session", "project", "global"},
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "semantic_forget",
			"description": "Forget (delete) a specific memory from semantic memory.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"memory_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the memory to forget (required)",
					},
				},
				"required": []string{"memory_id"},
			},
		},
	}
}

func (p *SemanticMemoryProvider) HandleToolCall(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if p.semanticStore == nil {
		return "", fmt.Errorf("semantic store not initialized")
	}
	if p.currentAgent == uuid.Nil {
		return "", fmt.Errorf("no agent context available")
	}

	switch toolName {
	case "semantic_remember":
		return p.handleRemember(ctx, args)
	case "semantic_recall":
		return p.handleRecall(ctx, args)
	case "semantic_forget":
		return p.handleForget(ctx, args)
	default:
		return "", fmt.Errorf("unsupported tool: %s", toolName)
	}
}

func (p *SemanticMemoryProvider) handleRemember(ctx context.Context, args map[string]interface{}) (string, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return ToolError("content required"), fmt.Errorf("content required")
	}

	sourceStr, _ := args["source"].(string)
	source := types.MemorySource("other")
	if sourceStr != "" {
		source = types.MemorySource(sourceStr)
	}

	scope, _ := args["scope"].(string)
	if scope == "" {
		scope = "session"
	}

	metadata, _ := args["metadata"].(map[string]interface{})

	id, err := p.semanticStore.Remember(p.currentAgent, content, source, scope, metadata)
	if err != nil {
		return ToolError(fmt.Sprintf("failed to store memory: %v", err)), err
	}

	jsonResult := map[string]interface{}{
		"success":   true,
		"message":   "Memory stored successfully",
		"memory_id": id.String(),
	}
	jsonBytes, _ := json.Marshal(jsonResult)
	return string(jsonBytes), nil
}

func (p *SemanticMemoryProvider) handleRecall(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return ToolError("query required"), fmt.Errorf("query required")
	}

	limit := 10
	if limitVal, ok := args["limit"].(float64); ok {
		limit = int(limitVal)
		if limit > 50 {
			limit = 50
		}
		if limit < 1 {
			limit = 10
		}
	}

	var filter *types.MemoryFilter
	if scope, _ := args["scope"].(string); scope != "" {
		filter = &types.MemoryFilter{
			AgentID: &p.currentAgent,
			Scope:   &scope,
		}
	} else {
		filter = &types.MemoryFilter{
			AgentID: &p.currentAgent,
		}
	}

	memories, err := p.semanticStore.Recall(query, limit, filter)
	if err != nil {
		return ToolError(fmt.Sprintf("failed to recall memories: %v", err)), err
	}

	if len(memories) == 0 {
		jsonResult := map[string]interface{}{
			"success":  true,
			"message":  "No relevant memories found",
			"memories": []interface{}{},
		}
		jsonBytes, _ := json.Marshal(jsonResult)
		return string(jsonBytes), nil
	}

	memList := make([]map[string]interface{}, len(memories))
	for i, mem := range memories {
		memList[i] = map[string]interface{}{
			"id":         mem.ID.String(),
			"content":    mem.Content,
			"source":     string(mem.Source),
			"scope":      mem.Scope,
			"created_at": mem.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"metadata":   mem.Metadata,
		}
	}

	jsonResult := map[string]interface{}{
		"success":  true,
		"message":  fmt.Sprintf("Found %d relevant memories", len(memories)),
		"memories": memList,
	}
	jsonBytes, _ := json.Marshal(jsonResult)
	return string(jsonBytes), nil
}

func (p *SemanticMemoryProvider) handleForget(ctx context.Context, args map[string]interface{}) (string, error) {
	memoryIDStr, _ := args["memory_id"].(string)
	if memoryIDStr == "" {
		return ToolError("memory_id required"), fmt.Errorf("memory_id required")
	}

	memoryID, err := types.ParseMemoryID(memoryIDStr)
	if err != nil {
		return ToolError(fmt.Sprintf("invalid memory_id: %v", err)), err
	}

	err = p.semanticStore.Forget(memoryID)
	if err != nil {
		return ToolError(fmt.Sprintf("failed to forget memory: %v", err)), err
	}

	jsonResult := map[string]interface{}{
		"success":   true,
		"message":   "Memory forgotten successfully",
		"memory_id": memoryIDStr,
	}
	jsonBytes, _ := json.Marshal(jsonResult)
	return string(jsonBytes), nil
}
