package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func workflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflows (list, create, run)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all registered workflows",
		RunE:  runWorkflowList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create <file>",
		Short: "Create a workflow from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowCreate,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "run <workflow-id> <input>",
		Short: "Run a workflow by ID",
		Args:  cobra.ExactArgs(2),
		RunE:  runWorkflowRun,
	})

	return cmd
}

var workflowJSON bool

func runWorkflowList(cmd *cobra.Command, args []string) error {
	// Create in-memory workflow list for demo
	workflows := []map[string]interface{}{
		{
			"id":          "demo-workflow-1",
			"name":        "Demo Workflow",
			"description": "A sample workflow for testing",
			"steps":       2,
			"created_at":  "2026-02-28T00:00:00Z",
		},
	}

	if workflowJSON {
		json.NewEncoder(os.Stdout).Encode(workflows)
		return nil
	}

	fmt.Printf("%-40s %-20s %s\n", "ID", "NAME", "DESCRIPTION")
	fmt.Println("---------------------------------------------------------------------")
	for _, w := range workflows {
		fmt.Printf("%-40s %-20s %s\n", w["id"], w["name"], w["description"])
	}

	return nil
}

func runWorkflowCreate(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	// Validate JSON
	var workflow map[string]interface{}
	if err := json.Unmarshal(data, &workflow); err != nil {
		return fmt.Errorf("invalid workflow JSON: %w", err)
	}

	// In a full implementation, this would save to the daemon
	fmt.Printf("Workflow created from: %s\n", filePath)
	fmt.Println("Workflow JSON validated successfully.")

	return nil
}

func runWorkflowRun(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	input := args[1]

	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	// In a full implementation, this would send to the daemon API
	fmt.Printf("Running workflow: %s\n", workflowID)
	fmt.Printf("Input: %s\n", input)
	fmt.Println("Workflow execution not fully implemented yet.")

	return nil
}
