package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// FileMemoryProvider is the built-in memory provider that uses MEMORY.md and USER.md
type FileMemoryProvider struct {
	BaseMemoryProvider
	fileStore *FileStore

	// Prefetch state
	prefetchLock sync.Mutex
	prefetchMap  map[string]string // sessionID -> cached prefetch result

	// Snapshot frozen at initialization
	systemPromptSnapshot string
}

func NewFileMemoryProvider(fileStore *FileStore) *FileMemoryProvider {
	return &FileMemoryProvider{
		fileStore:   fileStore,
		prefetchMap: make(map[string]string),
	}
}

func (p *FileMemoryProvider) Name() string {
	return "builtin"
}

func (p *FileMemoryProvider) IsAvailable() bool {
	// Always available since it's file-based
	return true
}

func (p *FileMemoryProvider) Initialize(sessionID string, ctx map[string]interface{}) error {
	// Capture frozen snapshot from file store
	if p.fileStore != nil {
		memorySnap := p.fileStore.GetSnapshot("memory")
		userSnap := p.fileStore.GetSnapshot("user")

		var blocks []string
		if memorySnap != "" {
			blocks = append(blocks, fmt.Sprintf("══════════════════════════════════════════════\nMEMORY (your personal notes)\n══════════════════════════════════════════════\n%s", memorySnap))
		}
		if userSnap != "" {
			blocks = append(blocks, fmt.Sprintf("══════════════════════════════════════════════\nUSER PROFILE (who the user is)\n══════════════════════════════════════════════\n%s", userSnap))
		}
		p.systemPromptSnapshot = JoinBlocks(blocks)
	}
	return nil
}

func (p *FileMemoryProvider) SystemPromptBlock() string {
	return p.systemPromptSnapshot
}

func (p *FileMemoryProvider) Prefetch(query string, sessionID string) string {
	p.prefetchLock.Lock()
	defer p.prefetchLock.Unlock()

	if result, ok := p.prefetchMap[sessionID]; ok {
		delete(p.prefetchMap, sessionID)
		return result
	}
	return ""
}

func (p *FileMemoryProvider) QueuePrefetch(query string, sessionID string) {
	if p.fileStore == nil {
		return
	}

	var blocks []string
	memoryContent, _ := p.fileStore.Read("memory")
	userContent, _ := p.fileStore.Read("user")

	if memoryContent != "" {
		blocks = append(blocks, fmt.Sprintf("Recalled memory:\n%s", memoryContent))
	}
	if userContent != "" {
		blocks = append(blocks, fmt.Sprintf("Recalled user profile:\n%s", userContent))
	}

	if len(blocks) > 0 {
		p.prefetchLock.Lock()
		p.prefetchMap[sessionID] = JoinBlocks(blocks)
		p.prefetchLock.Unlock()
	}
}

func (p *FileMemoryProvider) GetToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name": "memory_manage",
			"description": "Read, write, replace, remove, clear, and snapshot memory files (MEMORY.md and USER.md)\n\nWHEN TO SAVE (do this proactively, don't wait to be asked):\n- User corrects you or says 'remember this' / 'don't do that again'\n- User shares a preference, habit, or personal detail (name, role, timezone, coding style)\n- You discover something about the environment (OS, installed tools, project structure)\n- You learn a convention, API quirk, or workflow specific to this user's setup\n\nPRIORITY: User preferences and corrections > environment facts\n\nTWO TARGETS:\n- 'user': who the user is -- name, role, preferences, communication style\n- 'memory': your notes -- environment facts, project conventions, lessons learned\n\nACTIONS: read, write, replace, remove, clear, snapshot\n\nSKIP: trivial/obvious info, things easily re-discovered, raw data dumps",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform: read, write, replace, remove, clear, snapshot",
						"enum":        []string{"read", "write", "replace", "remove", "clear", "snapshot"},
					},
					"target": map[string]interface{}{
						"type":        "string",
						"description": "Target memory file: 'memory' (MEMORY.md) or 'user' (USER.md)",
						"enum":        []string{"memory", "user"},
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write or replace with (required for write, replace)",
					},
					"old_content": map[string]interface{}{
						"type":        "string",
						"description": "Old content to replace (required for replace and remove actions)",
					},
				},
				"required": []string{"action", "target"},
			},
		},
	}
}

