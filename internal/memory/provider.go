package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// MemoryProvider is the interface that all memory providers must implement
type MemoryProvider interface {
	// Name returns the short identifier for this provider (e.g., "builtin", "honcho")
	Name() string

	// IsAvailable returns true if this provider is configured and ready
	IsAvailable() bool

	// Initialize initializes the provider for a session
	Initialize(sessionID string, ctx map[string]interface{}) error

	// SystemPromptBlock returns static text for the system prompt
	SystemPromptBlock() string

	// Prefetch returns relevant context for the upcoming turn (cached from queue_prefetch)
	Prefetch(query string, sessionID string) string

	// QueuePrefetch queues a background recall for the next turn
	QueuePrefetch(query string, sessionID string)

	// SyncTurn syncs a completed turn to the provider
	SyncTurn(userContent, assistantContent, sessionID string)

	// GetToolSchemas returns tool schemas this provider exposes
	GetToolSchemas() []map[string]interface{}

	// HandleToolCall handles a tool call for this provider
	HandleToolCall(ctx context.Context, toolName string, args map[string]interface{}) (string, error)

	// Shutdown cleans up resources
	Shutdown()

	// Optional lifecycle hooks
	OnTurnStart(turnNumber int, message string, ctx map[string]interface{})
	OnSessionEnd(messages []map[string]interface{})
	OnPreCompress(messages []map[string]interface{}) string
	OnMemoryWrite(action, target, content string)
	OnDelegation(task, result, childSessionID string)
}

// BaseMemoryProvider provides default implementations for optional hooks
type BaseMemoryProvider struct{}

func (b *BaseMemoryProvider) OnTurnStart(turnNumber int, message string, ctx map[string]interface{}) {
}
func (b *BaseMemoryProvider) OnSessionEnd(messages []map[string]interface{})           {}
func (b *BaseMemoryProvider) OnPreCompress(messages []map[string]interface{}) string   { return "" }
func (b *BaseMemoryProvider) OnMemoryWrite(action, target, content string)             {}
func (b *BaseMemoryProvider) OnDelegation(task, result, childSessionID string)         {}
func (b *BaseMemoryProvider) QueuePrefetch(query string, sessionID string)             {}
func (b *BaseMemoryProvider) SyncTurn(userContent, assistantContent, sessionID string) {}
func (b *BaseMemoryProvider) Prefetch(query string, sessionID string) string           { return "" }
func (b *BaseMemoryProvider) Shutdown()                                                {}

// MemoryManager orchestrates multiple memory providers
type MemoryManager struct {
	mu             sync.RWMutex
	providers      []MemoryProvider
	toolToProvider map[string]MemoryProvider
	hasExternal    bool
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager() *MemoryManager {
	return &MemoryManager{
		providers:      []MemoryProvider{},
		toolToProvider: map[string]MemoryProvider{},
		hasExternal:    false,
	}
}

// AddProvider adds a memory provider
func (mm *MemoryManager) AddProvider(provider MemoryProvider) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	isBuiltin := provider.Name() == "builtin"

	if !isBuiltin {
		if mm.hasExternal {
			// Find existing external provider name
			var existing string
			for _, p := range mm.providers {
				if p.Name() != "builtin" {
					existing = p.Name()
					break
				}
			}
			fmt.Printf("Warning: Rejected memory provider '%s' — external provider '%s' is already registered\n", provider.Name(), existing)
			return
		}
		mm.hasExternal = true
	}

	mm.providers = append(mm.providers, provider)

	// Index tool names to provider
	for _, schema := range provider.GetToolSchemas() {
		if toolName, ok := schema["name"].(string); ok && toolName != "" {
			if _, exists := mm.toolToProvider[toolName]; !exists {
				mm.toolToProvider[toolName] = provider
			} else {
				fmt.Printf("Warning: Memory tool name conflict: '%s' already registered by %s\n", toolName, mm.toolToProvider[toolName].Name())
			}
		}
	}

	fmt.Printf("Memory provider '%s' registered\n", provider.Name())
}

// Providers returns all registered providers
func (mm *MemoryManager) Providers() []MemoryProvider {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]MemoryProvider, len(mm.providers))
	copy(result, mm.providers)
	return result
}

