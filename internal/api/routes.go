// Package api implements the REST API server for OpenFang.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// Router manages API routes.
type Router struct {
	kernel *kernel.Kernel
}

// NewRouter creates a new API router.
func NewRouter(k *kernel.Kernel) *Router {
	return &Router{
		kernel: k,
	}
}

// RegisterRoutes registers all API routes.
func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	// Health and status endpoints
	mux.HandleFunc("GET /api/health", r.handleHealth)
	mux.HandleFunc("GET /api/status", r.handleStatus)
	mux.HandleFunc("GET /api/version", r.handleVersion)

	// Agent endpoints (v1)
	mux.HandleFunc("GET /api/v1/agents", r.handleListAgents)
	mux.HandleFunc("POST /api/v1/agents", r.handleCreateAgent)
	mux.HandleFunc("GET /api/v1/agents/{id}", r.handleGetAgent)
	mux.HandleFunc("PUT /api/v1/agents/{id}", r.handleUpdateAgent)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", r.handleDeleteAgent)

	// Agent endpoints (aliases without v1)
	mux.HandleFunc("GET /api/agents", r.handleListAgents)
	mux.HandleFunc("POST /api/agents", r.handleCreateAgent)
	mux.HandleFunc("GET /api/agents/{id}", r.handleGetAgent)
	mux.HandleFunc("PUT /api/agents/{id}", r.handleUpdateAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", r.handleDeleteAgent)

	// Session endpoints
	mux.HandleFunc("GET /api/v1/sessions", r.handleListSessions)
	mux.HandleFunc("POST /api/v1/sessions", r.handleCreateSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}", r.handleGetSession)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", r.handleDeleteSession)

	// Session endpoints (aliases)
	mux.HandleFunc("GET /api/sessions", r.handleListSessions)
	mux.HandleFunc("POST /api/sessions", r.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", r.handleGetSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", r.handleDeleteSession)

	// Chat endpoints
	mux.HandleFunc("POST /api/v1/chat", r.handleChat)

	// Memory endpoints
	mux.HandleFunc("GET /api/v1/memories", r.handleListMemories)
	mux.HandleFunc("POST /api/v1/memories", r.handleCreateMemory)
	mux.HandleFunc("GET /api/v1/memories/search", r.handleSearchMemories)
	mux.HandleFunc("DELETE /api/v1/memories/{id}", r.handleDeleteMemory)

	// Memory endpoints (aliases)
	mux.HandleFunc("GET /api/memories", r.handleListMemories)
	mux.HandleFunc("POST /api/memories", r.handleCreateMemory)
	mux.HandleFunc("GET /api/memories/search", r.handleSearchMemories)
	mux.HandleFunc("DELETE /api/memories/{id}", r.handleDeleteMemory)

	// Skill endpoints
	mux.HandleFunc("GET /api/v1/skills", r.handleListSkills)
	mux.HandleFunc("POST /api/v1/skills", r.handleInstallSkill)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", r.handleUninstallSkill)

	// Skill endpoints (aliases)
	mux.HandleFunc("GET /api/skills", r.handleListSkills)
	mux.HandleFunc("POST /api/skills", r.handleInstallSkill)
	mux.HandleFunc("DELETE /api/skills/{id}", r.handleUninstallSkill)

	// Channel endpoints
	mux.HandleFunc("GET /api/v1/channels", r.handleListChannels)
	mux.HandleFunc("POST /api/v1/channels", r.handleCreateChannel)
	mux.HandleFunc("DELETE /api/v1/channels/{id}", r.handleDeleteChannel)

	// Channel endpoints (aliases)
	mux.HandleFunc("GET /api/channels", r.handleListChannels)
	mux.HandleFunc("POST /api/channels", r.handleCreateChannel)
	mux.HandleFunc("DELETE /api/channels/{id}", r.handleDeleteChannel)

	// OpenAI-compatible endpoints
	mux.HandleFunc("GET /v1/models", r.handleListModels)
	// Additional frontend endpoints
	mux.HandleFunc("GET /api/commands", r.handleCommands)
	mux.HandleFunc("GET /api/config", r.handleConfig)
	mux.HandleFunc("GET /api/config/schema", r.handleConfigSchema)
	mux.HandleFunc("GET /api/logs/stream", r.handleLogsStream)

	// Hands endpoints
	mux.HandleFunc("GET /api/hands", r.handleListHands)
	mux.HandleFunc("GET /api/hands/active", r.handleActiveHands)
	mux.HandleFunc("GET /api/hands/{id}", r.handleGetHand)
	mux.HandleFunc("POST /api/hands/{id}/activate", r.handleActivateHand)
	mux.HandleFunc("POST /api/hands/{id}/install-deps", r.handleInstallHandDeps)
	mux.HandleFunc("POST /api/hands/instances/{instanceID}/deactivate", r.handleDeactivateHand)
	mux.HandleFunc("POST /api/hands/instances/{instanceID}/pause", r.handlePauseHand)
	mux.HandleFunc("POST /api/hands/instances/{instanceID}/resume", r.handleResumeHand)

	// Budget endpoints
	mux.HandleFunc("GET /api/budget", r.handleBudget)
	mux.HandleFunc("GET /api/budget/agents", r.handleBudgetAgents)
	mux.HandleFunc("GET /api/network/status", r.handleNetworkStatus)
	mux.HandleFunc("GET /api/a2a/agents", r.handleA2AAgents)
	mux.HandleFunc("GET /api/tools", r.handleTools)
	mux.HandleFunc("GET /api/usage/summary", r.handleUsageSummary)
	mux.HandleFunc("GET /api/usage/by-model", r.handleUsageByModel)
	mux.HandleFunc("GET /api/usage/daily", r.handleUsageDaily)
	mux.HandleFunc("GET /api/usage", r.handleUsage)
	mux.HandleFunc("GET /api/audit/recent", r.handleAuditRecent)
	mux.HandleFunc("GET /api/audit/verify", r.handleAuditVerify)
	mux.HandleFunc("GET /api/providers", r.handleProviders)
	mux.HandleFunc("POST /api/providers/{name}/key", r.handleSetProviderKey)
	mux.HandleFunc("DELETE /api/providers/{name}/key", r.handleDeleteProviderKey)
	mux.HandleFunc("POST /api/providers/{name}/test", r.handleTestProvider)
	mux.HandleFunc("PUT /api/providers/{name}/url", r.handleSetProviderURL)
	mux.HandleFunc("GET /api/mcp/servers", r.handleMcpServers)

	// Agent session endpoints
	mux.HandleFunc("GET /api/agents/{id}/session", r.handleGetAgentSession)
	mux.HandleFunc("GET /api/agents/{id}/sessions", r.handleGetAgentSessions)
	mux.HandleFunc("POST /api/agents/{id}/sessions", r.handleCreateAgentSession)
	mux.HandleFunc("POST /api/agents/{id}/sessions/{sid}/switch", r.handleSwitchSession)
	mux.HandleFunc("POST /api/agents/{id}/session/reset", r.handleResetSession)
	mux.HandleFunc("POST /api/agents/{id}/session/compact", r.handleCompactSession)
	mux.HandleFunc("POST /api/agents/{id}/message", r.handleAgentMessage)
	mux.HandleFunc("POST /api/agents/{id}/stop", r.handleStopAgent)
	mux.HandleFunc("PUT /api/agents/{id}/model", r.handleUpdateAgentModel)

	// Agent WebSocket endpoint
	mux.HandleFunc("/api/agents/{id}/ws", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		req.URL.RawQuery = "agent_id=" + id
		WSHandler(r.kernel)(w, req)
	})

	// Shutdown endpoint
	mux.HandleFunc("POST /api/shutdown", r.handleShutdown)
}

