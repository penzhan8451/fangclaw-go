package api

import (
	"net/http"
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
