package main

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version     = "dev"
	BuildTime   = "unknown"
	GitCommit   = "unknown"
	metricsPort = 9091
)

var rootCmd = &cobra.Command{
	Use:     "pulse",
	Short:   "Pulse - Proxmox VE and PBS monitoring system",
	Long:    `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer(context.Background())
	},
}

func runServer(ctx context.Context) error {
	server.MetricsPort = metricsPort
	return server.Run(ctx, Version)
}

func init() {
	// Add config command
	rootCmd.AddCommand(configCmd)
	// Add version command
	rootCmd.AddCommand(versionCmd)
	// Add bootstrap-token command
	rootCmd.AddCommand(bootstrapTokenCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Pulse %s\n", Version)
		if BuildTime != "unknown" {
			fmt.Printf("Built: %s\n", BuildTime)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Commit: %s\n", GitCommit)
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// osExit is defined in bootstrap.go
		osExit(1)
	}
}

// Force rebuild 1769525035
