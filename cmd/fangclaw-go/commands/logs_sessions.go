package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func logsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail the OpenFang log file",
		RunE:  runLogs,
	}

	cmd.Flags().Int64P("lines", "n", 50, "Number of lines to show")
	cmd.Flags().BoolP("follow", "f", false, "Follow log output in real time")

	return cmd
}

var logsLines int64
var logsFollow bool

func runLogs(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	logFile := filepath.Join(homeDir, ".fangclaw-go", "daemon.log")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Println("No log file found. Daemon may not have started yet.")
		return nil
	}

	if logsFollow {
		// Use tail -f
		exec.Command("tail", "-f", "-n", fmt.Sprintf("%d", logsLines), logFile).Run()
	} else {
		// Use tail -n
		out, err := exec.Command("tail", "-n", fmt.Sprintf("%d", logsLines), logFile).Output()
		if err != nil {
			return fmt.Errorf("failed to read log file: %w", err)
		}
		fmt.Print(string(out))
	}

	return nil
}

func sessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List conversation sessions",
		RunE:  runSessions,
	}

	cmd.Flags().StringVar(&sessionsAgent, "agent", "", "Agent name or ID to filter by")
	cmd.Flags().BoolVar(&sessionsJSON, "json", false, "Output as JSON")

	return cmd
}

var sessionsAgent string
var sessionsJSON bool

func runSessions(cmd *cobra.Command, args []string) error {
	// Demo data
	sessions := []map[string]interface{}{
		{
			"id":            "session-001",
			"agent_id":      "agent-123",
			"agent_name":    "assistant",
			"created_at":    "2026-02-28T10:00:00Z",
			"message_count": 15,
		},
		{
			"id":            "session-002",
			"agent_id":      "agent-456",
			"agent_name":    "coder",
			"created_at":    "2026-02-28T11:30:00Z",
			"message_count": 8,
		},
	}

	// Filter by agent if specified
	if sessionsAgent != "" {
		filtered := make([]map[string]interface{}, 0)
		for _, s := range sessions {
			if s["agent_id"] == sessionsAgent || s["agent_name"] == sessionsAgent {
				filtered = append(filtered, s)
			}
		}
		sessions = filtered
	}

	if sessionsJSON {
		json.NewEncoder(os.Stdout).Encode(sessions)
		return nil
	}

	fmt.Printf("%-40s %-20s %-25s %s\n", "ID", "AGENT", "CREATED AT", "MESSAGES")
	fmt.Println("---------------------------------------------------------------------------------------------------")
	for _, s := range sessions {
		fmt.Printf("%-40s %-20s %-25s %d\n", s["id"], s["agent_name"], s["created_at"], s["message_count"])
	}

	return nil
}
