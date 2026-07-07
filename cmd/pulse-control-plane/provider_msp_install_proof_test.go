package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
)

func TestProviderMSPCommandExposesInstallProof(t *testing.T) {
	cmd := newProviderMSPCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "install-proof" {
			return
		}
	}
	t.Fatal("provider-msp install-proof command is not registered")
}

func TestProviderMSPInstallProofRunsFreshInstallSequence(t *testing.T) {
	cfg := testProviderMSPProofConfig(t)
	var steps []string
	var statusCalls int
	fakeRuntime := &fakeProviderMSPInstallProofRuntime{
		proof: healthyProviderMSPInstallProofProofReport(),
		onProof: func(opts providerMSPProofOptions) {
			steps = append(steps, "proof")
			if opts.Cleanup {
				t.Fatal("install-proof must run workspace proof with Cleanup=false so backup captures proof tenants first")
			}
			if opts.WorkspaceCount != 2 || opts.InstallType != "pbs" {
				t.Fatalf("proof opts = count %d install %q", opts.WorkspaceCount, opts.InstallType)
			}
			if opts.TargetPath != "/settings/infrastructure?add=linux-host" {
				t.Fatalf("TargetPath = %q", opts.TargetPath)
			}
		},
		onCleanup: func([]string) {
			steps = append(steps, "cleanup")
		},
	}

	report, err := runProviderMSPInstallProofWithDependencies(context.Background(), cfg, providerMSPInstallProofOptions{
		AccountName:          "Example MSP",
		OwnerEmail:           "Owner@Example.com",
		WorkspacePrefix:      "Provider MSP Proof",
		WorkspaceCount:       2,
		InstallType:          "pbs",
		TargetPath:           "/settings/infrastructure?add=linux-host",
		BackupOutput:         "/tmp/provider-msp-install-proof.tar.gz",
		RestoreTargetDataDir: "/tmp/provider-msp-restore-drill",
		Cleanup:              true,
	}, providerMSPInstallProofDependencies{
		Bootstrap: func(_ context.Context, _ *cloudcp.CPConfig, opts cloudcp.ProviderMSPBootstrapOptions) (*cloudcp.ProviderMSPBootstrapResult, error) {
			steps = append(steps, "bootstrap")
			if opts.GenerateMagicLink {
				t.Fatal("install-proof bootstrap should not generate a magic link")
			}
			if opts.AccountName != "Example MSP" || opts.OwnerEmail != "owner@example.com" {
				t.Fatalf("bootstrap opts = %#v", opts)
			}
			return healthyProviderMSPInstallProofBootstrap(), nil
		},
		RunPreflight: func(_ context.Context, _ *cloudcp.CPConfig, opts providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
			steps = append(steps, "preflight")
			if opts.AllowEnvPlan || opts.SkipImagePull {
				t.Fatalf("preflight opts = %#v", opts)
			}
			return healthyProviderMSPStatusPreflightReport(), nil
		},
		RunStatus: func(context.Context, *cloudcp.CPConfig, providerMSPStatusOptions) (*providerMSPStatusReport, error) {
			statusCalls++
			steps = append(steps, "status-"+string(rune('0'+statusCalls)))
			return &providerMSPStatusReport{
				OK:             true,
				TotalTenants:   0,
				HealthyTenants: 0,
			}, nil
		},
		NewProofRuntime: func(*cloudcp.CPConfig) (providerMSPInstallProofRuntime, error) {
			steps = append(steps, "proof-runtime")
			return fakeRuntime, nil
		},
		CreateBackup: func(_ context.Context, _ *cloudcp.CPConfig, outputPath string) (*cloudcp.ProviderMSPBackupCreateResult, error) {
			steps = append(steps, "backup-create")
			if outputPath != "/tmp/provider-msp-install-proof.tar.gz" {
				t.Fatalf("backup output = %q", outputPath)
			}
			return &cloudcp.ProviderMSPBackupCreateResult{
				ArchivePath:  outputPath,
				BytesWritten: 4096,
			}, nil
		},
		VerifyBackup: func(_ context.Context, archivePath string) (*cloudcp.ProviderMSPBackupVerifyResult, error) {
			steps = append(steps, "backup-verify")
			if archivePath != "/tmp/provider-msp-install-proof.tar.gz" {
				t.Fatalf("verify archive = %q", archivePath)
			}
			return &cloudcp.ProviderMSPBackupVerifyResult{ArchivePath: archivePath}, nil
		},
		RestoreBackup: func(_ context.Context, opts cloudcp.ProviderMSPBackupRestoreOptions) (*cloudcp.ProviderMSPBackupRestoreResult, error) {
			steps = append(steps, "backup-restore")
			if !opts.DryRun {
				t.Fatal("install-proof restore must be a dry-run")
			}
			if opts.TargetDataDir != "/tmp/provider-msp-restore-drill" {
				t.Fatalf("restore target = %q", opts.TargetDataDir)
			}
			return &cloudcp.ProviderMSPBackupRestoreResult{
				ArchivePath:   opts.ArchivePath,
				TargetDataDir: opts.TargetDataDir,
				DryRun:        true,
				Manifest:      cloudcp.ProviderMSPBackupManifest{RegistryTenantCount: 2},
			}, nil
		},
		Recover: func(_ context.Context, _ *cloudcp.CPConfig, opts cloudcp.ProviderMSPRecoveryOptions) (*cloudcp.ProviderMSPRecoveryReport, error) {
			steps = append(steps, "recovery")
			if !opts.AllDegraded || !opts.DryRun {
				t.Fatalf("recovery opts = %#v", opts)
			}
			return &cloudcp.ProviderMSPRecoveryReport{OK: true, DryRun: true, SkippedCount: 2}, nil
		},
		Now: func() time.Time {
			return time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
		},
	})

	if err != nil {
		t.Fatalf("runProviderMSPInstallProofWithDependencies: %v", err)
	}
	wantSteps := []string{
		"bootstrap",
		"preflight",
		"status-1",
		"proof-runtime",
		"proof",
		"backup-create",
		"backup-verify",
		"backup-restore",
		"recovery",
		"cleanup",
		"status-2",
	}
	if !reflect.DeepEqual(steps, wantSteps) {
		t.Fatalf("steps = %#v, want %#v", steps, wantSteps)
	}
	if !report.OK || !report.BootstrapOK || !report.PreflightOK || !report.InitialStatusOK || !report.WorkspaceProofOK || !report.BackupCreated || !report.BackupVerified || !report.RestoreDryRunOK || !report.RecoveryDryRunOK || !report.CleanupOK || !report.FinalStatusOK {
		t.Fatalf("install proof report incomplete: %#v", report)
	}
	if report.OwnerEmail != "owner@example.com" || report.PlanSource != cloudcp.ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("account proof mismatch: %#v", report)
	}
	if report.WorkspaceCount != 2 || len(report.Workspaces) != 2 {
		t.Fatalf("workspace proof count = %d len=%d", report.WorkspaceCount, len(report.Workspaces))
	}
	if !report.TenantIsolationVerified || !report.DefaultRuntimeIsolationVerified || !report.TokenRotationVerified || !report.RotatedOutTokenRejectionVerified || !report.ReportScheduleVisible || !report.ActiveAlertRollupVisible {
		t.Fatalf("isolation/token proof missing: %#v", report)
	}
	if !report.NonProofTenantCountPreserved {
		t.Fatalf("non-proof tenant count was not preserved: %#v", report)
	}
	if !reflect.DeepEqual(fakeRuntime.cleanupTenantIDs, []string{"ws-proof-01", "ws-proof-02"}) {
		t.Fatalf("cleanup tenant ids = %#v", fakeRuntime.cleanupTenantIDs)
	}
	if !fakeRuntime.closed {
		t.Fatal("proof runtime was not closed")
	}
}

