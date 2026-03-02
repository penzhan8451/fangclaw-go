// Package mcp provides MCP (Model Context Protocol) integration for OpenFang.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// IntegrationCategory represents the category of an integration.
type IntegrationCategory string

const (
	IntegrationCategoryDevTools      IntegrationCategory = "dev_tools"
	IntegrationCategoryProductivity  IntegrationCategory = "productivity"
	IntegrationCategoryCommunication IntegrationCategory = "communication"
	IntegrationCategoryData          IntegrationCategory = "data"
	IntegrationCategoryCloud         IntegrationCategory = "cloud"
	IntegrationCategoryAI            IntegrationCategory = "ai"
)

// McpTransportType represents the type of MCP transport.
type McpTransportType string

const (
	McpTransportTypeStdio McpTransportType = "stdio"
	McpTransportTypeSse   McpTransportType = "sse"
)

// McpTransport represents the MCP transport configuration.
type McpTransport struct {
	Type    McpTransportType `json:"type"`
	Command string           `json:"command,omitempty"`
	Args    []string         `json:"args,omitempty"`
	URL     string           `json:"url,omitempty"`
}

// RequiredEnvVar represents a required environment variable.
type RequiredEnvVar struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	IsSecret    bool   `json:"is_secret"`
	Default     string `json:"default,omitempty"`
}

