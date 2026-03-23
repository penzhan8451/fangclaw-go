// Package tui provides the interactive TUI for fangclaw-go using Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Tab represents a tab in the application.
type Tab int

const (
	DashboardTab Tab = iota
	ChatTab
	AgentsTab
)

func (t Tab) String() string {
	switch t {
	case DashboardTab:
		return "Dashboard"
	case ChatTab:
		return "Chat"
	case AgentsTab:
		return "Agents"
	default:
		return ""
	}
}

// Model is the main application model.
type Model struct {
	tabs      []Tab
	activeTab Tab
	quitting  bool

	// Tab-specific data
	dashboardData DashboardModel
	chatData      ChatModel
	agentsData    AgentsModel
}

// DashboardModel holds dashboard state.
type DashboardModel struct {
	agentCount int
	uptime     string
	version    string
	provider   string
	model      string
	loaded     bool
}

// ChatModel holds chat state.
type ChatModel struct {
	messages    []ChatMessage
	input       string
	cursorBlink bool
	focused     bool // true = Input Mode (typing), false = Navigation Mode
}

// ChatMessage represents a chat message.
type ChatMessage struct {
	Role      string // "user", "assistant", "system"
	Content   string
	Timestamp string
}

// NewChatModel creates a new chat model with initial messages.
func NewChatModel() ChatModel {
	return ChatModel{
		messages: []ChatMessage{
			{Role: "system", Content: "Welcome to FangClaw Chat! Press Enter or 'i' to start typing.", Timestamp: "Now"},
		},
		input:       "",
		cursorBlink: false,
		focused:     false, // Start in Navigation Mode
	}
}

// handleChatInputMode handles ALL keyboard input when in Chat Input Mode (typing)
// This is LAYER 3 - highest priority, all keys go to chat input except Esc/Ctrl+C
func (m Model) handleChatInputMode(msg tea.Msg) (ChatModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Global escape - always works, even in input mode
		case "esc":
			// Exit Input Mode, return to Navigation Mode
			m.chatData.focused = false
			m.chatData.input = "" // Clear any partial input
			return m.chatData, nil

		// Send message but STAY in Input Mode (for continuous chatting)
		case "enter":
			// Check for exit commands BEFORE sending
			inputLower := strings.ToLower(strings.TrimSpace(m.chatData.input))
			if inputLower == "exit" || inputLower == "bye" || inputLower == "quit" {
				// Exit Input Mode without sending as a chat message
				m.chatData.focused = false
				m.chatData.input = ""
				// Add system message about exiting
				m.chatData.messages = append(m.chatData.messages, ChatMessage{
					Role:      "system",
					Content:   "Exited chat mode. Press Enter or 'i' to start chatting again.",
					Timestamp: "Now",
				})
				return m.chatData, nil
			}

			// Normal message sending
			if strings.TrimSpace(m.chatData.input) != "" {
				// Add user message
				m.chatData.messages = append(m.chatData.messages, ChatMessage{
					Role:      "user",
					Content:   m.chatData.input,
					Timestamp: "Now",
				})

				// Save input for response generation
				userInput := m.chatData.input

				// Clear input field (but stay focused!)
				m.chatData.input = ""

				// Simulate assistant response (async)
				return m.chatData, func() tea.Msg {
					time.Sleep(500 * time.Millisecond)
					response := generateResponseFromInput(userInput)
					return ChatMessage{
						Role:      "assistant",
						Content:   response,
						Timestamp: "Now",
					}
				}
			}
			return m.chatData, nil

		// Standard text editing
		case "backspace":
			if len(m.chatData.input) > 0 {
				m.chatData.input = m.chatData.input[:len(m.chatData.input)-1]
			}
			return m.chatData, nil

		case "ctrl+u":
			// Clear entire input
			m.chatData.input = ""
			return m.chatData, nil

		// ALL OTHER KEYS - Type into chat input
		// This includes: a-z, 0-9, ?, !, @, #, $, %, etc.
		// Critically: 1, 2, 3, r, q all go to input (don't trigger shortcuts)
		default:
			// Allow all printable ASCII characters (space through tilde)
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.chatData.input += msg.String()
			}
			return m.chatData, nil
		}

	case ChatMessage:
		// Add incoming message from async response
		m.chatData.messages = append(m.chatData.messages, msg)
		return m.chatData, nil
	}

	return m.chatData, nil
}

// handleDashboardView handles view-specific actions for Dashboard tab (LAYER 2)
func (m Model) handleDashboardView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Refresh dashboard data
		m.dashboardData.loaded = false
		return m, nil
	}
	return m, nil
}

