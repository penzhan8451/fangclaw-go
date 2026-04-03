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
	"github.com/rs/zerolog/log"
)

type ModelCatalog struct {
	mu              sync.RWMutex
	models          []types.ModelCatalogEntry
	aliases         map[string]string
	providers       []types.ProviderInfo
	custom          []types.ModelCatalogEntry
	configPath      string
	lastModified    time.Time
	sharedModels    []types.ModelCatalogEntry
	sharedAliases   map[string]string
	sharedProviders []types.ProviderInfo
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

// NewModelCatalogWithShared creates a new ModelCatalog with shared built-in models.
func NewModelCatalogWithShared(configPath string, sharedModels []types.ModelCatalogEntry, sharedAliases map[string]string, sharedProviders []types.ProviderInfo) *ModelCatalog {
	catalog := &ModelCatalog{
		configPath:      configPath,
		custom:          []types.ModelCatalogEntry{},
		sharedModels:    sharedModels,
		sharedAliases:   sharedAliases,
		sharedProviders: sharedProviders,
	}

	catalog.loadOrInitializeWithShared()
	return catalog
}

// loadOrInitialize loads the catalog from config file, or initializes with defaults.
func (c *ModelCatalog) loadOrInitialize() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.configPath != "" {
		if _, err := os.Stat(c.configPath); err == nil {
			if err := c.loadFromFileUnsafe(); err == nil {
				log.Info().Str("path", c.configPath).Msg("Loaded model catalog")
				return
			}
			log.Warn().Err(err).Str("path", c.configPath).Msg("Failed to load model catalog, using defaults")
		}

		c.loadDefaultsUnsafe()
		if err := c.saveToFileUnsafe(); err == nil {
			log.Info().Str("path", c.configPath).Msg("Created default model catalog")
		} else {
			log.Warn().Err(err).Str("path", c.configPath).Msg("Failed to save default model catalog")
		}
		return
	}

	c.loadDefaultsUnsafe()
}

// loadOrInitializeWithShared loads the catalog with shared built-in models.
func (c *ModelCatalog) loadOrInitializeWithShared() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// When using shared content, don't try to load/save from user directory
	// Just use the shared content directly
	c.useSharedContent()
}

// useSharedContent uses the shared built-in models, aliases, and providers.
func (c *ModelCatalog) useSharedContent() {
	if len(c.sharedModels) > 0 {
		c.models = c.sharedModels
	} else {
		c.models = types.BuiltinModels()
	}

	if len(c.sharedAliases) > 0 {
		c.aliases = c.sharedAliases
	} else {
		c.aliases = types.BuiltinAliases()
	}

	if len(c.sharedProviders) > 0 {
		c.providers = c.sharedProviders
	} else {
		c.providers = types.BuiltinProviders()
	}
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

	builtInModels := types.BuiltinModels()
	builtInProviders := types.BuiltinProviders()
	builtInAliases := types.BuiltinAliases()

	existingModelIDs := make(map[string]bool)
	for _, m := range file.Models {
		existingModelIDs[m.ID] = true
	}

	for _, m := range builtInModels {
		if !existingModelIDs[m.ID] {
			file.Models = append(file.Models, m)
		}
	}

	existingProviderIDs := make(map[string]bool)
	for _, p := range file.Providers {
		existingProviderIDs[p.ID] = true
	}

	for _, p := range builtInProviders {
		if !existingProviderIDs[p.ID] {
			file.Providers = append(file.Providers, p)
		}
	}

	aliases := make(map[string]string)
	for _, model := range file.Models {
		for _, alias := range model.Aliases {
			aliases[strings.ToLower(alias)] = model.ID
		}
	}

	for k, v := range builtInAliases {
		if _, ok := aliases[k]; !ok {
			aliases[k] = v
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

	file := types.ModelCatalogFile{
		Version:   "1.0",
		Providers: c.providers,
		Models:    c.models,
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0644)
}

// Reload reloads the catalog from the config file if it has changed.
func (c *ModelCatalog) Reload() error {
	// When using shared content, don't reload from file
	if len(c.sharedModels) > 0 || len(c.sharedAliases) > 0 || len(c.sharedProviders) > 0 {
		return nil
	}

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
		log.Debug().Str("path", c.configPath).Msg("Reloading model catalog (file changed)")
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
			// NOTE: Commented out for multi-tenant mode - use DetectAuthWithSecrets instead
			// _, hasKey = os.LookupEnv(c.providers[i].APIKeyEnv)
		}

		if c.providers[i].ID == "gemini" && !hasKey {
			// NOTE: Commented out for multi-tenant mode
			// _, hasKey = os.LookupEnv("GOOGLE_API_KEY")
		}

		if hasKey {
			c.providers[i].AuthStatus = types.AuthStatusConfigured
		} else {
			c.providers[i].AuthStatus = types.AuthStatusMissing
		}
	}
}

func (c *ModelCatalog) DetectAuthWithSecrets(secrets map[string]string) {
	_ = c.Reload()
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.providers {
		if !c.providers[i].KeyRequired {
			c.providers[i].AuthStatus = types.AuthStatusNotRequired
			continue
		}

		hasKey := false
		if c.providers[i].APIKeyEnv != "" && secrets != nil {
			_, hasKey = secrets[c.providers[i].APIKeyEnv]
		}

		if c.providers[i].ID == "gemini" && !hasKey && secrets != nil {
			_, hasKey = secrets["GOOGLE_API_KEY"]
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

// GetSharedModels returns the shared built-in models.
func (c *ModelCatalog) GetSharedModels() []types.ModelCatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]types.ModelCatalogEntry, len(c.models))
	copy(result, c.models)
	return result
}

// GetSharedAliases returns the shared aliases.
func (c *ModelCatalog) GetSharedAliases() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]string, len(c.aliases))
	for k, v := range c.aliases {
		result[k] = v
	}
	return result
}

// GetSharedProviders returns the shared providers.
func (c *ModelCatalog) GetSharedProviders() []types.ProviderInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]types.ProviderInfo, len(c.providers))
	copy(result, c.providers)
	return result
}
