package commands

import (
	"os"

	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for OpenFang.

Supported shells:
  - bash
  - zsh
  - fish
  - powershell

Example:
  # Bash
  fangclaw-go completion bash > /etc/bash_completion.d/fangclaw

  # Zsh
  fangclaw-go completion zsh > ~/.zsh/completions/_fangclaw

  # Fish
  fangclaw-go completion fish > ~/.config/fish/completions/fangclaw.fish`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE:      runCompletion,
	}
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]

	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		return cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
	default:
		return cmd.Root().GenZshCompletion(os.Stdout)
	}
}
