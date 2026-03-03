// Package approvals provides execution approval management.
package approvals

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// RiskLevel represents the risk level of an operation.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// ApprovalPolicy defines which operations require approval.
type ApprovalPolicy struct {
	RequireApproval []string `json:"require_approval"` // Tool names requiring approval
	AutoApproveLow  bool     `json:"auto_approve_low"` // Auto-approve low risk
}

// DefaultApprovalPolicy returns the default approval policy.
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		RequireApproval: []string{
			"shell",
			"write_file",
			"delete",
			"exec",
			"browser",
			"purchase",
			"send_message",
		},
		AutoApproveLow: true,
	}
}

// ApprovalRequest represents an approval request.
type ApprovalRequest struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agent_id"`
	ToolName      string    `json:"tool_name"`
	Description   string    `json:"description"`
	ActionSummary string    `json:"action_summary"`
	Action        string    `json:"action"`
	Details       string    `json:"details"`
	RiskLevel     RiskLevel `json:"risk_level"`
	RequestedAt   time.Time `json:"requested_at"`
	CreatedAt     time.Time `json:"created_at"`
	TimeoutSecs   int       `json:"timeout_secs"`
}

// NewApprovalRequest creates a new approval request.
func NewApprovalRequest(agentID, toolName, description, actionSummary, action, details string, riskLevel RiskLevel) *ApprovalRequest {
	now := time.Now()
	return &ApprovalRequest{
		ID:            uuid.New().String(),
		AgentID:       agentID,
		ToolName:      toolName,
		Description:   description,
		ActionSummary: actionSummary,
		Action:        action,
		Details:       details,
		RiskLevel:     riskLevel,
		RequestedAt:   now,
		CreatedAt:     now,
		TimeoutSecs:   300, // 5 minutes default
	}
}

// ApprovalDecision represents the decision on an approval request.
type ApprovalDecision string

const (
	ApprovalDecisionApproved ApprovalDecision = "approved"
	ApprovalDecisionDenied   ApprovalDecision = "denied"
	ApprovalDecisionTimedOut ApprovalDecision = "timed_out"
)

// ApprovalResponse represents an approval response.
type ApprovalResponse struct {
	RequestID  string           `json:"request_id"`
	Decision   ApprovalDecision `json:"decision"`
	Reason     string           `json:"reason,omitempty"`
	ResolvedAt time.Time        `json:"resolved_at"`
}

// PendingRequest represents a pending approval request.
type PendingRequest struct {
	Request  ApprovalRequest
	ResultCh chan ApprovalDecision
}

// ApprovalManager manages approval requests.
type ApprovalManager struct {
	mu         sync.RWMutex
	pending    map[string]*PendingRequest
	policy     ApprovalPolicy
	history    []ApprovalResponse
	maxPending int
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager(policy ApprovalPolicy) *ApprovalManager {
	return &ApprovalManager{
		pending:    make(map[string]*PendingRequest),
		policy:     policy,
		history:    make([]ApprovalResponse, 0),
		maxPending: 5,
	}
}

// RequiresApproval checks if a tool requires approval based on current policy.
func (m *ApprovalManager) RequiresApproval(toolName string) bool {
	for _, t := range m.policy.RequireApproval {
		if t == toolName {
			return true
		}
	}
	return false
}

// GetRiskLevel determines the risk level of an operation.
func (m *ApprovalManager) GetRiskLevel(toolName string) RiskLevel {
	highRisk := map[string]bool{
		"shell":      true,
		"exec":       true,
		"delete":     true,
		"purchase":   true,
		"send_money": true,
	}
	mediumRisk := map[string]bool{
		"write_file":   true,
		"browser":      true,
		"send_message": true,
	}

	if highRisk[toolName] {
		return RiskLevelHigh
	}
	if mediumRisk[toolName] {
		return RiskLevelMedium
	}
	return RiskLevelLow
}

// RequestApproval submits an approval request.
func (m *ApprovalManager) RequestApproval(req *ApprovalRequest) (<-chan ApprovalDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this tool requires approval
	if !m.RequiresApproval(req.ToolName) {
		ch := make(chan ApprovalDecision, 1)
		ch <- ApprovalDecisionApproved
		close(ch)
		return ch, nil
	}

	// Check auto-approve for low risk
	if m.policy.AutoApproveLow && req.RiskLevel == RiskLevelLow {
		ch := make(chan ApprovalDecision, 1)
		ch <- ApprovalDecisionApproved
		close(ch)
		return ch, nil
	}

	// Check per-agent pending limit
	pendingCount := 0
	for _, p := range m.pending {
		if p.Request.AgentID == req.AgentID {
			pendingCount++
		}
	}
	if pendingCount >= m.maxPending {
		ch := make(chan ApprovalDecision, 1)
		ch <- ApprovalDecisionDenied
		close(ch)
		return ch, &ApprovalError{Message: "too many pending approval requests"}
	}

	// Create pending request
	resultCh := make(chan ApprovalDecision, 1)
	m.pending[req.ID] = &PendingRequest{
		Request:  *req,
		ResultCh: resultCh,
	}

	return resultCh, nil
}

// Resolve resolves a pending approval request.
func (m *ApprovalManager) Resolve(requestID string, decision ApprovalDecision, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, ok := m.pending[requestID]
	if !ok {
		return &ApprovalError{Message: "request not found"}
	}

	// Send decision
	select {
	case pending.ResultCh <- decision:
	default:
	}

	// Record in history
	m.history = append(m.history, ApprovalResponse{
		RequestID:  requestID,
		Decision:   decision,
		Reason:     reason,
		ResolvedAt: time.Now(),
	})

	// Remove from pending
	delete(m.pending, requestID)

	return nil
}

// ListPending returns all pending approval requests (same as GetPending for Rust compatibility).
func (m *ApprovalManager) ListPending() []*ApprovalRequest {
	return m.GetPending()
}

// GetPending returns all pending approval requests.
func (m *ApprovalManager) GetPending() []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]*ApprovalRequest, 0, len(m.pending))
	for _, p := range m.pending {
		req := p.Request
		requests = append(requests, &req)
	}
	return requests
}

// GetHistory returns approval history.
func (m *ApprovalManager) GetHistory() []ApprovalResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]ApprovalResponse, len(m.history))
	copy(history, m.history)
	return history
}

// GetPolicy returns the current approval policy.
func (m *ApprovalManager) GetPolicy() ApprovalPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// SetPolicy updates the approval policy.
func (m *ApprovalManager) SetPolicy(policy ApprovalPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
}

// ApprovalError represents an approval error.
type ApprovalError struct {
	Message string
}

func (e *ApprovalError) Error() string {
	return e.Message
}

// AutoApprove checks if a tool should be auto-approved.
func (m *ApprovalManager) AutoApprove(toolName string) bool {
	// Tools not in the require list are auto-approved
	return !m.RequiresApproval(toolName)
}
