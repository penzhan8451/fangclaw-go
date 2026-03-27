// Package tui provides a terminal user interface for FangClaw-Go.
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

// =============================================================================
// Tab System
// =============================================================================

type Tab int

const (
	TabDashboard Tab = iota
	TabAgents
	TabChat
	TabSessions
	TabWorkflows
	TabTriggers
	TabMemory
	TabChannels
	TabSkills
	TabHands
	TabExtensions
	TabTemplates
	TabPeers
	TabSecurity
	TabAudit
	TabUsage
	TabSettings
	TabLogs
)

const TabCount = 18

var tabNames = []string{
	"Dashboard",
	"Agents",
	"Chat",
	"Sessions",
	"Workflows",
	"Triggers",
	"Memory",
	"Channels",
	"Skills",
	"Hands",
	"Extensions",
	"Templates",
	"Peers",
	"Security",
	"Audit",
	"Usage",
	"Settings",
	"Logs",
}

func (t Tab) String() string {
	if t >= 0 && int(t) < len(tabNames) {
		return tabNames[t]
	}
	return "Unknown"
}

// =============================================================================
// Theme & Styles
// =============================================================================

type Theme struct {
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Muted      lipgloss.Color
	Background lipgloss.Color
}

var DefaultTheme = Theme{
	Primary:    lipgloss.Color("#FF6B6B"),
	Secondary:  lipgloss.Color("#4ECDC4"),
	Success:    lipgloss.Color("#51CF66"),
	Warning:    lipgloss.Color("#FFD43B"),
	Error:      lipgloss.Color("#FF6B6B"),
	Muted:      lipgloss.Color("#868E96"),
	Background: lipgloss.Color("#1A1A2E"),
}

type Styles struct {
	theme       Theme
	Title       lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	Box         lipgloss.Style
	BoxHeader   lipgloss.Style
	Success     lipgloss.Style
	Warning     lipgloss.Style
	Error       lipgloss.Style
	Muted       lipgloss.Style
}

func NewStyles(theme Theme) *Styles {
	return &Styles{
		theme: theme,
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),
		TabActive: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(theme.Primary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(theme.Primary),
		TabInactive: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(theme.Muted).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(theme.Muted),
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Secondary),
		BoxHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Secondary),
		Success: lipgloss.NewStyle().
			Foreground(theme.Success),
		Warning: lipgloss.NewStyle().
			Foreground(theme.Warning),
		Error: lipgloss.NewStyle().
			Foreground(theme.Error),
		Muted: lipgloss.NewStyle().
			Foreground(theme.Muted),
	}
}

// =============================================================================
// Backend Interface
// =============================================================================

type Agent struct {
	ID     string
	Name   string
	Status string
}

type Backend interface {
	GetAgents() ([]Agent, error)
	GetAgent(id string) (*Agent, error)
	GetVersion() (string, error)
	SpawnAgent(name, systemPrompt string) (string, error)
	DeleteAgent(id string) error
	SendMessage(agentID, message string) (string, error)
}

type InProcessBackend struct {
	kernel *kernel.Kernel
}

func NewInProcessBackend(k *kernel.Kernel) *InProcessBackend {
	return &InProcessBackend{kernel: k}
}

func (b *InProcessBackend) GetAgents() ([]Agent, error) {
	entries := b.kernel.AgentRegistry().List()
	agents := make([]Agent, 0, len(entries))
	for _, entry := range entries {
		agents = append(agents, Agent{
			ID:     entry.ID.String(),
			Name:   entry.Name,
			Status: string(entry.State),
		})
	}
	return agents, nil
}

func (b *InProcessBackend) GetAgent(id string) (*Agent, error) {
	agents, err := b.GetAgents()
	if err != nil {
		return nil, err
	}
	for _, a := range agents {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("agent not found")
}

func (b *InProcessBackend) GetVersion() (string, error) {
	return "v0.2.0", nil
}

func (b *InProcessBackend) SpawnAgent(name, systemPrompt string) (string, error) {
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant."
	}
	manifest := types.AgentManifest{
		Name:         name,
		SystemPrompt: systemPrompt,
	}
	agentID, _, err := b.kernel.SpawnAgent(manifest)
	return agentID, err
}

