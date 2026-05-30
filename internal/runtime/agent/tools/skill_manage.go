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
	return "Manage skills: create, view, list, delete, patch, list_files, view_file, history, rollback. Use proactively to create and refine skills for repeated tasks."
}

func (t *SkillManageTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "skill_manage",
			"description": "Create, view, list, delete, patch, list_files, view_file, history, rollback skills.\n\nWHEN TO USE (use proactively):\n- When you identify a repeated workflow or pattern\n- When a task requires multiple steps that could be standardized\n- When you create custom instructions for a specific project\n- When you want to capture and reuse successful problem-solving approaches\n\nWHEN TO CREATE A SKILL:\n- The task is performed more than once\n- The steps are consistent and repeatable\n- There's value in standardizing the approach\n\nSKILL CONTENT GUIDELINES:\n- Start with a clear purpose statement\n- Outline step-by-step instructions\n- Include examples when helpful\n- Keep it focused and actionable\n- Document any assumptions or prerequisites\n\nACTIONS:\n- list: List all available skills (grouped by category)\n- view: View a specific skill's content\n- create: Create a new skill\n- delete: Delete an existing skill\n- patch: Modify a skill using a structured edit operation\n- list_files: List all files in a skill directory (including references/, templates/)\n- view_file: View a specific file from a skill directory\n- history: View version history of a skill (for rollback)\n- rollback: Roll back a skill to a previous version\n\nEDIT OPERATIONS (for patch action):\n- append: Add new content at the end of the skill document\n- insert_after: Insert new content after a specific target text (use 'target' param)\n- replace: Replace exact target text with new content (use 'target' param)\n- delete: Remove exact target text from the skill (use 'target' param, 'content' not needed)\n\nMUTATION STRATEGIES (for patch action, describes WHY you are editing):\n- add_example: Add a concrete example to illustrate instructions\n- add_constraint: Add a rule or constraint to prevent errors\n- restructure: Reorganize the structure or flow of instructions\n- add_edge_case: Add handling for a boundary or edge case\n- refine: Improve wording, clarity, or specificity of existing content",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform: create, view, list, delete, patch, list_files, view_file, history, rollback",
						"enum":        []string{"create", "view", "list", "delete", "patch", "list_files", "view_file", "history", "rollback"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Skill name (required for create, view, delete, patch, list_files, view_file, history, rollback)",
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
						"description": "Content for the edit operation. For create: skill body. For patch: new content to add/insert/replace. Not needed for delete.",
					},
					"op": map[string]interface{}{
						"type":        "string",
						"description": "Edit operation for patch action: append (add at end), insert_after (insert after target), replace (replace target with content), delete (remove target)",
						"enum":        []string{"append", "insert_after", "replace", "delete"},
					},
					"target": map[string]interface{}{
						"type":        "string",
						"description": "Target text in the skill document (required for insert_after, replace, delete operations). Must be exact text that exists in the document.",
					},
					"strategy": map[string]interface{}{
						"type":        "string",
						"description": "Mutation strategy for patch action: add_example, add_constraint, restructure, add_edge_case, refine",
						"enum":        []string{"add_example", "add_constraint", "restructure", "add_edge_case", "refine"},
					},
					"version": map[string]interface{}{
						"type":        "number",
						"description": "Version number to roll back to (required for rollback action)",
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
		op, _ := args["op"].(string)
		target, _ := args["target"].(string)
		content, _ := args["content"].(string)
		strategy, _ := args["strategy"].(string)
		return t.patchSkill(name, op, target, content, strategy)
	case "list_files":
		name, _ := args["name"].(string)
		return t.listSkillFiles(name)
	case "view_file":
		name, _ := args["name"].(string)
		filePath, _ := args["file_path"].(string)
		return t.viewSkillFile(name, filePath)
	case "history":
		name, _ := args["name"].(string)
		return t.historySkill(name)
	case "rollback":
		name, _ := args["name"].(string)
		version := 0
		if v, ok := args["version"].(float64); ok {
			version = int(v)
		}
		return t.rollbackSkill(name, version)
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

func (t *SkillManageTool) patchSkill(name, op, target, content, strategy string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}
	if op == "" {
		return "", fmt.Errorf("edit operation (op) required for patch: append, insert_after, replace, or delete")
	}

	editOp := skills.EditOp(op)
	switch editOp {
	case skills.EditOpAppend, skills.EditOpInsertAfter, skills.EditOpReplace, skills.EditOpDelete:
	default:
		return "", fmt.Errorf("invalid edit operation '%s': must be append, insert_after, replace, or delete", op)
	}

	if editOp != skills.EditOpDelete && content == "" {
		return "", fmt.Errorf("content is required for %s operation (only delete can omit content)", op)
	}
	if (editOp == skills.EditOpInsertAfter || editOp == skills.EditOpReplace || editOp == skills.EditOpDelete) && target == "" {
		return "", fmt.Errorf("target is required for %s operation (specify the exact text in the skill document)", op)
	}

	edit := skills.Edit{
		Op:      editOp,
		Content: content,
		Target:  target,
	}

	description := ""
	if strategy != "" {
		description = fmt.Sprintf("Strategy: %s", strategy)
	}

	skill, err := t.loader.UpdateSkillWithEdit(name, edit, strategy, description)
	if err != nil {
		return "", fmt.Errorf("failed to patch skill: %w", err)
	}

	validationResult := validateSkillContent(skill.Manifest.PromptContext, strategy)

	if !validationResult.Valid {
		versions, histErr := t.loader.ListSkillVersions(name)
		if histErr == nil && len(versions) > 0 {
			latestVersion := versions[len(versions)-1].Version
			_, rollbackErr := t.loader.RollbackSkill(name, latestVersion)
			if rollbackErr != nil {
				return "", fmt.Errorf("patch validation failed (%s), and auto-rollback also failed: %v", validationResult.Reason, rollbackErr)
			}
			return fmt.Sprintf("Patch validation failed: %s. Auto-rolled back to v%d.", validationResult.Reason, latestVersion), nil
		}
		return "", fmt.Errorf("patch validation failed: %s (no version history available for rollback)", validationResult.Reason)
	}

	result := fmt.Sprintf("Skill '%s' patched successfully (op=%s)", skill.Manifest.Name, op)
	if target != "" {
		displayTarget := target
		if len(displayTarget) > 60 {
			displayTarget = displayTarget[:60] + "..."
		}
		result += fmt.Sprintf(" target=\"%s\"", displayTarget)
	}
	if strategy != "" {
		result += fmt.Sprintf(" strategy=%s", strategy)
	}
	if len(validationResult.Warnings) > 0 {
		result += fmt.Sprintf("\nWarnings: %s", strings.Join(validationResult.Warnings, "; "))
	}
	return result, nil
}

func (t *SkillManageTool) historySkill(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}

	versions, err := t.loader.ListSkillVersions(name)
	if err != nil {
		return "", fmt.Errorf("failed to get skill history: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Sprintf("No version history found for skill '%s'.", name), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Version history for skill '%s':\n\n", name))
	for _, v := range versions {
		strategyInfo := ""
		if v.Strategy != "" {
			strategyInfo = fmt.Sprintf(" [%s]", v.Strategy)
		}
		descInfo := ""
		if v.Description != "" {
			descInfo = fmt.Sprintf(" - %s", v.Description)
		}
		result.WriteString(fmt.Sprintf("  v%d%s (%s)%s\n", v.Version, strategyInfo, v.Timestamp.Format("2006-01-02 15:04:05"), descInfo))
	}
	result.WriteString(fmt.Sprintf("\nUse rollback action with version number to restore a previous version."))

	return result.String(), nil
}

