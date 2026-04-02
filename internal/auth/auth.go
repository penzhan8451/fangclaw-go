package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

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
	PermUserManage     Permission = "user:manage"
)

var RolePermissions = map[Role][]Permission{
	RoleOwner: {
		PermAgentCreate, PermAgentRead, PermAgentWrite, PermAgentDelete,
		PermConfigRead, PermConfigWrite,
		PermChannelRead, PermChannelWrite,
		PermSkillInstall, PermSkillUninstall,
		PermHandActivate, PermHandDeactivate,
		PermMCPRead, PermMCPWrite,
		PermAuditRead, PermBudgetRead, PermBudgetWrite,
		PermAPIKeyWrite, PermShutdown, PermUserManage,
	},
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
		PermHandActivate, PermHandDeactivate,
		PermMCPRead, PermBudgetRead,
	},
	RoleGuest: {
		PermAgentRead, PermChannelRead,
	},
}

type User struct {
	ID              string                 `json:"id"`
	Username        string                 `json:"username"`
	Email           string                 `json:"email,omitempty"`
	PasswordHash    string                 `json:"-"`
	Role            Role                   `json:"role"`
	APIKeys         []string               `json:"api_keys,omitempty"`
	ChannelBindings map[string]string      `json:"channel_bindings,omitempty"`
	Settings        map[string]interface{} `json:"settings,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	LastLogin       *time.Time             `json:"last_login,omitempty"`
	LastActivityAt  *time.Time             `json:"last_activity_at,omitempty"`
	Disabled        bool                   `json:"disabled"`
	IsVIP           bool                   `json:"is_vip"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

type AuthDB struct {
	*sql.DB
	Path string
	mu   sync.RWMutex
}

type AuthManager struct {
	db                *AuthDB
	sessionTokens     map[string]string
	mu                sync.RWMutex
	sessionTTL        time.Duration
	githubOAuthState  map[string]*GitHubOAuthStateData
	githubOAuthConfig *GitHubOAuthConfigData
}

type GitHubOAuthStateData struct {
	State        string
	PKCEVerifier string
	RedirectURL  string
	CreatedAt    time.Time
}

type GitHubOAuthConfigData struct {
	ClientID     string
	ClientSecret string
	Enabled      bool
}

func NewAuthDB(path string) (*AuthDB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create auth db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open auth database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping auth database: %w", err)
	}

	authDB := &AuthDB{DB: db, Path: path}
	if err := authDB.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate auth database: %w", err)
	}

	return authDB, nil
}

func (db *AuthDB) Migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			api_keys TEXT DEFAULT '[]',
			channel_bindings TEXT DEFAULT '{}',
			settings TEXT DEFAULT '{}',
			created_at TEXT NOT NULL,
			last_login TEXT,
			last_activity_at TEXT,
			disabled INTEGER DEFAULT 0,
			is_vip INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token TEXT UNIQUE NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			id TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

func NewAuthManager(dbPath string) (*AuthManager, error) {
	db, err := NewAuthDB(dbPath)
	if err != nil {
		return nil, err
	}

	return &AuthManager{
		db:               db,
		sessionTokens:    make(map[string]string),
		sessionTTL:       24 * time.Hour * 7,
		githubOAuthState: make(map[string]*GitHubOAuthStateData),
	}, nil
}

func (am *AuthManager) SetGitHubOAuthConfig(clientID, clientSecret string, enabled bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.githubOAuthConfig = &GitHubOAuthConfigData{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Enabled:      enabled,
	}
}

func (am *AuthManager) GetGitHubOAuthConfig() *GitHubOAuthConfigData {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.githubOAuthConfig
}

func (am *AuthManager) StoreGitHubOAuthState(state string, data *GitHubOAuthStateData) {
	am.mu.Lock()
	defer am.mu.Unlock()
	data.CreatedAt = time.Now()
	am.githubOAuthState[state] = data
}

func (am *AuthManager) GetGitHubOAuthState(state string) (*GitHubOAuthStateData, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	data, ok := am.githubOAuthState[state]
	return data, ok
}

func (am *AuthManager) RemoveGitHubOAuthState(state string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.githubOAuthState, state)
}

