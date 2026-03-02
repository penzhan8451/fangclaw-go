// Package hands provides autonomous capability packages (Hands) for OpenFang.
package hands

import "errors"

// Error definitions for the hands package.
var (
	ErrRequestNotFound  = errors.New("approval request not found")
	ErrHandNotFound     = errors.New("hand not found")
	ErrHandNotRunning   = errors.New("hand not running")
	ErrHandAlreadyRunning = errors.New("hand already running")
	ErrApprovalRequired  = errors.New("approval required")
)
