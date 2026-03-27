package wizard

import (
	"strings"
	"testing"
)

func sampleIntent() AgentIntent {
	return AgentIntent{
		Name:         "research-bot",
		Description:  "Researches topics and provides summaries",
		Task:         "Search the web for information and provide concise summaries",
		Skills:       []string{"web-summarizer"},
		ModelTier:    "medium",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"web", "memory"},
	}
}

func TestBuildPlanBasic(t *testing.T) {
	intent := sampleIntent()
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Name != "research-bot" {
		t.Errorf("Expected name 'research-bot', got '%s'", plan.Manifest.Name)
	}

	if plan.Manifest.Model.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", plan.Manifest.Model.Provider)
	}

	if !strings.Contains(plan.Summary, "research-bot") {
		t.Errorf("Summary should contain 'research-bot', got '%s'", plan.Summary)
	}
}

func TestBuildPlanComplexTier(t *testing.T) {
	intent := sampleIntent()
	intent.ModelTier = "complex"
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Model.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", plan.Manifest.Model.Provider)
	}

	if !strings.Contains(plan.Manifest.Model.Model, "claude") {
		t.Errorf("Expected model to contain 'claude', got '%s'", plan.Manifest.Model.Model)
	}
}

func TestBuildPlanSimpleTier(t *testing.T) {
	intent := sampleIntent()
	intent.ModelTier = "simple"
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Model.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", plan.Manifest.Model.Provider)
	}

	if plan.Manifest.Model.Model != "gpt-4o-mini" {
		t.Errorf("Expected model 'gpt-4o-mini', got '%s'", plan.Manifest.Model.Model)
	}
}

func TestParseIntentJSON(t *testing.T) {
	jsonStr := `{
		"name": "code-reviewer",
		"description": "Reviews code and suggests improvements",
		"task": "Analyze pull requests and provide feedback",
		"skills": [],
		"model_tier": "complex",
		"scheduled": false,
		"schedule": null,
		"capabilities": ["file"]
	}`

	wizard := NewSetupWizard()
	intent, err := wizard.ParseIntent(jsonStr)
	if err != nil {
		t.Fatalf("Failed to parse intent: %v", err)
	}

	if intent.Name != "code-reviewer" {
		t.Errorf("Expected name 'code-reviewer', got '%s'", intent.Name)
	}

	if intent.ModelTier != "complex" {
		t.Errorf("Expected model_tier 'complex', got '%s'", intent.ModelTier)
	}
}

func TestGenerateJSON(t *testing.T) {
	intent := sampleIntent()
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	jsonStr, err := wizard.GenerateJSON(plan.Manifest)
	if err != nil {
		t.Fatalf("Failed to generate JSON: %v", err)
	}

	if !strings.Contains(jsonStr, "research-bot") {
		t.Errorf("JSON should contain 'research-bot', got '%s'", jsonStr)
	}

	if !strings.Contains(jsonStr, "system_prompt") {
		t.Errorf("JSON should contain 'system_prompt', got '%s'", jsonStr)
	}
}

func TestWebToolsAutoAdded(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"web"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if !containsTool(plan.Manifest.Tools, "fetch") {
		t.Errorf("Expected tools to contain 'fetch', got %v", plan.Manifest.Tools)
	}

	if !containsTool(plan.Manifest.Tools, "web_search") {
		t.Errorf("Expected tools to contain 'web_search', got %v", plan.Manifest.Tools)
	}

	if plan.Manifest.Capabilities == nil || len(plan.Manifest.Capabilities.Network) == 0 {
		t.Errorf("Expected network capability to be set")
	}
}

func TestMemoryCapability(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"memory"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Capabilities == nil {
		t.Errorf("Expected capabilities to be set")
	} else {
		if len(plan.Manifest.Capabilities.MemoryRead) == 0 {
			t.Errorf("Expected memory_read capability to be set")
		}
		if len(plan.Manifest.Capabilities.MemoryWrite) == 0 {
			t.Errorf("Expected memory_write capability to be set")
		}
	}
}

func TestFileToolsAutoAdded(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"file"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if !containsTool(plan.Manifest.Tools, "read_file") {
		t.Errorf("Expected tools to contain 'read_file', got %v", plan.Manifest.Tools)
	}

	if !containsTool(plan.Manifest.Tools, "write_file") {
		t.Errorf("Expected tools to contain 'write_file', got %v", plan.Manifest.Tools)
	}

	if !containsTool(plan.Manifest.Tools, "list_dir") {
		t.Errorf("Expected tools to contain 'list_dir', got %v", plan.Manifest.Tools)
	}
}

