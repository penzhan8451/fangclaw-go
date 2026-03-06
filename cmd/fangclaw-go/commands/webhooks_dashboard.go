package commands

import (
	"fmt"

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
	fmt.Println("Opening web dashboard...")
	fmt.Println("URL: http://127.0.0.1:4200/")
	fmt.Println("(Requires daemon to be running)")
	return nil
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive terminal dashboard",
		RunE:  runTui,
	}
}

func runTui(cmd *cobra.Command, args []string) error {
	fmt.Println("Launching terminal dashboard (TUI)...")
	fmt.Println("(TUI not implemented in Go version - use web dashboard)")
	return nil
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP (Model Context Protocol) server over stdio",
		RunE:  runMcp,
	}
}

func runMcp(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting MCP server...")
	fmt.Println("(MCP server not implemented in v1)")
	return nil
}
