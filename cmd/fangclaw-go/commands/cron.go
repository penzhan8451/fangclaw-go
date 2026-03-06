package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func cronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage scheduled jobs (list, create, delete, enable, disable)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List scheduled jobs",
		RunE:  runCronList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create <agent> <spec> <prompt>",
		Short: "Create a new scheduled job",
		Args:  cobra.ExactArgs(3),
		RunE:  runCronCreate,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE:  runCronDelete,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a disabled job",
		Args:  cobra.ExactArgs(1),
		RunE:  runCronEnable,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a job without deleting it",
		Args:  cobra.ExactArgs(1),
		RunE:  runCronDisable,
	})

	return cmd
}

var cronJSON bool

func runCronList(cmd *cobra.Command, args []string) error {
	jobs := []map[string]interface{}{
		{
			"id":       "cron-001",
			"agent":    "assistant",
			"spec":     "0 */6 * * *",
			"prompt":   "Summarize recent activities",
			"enabled":  true,
			"next_run": "2026-03-01T00:00:00Z",
			"last_run": "2026-02-28T18:00:00Z",
		},
		{
			"id":       "cron-002",
			"agent":    "coder",
			"spec":     "0 9 * * 1-5",
			"prompt":   "Check for dependency updates",
			"enabled":  true,
			"next_run": "2026-03-01T09:00:00Z",
			"last_run": "",
		},
	}

	if cronJSON {
		json.NewEncoder(os.Stdout).Encode(jobs)
		return nil
	}

	fmt.Printf("%-15s %-15s %-20s %-10s %s\n", "ID", "AGENT", "SCHEDULE", "ENABLED", "NEXT RUN")
	fmt.Println("---------------------------------------------------------------------------------------------------")
	for _, j := range jobs {
		enabled := "true"
		if !j["enabled"].(bool) {
			enabled = "false"
		}
		fmt.Printf("%-15s %-15s %-20s %-10s %s\n", j["id"], j["agent"], j["spec"], enabled, j["next_run"])
	}

	return nil
}

func runCronCreate(cmd *cobra.Command, args []string) error {
	agent := args[0]
	spec := args[1]
	prompt := args[2]

	fmt.Printf("Creating scheduled job:\n")
	fmt.Printf("  Agent: %s\n", agent)
	fmt.Printf("  Schedule: %s\n", spec)
	fmt.Printf("  Prompt: %s\n", prompt)
	fmt.Println("(Requires daemon to be running)")

	return nil
}

func runCronDelete(cmd *cobra.Command, args []string) error {
	id := args[0]
	fmt.Printf("Deleting job: %s\n", id)
	return nil
}

func runCronEnable(cmd *cobra.Command, args []string) error {
	id := args[0]
	fmt.Printf("Enabling job: %s\n", id)
	return nil
}

func runCronDisable(cmd *cobra.Command, args []string) error {
	id := args[0]
	fmt.Printf("Disabling job: %s\n", id)
	return nil
}
