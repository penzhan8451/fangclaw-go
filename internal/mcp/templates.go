package mcp

import "github.com/penzhan8451/fangclaw-go/internal/types"

type McpServerTemplate struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Logo        string             `json:"logo"`
	Category    string             `json:"category"`
	Transport   types.McpTransport `json:"transport"`
	Env         []string           `json:"env,omitempty"`
	TimeoutSecs uint64             `json:"timeout_secs,omitempty"`
	Fields      []TemplateField    `json:"fields,omitempty"`
}

type TemplateField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Placeholder string   `json:"placeholder,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
}

func GetDefaultMcpTemplates() []McpServerTemplate {
	return []McpServerTemplate{
		{
			Name:        "filesystem",
			Description: "Access local filesystem with read/write capabilities",
			Logo:        "📁",
			Category:    "storage",
			Transport: types.McpTransport{
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
			},
			TimeoutSecs: 30,
			Fields: []TemplateField{
				{
					Name:        "path",
					Label:       "Root Path",
					Type:        "string",
					Required:    true,
					Placeholder: "/home/user/docs",
				},
			},
		},
		{
			Name:        "github",
			Description: "Interact with GitHub repositories, issues, and pull requests",
			Logo:        "🐙",
			Category:    "development",
			Transport: types.McpTransport{
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-github"},
			},
			TimeoutSecs: 30,
			Fields: []TemplateField{
				{
					Name:        "GITHUB_PERSONAL_ACCESS_TOKEN",
					Label:       "Personal Access Token",
					Type:        "string",
					Required:    true,
					Placeholder: "ghp_...",
				},
			},
		},
		{
			Name:        "postgres",
			Description: "Connect to PostgreSQL databases",
			Logo:        "🐘",
			Category:    "database",
			Transport: types.McpTransport{
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-postgres"},
			},
			TimeoutSecs: 60,
			Fields: []TemplateField{
				{
					Name:        "connection_string",
					Label:       "Connection String",
					Type:        "string",
					Required:    true,
					Placeholder: "postgresql://user:pass@localhost:5432/db",
				},
			},
		},
		{
			Name:        "brave-search",
			Description: "Web search using Brave Search API",
			Logo:        "🔍",
			Category:    "search",
			Transport: types.McpTransport{
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-brave-search"},
			},
			TimeoutSecs: 30,
			Fields: []TemplateField{
				{
					Name:        "BRAVE_SEARCH_API_KEY",
					Label:       "API Key",
					Type:        "string",
					Required:    true,
					Placeholder: "BSA...",
				},
			},
		},
		{
			Name:        "slack",
			Description: "Send messages and manage Slack channels",
			Logo:        "💬",
			Category:    "communication",
			Transport: types.McpTransport{
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-slack"},
			},
			TimeoutSecs: 30,
			Fields: []TemplateField{
				{
					Name:        "SLACK_BOT_TOKEN",
					Label:       "Bot Token",
					Type:        "string",
					Required:    true,
					Placeholder: "xoxb-...",
				},
			},
		},
		{
			Name:        "sse",
			Description: "Connect to any SSE-based MCP server",
			Logo:        "🌐",
			Category:    "custom",
			Transport: types.McpTransport{
				Type: "sse",
				URL:  "",
			},
			TimeoutSecs: 60,
			Fields: []TemplateField{
				{
					Name:        "url",
					Label:       "Server URL",
					Type:        "string",
					Required:    true,
					Placeholder: "https://mcp.example.com/sse",
				},
			},
		},
	}
}

func GetMcpTemplateCategories() []string {
	return []string{
		"storage",
		"development",
		"database",
		"search",
		"communication",
		"custom",
	}
}