// GetProvider returns a provider by name
func (mm *MemoryManager) GetProvider(name string) MemoryProvider {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// BuildSystemPrompt collects system prompt blocks from all providers
func (mm *MemoryManager) BuildSystemPrompt() string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var blocks []string
	for _, p := range mm.providers {
		if block := p.SystemPromptBlock(); block != "" {
			blocks = append(blocks, block)
		}
	}
	return JoinBlocks(blocks)
}

// PrefetchAll collects prefetch context from all providers
func (mm *MemoryManager) PrefetchAll(query string, sessionID string) string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var parts []string
	for _, p := range mm.providers {
		if result := p.Prefetch(query, sessionID); result != "" {
			parts = append(parts, result)
		}
	}
	return JoinBlocks(parts)
}

// QueuePrefetchAll queues background prefetch for next turn on all providers
func (mm *MemoryManager) QueuePrefetchAll(query string, sessionID string) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		p.QueuePrefetch(query, sessionID)
	}
}

// SyncAll syncs a completed turn to all providers
func (mm *MemoryManager) SyncAll(userContent, assistantContent, sessionID string) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		p.SyncTurn(userContent, assistantContent, sessionID)
	}
}

// GetAllToolSchemas collects tool schemas from all providers
func (mm *MemoryManager) GetAllToolSchemas() []map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var schemas []map[string]interface{}
	seen := map[string]bool{}
	for _, p := range mm.providers {
		for _, schema := range p.GetToolSchemas() {
			if name, ok := schema["name"].(string); ok && name != "" && !seen[name] {
				schemas = append(schemas, schema)
				seen[name] = true
			}
		}
	}
	return schemas
}

// HasTool checks if any provider handles this tool
func (mm *MemoryManager) HasTool(toolName string) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	_, ok := mm.toolToProvider[toolName]
	return ok
}

// HandleToolCall routes a tool call to the correct provider
func (mm *MemoryManager) HandleToolCall(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	mm.mu.RLock()
	provider, ok := mm.toolToProvider[toolName]
	mm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("no memory provider handles tool '%s'", toolName)
	}

	result, err := provider.HandleToolCall(ctx, toolName, args)
	if err != nil {
		return "", fmt.Errorf("memory tool '%s' failed: %w", toolName, err)
	}
	return result, nil
}

// Lifecycle hooks
func (mm *MemoryManager) OnTurnStart(turnNumber int, message string, ctx map[string]interface{}) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		p.OnTurnStart(turnNumber, message, ctx)
	}
}

func (mm *MemoryManager) OnSessionEnd(messages []map[string]interface{}) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		p.OnSessionEnd(messages)
	}
}

func (mm *MemoryManager) OnPreCompress(messages []map[string]interface{}) string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var parts []string
	for _, p := range mm.providers {
		if result := p.OnPreCompress(messages); result != "" {
			parts = append(parts, result)
		}
	}
	return JoinBlocks(parts)
}

func (mm *MemoryManager) OnMemoryWrite(action, target, content string) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		if p.Name() == "builtin" {
			continue
		}
		p.OnMemoryWrite(action, target, content)
	}
}

func (mm *MemoryManager) OnDelegation(task, result, childSessionID string) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		p.OnDelegation(task, result, childSessionID)
	}
}

func (mm *MemoryManager) InitializeAll(sessionID string, ctx map[string]interface{}) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, p := range mm.providers {
		if err := p.Initialize(sessionID, ctx); err != nil {
			fmt.Printf("Warning: Memory provider '%s' initialize failed: %v\n", p.Name(), err)
		}
	}
}

func (mm *MemoryManager) ShutdownAll() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Shutdown in reverse order for clean teardown
	for i := len(mm.providers) - 1; i >= 0; i-- {
		mm.providers[i].Shutdown()
	}
}

// Helper functions
func JoinBlocks(blocks []string) string {
	if len(blocks) == 0 {
		return ""
	}
	result := ""
	for i, b := range blocks {
		if i > 0 {
			result += "\n\n"
		}
		result += b
	}
	return result
}

func ToolError(msg string) string {
	result := map[string]interface{}{
		"success": false,
		"error":   msg,
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}
