package types

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type AgentTemplate struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Icon         string   `json:"icon,omitempty"`
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	Profile      string   `json:"profile"`
	SystemPrompt string   `json:"system_prompt"`
	Tools        []string `json:"tools"`
	Skills       []string `json:"skills"`
	McpServers   []string `json:"mcp_servers"`
}

func (t *AgentTemplate) ToAgentManifest() AgentManifest {
	return AgentManifest{
		Name:         t.Name,
		Description:  t.Description,
		SystemPrompt: t.SystemPrompt,
		Model: ModelConfig{
			Provider: t.Provider,
			Model:    t.Model,
		},
		Tools:      t.Tools,
		Skills:     t.Skills,
		McpServers: t.McpServers,
	}
}

//go:embed agent_templates/*.json
var templatesFS embed.FS

var defaultTemplates []AgentTemplate

func init() {
	loadDefaultTemplates()
}

func loadDefaultTemplates() {
	templateIDs := []string{
		"assistant",
		"coder",
		"researcher",
		"writer",
		"data-analyst",
		"devops",
		"support",
		"tutor",
		"api-designer",
		"meeting-notes",
		"code-with-opencode",
	}

	for _, id := range templateIDs {
		path := filepath.Join("agent_templates", id+".json")
		data, err := templatesFS.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: Failed to read %s: %v\n", path, err)
			continue
		}

		var tpl AgentTemplate
		if err := json.Unmarshal(data, &tpl); err != nil {
			fmt.Printf("Warning: Failed to parse %s: %v\n", path, err)
			continue
		}

		defaultTemplates = append(defaultTemplates, tpl)
	}
}

func GetDefaultAgentTemplates() []AgentTemplate {
	result := make([]AgentTemplate, len(defaultTemplates))
	copy(result, defaultTemplates)
	return result
}
