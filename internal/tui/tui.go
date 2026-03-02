// Package tui provides a terminal user interface for OpenFang.
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab represents the available TUI tabs.
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

// Model represents the TUI application state.
type Model struct {
	activeTab     Tab
	tabOffset     int
	width         int
	height        int
	quitting      bool
	showHelp      bool
	showWelcome   bool
	showWizard    bool
	ctrlCPending  bool
	ctrlCTick     int
	tickCount     int
	kernelReady   bool
	statusMessage string
	lastTick      time.Time
}

// NewModel creates a new TUI model.
func NewModel() Model {
	return Model{
		activeTab:   TabDashboard,
		showWelcome: true,
		lastTick:    time.Now(),
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("OpenFang 🦊"),
		tickCmd(),
	)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.ctrlCPending {
				m.quitting = true
				return m, tea.Quit
			}
			m.ctrlCPending = true
			m.ctrlCTick = m.tickCount
			m.statusMessage = "Press Ctrl+C again to quit"
			return m, nil

		case "left", "h":
			if m.activeTab > 0 {
				m.activeTab--
				m.adjustTabOffset()
			}

		case "right", "l":
			if m.activeTab < TabCount-1 {
				m.activeTab++
				m.adjustTabOffset()
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < TabCount {
				m.activeTab = Tab(idx)
				m.adjustTabOffset()
			}

		case "0":
			if TabCount > 9 {
				m.activeTab = Tab(9)
				m.adjustTabOffset()
			}

		case "?":
			m.showHelp = !m.showHelp

		case "enter":
			if m.showWelcome || m.showWizard {
				m.showWelcome = false
				m.showWizard = false
			}

		case "esc":
			m.ctrlCPending = false
			m.statusMessage = ""
			m.showHelp = false
			m.showWelcome = false
			m.showWizard = false

		case "r":
			m.statusMessage = "Refreshing..."
			return m, tea.Batch(tickCmd())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.tickCount++
		m.lastTick = time.Now()

		if m.ctrlCPending && m.tickCount-m.ctrlCTick > 40 {
			m.ctrlCPending = false
			m.statusMessage = ""
		}

		return m, tickCmd()
	}

	return m, nil
}

// View renders the model.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye! 👋\n"
	}

	if m.showWelcome {
		return m.welcomeView()
	}

	if m.showHelp {
		return m.helpView()
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
			Height(m.height-6).
			Render(content),
		m.footerView(),
	)
}

// headerView renders the header.
func (m Model) headerView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B6B")).
		Render("🦊 OpenFang")

	version := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666")).
		Render("v0.2.0")

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		"  ",
		version,
	)
}

// tabBarView renders the tab bar.
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
		style := lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.NormalBorder(), false, false, true, false)

		if tab == m.activeTab {
			style = style.
				Foreground(lipgloss.Color("#FF6B6B")).
				BorderForeground(lipgloss.Color("#FF6B6B"))
		} else {
			style = style.
				Foreground(lipgloss.Color("#666"))
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

// footerView renders the footer.
func (m Model) footerView() string {
	var parts []string

	helpKey := "?"
	if m.showHelp {
		helpKey = "? Hide help"
	}
	parts = append(parts, fmt.Sprintf("[%s] Help", helpKey))

	parts = append(parts, "[←/→] Navigate")
	parts = append(parts, "[Q] Quit")

	if m.statusMessage != "" {
		parts = append(parts, m.statusMessage)
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666")).
		Render(strings.Join(parts, "  "))
}

// welcomeView renders the welcome screen.
func (m Model) welcomeView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height-3).
		Align(lipgloss.Center, lipgloss.Center)

	content := fmt.Sprintf(`
🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊

  Welcome to OpenFang!
  
  Intelligent Agent Operating System
  
  Version 0.2.0

  [Enter] Continue
  [W] Setup Wizard
  [Q] Quit

🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊🦊
`)

	return style.Render(content)
}