func (am *AuthManager) CreateSession(userID string) (*Session, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	token := GenerateSecureToken()
	session := &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.sessionTTL),
	}

	am.sessionTokens[token] = userID

	now := time.Now()
	_, err := am.db.Exec(`
		INSERT INTO sessions (id, user_id, token, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.Token, session.CreatedAt.Format(time.RFC3339), session.ExpiresAt.Format(time.RFC3339))

	if err != nil {
		delete(am.sessionTokens, token)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	am.db.Exec("UPDATE users SET last_login = ? WHERE id = ?", now.Format(time.RFC3339), userID)

	return session, nil
}

func HashSHA256(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func GeneratePKCEVerifier() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func GeneratePKCEChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func (am *AuthManager) Close() error {
	return am.db.Close()
}

func (am *AuthManager) CreateUser(username, email, password string, role Role) (*User, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	var count int
	err := am.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if count > 0 {
		return nil, &AuthError{Message: "username already exists"}
	}

	if email != "" {
		err = am.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to check email: %w", err)
		}
		if count > 0 {
			return nil, &AuthError{Message: "email already exists"}
		}
	}

	user := &User{
		ID:              uuid.New().String(),
		Username:        username,
		Email:           email,
		Role:            role,
		APIKeys:         []string{},
		ChannelBindings: make(map[string]string),
		Settings:        make(map[string]interface{}),
		CreatedAt:       time.Now(),
		Disabled:        false,
		IsVIP:           false,
	}

	if password != "" {
		user.PasswordHash = hashPassword(password)
	}

	apiKeysJSON, _ := json.Marshal(user.APIKeys)
	bindingsJSON, _ := json.Marshal(user.ChannelBindings)
	settingsJSON, _ := json.Marshal(user.Settings)

	_, err = am.db.Exec(`
		INSERT INTO users (id, username, email, password_hash, role, api_keys, channel_bindings, settings, created_at, disabled, is_vip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.Email, user.PasswordHash, user.Role, string(apiKeysJSON), string(bindingsJSON), string(settingsJSON), user.CreatedAt.Format(time.RFC3339), 0, 0)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (am *AuthManager) AuthenticateUser(username, password string) (*Session, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	var user User
	var apiKeysJSON, bindingsJSON, settingsJSON string
	var createdAtStr string
	var lastLogin sql.NullString
	var disabled int
	var isVIP int

	err := am.db.QueryRow(`
		SELECT id, username, email, password_hash, role, api_keys, channel_bindings, settings, created_at, last_login, disabled, is_vip
		FROM users WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&apiKeysJSON, &bindingsJSON, &settingsJSON, &createdAtStr, &lastLogin, &disabled, &isVIP,
	)

	if err == sql.ErrNoRows {
		return nil, &AuthError{Message: "invalid credentials"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	user.Disabled = disabled == 1
	user.IsVIP = isVIP == 1

	if user.Disabled {
		return nil, &AuthError{Message: "account disabled"}
	}

	if user.PasswordHash != hashPassword(password) {
		return nil, &AuthError{Message: "invalid credentials"}
	}

	json.Unmarshal([]byte(apiKeysJSON), &user.APIKeys)
	json.Unmarshal([]byte(bindingsJSON), &user.ChannelBindings)
	json.Unmarshal([]byte(settingsJSON), &user.Settings)

	if lastLogin.Valid {
		t, _ := time.Parse(time.RFC3339, lastLogin.String)
		user.LastLogin = &t
	}

	now := time.Now()
	session := &Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     GenerateSecureToken(),
		CreatedAt: now,
		ExpiresAt: now.Add(am.sessionTTL),
	}

	_, err = am.db.Exec(`
		INSERT INTO sessions (id, user_id, token, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.Token, session.CreatedAt.Format(time.RFC3339), session.ExpiresAt.Format(time.RFC3339))

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	_, err = am.db.Exec(`UPDATE users SET last_login = ? WHERE id = ?`, now.Format(time.RFC3339), user.ID)
	if err != nil {
		fmt.Printf("Warning: failed to update last_login: %v\n", err)
	}

	am.sessionTokens[session.Token] = user.ID

	return session, nil
}

func (am *AuthManager) ValidateToken(token string) (*User, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var session Session
	var createdAtStr, expiresAtStr string
	var ipAddress, userAgent sql.NullString
	err := am.db.QueryRow(`
		SELECT id, user_id, token, created_at, expires_at, ip_address, user_agent
		FROM sessions WHERE token = ?
	`, token).Scan(
		&session.ID, &session.UserID, &session.Token, &createdAtStr, &expiresAtStr,
		&ipAddress, &userAgent,
	)

	if err == sql.ErrNoRows {
		return nil, &AuthError{Message: "invalid token"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	session.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	session.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAtStr)
	if ipAddress.Valid {
		session.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}

	if time.Now().After(session.ExpiresAt) {
		am.db.Exec("DELETE FROM sessions WHERE token = ?", token)
		return nil, &AuthError{Message: "session expired"}
	}

	now := time.Now()
	_, err = am.db.Exec(`UPDATE users SET last_activity_at = ? WHERE id = ?`, now.Format(time.RFC3339), session.UserID)
	if err != nil {
		fmt.Printf("Warning: failed to update last_activity_at: %v\n", err)
	}

	return am.GetUserByID(session.UserID)
}

func (am *AuthManager) Logout(token string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	_, err := am.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	delete(am.sessionTokens, token)
	return err
}

func (am *AuthManager) GetUserByID(userID string) (*User, error) {
	var user User
	var apiKeysJSON, bindingsJSON, settingsJSON string
	var createdAtStr string
	var lastLogin, lastActivityAt sql.NullString
	var disabled int
	var isVIP int

	err := am.db.QueryRow(`
		SELECT id, username, email, password_hash, role, api_keys, channel_bindings, settings, created_at, last_login, last_activity_at, disabled, is_vip
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&apiKeysJSON, &bindingsJSON, &settingsJSON, &createdAtStr, &lastLogin, &lastActivityAt, &disabled, &isVIP,
	)

	if err == sql.ErrNoRows {
		return nil, &AuthError{Message: "user not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	user.Disabled = disabled == 1
	user.IsVIP = isVIP == 1

	json.Unmarshal([]byte(apiKeysJSON), &user.APIKeys)
	json.Unmarshal([]byte(bindingsJSON), &user.ChannelBindings)
	json.Unmarshal([]byte(settingsJSON), &user.Settings)

	if lastLogin.Valid {
		t, _ := time.Parse(time.RFC3339, lastLogin.String)
		user.LastLogin = &t
	}

	if lastActivityAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastActivityAt.String)
		user.LastActivityAt = &t
	}

	return &user, nil
}

func (am *AuthManager) GetUserByUsername(username string) (*User, error) {
	var user User
	var apiKeysJSON, bindingsJSON, settingsJSON string
	var createdAtStr string
	var lastLogin, lastActivityAt sql.NullString
	var disabled int
	var isVIP int

	err := am.db.QueryRow(`
		SELECT id, username, email, password_hash, role, api_keys, channel_bindings, settings, created_at, last_login, last_activity_at, disabled, is_vip
		FROM users WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&apiKeysJSON, &bindingsJSON, &settingsJSON, &createdAtStr, &lastLogin, &lastActivityAt, &disabled, &isVIP,
	)

	if err == sql.ErrNoRows {
		return nil, &AuthError{Message: "user not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	user.Disabled = disabled == 1
	user.IsVIP = isVIP == 1

	json.Unmarshal([]byte(apiKeysJSON), &user.APIKeys)
	json.Unmarshal([]byte(bindingsJSON), &user.ChannelBindings)
	json.Unmarshal([]byte(settingsJSON), &user.Settings)

	if lastLogin.Valid {
		t, _ := time.Parse(time.RFC3339, lastLogin.String)
		user.LastLogin = &t
	}

	if lastActivityAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastActivityAt.String)
		user.LastActivityAt = &t
	}

	return &user, nil
}

func (am *AuthManager) ListUsers() ([]*User, error) {
	rows, err := am.db.Query(`
		SELECT id, username, email, password_hash, role, api_keys, channel_bindings, settings, created_at, last_login, last_activity_at, disabled, is_vip
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var apiKeysJSON, bindingsJSON, settingsJSON string
		var createdAtStr string
		var lastLogin, lastActivityAt sql.NullString
		var disabled int
		var isVIP int

		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
			&apiKeysJSON, &bindingsJSON, &settingsJSON, &createdAtStr, &lastLogin, &lastActivityAt, &disabled, &isVIP,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		user.Disabled = disabled == 1
		user.IsVIP = isVIP == 1

		json.Unmarshal([]byte(apiKeysJSON), &user.APIKeys)
		json.Unmarshal([]byte(bindingsJSON), &user.ChannelBindings)
		json.Unmarshal([]byte(settingsJSON), &user.Settings)

		if lastLogin.Valid {
			t, _ := time.Parse(time.RFC3339, lastLogin.String)
			user.LastLogin = &t
		}

		if lastActivityAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastActivityAt.String)
			user.LastActivityAt = &t
		}

		users = append(users, &user)
	}

	return users, nil
}

func (am *AuthManager) UpdateUser(userID string, updates map[string]interface{}) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if role, ok := updates["role"].(string); ok {
		_, err := am.db.Exec("UPDATE users SET role = ? WHERE id = ?", role, userID)
		if err != nil {
			return fmt.Errorf("failed to update role: %w", err)
		}
	}

	if email, ok := updates["email"].(string); ok {
		_, err := am.db.Exec("UPDATE users SET email = ? WHERE id = ?", email, userID)
		if err != nil {
			return fmt.Errorf("failed to update email: %w", err)
		}
	}

	if password, ok := updates["password"].(string); ok {
		passwordHash := hashPassword(password)
		_, err := am.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
		if err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
	}

	if disabled, ok := updates["disabled"].(bool); ok {
		var disabledInt int
		if disabled {
			disabledInt = 1
		}
		_, err := am.db.Exec("UPDATE users SET disabled = ? WHERE id = ?", disabledInt, userID)
		if err != nil {
			return fmt.Errorf("failed to update disabled: %w", err)
		}
	}

	if isVIP, ok := updates["is_vip"].(bool); ok {
		var vipInt int
		if isVIP {
			vipInt = 1
		}
		_, err := am.db.Exec("UPDATE users SET is_vip = ? WHERE id = ?", vipInt, userID)
		if err != nil {
			return fmt.Errorf("failed to update is_vip: %w", err)
		}
	}

	if settings, ok := updates["settings"].(map[string]interface{}); ok {
		settingsJSON, _ := json.Marshal(settings)
		_, err := am.db.Exec("UPDATE users SET settings = ? WHERE id = ?", string(settingsJSON), userID)
		if err != nil {
			return fmt.Errorf("failed to update settings: %w", err)
		}
	}

	if bindings, ok := updates["channel_bindings"].(map[string]string); ok {
		bindingsJSON, _ := json.Marshal(bindings)
		_, err := am.db.Exec("UPDATE users SET channel_bindings = ? WHERE id = ?", string(bindingsJSON), userID)
		if err != nil {
			return fmt.Errorf("failed to update channel_bindings: %w", err)
		}
	}

	return nil
}

func (am *AuthManager) UpdateLastActivity(userID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	_, err := am.db.Exec("UPDATE users SET last_activity_at = ? WHERE id = ?", now.Format(time.RFC3339), userID)
	if err != nil {
		return fmt.Errorf("failed to update last_activity_at: %w", err)
	}

	return nil
}

func (am *AuthManager) DeleteUser(userID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	result, err := am.db.Exec("DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &AuthError{Message: "user not found"}
	}

	return nil
}

func (am *AuthManager) HasPermission(userID string, permission Permission) (bool, error) {
	user, err := am.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	if user.Disabled {
		return false, nil
	}

	perms, ok := RolePermissions[user.Role]
	if !ok {
		return false, nil
	}

	for _, p := range perms {
		if p == permission {
			return true, nil
		}
	}

	return false, nil
}

func (am *AuthManager) AddAPIKey(userID, apiKey string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, err := am.GetUserByID(userID)
	if err != nil {
		return err
	}

	user.APIKeys = append(user.APIKeys, apiKey)
	apiKeysJSON, _ := json.Marshal(user.APIKeys)

	_, err = am.db.Exec("UPDATE users SET api_keys = ? WHERE id = ?", string(apiKeysJSON), userID)
	return err
}

func (am *AuthManager) RemoveAPIKey(userID, apiKey string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	user, err := am.GetUserByID(userID)
	if err != nil {
		return err
	}

	for i, key := range user.APIKeys {
		if key == apiKey {
			user.APIKeys = append(user.APIKeys[:i], user.APIKeys[i+1:]...)
			break
		}
	}

	apiKeysJSON, _ := json.Marshal(user.APIKeys)
	_, err = am.db.Exec("UPDATE users SET api_keys = ? WHERE id = ?", string(apiKeysJSON), userID)
	return err
}

func (am *AuthManager) AuthenticateAPIKey(apiKey string) (*User, error) {
	rows, err := am.db.Query("SELECT id, api_keys FROM users WHERE disabled = 0")
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID, apiKeysJSON string
		if err := rows.Scan(&userID, &apiKeysJSON); err != nil {
			continue
		}

		var apiKeys []string
		json.Unmarshal([]byte(apiKeysJSON), &apiKeys)

		for _, key := range apiKeys {
			if key == apiKey {
				return am.GetUserByID(userID)
			}
		}
	}

	return nil, &AuthError{Message: "invalid API key"}
}

func (am *AuthManager) UserCount() (int, error) {
	var count int
	err := am.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (am *AuthManager) CleanExpiredSessions() error {
	_, err := am.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().Format(time.RFC3339))
	return err
}

func (am *AuthManager) GetDB() *AuthDB {
	return am.db
}

func hashPassword(password string) string {
	salt := "fangclaw-go-auth-salt-2024"
	hash := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(hash[:])
}

func GenerateSecureToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

func GetDefaultAuthDBPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".fangclaw-go", "auth.db"), nil
}
