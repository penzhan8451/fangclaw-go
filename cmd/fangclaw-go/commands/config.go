package commands

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or edit configuration",
		Long:  "Manage FangClawGo configuration.",
	}

	cmd.AddCommand(configShowCmd())
	cmd.AddCommand(configGetCmd())
	cmd.AddCommand(configSetCmd())
	cmd.AddCommand(configUnsetCmd())

	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current configuration",
		RunE:  runConfigShow,
	}
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	encoder := toml.NewEncoder(os.Stdout)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value, err := config.Get(key)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}
	fmt.Println(value)
	return nil
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	if err := config.Set(key, value); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func configUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigUnset,
	}
}

func runConfigUnset(cmd *cobra.Command, args []string) error {
	key := args[0]

	if err := config.Unset(key); err != nil {
		return fmt.Errorf("failed to unset config: %w", err)
	}

	fmt.Printf("Removed %s\n", key)
	return nil
}
