// Package types provides core data structures for OpenFang.
package types

import (
	"time"
)

// CronShellSecurityConfig represents cron shell security configuration.
type CronShellSecurityConfig struct {
	EnableExecuteShell    bool     `toml:"enable_execute_shell" json:"enable_execute_shell"`
	SecurityMode          string   `toml:"security_mode" json:"security_mode"`
	AllowedCommands       []string `toml:"allowed_commands" json:"allowed_commands"`
	AllowedPaths          []string `toml:"allowed_paths" json:"allowed_paths"`
	ForbiddenCommands     []string `toml:"forbidden_commands" json:"forbidden_commands"`
	ForbiddenArgsPatterns []string `toml:"forbidden_args_patterns" json:"forbidden_args_patterns"`
}

// KernelConfig represents the kernel configuration.
type KernelConfig struct {
	DataDir           string                  `toml:"data_dir" json:"data_dir"`
	LogLevel          string                  `toml:"log_level" json:"log_level"`
	API               APIConfig               `toml:"api" json:"api"`
	Models            ModelsConfig            `toml:"models" json:"models"`
	Memory            MemoryConfig            `toml:"memory" json:"memory"`
	Security          SecurityConfig          `toml:"security" json:"security"`
	Skills            SkillsConfig            `toml:"skills" json:"skills"`
	Extensions        ExtensionsConfig        `toml:"extensions" json:"extensions"`
	CronShellSecurity CronShellSecurityConfig `toml:"cron_shell_security" json:"cron_shell_security"`
	McpServers        []McpServerConfig       `toml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
	A2a               A2aConfig               `toml:"a2a" json:"a2a"`
	Browser           BrowserConfig           `toml:"browser" json:"browser"`
	Include           []string                `toml:"include,omitempty" json:"include,omitempty"`
	Auth              AuthConfig              `toml:"auth" json:"auth"`
	Quotas            QuotasConfig            `toml:"quotas" json:"quotas"`
	Budget            BudgetConfig            `toml:"budget" json:"budget"`
	UserID            string                  `toml:"-" json:"user_id,omitempty"`
	Username          string                  `toml:"-" json:"username,omitempty"`
}

type AuthConfig struct {
	Enabled    bool              `toml:"enabled" json:"enabled"`
	DBPath     string            `toml:"db_path" json:"db_path"`
	SessionTTL string            `toml:"session_ttl" json:"session_ttl"`
	GitHub     GitHubOAuthConfig `toml:"github" json:"github"`
}

type GitHubOAuthConfig struct {
	ClientID     string `toml:"client_id" json:"client_id"`
	ClientSecret string `toml:"client_secret" json:"client_secret"`
	Enabled      bool   `toml:"enabled" json:"enabled"`
	RedirectURL  string `toml:"redirect_url" json:"redirect_url"`
}

// internal used by kernel.
type BrowserConfig struct {
	Enabled        bool   `toml:"enabled" json:"enabled"`
	ChromiumPath   string `toml:"chromium_path" json:"chromium_path"`
	Headless       bool   `toml:"headless" json:"headless"`
	ViewportWidth  int    `toml:"viewport_width" json:"viewport_width"`
	ViewportHeight int    `toml:"viewport_height" json:"viewport_height"`
	MaxSessions    int    `toml:"max_sessions" json:"max_sessions"`
}

// APIConfig represents API server configuration.
type APIConfig struct {
	Host         string        `toml:"host" json:"host"`
	Port         int           `toml:"port" json:"port"`
	EnableTLS    bool          `toml:"enable_tls" json:"enable_tls"`
	CertFile     string        `toml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile      string        `toml:"key_file,omitempty" json:"key_file,omitempty"`
	ReadTimeout  time.Duration `toml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout" json:"write_timeout"`
}

// ModelsConfig represents LLM model configuration.
type ModelsConfig struct {
	DefaultProvider string              `toml:"default_provider" json:"default_provider"`
	DefaultModel    string              `toml:"default_model" json:"default_model"`
	Providers       map[string]Provider `toml:"providers" json:"providers"`
	Models          map[string]Model    `toml:"models" json:"models"`
	Routing         ModelRoutingConfig  `toml:"routing" json:"routing"`
}

