package wizard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type AgentIntent struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Task         string   `json:"task"`
	Skills       []string `json:"skills"`
	ModelTier    string   `json:"model_tier"`
	Scheduled    bool     `json:"scheduled"`
	Schedule     *string  `json:"schedule"`
	Capabilities []string `json:"capabilities"`
}

type SetupPlan struct {
	Intent          AgentIntent         `json:"intent"`
	Manifest        types.AgentManifest `json:"manifest"`
	SkillsToInstall []string            `json:"skills_to_install"`
	Summary         string              `json:"summary"`
}

type SetupWizard struct{}

func NewSetupWizard() *SetupWizard {
	return &SetupWizard{}
}

func (sw *SetupWizard) BuildPlan(intent AgentIntent) SetupPlan {
	provider, model := sw.getModelForTier(intent.ModelTier)
	caps, tools := sw.buildCapabilities(intent.Capabilities)
	systemPrompt := sw.generateSystemPrompt(intent, tools)

	manifest := types.AgentManifest{
		Name:         intent.Name,
		Description:  intent.Description,
		SystemPrompt: systemPrompt,
		Model: types.ModelConfig{
			Provider: provider,
			Model:    model,
		},
		Capabilities: caps,
		Skills:       intent.Skills,
		Tools:        tools,
		McpServers:   []string{},
		Metadata:     map[string]string{},
	}

	summary := sw.generateSummary(intent, manifest)

	return SetupPlan{
		Intent:          intent,
		Manifest:        manifest,
		SkillsToInstall: intent.Skills,
		Summary:         summary,
	}
}

func (sw *SetupWizard) getModelForTier(tier string) (string, string) {
	switch strings.ToLower(tier) {
	case "simple":
		return "openai", "gpt-4o-mini"
	case "complex":
		return "anthropic", "claude-3-5-sonnet-20241022"
	default:
		return "openai", "gpt-4o"
	}
}

func (sw *SetupWizard) buildCapabilities(capabilities []string) (*types.ManifestCaps, []string) {
	caps := &types.ManifestCaps{}
	var tools []string

	for _, cap := range capabilities {
		switch strings.ToLower(cap) {
		case "web", "network":
			caps.Network = append(caps.Network, "*")
			tools = sw.addToolIfMissing(tools, "web_search")
			tools = sw.addToolIfMissing(tools, "fetch")

		case "browser":
			caps.Network = append(caps.Network, "*")

		case "file", "files":
			tools = sw.addToolIfMissing(tools, "read_file")
			tools = sw.addToolIfMissing(tools, "write_file")
			tools = sw.addToolIfMissing(tools, "list_dir")

		case "shell":
			caps.Shell = append(caps.Shell, "*")
			tools = sw.addToolIfMissing(tools, "shell_exec")

		case "memory":
			caps.MemoryRead = append(caps.MemoryRead, "*")
			caps.MemoryWrite = append(caps.MemoryWrite, "*")

		case "agent":
			caps.AgentSpawn = true
			caps.AgentMessage = []string{"*"}

		case "schedule":
			caps.Schedule = true

		case "mcp":
			caps.McpServers = []string{"*"}

		case "code", "coding":
			tools = sw.addToolIfMissing(tools, "read_file")
			tools = sw.addToolIfMissing(tools, "write_file")
			tools = sw.addToolIfMissing(tools, "shell_exec")
			caps.Shell = append(caps.Shell, "*")

		case "calculator":
			tools = sw.addToolIfMissing(tools, "calculator")

		case "datetime":
			tools = sw.addToolIfMissing(tools, "datetime")

		case "weather":
			tools = sw.addToolIfMissing(tools, "weather")

		case "search":
			tools = sw.addToolIfMissing(tools, "search")
			caps.Network = append(caps.Network, "*")

		case "json":
			tools = sw.addToolIfMissing(tools, "json")

		case "hash":
			tools = sw.addToolIfMissing(tools, "hash")

		default:
			tools = sw.addToolIfMissing(tools, cap)
		}
	}

	return caps, tools
}

