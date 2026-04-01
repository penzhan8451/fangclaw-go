// Package commands provides CLI commands for OpenFang.
package commands

import (
	"github.com/spf13/cobra"
)

// Register registers all commands with the root command.
func Register(root *cobra.Command) {
	// Root-level flags
	root.PersistentFlags().StringP("config", "c", "", "Path to config file")

	// Core commands
	root.AddCommand(initCmd())
	root.AddCommand(startCmd())
	root.AddCommand(daemonCmd())
	root.AddCommand(stopCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(healthCmd())
	root.AddCommand(configCmd())
	root.AddCommand(doctorCmd())
	root.AddCommand(completionCmd())

	// Agent commands
	root.AddCommand(agentCmd())
	root.AddCommand(chatCmd())
	root.AddCommand(messageCmd())

	// Workflow & triggers
	root.AddCommand(workflowCmd())
	root.AddCommand(triggerCmd())

	// Models & skills
	root.AddCommand(modelCmd())
	root.AddCommand(skillCmd())

	// System
	root.AddCommand(systemCmd())
	root.AddCommand(logsCmd())

	// Security & governance
	root.AddCommand(securityCmd())
	root.AddCommand(approvalsCmd())
	root.AddCommand(cronCmd())

	// Storage
	root.AddCommand(memoryCmd())
	root.AddCommand(vaultCmd())

	// Channels & devices
	root.AddCommand(channelCmd())
	root.AddCommand(devicesCmd())
	root.AddCommand(qrCmd())
	root.AddCommand(webhooksCmd())

	// Integrations
	root.AddCommand(integrationsCmd())
	root.AddCommand(addCmd())
	root.AddCommand(removeCmd())
	root.AddCommand(newCmd())

	// Setup & config
	root.AddCommand(setupCmd())
	root.AddCommand(configureCmd())
	root.AddCommand(onboardCmd())
	root.AddCommand(resetCmd())
	root.AddCommand(migrateCmd())

	// UI
	root.AddCommand(dashboardCmd())
	root.AddCommand(tuiCmd())

	// Protocol
	root.AddCommand(mcpCmd())

	// Hands
	root.AddCommand(handCmd())

	// Wizard
	root.AddCommand(wizardCmd())

	// User management (multi-tenant mode)
	root.AddCommand(NewUserCmd())
}
