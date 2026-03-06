package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func triggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Manage event triggers (list, create, delete)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all triggers (optionally filtered by agent)",
		RunE:  runTriggerList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create <agent-id> <pattern-json>",
		Short: "Create a trigger for an agent",
		Args:  cobra.ExactArgs(2),
		RunE:  runTriggerCreate,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <trigger-id>",
		Short: "Delete a trigger by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runTriggerDelete,
	})

	return cmd
}

var triggerAgentID string

func runTriggerList(cmd *cobra.Command, args []string) error {
	// Create in-memory trigger list for demo
	triggers := []map[string]interface{}{
		{
			"id":              "trigger-1",
			"agent_id":        "agent-123",
			"pattern":         "lifecycle",
			"prompt_template": "Event: {{event}}",
			"enabled":         true,
			"fire_count":      5,
			"max_fires":       0,
			"created_at":      "2026-02-28T00:00:00Z",
		},
	}

	// Filter by agent if specified
	if triggerAgentID != "" {
		filtered := make([]map[string]interface{}, 0)
		for _, t := range triggers {
			if t["agent_id"] == triggerAgentID {
				filtered = append(filtered, t)
			}
		}
		triggers = filtered
	}

	if len(triggers) == 0 {
		fmt.Println("No triggers found.")
		return nil
	}

	fmt.Printf("%-40s %-20s %-15s %s\n", "ID", "AGENT ID", "PATTERN", "ENABLED")
	fmt.Println("---------------------------------------------------------------------")
	for _, t := range triggers {
		enabled := "true"
		if !t["enabled"].(bool) {
			enabled = "false"
		}
		fmt.Printf("%-40s %-20s %-15s %s\n", t["id"], t["agent_id"], t["pattern"], enabled)
	}

	return nil
}

var triggerPrompt string
var triggerMaxFires uint64

func runTriggerCreate(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	patternJSON := args[1]

	// Validate JSON pattern
	var pattern map[string]interface{}
	if err := json.Unmarshal([]byte(patternJSON), &pattern); err != nil {
		return fmt.Errorf("invalid pattern JSON: %w", err)
	}

	// Default prompt template
	if triggerPrompt == "" {
		triggerPrompt = "Event: {{event}}"
	}

	if triggerMaxFires == 0 {
		triggerMaxFires = 0 // unlimited
	}

	// In a full implementation, this would send to the daemon API
	fmt.Printf("Trigger created for agent: %s\n", agentID)
	fmt.Printf("Pattern: %s\n", patternJSON)
	fmt.Printf("Prompt: %s\n", triggerPrompt)
	fmt.Printf("Max fires: %d\n", triggerMaxFires)

	return nil
}

func runTriggerDelete(cmd *cobra.Command, args []string) error {
	triggerID := args[0]

	// In a full implementation, this would send to the daemon API
	fmt.Printf("Trigger deleted: %s\n", triggerID)

	return nil
}