// helpView renders the help screen.
func (m Model) helpView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - 3)

	content := `
OpenFang TUI Help
==================

Navigation:
  ←/h, →/l    Switch tabs
  1-9, 0      Go to specific tab
  Enter       Select/Confirm

Actions:
  ?           Toggle help
  R           Refresh
  Q, Ctrl+C   Quit (Ctrl+C twice for force quit)

Tabs:
  Dashboard   System overview
  Agents      Agent management
  Chat        Chat interface
  Channels    Channel configuration
  Hands       Hand packages
  ...and more

Press Esc to close help
`

	return style.Render(content)
}

// dashboardView renders the dashboard.
func (m Model) dashboardView() string {
	style := lipgloss.NewStyle().Padding(1, 2)

	uptime := time.Since(m.lastTick).Round(time.Second)

	content := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════╗
║                    📊 System Dashboard                        ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Status:     ████████████████  Running                       ║
║  Uptime:     %-50s ║
║  Agents:     0                                               ║
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
`, uptime)

	return style.Render(content)
}

// agentsView renders the agents tab.
func (m Model) agentsView() string {
	style := lipgloss.NewStyle().Padding(1, 2)

	content := `
╔══════════════════════════════════════════════════════════════╗
║                     🤖 Agents                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No agents running.                                           ║
║                                                               ║
║  [A] Create agent  [S] Start agent  [D] Stop agent         ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`

	return style.Render(content)
}

// chatView renders the chat tab.
func (m Model) chatView() string {
	style := lipgloss.NewStyle().Padding(1, 2)

	content := `
╔══════════════════════════════════════════════════════════════╗
║                     💬 Chat                                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Select an agent first to start chatting.                    ║
║                                                               ║
║  Go to Agents tab to create or select an agent.              ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`

	return style.Render(content)
}

// sessionsView renders the sessions tab.
func (m Model) sessionsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                   📝 Sessions                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No active sessions.                                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// workflowsView renders the workflows tab.
func (m Model) workflowsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🔄 Workflows                                 ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No workflows defined.                                        ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// triggersView renders the triggers tab.
func (m Model) triggersView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  ⚡ Triggers                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No triggers defined.                                         ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// memoryView renders the memory tab.
func (m Model) memoryView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🧠 Memory                                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Memory is empty.                                             ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// channelsView renders the channels tab.
func (m Model) channelsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
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

// skillsView renders the skills tab.
func (m Model) skillsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🛠️ Skills                                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No skills installed.                                         ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// handsView renders the hands tab.
func (m Model) handsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  ✋ Hands                                      ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No Hands installed.                                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// extensionsView renders the extensions tab.
func (m Model) extensionsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🧩 Extensions                                ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No extensions installed.                                     ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// templatesView renders the templates tab.
func (m Model) templatesView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📄 Templates                                 ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No templates defined.                                        ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// peersView renders the peers tab.
func (m Model) peersView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  🌐 Peers                                     ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No peers connected.                                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// securityView renders the security tab.
func (m Model) securityView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
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

// auditView renders the audit tab.
func (m Model) auditView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📋 Audit                                     ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No audit entries.                                            ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// usageView renders the usage tab.
func (m Model) usageView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📊 Usage                                     ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No usage data yet.                                           ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// settingsView renders the settings tab.
func (m Model) settingsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  ⚙️ Settings                                  ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Configuration settings coming soon!                          ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// logsView renders the logs tab.
func (m Model) logsView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(`
╔══════════════════════════════════════════════════════════════╗
║                  📜 Logs                                      ║
╠══════════════════════════════════════════════════════════════╣
║                                                               ║
║  No logs available.                                           ║
║                                                               ║
╚══════════════════════════════════════════════════════════════╝
`)
}

// adjustTabOffset adjusts the tab offset to keep the active tab visible.
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

// tickMsg represents a tick message.
type tickMsg time.Time

// tickCmd sends a tick message every 50ms.
func tickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Run starts the TUI application.
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
	return nil
}
