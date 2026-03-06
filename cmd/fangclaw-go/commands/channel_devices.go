package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func channelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "Manage channel integrations (setup, test, enable, disable)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured channels and their status",
		RunE:  runChannelList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "setup [channel]",
		Short: "Interactive setup wizard for a channel",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runChannelSetup,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "test <channel>",
		Short: "Test a channel by sending a test message",
		Args:  cobra.ExactArgs(1),
		RunE:  runChannelTest,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "enable <channel>",
		Short: "Enable a channel",
		Args:  cobra.ExactArgs(1),
		RunE:  runChannelEnable,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable <channel>",
		Short: "Disable a channel without removing its configuration",
		Args:  cobra.ExactArgs(1),
		RunE:  runChannelDisable,
	})

	return cmd
}

func runChannelList(cmd *cobra.Command, args []string) error {
	fmt.Println("Configured Channels:")
	fmt.Println("(Channel management requires daemon)")
	fmt.Println("")
	fmt.Println("Available channels (not configured):")
	channels := []string{"telegram", "discord", "slack", "whatsapp", "sms", "email"}
	for _, ch := range channels {
		fmt.Printf("  - %s\n", ch)
	}
	return nil
}

func runChannelSetup(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		fmt.Printf("Setting up channel: %s\n", args[0])
	} else {
		fmt.Println("Interactive channel setup...")
	}
	fmt.Println("(Channel setup requires daemon)")
	return nil
}

func runChannelTest(cmd *cobra.Command, args []string) error {
	channel := args[0]
	fmt.Printf("Testing channel: %s\n", channel)
	fmt.Println("(Channel test requires daemon)")
	return nil
}

func runChannelEnable(cmd *cobra.Command, args []string) error {
	channel := args[0]
	fmt.Printf("Enabling channel: %s\n", channel)
	return nil
}

func runChannelDisable(cmd *cobra.Command, args []string) error {
	channel := args[0]
	fmt.Printf("Disabling channel: %s\n", channel)
	return nil
}

func devicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Device pairing and token management",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List paired devices",
		RunE:  runDevicesList,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "pair",
		Short: "Pair a new device",
		RunE:  runDevicesPair,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove <device-id>",
		Short: "Remove a paired device",
		Args:  cobra.ExactArgs(1),
		RunE:  runDevicesRemove,
	})

	return cmd
}

func runDevicesList(cmd *cobra.Command, args []string) error {
	fmt.Println("Paired devices:")
	fmt.Println("(Device management requires daemon)")
	return nil
}

func runDevicesPair(cmd *cobra.Command, args []string) error {
	fmt.Println("Pairing new device...")
	fmt.Println("(Use 'fangclaw-go qr' to generate QR code)")
	return nil
}

func runDevicesRemove(cmd *cobra.Command, args []string) error {
	deviceID := args[0]
	fmt.Printf("Removing device: %s\n", deviceID)
	return nil
}

func qrCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "qr",
		Short: "Generate device pairing QR code",
		RunE:  runQr,
	}
}

func runQr(cmd *cobra.Command, args []string) error {
	fmt.Println("Generating device pairing QR code...")
	fmt.Println("(QR generation requires daemon)")
	fmt.Println("")
	fmt.Println("To pair a device:")
	fmt.Println("1. Install OpenFang mobile app")
	fmt.Println("2. Scan this QR code")
	return nil
}
