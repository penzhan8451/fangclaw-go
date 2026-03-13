package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/penzhan8451/fangclaw-go/internal/runtime/model_catalog"
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

func getModelCatalog() (*model_catalog.ModelCatalog, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	modelCatalogPath := filepath.Join(homeDir, ".fangclaw-go", "model_catalog.json")
	catalog := model_catalog.NewModelCatalog(modelCatalogPath)
	return catalog, nil
}

func runModelList(cmd *cobra.Command, args []string) error {
	catalog, err := getModelCatalog()
	if err != nil {
		return err
	}
	models := catalog.ListModels()

	if modelsJSON {
		json.NewEncoder(os.Stdout).Encode(models)
		return nil
	}

	fmt.Printf("%-40s %-12s %s\n", "MODEL", "PROVIDER", "CONTEXT")
	fmt.Println("--------------------------------------------------------------------")
	for _, m := range models {
		modelID := fmt.Sprintf("%s/%s", m.Provider, m.ModelName)
		fmt.Printf("%-40s %-12s %d\n", modelID, m.Provider, m.ContextWindow)
	}

	return nil
}

func runModelProviders(cmd *cobra.Command, args []string) error {
	catalog, err := getModelCatalog()
	if err != nil {
		return err
	}
	providers := catalog.ListProviders()

	if modelsJSON {
		json.NewEncoder(os.Stdout).Encode(providers)
		return nil
	}

	fmt.Printf("%-15s %-20s %s\n", "PROVIDER", "ENV VAR", "MODEL COUNT")
	fmt.Println("------------------------------------------------")
	for _, p := range providers {
		fmt.Printf("%-15s %-20s %d\n", p.DisplayName, p.APIKeyEnv, p.ModelCount)
	}

	return nil
}
