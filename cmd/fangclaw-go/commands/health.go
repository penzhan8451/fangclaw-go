package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var healthJSON bool

func healthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Quick daemon health check",
		Long:  "Check if the daemon is running and healthy.",
		RunE:  runHealth,
	}

	cmd.Flags().BoolVarP(&healthJSON, "json", "", false, "Output as JSON")

	return cmd
}

func runHealth(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		if healthJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]bool{
				"healthy": false,
			})
			return nil
		}
		fmt.Println("Daemon is not running.")
		os.Exit(1)
	}

	resp, err := http.Get("http://127.0.0.1:4200/api/health")
	if err != nil {
		if healthJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"healthy": false,
				"error":   err.Error(),
			})
			return nil
		}
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if healthJSON {
		fmt.Println(string(body))
		return nil
	}

	var health struct {
		Status  string `json:"status"`
		Healthy bool   `json:"healthy"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if health.Healthy {
		fmt.Println("✓ Daemon is healthy")
	} else {
		fmt.Println("✗ Daemon is unhealthy:", health.Status)
		os.Exit(1)
	}

	return nil
}
