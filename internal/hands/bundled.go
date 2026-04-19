package hands

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

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
