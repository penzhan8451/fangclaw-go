package capabilities

import (
	"testing"
)

func TestCapabilityMatches(t *testing.T) {
	tests := []struct {
		name     string
		granted  Capability
		required Capability
		want     bool
	}{
		{
			name:     "exact match",
			granted:  Capability{Type: CapNetConnect, Resource: "api.openai.com:443"},
			required: Capability{Type: CapNetConnect, Resource: "api.openai.com:443"},
			want:     true,
		},
		{
			name:     "wildcard match",
			granted:  Capability{Type: CapNetConnect, Resource: "*.openai.com:443"},
			required: Capability{Type: CapNetConnect, Resource: "api.openai.com:443"},
			want:     true,
		},
		{
			name:     "star matches all",
			granted:  Capability{Type: CapAgentMessage, Resource: "*"},
			required: Capability{Type: CapAgentMessage, Resource: "any-agent"},
			want:     true,
		},
		{
			name:     "tool_all grants specific",
			granted:  Capability{Type: CapToolAll},
			required: Capability{Type: CapToolInvoke, Resource: "web_search"},
			want:     true,
		},
		{
			name:     "different types don't match",
			granted:  Capability{Type: CapFileRead, Resource: "*"},
			required: Capability{Type: CapFileWrite, Resource: "/tmp/test"},
			want:     false,
		},
		{
			name:     "prefix wildcard",
			granted:  Capability{Type: CapFileRead, Resource: "/data/*"},
			required: Capability{Type: CapFileRead, Resource: "/data/test.txt"},
			want:     true,
		},
		{
			name:     "suffix wildcard",
			granted:  Capability{Type: CapShellExec, Resource: "ls*"},
			required: Capability{Type: CapShellExec, Resource: "ls -la"},
			want:     true,
		},
		{
			name:     "empty resource matches all",
			granted:  Capability{Type: CapMemoryRead, Resource: ""},
			required: Capability{Type: CapMemoryRead, Resource: "any-scope"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CapabilityMatches(tt.granted, tt.required); got != tt.want {
				t.Errorf("CapabilityMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobMatches(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"exact", "exact", true},
		{"exact", "different", false},
		{"*.openai.com", "api.openai.com", true},
		{"*.openai.com", "api.anthropic.com", false},
		{"/data/*", "/data/test.txt", true},
		{"/data/*", "/other/test.txt", false},
		{"api.*.com", "api.openai.com", true},
		{"api.*.com", "api.openai.org", false},
		{"ls*", "ls -la", true},
		{"ls*", "cat file", false},
		{"", "", true},
		{"", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			if got := globMatches(tt.pattern, tt.value); got != tt.want {
				t.Errorf("globMatches(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestCapabilityManager(t *testing.T) {
	mgr := NewCapabilityManager()
	agentID := "test-agent-123"

	t.Run("no capabilities registered", func(t *testing.T) {
		result, msg := mgr.Check(agentID, Capability{Type: CapToolInvoke, Resource: "test"})
		if result != CapabilityCheckDenied {
			t.Errorf("expected denied, got %v", result)
		}
		if msg == "" {
			t.Error("expected error message")
		}
	})

	t.Run("grant and check", func(t *testing.T) {
		mgr.Grant(agentID, []Capability{
			{Type: CapToolInvoke, Resource: "file_read"},
			{Type: CapShellExec, Resource: "ls*"},
		})

		result, _ := mgr.Check(agentID, Capability{Type: CapToolInvoke, Resource: "file_read"})
		if result != CapabilityCheckGranted {
			t.Errorf("expected granted, got %v", result)
		}

		result, _ = mgr.Check(agentID, Capability{Type: CapToolInvoke, Resource: "other_tool"})
		if result != CapabilityCheckDenied {
			t.Errorf("expected denied for other tool, got %v", result)
		}
	})

	t.Run("check with default - backward compatible", func(t *testing.T) {
		unknownAgent := "unknown-agent"
		result := mgr.CheckWithDefault(unknownAgent, Capability{Type: CapToolInvoke, Resource: "any"})
		if result != CapabilityCheckGranted {
			t.Errorf("expected granted for unknown agent (backward compatible), got %v", result)
		}
	})

	t.Run("list capabilities", func(t *testing.T) {
		caps := mgr.List(agentID)
		if len(caps) != 2 {
			t.Errorf("expected 2 capabilities, got %d", len(caps))
		}
	})

	t.Run("revoke all", func(t *testing.T) {
		mgr.RevokeAll(agentID)
		caps := mgr.List(agentID)
		if len(caps) != 0 {
			t.Errorf("expected 0 capabilities after revoke, got %d", len(caps))
		}
	})
}

func TestValidateCapabilityInheritance(t *testing.T) {
	tests := []struct {
		name       string
		parent     []Capability
		child      []Capability
		wantErr    bool
		errContain string
	}{
		{
			name: "child subset of parent - ok",
			parent: []Capability{
				{Type: CapFileRead, Resource: "*"},
				{Type: CapNetConnect, Resource: "*.example.com:443"},
			},
			child: []Capability{
				{Type: CapFileRead, Resource: "/data/*"},
				{Type: CapNetConnect, Resource: "api.example.com:443"},
			},
			wantErr: false,
		},
		{
			name: "child requests more than parent - denied",
			parent: []Capability{
				{Type: CapFileRead, Resource: "/data/*"},
			},
			child: []Capability{
				{Type: CapFileRead, Resource: "*"},
				{Type: CapShellExec, Resource: "*"},
			},
			wantErr:    true,
			errContain: "privilege escalation denied",
		},
		{
			name: "tool_all covers all tools",
			parent: []Capability{
				{Type: CapToolAll},
			},
			child: []Capability{
				{Type: CapToolInvoke, Resource: "any_tool"},
			},
			wantErr: false,
		},
		{
			name: "empty child - ok",
			parent: []Capability{
				{Type: CapFileRead, Resource: "*"},
			},
			child:   []Capability{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapabilityInheritance(tt.parent, tt.child)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContain != "" && !contains(err.Error(), tt.errContain) {
					t.Errorf("error should contain %q, got %q", tt.errContain, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestMergeCapabilities(t *testing.T) {
	base := []Capability{
		{Type: CapFileRead, Resource: "*"},
		{Type: CapToolInvoke, Resource: "read"},
	}
	extra := []Capability{
		{Type: CapFileWrite, Resource: "/tmp/*"},
		{Type: CapToolInvoke, Resource: "read"},
	}

	merged := MergeCapabilities(base, extra)

	if len(merged) != 3 {
		t.Errorf("expected 3 capabilities after merge, got %d", len(merged))
	}

	hasFileWrite := false
	for _, c := range merged {
		if c.Type == CapFileWrite {
			hasFileWrite = true
		}
	}
	if !hasFileWrite {
		t.Error("expected CapFileWrite in merged capabilities")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
