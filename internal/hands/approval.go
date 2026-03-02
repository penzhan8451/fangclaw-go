// Package hands provides autonomous capability packages (Hands) for OpenFang.
package hands

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ApprovalStatus represents the status of an approval request.
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

// ApprovalRequest represents a request for Hand execution approval.
type ApprovalRequest struct {
	ID          string                 `json:"id"`
	HandID      string                 `json:"hand_id"`
	HandName    string                 `json:"hand_name"`
	Reason      string                 `json:"reason"`
	Status      ApprovalStatus         `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	ReviewedAt  *time.Time             `json:"reviewed_at,omitempty"`
	ReviewedBy  string                 `json:"reviewed_by,omitempty"`
	ReviewNote  string                 `json:"review_note,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ApprovalGate manages Hand execution approvals.
type ApprovalGate struct {
	mu       sync.RWMutex
	requests map[string]*ApprovalRequest
}

// NewApprovalGate creates a new approval gate.
func NewApprovalGate() *ApprovalGate {
	return &ApprovalGate{
		requests: make(map[string]*ApprovalRequest),
	}
}

// RequestApproval creates a new approval request.
func (g *ApprovalGate) RequestApproval(handID, handName, reason string, parameters map[string]interface{}) (*ApprovalRequest, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	req := &ApprovalRequest{
		ID:         uuid.New().String(),
		HandID:     handID,
		HandName:   handName,
		Reason:     reason,
		Status:     ApprovalStatusPending,
		CreatedAt:  time.Now(),
		Parameters: parameters,
	}

	g.requests[req.ID] = req
	return req, nil
}

// Approve approves an approval request.
func (g *ApprovalGate) Approve(requestID, reviewer, note string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, ok := g.requests[requestID]
	if !ok {
		return ErrRequestNotFound
	}

	now := time.Now()
	req.Status = ApprovalStatusApproved
	req.ReviewedAt = &now
	req.ReviewedBy = reviewer
	req.ReviewNote = note

	return nil
}

// Reject rejects an approval request.
func (g *ApprovalGate) Reject(requestID, reviewer, note string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, ok := g.requests[requestID]
	if !ok {
		return ErrRequestNotFound
	}

	now := time.Now()
	req.Status = ApprovalStatusRejected
	req.ReviewedAt = &now
	req.ReviewedBy = reviewer
	req.ReviewNote = note

	return nil
}

// GetRequest retrieves an approval request by ID.
func (g *ApprovalGate) GetRequest(requestID string) (*ApprovalRequest, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	req, ok := g.requests[requestID]
	return req, ok
}

// ListRequests lists all approval requests.
func (g *ApprovalGate) ListRequests() []*ApprovalRequest {
	g.mu.RLock()
	defer g.mu.RUnlock()

	requests := make([]*ApprovalRequest, 0, len(g.requests))
	for _, req := range g.requests {
		requests = append(requests, req)
	}
	return requests
}

// ListRequestsByStatus lists approval requests by status.
func (g *ApprovalGate) ListRequestsByStatus(status ApprovalStatus) []*ApprovalRequest {
	g.mu.RLock()
	defer g.mu.RUnlock()

	requests := make([]*ApprovalRequest, 0)
	for _, req := range g.requests {
		if req.Status == status {
			requests = append(requests, req)
		}
	}
	return requests
}

// IsApproved checks if a request is approved.
func (g *ApprovalGate) IsApproved(requestID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	req, ok := g.requests[requestID]
	if !ok {
		return false
	}
	return req.Status == ApprovalStatusApproved
}