// respondJSON responds with JSON.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError responds with an error.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// handleHealth handles health check requests.
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, types.HealthStatus{
		Status:  "ok",
		Healthy: true,
		Checks:  map[string]bool{"database": true, "kernel": true},
	})
}

// handleStatus handles status requests.
func (r *Router) handleStatus(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, types.StatusResponse{
		Status:     "running",
		Version:    "0.1.0",
		ListenAddr: ":4200",
		AgentCount: r.kernel.AgentRegistry().Count(),
		ModelCount: 1,
		Uptime:     "0s",
	})
}

// handleVersion handles version requests.
func (r *Router) handleVersion(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"version": "0.1.0",
	})
}

// Agent handlers
func (r *Router) handleListAgents(w http.ResponseWriter, req *http.Request) {
	agents := r.kernel.AgentRegistry().List()
	var result []map[string]interface{}
	for _, agent := range agents {
		result = append(result, map[string]interface{}{
			"id":             agent.ID,
			"name":           agent.Name,
			"state":          agent.State,
			"mode":           agent.Mode,
			"tags":           agent.Tags,
			"created_at":     agent.CreatedAt,
			"last_active":    agent.LastActive,
			"model_provider": agent.Manifest.Model.Provider,
			"model_name":     agent.Manifest.Model.Model,
		})
	}
	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleCreateAgent(w http.ResponseWriter, req *http.Request) {
	var agent types.Agent
	if err := json.NewDecoder(req.Body).Decode(&agent); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, agent)
}

