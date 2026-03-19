package types

type AgentTemplate struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Icon         string   `json:"icon,omitempty"`
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	Profile      string   `json:"profile"`
	SystemPrompt string   `json:"system_prompt"`
	Tools        []string `json:"tools"`
	Skills       []string `json:"skills"`
	McpServers   []string `json:"mcp_servers"`
}

func (t *AgentTemplate) ToAgentManifest() AgentManifest {
	return AgentManifest{
		Name:         t.Name,
		Description:  t.Description,
		SystemPrompt: t.SystemPrompt,
		Model: ModelConfig{
			Provider: t.Provider,
			Model:    t.Model,
		},
		Tools:      t.Tools,
		Skills:     t.Skills,
		McpServers: t.McpServers,
	}
}

func GetDefaultAgentTemplates() []AgentTemplate {
	return []AgentTemplate{
		{
			ID:           "assistant",
			Name:         "General Assistant",
			Description:  "A versatile helper for everyday tasks, answering questions, and providing recommendations.",
			Icon:         "GA",
			Category:     "General",
			Provider:     "deepseek",
			Model:        "deepseek-chat",
			Profile:      "balanced",
			SystemPrompt: "You are a helpful, friendly assistant. Provide clear, accurate, and concise responses. Ask clarifying questions when needed.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "coder",
			Name:         "Code Helper",
			Description:  "A programming-focused agent that writes, reviews, and debugs code across multiple languages.",
			Icon:         "CH",
			Category:     "Development",
			Provider:     "deepseek",
			Model:        "deepseek-chat",
			Profile:      "precise",
			SystemPrompt: "You are an expert programmer. Help users write clean, efficient code. Explain your reasoning. Follow best practices and conventions for the language being used.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "researcher",
			Name:         "Researcher",
			Description:  "An analytical agent that breaks down complex topics, synthesizes information, and provides cited summaries.",
			Icon:         "RS",
			Category:     "Research",
			Provider:     "zhipu",
			Model:        "glm-4-flash",
			Profile:      "balanced",
			SystemPrompt: "You are a research analyst. Break down complex topics into clear explanations. Provide structured analysis with key findings. Cite sources when available.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "writer",
			Name:         "Writer",
			Description:  "A creative writing agent that helps with drafting, editing, and improving written content of all kinds.",
			Icon:         "WR",
			Category:     "Writing",
			Provider:     "deepseek",
			Model:        "deepseek-chat",
			Profile:      "creative",
			SystemPrompt: "You are a skilled writer and editor. Help users create polished content. Adapt your tone and style to match the intended audience. Offer constructive suggestions for improvement.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "data-analyst",
			Name:         "Data Analyst",
			Description:  "A data-focused agent that helps analyze datasets, create queries, and interpret statistical results.",
			Icon:         "DA",
			Category:     "Development",
			Provider:     "zhipu",
			Model:        "glm-4-flash",
			Profile:      "precise",
			SystemPrompt: "You are a data analysis expert. Help users understand their data, write SQL/Python queries, and interpret results. Present findings clearly with actionable insights.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "devops",
			Name:         "DevOps Engineer",
			Description:  "A systems-focused agent for CI/CD, infrastructure, Docker, and deployment troubleshooting.",
			Icon:         "DO",
			Category:     "Development",
			Provider:     "deepseek",
			Model:        "deepseek-chat",
			Profile:      "precise",
			SystemPrompt: "You are a DevOps engineer. Help with CI/CD pipelines, Docker, Kubernetes, infrastructure as code, and deployment. Prioritize reliability and security.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "support",
			Name:         "Customer Support",
			Description:  "A professional, empathetic agent for handling customer inquiries and resolving issues.",
			Icon:         "CS",
			Category:     "Business",
			Provider:     "zhipu",
			Model:        "glm-4-flash",
			Profile:      "balanced",
			SystemPrompt: "You are a professional customer support representative. Be empathetic, patient, and solution-oriented. Acknowledge concerns before offering solutions. Escalate complex issues appropriately.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "tutor",
			Name:         "Tutor",
			Description:  "A patient educational agent that explains concepts step-by-step and adapts to the learner's level.",
			Icon:         "TU",
			Category:     "General",
			Provider:     "zhipu",
			Model:        "glm-4-flash",
			Profile:      "balanced",
			SystemPrompt: "You are a patient and encouraging tutor. Explain concepts step by step, starting from fundamentals. Use analogies and examples. Check understanding before moving on. Adapt to the learner's pace.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "api-designer",
			Name:         "API Designer",
			Description:  "An agent specialized in RESTful API design, OpenAPI specs, and integration architecture.",
			Icon:         "AD",
			Category:     "Development",
			Provider:     "deepseek",
			Model:        "deepseek-chat",
			Profile:      "precise",
			SystemPrompt: "You are an API design expert. Help users design clean, consistent RESTful APIs following best practices. Cover endpoint naming, request/response schemas, error handling, and versioning.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
		{
			ID:           "meeting-notes",
			Name:         "Meeting Notes",
			Description:  "Summarizes meeting transcripts into structured notes with action items and key decisions.",
			Icon:         "MN",
			Category:     "Business",
			Provider:     "zhipu",
			Model:        "glm-4-flash",
			Profile:      "precise",
			SystemPrompt: "You are a meeting summarizer. When given a meeting transcript or notes, produce a structured summary with: key decisions, action items (with owners), discussion highlights, and follow-up questions.",
			Tools:        []string{"file_read", "web_search"},
			Skills:       []string{},
			McpServers:   []string{},
		},
	}
}