// ModelRoutingConfig represents model routing configuration.
type ModelRoutingConfig struct {
	SimpleModel      string `toml:"simple_model" json:"simple_model"`
	MediumModel      string `toml:"medium_model" json:"medium_model"`
	ComplexModel     string `toml:"complex_model" json:"complex_model"`
	SimpleThreshold  uint32 `toml:"simple_threshold" json:"simple_threshold"`
	ComplexThreshold uint32 `toml:"complex_threshold" json:"complex_threshold"`
}

// MemoryConfig represents memory configuration.
type MemoryConfig struct {
	Path              string        `toml:"path" json:"path"`
	EnableSemantic    bool          `toml:"enable_semantic" json:"enable_semantic"`
	EmbeddingModel    string        `toml:"embedding_model" json:"embedding_model"`
	MaxMemorySize     int           `toml:"max_memory_size" json:"max_memory_size"`
	RetentionDuration time.Duration `toml:"retention_duration" json:"retention_duration"`
}

// SecurityConfig represents security configuration.
type SecurityConfig struct {
	EnableRBAC      bool   `toml:"enable_rbac" json:"enable_rbac"`
	SecretKey       string `toml:"secret_key" json:"secret_key"`
	EnableAuditLogs bool   `toml:"enable_audit_logs" json:"enable_audit_logs"`
	AuditLogPath    string `toml:"audit_log_path" json:"audit_log_path"`
}

// SkillsConfig represents skills configuration.
type SkillsConfig struct {
	Path               string   `toml:"path" json:"path"`
	EnableRemoteSkills bool     `toml:"enable_remote_skills" json:"enable_remote_skills"`
	ClawHubURL         string   `toml:"clawhub_url" json:"clawhub_url"`
	AllowedRuntimes    []string `toml:"allowed_runtimes" json:"allowed_runtimes"`
}

// A2aConfig represents A2A (Agent-to-Agent) protocol configuration.
type A2aConfig struct {
	Enabled        bool            `toml:"enabled" json:"enabled"`
	ListenPath     string          `toml:"listen_path" json:"listen_path"`
	ExternalAgents []ExternalAgent `toml:"external_agents" json:"external_agents"`
}

// ExternalAgent represents an external A2A agent configuration.
type ExternalAgent struct {
	Name string `toml:"name" json:"name"`
	URL  string `toml:"url" json:"url"`
}

// ExtensionsConfig represents extensions configuration.
type ExtensionsConfig struct {
	Path               string   `toml:"path" json:"path"`
	EnableExtensions   bool     `toml:"enable_extensions" json:"enable_extensions"`
	AutoLoadExtensions []string `toml:"auto_load_extensions" json:"auto_load_extensions"`
}

// AutonomousConfig represents autonomous agent configuration.
type AutonomousConfig struct {
	QuietHours        *string       `toml:"quiet_hours,omitempty" json:"quiet_hours,omitempty"`
	MaxIterations     uint32        `toml:"max_iterations" json:"max_iterations"`
	MaxRestarts       uint32        `toml:"max_restarts" json:"max_restarts"`
	HeartbeatInterval time.Duration `toml:"heartbeat_interval" json:"heartbeat_interval"`
	HeartbeatChannel  *string       `toml:"heartbeat_channel,omitempty" json:"heartbeat_channel,omitempty"`
}

// QuotasConfig represents resource quotas configuration.
type QuotasConfig struct {
	Default ResourceQuota            `toml:"default" json:"default"`
	Agents  map[string]ResourceQuota `toml:"agents" json:"agents"`
}

// ResourceQuota defines spending limits for an agent.
type ResourceQuota struct {
	MaxTokensPerHour    int     `toml:"max_tokens_per_hour" json:"max_tokens_per_hour"`
	MaxToolCallsPerHour int     `toml:"max_tool_calls_per_hour" json:"max_tool_calls_per_hour"`
	MaxCostPerHourUSD   float64 `toml:"max_cost_per_hour_usd" json:"max_cost_per_hour_usd"`
}

