package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/skills"
)

type SkillManageTool struct {
	loader *skills.Loader
}

func NewSkillManageTool(loader *skills.Loader) *SkillManageTool {
	return &SkillManageTool{loader: loader}
}

func (t *SkillManageTool) Name() string {
	return "skill_manage"
}

func (t *SkillManageTool) Description() string {
	return "Manage skills: create, view, list, delete, and patch skills"
}

func (t *SkillManageTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "skill_manage",
			"description": "Create, view, list, delete, and patch skills",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform: create, view, list, delete, patch",
						"enum":        []string{"create", "view", "list", "delete", "patch"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Skill name (required for create, view, delete, patch)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Skill description (required for create)",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Skill content (SKILL.md body, required for create, or new content for patch)",
					},
					"old_content": map[string]interface{}{
						"type":        "string",
						"description": "Old content to replace (for patch action). If empty, replaces entire file.",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

func (t *SkillManageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action required")
	}

	switch action {
	case "list":
		return t.listSkills()
	case "view":
		name, _ := args["name"].(string)
		return t.viewSkill(name)
	case "create":
		name, _ := args["name"].(string)
		desc, _ := args["description"].(string)
		content, _ := args["content"].(string)
		return t.createSkill(name, desc, content)
	case "delete":
		name, _ := args["name"].(string)
		return t.deleteSkill(name)
	case "patch":
		name, _ := args["name"].(string)
		oldContent, _ := args["old_content"].(string)
		content, _ := args["content"].(string)
		return t.patchSkill(name, oldContent, content)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *SkillManageTool) listSkills() (string, error) {
	skillsList, err := t.loader.ListSkills()
	if err != nil {
		return "", fmt.Errorf("failed to list skills: %w", err)
	}

	if len(skillsList) == 0 {
		return "No skills found.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d skills:\n\n", len(skillsList)))
	for _, s := range skillsList {
		result.WriteString(fmt.Sprintf("- %s: %s (v%s)\n",
			s.Manifest.Name, s.Manifest.Description, s.Manifest.Version))
	}
	return result.String(), nil
}

func (t *SkillManageTool) viewSkill(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}

	skill, err := t.loader.LoadSkill(name)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	return fmt.Sprintf("Skill: %s\nVersion: %s\nDescription: %s\n\nPrompt Context:\n%s",
		skill.Manifest.Name, skill.Manifest.Version, skill.Manifest.Description, skill.Manifest.PromptContext), nil
}

func (t *SkillManageTool) createSkill(name, description, content string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}
	if description == "" {
		return "", fmt.Errorf("skill description required")
	}
	if content == "" {
		return "", fmt.Errorf("skill content required")
	}

	skill, err := t.loader.CreateAgentSkill(name, description, content)
	if err != nil {
		return "", fmt.Errorf("failed to create skill: %w", err)
	}

	return fmt.Sprintf("Skill '%s' created successfully at %s", skill.Manifest.Name, skill.InstallPath), nil
}

func (t *SkillManageTool) deleteSkill(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}

	err := t.loader.UninstallSkill(name)
	if err != nil {
		return "", fmt.Errorf("failed to delete skill: %w", err)
	}

	return fmt.Sprintf("Skill '%s' deleted successfully", name), nil
}

func (t *SkillManageTool) patchSkill(name, oldContent, newContent string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}
	if newContent == "" {
		return "", fmt.Errorf("new content required for patch")
	}

	skill, err := t.loader.UpdateSkill(name, oldContent, newContent)
	if err != nil {
		return "", fmt.Errorf("failed to patch skill: %w", err)
	}

	return fmt.Sprintf("Skill '%s' patched successfully at %s", skill.Manifest.Name, skill.InstallPath), nil
}
