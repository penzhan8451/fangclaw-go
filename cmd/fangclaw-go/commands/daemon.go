package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/api"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/spf13/cobra"
)

type DaemonInfo struct {
	PID        int    `json:"pid"`
	ListenAddr string `json:"listen_addr"`
	StartedAt  string `json:"started_at"`
	Version    string `json:"version"`
	Platform   string `json:"platform"`
}

func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the FangClaw-Go daemon",
		Long:  "Internal command to run the daemon mode.",
		RunE:  runDaemon,
	}
}

func getDaemonPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	daemonDir := filepath.Join(homeDir, ".fangclaw-go")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(daemonDir, "daemon"), nil
}

func writeDaemonInfo(daemonPath string, cfg *config.Config) error {
	info := DaemonInfo{
		PID:        os.Getpid(),
		ListenAddr: cfg.APIListen,
		StartedAt:  time.Now().Format(time.RFC3339),
		Version:    "0.2.0",
		Platform:   runtime.GOOS,
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(daemonPath, data, 0600)
}

func cleanupDaemonInfo(daemonPath string) {
	_ = os.Remove(daemonPath)
}

func setupLogFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	logFile := filepath.Join(homeDir, ".fangclaw-go", "daemon.log")

	// Check log file size
	if fi, err := os.Stat(logFile); err == nil {
		if fi.Size() > 50*1024*1024 { // 50MB limit
			// Truncate log file
			if err := os.Truncate(logFile, 0); err != nil {
				return fmt.Errorf("failed to truncate log file: %w", err)
			}
		}
	}

	// Open log file for append
	// f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// if err != nil {
	// 	return fmt.Errorf("failed to open log file: %w", err)
	// }

	// // Redirect stdout and stderr to log file
	// os.Stdout = f
	// os.Stderr = f

	// Add timestamp to each log line
	// log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Log startup message
	log.Println("Daemon starting...")
	log.Printf("Log file: %s", logFile)

	return nil
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Setup log file
	if err := setupLogFile(); err != nil {
		return fmt.Errorf("failed to setup log file: %w", err)
	}

	daemonPath, err := getDaemonPath()
	if err != nil {
		return fmt.Errorf("failed to get daemon path: %w", err)
	}

	if _, err := os.Stat(daemonPath); err == nil {
		data, err := os.ReadFile(daemonPath)
		if err == nil {
			var info DaemonInfo
			if json.Unmarshal(data, &info) == nil {
				if isProcessRunning(info.PID) {
					return fmt.Errorf("another daemon (PID %d) is already running", info.PID)
				}
			}
		}
		_ = os.Remove(daemonPath)
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := writeDaemonInfo(daemonPath, cfg); err != nil {
		return fmt.Errorf("failed to write daemon info: %w", err)
	}
	defer cleanupDaemonInfo(daemonPath)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	k, err := kernel.Boot("")
	if err != nil {
		return fmt.Errorf("failed to boot kernel: %w", err)
	}

	// Start the cron scheduler
	if err := k.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start kernel: %w", err)
	}

	// Auto-register all configured channels (from env vars)
	if err := channels.AutoRegisterAll(k.Registry()); err != nil {
		fmt.Printf("Warning: Failed to auto-register some channels: %v\n", err)
	}

	// Also load channels from config.toml
	fmt.Println("Loading channels from config file...")
	started, err := channels.LoadConfiguredChannels(k.Registry(), cfg)
	if err != nil {
		fmt.Printf("Warning: Failed to load channels from config: %v\n", err)
	} else if len(started) > 0 {
		fmt.Printf("Started channels from config: %v\n", started)
	}

	// Create and start Channel Bridge Manager
	router := channels.NewAgentRouter()

	// Read default agent from config file
	fmt.Printf("Config loaded. Default agent from config: '%s'\n", cfg.DefaultAgent)

	// Find and set default agent from agent registry
	agents := k.AgentRegistry().List()
	fmt.Printf("Agents in registry at startup: %d agents found\n", len(agents))
	for i, agent := range agents {
		fmt.Printf("  Agent %d: Name='%s', ID='%s'\n", i+1, agent.Name, agent.ID.String())
	}

	var defaultAgentID string
	if cfg.DefaultAgent != "" {
		// Priority given to the agent specified in the config file
		// Method 1: Find by agent name
		for _, agent := range agents {
			if agent.Name == cfg.DefaultAgent {
				defaultAgentID = agent.ID.String()
				fmt.Printf("Using default agent from config: %s (ID: %s)\n", agent.Name, defaultAgentID)
				break
			}
		}
		// Method 2: If not found by name, try to find by hand ID (check if tags have "hand:xxx")
		if defaultAgentID == "" {
			for _, agent := range agents {
				for _, tag := range agent.Tags {
					if tag == "hand:"+cfg.DefaultAgent {
						defaultAgentID = agent.ID.String()
						fmt.Printf("Using default agent from config (hand: %s): %s (ID: %s)\n", cfg.DefaultAgent, agent.Name, defaultAgentID)
						break
					}
				}
				if defaultAgentID != "" {
					break
				}
			}
		}
		if defaultAgentID == "" {
			fmt.Printf("Warning: Default agent '%s' not found in agent list\n", cfg.DefaultAgent)
		}
	}

	if defaultAgentID == "" && len(agents) > 0 {
		// If not specified in config or not found, use the first agent as default
		defaultAgentID = agents[0].ID.String()
		fmt.Printf("Set default agent to: %s (ID: %s)\n", agents[0].Name, defaultAgentID)
	} else if defaultAgentID == "" {
		fmt.Println("Warning: No agents found. Users will need to activate hands first.")
	}

	if defaultAgentID != "" {
		router.SetDefault(defaultAgentID)
	}

	bridgeManager := channels.NewBridgeManager(k, router)

	// Register all adapters to bridge manager
	adapters := k.Registry().ListAdapters()
	for id, adapter := range adapters {
		if err := bridgeManager.RegisterAdapter(id, adapter); err != nil {
			fmt.Printf("Warning: Failed to register adapter %s: %v\n", id, err)
		}
	}

	// Start bridge manager
	if err := bridgeManager.Start(); err != nil {
		return fmt.Errorf("failed to start bridge manager: %w", err)
	}
	defer bridgeManager.Stop()

	// Handle signals
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cleanupDaemonInfo(daemonPath)
		bridgeManager.Stop()
		os.Exit(0)
	}()

	// Create API server config from config file
	fmt.Printf("Configured Listen Address:%s\n", cfg.APIListen)
	apiCfg := &api.ServerConfig{
		ListenAddr: cfg.APIListen,
	}

	if err := api.RunServer(k, apiCfg, defaultAgentID); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}
