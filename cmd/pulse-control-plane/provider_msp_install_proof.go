package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/spf13/cobra"
)

type providerMSPInstallProofOptions struct {
	AccountName                string
	OwnerEmail                 string
	WorkspacePrefix            string
	WorkspaceCount             int
	InstallType                string
	TargetPath                 string
	BackupOutput               string
	RestoreTargetDataDir       string
	Cleanup                    bool
	AllowEnvPlan               bool
	AllowNonProofWorkspaceName bool
	SkipImagePull              bool
}

type providerMSPInstallProofReport struct {
	OK                                  bool
	AccountID                           string
	AccountName                         string
	OwnerUserID                         string
	OwnerEmail                          string
	PlanVersion                         string
	PlanSource                          string
	LicenseID                           string
	LicenseEmail                        string
	WorkspaceLimit                      int
	BootstrapOK                         bool
	PreflightOK                         bool
	InitialStatusOK                     bool
	WorkspaceProofOK                    bool
	BackupCreated                       bool
	BackupVerified                      bool
	RestoreDryRunOK                     bool
	RecoveryDryRunOK                    bool
	CleanupRequested                    bool
	CleanupOK                           bool
	FinalStatusOK                       bool
	BackupPath                          string
	BackupBytes                         int64
	RestoreTargetDataDir                string
	RecoveryRecoverCount                int
	RecoverySkippedCount                int
	WorkspaceCount                      int
	Workspaces                          []providerMSPProofWorkspace
	DockerlessProvisioning              bool
	RuntimeContainerVerified            bool
	TenantIsolationVerified             bool
	DefaultRuntimeIsolationVerified     bool
	HandoffExchangeVerified             bool
	InstallTokenBoundaryOK              bool
	SetupFactsTokenUseVisible           bool
	AgentReportIngestVerified           bool
	TokenRotationVerified               bool
	RotatedOutTokenRejectionVerified    bool
	NonProofTenantCountPreserved        bool
	InitialStatusTotalTenants           int
	FinalStatusTotalTenants             int
	FinalStatusHealthyTenants           int
	FinalStatusFailedTenants            int
	FinalStatusUnhealthyTenants         int
	FinalStatusStuckProvisioningTenants int
	Failures                            []string
}

type providerMSPInstallProofRuntime interface {
	RunProviderMSPProof(context.Context, providerMSPProofOptions) (*providerMSPProofReport, error)
	CleanupProviderMSPProofTenants(context.Context, []string) error
	Close()
}

type providerMSPInstallProofDependencies struct {
	Bootstrap       func(context.Context, *cloudcp.CPConfig, cloudcp.ProviderMSPBootstrapOptions) (*cloudcp.ProviderMSPBootstrapResult, error)
	RunPreflight    func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error)
	RunStatus       func(context.Context, *cloudcp.CPConfig, providerMSPStatusOptions) (*providerMSPStatusReport, error)
	NewProofRuntime func(*cloudcp.CPConfig) (providerMSPInstallProofRuntime, error)
	CreateBackup    func(context.Context, *cloudcp.CPConfig, string) (*cloudcp.ProviderMSPBackupCreateResult, error)
	VerifyBackup    func(context.Context, string) (*cloudcp.ProviderMSPBackupVerifyResult, error)
	RestoreBackup   func(context.Context, cloudcp.ProviderMSPBackupRestoreOptions) (*cloudcp.ProviderMSPBackupRestoreResult, error)
	Recover         func(context.Context, *cloudcp.CPConfig, cloudcp.ProviderMSPRecoveryOptions) (*cloudcp.ProviderMSPRecoveryReport, error)
	Now             func() time.Time
}

type providerMSPInstallProofRuntimeAdapter struct {
	runtime *providerMSPProofRuntime
}

