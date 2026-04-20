package hands

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// ResolvedSettings represents the result of resolving user-chosen settings against the schema.
type ResolvedSettings struct {
	// PromptBlock is a markdown block to append to the system prompt
	PromptBlock string
	// EnvVars are the env var names the agent's subprocess should have access to
	EnvVars []string
}

// ResolveSettings resolves user config values against a hand's settings schema.
func ResolveSettings(settings []HandSetting, config map[string]interface{}) ResolvedSettings {
	var lines []string
	var envVars []string

	for _, setting := range settings {
		// Get the chosen value from config, or fall back to default
		var chosenValue string
		if val, ok := config[setting.Key]; ok {
			// Try to convert to string
			if s, ok := val.(string); ok {
				chosenValue = s
			} else {
				// Fall back to default if type is wrong
				chosenValue = setting.Default
			}
		} else {
			chosenValue = setting.Default
		}

		switch setting.SettingType {
		case HandSettingTypeSelect:
			// Find the matching option
			var matched *HandSettingOption
			for i, opt := range setting.Options {
				if opt.Value == chosenValue {
					matched = &setting.Options[i]
					break
				}
			}

			var display string
			if matched != nil {
				display = matched.Label
			} else {
				display = chosenValue
			}

			lines = append(lines, fmt.Sprintf("- %s: %s (%s)", setting.Label, display, chosenValue))

			if matched != nil && matched.ProviderEnv != "" {
				envVars = append(envVars, matched.ProviderEnv)
			}

		case HandSettingTypeToggle:
			enabled := chosenValue == "true" || chosenValue == "1" || chosenValue == "yes"
			status := "Disabled"
			if enabled {
				status = "Enabled"
			}
			lines = append(lines, fmt.Sprintf("- %s: %s", setting.Label, status))

		case HandSettingTypeText:
			if chosenValue != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", setting.Label, chosenValue))
			}
		}
	}

	var promptBlock string
	if len(lines) > 0 {
		promptBlock = fmt.Sprintf("## User Configuration\n\n%s", joinLines(lines))
	}

	return ResolvedSettings{
		PromptBlock: promptBlock,
		EnvVars:     envVars,
	}
}

// joinLines joins lines with newlines
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

//go:embed bundled_hands/*/*.json
//go:embed bundled_hands/*/SKILL.md
var bundledHandsFS embed.FS

var bundledHands []*HandDefinition

func init() {
	loadBundledHands()
}

func loadBundledHands() {
	handIDs := []string{
		"researcher",
		"lead",
		"collector",
		"predictor",
		"clip",
		"twitter",
		"browser",
	}

	for _, handID := range handIDs {
		path := filepath.Join("bundled_hands", handID, handID+".json")
		data, err := bundledHandsFS.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: Failed to read %s: %v\n", path, err)
			continue
		}

		var hand HandDefinition
		if err := json.Unmarshal(data, &hand); err != nil {
			fmt.Printf("Warning: Failed to parse %s: %v\n", path, err)
			continue
		}

		skillPath := filepath.Join("bundled_hands", handID, "SKILL.md")
		skillData, err := bundledHandsFS.ReadFile(skillPath)
		if err == nil {
			hand.SkillContent = string(skillData)
		}

		if hand.Agent.CronRules != "" {
			if hand.Agent.SystemPrompt != "" {
				hand.Agent.SystemPrompt += "\n\n"
			}
			hand.Agent.SystemPrompt += "## Schedule Management\n" + hand.Agent.CronRules
		}

		bundledHands = append(bundledHands, &hand)
	}
}

// GetBundledHands returns all bundled Hand definitions
func GetBundledHands() []*HandDefinition {
	return bundledHands
}

// GetBundledHand returns a specific bundled Hand by ID
func GetBundledHand(id string) (*HandDefinition, bool) {
	for _, hand := range bundledHands {
		if hand.ID == id {
			return hand, true
		}
	}
	return nil, false
}

// CreateHandInstance creates a new Hand instance from a definition
func CreateHandInstance(def *HandDefinition, config map[string]interface{}) *HandInstance {
	now := time.Now()
	return &HandInstance{
		InstanceID:  "inst_" + def.ID + "_" + now.Format("20060102150405"),
		HandID:      def.ID,
		Status:      HandStatusInactive,
		AgentID:     "agent_" + def.ID,
		AgentName:   def.Agent.Name,
		Config:      config,
		ActivatedAt: now,
		UpdatedAt:   now,
	}
}
