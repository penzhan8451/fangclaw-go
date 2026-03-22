package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrPathTraversalDetected = errors.New("path traversal detected")
	ErrAbsolutePathRequired  = errors.New("absolute path required")
)

// SafePathConfig holds safe path resolution configuration.
type SafePathConfig struct {
	BaseDir     string
	AllowEscape bool
}

// SafeResolvePath safely resolves a path to prevent path traversal attacks.
func SafeResolvePath(baseDir, path string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("base directory cannot be empty")
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base path: %w", err)
	}

	joinedPath := filepath.Join(absBaseDir, path)
	cleanPath := filepath.Clean(joinedPath)

	absCleanPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute clean path: %w", err)
	}

	if !strings.HasPrefix(absCleanPath, absBaseDir+string(filepath.Separator)) &&
		absCleanPath != absBaseDir {
		return "", ErrPathTraversalDetected
	}

	return absCleanPath, nil
}

// IsSafePath checks if a path is safe (no path traversal components).
func IsSafePath(path string) bool {
	cleanPath := filepath.Clean(path)

	if strings.Contains(cleanPath, "..") {
		return false
	}

	return true
}

// SafeEnvConfig holds safe environment variable configuration.
type SafeEnvConfig struct {
	AllowedVars []string
	BlockedVars []string
}

// DefaultSafeEnvConfig returns the default safe environment configuration.
func DefaultSafeEnvConfig() *SafeEnvConfig {
	return &SafeEnvConfig{
		AllowedVars: []string{
			"PATH",
			"HOME",
			"USER",
			"TMP",
			"TEMP",
			"TMPDIR",
			"LANG",
			"LC_ALL",
			"LC_CTYPE",
		},
		BlockedVars: []string{},
	}
}

// CleanEnvironment cleans the environment variables for subprocess execution.
func CleanEnvironment(config *SafeEnvConfig) []string {
	if config == nil {
		config = DefaultSafeEnvConfig()
	}

	var allowedMap map[string]bool
	if len(config.AllowedVars) > 0 {
		allowedMap = make(map[string]bool)
		for _, v := range config.AllowedVars {
			allowedMap[v] = true
		}
	}

	blockedMap := make(map[string]bool)
	for _, v := range config.BlockedVars {
		blockedMap[v] = true
	}

	var cleanEnv []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		if blockedMap[key] {
			continue
		}

		if allowedMap != nil && !allowedMap[key] {
			continue
		}

		cleanEnv = append(cleanEnv, env)
	}

	return cleanEnv
}

// Zeroize overwrites sensitive data in memory.
func Zeroize(data []byte) {
	if data == nil {
		return
	}

	for i := range data {
		data[i] = 0
	}

	runtime.KeepAlive(data)
}

// ZeroizeString overwrites a string in memory.
func ZeroizeString(s *string) {
	if s == nil || *s == "" {
		return
	}

	data := []byte(*s)
	Zeroize(data)
	*s = string(data)
}

// SensitiveBuffer represents a buffer that automatically zeroizes on close.
type SensitiveBuffer struct {
	data []byte
}

// NewSensitiveBuffer creates a new sensitive buffer.
func NewSensitiveBuffer(size int) *SensitiveBuffer {
	return &SensitiveBuffer{
		data: make([]byte, size),
	}
}

// Bytes returns the underlying bytes.
func (b *SensitiveBuffer) Bytes() []byte {
	return b.data
}

// Close zeroizes and releases the buffer.
func (b *SensitiveBuffer) Close() {
	if b.data != nil {
		Zeroize(b.data)
		b.data = nil
	}
}

// SecureExecConfig holds configuration for secure subprocess execution.
type SecureExecConfig struct {
	PathConfig *SafePathConfig
	EnvConfig  *SafeEnvConfig
	WorkingDir string
}

// DefaultSecureExecConfig returns the default secure execution configuration.
func DefaultSecureExecConfig() *SecureExecConfig {
	return &SecureExecConfig{
		PathConfig: nil,
		EnvConfig:  DefaultSafeEnvConfig(),
		WorkingDir: "",
	}
}

// PrepareSecureExec prepares for secure subprocess execution.
func PrepareSecureExec(config *SecureExecConfig) (env []string, workDir string, err error) {
	if config == nil {
		config = DefaultSecureExecConfig()
	}

	env = CleanEnvironment(config.EnvConfig)
	workDir = config.WorkingDir

	if workDir != "" {
		absWorkDir, err := filepath.Abs(workDir)
		if err != nil {
			return nil, "", fmt.Errorf("invalid working directory: %w", err)
		}
		workDir = absWorkDir
	}

	return env, workDir, nil
}
