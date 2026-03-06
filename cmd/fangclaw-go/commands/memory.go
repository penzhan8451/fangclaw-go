package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func memoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Search and manage agent memory (KV store)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <agent>",
		Short: "List KV pairs for an agent",
		Args:  cobra.ExactArgs(1),
		RunE:  runMemoryList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get <agent> <key>",
		Short: "Get a specific KV value",
		Args:  cobra.ExactArgs(2),
		RunE:  runMemoryGet,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <agent> <key> <value>",
		Short: "Set a KV value",
		Args:  cobra.ExactArgs(3),
		RunE:  runMemorySet,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <agent> <key>",
		Short: "Delete a KV value",
		Args:  cobra.ExactArgs(2),
		RunE:  runMemoryDelete,
	})

	return cmd
}

var memoryJSON bool

func runMemoryList(cmd *cobra.Command, args []string) error {
	agent := args[0]

	// Demo data
	memories := []map[string]interface{}{
		{"key": "preferences", "value": "{\"theme\": \"dark\"}", "updated_at": "2026-02-28T00:00:00Z"},
		{"key": "last_conversation", "value": "Discussed project planning", "updated_at": "2026-02-28T00:00:00Z"},
	}

	if memoryJSON {
		json.NewEncoder(os.Stdout).Encode(memories)
		return nil
	}

	fmt.Printf("Memory for agent: %s\n", agent)
	fmt.Printf("%-30s %-40s %s\n", "KEY", "VALUE", "UPDATED_AT")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, m := range memories {
		fmt.Printf("%-30s %-40s %s\n", m["key"], m["value"], m["updated_at"])
	}

	return nil
}

func runMemoryGet(cmd *cobra.Command, args []string) error {
	agent := args[0]
	key := args[1]

	fmt.Printf("Getting key '%s' for agent '%s'\n", key, agent)
	fmt.Println("(Memory retrieval requires daemon)")

	return nil
}

func runMemorySet(cmd *cobra.Command, args []string) error {
	agent := args[0]
	key := args[1]
	value := args[2]

	fmt.Printf("Setting key '%s' = '%s' for agent '%s'\n", key, value, agent)
	fmt.Println("(Memory set requires daemon)")

	return nil
}

func runMemoryDelete(cmd *cobra.Command, args []string) error {
	agent := args[0]
	key := args[1]

	fmt.Printf("Deleting key '%s' for agent '%s'\n", key, agent)
	fmt.Println("(Memory delete requires daemon)")

	return nil
}
