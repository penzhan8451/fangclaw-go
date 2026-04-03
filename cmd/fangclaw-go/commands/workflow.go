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

func workflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflows (list, create, delete, run)",
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
		Use:   "delete <id>",
		Short: "Delete a workflow by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowDelete,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get <id>",
		Short: "Get a workflow by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowGet,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "run <workflow-id> <input>",
		Short: "Run a workflow by ID",
		Args:  cobra.ExactArgs(2),
		RunE:  runWorkflowRun,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create-from-template <template-id> [custom-name] [custom-description]",
		Short: "Create a workflow from a template",
		Args:  cobra.RangeArgs(1, 3),
		RunE:  runWorkflowCreateFromTemplate,
	})

	templatesCmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage workflow templates",
	}
	templatesCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all workflow templates",
		RunE:  runWorkflowTemplatesList,
	})
	templatesCmd.AddCommand(&cobra.Command{
		Use:   "get <id>",
		Short: "Get a workflow template by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowTemplatesGet,
	})
	cmd.AddCommand(templatesCmd)

	cmd.PersistentFlags().BoolVarP(&workflowJSON, "json", "", false, "Output as JSON")

	return cmd
}

var workflowJSON bool

func runWorkflowList(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPGet(daemonAddr + "/api/workflows")
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

	if workflowJSON {
		fmt.Println(string(body))
		return nil
	}

	var workflows []map[string]interface{}
	if err := json.Unmarshal(body, &workflows); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		return nil
	}

	fmt.Printf("%-40s %-20s %-5s %s\n", "ID", "NAME", "STEPS", "CREATED AT")
	fmt.Println("-----------------------------------------------------------------------------------------------")
	for _, w := range workflows {
		fmt.Printf("%-40s %-20s %-5v %s\n", w["id"], w["name"], w["steps"], w["created_at"])
	}
	fmt.Printf("\nTotal: %d workflows\n", len(workflows))

	return nil
}

func runWorkflowCreate(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	var workflowData map[string]interface{}
	if err := json.Unmarshal(data, &workflowData); err != nil {
		return fmt.Errorf("invalid workflow JSON: %w", err)
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPPost(daemonAddr+"/api/workflows", "application/json", bytes.NewReader(data))
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

	if workflowJSON {
		fmt.Println(string(body))
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Workflow created successfully!\n")
	fmt.Printf("Workflow ID: %s\n", result["workflow_id"])

	return nil
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	workflowID := args[0]

	daemonAddr := mustGetDaemonAddress()
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/workflows/%s", daemonAddr, workflowID), nil)
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

	if workflowJSON {
		fmt.Println(string(body))
		return nil
	}

	fmt.Printf("Workflow deleted successfully: %s\n", workflowID)

	return nil
}

func runWorkflowGet(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	workflowID := args[0]

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPGet(fmt.Sprintf("%s/api/workflows/%s", daemonAddr, workflowID))
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

	fmt.Println(string(body))

	return nil
}

func runWorkflowRun(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	workflowID := args[0]
	input := args[1]

	reqBody := map[string]string{
		"input": input,
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPPost(fmt.Sprintf("%s/api/workflows/%s/run", daemonAddr, workflowID), "application/json", bytes.NewReader(reqData))
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

	if workflowJSON {
		fmt.Println(string(body))
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Workflow executed successfully!\n")
	fmt.Printf("Run ID: %s\n", result["run_id"])
	fmt.Printf("Status: %s\n", result["status"])
	fmt.Printf("Output:\n%s\n", result["output"])

	return nil
}

func runWorkflowCreateFromTemplate(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	templateID := args[0]

	reqBody := map[string]interface{}{
		"template_id": templateID,
	}

	if len(args) > 1 {
		reqBody["custom_name"] = args[1]
	}

	if len(args) > 2 {
		reqBody["custom_description"] = args[2]
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPPost(daemonAddr+"/api/workflows/from-template", "application/json", bytes.NewReader(reqData))
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

	if workflowJSON {
		fmt.Println(string(body))
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Workflow created from template successfully!\n")
	fmt.Printf("Workflow ID: %s\n", result["id"])
	fmt.Printf("Name: %s\n", result["name"])

	return nil
}

func runWorkflowTemplatesList(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPGet(daemonAddr + "/api/workflow-templates")
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

	if workflowJSON {
		fmt.Println(string(body))
		return nil
	}

	var templates []map[string]interface{}
	if err := json.Unmarshal(body, &templates); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(templates) == 0 {
		fmt.Println("No workflow templates found.")
		return nil
	}

	fmt.Printf("%-25s %-30s %-15s %s\n", "ID", "NAME", "CATEGORY", "DESCRIPTION")
	fmt.Println("-----------------------------------------------------------------------------------------------")
	for _, t := range templates {
		fmt.Printf("%-25s %-30s %-15s %s\n", t["id"], t["name"], t["category"], t["description"])
	}
	fmt.Printf("\nTotal: %d templates\n", len(templates))

	return nil
}

func runWorkflowTemplatesGet(cmd *cobra.Command, args []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	templateID := args[0]

	daemonAddr := mustGetDaemonAddress()
	resp, err := cliHTTPGet(fmt.Sprintf("%s/api/workflow-templates/%s", daemonAddr, templateID))
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

	fmt.Println(string(body))

	return nil
}
