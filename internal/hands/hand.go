// Package hands provides autonomous capability packages (Hands) for OpenFang.
package hands

import (
	"time"
)

// HandCategory represents the category of a Hand.
type HandCategory string

const (
	HandCategoryContent       HandCategory = "content"
	HandCategorySecurity      HandCategory = "security"
	HandCategoryProductivity  HandCategory = "productivity"
	HandCategoryDevelopment   HandCategory = "development"
	HandCategoryCommunication HandCategory = "communication"
	HandCategoryData          HandCategory = "data"
)

// HandState represents the state of a Hand.
type HandState string

const (
	HandStateIdle     HandState = "idle"
	HandStateRunning  HandState = "running"
	HandStatePaused   HandState = "paused"
	HandStateError    HandState = "error"
	HandStateComplete HandState = "complete"
)

// HandStatus represents the runtime status of a Hand instance.
type HandStatus string

const (
	HandStatusActive   HandStatus = "active"
	HandStatusPaused   HandStatus = "paused"
	HandStatusInactive HandStatus = "inactive"
)

// RequirementType represents the type of a requirement check.
type RequirementType string

const (
	RequirementTypeBinary RequirementType = "binary"
	RequirementTypeEnvVar RequirementType = "env_var"
	RequirementTypeAPIKey RequirementType = "api_key"
)

// HandSettingType represents the type of a Hand setting control.
type HandSettingType string

const (
	HandSettingTypeSelect HandSettingType = "select"
	HandSettingTypeText   HandSettingType = "text"
	HandSettingTypeToggle HandSettingType = "toggle"
)

// Hand represents an autonomous capability package.
type Hand struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    HandCategory `json:"category"`
	Icon        string       `json:"icon"`
	State       HandState    `json:"state"`
	LastRun     *time.Time   `json:"last_run,omitempty"`
	NextRun     *time.Time   `json:"next_run,omitempty"`
	Schedule    string       `json:"schedule,omitempty"`
	Config      HandConfig   `json:"config"`
	Metrics     HandMetrics  `json:"metrics,omitempty"`
}

// HandConfig represents the configuration for a Hand.
type HandConfig struct {
	Tools        []string          `json:"tools"`
	Skills       []string          `json:"skills,omitempty"`
	MCPServers   []string          `json:"mcp_servers,omitempty"`
	Settings     map[string]string `json:"settings"`
	Requirements []HandRequirement `json:"requirements,omitempty"`
}

// HandRequirement represents a single requirement the user must satisfy.
type HandRequirement struct {
	Key             string           `json:"key"`
	Label           string           `json:"label"`
	RequirementType RequirementType  `json:"requirement_type"`
	CheckValue      string           `json:"check_value"`
	Description     string           `json:"description,omitempty"`
	Install         *HandInstallInfo `json:"install,omitempty"`
}

// HandInstallInfo represents platform-specific install commands.
type HandInstallInfo struct {
	MacOS         string   `json:"macos,omitempty"`
	Windows       string   `json:"windows,omitempty"`
	LinuxApt      string   `json:"linux_apt,omitempty"`
	LinuxDnf      string   `json:"linux_dnf,omitempty"`
	LinuxPacman   string   `json:"linux_pacman,omitempty"`
	Pip           string   `json:"pip,omitempty"`
	SignupURL     string   `json:"signup_url,omitempty"`
	DocsURL       string   `json:"docs_url,omitempty"`
	EnvExample    string   `json:"env_example,omitempty"`
	ManualURL     string   `json:"manual_url,omitempty"`
	EstimatedTime string   `json:"estimated_time,omitempty"`
	Steps         []string `json:"steps,omitempty"`
}

// HandSetting represents a configurable setting declared in HAND.toml.
type HandSetting struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	SettingType HandSettingType     `json:"setting_type"`
	Default     string              `json:"default"`
	Options     []HandSettingOption `json:"options,omitempty"`
}

// HandSettingOption represents a single option within a Select-type setting.
type HandSettingOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	ProviderEnv string `json:"provider_env,omitempty"`
	Binary      string `json:"binary,omitempty"`
}

// HandMetric represents a metric displayed on the Hand dashboard.
type HandMetric struct {
	Label     string `json:"label"`
	MemoryKey string `json:"memory_key"`
	Format    string `json:"format"`
}

// HandDashboard represents the dashboard schema for a Hand's metrics.
type HandDashboard struct {
	Metrics []HandMetric `json:"metrics"`
}

// HandMetrics represents the metrics for a Hand.
type HandMetrics struct {
	RunCount     int           `json:"run_count"`
	SuccessCount int           `json:"success_count"`
	LastSuccess  *time.Time    `json:"last_success,omitempty"`
	TotalRuntime time.Duration `json:"total_runtime"`
}

// HandAgentConfig represents the agent configuration embedded in a Hand definition.
type HandAgentConfig struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Module        string  `json:"module"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	APIKeyEnv     string  `json:"api_key_env,omitempty"`
	BaseURL       string  `json:"base_url,omitempty"`
	MaxTokens     int     `json:"max_tokens"`
	Temperature   float32 `json:"temperature"`
	SystemPrompt  string  `json:"system_prompt"`
	CronRules     string  `json:"cron_rules,omitempty"`
	MaxIterations *int    `json:"max_iterations,omitempty"`
}

// HandDefinition represents a complete Hand definition parsed from HAND.toml.
type HandDefinition struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Category     HandCategory      `json:"category"`
	Icon         string            `json:"icon"`
	Tools        []string          `json:"tools"`
	Skills       []string          `json:"skills,omitempty"`
	MCPServers   []string          `json:"mcp_servers,omitempty"`
	Requires     []HandRequirement `json:"requires,omitempty"`
	Settings     []HandSetting     `json:"settings,omitempty"`
	Agent        HandAgentConfig   `json:"agent"`
	Dashboard    HandDashboard     `json:"dashboard,omitempty"`
	SkillContent string            `json:"skill_content,omitempty"`
}

// HandInstance represents a running Hand instance.
type HandInstance struct {
	InstanceID  string                 `json:"instance_id"`
	HandID      string                 `json:"hand_id"`
	Status      HandStatus             `json:"status"`
	AgentID     string                 `json:"agent_id,omitempty"`
	AgentName   string                 `json:"agent_name"`
	Config      map[string]interface{} `json:"config"`
	ActivatedAt time.Time              `json:"activated_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// HandRunner is an interface for running Hands.
type HandRunner interface {
	Run(ctx interface{}) error
	Pause() error
	Resume() error
	Stop() error
}

// HandFactory is a function that creates a HandRunner.
type HandFactory func(hand *Hand) HandRunner