// handleChatView handles view-specific actions for Chat tab when NOT in Input Mode (LAYER 2)
func (m Model) handleChatView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Refresh chat history
		m.chatData.messages = append(m.chatData.messages, ChatMessage{
			Role:      "system",
			Content:   "Chat refreshed",
			Timestamp: "Now",
		})
		return m, nil

	case "enter", "i", "I":
		// Enter Input Mode to start typing
		m.chatData.focused = true
		return m, nil
	}
	return m, nil
}

// handleAgentsView handles view-specific actions for Agents tab (LAYER 2)
func (m Model) handleAgentsView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Refresh agents list
		m.agentsData.loaded = false
		return m, nil
	}
	return m, nil
}

// generateResponseFromInput generates a simple response based on user input.
func generateResponseFromInput(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))

	// Simple keyword-based responses
	switch {
	case strings.Contains(lower, "hello") || strings.Contains(lower, "hi"):
		return "Hello! How can I help you today?"
	case strings.Contains(lower, "help"):
		return "I'm here to help! You can ask me questions or give me commands."
	case strings.Contains(lower, "weather"):
		return "I don't have access to weather data yet, but I can check the forecast for you in the future!"
	case strings.Contains(lower, "time"):
		return fmt.Sprintf("Current time is %s", time.Now().Format("3:04 PM"))
	case strings.Contains(lower, "date"):
		return fmt.Sprintf("Today is %s", time.Now().Format("Monday, January 2, 2006"))
	case strings.Contains(lower, "thank"):
		return "You're welcome! Is there anything else I can help you with?"
	case strings.Contains(lower, "bye"):
		return "Goodbye! Feel free to come back if you need help."
	default:
		return fmt.Sprintf("You said: \"%s\". That's interesting! Tell me more.", input)
	}
}

// AgentsModel holds agents state.
type AgentsModel struct {
	agents []AgentInfo
	loaded bool
}

// AgentInfo represents an agent.
type AgentInfo struct {
	ID     string
	Name   string
	Status string
}

// NewModel creates a new application model.
func NewModel() Model {
	return Model{
		tabs:      []Tab{DashboardTab, ChatTab, AgentsTab},
		activeTab: DashboardTab,
		dashboardData: DashboardModel{
			agentCount: 3,
			uptime:     "2h 30m",
			version:    "v0.2.0",
			provider:   "OpenAI",
			model:      "gpt-4",
			loaded:     true,
		},
		chatData: NewChatModel(),
		agentsData: AgentsModel{
			agents: []AgentInfo{
				{ID: "agent-1", Name: "Assistant", Status: "running"},
				{ID: "agent-2", Name: "Coder", Status: "stopped"},
				{ID: "agent-3", Name: "Researcher", Status: "running"},
			},
			loaded: true,
		},
	}
}

// Init initializes the application.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// LAYER 3: Input Mode - Highest priority when active
	// If in Chat tab AND chat is focused (typing), ALL keys go to chat input
	// except Esc and Ctrl+C which always work globally
	if m.activeTab == ChatTab && m.chatData.focused {
		updatedChat, cmd := m.handleChatInputMode(msg)
		m.chatData = updatedChat
		return m, cmd
	}

	// LAYER 1 & 2: Global Navigation + View-Specific Actions
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ========== LAYER 1: GLOBAL NAVIGATION (Always available when not in Input Mode) ==========
		switch msg.String() {
		// Quit application
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		// Tab switching - Global keys 1, 2, 3
		case "1":
			m.activeTab = DashboardTab
			return m, nil
		case "2":
			m.activeTab = ChatTab
			return m, nil
		case "3":
			m.activeTab = AgentsTab
			return m, nil

		// Cycle tabs with Tab / Shift+Tab
		case "tab":
			m.activeTab = Tab((int(m.activeTab) + 1) % len(m.tabs))
			return m, nil
		case "shift+tab":
			m.activeTab = Tab((int(m.activeTab) - 1 + len(m.tabs)) % len(m.tabs))
			return m, nil
		}

		// ========== LAYER 2: VIEW-SPECIFIC ACTIONS (When not in Input Mode) ==========
		switch m.activeTab {
		case DashboardTab:
			return m.handleDashboardView(msg)
		case ChatTab:
			return m.handleChatView(msg)
		case AgentsTab:
			return m.handleAgentsView(msg)
		}

	case tea.Cmd:
		// Execute command (for async operations)
		return m.Update(msg())

	case ChatMessage:
		// Handle incoming chat message (from async response)
		if m.activeTab == ChatTab {
			m.chatData.messages = append(m.chatData.messages, msg)
		}
		return m, nil
	}

	return m, nil
}

