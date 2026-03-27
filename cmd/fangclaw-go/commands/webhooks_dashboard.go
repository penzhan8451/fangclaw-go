package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/penzhan8451/fangclaw-go/cmd/fangclaw-go/tui"
	"github.com/penzhan8451/fangclaw-go/internal/kernel"
	"github.com/penzhan8451/fangclaw-go/internal/mcp"
	"github.com/penzhan8451/fangclaw-go/internal/types"
	"github.com/spf13/cobra"
)

func webhooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Webhook helpers and trigger management",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered webhooks",
		RunE:  runWebhooksList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create <url> <events>",
		Short: "Create a new webhook",
		Args:  cobra.ExactArgs(2),
		RunE:  runWebhooksCreate,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhooksDelete,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "test <webhook-id>",
		Short: "Test a webhook by sending a ping",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhooksTest,
	})

	return cmd
}

func runWebhooksList(cmd *cobra.Command, args []string) error {
	fmt.Println("Registered webhooks:")
	fmt.Println("(Webhook management requires daemon)")
	return nil
}

func runWebhooksCreate(cmd *cobra.Command, args []string) error {
	url := args[0]
	events := args[1]
	fmt.Printf("Creating webhook:\n")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Events: %s\n", events)
	return nil
}

func runWebhooksDelete(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	fmt.Printf("Deleting webhook: %s\n", webhookID)
	return nil
}

func runWebhooksTest(cmd *cobra.Command, args []string) error {
	webhookID := args[0]
	fmt.Printf("Testing webhook: %s\n", webhookID)
	return nil
}

func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Open the web dashboard in the default browser",
		RunE:  runDashboard,
	}
}

func runDashboard(cmd *cobra.Command, args []string) error {
	daemonAddr := mustGetDaemonAddress()
	fmt.Println("Opening web dashboard...")
	fmt.Println("URL: " + daemonAddr + "/")
	fmt.Println("(Requires daemon to be running)")
	return nil
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive TUI mode",
		Long: `Launch the interactive terminal user interface for fangclaw-go.

The TUI provides a visual interface with the following features:
  - Dashboard: View system statistics and status
  - Chat: Interactive chat with agents
  - Agents: Manage and monitor agents

Keyboard Shortcuts:
  1, 2, 3     Switch to specific tab
  ←, →        Navigate between tabs
  r           Refresh current screen
  ?           Show help
  q           Quit
  Ctrl+C      Force quit`,
		RunE: runTUI,
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	if isDaemonRunning() {
		return runTUIWithDaemon()
	}
	return runTUIInProcess()
}

func runTUIWithDaemon() error {
	daemonAddr := mustGetDaemonAddress()
	backend := tui.NewDaemonBackend(daemonAddr)
	return tui.Run(backend)
}

func runTUIInProcess() error {
	cfg := types.DefaultConfig()

	k, err := kernel.NewKernel(cfg)
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}

	if err := k.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start kernel: %w", err)
	}
	defer k.Stop(context.Background())

	backend := tui.NewInProcessBackend(k)
	return tui.Run(backend)
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the fangclaw-go MCP server",
		Long:  "Start the MCP (Model Context Protocol) server that exposes fangclaw-go agents as tools.",
		RunE:  runMcp,
	}
}

func runMcp(cmd *cobra.Command, args []string) error {
	if isDaemonRunning() {
		return runMcpWithDaemon()
	}
	return runMcpInProcess()
}

func runMcpWithDaemon() error {
	daemonAddr := mustGetDaemonAddress()
	backend := NewDaemonMcpBackend(daemonAddr)
	server := mcp.NewMcpServer(backend)
	mcp.RunStdioServer(server)
	return nil
}

func runMcpInProcess() error {
	cfg := types.DefaultConfig()

	k, err := kernel.NewKernel(cfg)
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}

	if err := k.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start kernel: %w", err)
	}
	defer k.Stop(context.Background())

	backend := mcp.NewKernelMcpBackend(k)
	server := mcp.NewMcpServer(backend)
	mcp.RunStdioServer(server)

	return nil
}

type DaemonMcpBackend struct {
	baseURL string
	client  *http.Client
}

func NewDaemonMcpBackend(baseURL string) *DaemonMcpBackend {
	return &DaemonMcpBackend{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (b *DaemonMcpBackend) ListAgents() ([]*mcp.AgentInfo, error) {
	resp, err := b.client.Get(fmt.Sprintf("%s/api/agents", b.baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list agents: status %d", resp.StatusCode)
	}

	var agents []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("failed to parse agents: %w", err)
	}

	result := make([]*mcp.AgentInfo, 0, len(agents))
	for _, a := range agents {
		result = append(result, &mcp.AgentInfo{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
		})
	}
	return result, nil
}

func (b *DaemonMcpBackend) SendMessage(agentID, message string) (string, error) {
	reqBody := map[string]string{
		"message": message,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := b.client.Post(
		fmt.Sprintf("%s/api/agents/%s/message", b.baseURL, agentID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"response"`
		Error    string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}

	return result.Response, nil
}