func (p *FileMemoryProvider) HandleToolCall(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if toolName != "memory_manage" {
		return "", fmt.Errorf("unsupported tool: %s", toolName)
	}

	if p.fileStore == nil {
		return "", fmt.Errorf("file store not initialized")
	}

	action, _ := args["action"].(string)
	if action == "" {
		return ToolError("action required"), fmt.Errorf("action required")
	}

	target, _ := args["target"].(string)
	if target == "" {
		return ToolError("target required (must be 'memory' or 'user')"), fmt.Errorf("target required")
	}

	var result string
	var err error

	switch action {
	case "read":
		result, err = p.read(target)
	case "write":
		content, _ := args["content"].(string)
		result, err = p.write(target, content)
	case "replace":
		oldContent, _ := args["old_content"].(string)
		newContent, _ := args["content"].(string)
		result, err = p.replace(target, oldContent, newContent)
	case "remove":
		textToRemove, _ := args["old_content"].(string)
		result, err = p.remove(target, textToRemove)
	case "clear":
		result, err = p.clear(target)
	case "snapshot":
		result, err = p.snapshot(target)
	default:
		return ToolError(fmt.Sprintf("unknown action: %s", action)), fmt.Errorf("unknown action")
	}

	if err != nil {
		return ToolError(err.Error()), err
	}

	jsonResult := map[string]interface{}{
		"success": true,
		"message": result,
	}
	jsonBytes, _ := json.Marshal(jsonResult)

	// Notify other providers about memory write
	if action == "write" || action == "replace" || action == "remove" || action == "clear" {
		p.OnMemoryWrite(action, target, "") // TODO: pass actual content if needed
	}

	return string(jsonBytes), nil
}

func (p *FileMemoryProvider) read(target string) (string, error) {
	content, err := p.fileStore.Read(target)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", target, err)
	}
	if content == "" {
		return fmt.Sprintf("%s file is currently empty.", strings.Title(target)), nil
	}
	return fmt.Sprintf("%s content:\n\n%s", strings.Title(target), content), nil
}

func (p *FileMemoryProvider) write(target, content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("content required for write")
	}
	if err := p.fileStore.Write(target, content); err != nil {
		return "", fmt.Errorf("failed to write to %s: %w", target, err)
	}
	return fmt.Sprintf("Successfully wrote content to %s.", strings.Title(target)), nil
}

func (p *FileMemoryProvider) replace(target, oldStr, newStr string) (string, error) {
	if oldStr == "" {
		return "", fmt.Errorf("old_content required for replace")
	}
	if newStr == "" {
		return "", fmt.Errorf("new content required for replace")
	}
	if err := p.fileStore.Replace(target, oldStr, newStr); err != nil {
		return "", fmt.Errorf("failed to replace content in %s: %w", target, err)
	}
	return fmt.Sprintf("Successfully replaced content in %s.", strings.Title(target)), nil
}

func (p *FileMemoryProvider) remove(target, textToRemove string) (string, error) {
	if textToRemove == "" {
		return "", fmt.Errorf("old_content (text to remove) required")
	}
	if err := p.fileStore.Remove(target, textToRemove); err != nil {
		return "", fmt.Errorf("failed to remove content from %s: %w", target, err)
	}
	return fmt.Sprintf("Successfully removed content from %s.", strings.Title(target)), nil
}

func (p *FileMemoryProvider) clear(target string) (string, error) {
	if err := p.fileStore.Clear(target); err != nil {
		return "", fmt.Errorf("failed to clear %s: %w", target, err)
	}
	return fmt.Sprintf("Successfully cleared all content from %s.", strings.Title(target)), nil
}

func (p *FileMemoryProvider) snapshot(target string) (string, error) {
	snapshotContent := p.fileStore.GetSnapshot(target)
	if snapshotContent == "" {
		return fmt.Sprintf("Current %s snapshot is empty.", strings.Title(target)), nil
	}
	return fmt.Sprintf("%s snapshot (freeze state):\n\n%s", strings.Title(target), snapshotContent), nil
}
