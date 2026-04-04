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
	cmd.AddCommand(newTenantRuntimeReconcileCmd())
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

func newTenantRuntimeReconcileCmd() *cobra.Command {
	var tenantIDs []string
	var all bool
	var dryRun bool
	var runID string
	var snapshotRoot string
	var healthTimeout time.Duration
	var prunePrevious bool

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Reconcile hosted tenant runtime contract drift without changing each tenant's image line",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			plan, err := cloudcp.PlanTenantRuntimeContractReconcile(cmd.Context(), cfg, cloudcp.TenantRuntimeContractReconcilePlanOptions{
				TenantIDs: tenantIDs,
				All:       all,
			})
			if err != nil {
				return err
			}

			if dryRun {
				printTenantRuntimeReconcilePlan(plan)
				return nil
			}

			rolloutCount := 0
			noopCount := 0
			skipCount := 0
			failureCount := 0
			for _, item := range plan.Tenants {
				if item == nil {
					continue
				}
				switch item.Action {
				case "skip":
					skipCount++
					fmt.Printf("tenant_id=%s\nstatus=skip\nreason=%s\n\n", item.TenantID, item.Reason)
					continue
				case "noop":
					noopCount++
					fmt.Printf("tenant_id=%s\nstatus=noop\nimage_ref=%s\nreason=%s\n\n", item.TenantID, item.ImageRef, item.Reason)
					continue
				}

				result, err := cloudcp.RolloutTenantRuntime(cmd.Context(), cfg, cloudcp.TenantRuntimeRolloutOptions{
					TenantID:      item.TenantID,
					Image:         item.ImageRef,
					RunID:         runID,
					SnapshotRoot:  snapshotRoot,
					HealthTimeout: healthTimeout,
					PrunePrevious: prunePrevious,
				})
				if err != nil {
					failureCount++
					fmt.Printf("tenant_id=%s\nstatus=error\nimage_ref=%s\nerror=%v\n\n", item.TenantID, item.ImageRef, err)
					continue
				}
				rolloutCount++
				fmt.Printf("tenant_id=%s\nstatus=rolled\nactive_container_id=%s\nactive_image_ref=%s\nactive_image_id=%s\nreconciled_only=%t\n",
					result.TenantID,
					result.ActiveContainerID,
					result.ActiveImageRef,
					result.ActiveImageID,
					result.ReconciledOnly,
				)
				if result.PreviousContainerID != "" {
					fmt.Printf("previous_container_id=%s\n", result.PreviousContainerID)
				}
				if result.BackupContainerName != "" {
					fmt.Printf("backup_container_name=%s\n", result.BackupContainerName)
				}
				fmt.Println()
			}

			fmt.Printf("summary_rollout=%d\nsummary_noop=%d\nsummary_skip=%d\nsummary_error=%d\n", rolloutCount, noopCount, skipCount, failureCount)
			if failureCount > 0 {
				return fmt.Errorf("%d tenant runtime reconciles failed", failureCount)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&tenantIDs, "tenant-id", nil, "Hosted tenant ID to reconcile (repeatable)")
	cmd.Flags().BoolVar(&all, "all", false, "Reconcile all registered tenants")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the reconcile plan without mutating tenant runtimes")
	cmd.Flags().StringVar(&runID, "run-id", "", "Operator-visible reconcile run identifier")
	cmd.Flags().StringVar(&snapshotRoot, "snapshot-root", "", "Override tenant snapshot root (default: <CP_DATA_DIR>/backups/rollout)")
	cmd.Flags().DurationVar(&healthTimeout, "health-timeout", 90*time.Second, "How long to wait for the target runtime to become healthy")
	cmd.Flags().BoolVar(&prunePrevious, "prune-previous", false, "Remove the preserved pre-rollout container after success")
	return cmd
}

func printTenantRuntimeReconcilePlan(plan *cloudcp.TenantRuntimeContractReconcilePlan) {
	if plan == nil {
		fmt.Println("summary_total=0")
		return
	}
	rolloutCount := 0
	noopCount := 0
	skipCount := 0
	for _, item := range plan.Tenants {
		if item == nil {
			continue
		}
		switch item.Action {
		case "rollout":
			rolloutCount++
		case "noop":
			noopCount++
		default:
			skipCount++
		}
		fmt.Printf("tenant_id=%s\naction=%s\nreason=%s\n", item.TenantID, item.Action, item.Reason)
		if item.LiveContainerID != "" {
			fmt.Printf("live_container_id=%s\n", item.LiveContainerID)
		}
		if item.ImageRef != "" {
			fmt.Printf("image_ref=%s\n", item.ImageRef)
		}
		if item.LiveRouteHost != "" || item.DesiredRouteHost != "" {
			fmt.Printf("live_route_host=%s\ndesired_route_host=%s\n", item.LiveRouteHost, item.DesiredRouteHost)
		}
		if item.LivePublicURL != "" || item.DesiredPublicURL != "" {
			fmt.Printf("live_public_url=%s\ndesired_public_url=%s\n", item.LivePublicURL, item.DesiredPublicURL)
		}
		fmt.Println()
	}
	fmt.Printf("summary_rollout=%d\nsummary_noop=%d\nsummary_skip=%d\nsummary_total=%d\n", rolloutCount, noopCount, skipCount, len(plan.Tenants))
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
