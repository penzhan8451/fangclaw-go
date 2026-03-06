package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func handCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hand",
		Short: "Manage autonomous capability packages (Hands)",
		Long: `Hands are autonomous capability packages that work for you — on schedules, without prompting.

The 7 bundled Hands:
  - researcher: Deep autonomous research
  - lead: Daily lead generation
  - collector: OSINT intelligence collection
  - predictor: Superforecasting engine
  - clip: YouTube video processing
  - twitter: Autonomous Twitter/X management
  - browser: Web automation

Examples:
  fangclaw-go hand list
  fangclaw-go hand activate researcher
  fangclaw-go hand status researcher
  fangclaw-go hand pause researcher
  fangclaw-go hand deactivate researcher`,
	}

	cmd.AddCommand(handListCmd())
	cmd.AddCommand(handActivateCmd())
	cmd.AddCommand(handStatusCmd())
	cmd.AddCommand(handPauseCmd())
	cmd.AddCommand(handDeactivateCmd())

	return cmd
}

func handListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available Hands",
		RunE:  runHandList,
	}
	return cmd
}

func handActivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activate <hand>",
		Short: "Activate a Hand",
		Args:  cobra.ExactArgs(1),
		RunE:  runHandActivate,
	}
	return cmd
}

func handStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [hand]",
		Short: "Show status of Hands",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runHandStatus,
	}
	return cmd
}

func handPauseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause <hand>",
		Short: "Pause a Hand",
		Args:  cobra.ExactArgs(1),
		RunE:  runHandPause,
	}
	return cmd
}

func handDeactivateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deactivate <hand>",
		Short: "Deactivate a Hand",
		Args:  cobra.ExactArgs(1),
		RunE:  runHandDeactivate,
	}
	return cmd
}

var bundledHands = []map[string]string{
	{
		"id":          "researcher",
		"name":        "Researcher",
		"description": "Deep autonomous researcher. Cross-references multiple sources, evaluates credibility, generates cited reports.",
		"category":    "content",
		"status":      "inactive",
	},
	{
		"id":          "lead",
		"name":        "Lead",
		"description": "Runs daily. Discovers prospects matching your ICP, enriches with research, scores 0-100.",
		"category":    "productivity",
		"status":      "inactive",
	},
	{
		"id":          "collector",
		"name":        "Collector",
		"description": "OSINT-grade intelligence. Monitors targets continuously with change detection and knowledge graphs.",
		"category":    "data",
		"status":      "inactive",
	},
	{
		"id":          "predictor",
		"name":        "Predictor",
		"description": "Superforecasting engine. Collects signals, builds calibrated reasoning chains, makes predictions.",
		"category":    "data",
		"status":      "inactive",
	},
	{
		"id":          "clip",
		"name":        "Clip",
		"description": "YouTube video processing. Downloads, identifies best moments, cuts into vertical shorts.",
		"category":    "content",
		"status":      "inactive",
	},
	{
		"id":          "twitter",
		"name":        "Twitter",
		"description": "Autonomous Twitter/X account manager. Creates content, schedules posts, responds to mentions.",
		"category":    "communication",
		"status":      "inactive",
	},
	{
		"id":          "browser",
		"name":        "Browser",
		"description": "Web automation agent. Navigates sites, fills forms, handles multi-step workflows.",
		"category":    "productivity",
		"status":      "inactive",
	},
}

func init() {
	loadHandsStatus()
}

func getHandsFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".fangclaw-go", "hands.json")
}

func loadHandsStatus() {
	path := getHandsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var savedHands []map[string]string
	if err := json.Unmarshal(data, &savedHands); err != nil {
		return
	}
	for _, saved := range savedHands {
		for j := range bundledHands {
			if bundledHands[j]["id"] == saved["id"] {
				bundledHands[j]["status"] = saved["status"]
				break
			}
		}
	}
}

