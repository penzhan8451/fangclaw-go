// Package security provides security functionality for OpenFang.
package security

import (
	"time"
)

// SecurityLevel represents the security level of the system.
type SecurityLevel string

// SecurityStatus represents the security status of the system.
type SecurityStatus string

// AuditAction represents the type of audit action.
type AuditAction string

// EncryptionAlgorithm represents the encryption algorithm.
type EncryptionAlgorithm string

const (
	// Security levels
	SecurityLevelLow    SecurityLevel = "low"
	SecurityLevelMedium SecurityLevel = "medium"
	SecurityLevelHigh   SecurityLevel = "high"
	SecurityLevelCritical SecurityLevel = "critical"

	// Security statuses
	SecurityStatusSecure   SecurityStatus = "secure"
	SecurityStatusWarning  SecurityStatus = "warning"
	SecurityStatusCritical SecurityStatus = "critical"

	// Audit actions
	AuditActionLogin      AuditAction = "login"
	AuditActionLogout     AuditAction = "logout"
	AuditActionCreate     AuditAction = "create"
	AuditActionUpdate     AuditAction = "update"
	AuditActionDelete     AuditAction = "delete"
	AuditActionExecute    AuditAction = "execute"
	AuditActionAccess     AuditAction = "access"
	AuditActionError      AuditAction = "error"
	AuditActionWarning    AuditAction = "warning"

	// Encryption algorithms
	EncryptionAlgorithmAES    EncryptionAlgorithm = "aes"
	EncryptionAlgorithmRSA    EncryptionAlgorithm = "rsa"
	EncryptionAlgorithmECC    EncryptionAlgorithm = "ecc"
	EncryptionAlgorithmSHA256 EncryptionAlgorithm = "sha256"
	EncryptionAlgorithmSHA512 EncryptionAlgorithm = "sha512"
)

// SecurityConfig represents the security configuration.
type SecurityConfig struct {
	Level                 SecurityLevel      `json:"level"`
	EnableHTTPS           bool              `json:"enable_https"`
	EnableCORS            bool              `json:"enable_cors"`
	EnableRateLimiting    bool              `json:"enable_rate_limiting"`
	EnableAuditLogging    bool              `json:"enable_audit_logging"`
	EnableEncryption      bool              `json:"enable_encryption"`
	EncryptionAlgorithm   EncryptionAlgorithm `json:"encryption_algorithm"`
	JWTSecret             string            `json:"jwt_secret"`
	JWTExpiration         time.Duration     `json:"jwt_expiration"`
	MaxLoginAttempts      int               `json:"max_login_attempts"`
	LockoutDuration       time.Duration     `json:"lockout_duration"`
	CORSAllowOrigins      []string          `json:"cors_allow_origins"`
	RateLimitRequests     int               `json:"rate_limit_requests"`
	RateLimitWindow       time.Duration     `json:"rate_limit_window"`
}

// User represents a user in the system.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	Enabled      bool      `json:"enabled"`
	LastLogin    time.Time `json:"last_login"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Role represents a role in the system.
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Permission represents a permission in the system.
type Permission struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Action    AuditAction `json:"action"`
	Resource  string      `json:"resource"`
	IPAddress string      `json:"ip_address"`
	UserAgent string      `json:"user_agent"`
	Details   string      `json:"details"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

// SecurityAlert represents a security alert.
type SecurityAlert struct {
	ID        string        `json:"id"`
	Level     SecurityLevel `json:"level"`
	Message   string        `json:"message"`
	Details   string        `json:"details"`
	Resolved  bool          `json:"resolved"`
	CreatedAt time.Time     `json:"created_at"`
	ResolvedAt *time.Time   `json:"resolved_at,omitempty"`
}

// Token represents an authentication token.
type Token struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// SecurityContext represents the security context for a request.
type SecurityContext struct {
	User      *User      `json:"user,omitempty"`
	Token     *Token     `json:"token,omitempty"`
	IPAddress string     `json:"ip_address"`
	UserAgent string     `json:"user_agent"`
	Permissions []string `json:"permissions,omitempty"`
}

// AuthProvider represents an authentication provider.
type AuthProvider interface {
	// Authenticate authenticates a user.
	Authenticate(username, password string) (*User, error)

	// GenerateToken generates an authentication token.
	GenerateToken(user *User) (string, error)

	// ValidateToken validates an authentication token.
	ValidateToken(token string) (*Token, error)

	// RevokeToken revokes an authentication token.
	RevokeToken(token string) error
}

// AuthorizationProvider represents an authorization provider.
type AuthorizationProvider interface {
	// CheckPermission checks if a user has a permission.
	CheckPermission(userID, permission string) (bool, error)

	// GetUserPermissions gets all permissions for a user.
	GetUserPermissions(userID string) ([]string, error)

	// AddPermission adds a permission to a role.
	AddPermission(roleID, permissionID string) error

	// RemovePermission removes a permission from a role.
	RemovePermission(roleID, permissionID string) error

	// AssignRole assigns a role to a user.
	AssignRole(userID, roleID string) error

	// RemoveRole removes a role from a user.
	RemoveRole(userID, roleID string) error
}

// AuditProvider represents an audit provider.
type AuditProvider interface {
	// Log logs an audit event.
	Log(action AuditAction, resource, userID, ipAddress, userAgent, details, status string) error

	// GetLogs gets audit logs.
	GetLogs(filter map[string]interface{}, limit, offset int) ([]*AuditLog, error)

	// GetLog gets an audit log by ID.
	GetLog(id string) (*AuditLog, error)
}

// EncryptionProvider represents an encryption provider.
type EncryptionProvider interface {
	// Encrypt encrypts data.
	Encrypt(data []byte) ([]byte, error)

	// Decrypt decrypts data.
	Decrypt(data []byte) ([]byte, error)

	// Hash hashes data.
	Hash(data []byte) ([]byte, error)

	// VerifyHash verifies a hash.
	VerifyHash(data, hash []byte) (bool, error)

	// GenerateKey generates a key.
	GenerateKey() ([]byte, error)
}

// SecurityProvider represents a security provider.
type SecurityProvider interface {
	// GetAuthProvider returns the authentication provider.
	GetAuthProvider() AuthProvider

	// GetAuthorizationProvider returns the authorization provider.
	GetAuthorizationProvider() AuthorizationProvider

	// GetAuditProvider returns the audit provider.
	GetAuditProvider() AuditProvider

	// GetEncryptionProvider returns the encryption provider.
	GetEncryptionProvider() EncryptionProvider

	// GetConfig returns the security configuration.
	GetConfig() *SecurityConfig

	// SetConfig sets the security configuration.
	SetConfig(config *SecurityConfig) error

	// GetStatus returns the security status.
	GetStatus() SecurityStatus

	// GetAlerts returns security alerts.
	GetAlerts(resolved bool) ([]*SecurityAlert, error)

	// CreateAlert creates a security alert.
	CreateAlert(level SecurityLevel, message, details string) error

	// ResolveAlert resolves a security alert.
	ResolveAlert(id string) error
}
