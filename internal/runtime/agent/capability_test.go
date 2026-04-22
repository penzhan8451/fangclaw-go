package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/capabilities"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent/tools"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

func TestCapabilityCheck(t *testing.T) {
	r := NewRuntime(nil, nil, nil, nil, &skills.Loader{}, "", nil, nil, nil, nil, nil)

	tools.RegisterAllTools(r)

	capMgr := capabilities.NewCapabilityManager()
	testAgentID := uuid.New().String()

	capMgr.Grant(testAgentID, []capabilities.Capability{
		{Type: capabilities.CapToolInvoke, Resource: "web_search"},
		{Type: capabilities.CapToolInvoke, Resource: "fetch"},
		{Type: capabilities.CapNetConnect, Resource: "*"},
	})

	r.SetCapabilityChecker(func(id, capType, resource string) bool {
		cap := capabilities.Capability{
			Type:     capabilities.CapabilityType(capType),
			Resource: resource,
		}
		result := capMgr.CheckWithDefault(id, cap)
		return result.Granted()
	})

	agentCtx := &AgentContext{
		ID:        testAgentID,
		Name:      "test-agent",
		Provider:  "openai",
		Model:     "gpt-4",
		AgentID:   uuid.New(),
		SessionID: types.NewSessionID(),
		Config:    types.LoopConfig{MaxIterations: 10, MaxTokens: 4096},
		Tools:     []string{"web_search", "fetch"},
		Messages:  make([]types.Message, 0),
	}

	t.Run("denied tool - shell_exec", func(t *testing.T) {
		_, err := r.executeTool(context.Background(), agentCtx, "shell_exec", map[string]interface{}{
			"command": "ls -la",
		})
		if err == nil {
			t.Error("shell_exec should be denied")
			return
		}
		expectedErr := "capability denied: agent does not have permission to invoke tool 'shell_exec'"
		if err.Error() != expectedErr {
			t.Errorf("expected '%s', got: %v", expectedErr, err)
		}
	})

	t.Run("denied tool - calculator", func(t *testing.T) {
		_, err := r.executeTool(context.Background(), agentCtx, "calculator", map[string]interface{}{
			"expression": "1+1",
		})
		if err == nil {
			t.Error("calculator should be denied")
			return
		}
		expectedErr := "capability denied: agent does not have permission to invoke tool 'calculator'"
		if err.Error() != expectedErr {
			t.Errorf("expected '%s', got: %v", expectedErr, err)
		}
	})

	t.Run("allowed tool - calculator with CapToolAll", func(t *testing.T) {
		allToolsAgentID := uuid.New().String()
		capMgr.Grant(allToolsAgentID, []capabilities.Capability{
			{Type: capabilities.CapToolAll},
		})

		allToolsAgentCtx := &AgentContext{
			ID:        allToolsAgentID,
			Name:      "all-tools-agent",
			Provider:  "openai",
			Model:     "gpt-4",
			AgentID:   uuid.New(),
			SessionID: types.NewSessionID(),
			Config:    types.LoopConfig{MaxIterations: 10, MaxTokens: 4096},
			Tools:     []string{},
			Messages:  make([]types.Message, 0),
		}

		_, err := r.executeTool(context.Background(), allToolsAgentCtx, "calculator", map[string]interface{}{
			"expression": "1+1",
		})
		if err != nil {
			t.Errorf("calculator should be allowed with CapToolAll, got: %v", err)
		}
	})

	t.Run("agent without capabilities - backward compatible", func(t *testing.T) {
		unknownAgentID := uuid.New().String()
		unknownAgentCtx := &AgentContext{
			ID:        unknownAgentID,
			Name:      "unknown-agent",
			Provider:  "openai",
			Model:     "gpt-4",
			AgentID:   uuid.New(),
			SessionID: types.NewSessionID(),
			Config:    types.LoopConfig{MaxIterations: 10, MaxTokens: 4096},
			Tools:     []string{},
			Messages:  make([]types.Message, 0),
		}

		_, err := r.executeTool(context.Background(), unknownAgentCtx, "calculator", map[string]interface{}{
			"expression": "1+1",
		})
		if err != nil && err.Error() == "capability denied: agent does not have permission to invoke tool 'calculator'" {
			t.Errorf("unknown agent should be backward compatible (allowed), got: %v", err)
		}
	})
}
