package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/memory"
)

type MemoryManageTool struct {
	fileStore *memory.FileStore
}

func NewMemoryManageTool(fileStore *memory.FileStore) *MemoryManageTool {
	return &MemoryManageTool{fileStore: fileStore}
}

func (t *MemoryManageTool) Name() string {
	return "memory_manage"
}

func (t *MemoryManageTool) Description() string {
	return "Manage memory: read, write, replace, remove, clear, and snapshot memory files (MEMORY.md and USER.md). Use proactively to remember user preferences, environment details, and lessons learned."
}

func (t *MemoryManageTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "memory_manage",
			"description": "Read, write, replace, remove, clear, and snapshot memory files (MEMORY.md and USER.md).\n\nWHEN TO SAVE (do this proactively, don't wait to be asked):\n- User corrects you or says \"remember this\" / \"don't do that again\"\n- User shares a preference, habit, or personal detail (name, role, timezone, coding style)\n- You discover something about the environment (OS, installed tools, project structure)\n- You learn a convention, API quirk, or workflow specific to this user's setup\n- You identify a stable fact that will be useful again in future sessions\n\nPRIORITY: User preferences and corrections > environment facts > procedural knowledge. The most valuable memory prevents the user from having to repeat themselves.\n\nDo NOT save task progress, session outcomes, completed-work logs, or temporary TODO state to memory.\n\nTWO TARGETS:\n- 'user': who the user is -- name, role, preferences, communication style, pet peeves\n- 'memory': your notes -- environment facts, project conventions, tool quirks, lessons learned\n\nACTIONS: read, write, replace, remove, clear, snapshot.\n\nSKIP: trivial/obvious info, things easily re-discovered, raw data dumps, and temporary task state.",
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

func (t *MemoryManageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action required")
	}

	target, _ := args["target"].(string)
	if target == "" {
		return "", fmt.Errorf("target required (must be 'memory' or 'user')")
	}

	switch action {
	case "read":
		return t.read(target)
	case "write":
		content, _ := args["content"].(string)
		return t.write(target, content)
	case "replace":
		oldContent, _ := args["old_content"].(string)
		newContent, _ := args["content"].(string)
		return t.replace(target, oldContent, newContent)
	case "remove":
		textToRemove, _ := args["old_content"].(string)
		return t.remove(target, textToRemove)
	case "clear":
		return t.clear(target)
	case "snapshot":
		return t.snapshot(target)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *MemoryManageTool) read(target string) (string, error) {
	content, err := t.fileStore.Read(target)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", target, err)
	}

	if content == "" {
		return fmt.Sprintf("%s file is currently empty.", strings.Title(target)), nil
	}

	return fmt.Sprintf("%s content:\n\n%s", strings.Title(target), content), nil
}

func (t *MemoryManageTool) write(target, content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("content required for write")
	}

	if err := t.fileStore.Write(target, content); err != nil {
		return "", fmt.Errorf("failed to write to %s: %w", target, err)
	}

	return fmt.Sprintf("Successfully wrote content to %s.", strings.Title(target)), nil
}

func (t *MemoryManageTool) replace(target, oldStr, newStr string) (string, error) {
	if oldStr == "" {
		return "", fmt.Errorf("old_content required for replace")
	}
	if newStr == "" {
		return "", fmt.Errorf("new content required for replace")
	}

	if err := t.fileStore.Replace(target, oldStr, newStr); err != nil {
		return "", fmt.Errorf("failed to replace content in %s: %w", target, err)
	}

	return fmt.Sprintf("Successfully replaced content in %s.", strings.Title(target)), nil
}

func (t *MemoryManageTool) remove(target, textToRemove string) (string, error) {
	if textToRemove == "" {
		return "", fmt.Errorf("old_content (text to remove) required")
	}

	if err := t.fileStore.Remove(target, textToRemove); err != nil {
		return "", fmt.Errorf("failed to remove content from %s: %w", target, err)
	}

	return fmt.Sprintf("Successfully removed content from %s.", strings.Title(target)), nil
}

func (t *MemoryManageTool) clear(target string) (string, error) {
	if err := t.fileStore.Clear(target); err != nil {
		return "", fmt.Errorf("failed to clear %s: %w", target, err)
	}

	return fmt.Sprintf("Successfully cleared all content from %s.", strings.Title(target)), nil
}

func (t *MemoryManageTool) snapshot(target string) (string, error) {
	snapshotContent := t.fileStore.GetSnapshot(target)
	if snapshotContent == "" {
		return fmt.Sprintf("Current %s snapshot is empty.", strings.Title(target)), nil
	}

	return fmt.Sprintf("%s snapshot (freeze state):\n\n%s", strings.Title(target), snapshotContent), nil
}
