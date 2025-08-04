package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long:  `Manage Pulse configuration settings`,
}

var configInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show configuration information",
	Long:  `Display information about Pulse configuration`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Pulse Configuration Information")
		fmt.Println("==============================")
		fmt.Println()
		fmt.Println("Configuration is managed through the web UI.")
		fmt.Println("Settings are stored in encrypted files at /etc/pulse/")
		fmt.Println()
		fmt.Println("Configuration files:")
		fmt.Println("  - nodes.enc      : Encrypted Proxmox node configurations")
		fmt.Println("  - email.enc      : Encrypted email settings")
		fmt.Println("  - system.json    : System settings (polling interval, etc)")
		fmt.Println("  - alerts.json    : Alert rules and thresholds")
		fmt.Println("  - webhooks.json  : Webhook configurations")
		fmt.Println()
		fmt.Println("To configure Pulse, use the Settings tab in the web UI.")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInfoCmd)
}