// Integration represents an MCP integration.
type Integration struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    IntegrationCategory `json:"category"`
	Icon        string              `json:"icon,omitempty"`
	Transport   McpTransport        `json:"transport"`
	EnvVars     []RequiredEnvVar    `json:"env_vars,omitempty"`
	Enabled     bool                `json:"enabled"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	LastHealth  *time.Time          `json:"last_health,omitempty"`
	Healthy     bool                `json:"healthy"`
}

// IntegrationTemplate represents a template for an integration.
type IntegrationTemplate struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    IntegrationCategory `json:"category"`
	Icon        string              `json:"icon,omitempty"`
	Transport   McpTransport        `json:"transport"`
	EnvVars     []RequiredEnvVar    `json:"env_vars,omitempty"`
	HasOAuth    bool                `json:"has_oauth,omitempty"`
}

// McpServer represents a running MCP server.
type McpServer struct {
	mu          sync.RWMutex
	integration *Integration
	cmd         *exec.Cmd
	stdin       *os.File
	stdout      *os.File
	stderr      *os.File
	ctx         context.Context
	cancel      context.CancelFunc
	connected   bool
}

// IntegrationRegistry manages MCP integrations.
type IntegrationRegistry struct {
	mu           sync.RWMutex
	integrations map[string]*Integration
	templates    map[string]*IntegrationTemplate
}

// NewIntegrationRegistry creates a new integration registry.
func NewIntegrationRegistry() *IntegrationRegistry {
	return &IntegrationRegistry{
		integrations: make(map[string]*Integration),
		templates:    make(map[string]*IntegrationTemplate),
	}
}

// RegisterTemplate registers an integration template.
func (r *IntegrationRegistry) RegisterTemplate(template *IntegrationTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[template.ID] = template
}

// ListTemplates lists all available integration templates.
func (r *IntegrationRegistry) ListTemplates() []*IntegrationTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := make([]*IntegrationTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		templates = append(templates, t)
	}
	return templates
}

// GetTemplate gets an integration template by ID.
func (r *IntegrationRegistry) GetTemplate(id string) (*IntegrationTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.templates[id]
	return t, ok
}

// Install installs an integration from a template.
func (r *IntegrationRegistry) Install(templateID string, envVars map[string]string) (*Integration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	template, ok := r.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	if _, exists := r.integrations[templateID]; exists {
		return nil, fmt.Errorf("integration already installed: %s", templateID)
	}

	now := time.Now()
	integration := &Integration{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Category:    template.Category,
		Icon:        template.Icon,
		Transport:   template.Transport,
		EnvVars:     template.EnvVars,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Healthy:     false,
	}

	r.integrations[integration.ID] = integration
	return integration, nil
}

// Uninstall uninstalls an integration.
func (r *IntegrationRegistry) Uninstall(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.integrations[id]; !ok {
		return fmt.Errorf("integration not found: %s", id)
	}

	delete(r.integrations, id)
	return nil
}

// List lists all installed integrations.
func (r *IntegrationRegistry) List() []*Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	integrations := make([]*Integration, 0, len(r.integrations))
	for _, i := range r.integrations {
		integrations = append(integrations, i)
	}
	return integrations
}

// Get gets an installed integration by ID.
func (r *IntegrationRegistry) Get(id string) (*Integration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.integrations[id]
	return i, ok
}

// Enable enables an integration.
func (r *IntegrationRegistry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	integration, ok := r.integrations[id]
	if !ok {
		return fmt.Errorf("integration not found: %s", id)
	}

	integration.Enabled = true
	integration.UpdatedAt = time.Now()
	return nil
}

// Disable disables an integration.
func (r *IntegrationRegistry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	integration, ok := r.integrations[id]
	if !ok {
		return fmt.Errorf("integration not found: %s", id)
	}

	integration.Enabled = false
	integration.UpdatedAt = time.Now()
	return nil
}

// NewMcpServer creates a new MCP server.
func NewMcpServer(integration *Integration) *McpServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &McpServer{
		integration: integration,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the MCP server.
func (s *McpServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected {
		return fmt.Errorf("server already connected")
	}

	if s.integration.Transport.Type == McpTransportTypeStdio {
		return s.startStdio()
	}

	return fmt.Errorf("unsupported transport type: %s", s.integration.Transport.Type)
}

// startStdio starts a stdio-based MCP server.
func (s *McpServer) startStdio() error {
	cmd := exec.CommandContext(s.ctx, s.integration.Transport.Command, s.integration.Transport.Args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	s.stdin = stdin.(*os.File)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	s.stdout = stdout.(*os.File)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	s.stderr = stderr.(*os.File)

	s.cmd = cmd

	if err := cmd.Start(); err != nil {
		return err
	}

	s.connected = true
	s.integration.Healthy = true
	now := time.Now()
	s.integration.LastHealth = &now

	go s.wait()

	return nil
}

// Stop stops the MCP server.
func (s *McpServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return nil
	}

	s.cancel()

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}

	s.connected = false
	s.integration.Healthy = false
	return nil
}

// wait waits for the server to exit.
func (s *McpServer) wait() {
	if s.cmd != nil {
		s.cmd.Wait()
	}

	s.mu.Lock()
	s.connected = false
	s.integration.Healthy = false
	s.mu.Unlock()
}

// SendRequest sends a request to the MCP server.
func (s *McpServer) SendRequest(request interface{}) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected {
		return nil, fmt.Errorf("server not connected")
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	if _, err := s.stdin.Write(append(requestJSON, '\n')); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("response handling not implemented")
}

// IsConnected returns whether the server is connected.
func (s *McpServer) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// LoadBundledTemplates loads the bundled integration templates.
func LoadBundledTemplates(registry *IntegrationRegistry) {
	templates := []*IntegrationTemplate{
		{
			ID:          "github",
			Name:        "GitHub",
			Description: "GitHub integration for repository management, pull requests, and issues",
			Category:    IntegrationCategoryDevTools,
			Icon:        "🐙",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "GITHUB_PERSONAL_ACCESS_TOKEN",
					Label:       "Personal Access Token",
					Description: "GitHub personal access token with repo scope",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "gitlab",
			Name:        "GitLab",
			Description: "GitLab integration for Git repositories, CI/CD, and project management",
			Category:    IntegrationCategoryDevTools,
			Icon:        "🔬",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "bitbucket",
			Name:        "Bitbucket",
			Description: "Bitbucket integration for Git and Mercurial repositories",
			Category:    IntegrationCategoryDevTools,
			Icon:        "🪣",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "jira",
			Name:        "Jira",
			Description: "Jira integration for issue tracking and project management",
			Category:    IntegrationCategoryDevTools,
			Icon:        "📋",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "linear",
			Name:        "Linear",
			Description: "Linear integration for modern project management",
			Category:    IntegrationCategoryDevTools,
			Icon:        "⚡",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "sentry",
			Name:        "Sentry",
			Description: "Sentry integration for error tracking and performance monitoring",
			Category:    IntegrationCategoryDevTools,
			Icon:        "🔍",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "SENTRY_AUTH_TOKEN",
					Label:       "Auth Token",
					Description: "Sentry authentication token",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "slack",
			Name:        "Slack",
			Description: "Slack integration for messaging and team communication",
			Category:    IntegrationCategoryCommunication,
			Icon:        "💬",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "SLACK_BOT_TOKEN",
					Label:       "Bot Token",
					Description: "Slack bot token",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "discord-mcp",
			Name:        "Discord",
			Description: "Discord integration for chat and community management",
			Category:    IntegrationCategoryCommunication,
			Icon:        "🎮",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "teams-mcp",
			Name:        "Microsoft Teams",
			Description: "Microsoft Teams integration for enterprise collaboration",
			Category:    IntegrationCategoryCommunication,
			Icon:        "👥",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "google-drive",
			Name:        "Google Drive",
			Description: "Google Drive integration for cloud file storage and management",
			Category:    IntegrationCategoryProductivity,
			Icon:        "📁",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "google-calendar",
			Name:        "Google Calendar",
			Description: "Google Calendar integration for scheduling and event management",
			Category:    IntegrationCategoryProductivity,
			Icon:        "📅",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "gmail",
			Name:        "Gmail",
			Description: "Gmail integration for email management and automation",
			Category:    IntegrationCategoryProductivity,
			Icon:        "📧",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "notion",
			Name:        "Notion",
			Description: "Notion integration for knowledge management and documentation",
			Category:    IntegrationCategoryProductivity,
			Icon:        "📝",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "NOTION_API_KEY",
					Label:       "API Key",
					Description: "Notion API key",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "todoist",
			Name:        "Todoist",
			Description: "Todoist integration for task management and productivity",
			Category:    IntegrationCategoryProductivity,
			Icon:        "✅",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "dropbox",
			Name:        "Dropbox",
			Description: "Dropbox integration for cloud file storage and sharing",
			Category:    IntegrationCategoryProductivity,
			Icon:        "📦",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "postgresql",
			Name:        "PostgreSQL",
			Description: "PostgreSQL integration for relational database access",
			Category:    IntegrationCategoryData,
			Icon:        "🐘",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "POSTGRESQL_CONNECTION_STRING",
					Label:       "Connection String",
					Description: "PostgreSQL connection string",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "sqlite-mcp",
			Name:        "SQLite",
			Description: "SQLite integration for local file-based database access",
			Category:    IntegrationCategoryData,
			Icon:        "🗃️",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "mongodb",
			Name:        "MongoDB",
			Description: "MongoDB integration for NoSQL document database access",
			Category:    IntegrationCategoryData,
			Icon:        "🍃",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "MONGODB_URI",
					Label:       "Connection URI",
					Description: "MongoDB connection URI",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "redis",
			Name:        "Redis",
			Description: "Redis integration for caching and key-value storage",
			Category:    IntegrationCategoryData,
			Icon:        "🔴",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "REDIS_URL",
					Label:       "Redis URL",
					Description: "Redis connection URL",
					IsSecret:    false,
				},
			},
		},
		{
			ID:          "elasticsearch",
			Name:        "Elasticsearch",
			Description: "Elasticsearch integration for search and analytics",
			Category:    IntegrationCategoryData,
			Icon:        "🔎",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "aws",
			Name:        "AWS",
			Description: "Amazon Web Services integration for cloud infrastructure",
			Category:    IntegrationCategoryCloud,
			Icon:        "☁️",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "AWS_ACCESS_KEY_ID",
					Label:       "Access Key ID",
					Description: "AWS access key ID",
					IsSecret:    false,
				},
				{
					Name:        "AWS_SECRET_ACCESS_KEY",
					Label:       "Secret Access Key",
					Description: "AWS secret access key",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "azure-mcp",
			Name:        "Microsoft Azure",
			Description: "Microsoft Azure integration for cloud services",
			Category:    IntegrationCategoryCloud,
			Icon:        "🔷",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "gcp-mcp",
			Name:        "Google Cloud Platform",
			Description: "Google Cloud Platform integration for cloud services",
			Category:    IntegrationCategoryCloud,
			Icon:        "🔶",
			HasOAuth:    true,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
		},
		{
			ID:          "brave-search",
			Name:        "Brave Search",
			Description: "Brave Search integration for private web search",
			Category:    IntegrationCategoryAI,
			Icon:        "🦊",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "BRAVE_API_KEY",
					Label:       "API Key",
					Description: "Brave Search API key",
					IsSecret:    true,
				},
			},
		},
		{
			ID:          "exa-search",
			Name:        "Exa Search",
			Description: "Exa (formerly Metaphor) integration for AI-powered web search",
			Category:    IntegrationCategoryAI,
			Icon:        "🔍",
			HasOAuth:    false,
			Transport: McpTransport{
				Type: McpTransportTypeStdio,
			},
			EnvVars: []RequiredEnvVar{
				{
					Name:        "EXA_API_KEY",
					Label:       "API Key",
					Description: "Exa Search API key",
					IsSecret:    true,
				},
			},
		},
	}

	for _, t := range templates {
		registry.RegisterTemplate(t)
	}
}
