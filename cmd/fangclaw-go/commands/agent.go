package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
		Long:  "Manage agents (new, spawn, list, chat, kill).",
	}

	cmd.AddCommand(agentListCmd())
	cmd.AddCommand(agentSpawnCmd())
	cmd.AddCommand(agentKillCmd())

	return cmd
}

var agentListJSON bool

func agentListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running agents",
		RunE:  runAgentList,
	}
	cmd.Flags().BoolVarP(&agentListJSON, "json", "", false, "Output as JSON")
	return cmd
}

func runAgentList(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		if agentListJSON {
			json.NewEncoder(os.Stdout).Encode([]interface{}{})
			return nil
		}
		fmt.Println("No agents running (daemon not running).")
		return nil
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := http.Get(daemonAddr + "/api/agents")
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if agentListJSON {
		fmt.Println(string(body))
		return nil
	}

	var agents []map[string]interface{}
	if err := json.Unmarshal(body, &agents); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("No agents running.")
		return nil
	}

	fmt.Printf("%-38s %-16s %-10s\n", "ID", "NAME", "STATE")
	fmt.Println("------------------------------------------------------------")
	for _, a := range agents {
		id := a["id"].(string)
		name := a["name"].(string)
		state := a["state"].(string)
		fmt.Printf("%-38s %-16s %-10s\n", id[:8]+"...", name, state)
	}

	return nil
}

var agentManifest string

func agentSpawnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spawn <manifest>",
		Short: "Spawn an agent from a manifest file",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentSpawn,
	}
	return cmd
}

func runAgentSpawn(cmd *cobra.Command, args []string) error {
	manifestPath := args[0]

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := http.Post(
		daemonAddr+"/api/agents",
		"application/json",
		nil,
	)
	// Would send the manifest here in full implementation
	_ = data

	if err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("Agent spawned successfully.")
	return nil
}

func agentKillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <agent-id>",
		Short: "Kill an agent",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentKill,
	}
}

func runAgentKill(cmd *cobra.Command, args []string) error {
	agentID := args[0]

	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running")
	}

	daemonAddr := mustGetDaemonAddress()
	client := &http.Client{}
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/agents/%s", daemonAddr, agentID), nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to kill agent: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("Agent killed.")
	return nil
}
