package api

import (
	"context"
	"net/http"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
)

const UserKey contextKey = "user"

func SetUserInContext(r *http.Request, user *auth.User) *http.Request {
	ctx := context.WithValue(r.Context(), UserKey, user)
	return r.WithContext(ctx)
}

func GetUserFromContext(ctx context.Context) *auth.User {
	if user, ok := ctx.Value(UserKey).(*auth.User); ok {
		return user
	}
	return nil
}

func HasPermission(user *auth.User, permission auth.Permission) bool {
	if user == nil {
		return false
	}

	permissions, ok := auth.RolePermissions[user.Role]
	if !ok {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func HasAnyPermission(user *auth.User, permissions ...auth.Permission) bool {
	if user == nil {
		return false
	}

	for _, perm := range permissions {
		if HasPermission(user, perm) {
			return true
		}
	}
	return false
}

func HasAllPermissions(user *auth.User, permissions ...auth.Permission) bool {
	if user == nil {
		return false
	}

	for _, perm := range permissions {
		if !HasPermission(user, perm) {
			return false
		}
	}
	return true
}

func RequirePermissionMiddleware(permission auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAnyPermissionMiddleware(permissions ...auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				respondUnauthorized(w, "Authentication required")
				return
			}

			if !HasAnyPermission(user, permissions...) {
				WriteJSON(w, http.StatusForbidden, map[string]string{
					"error": "Permission denied",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireRoleMiddleware(roles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user == nil {
				respondUnauthorized(w, "Authentication required")
				return
			}

			roleAllowed := false
			for _, role := range roles {
				if user.Role == role {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				WriteJSON(w, http.StatusForbidden, map[string]string{
					"error": "Insufficient role",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireOwnerMiddleware() func(http.Handler) http.Handler {
	return RequireRoleMiddleware(auth.RoleOwner)
}

func RequireAdminMiddleware() func(http.Handler) http.Handler {
	return RequireRoleMiddleware(auth.RoleOwner, auth.RoleAdmin)
}

func IsOwner(user *auth.User) bool {
	return user != nil && user.Role == auth.RoleOwner
}

func IsAdmin(user *auth.User) bool {
	return user != nil && (user.Role == auth.RoleOwner || user.Role == auth.RoleAdmin)
}

func CanManageUser(actor *auth.User, target *auth.User) bool {
	if actor == nil || target == nil {
		return false
	}

	if actor.Role == auth.RoleOwner {
		return true
	}

	if actor.Role == auth.RoleAdmin && target.Role != auth.RoleOwner {
		return true
	}

	return false
}

func CanModifyRole(actor *auth.User, targetRole auth.Role) bool {
	if actor == nil {
		return false
	}

	if actor.Role == auth.RoleOwner {
		return true
	}

	if actor.Role == auth.RoleAdmin && targetRole != auth.RoleOwner {
		return true
	}

	return false
}

type PermissionCheck func(*auth.User) bool

func WithPermissionCheck(check PermissionCheck, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			respondUnauthorized(w, "Authentication required")
			return
		}

		if !check(user) {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error": "Permission denied",
			})
			return
		}

		handler(w, r)
	}
}
