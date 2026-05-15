package api

import (
	"encoding/json"
	"net/http"
	"strings"

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
		"cron_shell_security": map[string]interface{}{
			"enable_execute_shell": userCfg.CronShellSecurity.EnableExecuteShell,
			// "security_mode":           userCfg.CronShellSecurity.SecurityMode,
			// "allowed_commands":        strings.Join(userCfg.CronShellSecurity.AllowedCommands, ", "),
			// "allowed_paths":           strings.Join(userCfg.CronShellSecurity.AllowedPaths, ", "),
			// "forbidden_commands":      strings.Join(userCfg.CronShellSecurity.ForbiddenCommands, ", "),
			// "forbidden_args_patterns": strings.Join(userCfg.CronShellSecurity.ForbiddenArgsPatterns, ", "),
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
	case "cron_shell_security.enable_execute_shell":
		if v, ok := reqBody.Value.(bool); ok {
			userCfg.CronShellSecurity.EnableExecuteShell = v
		}
	case "cron_shell_security.security_mode":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.CronShellSecurity.SecurityMode = v
		}
	case "cron_shell_security.allowed_commands":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.CronShellSecurity.AllowedCommands = parseCommaList(v)
		}
	case "cron_shell_security.allowed_paths":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.CronShellSecurity.AllowedPaths = parseCommaList(v)
		}
	case "cron_shell_security.forbidden_commands":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.CronShellSecurity.ForbiddenCommands = parseCommaList(v)
		}
	case "cron_shell_security.forbidden_args_patterns":
		if v, ok := reqBody.Value.(string); ok {
			userCfg.CronShellSecurity.ForbiddenArgsPatterns = parseCommaList(v)
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
		if strings.HasPrefix(reqBody.Path, "cron_shell_security.") {
			newKernelConfig := k.Config()
			newKernelConfig.CronShellSecurity = userCfg.CronShellSecurity
			k.ReloadConfig(newKernelConfig)
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
			"cron_shell_security": []map[string]interface{}{
				{"name": "enable_execute_shell", "type": "boolean", "label": "Enable Shell Execution", "description": "Allow cron jobs to execute shell commands"},
				// {"name": "security_mode", "type": "select", "label": "Security Mode", "options": []string{"strict", "path", "none"}, "description": "strict: only allowed commands | path: only from allowed paths | none: no restrictions"},
				// {"name": "allowed_commands", "type": "string", "label": "Allowed Commands", "description": "Comma-separated list of allowed commands (used in strict mode)"},
				// {"name": "allowed_paths", "type": "string", "label": "Allowed Paths", "description": "Comma-separated list of allowed binary paths (used in path mode)"},
				// {"name": "forbidden_commands", "type": "string", "label": "Forbidden Commands", "description": "Comma-separated list of forbidden commands (always enforced)"},
				// {"name": "forbidden_args_patterns", "type": "string", "label": "Forbidden Arg Patterns", "description": "Comma-separated regex patterns for forbidden arguments"},
			},
		},
	})
}

func parseCommaList(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func RegisterUserConfigRoutes(mux *http.ServeMux, router *Router) {
	handler := &UserConfigHandler{router: router}

	mux.HandleFunc("GET /api/user/config", handler.handleGetUserConfig)
	mux.HandleFunc("POST /api/user/config/set", handler.handleSetUserConfig)
	mux.HandleFunc("GET /api/user/config/schema", handler.handleGetUserConfigSchema)
}
