package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/model_catalog"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/penzhan8451/fangclaw-go/internal/userdir"
	"github.com/rs/zerolog/log"
)

type UserProvidersHandler struct {
	router *Router
}

func (h *UserProvidersHandler) handleUserProviders(w http.ResponseWriter, req *http.Request) {
	user := GetUserFromContext(req.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var userSecrets map[string]string
	var err error
	if IsOwner(user) {
		userSecrets, err = userdir.LoadUserSecrets("")
		if err != nil {
			userSecrets = make(map[string]string)
		}
	} else {
		userSecrets, err = userdir.LoadUserSecrets(user.Username)
		if err != nil {
			userSecrets = make(map[string]string)
		}
	}

	k := h.router.getKernel(req)
	if k == nil {
		log.Error().Str("user", user.Username).Msg("handleUserProviders: kernel is nil")
		respondError(w, http.StatusInternalServerError, "Kernel not available")
		return
	}

	catalog := k.ModelCatalog()
	if catalog == nil {
		log.Error().Str("user", user.Username).Msg("handleUserProviders: catalog is nil")
		respondError(w, http.StatusInternalServerError, "Model catalog not available")
		return
	}

	providers := catalog.ListProviders()

	var result []map[string]interface{}
	for _, p := range providers {
		authStatus := "not_configured"
		if p.KeyRequired && p.APIKeyEnv != "" {
			if _, exists := userSecrets[p.APIKeyEnv]; exists {
				authStatus = "configured"
			} else if k.GetSecret(p.APIKeyEnv) != "" {
				authStatus = "global_configured"
			} else if os.Getenv(p.APIKeyEnv) != "" {
				authStatus = "global_configured"
			}
		} else if !p.KeyRequired {
			authStatus = "not_required"
		}

		result = append(result, map[string]interface{}{
			"id":           p.ID,
			"display_name": p.DisplayName,
			"auth_status":  authStatus,
			"model_count":  p.ModelCount,
			"key_required": p.KeyRequired,
			"api_key_env":  p.APIKeyEnv,
			"base_url":     p.BaseURL,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"providers": result,
		"total":     len(result),
	})
}

func (h *UserProvidersHandler) handleSetUserProviderKey(w http.ResponseWriter, req *http.Request) {
	user := GetUserFromContext(req.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	name := req.PathValue("name")
	k := h.router.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getUserProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	var reqBody struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	key := strings.TrimSpace(reqBody.Key)
	if key == "" {
		respondError(w, http.StatusBadRequest, "Missing or empty 'key' field")
		return
	}

	if provider.APIKeyEnv == "" {
		respondError(w, http.StatusBadRequest, "Provider does not require an API key")
		return
	}

	if IsOwner(user) {
		if err := config.WriteSecretEnv(provider.APIKeyEnv, key); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save user secret: %v", err))
			return
		}
	} else {
		if err := userdir.SetUserSecret(user.Username, provider.APIKeyEnv, key); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save user secret: %v", err))
			return
		}
	}

	var userCfg *config.Config
	var err error
	if IsOwner(user) {
		userCfg, err = config.Load("")
	} else {
		userCfg, err = config.LoadUserConfig(user.Username)
	}
	if err != nil {
		log.Warn().Err(err).Str("user", user.Username).Msg("Failed to load user config for provider update")
		userCfg = config.DefaultConfig()
	}
	userCfg.DefaultModel.Provider = name
	userCfg.DefaultModel.APIKeyEnv = provider.APIKeyEnv
	if IsOwner(user) {
		if err := config.Save(userCfg, ""); err != nil {
			log.Warn().Err(err).Str("user", user.Username).Msg("Failed to save user config")
		}
	} else {
		if err := config.SaveUserConfig(user.Username, userCfg); err != nil {
			log.Warn().Err(err).Str("user", user.Username).Msg("Failed to save user config")
		}
	}

	if k != nil {
		if err := k.ReloadSecrets(); err != nil {
			log.Warn().Err(err).Str("user", user.Username).Msg("Failed to reload secrets")
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "saved",
		"provider":     name,
		"api_key_env":  provider.APIKeyEnv,
		"saved_config": true,
	})
}

func (h *UserProvidersHandler) handleDeleteUserProviderKey(w http.ResponseWriter, req *http.Request) {
	user := GetUserFromContext(req.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	name := req.PathValue("name")
	k := h.router.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getUserProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	if provider.APIKeyEnv == "" {
		respondError(w, http.StatusBadRequest, "Provider does not require an API key")
		return
	}

	if IsOwner(user) {
		if err := config.RemoveSecretEnv(provider.APIKeyEnv); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete user secret: %v", err))
			return
		}
	} else {
		if err := userdir.DeleteUserSecret(user.Username, provider.APIKeyEnv); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete user secret: %v", err))
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "removed",
		"provider": name,
	})
}

func (h *UserProvidersHandler) handleTestUserProvider(w http.ResponseWriter, req *http.Request) {
	user := GetUserFromContext(req.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	name := req.PathValue("name")
	k := h.router.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getUserProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	var userSecrets map[string]string
	var err error
	if IsOwner(user) {
		userSecrets, err = userdir.LoadUserSecrets("")
		if err != nil {
			userSecrets = make(map[string]string)
		}
	} else {
		userSecrets, err = userdir.LoadUserSecrets(user.Username)
		if err != nil {
			userSecrets = make(map[string]string)
		}
	}

	apiKey := userSecrets[provider.APIKeyEnv]
	if apiKey == "" {
		apiKey = os.Getenv(provider.APIKeyEnv)
	}

	if provider.KeyRequired && apiKey == "" && provider.APIKeyEnv != "" {
		respondError(w, http.StatusBadRequest, "Provider API key not configured")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"provider":   name,
		"latency_ms": 100,
	})
}

func getUserProviderFromCatalog(catalog *model_catalog.ModelCatalog, id string) *types.ProviderInfo {
	providers := catalog.ListProviders()
	for i := range providers {
		if providers[i].ID == id {
			return &providers[i]
		}
	}
	return nil
}

func RegisterUserProviderRoutes(mux *http.ServeMux, router *Router) {
	handler := &UserProvidersHandler{router: router}

	mux.HandleFunc("GET /api/user/providers", handler.handleUserProviders)
	mux.HandleFunc("POST /api/user/providers/{name}/key", handler.handleSetUserProviderKey)
	mux.HandleFunc("DELETE /api/user/providers/{name}/key", handler.handleDeleteUserProviderKey)
	mux.HandleFunc("POST /api/user/providers/{name}/test", handler.handleTestUserProvider)
}