func (b *InProcessBackend) DeleteAgent(id string) error {
	return b.kernel.DeleteAgent(id)
}

func (b *InProcessBackend) SendMessage(agentID, message string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return b.kernel.SendMessage(ctx, agentID, message)
}

type DaemonBackend struct {
	baseURL string
}

func NewDaemonBackend(baseURL string) *DaemonBackend {
	return &DaemonBackend{baseURL: baseURL}
}

func (b *DaemonBackend) GetAgents() ([]Agent, error) {
	return []Agent{}, nil
}

func (b *DaemonBackend) GetAgent(id string) (*Agent, error) {
	return nil, nil
}

func (b *DaemonBackend) GetVersion() (string, error) {
	return "v0.2.0", nil
}

func (b *DaemonBackend) SpawnAgent(name, systemPrompt string) (string, error) {
	return "", nil
}

func (b *DaemonBackend) DeleteAgent(id string) error {
	return nil
}

func (b *DaemonBackend) SendMessage(agentID, message string) (string, error) {
	return "", nil
}

// =============================================================================
// Event System
// =============================================================================

type AppEvent interface{}

type TickEvent time.Time

type AgentsLoadedEvent struct {
	Agents []Agent
}

type ErrorEvent struct {
	Err error
}

type ChatResponseEvent struct {
	AgentID  string
	Response string
}

// =============================================================================
// Screen States
// =============================================================================

type WelcomeState struct {
	showWizard bool
}

type DashboardState struct {
	loaded     bool
	agentCount int
	uptime     time.Duration
	version    string
	provider   string
	model      string
	startTime  time.Time
}

type AgentsState struct {
	loaded   bool
	agents   []Agent
	selected int
	mode     string
	input    textinput.Model
}

type ChatMessage struct {
	Role    string
	Content string
	Time    time.Time
}

type ChatState struct {
	loaded        bool
	agents        []Agent
	selectedAgent int
	messages      []ChatMessage
	input         textinput.Model
	loading       bool
}

// =============================================================================
// Main Model
// =============================================================================

type Phase int

const (
	PhaseBoot Phase = iota
	PhaseMain
)

type Model struct {
	phase     Phase
	activeTab Tab
	tabOffset int
	width     int
	height    int
	quitting  bool
	showHelp  bool

	ctrlCPending bool
	ctrlCTick    int
	tickCount    int

	statusMessage string
	lastTick      time.Time

	styles  *Styles
	backend Backend

	welcome   WelcomeState
	dashboard DashboardState
	agents    AgentsState
	chat      ChatState
}

