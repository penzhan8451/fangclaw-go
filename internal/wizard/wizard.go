package wizard

import (
	"fmt"
	"strings"
)

type AgentIntent struct {
	Name         string
	Description  string
	Task         string
	Skills       []string
	ModelTier    string
	Scheduled    bool
	Schedule     *string
	Capabilities []string
}

type SetupPlan struct {
	Intent          AgentIntent
	Manifest        AgentManifest
	SkillsToInstall []string
	Summary         string
}

type AgentManifest struct {
	Name          string
	Description   string
	ModelConfig   ModelConfig
	Priority      string
	ResourceQuota ResourceQuota
	ScheduleMode  string
	Skills        []string
	Capabilities  []string
}

type ModelConfig struct {
	Provider string
	Model    string
}

type ResourceQuota struct {
	MaxIterations uint32
	MaxRestarts   uint32
}

type SetupWizard struct{}

func NewSetupWizard() *SetupWizard {
	return &SetupWizard{}
}

func (sw *SetupWizard) BuildPlan(intent AgentIntent) SetupPlan {
	provider, model := sw.getModelForTier(intent.ModelTier)

	manifest := AgentManifest{
		Name:        intent.Name,
		Description: intent.Description,
		ModelConfig: ModelConfig{
			Provider: provider,
			Model:    model,
		},
		Priority: "normal",
		ResourceQuota: ResourceQuota{
			MaxIterations: 10,
			MaxRestarts:   3,
		},
		ScheduleMode: sw.getScheduleMode(intent),
		Skills:       intent.Skills,
		Capabilities: intent.Capabilities,
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

func (sw *SetupWizard) getScheduleMode(intent AgentIntent) string {
	if !intent.Scheduled {
		return "reactive"
	}
	if intent.Schedule != nil {
		schedule := *intent.Schedule
		if strings.HasPrefix(schedule, "cron:") || strings.Contains(schedule, "*") {
			return "periodic"
		}
		return "continuous"
	}
	return "reactive"
}

func (sw *SetupWizard) generateSummary(intent AgentIntent, manifest AgentManifest) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Creating agent: %s\n", intent.Name))
	summary.WriteString(fmt.Sprintf("Description: %s\n", intent.Description))
	summary.WriteString(fmt.Sprintf("Task: %s\n", intent.Task))
	summary.WriteString(fmt.Sprintf("Model: %s/%s\n", manifest.ModelConfig.Provider, manifest.ModelConfig.Model))

	if len(intent.Skills) > 0 {
		summary.WriteString(fmt.Sprintf("Skills: %s\n", strings.Join(intent.Skills, ", ")))
	}

	if len(intent.Capabilities) > 0 {
		summary.WriteString(fmt.Sprintf("Capabilities: %s\n", strings.Join(intent.Capabilities, ", ")))
	}

	if intent.Scheduled {
		summary.WriteString("Schedule mode: ")
		if intent.Schedule != nil {
			summary.WriteString(*intent.Schedule)
		} else {
			summary.WriteString("enabled")
		}
		summary.WriteString("\n")
	}

	return summary.String()
}

func (sw *SetupWizard) GenerateTOML(manifest AgentManifest) string {
	var toml strings.Builder

	toml.WriteString("[agent]\n")
	toml.WriteString(fmt.Sprintf("name = \"%s\"\n", manifest.Name))
	toml.WriteString(fmt.Sprintf("description = \"%s\"\n", manifest.Description))
	toml.WriteString("\n")

	toml.WriteString("[model]\n")
	toml.WriteString(fmt.Sprintf("provider = \"%s\"\n", manifest.ModelConfig.Provider))
	toml.WriteString(fmt.Sprintf("model = \"%s\"\n", manifest.ModelConfig.Model))
	toml.WriteString("\n")

	toml.WriteString("[priority]\n")
	toml.WriteString(fmt.Sprintf("level = \"%s\"\n", manifest.Priority))
	toml.WriteString("\n")

	toml.WriteString("[quota]\n")
	toml.WriteString(fmt.Sprintf("max_iterations = %d\n", manifest.ResourceQuota.MaxIterations))
	toml.WriteString(fmt.Sprintf("max_restarts = %d\n", manifest.ResourceQuota.MaxRestarts))
	toml.WriteString("\n")

	toml.WriteString("[schedule]\n")
	toml.WriteString(fmt.Sprintf("mode = \"%s\"\n", manifest.ScheduleMode))
	toml.WriteString("\n")

	if len(manifest.Skills) > 0 {
		toml.WriteString("[skills]\n")
		toml.WriteString("enabled = [\n")
		for _, skill := range manifest.Skills {
			toml.WriteString(fmt.Sprintf("  \"%s\",\n", skill))
		}
		toml.WriteString("]\n")
		toml.WriteString("\n")
	}

	if len(manifest.Capabilities) > 0 {
		toml.WriteString("[capabilities]\n")
		toml.WriteString("enabled = [\n")
		for _, capability := range manifest.Capabilities {
			toml.WriteString(fmt.Sprintf("  \"%s\",\n", capability))
		}
		toml.WriteString("]\n")
	}

	return toml.String()
}