// BudgetConfig defines global budget limits.
type BudgetConfig struct {
	MaxHourlyUSD               float64 `toml:"max_hourly_usd" json:"max_hourly_usd"`
	MaxDailyUSD                float64 `toml:"max_daily_usd" json:"max_daily_usd"`
	MaxMonthlyUSD              float64 `toml:"max_monthly_usd" json:"max_monthly_usd"`
	AlertThreshold             float64 `toml:"alert_threshold" json:"alert_threshold"`
	DefaultMaxLLMTokensPerHour uint64  `toml:"default_max_llm_tokens_per_hour" json:"default_max_llm_tokens_per_hour"`
}

// DefaultConfig returns the default kernel configuration.
func DefaultConfig() KernelConfig {
	return KernelConfig{
		DataDir:  "~/.fangclaw-go",
		LogLevel: "info",
		API: APIConfig{
			Host:         "127.0.0.1",
			Port:         4200,
			EnableTLS:    false,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Models: ModelsConfig{
			DefaultProvider: "openai",
			DefaultModel:    "gpt-4o",
			Routing: ModelRoutingConfig{
				SimpleModel:      "gpt-4o-mini",
				MediumModel:      "gpt-4o",
				ComplexModel:     "gpt-4o",
				SimpleThreshold:  100,
				ComplexThreshold: 500,
			},
		},
		Memory: MemoryConfig{
			Path:           "~/.fangclaw-go/memory.db",
			EnableSemantic: true,
			EmbeddingModel: "text-embedding-3-small",
			MaxMemorySize:  10000,
		},
		Security: SecurityConfig{
			EnableRBAC:      false,
			EnableAuditLogs: true,
			AuditLogPath:    "~/.fangclaw-go/audit.log",
		},
		Skills: SkillsConfig{
			Path:               "~/.fangclaw-go/skills",
			EnableRemoteSkills: true,
			ClawHubURL:         "https://clawhub.io",
			AllowedRuntimes:    []string{"python", "node", "wasm", "builtin", "prompt"},
		},
		A2a: A2aConfig{
			Enabled:        false,
			ListenPath:     "/a2a",
			ExternalAgents: []ExternalAgent{},
		},
		Extensions: ExtensionsConfig{
			Path:             "~/.fangclaw-go/extensions",
			EnableExtensions: true,
		},
		CronShellSecurity: CronShellSecurityConfig{
			EnableExecuteShell: true,
			SecurityMode:       "strict",
			AllowedCommands: []string{
				"date",
				"echo",
				"ls",
				"cat",
				"pwd",
				"whoami",
				"uptime",
				"df",
				"du",
			},
			AllowedPaths: []string{
				"/bin",
				"/usr/bin",
				"/usr/local/bin",
			},
			ForbiddenCommands: []string{
				"rm",
				"rmdir",
				"mkfs",
				"dd",
				"chmod",
				"chown",
				"su",
				"sudo",
			},
			ForbiddenArgsPatterns: []string{
				"^/",
				"^\\.\\./",
				"&&",
				"\\|\\|",
				";",
				">",
				">>",
				"<",
				"\\$(",
				"`",
				"\\*",
				"\\?",
				"\\[",
				"\\]",
			},
		},
		McpServers: []McpServerConfig{},
		Quotas: QuotasConfig{
			Default: ResourceQuota{
				MaxTokensPerHour:    100000,
				MaxToolCallsPerHour: 100,
				MaxCostPerHourUSD:   10.0,
			},
			Agents: map[string]ResourceQuota{},
		},
		Budget: BudgetConfig{
			MaxHourlyUSD:               0.0,
			MaxDailyUSD:                0.0,
			MaxMonthlyUSD:              0.0,
			AlertThreshold:             0.8,
			DefaultMaxLLMTokensPerHour: 0,
		},
		Auth: AuthConfig{
			Enabled:    true,
			DBPath:     "",
			SessionTTL: "24h",
			GitHub: GitHubOAuthConfig{
				ClientID:     "",
				ClientSecret: "",
				Enabled:      false,
				RedirectURL:  "",
			},
		},
	}
}
