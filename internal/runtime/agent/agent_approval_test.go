package agent

import (
	"testing"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/approvals"
)

func TestExecuteTool_ApprovalIntegration(t *testing.T) {
	t.Skip("Skipping integration test - requires full setup")
}

func TestApprovalManagerIntegration(t *testing.T) {
	policy := approvals.DefaultApprovalPolicy()
	policy.TimeoutSecs = 1
	mgr := approvals.NewApprovalManager(policy)

	var callbackCalled bool
	mgr.SetOnNewRequest(func(req *approvals.ApprovalRequest) {
		callbackCalled = true
		if req.ToolName != "shell_exec" {
			t.Errorf("Expected tool_name shell_exec, got %v", req.ToolName)
		}
	})

	req := approvals.NewApprovalRequest(
		"test-agent",
		"shell_exec",
		"test-desc",
		"test-summary",
		"test-action",
		"test-details",
		approvals.RiskLevelHigh,
	)

	decisionCh, err := mgr.RequestApproval(req)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	if !callbackCalled {
		t.Error("Expected callback to be called")
	}

	select {
	case decision := <-decisionCh:
		if decision != approvals.ApprovalDecisionTimedOut {
			t.Errorf("Expected timed_out, got %v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for decision")
	}
}
