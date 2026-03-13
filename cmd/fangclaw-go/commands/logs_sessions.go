package commands

import (
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

	cmd.Flags().Int64VarP(&logsLines, "lines", "n", 50, "Number of lines to show")
	cmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output in real time")

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
