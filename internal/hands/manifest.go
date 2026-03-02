// Package hands provides autonomous capability packages (Hands) for OpenFang.
package hands

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HandManifest represents the HAND.toml manifest file.
type HandManifest struct {
	ID          string            `toml:"id"`
	Name        string            `toml:"name"`
	Description string            `toml:"description"`
	Category    string            `toml:"category"`
	Icon        string            `toml:"icon,omitempty"`
	Version     string            `toml:"version,omitempty"`
	Author      string            `toml:"author,omitempty"`
	License     string            `toml:"license,omitempty"`
	Schedule    string            `toml:"schedule,omitempty"`
	Approval    ApprovalConfig    `toml:"approval,omitempty"`
	Tools       []string          `toml:"tools"`
	Skills      []string          `toml:"skills,omitempty"`
	MCPServers  []string          `toml:"mcp_servers,omitempty"`
	Settings    []HandSetting     `toml:"settings,omitempty"`
	Requires    []HandRequirement `toml:"requires,omitempty"`
	Agent       HandAgentConfig   `toml:"agent"`
	Dashboard   HandDashboard     `toml:"dashboard,omitempty"`
}

// ApprovalConfig represents approval gate configuration.
type ApprovalConfig struct {
	Required  bool     `toml:"required"`
	Reviewers []string `toml:"reviewers,omitempty"`
	Rules     []string `toml:"rules,omitempty"`
}

// LoadHandManifest loads a Hand manifest from a file.
func LoadHandManifest(path string) (*HandManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest HandManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// LoadHandManifestFromDir loads a Hand manifest from a directory.
func LoadHandManifestFromDir(dir string) (*HandManifest, error) {
	return LoadHandManifest(filepath.Join(dir, "HAND.toml"))
}

// LoadSkill loads the SKILL.md file from a directory.
func LoadSkill(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ManifestToHandDefinition converts a HandManifest to a HandDefinition.
func ManifestToHandDefinition(manifest *HandManifest) *HandDefinition {
	category := HandCategory(manifest.Category)
	if category == "" {
		category = HandCategoryProductivity
	}

	return &HandDefinition{
		ID:           manifest.ID,
		Name:         manifest.Name,
		Description:  manifest.Description,
		Category:     category,
		Icon:         manifest.Icon,
		Tools:        manifest.Tools,
		Skills:       manifest.Skills,
		MCPServers:   manifest.MCPServers,
		Requires:     manifest.Requires,
		Settings:     manifest.Settings,
		Agent:        manifest.Agent,
		Dashboard:    manifest.Dashboard,
		SkillContent: "",
	}
}

// ManifestToHand converts a HandManifest to a Hand.
func ManifestToHand(manifest *HandManifest) *Hand {
	category := HandCategory(manifest.Category)
	if category == "" {
		category = HandCategoryProductivity
	}

	return &Hand{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Description: manifest.Description,
		Category:    category,
		Icon:        manifest.Icon,
		State:       HandStateIdle,
		Schedule:    manifest.Schedule,
		Config: HandConfig{
			Tools:        manifest.Tools,
			Skills:       manifest.Skills,
			MCPServers:   manifest.MCPServers,
			Settings:     make(map[string]string),
			Requirements: manifest.Requires,
		},
		Metrics: HandMetrics{},
	}
}
