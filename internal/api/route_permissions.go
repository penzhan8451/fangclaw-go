package api

import (
	"net/http"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
)

type RoutePermission struct {
	Path        string
	Method      string
	Permission  auth.Permission
	RequireAuth bool
}

var RoutePermissions = []RoutePermission{
	{Path: "/api/agents", Method: http.MethodPost, Permission: auth.PermAgentCreate, RequireAuth: true},
	{Path: "/api/agents", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/agents/*", Method: http.MethodPut, Permission: auth.PermAgentWrite, RequireAuth: true},
	{Path: "/api/agents/*", Method: http.MethodPatch, Permission: auth.PermAgentWrite, RequireAuth: true},
	{Path: "/api/v1/agents", Method: http.MethodPost, Permission: auth.PermAgentCreate, RequireAuth: true},
	{Path: "/api/v1/agents", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/v1/agents/*", Method: http.MethodPut, Permission: auth.PermAgentWrite, RequireAuth: true},

	{Path: "/api/sessions", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/sessions", Method: http.MethodPost, Permission: auth.PermAgentWrite, RequireAuth: true},
	{Path: "/api/v1/sessions", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/v1/sessions", Method: http.MethodPost, Permission: auth.PermAgentWrite, RequireAuth: true},

	{Path: "/api/memories", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/memories", Method: http.MethodPost, Permission: auth.PermAgentWrite, RequireAuth: true},
	{Path: "/api/v1/memories", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/v1/memories", Method: http.MethodPost, Permission: auth.PermAgentWrite, RequireAuth: true},

	{Path: "/api/skills", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/skills/uninstall", Method: http.MethodPost, Permission: auth.PermSkillUninstall, RequireAuth: true},
	{Path: "/api/skills/create", Method: http.MethodPost, Permission: auth.PermSkillInstall, RequireAuth: true},
	{Path: "/api/skills/install", Method: http.MethodPost, Permission: auth.PermSkillInstall, RequireAuth: true},
	{Path: "/api/skills/*", Method: http.MethodDelete, Permission: auth.PermSkillUninstall, RequireAuth: true},

	{Path: "/api/channels", Method: http.MethodGet, Permission: auth.PermChannelRead, RequireAuth: true},
	{Path: "/api/channels", Method: http.MethodPost, Permission: auth.PermChannelWrite, RequireAuth: true},

	{Path: "/api/config", Method: http.MethodGet, Permission: auth.PermConfigRead, RequireAuth: true},
	{Path: "/api/config", Method: http.MethodPut, Permission: auth.PermConfigWrite, RequireAuth: true},

	{Path: "/api/hands", Method: http.MethodGet, Permission: auth.PermAgentRead, RequireAuth: true},
	{Path: "/api/hands/*", Method: http.MethodPost, Permission: auth.PermHandActivate, RequireAuth: true},
	{Path: "/api/hands/instances/*", Method: http.MethodPost, Permission: auth.PermHandDeactivate, RequireAuth: true},
	{Path: "/api/hands/instances/*", Method: http.MethodDelete, Permission: auth.PermHandDeactivate, RequireAuth: true},

	{Path: "/api/mcp", Method: http.MethodGet, Permission: auth.PermMCPRead, RequireAuth: true},
	{Path: "/api/mcp", Method: http.MethodPost, Permission: auth.PermMCPWrite, RequireAuth: true},

	{Path: "/api/audit", Method: http.MethodGet, Permission: auth.PermAuditRead, RequireAuth: true},

	{Path: "/api/budget", Method: http.MethodGet, Permission: auth.PermBudgetRead, RequireAuth: true},
	{Path: "/api/budget", Method: http.MethodPut, Permission: auth.PermBudgetWrite, RequireAuth: true},

	{Path: "/api/users", Method: http.MethodGet, Permission: auth.PermUserManage, RequireAuth: true},
	{Path: "/api/users", Method: http.MethodPost, Permission: auth.PermUserManage, RequireAuth: true},

	{Path: "/api/shutdown", Method: http.MethodPost, Permission: auth.PermShutdown, RequireAuth: true},
}

func CheckRoutePermission(r *http.Request) (bool, bool) {
	user := GetUserFromContext(r.Context())
	path := r.URL.Path
	method := r.Method

	for _, rp := range RoutePermissions {
		if matchesPath(rp.Path, path) && rp.Method == method {
			if !rp.RequireAuth {
				return true, true
			}

			if user == nil {
				return false, false
			}

			return HasPermission(user, rp.Permission), true
		}
	}

	return true, false
}

func matchesPath(pattern, path string) bool {
	if pattern == path {
		return true
	}

	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

func PermissionAwareHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowed, needsAuth := CheckRoutePermission(r)

		if needsAuth && !allowed {
			user := GetUserFromContext(r.Context())
			if user == nil {
				respondUnauthorized(w, "Authentication required")
				return
			}

			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error": "Permission denied",
			})
			return
		}

		handler(w, r)
	}
}

func RequireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			respondUnauthorized(w, "Authentication required")
			return
		}

		handler(w, r)
	}
}

func RequirePermissionForHandler(permission auth.Permission, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			respondUnauthorized(w, "Authentication required")
			return
		}

		if !HasPermission(user, permission) {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error":      "Permission denied",
				"permission": string(permission),
			})
			return
		}

		handler(w, r)
	}
}

func RequireAdminForHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			respondUnauthorized(w, "Authentication required")
			return
		}

		if !IsAdmin(user) {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error": "Admin access required",
			})
			return
		}

		handler(w, r)
	}
}

func RequireOwnerForHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			respondUnauthorized(w, "Authentication required")
			return
		}

		if !IsOwner(user) {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error": "Owner access required",
			})
			return
		}

		handler(w, r)
	}
}
