package api

import (
	"fmt"
	"net/http"
	"strings"
)

func (r *Router) handleListAgentTemplates(w http.ResponseWriter, req *http.Request) {
	templates := r.kernel.AgentTemplates().ListTemplates()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

func (r *Router) handleGetAgentTemplate(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	template := r.kernel.AgentTemplates().GetTemplate(id)

	if template == nil {
		respondError(w, http.StatusNotFound, "Template not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"template": template,
	})
}

func (r *Router) handleSpawnAgentFromTemplate(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	fmt.Println("Spawning agent from template-->:", id)
	template := r.kernel.AgentTemplates().GetTemplate(id)

	if template == nil {
		respondError(w, http.StatusNotFound, "Template not found")
		return
	}

	manifest := template.ToAgentManifest()
	agentID, agentName, err := r.kernel.SpawnAgent(manifest)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondError(w, http.StatusConflict, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to spawn agent: "+err.Error())
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"agent_id": agentID,
		"name":     agentName,
	})
}
