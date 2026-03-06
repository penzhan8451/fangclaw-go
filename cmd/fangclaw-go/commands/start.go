package commands

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	// "path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the OpenFang kernel daemon",
		Long:  "Start the kernel daemon which serves the API and manages agents.",
		RunE:  runStart,
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	// Check if daemon already running
	if isDaemonRunning() {
		fmt.Println("Daemon is already running. Use 'fangclaw-go status' to check.")
		return nil
	}

	// Get executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start the process
	proc := exec.Command(exePath, "daemon")
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	if err := proc.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Println("Daemon started (PID:", proc.Process.Pid, ")")
	fmt.Println("API:       http://127.0.0.1:4200")
	fmt.Println("Dashboard: http://127.0.0.1:4200/")

	// Wait for daemon to be ready
	fmt.Print("Waiting for daemon to be ready...")
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get("http://127.0.0.1:4200/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Println(" ✓")
				return nil
			}
		}
		fmt.Print(".")
	}
	fmt.Println(" (timeout)")

	return nil
}

func isDaemonRunning() bool {
	// Try to connect directly
	resp, err := http.Get("http://127.0.0.1:4200/api/health")
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
