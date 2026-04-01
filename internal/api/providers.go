package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/runtime/model_catalog"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/penzhan8451/fangclaw-go/internal/userdir"
)

func getSecretsPath(username string) (string, error) {
	if username == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, ".fangclaw-go", "secrets.env"), nil
	}
	mgr, err := userdir.GetDefaultManager()
	if err != nil {
		return "", err
	}
	return mgr.UserSecretsPath(username), nil
}

func writeSecretEnv(envVar, key, username string) error {
	secretsPath, err := getSecretsPath(username)
	if err != nil {
		return err
	}

	dir := filepath.Dir(secretsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	var lines []string
	if _, err := os.Stat(secretsPath); err == nil {
		file, err := os.Open(secretsPath)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		found := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, envVar+"=") {
				lines = append(lines, fmt.Sprintf("%s=%s", envVar, key))
				found = true
			} else {
				lines = append(lines, line)
			}
		}
		if !found {
			lines = append(lines, fmt.Sprintf("%s=%s", envVar, key))
		}
	} else {
		lines = append(lines, fmt.Sprintf("%s=%s", envVar, key))
	}

	return os.WriteFile(secretsPath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func removeSecretEnv(envVar, username string) error {
	secretsPath, err := getSecretsPath(username)
	if err != nil {
		return err
	}

	if _, err := os.Stat(secretsPath); err != nil {
		return nil
	}

	file, err := os.Open(secretsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, envVar+"=") {
			lines = append(lines, line)
		}
	}

	return os.WriteFile(secretsPath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func (r *Router) handleProviders(w http.ResponseWriter, req *http.Request) {
	k := r.getKernel(req)
	catalog := k.ModelCatalog()
	providers := catalog.ListProviders()

	var result []map[string]interface{}
	for _, p := range providers {
		authStatus := "not_configured"
		if p.KeyRequired && p.APIKeyEnv != "" {
			if os.Getenv(p.APIKeyEnv) != "" {
				authStatus = "configured"
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

func getProviderFromCatalog(catalog *model_catalog.ModelCatalog, id string) *types.ProviderInfo {
	providers := catalog.ListProviders()
	for i := range providers {
		if providers[i].ID == id {
			return &providers[i]
		}
	}
	return nil
}

func (r *Router) handleSetProviderKey(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	k := r.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	var reqBody struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Missing or empty 'key' field")
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

	username := GetUsernameFromContext(req.Context())
	if err := writeSecretEnv(provider.APIKeyEnv, key, username); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write secrets.env: %v", err))
		return
	}

	os.Setenv(provider.APIKeyEnv, key)

	catalog.DetectAuth()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "saved",
		"provider": name,
	})
}

func (r *Router) handleDeleteProviderKey(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	k := r.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	if provider.APIKeyEnv == "" {
		respondError(w, http.StatusBadRequest, "Provider does not require an API key")
		return
	}

	username := GetUsernameFromContext(req.Context())
	if err := removeSecretEnv(provider.APIKeyEnv, username); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update secrets.env: %v", err))
		return
	}

	os.Unsetenv(provider.APIKeyEnv)

	catalog.DetectAuth()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "removed",
		"provider": name,
	})
}

func (r *Router) handleTestProvider(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	k := r.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	apiKey := os.Getenv(provider.APIKeyEnv)
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

func (r *Router) handleSetProviderURL(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	k := r.getKernel(req)
	catalog := k.ModelCatalog()

	provider := getProviderFromCatalog(catalog, name)
	if provider == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Unknown provider '%s'", name))
		return
	}

	var reqBody struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, "Missing or empty 'base_url' field")
		return
	}

	baseURL := strings.TrimSpace(reqBody.BaseURL)
	if baseURL == "" {
		respondError(w, http.StatusBadRequest, "Missing or empty 'base_url' field")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "updated",
		"provider": name,
		"base_url": baseURL,
	})
}

func (r *Router) handleSetupStatus(w http.ResponseWriter, req *http.Request) {
	k := r.getKernel(req)
	isComplete := k.IsSetupComplete()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"setup_complete": isComplete,
		"message":        "Setup complete",
	})
}
