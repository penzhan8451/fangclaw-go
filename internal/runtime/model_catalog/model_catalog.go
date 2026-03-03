package model_catalog

import (
	"os"
	"strings"
	"sync"

	"github.com/penzhan8451/fangclaw-go/internal/types"
)

type ModelCatalog struct {
	mu        sync.RWMutex
	models    []types.ModelCatalogEntry
	aliases   map[string]string
	providers []types.ProviderInfo
	custom    []types.ModelCatalogEntry
}

func NewModelCatalog() *ModelCatalog {
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

	return &ModelCatalog{
		models:    models,
		aliases:   aliases,
		providers: providers,
		custom:    []types.ModelCatalogEntry{},
	}
}

func (c *ModelCatalog) DetectAuth() {
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]types.ModelCatalogEntry, 0, len(c.models)+len(c.custom))
	result = append(result, c.models...)
	result = append(result, c.custom...)
	return result
}

func (c *ModelCatalog) FindModel(idOrAlias string) *types.ModelCatalogEntry {
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aliases[strings.ToLower(alias)]
}

func (c *ModelCatalog) ListAliases() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string, len(c.aliases))
	for k, v := range c.aliases {
		result[k] = v
	}
	return result
}

func (c *ModelCatalog) ListProviders() []types.ProviderInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]types.ProviderInfo, len(c.providers))
	copy(result, c.providers)
	return result
}

func (c *ModelCatalog) GetProvider(id string) *types.ProviderInfo {
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