func newProviderMSPInstallProofCmd() *cobra.Command {
	opts := providerMSPInstallProofOptions{
		WorkspacePrefix: defaultProviderMSPProofWorkspacePrefix,
		WorkspaceCount:  2,
		InstallType:     "pve",
		TargetPath:      "/settings/infrastructure?add=linux-host",
		Cleanup:         true,
	}

	cmd := &cobra.Command{
		Use:   "install-proof",
		Short: "Prove a fresh provider-hosted MSP install end to end",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			report, err := runProviderMSPInstallProof(cmd.Context(), cfg, opts)
			printProviderMSPInstallProofReport(report)
			return err
		},
	}

	cmd.Flags().StringVar(&opts.AccountName, "account-name", "", "Provider MSP account display name")
	cmd.Flags().StringVar(&opts.OwnerEmail, "owner-email", "", "Provider owner email address")
	cmd.Flags().StringVar(&opts.WorkspacePrefix, "workspace-prefix", opts.WorkspacePrefix, "Proof workspace display-name prefix")
	cmd.Flags().IntVar(&opts.WorkspaceCount, "workspace-count", opts.WorkspaceCount, "Number of proof client workspaces to create; minimum 2")
	cmd.Flags().StringVar(&opts.InstallType, "install-type", opts.InstallType, "Hosted tenant install command type: pve or pbs")
	cmd.Flags().StringVar(&opts.TargetPath, "target-path", opts.TargetPath, "Tenant-local target path to verify during handoff exchange")
	cmd.Flags().StringVar(&opts.BackupOutput, "backup-output", "", "Backup archive path created during the install proof")
	cmd.Flags().StringVar(&opts.RestoreTargetDataDir, "restore-target-data-dir", "", "Target CP_DATA_DIR used for the restore dry-run")
	cmd.Flags().BoolVar(&opts.Cleanup, "cleanup", opts.Cleanup, "Delete proof workspaces after backup and restore proof completes")
	cmd.Flags().BoolVar(&opts.AllowEnvPlan, "allow-env-plan", false, "Allow CP_PROVIDER_MSP_PLAN_VERSION fallback instead of a signed provider MSP license file for local development")
	cmd.Flags().BoolVar(&opts.AllowNonProofWorkspaceName, "allow-non-proof-workspace-name", false, "Allow proof workspace names without proof/canary/rehearsal markers")
	cmd.Flags().BoolVar(&opts.SkipImagePull, "skip-image-pull", false, "Inspect the tenant runtime image instead of pulling it during preflight")
	_ = cmd.MarkFlagRequired("account-name")
	_ = cmd.MarkFlagRequired("owner-email")
	return cmd
}

func runProviderMSPInstallProof(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPInstallProofOptions) (*providerMSPInstallProofReport, error) {
	return runProviderMSPInstallProofWithDependencies(ctx, cfg, opts, providerMSPInstallProofDependencies{})
}