func NewModel(backend Backend) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter a name..."
	ti.CharLimit = 50
	ti.Width = 40

	chatTi := textinput.New()
	chatTi.Placeholder = "Type your message..."
	chatTi.CharLimit = 500
	chatTi.Width = 60
	chatTi.Focus()

	return Model{
		phase:     PhaseBoot,
		activeTab: TabDashboard,
		styles:    NewStyles(DefaultTheme),
		backend:   backend,
		lastTick:  time.Now(),
		welcome: WelcomeState{
			showWizard: false,
		},
		dashboard: DashboardState{
			version:   "v0.2.0",
			provider:  "OpenAI",
			model:     "gpt-4",
			startTime: time.Now(),
		},
		agents: AgentsState{
			mode:  "view",
			input: ti,
		},
		chat: ChatState{
			input: chatTi,
		},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("FangClaw-Go 🦊"),
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		isChatInputFocused := m.activeTab == TabChat && m.chat.input.Focused()

		if isChatInputFocused {
			if keyStr == "esc" || keyStr == "tab" ||
				keyStr == "up" || keyStr == "down" || keyStr == "k" || keyStr == "j" {
				newModel, newCmd := m.handleKeyMsg(msg)
				if newCmd != nil {
					return newModel, newCmd
				}
				return newModel, nil
			} else if keyStr == "enter" {
				newModel, newCmd := m.handleKeyMsg(msg)
				if newCmd != nil {
					return newModel, newCmd
				}
				if typedModel, ok := newModel.(Model); ok {
					m = typedModel
				}
			}
		} else {
			isSpecialKey := keyStr == "enter" || keyStr == "tab" || keyStr == "esc" ||
				keyStr == "ctrl+c" || keyStr == "q" || keyStr == "r" || keyStr == "a" || keyStr == "d" ||
				keyStr == "left" || keyStr == "right" || keyStr == "up" || keyStr == "down" ||
				keyStr == "h" || keyStr == "j" || keyStr == "k" || keyStr == "l" ||
				(keyStr >= "1" && keyStr <= "9") || keyStr == "0" || keyStr == "?"

			if isSpecialKey {
				newModel, newCmd := m.handleKeyMsg(msg)
				if newCmd != nil {
					return newModel, newCmd
				}
				return newModel, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TickEvent:
		m.tickCount++
		m.lastTick = time.Now()

		if m.ctrlCPending && m.tickCount-m.ctrlCTick > 40 {
			m.ctrlCPending = false
			m.statusMessage = ""
		}

		return m, tickCmd()

	case AgentsLoadedEvent:
		m.agents.agents = msg.Agents
		m.agents.loaded = true
		m.chat.agents = msg.Agents
		m.dashboard.agentCount = len(msg.Agents)
		return m, nil

	case ErrorEvent:
		m.statusMessage = fmt.Sprintf("Error: %v", msg.Err)
		return m, nil

	case ChatResponseEvent:
		m.chat.messages = append(m.chat.messages, ChatMessage{
			Role:    "assistant",
			Content: msg.Response,
			Time:    time.Now(),
		})
		m.chat.loading = false
		m.chat.input.Focus()
		return m, nil
	}

	if m.agents.mode == "create" {
		m.agents.input, cmd = m.agents.input.Update(msg)
		return m, cmd
	}

	if m.activeTab == TabChat && m.chat.input.Focused() {
		m.chat.input, cmd = m.chat.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	isChatTab := m.activeTab == TabChat
	isChatInputFocused := isChatTab && m.chat.input.Focused()
	isAgentsCreateActive := m.agents.mode == "create"
	keyStr := msg.String()

	switch keyStr {
	case "ctrl+c":
		if m.ctrlCPending {
			m.quitting = true
			return m, tea.Quit
		}
		m.ctrlCPending = true
		m.ctrlCTick = m.tickCount
		m.statusMessage = "Press Ctrl+C again to quit"
		return m, nil

	case "q":
		if m.phase == PhaseMain && m.agents.mode == "view" && m.activeTab != TabChat {
			m.quitting = true
			return m, tea.Quit
		}

	case "left", "h":
		if !isChatInputFocused && !isAgentsCreateActive && m.phase == PhaseMain && m.activeTab > 0 && m.agents.mode == "view" {
			m.activeTab--
			m.adjustTabOffset()
			if m.activeTab == TabChat {
				m.chat.input.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}

	case "right", "l":
		if !isChatInputFocused && !isAgentsCreateActive && m.phase == PhaseMain && m.activeTab < TabCount-1 && m.agents.mode == "view" {
			m.activeTab++
			m.adjustTabOffset()
			if m.activeTab == TabChat {
				m.chat.input.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if !isChatInputFocused && !isAgentsCreateActive && m.phase == PhaseMain && m.agents.mode == "view" {
			idx := int(keyStr[0] - '1')
			if idx >= 0 && idx < TabCount {
				m.activeTab = Tab(idx)
				m.adjustTabOffset()
				if m.activeTab == TabChat {
					m.chat.input.Focus()
					return m, textinput.Blink
				}
				return m, nil
			}
		}

	case "0":
		if !isChatInputFocused && !isAgentsCreateActive && m.phase == PhaseMain && TabCount > 9 && m.agents.mode == "view" {
			m.activeTab = Tab(9)
			m.adjustTabOffset()
			if m.activeTab == TabChat {
				m.chat.input.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}

	case "?":
		if m.agents.mode == "view" {
			m.showHelp = !m.showHelp
			return m, nil
		}

	case "tab":
		if m.phase == PhaseMain && m.activeTab == TabChat && len(m.chat.agents) > 0 {
			m.chat.selectedAgent = (m.chat.selectedAgent + 1) % len(m.chat.agents)
			return m, nil
		}

	case "enter":
		if m.phase == PhaseBoot {
			m.phase = PhaseMain
			return m, m.loadInitialData()
		} else if m.agents.mode == "create" {
			name := m.agents.input.Value()
			if name != "" {
				m.agents.mode = "view"
				m.agents.input.Reset()
				m.statusMessage = "Creating agent..."
				return m, m.spawnAgent(name)
			}
		} else if m.activeTab == TabAgents && m.agents.mode == "view" && len(m.agents.agents) > 0 {
			m.activeTab = TabChat
			m.chat.selectedAgent = m.agents.selected
			m.adjustTabOffset()
			m.chat.input.Focus()
			return m, textinput.Blink
		} else if m.activeTab == TabChat {
			if !m.chat.input.Focused() {
				m.chat.input.Focus()
				return m, textinput.Blink
			} else if !m.chat.loading {
				if len(m.chat.agents) > 0 && m.chat.selectedAgent < len(m.chat.agents) {
					message := m.chat.input.Value()
					if message != "" {
						m.chat.messages = append(m.chat.messages, ChatMessage{
							Role:    "user",
							Content: message,
							Time:    time.Now(),
						})
						m.chat.input.Reset()
						m.chat.loading = true
						agentID := m.chat.agents[m.chat.selectedAgent].ID
						return m, m.sendMessage(agentID, message)
					}
				}
			}
		}

	case "esc":
		m.ctrlCPending = false
		m.statusMessage = ""
		m.showHelp = false
		if m.agents.mode == "create" {
			m.agents.mode = "view"
			m.agents.input.Reset()
		}
		if m.activeTab == TabChat && m.chat.input.Focused() {
			m.chat.input.Blur()
			return m, nil
		}
		return m, nil

	case "r":
		if !isChatInputFocused && !isAgentsCreateActive && m.phase == PhaseMain && m.agents.mode == "view" {
			m.statusMessage = "Refreshing..."
			return m, m.refreshActiveTab()
		}

	case "a":
		if !isChatInputFocused && !isAgentsCreateActive && m.phase == PhaseMain && m.activeTab == TabAgents && m.agents.mode == "view" {
			m.agents.mode = "create"
			m.agents.input.Focus()
			m.statusMessage = "Enter agent name, press Enter to create"
			return m, textinput.Blink
		}

	case "up", "k":
		if !isAgentsCreateActive && m.phase == PhaseMain {
			if m.activeTab == TabAgents && m.agents.mode == "view" && len(m.agents.agents) > 0 {
				if m.agents.selected > 0 {
					m.agents.selected--
					return m, nil
				}
			} else if m.activeTab == TabChat && len(m.chat.agents) > 0 {
				if m.chat.selectedAgent > 0 {
					m.chat.selectedAgent--
					return m, nil
				}
			}
		}

	case "down", "j":
		if !isAgentsCreateActive && m.phase == PhaseMain {
			if m.activeTab == TabAgents && m.agents.mode == "view" && len(m.agents.agents) > 0 {
				if m.agents.selected < len(m.agents.agents)-1 {
					m.agents.selected++
					return m, nil
				}
			} else if m.activeTab == TabChat && len(m.chat.agents) > 0 {
				if m.chat.selectedAgent < len(m.chat.agents)-1 {
					m.chat.selectedAgent++
					return m, nil
				}
			}
		}

	case "d":
		if m.phase == PhaseMain && m.activeTab == TabAgents && m.agents.mode == "view" && len(m.agents.agents) > 0 {
			agent := m.agents.agents[m.agents.selected]
			m.statusMessage = fmt.Sprintf("Deleting agent: %s...", agent.Name)
			return m, m.deleteAgent(agent.ID)
		}
	}

	return m, nil
}

func (m Model) loadInitialData() tea.Cmd {
	return tea.Batch(
		m.loadAgents(),
	)
}

func (m Model) loadAgents() tea.Cmd {
	return func() tea.Msg {
		agents, err := m.backend.GetAgents()
		if err != nil {
			return ErrorEvent{Err: err}
		}
		return AgentsLoadedEvent{Agents: agents}
	}
}

func (m Model) spawnAgent(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.backend.SpawnAgent(name, "")
		if err != nil {
			return ErrorEvent{Err: err}
		}
		agents, err := m.backend.GetAgents()
		if err != nil {
			return ErrorEvent{Err: err}
		}
		return AgentsLoadedEvent{Agents: agents}
	}
}

func (m Model) deleteAgent(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.backend.DeleteAgent(id)
		if err != nil {
			return ErrorEvent{Err: err}
		}
		agents, err := m.backend.GetAgents()
		if err != nil {
			return ErrorEvent{Err: err}
		}
		return AgentsLoadedEvent{Agents: agents}
	}
}

func (m Model) sendMessage(agentID, message string) tea.Cmd {
	return func() tea.Msg {
		response, err := m.backend.SendMessage(agentID, message)
		if err != nil {
			return ErrorEvent{Err: err}
		}
		return ChatResponseEvent{AgentID: agentID, Response: response}
	}
}

func (m Model) refreshActiveTab() tea.Cmd {
	switch m.activeTab {
	case TabAgents:
		return m.loadAgents()
	}
	return nil
}

func (m Model) View() string {
	if m.quitting {
		return "Goodbye! 👋\n"
	}

	if m.showHelp {
		return m.helpView()
	}

	if m.phase == PhaseBoot {
		return m.welcomeView()
	}

	var content string
	switch m.activeTab {
	case TabDashboard:
		content = m.dashboardView()
	case TabAgents:
		content = m.agentsView()
	case TabChat:
		content = m.chatView()
	case TabSessions:
		content = m.sessionsView()
	case TabWorkflows:
		content = m.workflowsView()
	case TabTriggers:
		content = m.triggersView()
	case TabMemory:
		content = m.memoryView()
	case TabChannels:
		content = m.channelsView()
	case TabSkills:
		content = m.skillsView()
	case TabHands:
		content = m.handsView()
	case TabExtensions:
		content = m.extensionsView()
	case TabTemplates:
		content = m.templatesView()
	case TabPeers:
		content = m.peersView()
	case TabSecurity:
		content = m.securityView()
	case TabAudit:
		content = m.auditView()
	case TabUsage:
		content = m.usageView()
	case TabSettings:
		content = m.settingsView()
	case TabLogs:
		content = m.logsView()
	default:
		content = "Tab not implemented"
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		m.tabBarView(),
		lipgloss.NewStyle().
			Width(m.width).
			Height(max(0, m.height-6)).
			Render(content),
		m.footerView(),
	)
}

func (m Model) headerView() string {
	title := m.styles.Title.Render("🦊 FangClaw-Go")
	version := m.styles.Muted.Render(m.dashboard.version)
	return lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", version)
}

func (m Model) tabBarView() string {
	var tabs []string

	maxVisible := m.width / 12
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := m.tabOffset
	end := start + maxVisible
	if end > TabCount {
		end = TabCount
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		tab := Tab(i)
		var style lipgloss.Style
		if tab == m.activeTab {
			style = m.styles.TabActive
		} else {
			style = m.styles.TabInactive
		}

		label := tabNames[i]
		if i < 9 {
			label = fmt.Sprintf("%d. %s", i+1, label)
		} else if i == 9 {
			label = fmt.Sprintf("0. %s", label)
		}

		tabs = append(tabs, style.Render(label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
}

func (m Model) footerView() string {
	var parts []string

	helpKey := "?"
	if m.showHelp {
		helpKey = "? Hide help"
	}
	parts = append(parts, fmt.Sprintf("[%s] Help", helpKey))

	if m.phase == PhaseMain {
		parts = append(parts, "[←/→] Navigate")
		parts = append(parts, "[R] Refresh")
	}
	parts = append(parts, "[Q] Quit")

	if m.statusMessage != "" {
		parts = append(parts, m.statusMessage)
	}

	return m.styles.Muted.Render(strings.Join(parts, "  "))
}

func (m Model) welcomeView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(max(0, m.height-3)).
		Align(lipgloss.Center, lipgloss.Center)

	content := `
🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊

  Welcome to FangClaw-Go!
  
  Intelligent Agent Operating System
  
  Version 0.2.0

  [Enter] Continue
  [Q] Quit

🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊
`
	return style.Render(content)
}

func (m Model) helpView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(max(0, m.height-3))

	content := `
FangClaw-Go TUI Help
=====================

Navigation:
  ←/h, →/l    Switch tabs
  1-9, 0      Go to specific tab
  ↑/k, ↓/j    Select items in lists
  Enter       Select/Confirm/Send
  Esc         Cancel/Close

Agents Tab:
  [A] Create agent
  [D] Delete agent
  Enter       Select agent and go to Chat

Chat Tab:
  Tab, ↑/k, ↓/j  Switch agent
  Enter           Send message / Focus input
  Esc             Unfocus input
  Type and press Enter to chat

Actions:
  ?           Toggle help
  R           Refresh
  Q, Ctrl+C   Quit (Ctrl+C twice for force quit)

Tabs:
  Dashboard   System overview
  Agents      Agent management
  Chat        Chat interface
  ...and more

Press Esc to close help
`
	return style.Render(content)
}

func (m Model) dashboardView() string {
	style := lipgloss.NewStyle().Padding(1, 2)

	uptime := time.Since(m.dashboard.startTime).Round(time.Second)
	agentCount := len(m.agents.agents)

	content := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════╗
║                    📊 System Dashboard                        ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Status:     ████████████████  Running                       ║
║  Uptime:     %-50s ║
║  Agents:     %-50d ║
║  Channels:   0                                               ║
║  Hands:      0                                               ║
║                                                               ║
║  Today's Usage:                                              ║
║  • LLM Tokens:     0                                        ║
║  • LLM Requests:   0                                        ║
║  • Tool Calls:     0                                        ║
║  • Messages:       0                                        ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`, uptime, agentCount)

	return style.Render(content)
}

func (m Model) agentsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)

	if m.agents.mode == "create" {
		form := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════╗
║                     🤖 Create Agent                          ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Agent Name: %-50s ║
║                                                               ║
║  [Enter] Create  [Esc] Cancel                                ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`, m.agents.input.View())
		return style.Render(form)
	}

	if !m.agents.loaded {
		return style.Render("Loading agents...")
	}

	if len(m.agents.agents) == 0 {
		return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                     🤖 Agents                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No agents running.                                           ║
║                                                               ║
║  [A] Create agent                                             ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
	}

	var table strings.Builder
	table.WriteString("╔══════════════════════════════════════════════════════════════╗\n")
	table.WriteString("║                     🤖 Agents                                  ║\n")
	table.WriteString("╠══════════════════════════════════════════════════════════════╣\n")
	table.WriteString("║                                                               ║\n")
	table.WriteString("║  ID                  Name            Status                  ║\n")
	table.WriteString("║  ───────────────────────────────────────────────────────────  ║\n")

	for i, agent := range m.agents.agents {
		prefix := "  "
		if i == m.agents.selected {
			prefix = "▶ "
		}
		idShort := truncateText(agent.ID, 20)
		nameShort := truncateText(agent.Name, 16)
		statusShort := truncateText(agent.Status, 10)
		table.WriteString(fmt.Sprintf("║  %s%-20s  %-16s %-10s        ║\n", prefix, idShort, nameShort, statusShort))
	}

	table.WriteString("║                                                               ║\n")
	table.WriteString("║  [A] Create  [D] Delete  [Enter] Chat  [↑/↓] Select        ║\n")
	table.WriteString("║                                                               ║\n")
	table.WriteString("╚══════════════════════════════════════════════════════════════╝")

	return style.Render(table.String())
}

func (m Model) chatView() string {
	style := lipgloss.NewStyle().Padding(1, 2)

	if len(m.chat.agents) == 0 {
		return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                     💬 Chat                                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No agents available.                                         ║
║                                                               ║
║  Go to Agents tab to create an agent first.                  ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
	}

	var agentList strings.Builder
	for i, agent := range m.chat.agents {
		prefix := "  "
		if i == m.chat.selectedAgent {
			prefix = "▶ "
		}
		agentList.WriteString(fmt.Sprintf("%s%s\n", prefix, agent.Name))
	}

	var messages strings.Builder
	if len(m.chat.messages) == 0 {
		messages.WriteString("  No messages yet. Start a conversation!\n")
	} else {
		for _, msg := range m.chat.messages {
			role := "You"
			if msg.Role == "assistant" {
				role = m.chat.agents[m.chat.selectedAgent].Name
			}
			messages.WriteString(fmt.Sprintf("\n  %s:\n", role))
			wrapped := wrapText(msg.Content, 50)
			for _, line := range wrapped {
				messages.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
	}

	inputPrompt := ""
	if m.chat.loading {
		inputPrompt = "  [Agent is thinking...]"
	} else {
		inputPrompt = fmt.Sprintf("  You: %s", m.chat.input.View())
	}

	content := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════╗
║                     💬 Chat                                   ║
╠══════════════════════════════════════════════════════════════╣
║  Agents:                          │  Conversation:            ║
║  ──────────────────────────────────┼───────────────────────────║
║                                   │                           ║
%s
║                                   │                           ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
%s
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`, padRight(agentList.String(), 35), padRight(messages.String()+"\n"+inputPrompt, 50))

	return style.Render(content)
}

func (m Model) sessionsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                   📝 Sessions                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No active sessions.                                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) workflowsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🔄 Workflows                                 ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No workflows defined.                                        ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) triggersView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  ⚡ Triggers                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No triggers defined.                                         ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) memoryView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🧠 Memory                                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Memory is empty.                                             ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) channelsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📡 Channels                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No channels configured.                                      ║
║                                                               ║
║  Available types: Telegram, Discord, Slack, WhatsApp, ...   ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) skillsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🛠️ Skills                                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No skills installed.                                         ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) handsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  ✋ Hands                                      ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No Hands installed.                                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) extensionsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🧩 Extensions                                ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No extensions installed.                                     ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) templatesView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📄 Templates                                 ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No templates defined.                                        ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) peersView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🌐 Peers                                     ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No peers connected.                                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) securityView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🔒 Security                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Capabilities: 0                                             ║
║  Approvals:    0                                             ║
║  Audit Log:    0 entries                                     ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) auditView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📋 Audit                                     ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No audit entries.                                            ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) usageView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📊 Usage                                     ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No usage data yet.                                           ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) settingsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  ⚙️ Settings                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Configuration settings coming soon!                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m Model) logsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	return style.Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📜 Logs                                      ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No logs available.                                           ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func (m *Model) adjustTabOffset() {
	maxVisible := m.width / 12
	if maxVisible < 5 {
		maxVisible = 5
	}

	activeIdx := int(m.activeTab)

	if activeIdx < m.tabOffset {
		m.tabOffset = activeIdx
	} else if activeIdx >= m.tabOffset+maxVisible {
		m.tabOffset = activeIdx - maxVisible + 1
	}

	if m.tabOffset < 0 {
		m.tabOffset = 0
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return TickEvent(t)
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func wrapText(text string, width int) []string {
	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)
	return lines
}

func padRight(s string, width int) string {
	lines := strings.Split(s, "\n")
	var result strings.Builder
	for _, line := range lines {
		if len(line) > width {
			result.WriteString(line[:width])
		} else {
			result.WriteString(line)
			result.WriteString(strings.Repeat(" ", width-len(line)))
		}
		result.WriteString("\n")
	}
	return result.String()
}

func Run(backend Backend) error {
	p := tea.NewProgram(NewModel(backend), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
	return nil
}
