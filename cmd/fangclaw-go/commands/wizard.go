package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/penzhan8451/fangclaw-go/internal/wizard"
)

func wizardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wizard",
		Short: "AI Agent creation wizard from natural language",
		Long: `Generate agent configurations from natural language descriptions.

The wizard takes your description of what you want an agent to do,
extracts structured intent, and generates a complete agent manifest.

Examples:
  fangclaw-go wizard create "Create a web research assistant that can search and summarize"
  fangclaw-go wizard create --intent intent.json --output agent.json
  fangclaw-go wizard create --description "Code reviewer" --capabilities code,file --tier complex`,
	}

	cmd.AddCommand(wizardCreateCmd())
	cmd.AddCommand(wizardTemplatesCmd())

	return cmd
}

func wizardCreateCmd() *cobra.Command {
	var (
		intentFile    string
		outputFile    string
		description   string
		name          string
		task          string
		capabilities  []string
		skills        []string
		modelTier     string
		scheduled     bool
		schedule      string
		showManifest  bool
	)

	cmd := &cobra.Command{
		Use:   "create [description]",
		Short: "Create an agent from natural language description",
		Long: `Create an agent configuration from a natural language description.

If a description argument is provided, it will be used as the agent's task.
Alternatively, use --intent to load from a JSON file, or specify individual
fields with --name, --description, --task, etc.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var intent wizard.AgentIntent

			if intentFile != "" {
				data, err := os.ReadFile(intentFile)
				if err != nil {
					return fmt.Errorf("failed to read intent file: %w", err)
				}
				wiz := wizard.NewSetupWizard()
				intent, err = wiz.ParseIntent(string(data))
				if err != nil {
					return fmt.Errorf("failed to parse intent: %w", err)
				}
			} else {
				intent.Name = name
				intent.Description = description
				intent.ModelTier = modelTier
				intent.Scheduled = scheduled
				intent.Capabilities = capabilities
				intent.Skills = skills

				if len(args) > 0 && args[0] != "" {
					intent.Task = args[0]
				} else {
					intent.Task = task
				}

				if schedule != "" {
					intent.Schedule = &schedule
				}

				if intent.Name == "" {
					intent.Name = "my-agent"
				}
				if intent.Description == "" {
					intent.Description = intent.Task
				}
				if intent.ModelTier == "" {
					intent.ModelTier = "medium"
				}
			}

			if intent.Task == "" {
				return fmt.Errorf("please provide a task description (as argument or --task)")
			}

			wiz := wizard.NewSetupWizard()
			plan := wiz.BuildPlan(intent)

			fmt.Println("\n" + plan.Summary)

			manifestJSON, err := wiz.GenerateJSON(plan.Manifest)
			if err != nil {
				return fmt.Errorf("failed to generate manifest: %w", err)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(manifestJSON), 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Printf("\nManifest written to: %s\n", outputFile)
			} else if showManifest {
				fmt.Println("\n--- Generated Manifest ---")
				fmt.Println(manifestJSON)
			}

			if len(plan.SkillsToInstall) > 0 {
				fmt.Printf("\nSkills to install: %s\n", strings.Join(plan.SkillsToInstall, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&intentFile, "intent", "i", "", "Path to JSON file with intent definition")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Path to write generated manifest JSON")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Agent description")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Agent name (default: my-agent)")
	cmd.Flags().StringVarP(&task, "task", "t", "", "Agent task description")
	cmd.Flags().StringSliceVarP(&capabilities, "capabilities", "c", nil, "Agent capabilities (web, file, memory, shell, browser, code)")
	cmd.Flags().StringSliceVarP(&skills, "skills", "s", nil, "Skills to enable")
	cmd.Flags().StringVarP(&modelTier, "tier", "m", "medium", "Model tier (simple, medium, complex)")
	cmd.Flags().BoolVarP(&scheduled, "scheduled", "", false, "Enable scheduled execution")
	cmd.Flags().StringVarP(&schedule, "schedule", "", "", "Schedule expression (cron or interval)")
	cmd.Flags().BoolVarP(&showManifest, "show", "", true, "Show generated manifest")

	return cmd
}

func wizardTemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Show example intent templates",
		Long:  `Display example intent templates that can be used as starting points.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templates := []struct {
				Name        string
				Description string
				Intent      wizard.AgentIntent
			}{
				{
					Name:        "Research Assistant",
					Description: "Web research and summarization agent",
					Intent: wizard.AgentIntent{
						Name:         "research-assistant",
						Description:  "Researches topics and provides summaries",
						Task:         "Search the web for information and provide concise summaries",
						Skills:       []string{},
						ModelTier:    "medium",
						Scheduled:    false,
						Capabilities: []string{"web", "memory"},
					},
				},
				{
					Name:        "Code Reviewer",
					Description: "Code review and improvement suggestions",
					Intent: wizard.AgentIntent{
						Name:         "code-reviewer",
						Description:  "Reviews code and suggests improvements",
						Task:         "Analyze code files and provide feedback on quality, bugs, and improvements",
						Skills:       []string{},
						ModelTier:    "complex",
						Scheduled:    false,
						Capabilities: []string{"code", "file"},
					},
				},
				{
					Name:        "Data Collector",
					Description: "Periodic data collection agent",
					Intent: wizard.AgentIntent{
						Name:         "data-collector",
						Description:  "Collects and monitors data from various sources",
						Task:         "Periodically fetch data from configured sources and store for analysis",
						Skills:       []string{},
						ModelTier:    "simple",
						Scheduled:    true,
						Schedule:     strPtr("0 */6 * * *"),
						Capabilities: []string{"web", "file", "memory"},
					},
				},
				{
					Name:        "Browser Automation",
					Description: "Web browser automation agent",
					Intent: wizard.AgentIntent{
						Name:         "browser-bot",
						Description:  "Automates web browser interactions",
						Task:         "Navigate websites, fill forms, and extract data from web pages",
						Skills:       []string{},
						ModelTier:    "medium",
						Scheduled:    false,
						Capabilities: []string{"browser", "memory"},
					},
				},
			}

			fmt.Println("Available Intent Templates:")
			fmt.Println("==========================\n")

			for _, tpl := range templates {
				fmt.Printf("## %s\n%s\n\n", tpl.Name, tpl.Description)

				intentJSON, _ := json.MarshalIndent(tpl.Intent, "", "  ")
				fmt.Println(string(intentJSON))
				fmt.Println()
			}

			fmt.Println("Usage:")
			fmt.Println("  fangclaw-go wizard create --intent template.json")
			fmt.Println("  fangclaw-go wizard create 'Search the web and summarize' -c web,memory")

			return nil
		},
	}

	return cmd
}

func strPtr(s string) *string {
	return &s
}
