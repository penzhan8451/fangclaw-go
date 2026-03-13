// Package errors provides error types for OpenFang.
package errors

import (
	"errors"
	"fmt"
)

// Error codes for FangClaw-Go.
const (
	ErrCodeConfig       = "CONFIG"
	ErrCodeDatabase     = "DATABASE"
	ErrCodeKernel       = "KERNEL"
	ErrCodeAgent        = "AGENT"
	ErrCodeLLM          = "LLM"
	ErrCodeAuth         = "AUTH"
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeAlreadyExist = "ALREADY_EXISTS"
	ErrCodeIO           = "IO"
	ErrCodeInternal     = "INTERNAL"
)

// FangClawGoError represents an FangClawGo-specific error.
type FangClawGoError struct {
	Code    string
	Message string
	Err     error
}

func (e *FangClawGoError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *FangClawGoError) Unwrap() error {
	return e.Err
}

// New creates a new FangClawGoError.
func New(code, message string) error {
	return &FangClawGoError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with code and message.
func Wrap(code, message string, err error) error {
	return &FangClawGoError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Is checks if the error matches the target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As casts the error to the target type.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Common error constructors.
var (
	ErrConfigNotFound     = New(ErrCodeConfig, "configuration not found")
	ErrConfigInvalid      = New(ErrCodeConfig, "invalid configuration")
	ErrDatabaseLocked     = New(ErrCodeDatabase, "database is locked")
	ErrDatabaseCorrupt    = New(ErrCodeDatabase, "database is corrupted")
	ErrKernelNotRunning   = New(ErrCodeKernel, "kernel is not running")
	ErrAgentNotFound      = New(ErrCodeNotFound, "agent not found")
	ErrAgentAlreadyExists = New(ErrCodeAlreadyExist, "agent already exists")
	ErrLLMNotConfigured   = New(ErrCodeLLM, "LLM provider not configured")
	ErrLLMAPIError        = New(ErrCodeLLM, "LLM API error")
	ErrAuthFailed         = New(ErrCodeAuth, "authentication failed")
	ErrNotImplemented     = New(ErrCodeInternal, "not implemented")
)

// ConfigError creates a config-related error.
func ConfigError(format string, args ...interface{}) error {
	return &FangClawGoError{
		Code:    ErrCodeConfig,
		Message: fmt.Sprintf(format, args...),
	}
}

// DatabaseError creates a database-related error.
func DatabaseError(format string, args ...interface{}) error {
	return &FangClawGoError{
		Code:    ErrCodeDatabase,
		Message: fmt.Sprintf(format, args...),
	}
}

// KernelError creates a kernel-related error.
func KernelError(format string, args ...interface{}) error {
	return &FangClawGoError{
		Code:    ErrCodeKernel,
		Message: fmt.Sprintf(format, args...),
	}
}

// AgentError creates an agent-related error.
func AgentError(format string, args ...interface{}) error {
	return &FangClawGoError{
		Code:    ErrCodeAgent,
		Message: fmt.Sprintf(format, args...),
	}
}