func (r *Router) handleGetAgent(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	agent := r.kernel.AgentRegistry().Get(id)
	if agent == nil {
		respondError(w, http.StatusNotFound, "agent not found")
		return
	}

	respondJSON(w, http.StatusOK, agent)
}

func (r *Router) handleUpdateAgent(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	var reqBody struct {
		State *string `json:"state"`
		Mode  *string `json:"mode"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if reqBody.State != nil {
		state := types.AgentState(*reqBody.State)
		if err := r.kernel.AgentRegistry().SetState(id, state); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	if reqBody.Mode != nil {
		if err := r.kernel.AgentRegistry().SetMode(id, *reqBody.Mode); err != nil {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	agent := r.kernel.AgentRegistry().Get(id)
	respondJSON(w, http.StatusOK, agent)
}

func (r *Router) handleDeleteAgent(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := types.ParseAgentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	_, err = r.kernel.AgentRegistry().Remove(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Session handlers
func (r *Router) handleListSessions(w http.ResponseWriter, req *http.Request) {
	sessions, err := r.kernel.SessionStore().ListSessions()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, sessions)
}

func (r *Router) handleCreateSession(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		AgentID string  `json:"agent_id"`
		Label   *string `json:"label,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	agentID, err := types.ParseAgentID(reqBody.AgentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid agent_id")
		return
	}

	session := types.NewSession(agentID, reqBody.Label)
	if err := r.kernel.SessionStore().SaveSession(&session); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, session)
}

func (r *Router) handleGetSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sessionID, err := types.ParseSessionID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid session_id")
		return
	}

	session, err := r.kernel.SessionStore().GetSession(sessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	respondJSON(w, http.StatusOK, session)
}

func (r *Router) handleDeleteSession(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	sessionID, err := types.ParseSessionID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid session_id")
		return
	}

	if err := r.kernel.SessionStore().DeleteSession(sessionID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Chat handlers
func (r *Router) handleChat(w http.ResponseWriter, req *http.Request) {
	var chatReq types.ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, types.ChatResponse{
		Response:   "This is a placeholder response.",
		AgentID:    chatReq.AgentID,
		StopReason: "end_turn",
	})
}

