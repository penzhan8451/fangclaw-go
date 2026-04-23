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
	return "Manage skills: create, view, list, delete, patch, list_files, view_file. Use proactively to create and refine skills for repeated tasks."
}

func (t *SkillManageTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "skill_manage",
			"description": "Create, view, list, delete, patch, list_files, view_file skills.\n\nWHEN TO USE (use proactively):\n- When you identify a repeated workflow or pattern\n- When a task requires multiple steps that could be standardized\n- When you create custom instructions for a specific project\n- When you want to capture and reuse successful problem-solving approaches\n\nWHEN TO CREATE A SKILL:\n- The task is performed more than once\n- The steps are consistent and repeatable\n- There's value in standardizing the approach\n\nSKILL CONTENT GUIDELINES:\n- Start with a clear purpose statement\n- Outline step-by-step instructions\n- Include examples when helpful\n- Keep it focused and actionable\n- Document any assumptions or prerequisites\n\nACTIONS:\n- list: List all available skills (grouped by category)\n- view: View a specific skill's content\n- create: Create a new skill\n- delete: Delete an existing skill\n- patch: Modify a skill (entire file or specific section)\n- list_files: List all files in a skill directory (including references/, templates/)\n- view_file: View a specific file from a skill directory",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform: create, view, list, delete, patch, list_files, view_file",
						"enum":        []string{"create", "view", "list", "delete", "patch", "list_files", "view_file"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Skill name (required for create, view, delete, patch, list_files, view_file)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Skill description (required for create)",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Skill category (optional for create)",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Skill content (SKILL.md body, required for create, or new content for patch)",
					},
					"old_content": map[string]interface{}{
						"type":        "string",
						"description": "Old content to replace (for patch action. If empty, replaces entire file.",
					},
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "File path to view (required for view_file action, e.g., references/api.md)",
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
	case "list_files":
		name, _ := args["name"].(string)
		return t.listSkillFiles(name)
	case "view_file":
		name, _ := args["name"].(string)
		filePath, _ := args["file_path"].(string)
		return t.viewSkillFile(name, filePath)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *SkillManageTool) listSkills() (string, error) {
	grouped, err := t.loader.ListSkillsGrouped()
	if err != nil {
		return "", fmt.Errorf("failed to list skills: %w", err)
	}

	if len(grouped) == 0 {
		return "No skills found.", nil
	}

	var result strings.Builder
	for category, skillsList := range grouped {
		result.WriteString(fmt.Sprintf("## %s\n", category))
		for _, skill := range skillsList {
			result.WriteString(fmt.Sprintf("- %s: %s (v%s)\n",
				skill.Manifest.Name, skill.Manifest.Description, skill.Manifest.Version))
		}
		result.WriteString("\n")
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

	return fmt.Sprintf("Skill: %s\nVersion: %s\nDescription: %s\nCategory: %s\n\nPrompt Context:\n%s",
		skill.Manifest.Name, skill.Manifest.Version, skill.Manifest.Description, skill.Manifest.Category, skill.Manifest.PromptContext), nil
}

func (t *SkillManageTool) createSkill(name, description, promptContext string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}
	if description == "" {
		return "", fmt.Errorf("skill description required")
	}
	if promptContext == "" {
		return "", fmt.Errorf("skill content required")
	}

	skill, err := t.loader.CreateAgentSkill(name, description, promptContext)
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

func (t *SkillManageTool) listSkillFiles(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}

	files, err := t.loader.ListSkillFiles(name)
	if err != nil {
		return "", fmt.Errorf("failed to list skill files: %w", err)
	}

	if len(files) == 0 {
		return "No files found in skill directory.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Files in skill '%s':\n\n", name))
	for _, file := range files {
		result.WriteString(fmt.Sprintf("- %s\n", file))
	}
	return result.String(), nil
}

func (t *SkillManageTool) viewSkillFile(name, filePath string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}
	if filePath == "" {
		return "", fmt.Errorf("file path required")
	}

	content, err := t.loader.ViewSkillFile(name, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to view file: %w", err)
	}

	return content, nil
}
