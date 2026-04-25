// Package config provides configuration loading and management for OpenFang.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/penzhan8451/fangclaw-go/internal/approvals"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// config.toml: Config represents the FangClaw-Go configuration.
type Config struct {
	APIListen    string                   `toml:"api_listen"`
	DefaultModel ModelSettings            `toml:"default_model"`
	DefaultAgent string                   `toml:"default_agent"`
	Memory       MemorySettings           `toml:"memory"`
	Security     SecuritySettings         `toml:"security"`
	Log          LogSettings              `toml:"log"`
	Channels     ChannelsConfig           `toml:"channels"`
	McpServers   []types.McpServerConfig  `toml:"mcp_servers,omitempty"`
	Browser      BrowserSettings          `toml:"browser"`
	A2a          types.A2aConfig          `toml:"a2a"`
	Auth         types.AuthConfig         `toml:"auth"`
	Approvals    approvals.ApprovalPolicy `toml:"approvals"`
}

// config.toml: BrowserSettings represents the browser settings.
type BrowserSettings struct {
	Enabled        bool   `toml:"enabled"`
	ChromiumPath   string `toml:"chromium_path"`
	Headless       bool   `toml:"headless"`
	ViewportWidth  int    `toml:"viewport_width"`
	ViewportHeight int    `toml:"viewport_height"`
	MaxSessions    int    `toml:"max_sessions"`
}

// ChannelsConfig contains configuration for all channel adapters.
type ChannelsConfig struct {
	Telegram *ChannelConfig `toml:"telegram,omitempty"`
	Discord  *ChannelConfig `toml:"discord,omitempty"`
	Slack    *ChannelConfig `toml:"slack,omitempty"`
	WhatsApp *ChannelConfig `toml:"whatsapp,omitempty"`
	QQ       *ChannelConfig `toml:"qq,omitempty"`
	DingTalk *ChannelConfig `toml:"dingtalk,omitempty"`
	Feishu   *ChannelConfig `toml:"feishu,omitempty"`
	Weixin   *ChannelConfig `toml:"weixin,omitempty"`
}

// ChannelConfig represents a single channel adapter's configuration.
type ChannelConfig struct {
	BotToken           string `toml:"bot_token,omitempty"`
	BotTokenEnv        string `toml:"bot_token_env,omitempty"`
	AppToken           string `toml:"app_token,omitempty"`
	AppTokenEnv        string `toml:"app_token_env,omitempty"`
	AllowedUsers       string `toml:"allowed_users,omitempty"`
	AllowedGuilds      string `toml:"allowed_guilds,omitempty"`
	AllowedChannels    string `toml:"allowed_channels,omitempty"`
	DefaultAgent       string `toml:"default_agent,omitempty"`
	AccessToken        string `toml:"access_token,omitempty"`
	AccessTokenEnv     string `toml:"access_token_env,omitempty"`
	PhoneNumberID      string `toml:"phone_number_id,omitempty"`
	VerifyToken        string `toml:"verify_token,omitempty"`
	VerifyTokenEnv     string `toml:"verify_token_env,omitempty"`
	AppID              string `toml:"app_id,omitempty"`
	AppSecretEnv       string `toml:"app_secret_env,omitempty"`
	AppSecret          string `toml:"app_secret,omitempty"`
	Secret             string `toml:"secret,omitempty"`
	SecretEnv          string `toml:"secret_env,omitempty"`
	ClientID           string `toml:"client_id,omitempty"`
	ClientIDEnv        string `toml:"client_id_env,omitempty"`
	ClientSecret       string `toml:"client_secret,omitempty"`
	ClientSecretEnv    string `toml:"client_secret_env,omitempty"`
	Token              string `toml:"token,omitempty"`
	TokenEnv           string `toml:"token_env,omitempty"`
	BaseURL            string `toml:"base_url,omitempty"`
	CDNBaseURL         string `toml:"cdn_base_url,omitempty"`
	Proxy              string `toml:"proxy,omitempty"`
	ReasoningChannelID string `toml:"reasoning_channel_id,omitempty"`
	GroupTrigger       string `toml:"group_trigger,omitempty"`
}

// ModelSettings defines the default model configuration.
type ModelSettings struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	APIKeyEnv string `toml:"api_key_env"`
}

// MemorySettings defines memory-related configuration.
type MemorySettings struct {
	DecayRate float64 `toml:"decay_rate"`
}

// SecuritySettings defines security configuration.
type SecuritySettings struct {
	RateLimitPerMinute int `toml:"rate_limit_per_minute"`
}

