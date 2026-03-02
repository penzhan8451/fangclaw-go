// Package skills provides skill management for agents.
package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a reusable skill for agents.
type Skill struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Category    string            `json:"category"`
	Prompts    []string         `json:"prompts"`
	Tools      []string         `json:"tools"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Content    string           `json:"-"`
	Path       string           `json:"-"`
}

// SkillManifest represents a skill's manifest file.
type SkillManifest struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                `json:"version"`
	Category    string                `json:"category"`
	Author      string                `json:"author,omitempty"`
	Repository  string                `json:"repository,omitempty"`
	Keywords    []string             `json:"keywords,omitempty"`
	Prompts     []PromptConfig       `json:"prompts"`
	Tools       []string             `json:"tools"`
	Dependencies []Dependency         `json:"dependencies,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PromptConfig defines a prompt template.
type PromptConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
	System      bool   `json:"system,omitempty"`
}

// Dependency represents a skill dependency.
type Dependency struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

// SkillRegistry manages available skills.
type SkillRegistry struct {
	skills    map[string]*Skill
	installed map[string]string // skillID -> path
}

// NewSkillRegistry creates a new skill registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills:    make(map[string]*Skill),
		installed: make(map[string]string),
	}
}

// LoadSkill loads a skill from a directory.
func (r *SkillRegistry) LoadSkill(path string) (*Skill, error) {
	// Check for SKILL.md first
	skillMDPath := filepath.Join(path, "SKILL.md")
	if data, err := os.ReadFile(skillMDPath); err == nil {
		skill := &Skill{
			ID:       filepath.Base(path),
			Name:     filepath.Base(path),
			Path:     path,
			Content:  string(data),
			Metadata: make(map[string]interface{}),
		}
		r.skills[skill.ID] = skill
		return skill, nil
	}

	// Check for skill.json
	manifestPath := filepath.Join(path, "skill.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest SkillManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse skill manifest: %w", err)
		}

		skill := &Skill{
			ID:          manifest.ID,
			Name:        manifest.Name,
			Description: manifest.Description,
			Version:     manifest.Version,
			Category:    manifest.Category,
			Prompts:     extractPromptNames(manifest.Prompts),
			Tools:       manifest.Tools,
			Metadata:    manifest.Metadata,
			Path:        path,
		}

		r.skills[skill.ID] = skill
		return skill, nil
	}

	return nil, fmt.Errorf("no skill manifest found in %s", path)
}

// LoadSkillsFromDir loads all skills from a directory.
func (r *SkillRegistry) LoadSkillsFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name())
		if skill, err := r.LoadSkill(skillPath); err == nil {
			r.installed[skill.ID] = skillPath
		}
	}

	return nil
}

// Get retrieves a skill by ID.
func (r *SkillRegistry) Get(id string) (*Skill, bool) {
	skill, ok := r.skills[id]
	return skill, ok
}

