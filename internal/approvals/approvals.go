// Package approvals provides execution approval management.
package approvals

import (
	"strings"
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
	TimeoutSecs     int      `json:"timeout_secs"`     // Timeout in seconds before auto-deny
	// AutoApproveAutonomous bool     `json:"auto_approve_autonomous"` // Auto-approve in autonomous mode
	AutoApprove bool `json:"auto_approve"` // Alias: if true, clears require list
}

// DefaultApprovalPolicy returns the default approval policy.
func DefaultApprovalPolicy() ApprovalPolicy {
	policy := ApprovalPolicy{
		RequireApproval: []string{
			"shell",
			"shell_exec",
			"write_file",
			"file_write",
			// "read_file",
			// "file_read",
			// "list_dir",
			// "file_list",
			"delete",
			"exec",
			"browser",
			"purchase",
			"send_message",
		},
		AutoApproveLow: true,
		TimeoutSecs:    60 * 5, // Default 5 min timeout
		// AutoApproveAutonomous: false,
		AutoApprove: false,
	}

	// Apply shorthands: if auto_approve is true, clear require list
	if policy.AutoApprove {
		policy.RequireApproval = []string{}
	}

	return policy
}

// NormalizeToolName normalizes tool names for consistency across versions.
func NormalizeToolName(name string) string {
	// Check if it's an MCP tool
	if strings.HasPrefix(name, "mcp_") {
		parts := strings.SplitN(name, "_", 3)
		if len(parts) == 3 {
			// Extract the actual tool name part
			toolPart := parts[2]
			// Map MCP tool names to standard names
			mcpNameMap := map[string]string{
				"read_file":                 "file_read",
				"read_text_file":            "file_read",
				"read_media_file":           "file_read",
				"read_multiple_files":       "file_read",
				"write_file":                "file_write",
				"edit_file":                 "file_write",
				"create_directory":          "file_write",
				"list_directory":            "file_list",
				"list_directory_with_sizes": "file_list",
				"directory_tree":            "file_list",
				"move_file":                 "delete",
				"search_files":              "file_read",
				"get_file_info":             "file_read",
				"list_allowed_directories":  "file_read",
			}
			if normalized, ok := mcpNameMap[toolPart]; ok {
				return normalized
			}
			return toolPart
		}
	}

	nameMap := map[string]string{
		"shell":      "shell_exec",
		"write_file": "file_write",
		"read_file":  "file_read",
		"list_dir":   "file_list",
		"shell_exec": "shell_exec",
		"file_write": "file_write",
		"file_read":  "file_read",
		"file_list":  "file_list",
	}
	if normalized, ok := nameMap[name]; ok {
		return normalized
	}
	return name
}