// View renders the UI.
func (m Model) View() string {
	var b strings.Builder

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Content based on active tab
	switch m.activeTab {
	case DashboardTab:
		b.WriteString(m.renderDashboard())
	case ChatTab:
		b.WriteString(m.renderChat())
	case AgentsTab:
		b.WriteString(m.renderAgents())
	}

	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderTabBar renders the tab navigation.
func (m Model) renderTabBar() string {
	var parts []string

	for i, tab := range m.tabs {
		var part string
		if tab == m.activeTab {
			// Add ✏️ icon when Chat is in Input Mode
			if tab == ChatTab && m.chatData.focused {
				part = fmt.Sprintf("[ %s ✏️ ]", tab)
			} else {
				part = fmt.Sprintf("[ %s ]", tab)
			}
		} else {
			part = fmt.Sprintf("  %s  ", tab)
		}

		if i < len(m.tabs)-1 {
			part += " | "
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, "")
}

// renderDashboard renders the dashboard view.
func (m Model) renderDashboard() string {
	if !m.dashboardData.loaded {
		return "Loading..."
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("┌───── Agents ─────┐┌───── Uptime ─────┐┌───── Version ────┐┌──── Provider ────┐┌────── Model ─────┐\n"))
	b.WriteString(fmt.Sprintf("│ %-16s ││ %-16s ││ %-16s ││ %-16s ││ %-16s │\n",
		fmt.Sprintf("%d", m.dashboardData.agentCount),
		m.dashboardData.uptime,
		m.dashboardData.version,
		m.dashboardData.provider,
		m.dashboardData.model))
	b.WriteString(fmt.Sprintf("│                  ││                  ││                  ││                  ││                  │\n"))
	b.WriteString(fmt.Sprintf("│                  ││                  ││                  ││                  ││                  │\n"))
	b.WriteString(fmt.Sprintf("└──────────────────┘└──────────────────┘└──────────────────┘└──────────────────┘└──────────────────┘"))

	return b.String()
}

// renderChat renders the chat view.
func (m Model) renderChat() string {
	var b strings.Builder

	// Messages area
	b.WriteString("Messages:\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n")

	for i, msg := range m.chatData.messages {
		var prefix string
		switch msg.Role {
		case "user":
			prefix = "👤 You"
		case "assistant":
			prefix = "🤖 Assistant"
		case "system":
			prefix = "⚙️ System"
		}

		b.WriteString(fmt.Sprintf("%s [%s]: %s\n", prefix, msg.Timestamp, msg.Content))

		// Add separator between messages
		if i < len(m.chatData.messages)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n")

	// Input area - show different prompts based on focus mode
	if m.chatData.focused {
		// LAYER 3: Input Mode - typing a message
		b.WriteString(fmt.Sprintf("\n✏️  Type: %s_", m.chatData.input))
		b.WriteString("\n\n[Enter] Send [Esc] Exit chat mode [Backspace] Delete [Ctrl+U] Clear")
		b.WriteString("\n💡 Tip: In chat mode, ALL keys type (including 1-3, r, q)")
	} else {
		// LAYER 2: Navigation Mode in Chat tab
		b.WriteString(fmt.Sprintf("\n> _"))
		b.WriteString("\n\n[Enter] or [i] Start chatting [r] Refresh [q] Quit [1-3] Switch tabs")
		b.WriteString("\n💡 In chat mode: all keys type freely (even 1-3, r, q)")
	}

	return b.String()
}

// renderAgents renders the agents view.
func (m Model) renderAgents() string {
	if !m.agentsData.loaded {
		return "Loading agents..."
	}

	var b strings.Builder

	b.WriteString("ID              | Name       | Status\n")
	b.WriteString("----------------|------------|----------\n")

	for _, agent := range m.agentsData.agents {
		statusColor := "yellow"
		if agent.Status == "running" {
			statusColor = "green"
		} else if agent.Status == "stopped" {
			statusColor = "red"
		}

		b.WriteString(fmt.Sprintf("%-15s | %-10s | %s\n", agent.ID, agent.Name, statusColor))
	}

	return b.String()
}

// renderFooter renders the footer with help text.
func (m Model) renderFooter() string {
	return "Press 'r' to refresh • Press 1-3 to switch tabs • Press 'q' to quit"
}

// Run starts the TUI application.
func Run() error {
	p := tea.NewProgram(NewModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
	return nil
}
