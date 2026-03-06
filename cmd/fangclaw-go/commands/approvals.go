package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func approvalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approvals",
		Short: "Manage execution approvals (list, approve, reject)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List pending approvals",
		RunE:  runApprovalsList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "approve <id>",
		Short: "Approve a pending request",
		Args:  cobra.ExactArgs(1),
		RunE:  runApprovalsApprove,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reject <id>",
		Short: "Reject a pending request",
		Args:  cobra.ExactArgs(1),
		RunE:  runApprovalsReject,
	})

	return cmd
}

var approvalsJSON bool

func runApprovalsList(cmd *cobra.Command, args []string) error {
	approvals := []map[string]interface{}{
		{
			"id":           "approval-001",
			"agent_id":     "agent-123",
			"action":       "execute_command",
			"details":      "Run: rm -rf /tmp",
			"requested_at": "2026-02-28T12:00:00Z",
			"status":       "pending",
		},
	}

	if approvalsJSON {
		json.NewEncoder(os.Stdout).Encode(approvals)
		return nil
	}

	fmt.Printf("%-20s %-15s %-25s %s\n", "ID", "AGENT", "REQUESTED AT", "STATUS")
	fmt.Println("----------------------------------------------------------------------------------------")
	for _, a := range approvals {
		fmt.Printf("%-20s %-15s %-25s %s\n", a["id"], a["agent_id"], a["requested_at"], a["status"])
	}

	return nil
}

func runApprovalsApprove(cmd *cobra.Command, args []string) error {
	id := args[0]
	fmt.Printf("Approving: %s\n", id)
	return nil
}

func runApprovalsReject(cmd *cobra.Command, args []string) error {
	id := args[0]
	fmt.Printf("Rejecting: %s\n", id)
	return nil
}
