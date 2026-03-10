package model_catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type ModelCatalog struct {
	mu           sync.RWMutex
	models       []types.ModelCatalogEntry
	aliases      map[string]string
	providers    []types.ProviderInfo
	custom       []types.ModelCatalogEntry
	configPath   string
	lastModified time.Time
}

// NewModelCatalog creates a new ModelCatalog, loading from config file if available.
func NewModelCatalog(configPath string) *ModelCatalog {
	catalog := &ModelCatalog{
		configPath: configPath,
		custom:     []types.ModelCatalogEntry{},
	}

	catalog.loadOrInitialize()
	return catalog
}

// loadOrInitialize loads the catalog from config file, or initializes with defaults.
func (c *ModelCatalog) loadOrInitialize() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.configPath != "" {
		if _, err := os.Stat(c.configPath); err == nil {
			if err := c.loadFromFileUnsafe(); err == nil {
				fmt.Printf("Loaded model catalog from %s\n", c.configPath)
				return
			}
			fmt.Printf("Failed to load model catalog from %s, using defaults: %v\n", c.configPath, err)
		}

		if err := c.saveToFileUnsafe(); err == nil {
			fmt.Printf("Created default model catalog at %s\n", c.configPath)
		} else {
			fmt.Printf("Failed to save default model catalog: %v\n", err)
		}
	}

	c.loadDefaultsUnsafe()
}

// loadDefaultsUnsafe loads the built-in defaults (not thread-safe).
func (c *ModelCatalog) loadDefaultsUnsafe() {
	models := types.BuiltinModels()
	aliases := types.BuiltinAliases()
	providers := types.BuiltinProviders()

	for i := range providers {
		count := 0
		for _, m := range models {
			if m.Provider == providers[i].ID {
				count++
			}
		}
		providers[i].ModelCount = count
	}

	c.models = models
	c.aliases = aliases
	c.providers = providers
}

// loadFromFileUnsafe loads the catalog from the config file (not thread-safe).
func (c *ModelCatalog) loadFromFileUnsafe() error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	var file types.ModelCatalogFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	aliases := make(map[string]string)
	for _, model := range file.Models {
		for _, alias := range model.Aliases {
			aliases[strings.ToLower(alias)] = model.ID
		}
	}

	for i := range file.Providers {
		count := 0
		for _, m := range file.Models {
			if m.Provider == file.Providers[i].ID {
				count++
			}
		}
		file.Providers[i].ModelCount = count
	}

	c.models = file.Models
	c.aliases = aliases
	c.providers = file.Providers

	if info, err := os.Stat(c.configPath); err == nil {
		c.lastModified = info.ModTime()
	}

	return nil
}

// saveToFileUnsafe saves the catalog to the config file (not thread-safe).
func (c *ModelCatalog) saveToFileUnsafe() error {
	if c.configPath == "" {
		return fmt.Errorf("no config path specified")
	}

	if err := os.MkdirAll(filepath.Dir(c.configPath), 0755); err != nil {
		return err
	}

	models := types.BuiltinModels()
	providers := types.BuiltinProviders()

	file := types.ModelCatalogFile{
		Version:   "1.0",
		Providers: providers,
		Models:    models,
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0644)
}

// Reload reloads the catalog from the config file if it has changed.
func (c *ModelCatalog) Reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.configPath == "" {
		return nil
	}

	info, err := os.Stat(c.configPath)
	if err != nil {
		return err
	}

	if info.ModTime().After(c.lastModified) {
		fmt.Printf("Reloading model catalog from %s (file changed)\n", c.configPath)
		return c.loadFromFileUnsafe()
	}

	return nil
}

func (c *ModelCatalog) DetectAuth() {
	_ = c.Reload()
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.providers {
		if !c.providers[i].KeyRequired {
			c.providers[i].AuthStatus = types.AuthStatusNotRequired
			continue
		}

		hasKey := false
		if c.providers[i].APIKeyEnv != "" {
			_, hasKey = os.LookupEnv(c.providers[i].APIKeyEnv)
		}

		if c.providers[i].ID == "gemini" && !hasKey {
			_, hasKey = os.LookupEnv("GOOGLE_API_KEY")
		}

		if hasKey {
			c.providers[i].AuthStatus = types.AuthStatusConfigured
		} else {
			c.providers[i].AuthStatus = types.AuthStatusMissing
		}
	}
}

func (c *ModelCatalog) ListModels() []types.ModelCatalogEntry {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]types.ModelCatalogEntry, 0, len(c.models)+len(c.custom))
	result = append(result, c.models...)
	result = append(result, c.custom...)
	return result
}

func (c *ModelCatalog) FindModel(idOrAlias string) *types.ModelCatalogEntry {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()

	lower := strings.ToLower(idOrAlias)

	for _, m := range c.models {
		if strings.ToLower(m.ID) == lower {
			return &m
		}
	}

	for _, m := range c.custom {
		if strings.ToLower(m.ID) == lower {
			return &m
		}
	}

	if canonical, ok := c.aliases[lower]; ok {
		for _, m := range c.models {
			if m.ID == canonical {
				return &m
			}
		}
	}

	return nil
}

func (c *ModelCatalog) ResolveAlias(alias string) string {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aliases[strings.ToLower(alias)]
}

func (c *ModelCatalog) ListAliases() map[string]string {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string, len(c.aliases))
	for k, v := range c.aliases {
		result[k] = v
	}
	return result
}

func (c *ModelCatalog) ListProviders() []types.ProviderInfo {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]types.ProviderInfo, len(c.providers))
	copy(result, c.providers)
	return result
}

func (c *ModelCatalog) GetProvider(id string) *types.ProviderInfo {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.providers {
		if c.providers[i].ID == id {
			return &c.providers[i]
		}
	}
	return nil
}

func (c *ModelCatalog) AvailableModels() []types.ModelCatalogEntry {
	_ = c.Reload()
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []types.ModelCatalogEntry
	allModels := append([]types.ModelCatalogEntry{}, c.models...)
	allModels = append(allModels, c.custom...)

	for _, m := range allModels {
		p := c.getProviderUnsafe(m.Provider)
		if p != nil && p.AuthStatus != types.AuthStatusMissing {
			result = append(result, m)
		}
	}
	return result
}

func (c *ModelCatalog) AddCustomModel(model types.ModelCatalogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	model.Tier = types.ModelTierCustom
	c.custom = append(c.custom, model)
}

func (c *ModelCatalog) RemoveCustomModel(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, m := range c.custom {
		if m.ID == id {
			c.custom = append(c.custom[:i], c.custom[i+1:]...)
			return true
		}
	}
	return false
}

func (c *ModelCatalog) getProviderUnsafe(id string) *types.ProviderInfo {
	for i := range c.providers {
		if c.providers[i].ID == id {
			return &c.providers[i]
		}
	}
	return nil
}
