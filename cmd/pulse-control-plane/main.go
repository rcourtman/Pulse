package main

import (
	"context"
	"fmt"
	"os"
	"time"

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

func newTenantRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant-runtime",
		Short: "Operate on hosted tenant runtime containers",
	}
	cmd.AddCommand(newTenantRuntimeRolloutCmd())
	return cmd
}

func newTenantRuntimeRolloutCmd() *cobra.Command {
	var tenantID string
	var image string
	var runID string
	var snapshotRoot string
	var healthTimeout time.Duration
	var prunePrevious bool

	cmd := &cobra.Command{
		Use:   "rollout",
		Short: "Canonically roll a hosted tenant onto a target runtime image",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			result, err := cloudcp.RolloutTenantRuntime(cmd.Context(), cfg, cloudcp.TenantRuntimeRolloutOptions{
				TenantID:      tenantID,
				Image:         image,
				RunID:         runID,
				SnapshotRoot:  snapshotRoot,
				HealthTimeout: healthTimeout,
				PrunePrevious: prunePrevious,
			})
			if err != nil {
				return err
			}

			fmt.Printf("tenant_id=%s\n", result.TenantID)
			fmt.Printf("active_container_id=%s\n", result.ActiveContainerID)
			fmt.Printf("active_image_ref=%s\n", result.ActiveImageRef)
			fmt.Printf("active_image_id=%s\n", result.ActiveImageID)
			fmt.Printf("reconciled_only=%t\n", result.ReconciledOnly)
			if result.PreviousContainerID != "" {
				fmt.Printf("previous_container_id=%s\n", result.PreviousContainerID)
			}
			if result.BackupContainerName != "" {
				fmt.Printf("backup_container_name=%s\n", result.BackupContainerName)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Hosted tenant ID to roll")
	cmd.Flags().StringVar(&image, "image", "", "Target Pulse runtime image reference")
	cmd.Flags().StringVar(&runID, "run-id", "", "Operator-visible rollout run identifier")
	cmd.Flags().StringVar(&snapshotRoot, "snapshot-root", "", "Override tenant snapshot root (default: <CP_DATA_DIR>/backups/rollout)")
	cmd.Flags().DurationVar(&healthTimeout, "health-timeout", 90*time.Second, "How long to wait for the target runtime to become healthy")
	cmd.Flags().BoolVar(&prunePrevious, "prune-previous", false, "Remove the preserved pre-rollout container after success")
	_ = cmd.MarkFlagRequired("tenant-id")
	_ = cmd.MarkFlagRequired("image")
	return cmd
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newTenantRuntimeCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