// LogSettings defines logging configuration.
type LogSettings struct {
	Level string `toml:"level"`
	File  string `toml:"file"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		APIListen: "0.0.0.0:8080",
		DefaultModel: ModelSettings{
			Provider:  "groq",
			Model:     "llama-3.3-70b-versatile",
			APIKeyEnv: "GROQ_API_KEY",
		},
		DefaultAgent: "",
		Memory: MemorySettings{
			DecayRate: 0.05,
		},
		Security: SecuritySettings{
			RateLimitPerMinute: 60,
		},
		Log: LogSettings{
			Level: "info",
		},
		Auth: types.AuthConfig{
			Enabled:    true,
			DBPath:     "",
			SessionTTL: "24h",
			GitHub: types.GitHubOAuthConfig{
				ClientID:     "",
				ClientSecret: "",
				Enabled:      false,
				RedirectURL:  "",
			},
		},
		Approvals: approvals.DefaultApprovalPolicy(),
	}
}

// Load loads configuration from the given path.
func Load(path string) (*Config, error) {
	// If no path provided, use default location
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", err)
		}
		path = filepath.Join(homeDir, ".fangclaw-go", "config.toml")
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	// Load config from file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save saves the configuration to the given path.
func Save(cfg *Config, path string) error {
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}
		path = filepath.Join(homeDir, ".fangclaw-go", "config.toml")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to TOML
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Get returns a config value by key path.
func Get(key string) (string, error) {
	cfg, err := Load("")
	if err != nil {
		return "", err
	}

	// Simple key lookup
	switch key {
	case "api_listen":
		return cfg.APIListen, nil
	case "default_agent":
		return cfg.DefaultAgent, nil
	case "default_model.provider":
		return cfg.DefaultModel.Provider, nil
	case "default_model.model":
		return cfg.DefaultModel.Model, nil
	case "default_model.api_key_env":
		return cfg.DefaultModel.APIKeyEnv, nil
	case "memory.decay_rate":
		return fmt.Sprintf("%f", cfg.Memory.DecayRate), nil
	default:
		return "", fmt.Errorf("key not found: %s", key)
	}
}

// Set sets a config value by key path.
func Set(key, value string) error {
	cfg, err := Load("")
	if err != nil {
		return err
	}

	// Simple key set
	switch key {
	case "api_listen":
		cfg.APIListen = value
	case "default_agent":
		cfg.DefaultAgent = value
	case "default_model.provider":
		cfg.DefaultModel.Provider = value
	case "default_model.model":
		cfg.DefaultModel.Model = value
	case "default_model.api_key_env":
		cfg.DefaultModel.APIKeyEnv = value
	case "memory.decay_rate":
		fmt.Sscanf(value, "%f", &cfg.Memory.DecayRate)
	default:
		return fmt.Errorf("unsupported key: %s", key)
	}

	return Save(cfg, "")
}

// Unset removes a config key (sets to default).
func Unset(key string) error {
	cfg, err := Load("")
	if err != nil {
		return err
	}

	// Reset to defaults
	switch key {
	case "api_listen":
		cfg.APIListen = "127.0.0.1:4200"
	case "default_agent":
		cfg.DefaultAgent = ""
	case "default_model.provider":
		cfg.DefaultModel.Provider = "groq"
	case "default_model.model":
		cfg.DefaultModel.Model = "llama-3.3-70b-versatile"
	case "default_model.api_key_env":
		cfg.DefaultModel.APIKeyEnv = "GROQ_API_KEY"
	case "memory.decay_rate":
		cfg.Memory.DecayRate = 0.05
	default:
		return fmt.Errorf("unsupported key: %s", key)
	}

	return Save(cfg, "")
}

// GetSecretsPath returns the path to secrets.env file.
func GetSecretsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".fangclaw-go", "secrets.env"), nil
}

// WriteSecretEnv writes a secret to secrets.env file.
func WriteSecretEnv(key, value string) error {
	secretsPath, err := GetSecretsPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(secretsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	// Read existing secrets if file exists
	var lines []string
	data, err := os.ReadFile(secretsPath)
	if err == nil {
		lines = strings.Split(string(data), "\n")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read secrets: %w", err)
	}

	// Find and replace or add the key
	found := false
	newLine := fmt.Sprintf("%s=%s", key, value)
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = newLine
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, newLine)
	}

	// Write back - os.WriteFile will create file if it doesn't exist
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(secretsPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write secrets: %w", err)
	}

	// Also set in current process
	os.Setenv(key, value)

	return nil
}

// RemoveSecretEnv removes a secret from secrets.env file.
func RemoveSecretEnv(key string) error {
	secretsPath, err := GetSecretsPath()
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return nil
	}

	// Read existing secrets
	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return fmt.Errorf("failed to read secrets: %w", err)
	}
	lines := strings.Split(string(data), "\n")

	// Filter out the key
	var newLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, key+"=") {
			newLines = append(newLines, line)
		}
	}

	// Write back
	content := strings.Join(newLines, "\n")
	if err := os.WriteFile(secretsPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write secrets: %w", err)
	}

	// Also remove from current process
	os.Unsetenv(key)

	return nil
}