func TestProviderMSPInstallProofExercisesRealDockerlessArtifacts(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-provider-msp-install-proof-missing-docker.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	cfg := testProviderMSPProofConfig(t)
	licenseFile := writeProviderMSPProofLicenseForTest(t, "lic_provider_msp_install_proof", "provider@example.com", "msp_growth")
	cfg.ProviderMSPLicenseFile = licenseFile
	cfg.ProviderMSPLicenseID = "lic_provider_msp_install_proof"
	cfg.ProviderMSPLicenseEmail = "provider@example.com"

	backupPath := filepath.Join(t.TempDir(), "provider-msp-install-proof.tar.gz")
	restoreTarget := filepath.Join(t.TempDir(), "restore-drill")
	now := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)
	runStatus := func(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPStatusOptions) (*providerMSPStatusReport, error) {
		return runProviderMSPStatusWithDependencies(ctx, cfg, opts, providerMSPStatusDependencies{
			RunPreflight: func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
				return healthyProviderMSPStatusPreflightReport(), nil
			},
			Now: func() time.Time { return now },
		})
	}

	report, err := runProviderMSPInstallProofWithDependencies(context.Background(), cfg, providerMSPInstallProofOptions{
		AccountName:          "Example MSP",
		OwnerEmail:           "owner@example.com",
		WorkspacePrefix:      "Provider MSP Proof",
		WorkspaceCount:       2,
		InstallType:          "pve",
		TargetPath:           "/settings/infrastructure?add=linux-host",
		BackupOutput:         backupPath,
		RestoreTargetDataDir: restoreTarget,
		Cleanup:              true,
	}, providerMSPInstallProofDependencies{
		RunPreflight: func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
			return healthyProviderMSPStatusPreflightReport(), nil
		},
		RunStatus: runStatus,
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("runProviderMSPInstallProofWithDependencies: %v", err)
	}
	if !report.OK || !report.WorkspaceProofOK || !report.BackupCreated || !report.BackupVerified || !report.RestoreDryRunOK || !report.RecoveryDryRunOK || !report.CleanupOK || !report.FinalStatusOK {
		t.Fatalf("real install-proof report incomplete: %#v", report)
	}
	if report.BackupPath != backupPath || report.BackupBytes <= 0 {
		t.Fatalf("backup proof = path %q bytes %d", report.BackupPath, report.BackupBytes)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup archive was not written: %v", err)
	}
	if report.RestoreTargetDataDir != restoreTarget {
		t.Fatalf("RestoreTargetDataDir = %q, want %q", report.RestoreTargetDataDir, restoreTarget)
	}
	if report.WorkspaceCount != 2 || !report.AgentReportIngestVerified || !report.InstallTokenBoundaryOK || !report.TokenRotationVerified || !report.HandoffExchangeVerified {
		t.Fatalf("workspace proof did not exercise tenant runtime boundaries: %#v", report)
	}
	if !report.NonProofTenantCountPreserved || report.InitialStatusTotalTenants != 0 || report.FinalStatusTotalTenants != 0 {
		t.Fatalf("proof tenant cleanup did not preserve the starting tenant count: %#v", report)
	}
	entries, err := os.ReadDir(cfg.TenantsDir())
	if err == nil && len(entries) != 0 {
		t.Fatalf("proof tenant dirs remain after cleanup: %v", entries)
	}
}

