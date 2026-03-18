package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "pulse-control-plane",
	Short: "Pulse Cloud Control Plane",
	Long:  `Control plane for Pulse Cloud — manages tenant lifecycle, containers, and billing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cloudcp.Run(context.Background(), Version)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Pulse Control Plane %s\n", Version)
		if BuildTime != "unknown" {
			fmt.Printf("Built: %s\n", BuildTime)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Commit: %s\n", GitCommit)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
