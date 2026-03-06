package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func modelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Browse models and providers",
		Long:  "Browse available LLM models, aliases, and providers.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available models",
		RunE:  runModelList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "providers",
		Short: "List known LLM providers",
		RunE:  runModelProviders,
	})

	return cmd
}

var modelsJSON bool

func runModelList(cmd *cobra.Command, args []string) error {
	models := getDefaultModels()
	if modelsJSON {
		json.NewEncoder(os.Stdout).Encode(models)
		return nil
	}

	fmt.Printf("%-40s %-12s %s\n", "MODEL", "PROVIDER", "CONTEXT")
	fmt.Println("--------------------------------------------------------------------")
	for _, m := range models {
		fmt.Printf("%-40s %-12s %d\n", m["id"], m["provider"], m["context_size"])
	}

	return nil
}

func runModelProviders(cmd *cobra.Command, args []string) error {
	providers := getDefaultProviders()
	if modelsJSON {
		json.NewEncoder(os.Stdout).Encode(providers)
		return nil
	}

	fmt.Printf("%-15s %-10s %s\n", "PROVIDER", "ENV VAR", "STREAMING")
	fmt.Println("------------------------------------------------")
	for _, p := range providers {
		fmt.Printf("%-15s %-10s %s\n", p["name"], p["api_key_env"], "✓")
	}

	return nil
}

func getDefaultModels() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "groq/llama-3.3-70b-versatile", "name": "Llama 3.3 70B", "provider": "groq", "context_size": 128000},
		{"id": "anthropic/claude-sonnet-4-20250514", "name": "Claude Sonnet 4", "provider": "anthropic", "context_size": 200000},
		{"id": "openai/gpt-4o", "name": "GPT-4o", "provider": "openai", "context_size": 128000},
		{"id": "gemini/gemini-2.0-flash", "name": "Gemini 2.0 Flash", "provider": "gemini", "context_size": 1000000},
		{"id": "deepseek/deepseek-chat", "name": "DeepSeek Chat", "provider": "deepseek", "context_size": 64000},
	}
}

func getDefaultProviders() []map[string]string {
	return []map[string]string{
		{"id": "groq", "name": "Groq", "api_key_env": "GROQ_API_KEY"},
		{"id": "anthropic", "name": "Anthropic", "api_key_env": "ANTHROPIC_API_KEY"},
		{"id": "openai", "name": "OpenAI", "api_key_env": "OPENAI_API_KEY"},
		{"id": "gemini", "name": "Gemini", "api_key_env": "GEMINI_API_KEY"},
		{"id": "deepseek", "name": "DeepSeek", "api_key_env": "DEEPSEEK_API_KEY"},
	}
}