func TestProviderMSPInstallProofCleansUpAfterBackupFailure(t *testing.T) {
	cfg := testProviderMSPProofConfig(t)
	backupErr := errors.New("backup writer failed")
	fakeRuntime := &fakeProviderMSPInstallProofRuntime{
		proof: healthyProviderMSPInstallProofProofReport(),
	}

	report, err := runProviderMSPInstallProofWithDependencies(context.Background(), cfg, providerMSPInstallProofOptions{
		AccountName:          "Example MSP",
		OwnerEmail:           "owner@example.com",
		WorkspacePrefix:      "Provider MSP Proof",
		WorkspaceCount:       2,
		InstallType:          "pve",
		TargetPath:           "/settings/infrastructure?add=linux-host",
		BackupOutput:         "/tmp/provider-msp-install-proof.tar.gz",
		RestoreTargetDataDir: "/tmp/provider-msp-restore-drill",
		Cleanup:              true,
	}, providerMSPInstallProofDependencies{
		Bootstrap: func(context.Context, *cloudcp.CPConfig, cloudcp.ProviderMSPBootstrapOptions) (*cloudcp.ProviderMSPBootstrapResult, error) {
			return healthyProviderMSPInstallProofBootstrap(), nil
		},
		RunPreflight: func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
			return healthyProviderMSPStatusPreflightReport(), nil
		},
		RunStatus: func(context.Context, *cloudcp.CPConfig, providerMSPStatusOptions) (*providerMSPStatusReport, error) {
			return &providerMSPStatusReport{OK: true}, nil
		},
		NewProofRuntime: func(*cloudcp.CPConfig) (providerMSPInstallProofRuntime, error) {
			return fakeRuntime, nil
		},
		CreateBackup: func(context.Context, *cloudcp.CPConfig, string) (*cloudcp.ProviderMSPBackupCreateResult, error) {
			return nil, backupErr
		},
		VerifyBackup: func(context.Context, string) (*cloudcp.ProviderMSPBackupVerifyResult, error) {
			t.Fatal("backup verify should not run after create failure")
			return nil, nil
		},
		RestoreBackup: func(context.Context, cloudcp.ProviderMSPBackupRestoreOptions) (*cloudcp.ProviderMSPBackupRestoreResult, error) {
			t.Fatal("backup restore should not run after create failure")
			return nil, nil
		},
		Recover: func(context.Context, *cloudcp.CPConfig, cloudcp.ProviderMSPRecoveryOptions) (*cloudcp.ProviderMSPRecoveryReport, error) {
			t.Fatal("recovery should not run after create failure")
			return nil, nil
		},
	})

	if err == nil {
		t.Fatal("expected install-proof to fail")
	}
	if !strings.Contains(err.Error(), "backup create") {
		t.Fatalf("error = %v, want backup create failure", err)
	}
	if report == nil || report.OK {
		t.Fatalf("report.OK = %v, want false", report != nil && report.OK)
	}
	if !report.CleanupOK {
		t.Fatalf("proof workspaces were not cleaned up after failure: %#v", report)
	}
	if !reflect.DeepEqual(fakeRuntime.cleanupTenantIDs, []string{"ws-proof-01", "ws-proof-02"}) {
		t.Fatalf("cleanup tenant ids = %#v", fakeRuntime.cleanupTenantIDs)
	}
}

