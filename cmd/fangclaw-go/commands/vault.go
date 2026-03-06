package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func vaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage the credential vault (init, set, list, remove)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize the credential vault",
		RunE:  runVaultInit,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key>",
		Short: "Store a credential in the vault",
		Args:  cobra.ExactArgs(1),
		RunE:  runVaultSet,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all keys in the vault (values are hidden)",
		RunE:  runVaultList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove <key>",
		Short: "Remove a credential from the vault",
		Args:  cobra.ExactArgs(1),
		RunE:  runVaultRemove,
	})

	return cmd
}

func runVaultInit(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	vaultDir := filepath.Join(homeDir, ".fangclaw-go", "vault")
	if err := os.MkdirAll(vaultDir, 0700); err != nil {
		return fmt.Errorf("failed to create vault directory: %w", err)
	}

	vaultFile := filepath.Join(vaultDir, "credentials.enc")
	if _, err := os.Stat(vaultFile); os.IsNotExist(err) {
		// Create empty vault
		if err := os.WriteFile(vaultFile, []byte("{}"), 0600); err != nil {
			return fmt.Errorf("failed to create vault file: %w", err)
		}
		fmt.Println("Vault initialized at:", vaultFile)
	} else {
		fmt.Println("Vault already exists at:", vaultFile)
	}

	return nil
}

func runVaultSet(cmd *cobra.Command, args []string) error {
	key := args[0]

	fmt.Printf("Enter value for %s: ", key)
	// In a real implementation, this would read securely
	fmt.Println("(Vault set functionality - requires daemon)")
	fmt.Printf("Credential '%s' would be stored in vault\n", key)

	return nil
}

func runVaultList(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	vaultFile := filepath.Join(homeDir, ".fangclaw", "vault", "credentials.enc")
	if _, err := os.Stat(vaultFile); os.IsNotExist(err) {
		fmt.Println("Vault not initialized. Run 'fangclaw-go vault init' first.")
		return nil
	}

	fmt.Println("Keys in vault:")
	fmt.Println("  (Vault listing requires daemon for encrypted access)")

	return nil
}

func runVaultRemove(cmd *cobra.Command, args []string) error {
	key := args[0]
	fmt.Printf("Credential '%s' would be removed from vault\n", key)
	return nil
}
