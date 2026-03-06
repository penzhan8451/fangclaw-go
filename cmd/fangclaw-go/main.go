// Package main provides the OpenFang CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/penzhan8451/fangclaw-go/cmd/fangclaw-go/commands"
	"github.com/spf13/cobra"
)

const (
	name        = "fangclaw-go"
	description = "Open-source Agent Operating System"
	version     = "0.2.0"
)

func loadEnv() {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		envPath := filepath.Join(homeDir, ".fangclaw-go", ".fangclaw-go.env")
		_ = godotenv.Load(envPath)
		secretsPath := filepath.Join(homeDir, ".fangclaw-go", "secrets.env")
		_ = godotenv.Load(secretsPath)
	}
}

func main() {
	loadEnv()
	rootCmd := &cobra.Command{
		Use:   name,
		Short: description,
		Long: fmt.Sprintf(`%s — %s

Deploy, manage, and orchestrate AI agents from your terminal.
40 channels · 60 skills · 50+ models · infinite possibilities.

Quick Start:
  1. fangclaw-go init              Set up config + API key
  2. fangclaw-go start             Launch the daemon
  3. fangclaw-go chat              Start chatting!

More:
  Docs:       https://github.com/penzhan8451/fangclaw-go
  Dashboard:  http://127.0.0.1:4200/ (when daemon is running)`, name, description),
		Version: version,
	}

	// Register commands
	commands.Register(rootCmd)

	// If no command is specified, show help
	if len(os.Args) == 1 {
		rootCmd.Help()
		os.Exit(0)
	}

	// Execute Commands
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
