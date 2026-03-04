// Package config provides configuration loading and management for OpenFang.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
)

// Config represents the OpenFang configuration.
type Config struct {
	APIListen    string           `toml:"api_listen"`
	DefaultModel ModelSettings    `toml:"default_model"`
	DefaultAgent string           `toml:"default_agent"`
	Memory       MemorySettings   `toml:"memory"`
	Security     SecuritySettings `toml:"security"`
	Log          LogSettings      `toml:"log"`
	LLM          llm.Config       `toml:"llm"`
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
		APIListen: "127.0.0.1:4200",
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
