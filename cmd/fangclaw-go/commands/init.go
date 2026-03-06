package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initQuick bool

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize OpenFang (create ~/.fangclaw-go/ and default config)",
		Long: `Initialize OpenFang by creating the configuration directory
and default configuration file.`,
		RunE: runInit,
	}

	cmd.Flags().BoolVar(&initQuick, "quick", false, "Quick mode: no prompts, just write config")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	fangclawGoDir := filepath.Join(homeDir, ".fangclaw-go")

	// Create directories
	dirs := []string{
		fangclawGoDir,
		filepath.Join(fangclawGoDir, "data"),
		filepath.Join(fangclawGoDir, "agents"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		fmt.Printf("Created: %s\n", dir)
	}

	// Write default config if not exists
	configPath := filepath.Join(fangclawGoDir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := `# OpenFang Agent OS configuration
# See https://github.com/RightNow-AI/fangclaw for documentation

api_listen = "127.0.0.1:4200"

[default_model]
provider = "groq"
model = "llama-3.3-70b-versatile"
api_key_env = "GROQ_API_KEY"

[memory]
decay_rate = 0.05
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Printf("Created: %s\n", configPath)
	} else {
		fmt.Printf("Config already exists: %s\n", configPath)
	}

	// Write .env.example if not exists
	envPath := filepath.Join(fangclawGoDir, ".fangclaw-go.env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		envExample := `# OpenFang Environment Variables
# Copy this file to .fangclaw-go.env and add your API keys

# Groq (free tier available)
# GROQ_API_KEY=your_groq_key

# Anthropic
# ANTHROPIC_API_KEY=your_anthropic_key

# OpenAI
# OPENAI_API_KEY=your_openai_key

# Gemini
# GEMINI_API_KEY=your_gemini_key

# DeepSeek
# DEEPSEEK_API_KEY=your_deepseek_key

# OpenRouter
# OPENROUTER_API_KEY=your_openrouter_key
`
		if err := os.WriteFile(envPath, []byte(envExample), 0644); err != nil {
			return fmt.Errorf("failed to write .env: %w", err)
		}
		fmt.Printf("Created: %s\n", envPath)
	}

	fmt.Println("\n✓ OpenFang initialized!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit ~/.fangclaw-go/.fangclaw-go.env")
	fmt.Println("  2. Add your API key to ~/.fangclaw-go/.fangclaw-go.env")
	fmt.Println("  3. fangclaw-go start")

	return nil
}