func runProviderMSPInstallProofWithDependencies(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPInstallProofOptions, deps providerMSPInstallProofDependencies) (*providerMSPInstallProofReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	opts, err := normalizeProviderMSPInstallProofOptions(cfg, opts, deps.Now)
	if err != nil {
		return nil, err
	}
	deps = normalizeProviderMSPInstallProofDependencies(deps)

	report := &providerMSPInstallProofReport{
		OK:                   true,
		CleanupOK:            !opts.Cleanup,
		CleanupRequested:     opts.Cleanup,
		BackupPath:           opts.BackupOutput,
		RestoreTargetDataDir: opts.RestoreTargetDataDir,
	}
	addFailure := func(format string, args ...any) {
		report.OK = false
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
	}
	finishWithError := func(err error) (*providerMSPInstallProofReport, error) {
		if !report.OK && len(report.Failures) > 0 {
			return report, providerMSPInstallProofError(report)
		}
		if err == nil {
			return report, providerMSPInstallProofError(report)
		}
		return report, fmt.Errorf("provider MSP install proof failed: %w", err)
	}

	bootstrap, err := deps.Bootstrap(ctx, cfg, cloudcp.ProviderMSPBootstrapOptions{
		AccountName:       opts.AccountName,
		OwnerEmail:        opts.OwnerEmail,
		GenerateMagicLink: false,
	})
	if err != nil {
		addFailure("bootstrap provider MSP account: %v", err)
		return finishWithError(err)
	}
	report.BootstrapOK = true
	populateProviderMSPInstallProofAccount(report, bootstrap)

	preflight, err := deps.RunPreflight(ctx, cfg, providerMSPPreflightOptions{
		AllowEnvPlan:  opts.AllowEnvPlan,
		SkipImagePull: opts.SkipImagePull,
	})
	if err != nil {
		addFailure("preflight: %v", err)
		return finishWithError(err)
	}
	report.PreflightOK = preflight != nil && preflight.OK
	if !report.PreflightOK {
		err := fmt.Errorf("preflight report did not pass")
		addFailure("%v", err)
		return finishWithError(err)
	}
	if preflight != nil && preflight.WorkspaceLimit > 0 {
		report.WorkspaceLimit = preflight.WorkspaceLimit
	}

	initialStatus, err := deps.RunStatus(ctx, cfg, providerMSPStatusOptions{
		AllowEnvPlan: opts.AllowEnvPlan,
		PullImage:    false,
	})
	if err != nil {
		addFailure("initial status: %v", err)
		return finishWithError(err)
	}
	report.InitialStatusOK = initialStatus != nil && initialStatus.OK
	if initialStatus != nil {
		report.InitialStatusTotalTenants = initialStatus.TotalTenants
	}
	if !report.InitialStatusOK {
		err := fmt.Errorf("initial status report did not pass")
		addFailure("%v", err)
		return finishWithError(err)
	}

	rt, err := deps.NewProofRuntime(cfg)
	if err != nil {
		addFailure("open proof runtime: %v", err)
		return finishWithError(err)
	}
	defer rt.Close()

	var proofTenantIDs []string
	cleanupProofTenants := func() {
		if !opts.Cleanup || report.CleanupOK || len(proofTenantIDs) == 0 {
			return
		}
		if err := rt.CleanupProviderMSPProofTenants(context.Background(), proofTenantIDs); err != nil {
			addFailure("cleanup proof workspaces: %v", err)
			return
		}
		report.CleanupOK = true
	}
	failAfterProof := func(err error) (*providerMSPInstallProofReport, error) {
		cleanupProofTenants()
		return finishWithError(err)
	}

	proof, err := rt.RunProviderMSPProof(ctx, providerMSPProofOptions{
		AccountName:                opts.AccountName,
		OwnerEmail:                 opts.OwnerEmail,
		WorkspacePrefix:            opts.WorkspacePrefix,
		WorkspaceCount:             opts.WorkspaceCount,
		InstallType:                opts.InstallType,
		TargetPath:                 opts.TargetPath,
		Cleanup:                    false,
		AllowEnvPlan:               opts.AllowEnvPlan,
		AllowNonProofWorkspaceName: opts.AllowNonProofWorkspaceName,
	})
	if proof != nil {
		populateProviderMSPInstallProofProof(report, proof)
		proofTenantIDs = providerMSPInstallProofTenantIDs(proof.Workspaces)
	}
	if err != nil {
		addFailure("workspace proof: %v", err)
		return failAfterProof(err)
	}
	report.WorkspaceProofOK = true

	backup, err := deps.CreateBackup(ctx, cfg, opts.BackupOutput)
	if err != nil {
		addFailure("backup create: %v", err)
		return failAfterProof(err)
	}
	report.BackupCreated = true
	if backup != nil {
		report.BackupPath = backup.ArchivePath
		report.BackupBytes = backup.BytesWritten
	}
	if strings.TrimSpace(report.BackupPath) == "" {
		err := fmt.Errorf("backup create did not return an archive path")
		addFailure("%v", err)
		return failAfterProof(err)
	}

	if _, err := deps.VerifyBackup(ctx, report.BackupPath); err != nil {
		addFailure("backup verify: %v", err)
		return failAfterProof(err)
	}
	report.BackupVerified = true

	restore, err := deps.RestoreBackup(ctx, cloudcp.ProviderMSPBackupRestoreOptions{
		ArchivePath:   report.BackupPath,
		TargetDataDir: opts.RestoreTargetDataDir,
		DryRun:        true,
	})
	if err != nil {
		addFailure("backup restore dry-run: %v", err)
		return failAfterProof(err)
	}
	report.RestoreDryRunOK = restore != nil && restore.DryRun
	if restore != nil && restore.TargetDataDir != "" {
		report.RestoreTargetDataDir = restore.TargetDataDir
	}
	if !report.RestoreDryRunOK {
		err := fmt.Errorf("backup restore did not return a dry-run result")
		addFailure("%v", err)
		return failAfterProof(err)
	}

	recovery, err := deps.Recover(ctx, cfg, cloudcp.ProviderMSPRecoveryOptions{
		AllDegraded:  true,
		DryRun:       true,
		AllowEnvPlan: opts.AllowEnvPlan,
	})
	if err != nil {
		addFailure("recovery dry-run: %v", err)
		return failAfterProof(err)
	}
	report.RecoveryDryRunOK = recovery != nil && recovery.OK && recovery.DryRun
	if recovery != nil {
		report.RecoveryRecoverCount = recovery.RecoverCount
		report.RecoverySkippedCount = recovery.SkippedCount
	}
	if !report.RecoveryDryRunOK {
		err := fmt.Errorf("recovery did not return a passing dry-run result")
		addFailure("%v", err)
		return failAfterProof(err)
	}

	cleanupProofTenants()

	finalStatus, err := deps.RunStatus(ctx, cfg, providerMSPStatusOptions{
		AllowEnvPlan: opts.AllowEnvPlan,
		PullImage:    false,
	})
	if err != nil {
		addFailure("final status: %v", err)
		return finishWithError(err)
	}
	report.FinalStatusOK = finalStatus != nil && finalStatus.OK
	if finalStatus != nil {
		report.FinalStatusTotalTenants = finalStatus.TotalTenants
		report.FinalStatusHealthyTenants = finalStatus.HealthyTenants
		report.FinalStatusFailedTenants = finalStatus.FailedTenants
		report.FinalStatusUnhealthyTenants = finalStatus.UnhealthyTenants
		report.FinalStatusStuckProvisioningTenants = len(finalStatus.StuckProvisioningTenants)
	}
	if !report.FinalStatusOK {
		err := fmt.Errorf("final status report did not pass")
		addFailure("%v", err)
		return finishWithError(err)
	}
	report.NonProofTenantCountPreserved = !opts.Cleanup || report.FinalStatusTotalTenants == report.InitialStatusTotalTenants
	if !report.NonProofTenantCountPreserved {
		err := fmt.Errorf("final tenant count %d does not match initial tenant count %d after proof cleanup", report.FinalStatusTotalTenants, report.InitialStatusTotalTenants)
		addFailure("%v", err)
		return finishWithError(err)
	}
	return report, providerMSPInstallProofError(report)
}

