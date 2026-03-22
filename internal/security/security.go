// Package security provides security functionality for OpenFang.
package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "user"
	// TokenContextKey is the context key for the authentication token.
	TokenContextKey contextKey = "token"
)

// AuthMiddleware creates an HTTP middleware that authenticates requests.
func AuthMiddleware(authProvider AuthProvider) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Check Bearer scheme
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Invalid authorization scheme", http.StatusUnauthorized)
				return
			}

			// Extract token
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == "" {
				http.Error(w, "Token required", http.StatusUnauthorized)
				return
			}

			// Validate token
			token, err := authProvider.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Store user and token in context
			ctx := context.WithValue(r.Context(), UserContextKey, token.UserID)
			ctx = context.WithValue(ctx, TokenContextKey, token)

			// Call next handler with updated context
			next(w, r.WithContext(ctx))
		}
	}
}

// OptionalAuthMiddleware creates an HTTP middleware that optionally authenticates requests.
// If no token is provided, the request still proceeds but without user context.
func OptionalAuthMiddleware(authProvider AuthProvider) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				// No token, proceed without authentication
				next(w, r)
				return
			}

			// Extract and validate token
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := authProvider.ValidateToken(tokenString)
			if err != nil {
				// Invalid token, but still proceed (optional auth)
				next(w, r)
				return
			}

			// Store user and token in context
			ctx := context.WithValue(r.Context(), UserContextKey, token.UserID)
			ctx = context.WithValue(ctx, TokenContextKey, token)

			// Call next handler with updated context
			next(w, r.WithContext(ctx))
		}
	}
}

// GetUserFromContext retrieves the user ID from the request context.
func GetUserFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserContextKey).(string)
	return userID, ok
}

// GetTokenFromContext retrieves the token from the request context.
func GetTokenFromContext(ctx context.Context) (*Token, bool) {
	token, ok := ctx.Value(TokenContextKey).(*Token)
	return token, ok
}

// DefaultSecurityConfig returns the default security configuration.
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Level:                 SecurityLevelMedium,
		EnableHTTPS:           false,
		EnableCORS:            true,
		EnableRateLimiting:    true,
		EnableAuditLogging:    true,
		EnableEncryption:      true,
		EncryptionAlgorithm:   EncryptionAlgorithmAES,
		JWTSecret:             generateRandomString(32),
		JWTExpiration:         24 * time.Hour,
		MaxLoginAttempts:      5,
		LockoutDuration:       30 * time.Minute,
		CORSAllowOrigins:      []string{"*"},
		RateLimitRequests:     100,
		RateLimitWindow:       time.Minute,
	}
}

// generateRandomString generates a random string of the specified length.
func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "default-secret-key"
	}
	return base64.StdEncoding.EncodeToString(b)
}

// JWT Claims

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.StandardClaims
}

// AuthServiceImpl implements the AuthProvider interface.
type AuthServiceImpl struct {
	users    map[string]*User
	tokens   map[string]*Token
	config   *SecurityConfig
	lockouts map[string]time.Time
}

// NewAuthService creates a new authentication service.
func NewAuthService(config *SecurityConfig) AuthProvider {
	return &AuthServiceImpl{
		users:    make(map[string]*User),
		tokens:   make(map[string]*Token),
		config:   config,
		lockouts: make(map[string]time.Time),
	}
}

// Authenticate authenticates a user.
func (a *AuthServiceImpl) Authenticate(username, password string) (*User, error) {
	// Check if user is locked out
	if lockoutTime, ok := a.lockouts[username]; ok {
		if time.Now().Before(lockoutTime) {
			return nil, errors.New("account locked out")
		}
		delete(a.lockouts, username)
	}

	// Find user
	var user *User
	for _, u := range a.users {
		if u.Username == username && u.Enabled {
			user = u
			break
		}
	}

	if user == nil {
		// Add to lockout attempts
		a.lockouts[username] = time.Now().Add(a.config.LockoutDuration)
		return nil, errors.New("invalid username or password")
	}

	// Verify password
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		// Add to lockout attempts
		a.lockouts[username] = time.Now().Add(a.config.LockoutDuration)
		return nil, errors.New("invalid username or password")
	}

	// Update last login
	user.LastLogin = time.Now()
	user.UpdatedAt = time.Now()

	return user, nil
}

// GenerateToken generates an authentication token.
func (a *AuthServiceImpl) GenerateToken(user *User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(a.config.JWTExpiration).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "fangclaw",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", err
	}

	// Store token
	a.tokens[tokenString] = &Token{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     tokenString,
		ExpiresAt: time.Now().Add(a.config.JWTExpiration),
		CreatedAt: time.Now(),
	}

	return tokenString, nil
}

// ValidateToken validates an authentication token.
func (a *AuthServiceImpl) ValidateToken(tokenString string) (*Token, error) {
	// Check if token exists
	token, ok := a.tokens[tokenString]
	if !ok {
		return nil, errors.New("invalid token")
	}

	// Check if token is expired
	if time.Now().After(token.ExpiresAt) {
		delete(a.tokens, tokenString)
		return nil, errors.New("token expired")
	}

	// Validate token signature
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil || !t.Valid {
		delete(a.tokens, tokenString)
		return nil, errors.New("invalid token")
	}

	return token, nil
}

