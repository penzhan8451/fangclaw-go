package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/penzhan8451/fangclaw-go/internal/api"
	"github.com/penzhan8451/fangclaw-go/internal/auth"
	"github.com/penzhan8451/fangclaw-go/internal/channels"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/logging"
	"github.com/rs/zerolog/log"
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
	logFile := logging.DefaultLogFilePath()

	logLevel := "info"
	cfg, err := config.Load("")
	if err == nil && cfg.Log.Level != "" {
		logLevel = cfg.Log.Level
	}
	if cfg != nil && cfg.Log.File != "" {
		logFile = cfg.Log.File
	}

	if err := logging.SetupFileOnly(logLevel, logFile); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	log.Info().Str("log_file", logFile).Str("level", logLevel).Msg("Daemon starting")

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

	// Auto-register all configured channels (from kernel secrets)
	getSecret := func(key string) string {
		return k.GetSecret(key)
	}
	if err := channels.AutoRegisterAll(k.Registry(), getSecret); err != nil {
		log.Warn().Err(err).Msg("Failed to auto-register some channels")
	}

	log.Info().Msg("Loading channels from config file")
	started, err := channels.LoadConfiguredChannels(k.Registry(), cfg, getSecret)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load channels from config")
	} else if len(started) > 0 {
		log.Info().Strs("channels", started).Msg("Started channels from config")
	}

	router := channels.NewAgentRouter()

	log.Info().Str("default_agent", cfg.DefaultAgent).Msg("Config loaded")

	agents := k.AgentRegistry().List()
	log.Info().Int("count", len(agents)).Msg("Agents in registry at startup")
	for i, agent := range agents {
		log.Info().Int("index", i+1).Str("name", agent.Name).Str("id", agent.ID.String()).Msg("Registered agent")
	}

	var defaultAgentID string
	if cfg.DefaultAgent != "" {
		for _, agent := range agents {
			if agent.Name == cfg.DefaultAgent {
				defaultAgentID = agent.ID.String()
				log.Info().Str("name", agent.Name).Str("id", defaultAgentID).Msg("Using default agent from config")
				break
			}
		}
		if defaultAgentID == "" {
			for _, agent := range agents {
				for _, tag := range agent.Tags {
					if tag == "hand:"+cfg.DefaultAgent {
						defaultAgentID = agent.ID.String()
						log.Info().Str("hand", cfg.DefaultAgent).Str("name", agent.Name).Str("id", defaultAgentID).Msg("Using default agent from config (hand)")
						break
					}
				}
				if defaultAgentID != "" {
					break
				}
			}
		}
		if defaultAgentID == "" {
			log.Warn().Str("agent", cfg.DefaultAgent).Msg("Default agent not found in agent list")
		}
	}

	if defaultAgentID == "" && len(agents) > 0 {
		defaultAgentID = agents[0].ID.String()
		log.Info().Str("name", agents[0].Name).Str("id", defaultAgentID).Msg("Set default agent to first available")
	} else if defaultAgentID == "" {
		log.Warn().Msg("No agents found. Users will need to activate hands first")
	}

	if defaultAgentID != "" {
		router.SetDefault(defaultAgentID)
	}

	bridgeManager := channels.NewBridgeManager(k, router)

	adapters := k.Registry().ListAdapters()
	for id, adapter := range adapters {
		if err := bridgeManager.RegisterAdapter(id, adapter); err != nil {
			log.Warn().Str("adapter", id).Err(err).Msg("Failed to register adapter")
		}
	}

	if err := bridgeManager.Start(); err != nil {
		return fmt.Errorf("failed to start bridge manager: %w", err)
	}
	defer bridgeManager.Stop()

	go func() {
		<-sigChan
		log.Info().Msg("Shutting down...")
		cleanupDaemonInfo(daemonPath)
		bridgeManager.Stop()
		os.Exit(0)
	}()

	log.Info().Str("address", cfg.APIListen).Msg("Configured listen address")
	apiCfg := &api.ServerConfig{
		ListenAddr: cfg.APIListen,
	}

	authManager := k.AuthManager()
	if authManager == nil {
		return fmt.Errorf("auth manager not initialized in kernel")
	}
	defer authManager.Close()

	userCount, err := authManager.UserCount()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if userCount == 0 {
		log.Info().Msg("First time setup - No users found")

		ownerPassword := os.Getenv("FANGCLAW_OWNER_PASSWORD")
		if ownerPassword != "" {
			log.Info().Msg("Auto-creating owner user from FANGCLAW_OWNER_PASSWORD")
			_, err := authManager.CreateUser("owner", "", ownerPassword, auth.RoleOwner)
			if err != nil {
				return fmt.Errorf("failed to create owner user: %w", err)
			}
			log.Info().Msg("Owner user created successfully")
		} else {
			log.Info().Msg("Creating default owner user (owner/owner123)")

			_, err := authManager.CreateUser("owner", "", "owner123", auth.RoleOwner)
			if err != nil {
				return fmt.Errorf("failed to create default owner user: %w", err)
			}
			log.Info().Msg("Default owner user created. Please change the password after first login")
		}
	}

	// Run multi-tenant server
	if err := api.RunMultiTenantServer(k, authManager, apiCfg, defaultAgentID, bridgeManager); err != nil {
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
