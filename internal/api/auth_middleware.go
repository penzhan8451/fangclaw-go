package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
	UsernameKey contextKey = "username"
)

func AuthMiddleware(authManager *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractTokenFromRequest(r)
			if token == "" {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			user, err := authManager.ValidateToken(token)
			if err != nil {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, user.ID)
			ctx = context.WithValue(ctx, UserRoleKey, string(user.Role))
			ctx = context.WithValue(ctx, UsernameKey, user.Username)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(authManager *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractTokenFromRequest(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			user, err := authManager.ValidateToken(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, user.ID)
			ctx = context.WithValue(ctx, UserRoleKey, string(user.Role))
			ctx = context.WithValue(ctx, UsernameKey, user.Username)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := r.Context().Value(UserRoleKey)
			if role == nil {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			roleStr := role.(string)
			allowed := false
			for _, allowedRole := range roles {
				if roleStr == allowedRole {
					allowed = true
					break
				}
			}

			if !allowed {
				WriteJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequirePermission(authManager *auth.AuthManager, permission auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value(UserIDKey)
			if userID == nil {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			hasPermission, err := authManager.HasPermission(userID.(string), permission)
			if err != nil {
				WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check permissions"})
				return
			}

			if !hasPermission {
				WriteJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func APIKeyAuthMiddleware(authManager *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "API key required"})
				return
			}

			user, err := authManager.AuthenticateAPIKey(apiKey)
			if err != nil {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, user.ID)
			ctx = context.WithValue(ctx, UserRoleKey, string(user.Role))
			ctx = context.WithValue(ctx, UsernameKey, user.Username)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractTokenFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")

	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	cookie, err := r.Cookie("session")
	if err == nil {
		return cookie.Value
	}

	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	return ""
}

func GetUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

func GetUserRoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value(UserRoleKey).(string); ok {
		return role
	}
	return ""
}

func GetUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(UsernameKey).(string); ok {
		return username
	}
	return ""
}