func normalizeProviderMSPInstallProofOptions(cfg *cloudcp.CPConfig, opts providerMSPInstallProofOptions, now func() time.Time) (providerMSPInstallProofOptions, error) {
	proofOpts, err := normalizeProviderMSPProofOptions(providerMSPProofOptions{
		AccountName:                opts.AccountName,
		OwnerEmail:                 opts.OwnerEmail,
		WorkspacePrefix:            opts.WorkspacePrefix,
		WorkspaceCount:             opts.WorkspaceCount,
		InstallType:                opts.InstallType,
		TargetPath:                 opts.TargetPath,
		AllowEnvPlan:               opts.AllowEnvPlan,
		AllowNonProofWorkspaceName: opts.AllowNonProofWorkspaceName,
	})
	if err != nil {
		return opts, err
	}
	opts.AccountName = proofOpts.AccountName
	opts.OwnerEmail = proofOpts.OwnerEmail
	opts.WorkspacePrefix = proofOpts.WorkspacePrefix
	opts.WorkspaceCount = proofOpts.WorkspaceCount
	opts.InstallType = proofOpts.InstallType
	opts.TargetPath = proofOpts.TargetPath
	opts.BackupOutput = strings.TrimSpace(opts.BackupOutput)
	opts.RestoreTargetDataDir = strings.TrimSpace(opts.RestoreTargetDataDir)
	if opts.BackupOutput == "" {
		if now == nil {
			now = func() time.Time { return time.Now().UTC() }
		}
		opts.BackupOutput = cloudcp.DefaultProviderMSPBackupPath(cfg, now())
	}
	if opts.RestoreTargetDataDir == "" {
		opts.RestoreTargetDataDir = defaultProviderMSPInstallProofRestoreTargetDataDir(cfg)
	}
	return opts, nil
}

