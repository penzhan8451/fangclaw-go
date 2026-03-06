package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func skillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage skills",
		Long:  "Manage skills (list, install, remove).",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		RunE:  runSkillList,
	})

	return cmd
}

func runSkillList(cmd *cobra.Command, args []string) error {
	fmt.Println("Skills functionality coming soon.")
	return nil
}