// RevokeToken revokes an authentication token.
func (a *AuthServiceImpl) RevokeToken(tokenString string) error {
	if _, ok := a.tokens[tokenString]; ok {
		delete(a.tokens, tokenString)
		return nil
	}
	return errors.New("token not found")
}

// AuthorizationServiceImpl implements the AuthorizationProvider interface.
type AuthorizationServiceImpl struct {
	roles       map[string]*Role
	userRoles   map[string][]string
	permissions map[string]*Permission
}

// NewAuthorizationService creates a new authorization service.
func NewAuthorizationService() AuthorizationProvider {
	return &AuthorizationServiceImpl{
		roles:       make(map[string]*Role),
		userRoles:   make(map[string][]string),
		permissions: make(map[string]*Permission),
	}
}

// CheckPermission checks if a user has a permission.
func (a *AuthorizationServiceImpl) CheckPermission(userID, permission string) (bool, error) {
	// Get user roles
	roles, ok := a.userRoles[userID]
	if !ok {
		return false, nil
	}

	// Check each role for the permission
	for _, roleID := range roles {
		role, ok := a.roles[roleID]
		if !ok {
			continue
		}

		for _, perm := range role.Permissions {
			if perm == permission {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetUserPermissions gets all permissions for a user.
func (a *AuthorizationServiceImpl) GetUserPermissions(userID string) ([]string, error) {
	permissions := make(map[string]bool)

	// Get user roles
	roles, ok := a.userRoles[userID]
	if !ok {
		return []string{}, nil
	}

	// Collect permissions from all roles
	for _, roleID := range roles {
		role, ok := a.roles[roleID]
		if !ok {
			continue
		}

		for _, perm := range role.Permissions {
			permissions[perm] = true
		}
	}

	// Convert map to slice
	permSlice := make([]string, 0, len(permissions))
	for perm := range permissions {
		permSlice = append(permSlice, perm)
	}

	return permSlice, nil
}

// AddPermission adds a permission to a role.
func (a *AuthorizationServiceImpl) AddPermission(roleID, permissionID string) error {
	role, ok := a.roles[roleID]
	if !ok {
		return errors.New("role not found")
	}

	// Check if permission already exists
	for _, perm := range role.Permissions {
		if perm == permissionID {
			return nil
		}
	}

	// Add permission
	role.Permissions = append(role.Permissions, permissionID)
	role.UpdatedAt = time.Now()

	return nil
}

// RemovePermission removes a permission from a role.
func (a *AuthorizationServiceImpl) RemovePermission(roleID, permissionID string) error {
	role, ok := a.roles[roleID]
	if !ok {
		return errors.New("role not found")
	}

	// Remove permission
	for i, perm := range role.Permissions {
		if perm == permissionID {
			role.Permissions = append(role.Permissions[:i], role.Permissions[i+1:]...)
			role.UpdatedAt = time.Now()
			return nil
		}
	}

	return nil
}

// AssignRole assigns a role to a user.
func (a *AuthorizationServiceImpl) AssignRole(userID, roleID string) error {
	// Check if role exists
	if _, ok := a.roles[roleID]; !ok {
		return errors.New("role not found")
	}

	// Check if user already has the role
	roles, ok := a.userRoles[userID]
	if !ok {
		roles = []string{}
	}

	for _, r := range roles {
		if r == roleID {
			return nil
		}
	}

	// Assign role
	a.userRoles[userID] = append(roles, roleID)

	return nil
}

// RemoveRole removes a role from a user.
func (a *AuthorizationServiceImpl) RemoveRole(userID, roleID string) error {
	roles, ok := a.userRoles[userID]
	if !ok {
		return nil
	}

	// Remove role
	for i, r := range roles {
		if r == roleID {
			a.userRoles[userID] = append(roles[:i], roles[i+1:]...)
			return nil
		}
	}

	return nil
}

// AuditServiceImpl implements the AuditProvider interface.
type AuditServiceImpl struct {
	logs []*AuditLog
}

// NewAuditService creates a new audit service.
func NewAuditService() AuditProvider {
	return &AuditServiceImpl{
		logs: []*AuditLog{},
	}
}

// Log logs an audit event.
func (a *AuditServiceImpl) Log(action AuditAction, resource, userID, ipAddress, userAgent, details, status string) error {
	log := &AuditLog{
		ID:        uuid.New().String(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   details,
		Status:    status,
		CreatedAt: time.Now(),
	}

	a.logs = append(a.logs, log)
	return nil
}

// GetLogs gets audit logs.
func (a *AuditServiceImpl) GetLogs(filter map[string]interface{}, limit, offset int) ([]*AuditLog, error) {
	// Filter logs
	filtered := []*AuditLog{}
	for _, log := range a.logs {
		match := true
		for key, value := range filter {
			switch key {
			case "user_id":
				if log.UserID != value {
					match = false
				}
			case "action":
				if log.Action != value {
					match = false
				}
			case "resource":
				if log.Resource != value {
					match = false
				}
			case "status":
				if log.Status != value {
					match = false
				}
			}
		}
		if match {
			filtered = append(filtered, log)
		}
	}

	// Apply limit and offset
	if offset >= len(filtered) {
		return []*AuditLog{}, nil
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], nil
}

// GetLog gets an audit log by ID.
func (a *AuditServiceImpl) GetLog(id string) (*AuditLog, error) {
	for _, log := range a.logs {
		if log.ID == id {
			return log, nil
		}
	}
	return nil, errors.New("log not found")
}

// EncryptionServiceImpl implements the EncryptionProvider interface.
type EncryptionServiceImpl struct {
	key []byte
}

// NewEncryptionService creates a new encryption service.
func NewEncryptionService(key []byte) EncryptionProvider {
	if len(key) == 0 {
		key = []byte(generateRandomString(32))
	}
	return &EncryptionServiceImpl{
		key: key,
	}
}

// Encrypt encrypts data.
func (e *EncryptionServiceImpl) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

// Decrypt decrypts data.
func (e *EncryptionServiceImpl) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(data, data)

	return data, nil
}

// Hash hashes data.
func (e *EncryptionServiceImpl) Hash(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	return hash[:], nil
}

// VerifyHash verifies a hash.
func (e *EncryptionServiceImpl) VerifyHash(data, hash []byte) (bool, error) {
	computedHash := sha256.Sum256(data)
	return string(computedHash[:]) == string(hash), nil
}

// GenerateKey generates a key.
func (e *EncryptionServiceImpl) GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// SecurityServiceImpl implements the SecurityProvider interface.
type SecurityServiceImpl struct {
	config               *SecurityConfig
	authProvider         AuthProvider
	authorizationProvider AuthorizationProvider
	auditProvider        AuditProvider
	encryptionProvider   EncryptionProvider
	alerts               []*SecurityAlert
}

// NewSecurityService creates a new security service.
func NewSecurityService(config *SecurityConfig) SecurityProvider {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	return &SecurityServiceImpl{
		config:               config,
		authProvider:         NewAuthService(config),
		authorizationProvider: NewAuthorizationService(),
		auditProvider:        NewAuditService(),
		encryptionProvider:   NewEncryptionService(nil),
		alerts:               []*SecurityAlert{},
	}
}

// GetAuthProvider returns the authentication provider.
func (s *SecurityServiceImpl) GetAuthProvider() AuthProvider {
	return s.authProvider
}

// GetAuthorizationProvider returns the authorization provider.
func (s *SecurityServiceImpl) GetAuthorizationProvider() AuthorizationProvider {
	return s.authorizationProvider
}

// GetAuditProvider returns the audit provider.
func (s *SecurityServiceImpl) GetAuditProvider() AuditProvider {
	return s.auditProvider
}

// GetEncryptionProvider returns the encryption provider.
func (s *SecurityServiceImpl) GetEncryptionProvider() EncryptionProvider {
	return s.encryptionProvider
}

// GetConfig returns the security configuration.
func (s *SecurityServiceImpl) GetConfig() *SecurityConfig {
	return s.config
}

// SetConfig sets the security configuration.
func (s *SecurityServiceImpl) SetConfig(config *SecurityConfig) error {
	s.config = config
	return nil
}

// GetStatus returns the security status.
func (s *SecurityServiceImpl) GetStatus() SecurityStatus {
	// Check for critical alerts
	for _, alert := range s.alerts {
		if !alert.Resolved && alert.Level == SecurityLevelCritical {
			return SecurityStatusCritical
		}
	}

	// Check for warning alerts
	for _, alert := range s.alerts {
		if !alert.Resolved && alert.Level == SecurityLevelHigh {
			return SecurityStatusWarning
		}
	}

	return SecurityStatusSecure
}

// GetAlerts returns security alerts.
func (s *SecurityServiceImpl) GetAlerts(resolved bool) ([]*SecurityAlert, error) {
	filtered := []*SecurityAlert{}
	for _, alert := range s.alerts {
		if alert.Resolved == resolved {
			filtered = append(filtered, alert)
		}
	}
	return filtered, nil
}

// CreateAlert creates a security alert.
func (s *SecurityServiceImpl) CreateAlert(level SecurityLevel, message, details string) error {
	alert := &SecurityAlert{
		ID:        uuid.New().String(),
		Level:     level,
		Message:   message,
		Details:   details,
		Resolved:  false,
		CreatedAt: time.Now(),
	}

	s.alerts = append(s.alerts, alert)
	return nil
}

// ResolveAlert resolves a security alert.
func (s *SecurityServiceImpl) ResolveAlert(id string) error {
	for _, alert := range s.alerts {
		if alert.ID == id {
			alert.Resolved = true
			now := time.Now()
			alert.ResolvedAt = &now
			return nil
		}
	}
	return errors.New("alert not found")
}