func (t *SkillManageTool) rollbackSkill(name string, version int) (string, error) {
	if name == "" {
		return "", fmt.Errorf("skill name required")
	}
	if version <= 0 {
		return "", fmt.Errorf("version number required for rollback (use history action to see available versions)")
	}

	skill, err := t.loader.RollbackSkill(name, version)
	if err != nil {
		return "", fmt.Errorf("failed to rollback skill: %w", err)
	}

	return fmt.Sprintf("Skill '%s' rolled back to v%d successfully. Current version: %s", skill.Manifest.Name, version, skill.Manifest.Version), nil
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

type validationResult struct {
	Valid    bool
	Reason   string
	Warnings []string
}

func validateSkillContent(content, strategy string) validationResult {
	result := validationResult{Valid: true}

	if strings.TrimSpace(content) == "" {
		result.Valid = false
		result.Reason = "SKILL.md content is empty after patch"
		return result
	}

	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		result.Valid = false
		result.Reason = "SKILL.md is missing YAML frontmatter"
		return result
	}
	trimmed := strings.TrimSpace(content)
	afterFirst := trimmed[3:]
	closingIdx := strings.Index(afterFirst, "\n---")
	if closingIdx < 0 {
		result.Valid = false
		result.Reason = "SKILL.md frontmatter is not properly closed"
		return result
	}

	bodyStart := closingIdx + 4
	body := strings.TrimSpace(trimmed[bodyStart:])
	if body == "" {
		result.Valid = false
		result.Reason = "SKILL.md body is empty after frontmatter"
		return result
	}

	if len(body) < 20 {
		result.Valid = false
		result.Reason = fmt.Sprintf("SKILL.md body is too short (%d chars, minimum 20)", len(body))
		return result
	}

	switch strategy {
	case "add_example":
		exampleKeywords := []string{"example", "Example", "EXAMPLE", "示例", "e.g.", "E.g.", "for instance", "For instance"}
		if !containsAny(body, exampleKeywords) {
			result.Warnings = append(result.Warnings, "strategy is 'add_example' but no example-related keywords found (e.g., 'Example', '示例', 'e.g.')")
		}

	case "add_constraint":
		constraintKeywords := []string{"Note", "note", "Important", "important", "WARNING", "Warning", "注意", "必须", "must", "MUST", "never", "NEVER", "always", "ALWAYS", "do not", "Do not"}
		if !containsAny(body, constraintKeywords) {
			result.Warnings = append(result.Warnings, "strategy is 'add_constraint' but no constraint-related keywords found (e.g., 'Note', 'Important', '注意', 'must')")
		}

	case "restructure":
		headingCount := strings.Count(body, "\n## ")
		if headingCount < 2 {
			result.Warnings = append(result.Warnings, "strategy is 'restructure' but body has fewer than 2 subsection headings (##)")
		}

	case "add_edge_case":
		edgeKeywords := []string{"edge case", "Edge case", "boundary", "Boundary", "边界", "特殊情况", "corner case", "Corner case", "exception", "Exception", "异常", "when ...", "if ..."}
		if !containsAny(body, edgeKeywords) {
			result.Warnings = append(result.Warnings, "strategy is 'add_edge_case' but no edge-case-related keywords found (e.g., 'edge case', '边界', 'exception')")
		}

	case "refine":
		if len(body) < 50 {
			result.Warnings = append(result.Warnings, "strategy is 'refine' but body is very short, consider adding more detail")
		}
	}

	return result
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
