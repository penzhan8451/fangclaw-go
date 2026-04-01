package api

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/auth"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

func CanAccessAgent(user *auth.User, agentID string, k *kernel.Kernel) bool {
	if user == nil {
		return false
	}

	if IsAdmin(user) {
		return true
	}

	aid, err := uuid.Parse(agentID)
	if err != nil {
		return false
	}

	agent := k.AgentRegistry().Get(aid)
	if agent == nil {
		return false
	}

	return true
}

func CanModifyAgent(user *auth.User, agentID string, k *kernel.Kernel) bool {
	if user == nil {
		return false
	}

	if IsAdmin(user) {
		return true
	}

	if !HasPermission(user, auth.PermAgentWrite) {
		return false
	}

	return true
}

func CanDeleteAgent(user *auth.User, agentID string, k *kernel.Kernel) bool {
	if user == nil {
		return false
	}

	if IsAdmin(user) {
		return true
	}

	if !HasPermission(user, auth.PermAgentDelete) {
		return false
	}

	return true
}

func CanAccessSession(user *auth.User, sessionID string, k *kernel.Kernel) bool {
	if user == nil {
		return false
	}

	if IsAdmin(user) {
		return true
	}

	sid, err := types.ParseSessionID(sessionID)
	if err != nil {
		return false
	}

	session, err := k.SessionStore().GetSession(sid)
	if err != nil || session == nil {
		return false
	}

	return true
}

func CanModifySession(user *auth.User, sessionID string, k *kernel.Kernel) bool {
	if user == nil {
		return false
	}

	if IsAdmin(user) {
		return true
	}

	if !HasPermission(user, auth.PermAgentWrite) {
		return false
	}

	return true
}

func RequireAgentAccess(user *auth.User, agentID string, k *kernel.Kernel, w http.ResponseWriter) bool {
	if !CanAccessAgent(user, agentID, k) {
		if user == nil {
			respondUnauthorized(w, "Authentication required")
		} else {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error":    "Access denied to this agent",
				"agent_id": agentID,
			})
		}
		return false
	}
	return true
}

func RequireAgentModify(user *auth.User, agentID string, k *kernel.Kernel, w http.ResponseWriter) bool {
	if !CanModifyAgent(user, agentID, k) {
		if user == nil {
			respondUnauthorized(w, "Authentication required")
		} else {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error":    "Cannot modify this agent",
				"agent_id": agentID,
			})
		}
		return false
	}
	return true
}

func RequireSessionAccess(user *auth.User, sessionID string, k *kernel.Kernel, w http.ResponseWriter) bool {
	if !CanAccessSession(user, sessionID, k) {
		if user == nil {
			respondUnauthorized(w, "Authentication required")
		} else {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error":     "Access denied to this session",
				"session_id": sessionID,
			})
		}
		return false
	}
	return true
}

type ResourceOwnerChecker struct {
	kernel *kernel.Kernel
}

func NewResourceOwnerChecker(k *kernel.Kernel) *ResourceOwnerChecker {
	return &ResourceOwnerChecker{kernel: k}
}

func (c *ResourceOwnerChecker) CheckAgentOwnership(user *auth.User, agentID string) (bool, error) {
	if IsAdmin(user) {
		return true, nil
	}

	aid, err := uuid.Parse(agentID)
	if err != nil {
		return false, err
	}

	agent := c.kernel.AgentRegistry().Get(aid)
	if agent == nil {
		return false, nil
	}

	return true, nil
}

func (c *ResourceOwnerChecker) CheckSessionOwnership(user *auth.User, sessionID string) (bool, error) {
	if IsAdmin(user) {
		return true, nil
	}

	sid, err := types.ParseSessionID(sessionID)
	if err != nil {
		return false, err
	}

	session, err := c.kernel.SessionStore().GetSession(sid)
	if err != nil {
		return false, err
	}
	if session == nil {
		return false, nil
	}

	return true, nil
}
