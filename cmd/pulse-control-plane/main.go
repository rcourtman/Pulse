package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
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

func newCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Audit Pulse Cloud operational readiness",
	}
	cmd.AddCommand(newCloudAuditCmd())
	return cmd
}

func newCloudAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report hosted tenant, container, proof-tenant, and storage guardrail state",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			report, err := cloudcp.AuditCloud(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			printCloudAuditReport(report)
			if !report.OK {
				return fmt.Errorf("cloud audit failed: %s", strings.Join(report.Failures, "; "))
			}
			return nil
		},
	}
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
				if result.RestoredMissing {
					fmt.Printf("restored_missing=%t\n", result.RestoredMissing)
				}
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

func printCloudAuditReport(report *cloudcp.CloudAuditReport) {
	if report == nil {
		fmt.Println("audit_ok=false")
		return
	}
	fmt.Printf("audit_ok=%t\n", report.OK)
	fmt.Printf("tenant_total=%d\n", report.TenantTotal)
	for _, state := range []registry.TenantState{
		registry.TenantStateProvisioning,
		registry.TenantStateActive,
		registry.TenantStateSuspended,
		registry.TenantStateCanceled,
		registry.TenantStateDeleting,
		registry.TenantStateDeleted,
		registry.TenantStateFailed,
	} {
		fmt.Printf("tenant_%s=%d\n", state, report.TenantCounts[state])
	}
	fmt.Printf("tenant_registry_unhealthy_active=%d\n", report.RegistryUnhealthyActive)
	fmt.Printf("docker_managed_total=%d\n", report.DockerManagedTotal)
	fmt.Printf("docker_managed_running=%d\n", report.DockerManagedRunning)
	fmt.Printf("docker_managed_unhealthy=%d\n", report.DockerManagedUnhealthy)
	if report.DockerUnavailable != "" {
		fmt.Printf("docker_unavailable=%s\n", report.DockerUnavailable)
	}
	if report.Storage != nil {
		fmt.Printf("storage_guardrails_enabled=%t\n", report.Storage.Enabled)
		fmt.Printf("storage_ok=%t\n", report.Storage.OK)
		for _, fs := range report.Storage.Filesystems {
			status := "ok"
			if !fs.OK {
				status = "fail"
			}
			fmt.Printf("storage_%s_status=%s\n", fs.Name, status)
			fmt.Printf("storage_%s_path=%s\n", fs.Name, fs.Path)
			fmt.Printf("storage_%s_available_bytes=%d\n", fs.Name, fs.AvailableBytes)
			fmt.Printf("storage_%s_min_available_bytes=%d\n", fs.Name, fs.MinAvailableBytes)
			fmt.Printf("storage_%s_total_bytes=%d\n", fs.Name, fs.TotalBytes)
			if fs.Error != "" {
				fmt.Printf("storage_%s_error=%s\n", fs.Name, fs.Error)
			}
		}
		buildCacheStatus := "ok"
		if !report.Storage.BuildCache.OK {
			buildCacheStatus = "fail"
		}
		fmt.Printf("docker_build_cache_status=%s\n", buildCacheStatus)
		fmt.Printf("docker_build_cache_total_bytes=%d\n", report.Storage.BuildCache.TotalBytes)
		fmt.Printf("docker_build_cache_max_bytes=%d\n", report.Storage.BuildCache.MaxBytes)
		fmt.Printf("docker_build_cache_reclaimable_bytes=%d\n", report.Storage.BuildCache.ReclaimableBytes)
		if report.Storage.BuildCache.Error != "" {
			fmt.Printf("docker_build_cache_error=%s\n", report.Storage.BuildCache.Error)
		}
	}
	fmt.Printf("proof_tenant_stale_count=%d\n", len(report.StaleProofTenants))
	for _, tenant := range report.StaleProofTenants {
		fmt.Printf("proof_tenant_stale=%s state=%s account_id=%s email=%s age=%s\n",
			tenant.TenantID,
			tenant.State,
			tenant.AccountID,
			tenant.Email,
			tenant.Age.Round(time.Second),
		)
	}
	fmt.Printf("proof_account_stale_count=%d\n", len(report.StaleProofAccounts))
	for _, account := range report.StaleProofAccounts {
		fmt.Printf("proof_account_stale=%s kind=%s age=%s\n",
			account.AccountID,
			account.Kind,
			account.Age.Round(time.Second),
		)
	}
	fmt.Printf("hosted_paid_orphan_entitlement_count=%d\n", len(report.OrphanPaidHostedEntitlements))
	for _, entitlement := range report.OrphanPaidHostedEntitlements {
		fmt.Printf("hosted_paid_orphan_entitlement=%s tenant_id=%s kind=%s\n",
			entitlement.EntitlementID,
			entitlement.TenantID,
			entitlement.Kind,
		)
	}
	for _, container := range report.ManagedRuntimeContainers {
		if container.State == "running" && (container.HealthStatus == "" || container.HealthStatus == "none" || container.HealthStatus == "healthy") {
			continue
		}
		fmt.Printf("docker_unhealthy_container=%s name=%s state=%s health=%s status=%s\n",
			container.ID,
			container.Name,
			container.State,
			container.HealthStatus,
			container.Status,
		)
	}
	for _, failure := range report.Failures {
		fmt.Printf("failure=%s\n", failure)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newCloudCmd())
	rootCmd.AddCommand(newMobileProofCmd())
	rootCmd.AddCommand(newTenantRuntimeCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