// List returns all available skills.
func (r *SkillRegistry) List() []*Skill {
	skills := make([]*Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// ListByCategory returns skills filtered by category.
func (r *SkillRegistry) ListByCategory(category string) []*Skill {
	var result []*Skill
	for _, skill := range r.skills {
		if strings.EqualFold(skill.Category, category) {
			result = append(result, skill)
		}
	}
	return result
}

// Search searches skills by keyword.
func (r *SkillRegistry) Search(query string) []*Skill {
	query = strings.ToLower(query)
	var result []*Skill

	for _, skill := range r.skills {
		if strings.Contains(strings.ToLower(skill.Name), query) ||
			strings.Contains(strings.ToLower(skill.Description), query) {
			result = append(result, skill)
		}

		for _, kw := range skill.Metadata["keywords"].([]string) {
			if strings.Contains(strings.ToLower(kw), query) {
				result = append(result, skill)
				break
			}
		}
	}

	return result
}

// GetPrompt retrieves a prompt template from a skill.
func (r *SkillRegistry) GetPrompt(skillID, promptName string) (string, error) {
	skill, ok := r.skills[skillID]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", skillID)
	}

	// Check metadata for prompts
	if prompts, ok := skill.Metadata["prompts"].([]interface{}); ok {
		for _, p := range prompts {
			if pMap, ok := p.(map[string]interface{}); ok {
				if name, ok := pMap["name"].(string); ok && name == promptName {
					if template, ok := pMap["template"].(string); ok {
						return template, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("prompt not found: %s", promptName)
}

// Register adds a skill to the registry.
func (r *SkillRegistry) Register(skill *Skill) {
	r.skills[skill.ID] = skill
}

// Unregister removes a skill from the registry.
func (r *SkillRegistry) Unregister(id string) {
	delete(r.skills, id)
	delete(r.installed, id)
}

// Count returns the number of registered skills.
func (r *SkillRegistry) Count() int {
	return len(r.skills)
}

// ============= Skill Helpers =============

func extractPromptNames(prompts []PromptConfig) []string {
	names := make([]string, len(prompts))
	for i, p := range prompts {
		names[i] = p.Name
	}
	return names
}

// LoadSkillFromFile loads a skill from a file.
func LoadSkillFromFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	var manifest SkillManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
	return nil, fmt.Errorf("failed to parse skill manifest: %w", err)
	}

	return &Skill{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Description: manifest.Description,
		Version:     manifest.Version,
		Category:    manifest.Category,
		Prompts:    extractPromptNames(manifest.Prompts),
		Tools:       manifest.Tools,
		Metadata:    manifest.Metadata,
		Path:        filepath.Dir(path),
	}, nil
}

// SaveSkill saves a skill to a directory.
func SaveSkill(skill *Skill, dir string) error {
	skillDir := filepath.Join(dir, skill.ID)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	manifest := SkillManifest{
		ID:          skill.ID,
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		Category:    skill.Category,
		Prompts:     make([]PromptConfig, len(skill.Prompts)),
		Tools:       skill.Tools,
		Metadata:    skill.Metadata,
	}

	for i, name := range skill.Prompts {
		manifest.Prompts[i] = PromptConfig{
			Name:        name,
			Description: fmt.Sprintf("Prompt template: %s", name),
		}
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal skill manifest: %w", err)
	}

	return os.WriteFile(filepath.Join(skillDir, "skill.json"), data, 0644)
}

// ============= Default Skills =============

// DefaultSkills returns a list of built-in skills.
func DefaultSkills() []*Skill {
	return []*Skill{
		{
			ID:          "web-researcher",
			Name:        "Web Researcher",
			Description: "Research skills for web searches and information gathering",
			Category:    "research",
			Version:     "1.0.0",
			Prompts:     []string{"search", "analyze", "summarize"},
			Tools:       []string{"search", "fetch"},
		},
		{
			ID:          "code-assistant",
			Name:        "Code Assistant",
			Description: "Programming and code analysis skills",
			Category:    "development",
			Version:     "1.0.0",
			Prompts:     []string{"explain", "review", "refactor"},
			Tools:       []string{"read_file", "shell"},
		},
		{
			ID:          "data-analyst",
			Name:        "Data Analyst",
			Description: "Data analysis and visualization skills",
			Category:    "analysis",
			Version:     "1.0.0",
			Prompts:     []string{"analyze", "visualize", "report"},
			Tools:       []string{"calculator", "json", "csv"},
		},
		{
			ID:          "writer",
			Name:        "Writer",
			Description: "Writing and content creation skills",
			Category:    "content",
			Version:     "1.0.0",
			Prompts:     []string{"draft", "edit", "proofread"},
			Tools:       []string{},
		},
		{
			ID:          "translator",
			Name:        "Translator",
			Description: "Translation and localization skills",
			Category:    "language",
			Version:     "1.0.0",
			Prompts:     []string{"translate", "localize"},
		Tools:       []string{},
		},
	}
}

// RegisterDefaultSkills registers built-in skills.
func (r *SkillRegistry) RegisterDefaultSkills() {
	for _, skill := range DefaultSkills() {
		r.Register(skill)
	}
}
