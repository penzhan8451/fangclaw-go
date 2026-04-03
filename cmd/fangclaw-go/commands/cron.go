package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		Use:   "create <json-file>",
		Short: "Create a new scheduled job from JSON file",
		Args:  cobra.ExactArgs(1),
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

	cmd.AddCommand(&cobra.Command{
		Use:   "status <id>",
		Short: "Show job status",
		Args:  cobra.ExactArgs(1),
		RunE:  runCronStatus,
	})

	cmd.PersistentFlags().BoolVarP(&cronJSON, "json", "", false, "Output as JSON")

	return cmd
}

var cronJSON bool

func runCronList(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPGet(daemonAddr + "/api/cron/jobs")
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	if cronJSON {
		fmt.Println(string(body))
		return nil
	}

	var result struct {
		Jobs []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			AgentID string `json:"agent_id"`
			Enabled bool   `json:"enabled"`
			NextRun string `json:"next_run,omitempty"`
		} `json:"jobs"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("%-36s %-20s %-36s %-10s %s\n", "ID", "NAME", "AGENT ID", "ENABLED", "NEXT RUN")
	fmt.Println("---------------------------------------------------------------------------------------------------------------------------------------")
	for _, j := range result.Jobs {
		enabled := "true"
		if !j.Enabled {
			enabled = "false"
		}
		nextRun := j.NextRun
		if nextRun == "" {
			nextRun = "-"
		}
		fmt.Printf("%-36s %-20s %-36s %-10s %s\n", j.ID, j.Name, j.AgentID, enabled, nextRun)
	}
	fmt.Printf("\nTotal: %d jobs\n", result.Total)

	return nil
}

func runCronCreate(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var jobData map[string]interface{}
	if err := json.Unmarshal(data, &jobData); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPPost(daemonAddr+"/api/cron/jobs", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", string(body))
	}

	if cronJSON {
		fmt.Println(string(body))
		return nil
	}

	var result struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Job created successfully!\n")
	fmt.Printf("Job ID: %s\n", result.JobID)

	return nil
}

func runCronDelete(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	id := args[0]

	daemonAddr := mustGetDaemonAddress()
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/cron/jobs/%s", daemonAddr, id), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Client-Type", "cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	if cronJSON {
		fmt.Println(string(body))
		return nil
	}

	fmt.Printf("Job deleted successfully: %s\n", id)

	return nil
}

func runCronEnable(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	id := args[0]

	payload := map[string]bool{"enabled": true}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	daemonAddr := mustGetDaemonAddress()
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/cron/jobs/%s/enable", daemonAddr, id), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Client-Type", "cli")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	if cronJSON {
		fmt.Println(string(body))
		return nil
	}

	fmt.Printf("Job enabled successfully: %s\n", id)

	return nil
}

func runCronDisable(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	id := args[0]

	payload := map[string]bool{"enabled": false}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	daemonAddr := mustGetDaemonAddress()
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/cron/jobs/%s/enable", daemonAddr, id), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Client-Type", "cli")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	if cronJSON {
		fmt.Println(string(body))
		return nil
	}

	fmt.Printf("Job disabled successfully: %s\n", id)

	return nil
}

func runCronStatus(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	id := args[0]

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPGet(fmt.Sprintf("%s/api/cron/jobs/%s/status", daemonAddr, id))
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	if cronJSON {
		fmt.Println(string(body))
		return nil
	}

	var result struct {
		Job struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			AgentID string `json:"agent_id"`
			Enabled bool   `json:"enabled"`
		} `json:"job"`
		OneShot           bool    `json:"one_shot"`
		LastStatus        *string `json:"last_status"`
		ConsecutiveErrors uint32  `json:"consecutive_errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Job ID: %s\n", result.Job.ID)
	fmt.Printf("Name: %s\n", result.Job.Name)
	fmt.Printf("Agent ID: %s\n", result.Job.AgentID)
	fmt.Printf("Enabled: %t\n", result.Job.Enabled)
	fmt.Printf("One Shot: %t\n", result.OneShot)
	if result.LastStatus != nil {
		fmt.Printf("Last Status: %s\n", *result.LastStatus)
	}
	fmt.Printf("Consecutive Errors: %d\n", result.ConsecutiveErrors)

	return nil
}
