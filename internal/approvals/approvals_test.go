package approvals

import (
	"testing"
	"time"
)

func TestNormalizeToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"shell to shell_exec", "shell", "shell_exec"},
		{"write_file to file_write", "write_file", "file_write"},
		{"read_file to file_read", "read_file", "file_read"},
		{"list_dir to file_list", "list_dir", "file_list"},
		{"shell_exec remains", "shell_exec", "shell_exec"},
		{"file_write remains", "file_write", "file_write"},
		{"file_read remains", "file_read", "file_read"},
		{"file_list remains", "file_list", "file_list"},
		{"unknown tool remains", "calculator", "calculator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeToolName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeToolName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRequiresApproval(t *testing.T) {
	policy := DefaultApprovalPolicy()
	mgr := NewApprovalManager(policy)

	tests := []struct {
		name     string
		toolName string
		expected bool
	}{
		{"shell requires approval", "shell", true},
		{"shell_exec requires approval", "shell_exec", true},
		{"write_file requires approval", "write_file", true},
		{"file_write requires approval", "file_write", true},
		{"delete requires approval", "delete", true},
		{"exec requires approval", "exec", true},
		{"browser requires approval", "browser", true},
		{"purchase requires approval", "purchase", true},
		{"send_message requires approval", "send_message", true},
		{"read_file does not require approval", "read_file", false},
		{"file_read does not require approval", "file_read", false},
		{"list_dir does not require approval", "list_dir", false},
		{"file_list does not require approval", "file_list", false},
		{"calculator does not require approval", "calculator", false},
		{"memory_kv_get does not require approval", "memory_kv_get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.RequiresApproval(tt.toolName)
			if result != tt.expected {
				t.Errorf("RequiresApproval(%q) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestGetRiskLevel(t *testing.T) {
	policy := DefaultApprovalPolicy()
	mgr := NewApprovalManager(policy)

	tests := []struct {
		name     string
		toolName string
		expected RiskLevel
	}{
		{"shell is high risk", "shell", RiskLevelHigh},
		{"shell_exec is high risk", "shell_exec", RiskLevelHigh},
		{"exec is high risk", "exec", RiskLevelHigh},
		{"delete is high risk", "delete", RiskLevelHigh},
		{"purchase is high risk", "purchase", RiskLevelHigh},
		{"send_money is high risk", "send_money", RiskLevelHigh},
		{"write_file is medium risk", "write_file", RiskLevelMedium},
		{"file_write is medium risk", "file_write", RiskLevelMedium},
		{"browser is medium risk", "browser", RiskLevelMedium},
		{"send_message is medium risk", "send_message", RiskLevelMedium},
		{"read_file is low risk", "read_file", RiskLevelLow},
		{"file_read is low risk", "file_read", RiskLevelLow},
		{"list_dir is low risk", "list_dir", RiskLevelLow},
		{"file_list is low risk", "file_list", RiskLevelLow},
		{"calculator is low risk", "calculator", RiskLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.GetRiskLevel(tt.toolName)
			if result != tt.expected {
				t.Errorf("GetRiskLevel(%q) = %q, want %q", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestRequestApproval_Timeout(t *testing.T) {
	policy := DefaultApprovalPolicy()
	policy.TimeoutSecs = 1
	mgr := NewApprovalManager(policy)

	req := NewApprovalRequest(
		"test-agent",
		"shell_exec",
		"test-desc",
		"test-summary",
		"test-action",
		"test-details",
		RiskLevelHigh,
	)
	decisionCh, err := mgr.RequestApproval(req)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	select {
	case decision := <-decisionCh:
		if decision != ApprovalDecisionTimedOut {
			t.Errorf("Expected timed_out, got %v", decision)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for approval decision")
	}
}

func TestRequestApproval_Resolve(t *testing.T) {
	policy := DefaultApprovalPolicy()
	policy.TimeoutSecs = 10
	mgr := NewApprovalManager(policy)

	req := NewApprovalRequest(
		"test-agent",
		"shell_exec",
		"test-desc",
		"test-summary",
		"test-action",
		"test-details",
		RiskLevelHigh,
	)
	decisionCh, err := mgr.RequestApproval(req)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	// Resolve the approval
	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := mgr.Resolve(req.ID, ApprovalDecisionApproved, "test"); err != nil {
			t.Errorf("Resolve failed: %v", err)
		}
	}()

	select {
	case decision := <-decisionCh:
		if decision != ApprovalDecisionApproved {
			t.Errorf("Expected approved, got %v", decision)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for approval decision")
	}
}

func TestGetPendingApprovals(t *testing.T) {
	policy := DefaultApprovalPolicy()
	mgr := NewApprovalManager(policy)

	req1 := NewApprovalRequest("agent-1", "shell_exec", "desc1", "sum1", "act1", "det1", RiskLevelHigh)
	req2 := NewApprovalRequest("agent-1", "file_write", "desc2", "sum2", "act2", "det2", RiskLevelHigh)

	_, err := mgr.RequestApproval(req1)
	if err != nil {
		t.Fatalf("RequestApproval 1 failed: %v", err)
	}

	_, err = mgr.RequestApproval(req2)
	if err != nil {
		t.Fatalf("RequestApproval 2 failed: %v", err)
	}

	pending := mgr.GetPending()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending approvals, got %d", len(pending))
	}
}

func TestSetOnNewRequest(t *testing.T) {
	policy := DefaultApprovalPolicy()
	mgr := NewApprovalManager(policy)

	callbackCalled := false
	var receivedReq *ApprovalRequest

	mgr.SetOnNewRequest(func(req *ApprovalRequest) {
		callbackCalled = true
		receivedReq = req
	})

	req := NewApprovalRequest("test-agent", "shell_exec", "test-desc", "test-summary", "test-action", "test-details", RiskLevelHigh)
	_, err := mgr.RequestApproval(req)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	if !callbackCalled {
		t.Error("Expected callback to be called")
	}

	if receivedReq == nil || receivedReq.ID != req.ID {
		t.Error("Expected callback to receive the correct request")
	}
}