// ApprovalRequest represents an approval request.
type ApprovalRequest struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agent_id"`
	AgentName     string    `json:"agent_name"`
	ModelProvider string    `json:"model_provider,omitempty"`
	ModelName     string    `json:"model_name,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
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

// NewApprovalRequest creates a new approval request (backward compatible version).
func NewApprovalRequest(agentID, toolName, description, actionSummary, action, details string, riskLevel RiskLevel) *ApprovalRequest {
	now := time.Now()
	return &ApprovalRequest{
		ID:            uuid.New().String(),
		AgentID:       agentID,
		AgentName:     agentID,
		ModelProvider: "?",
		ModelName:     "?",
		SessionID:     "",
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

// NewApprovalRequestWithDetails creates a new approval request with all details.
func NewApprovalRequestWithDetails(agentID, agentName, modelProvider, modelName, sessionID, toolName, description, actionSummary, action, details string, riskLevel RiskLevel) *ApprovalRequest {
	now := time.Now()
	return &ApprovalRequest{
		ID:            uuid.New().String(),
		AgentID:       agentID,
		AgentName:     agentName,
		ModelProvider: modelProvider,
		ModelName:     modelName,
		SessionID:     sessionID,
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
	Request    *ApprovalRequest `json:"request,omitempty"` // Store original request
}

// PendingRequest represents a pending approval request.
type PendingRequest struct {
	Request  ApprovalRequest
	ResultCh chan ApprovalDecision
}

// ApprovalManager manages approval requests.
type ApprovalManager struct {
	mu           sync.RWMutex
	pending      map[string]*PendingRequest
	policy       ApprovalPolicy
	history      []ApprovalResponse
	maxPending   int
	onNewRequest func(req *ApprovalRequest)
	onResolve    func(req *ApprovalRequest, decision ApprovalDecision, reason string)
	requestsMap  map[string]*ApprovalRequest // Map to store all requests by ID
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager(policy ApprovalPolicy) *ApprovalManager {
	return &ApprovalManager{
		pending:     make(map[string]*PendingRequest),
		policy:      policy,
		history:     make([]ApprovalResponse, 0),
		maxPending:  5,
		requestsMap: make(map[string]*ApprovalRequest),
	}
}

// SetOnNewRequest sets the callback for new approval requests.
func (m *ApprovalManager) SetOnNewRequest(fn func(req *ApprovalRequest)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNewRequest = fn
}

// SetOnResolve sets the callback for when an approval request is resolved.
func (m *ApprovalManager) SetOnResolve(fn func(req *ApprovalRequest, decision ApprovalDecision, reason string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onResolve = fn
}

// RequiresApproval checks if a tool requires approval based on current policy.
func (m *ApprovalManager) RequiresApproval(toolName string) bool {
	normalizedName := NormalizeToolName(toolName)
	for _, t := range m.policy.RequireApproval {
		if t == toolName || t == normalizedName {
			return true
		}
	}
	return false
}

// GetRiskLevel determines the risk level of an operation.
func (m *ApprovalManager) GetRiskLevel(toolName string) RiskLevel {
	normalizedName := NormalizeToolName(toolName)
	highRisk := map[string]bool{
		"shell":      true,
		"shell_exec": true,
		"exec":       true,
		"rm":         true,
		"delete":     true,
		"purchase":   true,
		"send_money": true,
	}
	mediumRisk := map[string]bool{
		"write_file":   true,
		"file_write":   true,
		"read_file":    true,
		"file_read":    true,
		"list_dir":     true,
		"file_list":    true,
		"browser":      true,
		"send_message": true,
	}

	if highRisk[toolName] || highRisk[normalizedName] {
		return RiskLevelHigh
	}
	if mediumRisk[toolName] || mediumRisk[normalizedName] {
		return RiskLevelMedium
	}
	return RiskLevelLow
}

// RequestApproval submits an approval request.
func (m *ApprovalManager) RequestApproval(req *ApprovalRequest) (<-chan ApprovalDecision, error) {
	m.mu.Lock()

	// Store request in requestsMap
	reqCopy := *req
	m.requestsMap[req.ID] = &reqCopy

	// Check if this tool requires approval
	if !m.RequiresApproval(req.ToolName) {
		m.mu.Unlock()
		ch := make(chan ApprovalDecision, 1)
		ch <- ApprovalDecisionApproved
		close(ch)
		return ch, nil
	}

	// Check auto-approve for low risk
	if m.policy.AutoApproveLow && req.RiskLevel == RiskLevelLow {
		m.mu.Unlock()
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
		m.mu.Unlock()
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

	// Get timeout from policy or use request timeout
	timeoutSecs := m.policy.TimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = req.TimeoutSecs
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 60 // Default 60 seconds
	}

	// Save callback reference before unlocking
	var callback func(req *ApprovalRequest)
	if m.onNewRequest != nil {
		callback = m.onNewRequest
	}

	m.mu.Unlock()

	// Trigger callback outside the lock
	if callback != nil {
		callback(req) // notify the approval manager that a new request is pending
	}

	// Start timeout goroutine
	go func(requestID string, timeout time.Duration) {
		time.Sleep(timeout)

		m.mu.Lock()
		defer m.mu.Unlock()

		// Check if request still exists
		if pending, ok := m.pending[requestID]; ok {
			// Try to send timed out decision
			select {
			case pending.ResultCh <- ApprovalDecisionTimedOut:
				// Record in history
				m.history = append(m.history, ApprovalResponse{
					RequestID:  requestID,
					Decision:   ApprovalDecisionTimedOut,
					Reason:     "timeout",
					ResolvedAt: time.Now(),
					Request:    m.requestsMap[requestID],
				})
				// Remove from pending
				delete(m.pending, requestID)
			default:
				// Channel already received a decision, do nothing
			}
		}
	}(req.ID, time.Duration(timeoutSecs)*time.Second)

	return resultCh, nil
}

// Resolve resolves a pending approval request.
func (m *ApprovalManager) Resolve(requestID string, decision ApprovalDecision, reason string) error {
	m.mu.Lock()

	pending, ok := m.pending[requestID]
	if !ok {
		m.mu.Unlock()
		return &ApprovalError{Message: "request not found"}
	}

	// Make a copy of the request before removing it
	reqCopy := pending.Request

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
		Request:    m.requestsMap[requestID],
	})

	// Remove from pending
	delete(m.pending, requestID)

	// Save callback reference before unlocking
	var callback func(req *ApprovalRequest, decision ApprovalDecision, reason string)
	if m.onResolve != nil {
		callback = m.onResolve
	}

	m.mu.Unlock()

	// Trigger callback outside the lock
	if callback != nil {
		callback(&reqCopy, decision, reason)
	}

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

// GetAllApprovals returns all approvals (pending and history).
func (m *ApprovalManager) GetAllApprovals() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []map[string]interface{}

	// Add pending requests first
	for _, p := range m.pending {
		req := p.Request
		result = append(result, map[string]interface{}{
			"id":             req.ID,
			"agent_id":       req.AgentID,
			"agent_name":     req.AgentName,
			"model_provider": req.ModelProvider,
			"model_name":     req.ModelName,
			"session_id":     req.SessionID,
			"tool_name":      req.ToolName,
			"description":    req.Description,
			"action_summary": req.ActionSummary,
			"action":         req.ActionSummary,
			"risk_level":     req.RiskLevel,
			"requested_at":   req.RequestedAt,
			"created_at":     req.CreatedAt,
			"timeout_secs":   req.TimeoutSecs,
			"status":         "pending",
		})
	}

	// Add history records
	for _, resp := range m.history {
		if resp.Request != nil {
			req := resp.Request
			status := string(resp.Decision)
			// Map decision to status (denied -> rejected for UI compatibility)
			if status == "denied" {
				status = "rejected"
			}
			if status == "timed_out" {
				status = "expired"
			}
			result = append(result, map[string]interface{}{
				"id":             req.ID,
				"agent_id":       req.AgentID,
				"agent_name":     req.AgentName,
				"model_provider": req.ModelProvider,
				"model_name":     req.ModelName,
				"session_id":     req.SessionID,
				"tool_name":      req.ToolName,
				"description":    req.Description,
				"action_summary": req.ActionSummary,
				"action":         req.ActionSummary,
				"risk_level":     req.RiskLevel,
				"requested_at":   req.RequestedAt,
				"created_at":     req.CreatedAt,
				"timeout_secs":   req.TimeoutSecs,
				"status":         status,
				"resolved_at":    resp.ResolvedAt,
			})
		}
	}

	return result
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
