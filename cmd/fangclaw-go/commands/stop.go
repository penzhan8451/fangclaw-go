package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running daemon",
		Long:  "Stop the OpenFang daemon gracefully.",
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	daemonPath := filepath.Join(homeDir, ".fangclaw-go", "daemon")
	data, err := os.ReadFile(daemonPath)
	if err != nil {
		fmt.Println("No running daemon found.")
		return nil
	}

	var info struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("failed to parse daemon info: %w", err)
	}

	// Try graceful shutdown first
	daemonAddr := mustGetDaemonAddress()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(daemonAddr+"/api/shutdown", "", nil)
	if err == nil && resp.StatusCode == http.StatusOK {
		// Wait for daemon to stop
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			if !isDaemonRunning() {
				os.Remove(daemonPath)
				fmt.Println("Daemon stopped.")
				return nil
			}
		}
	}

	// Force kill
	fmt.Println("Graceful shutdown timed out, force killing...")
	killProcess(info.PID)
	os.Remove(daemonPath)
	fmt.Println("Daemon stopped (forced).")

	return nil
}

func killProcess(pid int) error {
	// Try Unix kill first (works on macOS/Linux)
	_, err := exec.Command("kill", fmt.Sprintf("%d", pid)).Output()
	if err == nil {
		return nil
	}

	// Try Windows
	_, err = exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/F").Output()
	return err
}
