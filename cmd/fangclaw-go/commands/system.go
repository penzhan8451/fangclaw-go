package commands

import (
	"fmt"
	"os"

	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/spf13/cobra"
)

func systemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System info and version",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE:  runSystemVersion,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show detailed system info",
		RunE:  runSystemInfo,
	})

	return cmd
}

func runSystemVersion(cmd *cobra.Command, args []string) error {
	fmt.Println("fangclaw-go version 0.2.0")
	fmt.Println("go1.21")
	return nil
}

func runSystemInfo(cmd *cobra.Command, args []string) error {
	fmt.Println("System: darwin")
	fmt.Println("Go version: go1.21")
	fmt.Println("Architecture: arm64")
	return nil
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic health checks",
		RunE:  runDoctor,
	}
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Running diagnostics...")

	// Check config
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("✗ Config error: %v\n", err)
	} else {
		fmt.Printf("✓ Config loaded: %s\n", cfg.DefaultModel.Provider)
	}

	// Check daemon
	if isDaemonRunning() {
		fmt.Println("✓ Daemon running")
	} else {
		fmt.Println("✗ Daemon not running")
	}

	// Check API key
	envKey := "GROQ_API_KEY"
	if os.Getenv(envKey) != "" {
		fmt.Printf("✓ %s set\n", envKey)
	} else {
		fmt.Printf("✗ %s not set\n", envKey)
	}

	return nil
}
