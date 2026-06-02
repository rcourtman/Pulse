package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/spf13/cobra"
)

func newProviderMSPRecoverCmd() *cobra.Command {
	var opts cloudcp.ProviderMSPRecoveryOptions

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover failed or degraded provider-hosted MSP client workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			report, err := cloudcp.RecoverProviderMSPWorkspaces(cmd.Context(), cfg, opts)
			printProviderMSPRecoveryReport(report)
			return err
		},
	}

	cmd.Flags().StringArrayVar(&opts.TenantIDs, "tenant-id", nil, "Client workspace tenant ID to recover (repeatable)")
	cmd.Flags().BoolVar(&opts.AllDegraded, "all-degraded", false, "Recover all failed, stuck provisioning, or unhealthy active client workspaces")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the recovery plan without mutating tenant runtimes")
	cmd.Flags().BoolVar(&opts.AllowEnvPlan, "allow-env-plan", false, "Allow CP_PROVIDER_MSP_PLAN_VERSION fallback instead of a signed provider MSP license file for local development")
	cmd.Flags().StringVar(&opts.Image, "image", "", "Tenant runtime image to use during recovery (default: CP_PULSE_IMAGE)")
	cmd.Flags().StringVar(&opts.RunID, "run-id", "", "Operator-visible recovery run identifier")
	cmd.Flags().StringVar(&opts.SnapshotRoot, "snapshot-root", "", "Override tenant snapshot root (default: <CP_DATA_DIR>/backups/rollout)")
	cmd.Flags().DurationVar(&opts.HealthTimeout, "health-timeout", 90*time.Second, "How long to wait for the recovered runtime to become healthy")
	cmd.Flags().BoolVar(&opts.PrunePrevious, "prune-previous", false, "Remove the preserved pre-recovery container after success")
	return cmd
}

func printProviderMSPRecoveryReport(report *cloudcp.ProviderMSPRecoveryReport) {
	if report == nil {
		fmt.Println("provider_msp_recovery_ok=false")
		return
	}
	fmt.Printf("provider_msp_recovery_ok=%t\n", report.OK)
	fmt.Printf("dry_run=%t\n", report.DryRun)
	fmt.Printf("plan_version=%s\n", report.PlanVersion)
	fmt.Printf("plan_source=%s\n", report.PlanSource)
	fmt.Printf("license_id=%s\n", report.LicenseID)
	fmt.Printf("license_email=%s\n", report.LicenseEmail)
	fmt.Printf("workspace_limit=%d\n", report.WorkspaceLimit)
	fmt.Printf("recover_count=%d\n", report.RecoverCount)
	fmt.Printf("recovered_count=%d\n", report.RecoveredCount)
	fmt.Printf("skipped_count=%d\n", report.SkippedCount)
	fmt.Printf("error_count=%d\n", report.ErrorCount)
	for _, item := range report.Items {
		fields := []string{
			"workspace=" + item.TenantID,
			"display_name=" + quoteProviderMSPRecoveryField(item.DisplayName),
			"state=" + item.State,
			"action=" + item.Action,
			"reason=" + quoteProviderMSPRecoveryField(item.Reason),
			fmt.Sprintf("stuck_provisioning=%t", item.StuckProvisioning),
			fmt.Sprintf("recovered=%t", item.Recovered),
		}
		if item.PreviousContainerID != "" {
			fields = append(fields, "previous_container_id="+item.PreviousContainerID)
		}
		if item.ActiveContainerID != "" {
			fields = append(fields, "active_container_id="+item.ActiveContainerID)
		}
		if item.ActiveImageRef != "" {
			fields = append(fields, "active_image_ref="+item.ActiveImageRef)
		}
		if item.ActiveImageID != "" {
			fields = append(fields, "active_image_id="+item.ActiveImageID)
		}
		if item.RestoredMissing {
			fields = append(fields, "restored_missing=true")
		}
		if item.ReconciledOnly {
			fields = append(fields, "reconciled_only=true")
		}
		if item.Error != "" {
			fields = append(fields, "error="+quoteProviderMSPRecoveryField(item.Error))
		}
		fmt.Println(strings.Join(fields, " "))
	}
}

func quoteProviderMSPRecoveryField(value string) string {
	return fmt.Sprintf("%q", strings.TrimSpace(value))
}