func normalizeProviderMSPInstallProofDependencies(deps providerMSPInstallProofDependencies) providerMSPInstallProofDependencies {
	if deps.Bootstrap == nil {
		deps.Bootstrap = cloudcp.BootstrapProviderMSP
	}
	if deps.RunPreflight == nil {
		deps.RunPreflight = runProviderMSPPreflight
	}
	if deps.RunStatus == nil {
		deps.RunStatus = runProviderMSPStatus
	}
	if deps.NewProofRuntime == nil {
		deps.NewProofRuntime = func(cfg *cloudcp.CPConfig) (providerMSPInstallProofRuntime, error) {
			rt, err := newProviderMSPProofRuntimeFromConfig(cfg)
			if err != nil {
				return nil, err
			}
			return providerMSPInstallProofRuntimeAdapter{runtime: rt}, nil
		}
	}
	if deps.CreateBackup == nil {
		deps.CreateBackup = cloudcp.CreateProviderMSPBackup
	}
	if deps.VerifyBackup == nil {
		deps.VerifyBackup = cloudcp.VerifyProviderMSPBackup
	}
	if deps.RestoreBackup == nil {
		deps.RestoreBackup = cloudcp.RestoreProviderMSPBackup
	}
	if deps.Recover == nil {
		deps.Recover = cloudcp.RecoverProviderMSPWorkspaces
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	return deps
}

func (rt providerMSPInstallProofRuntimeAdapter) RunProviderMSPProof(ctx context.Context, opts providerMSPProofOptions) (*providerMSPProofReport, error) {
	if rt.runtime == nil {
		return nil, fmt.Errorf("provider MSP proof runtime is not initialized")
	}
	return rt.runtime.runProviderMSPProof(ctx, opts)
}

func (rt providerMSPInstallProofRuntimeAdapter) CleanupProviderMSPProofTenants(ctx context.Context, tenantIDs []string) error {
	if rt.runtime == nil || rt.runtime.registry == nil {
		return fmt.Errorf("provider MSP proof runtime is not initialized")
	}
	tenants := make([]*registry.Tenant, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenantID = strings.TrimSpace(tenantID)
		if tenantID == "" {
			continue
		}
		tenant, err := rt.runtime.registry.Get(tenantID)
		if err != nil {
			return fmt.Errorf("load proof workspace %s for cleanup: %w", tenantID, err)
		}
		if tenant == nil {
			tenant = &registry.Tenant{ID: tenantID}
		}
		tenants = append(tenants, tenant)
	}
	return rt.runtime.cleanupProviderMSPProofTenants(ctx, tenants)
}

func (rt providerMSPInstallProofRuntimeAdapter) Close() {
	if rt.runtime != nil {
		rt.runtime.close()
	}
}

func populateProviderMSPInstallProofAccount(report *providerMSPInstallProofReport, bootstrap *cloudcp.ProviderMSPBootstrapResult) {
	if report == nil || bootstrap == nil {
		return
	}
	report.AccountID = bootstrap.AccountID
	report.AccountName = bootstrap.AccountName
	report.OwnerUserID = bootstrap.OwnerUserID
	report.OwnerEmail = bootstrap.OwnerEmail
	report.PlanVersion = bootstrap.PlanVersion
	report.PlanSource = bootstrap.PlanSource
	report.LicenseID = bootstrap.LicenseID
	report.LicenseEmail = bootstrap.LicenseEmail
	report.WorkspaceLimit = bootstrap.WorkspaceLimit
}

func populateProviderMSPInstallProofProof(report *providerMSPInstallProofReport, proof *providerMSPProofReport) {
	if report == nil || proof == nil {
		return
	}
	report.WorkspaceCount = proof.WorkspaceCount
	report.Workspaces = append([]providerMSPProofWorkspace(nil), proof.Workspaces...)
	report.DockerlessProvisioning = proof.DockerlessProvisioning
	report.RuntimeContainerVerified = proof.RuntimeContainerVerified
	report.HandoffExchangeVerified = proof.HandoffExchangeVerified
	report.InstallTokenBoundaryOK = proof.InstallTokenBoundaryOK
	report.SetupFactsTokenUseVisible = proof.SetupFactsTokenUseVisible
	report.AgentReportIngestVerified = proof.AgentReportIngestVerified
	report.TokenRotationVerified = proof.TokenRotationVerified
	report.TenantIsolationVerified = proof.InstallTokenBoundaryOK
	report.DefaultRuntimeIsolationVerified = proof.AgentReportIngestVerified
	report.RotatedOutTokenRejectionVerified = providerMSPInstallProofAllRotatedOutTokensRejected(proof.Workspaces)
	if strings.TrimSpace(report.AccountID) == "" {
		report.AccountID = proof.AccountID
		report.AccountName = proof.AccountName
		report.OwnerUserID = proof.OwnerUserID
		report.OwnerEmail = proof.OwnerEmail
		report.PlanVersion = proof.PlanVersion
		report.PlanSource = proof.PlanSource
		report.LicenseID = proof.LicenseID
		report.LicenseEmail = proof.LicenseEmail
		report.WorkspaceLimit = proof.WorkspaceLimit
	}
}

func providerMSPInstallProofTenantIDs(workspaces []providerMSPProofWorkspace) []string {
	ids := make([]string, 0, len(workspaces))
	seen := map[string]struct{}{}
	for _, workspace := range workspaces {
		tenantID := strings.TrimSpace(workspace.TenantID)
		if tenantID == "" {
			continue
		}
		if _, ok := seen[tenantID]; ok {
			continue
		}
		seen[tenantID] = struct{}{}
		ids = append(ids, tenantID)
	}
	return ids
}

func providerMSPInstallProofAllRotatedOutTokensRejected(workspaces []providerMSPProofWorkspace) bool {
	if len(workspaces) == 0 {
		return false
	}
	for _, workspace := range workspaces {
		if !workspace.OldInstallTokenRejected {
			return false
		}
	}
	return true
}

func defaultProviderMSPInstallProofRestoreTargetDataDir(cfg *cloudcp.CPConfig) string {
	base := providerMSPRestoreDefaultDataDir()
	if cfg != nil && strings.TrimSpace(cfg.DataDir) != "" {
		base = strings.TrimSpace(cfg.DataDir)
	}
	return filepath.Join(base, "install-proof-restore-drill")
}

func providerMSPInstallProofError(report *providerMSPInstallProofReport) error {
	if report == nil || report.OK {
		return nil
	}
	return fmt.Errorf("provider MSP install proof failed: %s", strings.Join(report.Failures, "; "))
}

func printProviderMSPInstallProofReport(report *providerMSPInstallProofReport) {
	if report == nil {
		fmt.Println("provider_msp_install_proof_ok=false")
		return
	}
	fmt.Printf("provider_msp_install_proof_ok=%t\n", report.OK)
	fmt.Printf("account_id=%s\n", report.AccountID)
	fmt.Printf("account_name=%s\n", report.AccountName)
	fmt.Printf("owner_user_id=%s\n", report.OwnerUserID)
	fmt.Printf("owner_email=%s\n", report.OwnerEmail)
	fmt.Printf("plan_version=%s\n", report.PlanVersion)
	fmt.Printf("plan_source=%s\n", report.PlanSource)
	fmt.Printf("license_id=%s\n", report.LicenseID)
	fmt.Printf("license_email=%s\n", report.LicenseEmail)
	fmt.Printf("workspace_limit=%d\n", report.WorkspaceLimit)
	fmt.Printf("bootstrap_ok=%t\n", report.BootstrapOK)
	fmt.Printf("preflight_ok=%t\n", report.PreflightOK)
	fmt.Printf("initial_status_ok=%t\n", report.InitialStatusOK)
	fmt.Printf("workspace_proof_ok=%t\n", report.WorkspaceProofOK)
	fmt.Printf("backup_created=%t\n", report.BackupCreated)
	fmt.Printf("backup_verified=%t\n", report.BackupVerified)
	fmt.Printf("restore_dry_run_ok=%t\n", report.RestoreDryRunOK)
	fmt.Printf("recovery_dry_run_ok=%t\n", report.RecoveryDryRunOK)
	fmt.Printf("cleanup_requested=%t\n", report.CleanupRequested)
	fmt.Printf("cleanup_ok=%t\n", report.CleanupOK)
	fmt.Printf("final_status_ok=%t\n", report.FinalStatusOK)
	fmt.Printf("backup_path=%s\n", report.BackupPath)
	fmt.Printf("backup_bytes=%d\n", report.BackupBytes)
	fmt.Printf("restore_target_data_dir=%s\n", report.RestoreTargetDataDir)
	fmt.Printf("recovery_recover_count=%d\n", report.RecoveryRecoverCount)
	fmt.Printf("recovery_skipped_count=%d\n", report.RecoverySkippedCount)
	fmt.Printf("workspace_count=%d\n", report.WorkspaceCount)
	fmt.Printf("dockerless_provisioning=%t\n", report.DockerlessProvisioning)
	fmt.Printf("runtime_container_verified=%t\n", report.RuntimeContainerVerified)
	fmt.Printf("tenant_isolation_verified=%t\n", report.TenantIsolationVerified)
	fmt.Printf("default_runtime_isolation_verified=%t\n", report.DefaultRuntimeIsolationVerified)
	fmt.Printf("handoff_exchange_verified=%t\n", report.HandoffExchangeVerified)
	fmt.Printf("install_token_boundary_verified=%t\n", report.InstallTokenBoundaryOK)
	fmt.Printf("setup_facts_token_use_visible=%t\n", report.SetupFactsTokenUseVisible)
	fmt.Printf("agent_report_ingest_verified=%t\n", report.AgentReportIngestVerified)
	fmt.Printf("token_rotation_verified=%t\n", report.TokenRotationVerified)
	fmt.Printf("rotated_out_token_rejection_verified=%t\n", report.RotatedOutTokenRejectionVerified)
	fmt.Printf("non_proof_tenant_count_preserved=%t\n", report.NonProofTenantCountPreserved)
	fmt.Printf("initial_status_total_tenants=%d\n", report.InitialStatusTotalTenants)
	fmt.Printf("final_status_total_tenants=%d\n", report.FinalStatusTotalTenants)
	fmt.Printf("final_status_healthy_tenants=%d\n", report.FinalStatusHealthyTenants)
	fmt.Printf("final_status_failed_tenants=%d\n", report.FinalStatusFailedTenants)
	fmt.Printf("final_status_unhealthy_tenants=%d\n", report.FinalStatusUnhealthyTenants)
	fmt.Printf("final_status_stuck_provisioning_tenants=%d\n", report.FinalStatusStuckProvisioningTenants)
	for _, workspace := range report.Workspaces {
		fmt.Printf("workspace=%s display_name=%q state=%s plan_version=%s container_id=%s public_url=%s install_type=%s install_token_id=%s install_command_generated=%t agent_token_auth_verified=%t setup_facts_token_use_visible=%t agent_report_ingest_verified=%t agent_report_agent_id=%s agent_report_hostname=%s token_rotation_verified=%t rotated_install_token_id=%s old_install_token_rejected=%t rotated_agent_report_verified=%t handoff_exchange_verified=%t handoff_target_path=%s entitlement_lease_checked=%t entitlement_lease_verified=%t entitlement_white_label=%t entitlement_skipped_reason=%s\n",
			workspace.TenantID,
			workspace.DisplayName,
			workspace.State,
			workspace.PlanVersion,
			workspace.ContainerID,
			workspace.PublicURL,
			workspace.InstallType,
			workspace.InstallTokenID,
			workspace.InstallCommandGenerated,
			workspace.AgentTokenAuthVerified,
			workspace.SetupFactsTokenUseVisible,
			workspace.AgentReportIngestVerified,
			workspace.AgentReportAgentID,
			workspace.AgentReportHostname,
			workspace.TokenRotationVerified,
			workspace.RotatedInstallTokenID,
			workspace.OldInstallTokenRejected,
			workspace.RotatedAgentReportVerified,
			workspace.HandoffExchangeVerified,
			workspace.HandoffTargetPath,
			workspace.EntitlementLeaseChecked,
			workspace.EntitlementLeaseVerified,
			workspace.EntitlementWhiteLabel,
			workspace.EntitlementSkippedReason,
		)
	}
	for _, failure := range report.Failures {
		fmt.Printf("failure=%s\n", failure)
	}
}
