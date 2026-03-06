package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

func writeDaemonInfo(daemonPath string) error {
	info := DaemonInfo{
		PID:        os.Getpid(),
		ListenAddr: "127.0.0.1:4200",
		StartedAt:  time.Now().Format(time.RFC3339),
		Version:    "0.2.0",
		Platform:   "darwin",
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

func runDaemon(cmd *cobra.Command, args []string) error {
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

	if err := writeDaemonInfo(daemonPath); err != nil {
		return fmt.Errorf("failed to write daemon info: %w", err)
	}
	defer cleanupDaemonInfo(daemonPath)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	k, err := kernel.Boot("")
	if err != nil {
		return fmt.Errorf("failed to boot kernel: %w", err)
	}

	// Auto-register all configured channels
	if err := channels.AutoRegisterAll(k.Registry()); err != nil {
		fmt.Printf("Warning: Failed to auto-register some channels: %v\n", err)
	}

	// 创建并启动Channel Bridge Manager
	router := channels.NewAgentRouter()

	// 从配置文件中读取default agent
	cfg, err := config.Load("")
	if err != nil {
		fmt.Printf("Warning: Failed to load config, using defaults: %v\n", err)
	} else {
		fmt.Printf("Config loaded. Default agent from config: '%s'\n", cfg.DefaultAgent)
	}

	// 从agent registry中查找并设置default agent
	agents := k.AgentRegistry().List()
	fmt.Printf("Agents in registry at startup: %d agents found\n", len(agents))
	for i, agent := range agents {
		fmt.Printf("  Agent %d: Name='%s', ID='%s'\n", i+1, agent.Name, agent.ID.String())
	}

	var defaultAgentID string
	if cfg.DefaultAgent != "" {
		// 优先使用配置文件中指定的agent
		// 方式1: 通过agent name查找
		for _, agent := range agents {
			if agent.Name == cfg.DefaultAgent {
				defaultAgentID = agent.ID.String()
				fmt.Printf("Using default agent from config: %s (ID: %s)\n", agent.Name, defaultAgentID)
				break
			}
		}
		// 方式2: 如果通过name没找到，尝试通过hand ID查找（检查tags中是否有"hand:xxx"）
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
		// 如果配置文件中没有指定或者找不到，使用第一个agent作为default
		defaultAgentID = agents[0].ID.String()
		fmt.Printf("Set default agent to: %s (ID: %s)\n", agents[0].Name, defaultAgentID)
	} else if defaultAgentID == "" {
		fmt.Println("Warning: No agents found. Users will need to activate hands first.")
	}

	if defaultAgentID != "" {
		router.SetDefault(defaultAgentID)
	}

	bridgeManager := channels.NewBridgeManager(k, router)

	// 注册所有adapters到bridge manager
	adapters := k.Registry().ListAdapters()
	for id, adapter := range adapters {
		if err := bridgeManager.RegisterAdapter(id, adapter); err != nil {
			fmt.Printf("Warning: Failed to register adapter %s: %v\n", id, err)
		}
	}

	// 启动bridge manager
	if err := bridgeManager.Start(); err != nil {
		return fmt.Errorf("failed to start bridge manager: %w", err)
	}
	defer bridgeManager.Stop()

	// 处理信号
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cleanupDaemonInfo(daemonPath)
		bridgeManager.Stop()
		os.Exit(0)
	}()

	if err := api.RunServer(k, nil); err != nil {
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
