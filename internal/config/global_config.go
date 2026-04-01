package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/penzhan8451/fangclaw-go/internal/userdir"
	"github.com/rs/zerolog/log"
)

type GlobalConfig struct {
	API APIGlobalConfig `toml:"api" json:"api"`
	Log LogGlobalConfig `toml:"log" json:"log"`
}

type APIGlobalConfig struct {
	Host         string `toml:"host" json:"host"`
	Port         int    `toml:"port" json:"port"`
	EnableTLS    bool   `toml:"enable_tls" json:"enable_tls"`
	CertFile     string `toml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile      string `toml:"key_file,omitempty" json:"key_file,omitempty"`
	ReadTimeout  int    `toml:"read_timeout" json:"read_timeout"`
	WriteTimeout int    `toml:"write_timeout" json:"write_timeout"`
}

type LogGlobalConfig struct {
	Level string `toml:"level" json:"level"`
	File  string `toml:"file" json:"file"`
}

func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		API: APIGlobalConfig{
			Host:         "127.0.0.1",
			Port:         4200,
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Log: LogGlobalConfig{
			Level: "info",
		},
	}
}

type GlobalConfigManager struct {
	mu       sync.RWMutex
	config   *GlobalConfig
	filePath string
}

var (
	globalConfigManager *GlobalConfigManager
	globalConfigOnce    sync.Once
)

func GetGlobalConfigManager() (*GlobalConfigManager, error) {
	var err error
	globalConfigOnce.Do(func() {
		mgr, e := userdir.GetDefaultManager()
		if e != nil {
			err = e
			return
		}
		globalConfigManager, err = NewGlobalConfigManager(mgr.GlobalConfigPath())
	})
	if err != nil {
		return nil, err
	}
	return globalConfigManager, nil
}

func NewGlobalConfigManager(filePath string) (*GlobalConfigManager, error) {
	gcm := &GlobalConfigManager{
		filePath: filePath,
		config:   DefaultGlobalConfig(),
	}

	if err := gcm.Load(); err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	return gcm, nil
}

func (gcm *GlobalConfigManager) Load() error {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()

	if _, err := os.Stat(gcm.filePath); os.IsNotExist(err) {
		return gcm.saveWithoutLock()
	}

	data, err := os.ReadFile(gcm.filePath)
	if err != nil {
		return fmt.Errorf("failed to read global config: %w", err)
	}

	var cfg GlobalConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse global config: %w", err)
	}

	gcm.config = &cfg
	return nil
}

func (gcm *GlobalConfigManager) Save() error {
	gcm.mu.Lock()
	defer gcm.mu.Unlock()
	return gcm.saveWithoutLock()
}

func (gcm *GlobalConfigManager) saveWithoutLock() error {
	dir := filepath.Dir(gcm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := toml.Marshal(gcm.config)
	if err != nil {
		return fmt.Errorf("failed to encode global config: %w", err)
	}

	if err := os.WriteFile(gcm.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write global config: %w", err)
	}

	return nil
}

func (gcm *GlobalConfigManager) Get() *GlobalConfig {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()
	return gcm.config
}

func (gcm *GlobalConfigManager) Set(cfg *GlobalConfig) error {
	gcm.mu.Lock()
	gcm.config = cfg
	gcm.mu.Unlock()
	return gcm.Save()
}

func (gcm *GlobalConfigManager) GetAPIConfig() APIGlobalConfig {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()
	return gcm.config.API
}

func (gcm *GlobalConfigManager) SetAPIConfig(api APIGlobalConfig) error {
	gcm.mu.Lock()
	gcm.config.API = api
	gcm.mu.Unlock()
	return gcm.Save()
}

func (gcm *GlobalConfigManager) GetLogConfig() LogGlobalConfig {
	gcm.mu.RLock()
	defer gcm.mu.RUnlock()
	return gcm.config.Log
}

func (gcm *GlobalConfigManager) SetLogConfig(log LogGlobalConfig) error {
	gcm.mu.Lock()
	gcm.config.Log = log
	gcm.mu.Unlock()
	return gcm.Save()
}

func (gcm *GlobalConfigManager) FilePath() string {
	return gcm.filePath
}

func LoadUserConfig(username string) (*Config, error) {
	mgr, err := userdir.GetDefaultManager()
	if err != nil {
		return nil, fmt.Errorf("failed to get userdir manager: %w", err)
	}

	userConfigPath := mgr.UserConfigPath(username)

	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		defaultCfg := DefaultConfig()
		if saveErr := SaveUserConfig(username, defaultCfg); saveErr != nil {
			log.Warn().Err(saveErr).Str("user", username).Msg("Failed to save default user config")
		} else {
			log.Info().Str("user", username).Str("path", userConfigPath).Msg("Created default user config")
		}
		return defaultCfg, nil
	}

	data, err := os.ReadFile(userConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	return &cfg, nil
}

func SaveUserConfig(username string, cfg *Config) error {
	mgr, err := userdir.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get userdir manager: %w", err)
	}

	if err := mgr.EnsureUserDir(username); err != nil {
		return fmt.Errorf("failed to ensure user directory: %w", err)
	}

	userConfigPath := mgr.UserConfigPath(username)

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode user config: %w", err)
	}

	if err := os.WriteFile(userConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write user config: %w", err)
	}

	return nil
}

func MergeConfigWithUser(globalCfg *GlobalConfig, userCfg *Config) *Config {
	merged := *userCfg

	if merged.APIListen == "" {
		host := globalCfg.API.Host
		if host == "" {
			host = "127.0.0.1"
		}
		port := globalCfg.API.Port
		if port == 0 {
			port = 4200
		}
		merged.APIListen = fmt.Sprintf("%s:%d", host, port)
	}

	if merged.Log.Level == "" {
		merged.Log.Level = globalCfg.Log.Level
	}

	return &merged
}