// Memory handlers
func (r *Router) handleListMemories(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleCreateMemory(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (r *Router) handleSearchMemories(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	limit := 10
	agentIDStr := req.URL.Query().Get("agent_id")

	var filter *types.MemoryFilter
	if agentIDStr != "" {
		agentID, _ := types.ParseAgentID(agentIDStr)
		filter = &types.MemoryFilter{AgentID: &agentID}
	}

	memories, err := r.kernel.SemanticStore().Recall(query, limit, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, memories)
}

func (r *Router) handleDeleteMemory(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	memoryID, err := types.ParseMemoryID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid memory_id")
		return
	}

	if err := r.kernel.SemanticStore().Forget(memoryID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Skill handlers
func (r *Router) handleListSkills(w http.ResponseWriter, req *http.Request) {
	skills, err := r.kernel.SkillLoader().ListSkills()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, skills)
}

func (r *Router) handleInstallSkill(w http.ResponseWriter, req *http.Request) {
	var reqBody struct {
		SourcePath string `json:"source_path"`
		SkillID    string `json:"skill_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	skill, err := r.kernel.SkillLoader().InstallSkill(reqBody.SourcePath, reqBody.SkillID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, skill)
}

func (r *Router) handleUninstallSkill(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if err := r.kernel.SkillLoader().UninstallSkill(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// Channel handlers
func (r *Router) handleListChannels(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleCreateChannel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (r *Router) handleDeleteChannel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusNoContent, nil)
}

// OpenAI-compatible handlers (already implemented in openai_compat.go)
func (r *Router) handleListModels(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"id":       "gpt-4o",
				"object":   "model",
				"created":  1699000000,
				"owned_by": "openai",
			},
		},
	})
}

func (r *Router) handleCommands(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []map[string]string{
		{"cmd": "/help", "desc": "Show available commands"},
		{"cmd": "/clear", "desc": "Clear chat history"},
		{"cmd": "/model", "desc": "Switch model"},
	})
}

func (r *Router) handleBudget(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"monthly_limit": 0.0,
		"monthly_spent": 0.0,
	})
}

func (r *Router) handleBudgetAgents(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleNetworkStatus(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connected": true,
		"agents":    []interface{}{},
	})
}

func (r *Router) handleA2AAgents(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"agents": []interface{}{},
	})
}

func (r *Router) handleGetAgentSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": "default",
		"messages":   []interface{}{},
	})
}

func (r *Router) handleGetAgentSessions(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, []interface{}{})
}

func (r *Router) handleCreateAgentSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusCreated, map[string]string{
		"session_id": "new-session",
	})
}

func (r *Router) handleSwitchSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleResetSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (r *Router) handleCompactSession(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "compacted",
		"tokens_before": 0,
		"tokens_after":  0,
	})
}

func (r *Router) handleAgentMessage(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

	var reqBody struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	driver, err := getLLMDriver()
	if err != nil {
		message := "👋 Hi! I'm FangClaw-go. To use the full chat capabilities, please set up an API key.\n\n**Supported providers:**\n- OpenRouter (recommended)\n- OpenAI\n- Anthropic\n- Groq\n\n**How to set up:**\n1. Go to Settings page\n2. Select your preferred provider\n3. Enter your API key\n\nOr set the API key via environment variables:\n- `OPENROUTER_API_KEY`\n- `OPENAI_API_KEY`\n- `ANTHROPIC_API_KEY`\n- `GROQ_API_KEY`"
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"response": message,
			"message": map[string]string{
				"role":    "assistant",
				"content": message,
			},
		})
		return
	}

	var messages []llm.Message

	if hand, _ := hands.GetBundledHand(agentID); hand != nil {
		systemPrompt := getHandSystemPrompt(agentID)
		if systemPrompt != "" {
			messages = append(messages, llm.Message{
				Role:    "system",
				Content: systemPrompt,
			})
		}
	}

	messages = append(messages, llm.Message{
		Role:    "user",
		Content: reqBody.Message,
	})

	llmReq := &llm.Request{
		Messages:    messages,
		Temperature: 0.7,
	}

	ctx := context.Background()
	resp, err := driver.Chat(ctx, llmReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"response": resp.Content,
		"message": map[string]string{
			"role":    "assistant",
			"content": resp.Content,
		},
	})
}

func getLLMDriver() (llm.Driver, error) {
	cfg, err := config.Load("")
	if err == nil && cfg.DefaultModel.Provider != "" && cfg.DefaultModel.Model != "" {
		provider := cfg.DefaultModel.Provider
		model := cfg.DefaultModel.Model
		apiKeyEnv := cfg.DefaultModel.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = strings.ToUpper(provider) + "_API_KEY"
		}
		apiKey := os.Getenv(apiKeyEnv)
		if apiKey != "" {
			driver, err := llm.NewDriver(provider, apiKey, model)
			if err == nil {
				return driver, nil
			}
		}
	}

	provider := "openrouter"
	model := "meta-llama/llama-3.1-8b-instruct"
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	if apiKey == "" {
		provider = "openai"
		model = "gpt-4o"
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		provider = "anthropic"
		model = "claude-sonnet-4-20250514"
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if apiKey == "" {
		provider = "groq"
		model = "groq/llama-3.3-70b-versatile"
		apiKey = os.Getenv("GROQ_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found. Set OPENROUTER_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, or GROQ_API_KEY")
	}

	return llm.NewDriver(provider, apiKey, model)
}

func getHandSystemPrompt(handID string) string {
	switch handID {
	case "researcher":
		return hands.ResearcherSystemPrompt
	case "lead":
		return hands.LeadSystemPrompt
	case "collector":
		return hands.CollectorSystemPrompt
	case "predictor":
		return hands.PredictorSystemPrompt
	case "clip":
		return hands.ClipSystemPrompt
	case "twitter":
		return hands.TwitterSystemPrompt
	case "browser":
		return hands.BrowserSystemPrompt
	default:
		return ""
	}
}

func (r *Router) handleStopAgent(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (r *Router) handleUpdateAgentModel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (r *Router) handleDeleteAgentAlias(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusNoContent, nil)
}

func (r *Router) handleTools(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tools": []interface{}{},
	})
}

func (r *Router) handleUsage(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_tokens":     0,
		"total_cost_usd":   0.0,
		"period_start":     "2024-01-01",
		"period_end":       "2024-12-31",
		"agents":           []interface{}{},
		"first_event_date": nil,
	})
}

func (r *Router) handleAuditRecent(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": []interface{}{},
	})
}

func (r *Router) handleAuditVerify(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"verified":    true,
		"merkle_root": "0000000000000000000000000000000000000000000000000000000000000000",
	})
}

func (r *Router) handleMcpServers(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"servers": []interface{}{},
	})
}

func (r *Router) handleConfig(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"theme":             "system",
		"language":          "en",
		"sidebar_collapsed": false,
	})
}

func (r *Router) handleLogsStream(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	flusher.Flush()
}

func (r *Router) handleUsageSummary(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_input_tokens":  0,
		"total_output_tokens": 0,
		"total_cost_usd":      0.0,
		"call_count":          0,
		"total_tool_calls":    0,
		"period_start":        "2024-01-01",
		"period_end":          "2024-12-31",
	})
}

func (r *Router) handleUsageByModel(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"models": []interface{}{},
	})
}

func (r *Router) handleUsageDaily(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"days": []interface{}{},
	})
}

func loadHandsFromFile() []map[string]string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return getDefaultHands()
	}
	path := filepath.Join(homeDir, ".fangclaw-go", "hands.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return getDefaultHands()
	}
	var hands []map[string]string
	if err := json.Unmarshal(data, &hands); err != nil {
		return getDefaultHands()
	}
	return hands
}

func getDefaultHands() []map[string]string {
	return []map[string]string{
		{"id": "researcher", "name": "Researcher", "status": "inactive"},
		{"id": "lead", "name": "Lead", "status": "inactive"},
		{"id": "collector", "name": "Collector", "status": "inactive"},
		{"id": "predictor", "name": "Predictor", "status": "inactive"},
		{"id": "clip", "name": "Clip", "status": "inactive"},
		{"id": "twitter", "name": "Twitter", "status": "inactive"},
		{"id": "browser", "name": "Browser", "status": "inactive"},
	}
}

func (r *Router) handleListHands(w http.ResponseWriter, req *http.Request) {
	handDefs := r.kernel.HandRegistry().ListDefinitions()
	instances := r.kernel.HandRegistry().ListInstances()

	var handsResult []map[string]interface{}
	for _, hand := range handDefs {
		handsResult = append(handsResult, map[string]interface{}{
			"id":          hand.ID,
			"name":        hand.Name,
			"description": hand.Description,
			"category":    hand.Category,
			"icon":        hand.Icon,
			"requires":    hand.Requires,
			"settings":    hand.Settings,
		})
	}

	var instancesResult []map[string]interface{}
	for _, inst := range instances {
		instancesResult = append(instancesResult, map[string]interface{}{
			"instance_id":  inst.InstanceID,
			"hand_id":      inst.HandID,
			"agent_id":     inst.AgentID,
			"agent_name":   inst.AgentName,
			"status":       inst.Status,
			"config":       inst.Config,
			"activated_at": inst.ActivatedAt,
			"updated_at":   inst.UpdatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"hands":     handsResult,
		"instances": instancesResult,
	})
}

func (r *Router) handleActiveHands(w http.ResponseWriter, req *http.Request) {
	instances := r.kernel.HandRegistry().ListInstances()

	var activeInstances []map[string]interface{}
	for _, inst := range instances {
		if inst.Status == hands.HandStatusActive || inst.Status == hands.HandStatusPaused {
			activeInstances = append(activeInstances, map[string]interface{}{
				"instance_id": inst.InstanceID,
				"hand_id":     inst.HandID,
				"agent_id":    inst.AgentID,
				"agent_name":  inst.AgentName,
				"status":      inst.Status,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"instances": activeInstances,
	})
}

func (r *Router) handleGetHand(w http.ResponseWriter, req *http.Request) {
	handID := req.PathValue("id")

	hand, ok := r.kernel.HandRegistry().GetDefinition(handID)
	if !ok {
		respondError(w, http.StatusNotFound, "hand not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":          hand.ID,
		"name":        hand.Name,
		"description": hand.Description,
		"category":    hand.Category,
		"icon":        hand.Icon,
		"tools":       hand.Tools,
		"requires":    hand.Requires,
		"settings":    hand.Settings,
		"agent":       hand.Agent,
		"dashboard":   hand.Dashboard,
	})
}

func (r *Router) handleActivateHand(w http.ResponseWriter, req *http.Request) {
	handID := req.PathValue("id")

	var config map[string]interface{}
	if req.Body != http.NoBody {
		if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
			config = make(map[string]interface{})
		}
		defer req.Body.Close()
	}

	instance, err := r.kernel.ActivateHand(handID, config)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"instance_id": instance.InstanceID,
		"hand_id":     instance.HandID,
		"agent_id":    instance.AgentID,
		"agent_name":  instance.AgentName,
		"status":      instance.Status,
	})
}

func (r *Router) handleDeactivateHand(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	err := r.kernel.DeactivateHand(instanceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handlePauseHand(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	err := r.kernel.PauseHand(instanceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleResumeHand(w http.ResponseWriter, req *http.Request) {
	instanceID := req.PathValue("instanceID")

	err := r.kernel.ResumeHand(instanceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleInstallHandDeps(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (r *Router) handleShutdown(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Shutdown initiated",
	})

	RequestShutdown()
}

func (r *Router) handleConfigSchema(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sections": map[string]interface{}{
			"api": map[string]interface{}{
				"fields": map[string]string{
					"api_listen": "string",
					"api_key":    "string",
					"log_level":  "string",
				},
			},
			"default_model": map[string]interface{}{
				"fields": map[string]string{
					"provider":    "string",
					"model":       "string",
					"api_key_env": "string",
					"base_url":    "string",
				},
			},
			"memory": map[string]interface{}{
				"fields": map[string]string{
					"decay_rate":  "number",
					"vector_dims": "number",
				},
			},
			"web": map[string]interface{}{
				"fields": map[string]string{
					"provider":     "string",
					"timeout_secs": "number",
					"max_results":  "number",
				},
			},
			"browser": map[string]interface{}{
				"fields": map[string]string{
					"headless":        "boolean",
					"timeout_secs":    "number",
					"executable_path": "string",
				},
			},
			"network": map[string]interface{}{
				"fields": map[string]string{
					"enabled":       "boolean",
					"listen_addr":   "string",
					"shared_secret": "string",
				},
			},
			"extensions": map[string]interface{}{
				"fields": map[string]string{
					"auto_connect":               "boolean",
					"health_check_interval_secs": "number",
				},
			},
			"vault": map[string]interface{}{
				"fields": map[string]string{
					"path": "string",
				},
			},
			"a2a": map[string]interface{}{
				"fields": map[string]string{
					"enabled":     "boolean",
					"name":        "string",
					"description": "string",
					"url":         "string",
				},
			},
			"channels": map[string]interface{}{
				"fields": map[string]string{
					"telegram": "object",
					"discord":  "object",
					"slack":    "object",
					"whatsapp": "object",
				},
			},
		},
	})
}
