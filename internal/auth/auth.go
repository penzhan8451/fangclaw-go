// Package auth provides RBAC authentication and authorization.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Role represents a user role.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

// Permission represents a specific permission.
type Permission string

const (
	PermAgentCreate    Permission = "agent:create"
	PermAgentRead      Permission = "agent:read"
	PermAgentWrite     Permission = "agent:write"
	PermAgentDelete    Permission = "agent:delete"
	PermConfigRead     Permission = "config:read"
	PermConfigWrite    Permission = "config:write"
	PermChannelRead    Permission = "channel:read"
	PermChannelWrite   Permission = "channel:write"
	PermSkillInstall   Permission = "skill:install"
	PermSkillUninstall Permission = "skill:uninstall"
	PermHandActivate   Permission = "hand:activate"
	PermHandDeactivate Permission = "hand:deactivate"
	PermMCPRead        Permission = "mcp:read"
	PermMCPWrite       Permission = "mcp:write"
	PermAuditRead      Permission = "audit:read"
	PermBudgetRead     Permission = "budget:read"
	PermBudgetWrite    Permission = "budget:write"
	PermAPIKeyWrite    Permission = "apikey:write"
	PermShutdown       Permission = "system:shutdown"
)

// RolePermissions defines permissions for each role.
var RolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermAgentCreate, PermAgentRead, PermAgentWrite, PermAgentDelete,
		PermConfigRead, PermConfigWrite,
		PermChannelRead, PermChannelWrite,
		PermSkillInstall, PermSkillUninstall,
		PermHandActivate, PermHandDeactivate,
		PermMCPRead, PermMCPWrite,
		PermAuditRead, PermBudgetRead, PermBudgetWrite,
		PermAPIKeyWrite, PermShutdown,
	},
	RoleUser: {
		PermAgentCreate, PermAgentRead, PermAgentWrite,
		PermChannelRead, PermChannelWrite,
		PermSkillInstall,
		PermHandActivate,
		PermMCPRead, PermBudgetRead,
	},
	RoleGuest: {
		PermAgentRead, PermChannelRead,
	},
}

// User represents a user in the system.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash,omitempty"`
	Role         Role      `json:"role"`
	APIKeys      []string  `json:"api_keys,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json   :"last_login,omitempty"`
	Disabled     bool      `json:"disabled"`
}

// AuthManager manages authentication and authorization.
type AuthManager struct {
	mu            sync.RWMutex
	users         map[string]*User
	apiKeyUsers   map[string]string // apiKey -> userID
	sessionTokens map[string]string // token -> userID
}

// NewAuthManager creates a new auth manager.
func NewAuthManager() *AuthManager {
	return &AuthManager{
		users:         make(map[string]*User),
		apiKeyUsers:   make(map[string]string),
		sessionTokens: make(map[string]string),
	}
}

// CreateUser creates a new user.
func (am *AuthManager) CreateUser(username string, password string, role Role) (*User, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.users[username]; exists {
		return nil, &AuthError{Message: "user already exists"}
	}

	user := &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: hashPassword(password),
		Role:         role,
		CreatedAt:    time.Now(),
	}

	am.users[username] = user
	return user, nil
}

// AuthenticateUser authenticates a user with username/password.
func (am *AuthManager) AuthenticateUser(username, password string) (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, ok := am.users[username]
	if !ok {
		return "", &AuthError{Message: "invalid credentials"}
	}

	if user.Disabled {
		return "", &AuthError{Message: "account disabled"}
	}

	if user.PasswordHash != hashPassword(password) {
		return "", &AuthError{Message: "invalid credentials"}
	}

	user.LastLogin = time.Now()

	// Generate session token
	token := generateToken()
	am.sessionTokens[token] = username

	return token, nil
}

// AuthenticateAPIKey authenticates using an API key.
func (am *AuthManager) AuthenticateAPIKey(apiKey string) (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	username, ok := am.apiKeyUsers[apiKey]
	if !ok {
		return "", &AuthError{Message: "invalid API key"}
	}

	user, ok := am.users[username]
	if !ok || user.Disabled {
		return "", &AuthError{Message: "invalid API key"}
	}

	return username, nil
}

// AddAPIKey adds an API key for a user.
func (am *AuthManager) AddAPIKey(username, apiKey string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, ok := am.users[username]
	if !ok {
		return &AuthError{Message: "user not found"}
	}

	user.APIKeys = append(user.APIKeys, apiKey)
	am.apiKeyUsers[apiKey] = username

	return nil
}

// RemoveAPIKey removes an API key.
func (am *AuthManager) RemoveAPIKey(apiKey string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	username, ok := am.apiKeyUsers[apiKey]
	if !ok {
		return &AuthError{Message: "API key not found"}
	}

	delete(am.apiKeyUsers, apiKey)

	if user, ok := am.users[username]; ok {
		for i, key := range user.APIKeys {
			if key == apiKey {
				user.APIKeys = append(user.APIKeys[:i], user.APIKeys[i+1:]...)
				break
			}
		}
	}

	return nil
}

// HasPermission checks if a user has a specific permission.
func (am *AuthManager) HasPermission(username string, permission Permission) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, ok := am.users[username]
	if !ok || user.Disabled {
		return false
	}

	perms, ok := RolePermissions[user.Role]
	if !ok {
		return false
	}

	for _, p := range perms {
		if p == permission {
			return true
		}
	}

	return false
}

// ValidateToken validates a session token.
func (am *AuthManager) ValidateToken(token string) (string, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	username, ok := am.sessionTokens[token]
	if !ok {
		return "", &AuthError{Message: "invalid token"}
	}

	user, ok := am.users[username]
	if !ok || user.Disabled {
		return "", &AuthError{Message: "invalid token"}
	}

	return username, nil
}

// RevokeToken revokes a session token.
func (am *AuthManager) RevokeToken(token string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.sessionTokens, token)
}

// GetUser returns a user by username.
func (am *AuthManager) GetUser(username string) (*User, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, ok := am.users[username]
	return user, ok
}

// ListUsers returns all users.
func (am *AuthManager) ListUsers() []*User {
	am.mu.RLock()
	defer am.mu.RUnlock()

	users := make([]*User, 0, len(am.users))
	for _, u := range am.users {
		users = append(users, u)
	}
	return users
}

// DisableUser disables a user account.
func (am *AuthManager) DisableUser(username string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, ok := am.users[username]
	if !ok {
		return &AuthError{Message: "user not found"}
	}

	user.Disabled = true
	return nil
}

// EnableUser enables a user account.
func (am *AuthManager) EnableUser(username string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, ok := am.users[username]
	if !ok {
		return &AuthError{Message: "user not found"}
	}

	user.Disabled = false
	return nil
}

// Helper functions
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateToken() string {
	return uuid.New().String()
}

// AuthError represents an authentication error.
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