func saveHandsStatus() {
	path := getHandsFilePath()
	data, err := json.MarshalIndent(bundledHands, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func getActiveHandInstances() ([]map[string]interface{}, error) {
	if !isDaemonRunning() {
		return nil, fmt.Errorf("daemon not running")
	}

	resp, err := http.Get("http://127.0.0.1:4200/api/hands/active")
	if err != nil {
		return nil, fmt.Errorf("failed to get active hands: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Instances []map[string]interface{} `json:"instances"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Instances, nil
}

func findHandInstanceByHandID(handID string) (map[string]interface{}, error) {
	instances, err := getActiveHandInstances()
	if err != nil {
		return nil, err
	}

	var candidate map[string]interface{}
	for _, inst := range instances {
		if inst["hand_id"] == handID {
			if agentID, ok := inst["agent_id"].(string); ok && agentID != "" {
				return inst, nil
			}
			if candidate == nil {
				candidate = inst
			}
		}
	}

	if candidate != nil {
		return candidate, nil
	}

	return nil, fmt.Errorf("no active instance found for hand: %s", handID)
}

func runHandList(cmd *cobra.Command, args []string) error {
	fmt.Println("Available Hands:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tCATEGORY\tSTATUS\tDESCRIPTION")
	fmt.Fprintln(w, "--\t----\t--------\t------\t-----------")

	for _, hand := range bundledHands {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			hand["id"],
			hand["name"],
			hand["category"],
			hand["status"],
			hand["description"])
	}

	w.Flush()
	fmt.Println()
	fmt.Println("Use 'fangclaw-go hand activate <id>' to activate a Hand.")

	return nil
}

func runHandActivate(cmd *cobra.Command, args []string) error {
	handID := args[0]

	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	resp, err := http.Post(
		fmt.Sprintf("http://127.0.0.1:4200/api/hands/%s/activate", handID),
		"application/json",
		bytes.NewBufferString(`{"config":{}}`),
	)
	if err != nil {
		return fmt.Errorf("failed to activate hand: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(body, &errResp)
		if errResp.Error != "" {
			return fmt.Errorf("failed to activate hand: %s", errResp.Error)
		}
		return fmt.Errorf("failed to activate hand: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Success    bool   `json:"success"`
		InstanceID string `json:"instance_id"`
		AgentID    string `json:"agent_id"`
		AgentName  string `json:"agent_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	for i, hand := range bundledHands {
		if hand["id"] == handID {
			bundledHands[i]["status"] = "active"
			saveHandsStatus()
			break
		}
	}

	fmt.Printf("✓ Hand '%s' activated successfully!\n", handID)
	fmt.Println()
	fmt.Printf("  Instance ID: %s\n", result.InstanceID)
	fmt.Printf("  Agent Name:  %s\n", result.AgentName)
	fmt.Println()
	fmt.Println("Hand will start working autonomously. Check status with:")
	fmt.Printf("  fangclaw-go hand status %s\n", handID)

	return nil
}

func runHandStatus(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		handID := args[0]

		if isDaemonRunning() {
			instances, err := getActiveHandInstances()
			if err == nil {
				for _, inst := range instances {
					if inst["hand_id"] == handID {
						fmt.Printf("Hand: %s\n", inst["agent_name"])
						fmt.Printf("Hand ID: %s\n", inst["hand_id"])
						fmt.Printf("Instance ID: %s\n", inst["instance_id"])
						fmt.Printf("Status: %s\n", inst["status"])
						fmt.Printf("Agent ID: %s\n", inst["agent_id"])
						return nil
					}
				}
			}
		}

		for _, hand := range bundledHands {
			if hand["id"] == handID {
				fmt.Printf("Hand: %s\n", hand["name"])
				fmt.Printf("ID: %s\n", hand["id"])
				fmt.Printf("Status: %s\n", hand["status"])
				fmt.Printf("Category: %s\n", hand["category"])
				fmt.Println()
				fmt.Println("Description:")
				fmt.Printf("  %s\n", hand["description"])
				return nil
			}
		}
		return fmt.Errorf("unknown hand: %s", handID)
	}

	fmt.Println("Hands Status:")
	fmt.Println()

	if isDaemonRunning() {
		instances, err := getActiveHandInstances()
		if err == nil && len(instances) > 0 {
			fmt.Println("Active Instances:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "HAND\tINSTANCE ID\tSTATUS\tAGENT NAME")
			fmt.Fprintln(w, "----\t-----------\t------\t----------")
			for _, inst := range instances {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					inst["hand_id"],
					inst["instance_id"],
					inst["status"],
					inst["agent_name"])
			}
			w.Flush()
			fmt.Println()
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS")
	fmt.Fprintln(w, "--\t----\t------")

	for _, hand := range bundledHands {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			hand["id"],
			hand["name"],
			hand["status"])
	}

	w.Flush()
	return nil
}

func runHandPause(cmd *cobra.Command, args []string) error {
	handID := args[0]

	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	instance, err := findHandInstanceByHandID(handID)
	if err != nil {
		return err
	}

	instanceID := instance["instance_id"].(string)

	resp, err := http.Post(
		fmt.Sprintf("http://127.0.0.1:4200/api/hands/instances/%s/pause", instanceID),
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to pause hand: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pause hand: HTTP %d", resp.StatusCode)
	}

	for i, hand := range bundledHands {
		if hand["id"] == handID {
			bundledHands[i]["status"] = "paused"
			saveHandsStatus()
			break
		}
	}

	fmt.Printf("✓ Hand '%s' paused successfully!\n", handID)
	return nil
}

func runHandDeactivate(cmd *cobra.Command, args []string) error {
	handID := args[0]

	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	instance, err := findHandInstanceByHandID(handID)
	if err != nil {
		return err
	}

	instanceID := instance["instance_id"].(string)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:4200/api/hands/instances/%s/deactivate", instanceID), nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deactivate hand: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to deactivate hand: HTTP %d", resp.StatusCode)
	}

	for i, hand := range bundledHands {
		if hand["id"] == handID {
			bundledHands[i]["status"] = "inactive"
			saveHandsStatus()
			break
		}
	}

	fmt.Printf("✓ Hand '%s' deactivated successfully!\n", handID)
	return nil
}
