// Package api implements the REST API server for OpenFang.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/penzhan8451/fangclaw-go/internal/kernel"
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

	// Agent endpoints
	mux.HandleFunc("GET /api/v1/agents", r.handleListAgents)
	mux.HandleFunc("POST /api/v1/agents", r.handleCreateAgent)
	mux.HandleFunc("GET /api/v1/agents/{id}", r.handleGetAgent)
	mux.HandleFunc("PUT /api/v1/agents/{id}", r.handleUpdateAgent)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", r.handleDeleteAgent)

	// Session endpoints
	mux.HandleFunc("GET /api/v1/sessions", r.handleListSessions)
	mux.HandleFunc("POST /api/v1/sessions", r.handleCreateSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}", r.handleGetSession)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", r.handleDeleteSession)

	// Chat endpoints
	mux.HandleFunc("POST /api/v1/chat", r.handleChat)

	// Memory endpoints
	mux.HandleFunc("GET /api/v1/memories", r.handleListMemories)
	mux.HandleFunc("POST /api/v1/memories", r.handleCreateMemory)
	mux.HandleFunc("GET /api/v1/memories/search", r.handleSearchMemories)
	mux.HandleFunc("DELETE /api/v1/memories/{id}", r.handleDeleteMemory)

	// Skill endpoints
	mux.HandleFunc("GET /api/v1/skills", r.handleListSkills)
	mux.HandleFunc("POST /api/v1/skills", r.handleInstallSkill)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", r.handleUninstallSkill)

	// Channel endpoints
	mux.HandleFunc("GET /api/v1/channels", r.handleListChannels)
	mux.HandleFunc("POST /api/v1/channels", r.handleCreateChannel)
	mux.HandleFunc("DELETE /api/v1/channels/{id}", r.handleDeleteChannel)

	// OpenAI-compatible endpoints
	mux.HandleFunc("GET /v1/models", r.handleListModels)
	mux.HandleFunc("POST /v1/chat/completions", r.handleChatCompletions)
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
		AgentCount: 0,
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
	respondJSON(w, http.StatusOK, []interface{}{})
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
	id := req.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (r *Router) handleUpdateAgent(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (r *Router) handleDeleteAgent(w http.ResponseWriter, req *http.Request) {
	_ = req.PathValue("id")
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

func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	// This is handled by the existing openai_compat.go implementation
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