func (sw *SetupWizard) addToolIfMissing(tools []string, tool string) []string {
	for _, t := range tools {
		if t == tool {
			return tools
		}
	}
	return append(tools, tool)
}

func (sw *SetupWizard) generateSystemPrompt(intent AgentIntent, tools []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are %s, an AI agent running inside the FangClaw Agent OS.\n", intent.Name))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("YOUR TASK: %s\n", intent.Task))
	sb.WriteString("\n")
	sb.WriteString("APPROACH:\n")
	sb.WriteString("- Understand the request fully before acting.\n")
	sb.WriteString("- Use your tools to accomplish the task rather than just describing what to do.\n")
	sb.WriteString("- If you need information, search for it. If you need to read a file, read it.\n")
	sb.WriteString("- Be concise in your responses. Lead with results, not process narration.\n")

	toolHints := sw.toolHintsFor(tools)
	if toolHints != "" {
		sb.WriteString("\nKEY TOOLS:\n")
		sb.WriteString(toolHints)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (sw *SetupWizard) toolHintsFor(tools []string) string {
	var hints []string

	has := func(name string) bool {
		for _, t := range tools {
			if t == name {
				return true
			}
		}
		return false
	}

	if has("web_search") || has("search") {
		hints = append(hints, "- Use web_search to find current information on any topic.")
	}
	if has("fetch") {
		hints = append(hints, "- Use fetch to read the full content of a specific URL.")
	}
	if has("read_file") {
		hints = append(hints, "- Use read_file to examine files before modifying them.")
	}
	if has("shell_exec") {
		hints = append(hints, "- Use shell_exec to run commands. Explain destructive commands before running.")
	}
	if has("calculator") {
		hints = append(hints, "- Use calculator for mathematical calculations.")
	}
	if has("datetime") {
		hints = append(hints, "- Use datetime to get current date and time information.")
	}
	if has("weather") {
		hints = append(hints, "- Use weather to get weather information for any location.")
	}

	return strings.Join(hints, "\n")
}

func (sw *SetupWizard) generateSummary(intent AgentIntent, manifest types.AgentManifest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Agent: %s\n", manifest.Name))
	if manifest.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", manifest.Description))
	}
	sb.WriteString(fmt.Sprintf("Model: %s/%s\n", manifest.Model.Provider, manifest.Model.Model))

	if len(manifest.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("Tools: %s\n", strings.Join(manifest.Tools, ", ")))
	}

	if len(manifest.Skills) > 0 {
		sb.WriteString(fmt.Sprintf("Skills: %s\n", strings.Join(manifest.Skills, ", ")))
	}

	if manifest.Capabilities != nil {
		if len(manifest.Capabilities.Network) > 0 {
			sb.WriteString(fmt.Sprintf("Network access: %s\n", strings.Join(manifest.Capabilities.Network, ", ")))
		}
		if len(manifest.Capabilities.Shell) > 0 {
			sb.WriteString(fmt.Sprintf("Shell access: %s\n", strings.Join(manifest.Capabilities.Shell, ", ")))
		}
		if manifest.Capabilities.AgentSpawn {
			sb.WriteString("Can spawn child agents: yes\n")
		}
		if manifest.Capabilities.Schedule {
			sb.WriteString("Can schedule tasks: yes\n")
		}
	}

	return sb.String()
}

func (sw *SetupWizard) GenerateJSON(manifest types.AgentManifest) (string, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return string(data), nil
}

func (sw *SetupWizard) ParseIntent(intentJSON string) (AgentIntent, error) {
	var intent AgentIntent
	if err := json.Unmarshal([]byte(intentJSON), &intent); err != nil {
		return intent, fmt.Errorf("failed to parse intent JSON: %w", err)
	}
	return intent, nil
}
