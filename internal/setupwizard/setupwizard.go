package setupwizard

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ProviderInfo struct {
	Name         string
	EnvVar       string
	DefaultModel string
	NeedsKey     bool
}

var Providers = []ProviderInfo{
	{
		Name:         "openai",
		EnvVar:       "OPENAI_API_KEY",
		DefaultModel: "gpt-4o",
		NeedsKey:     true,
	},
	{
		Name:         "anthropic",
		EnvVar:       "ANTHROPIC_API_KEY",
		DefaultModel: "claude-3-5-sonnet-20241022",
		NeedsKey:     true,
	},
	{
		Name:         "groq",
		EnvVar:       "GROQ_API_KEY",
		DefaultModel: "llama-3.3-70b-versatile",
		NeedsKey:     true,
	},
	{
		Name:         "openrouter",
		EnvVar:       "OPENROUTER_API_KEY",
		DefaultModel: "openai/gpt-4o",
		NeedsKey:     true,
	},
	{
		Name:         "ollama",
		EnvVar:       "OLLAMA_API_KEY",
		DefaultModel: "llama3.2",
		NeedsKey:     false,
	},
}

type SetupWizard struct {
	reader *bufio.Reader
}

func NewSetupWizard() *SetupWizard {
	return &SetupWizard{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (sw *SetupWizard) readInput(prompt string) (string, error) {
	fmt.Print(prompt)
	input, err := sw.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (sw *SetupWizard) Run() error {
	fmt.Println("======================================")
	fmt.Println("  FangClaw-go Setup Wizard")
	fmt.Println("======================================")
	fmt.Println()

	fmt.Println("Welcome! Let's set up your FangClaw-go configuration.")
	fmt.Println()

	provider, err := sw.selectProvider()
	if err != nil {
		return err
	}

	var apiKey string
	if provider.NeedsKey {
		apiKey, err = sw.readAPIKey(provider)
		if err != nil {
			return err
		}
	}

	model, err := sw.selectModel(provider)
	if err != nil {
		return err
	}

	dataDir, err := sw.selectDataDir()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("======================================")
	fmt.Println("  Configuration Summary")
	fmt.Println("======================================")
	fmt.Printf("Provider:     %s\n", provider.Name)
	if provider.NeedsKey {
		fmt.Printf("API Key:      %s... (masked)\n", maskAPIKey(apiKey))
	}
	fmt.Printf("Default Model:%s\n", model)
	fmt.Printf("Data Directory:%s\n", dataDir)
	fmt.Println()

	confirm, err := sw.readInput("Is this correct? (yes/no): ")
	if err != nil {
		return err
	}

	if strings.ToLower(confirm) != "yes" && strings.ToLower(confirm) != "y" {
		fmt.Println("Setup cancelled.")
		return nil
	}

	fmt.Println()
	fmt.Println("Writing configuration...")
	if err := sw.writeConfig(provider, apiKey, model, dataDir); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ Setup complete!")
	fmt.Println()
	fmt.Println("You can now run:")
	fmt.Println("  fangclaw-go start")
	fmt.Println()

	return nil
}

func (sw *SetupWizard) selectProvider() (ProviderInfo, error) {
	fmt.Println("Select your LLM provider:")
	fmt.Println()

	for i, p := range Providers {
		keyNote := ""
		if !p.NeedsKey {
			keyNote = " (no API key needed)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, p.Name, keyNote)
	}
	fmt.Println()

	for {
		input, err := sw.readInput("Enter provider number (1-5): ")
		if err != nil {
			return ProviderInfo{}, err
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 1 || choice > len(Providers) {
			fmt.Println("Invalid choice. Please try again.")
			continue
		}

		return Providers[choice-1], nil
	}
}

func (sw *SetupWizard) readAPIKey(provider ProviderInfo) (string, error) {
	fmt.Println()
	fmt.Printf("Please enter your %s API key:\n", provider.Name)
	fmt.Printf("(You can set this later via %s environment variable)\n", provider.EnvVar)
	fmt.Println()

	for {
		input, err := sw.readInput("API Key: ")
		if err != nil {
			return "", err
		}

		if input == "" {
			fmt.Println("API key cannot be empty. Please try again.")
			continue
		}

		return input, nil
	}
}

func (sw *SetupWizard) selectModel(provider ProviderInfo) (string, error) {
	fmt.Println()
	fmt.Printf("Select default model for %s:\n", provider.Name)
	fmt.Println()
	fmt.Printf("  1. %s (recommended)\n", provider.DefaultModel)
	fmt.Println("  2. Custom model")
	fmt.Println()

	for {
		input, err := sw.readInput("Enter choice (1-2): ")
		if err != nil {
			return "", err
		}

		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err == nil && choice == 1 {
			return provider.DefaultModel, nil
		}

		if err == nil && choice == 2 {
			customModel, err := sw.readInput("Enter custom model name: ")
			if err != nil {
				return "", err
			}
			if customModel != "" {
				return customModel, nil
			}
			fmt.Println("Model name cannot be empty.")
			continue
		}

		fmt.Println("Invalid choice. Please try again.")
	}
}

func (sw *SetupWizard) selectDataDir() (string, error) {
	fmt.Println()
	defaultDir := "~/.fangclaw-go"
	fmt.Printf("Select data directory (default: %s):\n", defaultDir)
	fmt.Println()

	input, err := sw.readInput("Data directory (leave empty for default): ")
	if err != nil {
		return "", err
	}

	if input == "" {
		return defaultDir, nil
	}

	return input, nil
}

func (sw *SetupWizard) writeConfig(provider ProviderInfo, apiKey, model, dataDir string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".fangclaw-go")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.toml")

	var configContent strings.Builder
	configContent.WriteString("# FangClaw-go Configuration\n")
	configContent.WriteString("\n")
	configContent.WriteString("[general]\n")
	configContent.WriteString(fmt.Sprintf("data_dir = \"%s\"\n", dataDir))
	configContent.WriteString("\n")
	configContent.WriteString("[models]\n")
	configContent.WriteString(fmt.Sprintf("default_provider = \"%s\"\n", provider.Name))
	configContent.WriteString(fmt.Sprintf("default_model = \"%s\"\n", model))
	configContent.WriteString("\n")

	if provider.NeedsKey {
		configContent.WriteString("[providers]\n")
		configContent.WriteString(fmt.Sprintf("[providers.%s]\n", provider.Name))
		configContent.WriteString(fmt.Sprintf("api_key = \"%s\"\n", apiKey))
		configContent.WriteString("\n")
	}

	if err := os.WriteFile(configPath, []byte(configContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration written to: %s\n", configPath)
	return nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func NeedsSetup() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true
	}
	configPath := filepath.Join(homeDir, ".fangclaw-go", "config.toml")
	_, err = os.Stat(configPath)
	return os.IsNotExist(err)
}
