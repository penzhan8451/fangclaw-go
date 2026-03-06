package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func integrationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "List or search integrations",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runIntegrations,
	}
	return cmd
}

var integrationsQuery string

func runIntegrations(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		integrationsQuery = args[0]
	}

	integrations := []map[string]interface{}{
		{"name": "github", "description": "GitHub integration", "installed": false},
		{"name": "slack", "description": "Slack integration", "installed": false},
		{"name": "notion", "description": "Notion integration", "installed": false},
		{"name": "jira", "description": "Jira integration", "installed": false},
		{"name": "linear", "description": "Linear integration", "installed": false},
	}

	// Filter if query provided
	if integrationsQuery != "" {
		filtered := make([]map[string]interface{}, 0)
		for _, i := range integrations {
			if fmt.Sprintf("%v", i["name"]) == integrationsQuery {
				filtered = append(filtered, i)
			}
		}
		integrations = filtered
	}

	if len(integrations) == 0 {
		fmt.Println("No integrations found.")
		return nil
	}

	fmt.Printf("%-20s %-40s %s\n", "NAME", "DESCRIPTION", "INSTALLED")
	fmt.Println("---------------------------------------------------------------------------------------------------")
	for _, i := range integrations {
		installed := "no"
		if i["installed"].(bool) {
			installed = "yes"
		}
		fmt.Printf("%-20s %-40s %s\n", i["name"], i["description"], installed)
	}

	return nil
}

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an integration (one-click MCP server setup)",
		Args:  cobra.ExactArgs(1),
		RunE:  runAdd,
	}

	cmd.Flags().StringVar(&addKey, "key", "", "API key or token to store in the vault")

	return cmd
}

var addKey string

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	fmt.Printf("Adding integration: %s\n", name)
	if addKey != "" {
		fmt.Printf("  API key provided: %s\n", addKey)
	}
	fmt.Println("(Integration installation requires daemon)")
	return nil
}

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed integration",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	fmt.Printf("Removing integration: %s\n", name)
	return nil
}

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Scaffold a new skill or integration template",
		RunE:  runNew,
	}

	cmd.Flags().StringVar(&newKind, "kind", "skill", "What to scaffold (skill or integration)")

	return cmd
}

var newKind string

func runNew(cmd *cobra.Command, args []string) error {
	fmt.Printf("Scaffolding new: %s\n", newKind)
	fmt.Println("(Scaffolding not implemented in v1)")
	return nil
}

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate from another agent framework to OpenFang",
		RunE:  runMigrate,
	}

	cmd.Flags().StringVar(&migrateFrom, "from", "", "Source framework (openclaw, langchain, autogpt)")
	cmd.Flags().StringVar(&migrateSourceDir, "source-dir", "", "Path to source workspace")
	cmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show what would be imported")

	return cmd
}

var migrateFrom string
var migrateSourceDir string
var migrateDryRun bool

func runMigrate(cmd *cobra.Command, args []string) error {
	fmt.Println("Migration tool")
	fmt.Println("(Migration not implemented in v1)")
	if migrateFrom != "" {
		fmt.Printf("  Source: %s\n", migrateFrom)
	}
	if migrateDryRun {
		fmt.Println("  Mode: dry-run")
	}
	return nil
}

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Quick non-interactive initialization",
		RunE:  runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("Running quick setup...")
	return runInit(cmd, []string{"--quick"})
}

func configureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Interactive setup wizard for credentials and channels",
		RunE:  runConfigure,
	}
}

func runConfigure(cmd *cobra.Command, args []string) error {
	fmt.Println("Interactive configuration wizard...")
	fmt.Println("(Use 'fangclaw-go init' for quick setup)")
	return nil
}

func resetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset local config and state",
		RunE:  runReset,
	}

	cmd.Flags().BoolVar(&resetConfirm, "confirm", false, "Skip confirmation prompt")

	return cmd
}

var resetConfirm bool

func runReset(cmd *cobra.Command, args []string) error {
	if !resetConfirm {
		fmt.Print("This will reset all local config and state. Continue? (y/N): ")
		// In interactive mode, would read from stdin
		fmt.Println("Cancelled. Use --confirm to skip prompt.")
		return nil
	}

	fmt.Println("Resetting local config and state...")
	homeDir, _ := os.UserHomeDir()
	fmt.Printf("Would remove: %s/.fangclaw-go\n", homeDir)
	return nil
}

func onboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Interactive onboarding wizard",
		RunE:  runOnboard,
	}

	cmd.Flags().BoolVar(&onboardQuick, "quick", false, "Quick non-interactive mode")

	return cmd
}

var onboardQuick bool

func runOnboard(cmd *cobra.Command, args []string) error {
	if onboardQuick {
		fmt.Println("Running quick onboard...")
		return runSetup(cmd, args)
	}

	fmt.Println("Interactive onboarding wizard...")
	fmt.Println("(Use 'fangclaw-go init --quick' for non-interactive setup)")
	return nil
}
