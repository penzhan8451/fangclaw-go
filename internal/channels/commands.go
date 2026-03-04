package channels

import (
	"fmt"
	"strings"
)

// handleCommand handles slash commands like /agents, /agent, /help, etc.
func (m *BridgeManager) handleCommand(_ Adapter /*Reserved*/, msg *Message, cmd string, args []string) string {
	switch cmd {
	case "start":
		agents, err := m.handle.ListAgents(m.ctx)
		if err != nil {
			return fmt.Sprintf("Error listing agents: %v", err)
		}

		msg := "Welcome to OpenFang! I connect you to AI agents.\n\nAvailable agents:\n"
		if len(agents) == 0 {
			msg += "  (none running)\n"
		} else {
			for _, agent := range agents {
				msg += fmt.Sprintf("  - %s\n", agent.Name)
			}
		}
		msg += "\nCommands:\n/agents - list agents\n/agent <name> - select an agent\n/help - show this help"
		return msg

	case "help":
		return "FangClaw-Go Bot Commands:\n" +
			"\n" +
			"Session:\n" +
			"/agents - list running agents\n" +
			"/agent <name> - select which agent to talk to\n" +
			"/help - show this help\n" +
			"\n" +
			"/start - show welcome message"

	case "agents":
		agents, err := m.handle.ListAgents(m.ctx)
		if err != nil {
			return fmt.Sprintf("Error listing agents: %v", err)
		}
		if len(agents) == 0 {
			return "No agents running."
		}
		msg := "Running agents:\n"
		for _, agent := range agents {
			msg += fmt.Sprintf("  - %s\n", agent.Name)
		}
		return msg

	case "agent":
		if len(args) == 0 {
			return "Usage: /agent <name> or /agent default"
		}
		if args[0] == "default" {
			// 重置为系统默认，清除用户自定义设置
			m.router.SetUserDefault(msg.Sender, "")
			return "Reset to default agent"
		}
		agentName := args[0]
		agentID, found := m.handle.FindAgentByName(m.ctx, agentName)
		if !found {
			return fmt.Sprintf("Agent '%s' not found.", agentName)
		}
		m.router.SetUserDefault(msg.Sender, agentID)
		return fmt.Sprintf("Now talking to agent: %s", agentName)

	default:
		return fmt.Sprintf("Unknown command: /%s", cmd)
	}
}

// isCommand checks if a message is a slash command.
func isCommand(text string) (string, []string, bool) {
	if !strings.HasPrefix(text, "/") {
		return "", nil, false
	}

	parts := strings.Fields(text[1:])
	if len(parts) == 0 {
		return "", nil, false
	}

	cmd := parts[0]
	var args []string

	// 对于 /agent 命令
	if cmd == "agent" && len(parts) > 1 {
		// 检查第二个词是不是 "default"
		if parts[1] == "default" {
			// 如果是 "default"，单独作为一个参数
			args = []string{"default"}
		} else {
			// 否则，将剩余部分合并为一个完整的agent名称
			args = []string{strings.Join(parts[1:], " ")}
		}
	} else {
		args = parts[1:]
	}

	validCommands := map[string]bool{
		"start":  true,
		"help":   true,
		"agents": true,
		"agent":  true,
	}

	if validCommands[cmd] {
		return cmd, args, true
	}

	return "", nil, false
}
