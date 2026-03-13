package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var statusJSON bool

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show kernel status",
		Long:  "Display the current status of the OpenFang kernel.",
		RunE:  runStatus,
	}

	cmd.Flags().BoolVarP(&statusJSON, "json", "", false, "Output as JSON")

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Try to connect to daemon
	if isDaemonRunning() {
		daemonAddr := mustGetDaemonAddress()
		resp, err := http.Get(daemonAddr + "/api/status")
		if err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if statusJSON {
			fmt.Println(string(body))
			return nil
		}

		var status struct {
			Status     string `json:"status"`
			Version    string `json:"version"`
			ListenAddr string `json:"listen_addr"`
			AgentCount int    `json:"agent_count"`
			ModelCount int    `json:"model_count"`
			Uptime     string `json:"uptime"`
		}
		if err := json.Unmarshal(body, &status); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		fmt.Printf("Status:     %s\n", status.Status)
		fmt.Printf("Version:    %s\n", status.Version)
		fmt.Printf("API:        http://%s\n", status.ListenAddr)
		fmt.Printf("Agents:     %d\n", status.AgentCount)
		fmt.Printf("Models:     %d\n", status.ModelCount)
		fmt.Printf("Uptime:     %s\n", status.Uptime)
		return nil
	}

	// Not running - show local info
	if statusJSON {
		json.NewEncoder(os.Stdout).Encode(map[string]string{
			"status": "stopped",
		})
		return nil
	}

	fmt.Println("Status: stopped")
	fmt.Println("Run 'fangclaw-go start' to start the daemon.")

	return nil
}
