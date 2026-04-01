package api

import (
	"encoding/json"
	"net/http"

	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/rs/zerolog/log"
)

type UserConfigHandler struct {
	router *Router
}

func (h *UserConfigHandler) handleGetUserConfig(w http.ResponseWriter, req *http.Request) {
	user := GetUserFromContext(req.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var userCfg *config.Config
	var err error
	if IsOwner(user) {
		userCfg, err = config.Load("")
	} else {
		userCfg, err = config.LoadUserConfig(user.Username)
	}
	if err != nil {
		log.Warn().Err(err).Str("user", user.Username).Msg("Failed to load user config, returning defaults")
		userCfg = config.DefaultConfig()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"default_model": map[string]interface{}{
			"provider":    userCfg.DefaultModel.Provider,
			"model":       userCfg.DefaultModel.Model,
			"api_key_env": userCfg.DefaultModel.APIKeyEnv,
		},
		"theme":             "system",
		"language":          "en",
		"sidebar_collapsed": false,
	})
}

func (h *UserConfigHandler) handleSetUserConfig(w http.ResponseWriter, req *http.Request) {
	user := GetUserFromContext(req.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var reqBody struct {
		Path  string      `json:"path"`
		Value interface{} `json:"value"`
	}

	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var userCfg *config.Config
	var err error
	if IsOwner(user) {
		userCfg, err = config.Load("")
	} else {
		userCfg, err = config.LoadUserConfig(user.Username)
	}
	if err != nil {
		log.Warn().Err(err).Str("user", user.Username).Msg("Failed to load user config, using defaults")
		userCfg = config.DefaultConfig()
	}

	switch reqBody.Path {
	case "default_model.provider":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.DefaultModel.Provider = v
		}
	case "default_model.model":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.DefaultModel.Model = v
		}
	case "default_model.api_key_env":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.DefaultModel.APIKeyEnv = v
		}
	}

	if IsOwner(user) {
		if err := config.Save(userCfg, ""); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to save config")
			return
		}
	} else {
		if err := config.SaveUserConfig(user.Username, userCfg); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to save config")
			return
		}
	}

	k := h.router.getKernel(req)
	if k != nil {
		if err := k.ReloadSecrets(); err != nil {
			log.Warn().Err(err).Str("user", user.Username).Msg("Failed to reload secrets")
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "saved",
		"path":   reqBody.Path,
	})
}

func (h *UserConfigHandler) handleGetUserConfigSchema(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sections": map[string]interface{}{
			"default_model": []map[string]interface{}{
				{"name": "provider", "type": "string", "label": "Provider"},
				{"name": "model", "type": "string", "label": "Model"},
				{"name": "api_key_env", "type": "string", "label": "API Key Env"},
			},
		},
	})
}

func RegisterUserConfigRoutes(mux *http.ServeMux, router *Router) {
	handler := &UserConfigHandler{router: router}

	mux.HandleFunc("GET /api/user/config", handler.handleGetUserConfig)
	mux.HandleFunc("POST /api/user/config/set", handler.handleSetUserConfig)
	mux.HandleFunc("GET /api/user/config/schema", handler.handleGetUserConfigSchema)
}