func TestShellCapability(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"shell"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if !containsTool(plan.Manifest.Tools, "shell_exec") {
		t.Errorf("Expected tools to contain 'shell_exec', got %v", plan.Manifest.Tools)
	}

	if plan.Manifest.Capabilities == nil || len(plan.Manifest.Capabilities.Shell) == 0 {
		t.Errorf("Expected shell capability to be set")
	}
}

func TestAgentCapability(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"agent"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Capabilities == nil {
		t.Errorf("Expected capabilities to be set")
	} else {
		if !plan.Manifest.Capabilities.AgentSpawn {
			t.Errorf("Expected agent_spawn capability to be true")
		}
		if len(plan.Manifest.Capabilities.AgentMessage) == 0 {
			t.Errorf("Expected agent_message capability to be set")
		}
	}
}

func TestScheduleCapability(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"schedule"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Capabilities == nil || !plan.Manifest.Capabilities.Schedule {
		t.Errorf("Expected schedule capability to be true")
	}
}

func TestMCPCapability(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"mcp"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if plan.Manifest.Capabilities == nil || len(plan.Manifest.Capabilities.McpServers) == 0 {
		t.Errorf("Expected mcp_servers capability to be set")
	}
}

func TestSystemPromptHasTask(t *testing.T) {
	intent := sampleIntent()
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if !strings.Contains(plan.Manifest.SystemPrompt, "YOUR TASK:") {
		t.Errorf("System prompt should contain 'YOUR TASK:', got '%s'", plan.Manifest.SystemPrompt)
	}

	if !strings.Contains(plan.Manifest.SystemPrompt, "Search the web") {
		t.Errorf("System prompt should contain task description, got '%s'", plan.Manifest.SystemPrompt)
	}
}

func TestSystemPromptHasToolHints(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"web"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if !strings.Contains(plan.Manifest.SystemPrompt, "KEY TOOLS:") {
		t.Errorf("System prompt should contain 'KEY TOOLS:', got '%s'", plan.Manifest.SystemPrompt)
	}

	if !strings.Contains(plan.Manifest.SystemPrompt, "web_search") {
		t.Errorf("System prompt should mention web_search tool, got '%s'", plan.Manifest.SystemPrompt)
	}
}

func TestCodeCapability(t *testing.T) {
	intent := AgentIntent{
		Name:         "coder",
		Description:  "Code assistant",
		Task:         "Write and review code",
		Skills:       []string{},
		ModelTier:    "complex",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"code"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	if !containsTool(plan.Manifest.Tools, "read_file") {
		t.Errorf("Expected tools to contain 'read_file' for code capability")
	}

	if !containsTool(plan.Manifest.Tools, "write_file") {
		t.Errorf("Expected tools to contain 'write_file' for code capability")
	}

	if !containsTool(plan.Manifest.Tools, "shell_exec") {
		t.Errorf("Expected tools to contain 'shell_exec' for code capability")
	}

	if plan.Manifest.Capabilities == nil || len(plan.Manifest.Capabilities.Shell) == 0 {
		t.Errorf("Expected shell capability to be set for code capability")
	}
}

func TestMultipleCapabilities(t *testing.T) {
	intent := AgentIntent{
		Name:         "multi-agent",
		Description:  "Multi-purpose agent",
		Task:         "Handle various tasks",
		Skills:       []string{},
		ModelTier:    "medium",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"web", "file", "shell"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	expectedTools := []string{"web_search", "fetch", "read_file", "write_file", "list_dir", "shell_exec"}
	for _, tool := range expectedTools {
		if !containsTool(plan.Manifest.Tools, tool) {
			t.Errorf("Expected tools to contain '%s', got %v", tool, plan.Manifest.Tools)
		}
	}
}

func TestNoDuplicateTools(t *testing.T) {
	intent := AgentIntent{
		Name:         "test",
		Description:  "test",
		Task:         "test",
		Skills:       []string{},
		ModelTier:    "simple",
		Scheduled:    false,
		Schedule:     nil,
		Capabilities: []string{"web", "file"},
	}
	wizard := NewSetupWizard()
	plan := wizard.BuildPlan(intent)

	toolCounts := make(map[string]int)
	for _, tool := range plan.Manifest.Tools {
		toolCounts[tool]++
	}

	for tool, count := range toolCounts {
		if count > 1 {
			t.Errorf("Tool '%s' appears %d times, expected 1", tool, count)
		}
	}
}

func containsTool(tools []string, tool string) bool {
	for _, t := range tools {
		if t == tool {
			return true
		}
	}
	return false
}