func healthyProviderMSPInstallProofBootstrap() *cloudcp.ProviderMSPBootstrapResult {
	return &cloudcp.ProviderMSPBootstrapResult{
		AccountID:      "acct_provider",
		AccountName:    "Example MSP",
		OwnerUserID:    "usr_owner",
		OwnerEmail:     "owner@example.com",
		PlanVersion:    "msp_growth",
		PlanSource:     cloudcp.ProviderMSPPlanSourceLicenseFile,
		LicenseID:      "lic_provider_msp_test",
		LicenseEmail:   "provider@example.com",
		WorkspaceLimit: 15,
	}
}

func healthyProviderMSPInstallProofProofReport() *providerMSPProofReport {
	return &providerMSPProofReport{
		AccountID:                 "acct_provider",
		AccountName:               "Example MSP",
		OwnerUserID:               "usr_owner",
		OwnerEmail:                "owner@example.com",
		PlanVersion:               "msp_growth",
		PlanSource:                cloudcp.ProviderMSPPlanSourceLicenseFile,
		LicenseID:                 "lic_provider_msp_test",
		LicenseEmail:              "provider@example.com",
		WorkspaceLimit:            15,
		WorkspaceCount:            2,
		RuntimeContainerVerified:  true,
		HandoffExchangeVerified:   true,
		InstallTokenBoundaryOK:    true,
		SetupFactsTokenUseVisible: true,
		AgentReportIngestVerified: true,
		TokenRotationVerified:     true,
		ReportScheduleVisible:     true,
		ActiveAlertRollupVisible:  true,
		Workspaces: []providerMSPProofWorkspace{
			healthyProviderMSPInstallProofWorkspace("ws-proof-01", "Provider MSP Proof 01"),
			healthyProviderMSPInstallProofWorkspace("ws-proof-02", "Provider MSP Proof 02"),
		},
	}
}

func healthyProviderMSPInstallProofWorkspace(tenantID, displayName string) providerMSPProofWorkspace {
	return providerMSPProofWorkspace{
		TenantID:                    tenantID,
		DisplayName:                 displayName,
		State:                       "active",
		PlanVersion:                 "msp_growth",
		ContainerID:                 "ctr-" + tenantID,
		PublicURL:                   "https://" + tenantID + ".msp.example.com",
		InstallType:                 "pve",
		InstallTokenID:              "tok-" + tenantID,
		InstallCommandGenerated:     true,
		AgentTokenAuthVerified:      true,
		SetupFactsTokenUseVisible:   true,
		AgentReportIngestVerified:   true,
		AgentReportAgentID:          "agent-" + tenantID,
		AgentReportHostname:         "pve1",
		TokenRotationVerified:       true,
		RotatedInstallTokenID:       "tok-rotated-" + tenantID,
		OldInstallTokenRejected:     true,
		RotatedAgentReportVerified:  true,
		HandoffExchangeVerified:     true,
		HandoffTargetPath:           "/settings/infrastructure?add=linux-host",
		ReportScheduleCreated:       true,
		ReportScheduleID:            "sched-" + tenantID,
		ReportScheduleVisible:       true,
		ReportScheduleCount:         1,
		DisabledReportScheduleCount: 0,
		ActiveAlertPersisted:        true,
		ActiveAlertRollupVisible:    true,
		CriticalAlertCount:          1,
		WarningAlertCount:           1,
	}
}

type fakeProviderMSPInstallProofRuntime struct {
	proof            *providerMSPProofReport
	proofErr         error
	cleanupErr       error
	cleanupTenantIDs []string
	closed           bool
	onProof          func(providerMSPProofOptions)
	onCleanup        func([]string)
}

func (f *fakeProviderMSPInstallProofRuntime) RunProviderMSPProof(_ context.Context, opts providerMSPProofOptions) (*providerMSPProofReport, error) {
	if f.onProof != nil {
		f.onProof(opts)
	}
	return f.proof, f.proofErr
}

func (f *fakeProviderMSPInstallProofRuntime) CleanupProviderMSPProofTenants(_ context.Context, tenantIDs []string) error {
	f.cleanupTenantIDs = append([]string(nil), tenantIDs...)
	if f.onCleanup != nil {
		f.onCleanup(tenantIDs)
	}
	return f.cleanupErr
}

func (f *fakeProviderMSPInstallProofRuntime) Close() {
	f.closed = true
}
