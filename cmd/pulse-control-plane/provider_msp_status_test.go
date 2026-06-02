package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestProviderMSPCommandExposesStatus(t *testing.T) {
	cmd := newProviderMSPCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "status" {
			return
		}
	}
	t.Fatal("provider-msp status command is not registered")
}

func TestProviderMSPStatusReportsHealthyOperatorSurface(t *testing.T) {
	cfg := testProviderMSPPreflightConfig(t, cloudcp.ProviderMSPPlanSourceLicenseFile)
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	createProviderMSPStatusTenant(t, cfg, &registry.Tenant{
		ID:            "t-HEALTHY",
		State:         registry.TenantStateActive,
		CreatedAt:     now.Add(-time.Hour),
		HealthCheckOK: true,
	})

	var gotPreflight providerMSPPreflightOptions
	report, err := runProviderMSPStatusWithDependencies(context.Background(), cfg, providerMSPStatusOptions{}, providerMSPStatusDependencies{
		RunPreflight: func(_ context.Context, _ *cloudcp.CPConfig, opts providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
			gotPreflight = opts
			return healthyProviderMSPStatusPreflightReport(), nil
		},
		CheckBackup: func(context.Context, *cloudcp.CPConfig) (*providerMSPBackupStatus, error) {
			return healthyProviderMSPBackupStatus(now), nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("runProviderMSPStatusWithDependencies: %v", err)
	}
	if !report.OK {
		t.Fatalf("report.OK = false, failures = %v", report.Failures)
	}
	if !gotPreflight.SkipImagePull {
		t.Fatal("status preflight should inspect the tenant runtime image by default")
	}
	if report.TotalTenants != 1 || report.HealthyTenants != 1 || report.UnhealthyTenants != 0 {
		t.Fatalf("tenant health summary = total %d healthy %d unhealthy %d", report.TotalTenants, report.HealthyTenants, report.UnhealthyTenants)
	}
	if report.WorkspaceLimit != 15 || report.PlanSource != cloudcp.ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("plan status = %q limit=%d", report.PlanSource, report.WorkspaceLimit)
	}
	if report.LicenseID != "lic_provider_msp_test" || report.LicenseEmail != "provider@example.com" {
		t.Fatalf("license status = %q %q", report.LicenseID, report.LicenseEmail)
	}
	if !report.BackupReadyForUpgrade || report.Backup == nil || !report.Backup.Verified {
		t.Fatalf("backup posture not ready for upgrade: %#v", report.Backup)
	}
	if report.Backup.LatestAge != 2*time.Hour {
		t.Fatalf("backup age = %s, want 2h", report.Backup.LatestAge)
	}
}

func TestProviderMSPStatusFailsOnFailedUnhealthyAndStuckWorkspaces(t *testing.T) {
	cfg := testProviderMSPPreflightConfig(t, cloudcp.ProviderMSPPlanSourceLicenseFile)
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	createProviderMSPStatusTenant(t, cfg, &registry.Tenant{
		ID:            "t-UNHEALTHY",
		State:         registry.TenantStateActive,
		CreatedAt:     now.Add(-time.Hour),
		HealthCheckOK: false,
	})
	createProviderMSPStatusTenant(t, cfg, &registry.Tenant{
		ID:        "t-FAILED",
		State:     registry.TenantStateFailed,
		CreatedAt: now.Add(-time.Hour),
	})
	createProviderMSPStatusTenant(t, cfg, &registry.Tenant{
		ID:        "t-STUCK",
		State:     registry.TenantStateProvisioning,
		CreatedAt: now.Add(-time.Hour),
	})

	report, err := runProviderMSPStatusWithDependencies(context.Background(), cfg, providerMSPStatusOptions{}, providerMSPStatusDependencies{
		RunPreflight: func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
			return healthyProviderMSPStatusPreflightReport(), nil
		},
		Now: func() time.Time { return now },
	})
	if err == nil {
		t.Fatal("expected provider MSP status to fail for degraded workspaces")
	}
	if report == nil || report.OK {
		t.Fatalf("report.OK = %v, want false", report != nil && report.OK)
	}
	if report.FailedTenants != 1 || report.UnhealthyTenants != 1 || len(report.StuckProvisioningTenants) != 1 {
		t.Fatalf("degraded summary = failed %d unhealthy %d stuck %#v", report.FailedTenants, report.UnhealthyTenants, report.StuckProvisioningTenants)
	}
	failures := strings.Join(report.Failures, "; ")
	for _, want := range []string{"failed workspaces: 1", "unhealthy active workspaces: 1", "stuck provisioning workspaces: t-STUCK"} {
		if !strings.Contains(failures, want) {
			t.Fatalf("failures = %q, want %q", failures, want)
		}
	}
}

func TestProviderMSPStatusBackupWarningBecomesFailureWhenRequired(t *testing.T) {
	cfg := testProviderMSPPreflightConfig(t, cloudcp.ProviderMSPPlanSourceLicenseFile)
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	deps := providerMSPStatusDependencies{
		RunPreflight: func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
			return healthyProviderMSPStatusPreflightReport(), nil
		},
		CheckBackup: func(context.Context, *cloudcp.CPConfig) (*providerMSPBackupStatus, error) {
			return &providerMSPBackupStatus{
				Directory: "/data/backups/provider-msp",
				Warning:   "no provider MSP backup archive found; run provider-msp backup create before upgrades or recovery drills",
			}, nil
		},
		Now: func() time.Time { return now },
	}

	report, err := runProviderMSPStatusWithDependencies(context.Background(), cfg, providerMSPStatusOptions{}, deps)
	if err != nil {
		t.Fatalf("status without required backup should warn, not fail: %v", err)
	}
	if !report.OK || report.BackupReadyForUpgrade {
		t.Fatalf("optional backup status = ok %t ready %t", report.OK, report.BackupReadyForUpgrade)
	}
	if got := strings.Join(report.Warnings, "; "); !strings.Contains(got, "provider-msp backup create") {
		t.Fatalf("warnings = %q, want backup create guidance", got)
	}

	requiredReport, err := runProviderMSPStatusWithDependencies(context.Background(), cfg, providerMSPStatusOptions{RequireBackup: true}, deps)
	if err == nil {
		t.Fatal("expected required backup status to fail")
	}
	if requiredReport == nil || requiredReport.OK {
		t.Fatalf("required backup report OK = %v, want false", requiredReport != nil && requiredReport.OK)
	}
	if got := strings.Join(requiredReport.Failures, "; "); !strings.Contains(got, "backup: no provider MSP backup archive found") {
		t.Fatalf("failures = %q, want missing backup failure", got)
	}
}

func healthyProviderMSPStatusPreflightReport() *providerMSPPreflightReport {
	return &providerMSPPreflightReport{
		OK:             true,
		Environment:    "production",
		ControlMode:    string(cloudcp.ControlPlaneModeProviderHostedMSP),
		BaseURL:        "https://msp.example.com",
		PlanVersion:    "msp_growth",
		PlanSource:     cloudcp.ProviderMSPPlanSourceLicenseFile,
		LicenseID:      "lic_provider_msp_test",
		LicenseEmail:   "provider@example.com",
		WorkspaceLimit: 15,
		RegistryReady:  true,
		Docker: &cpDocker.RuntimePrerequisiteReport{
			OK:              true,
			DockerReachable: true,
			NetworkName:     "pulse-provider-msp",
			NetworkOK:       true,
			ImageRef:        "pulse:test",
			ImageAvailable:  true,
		},
		Storage: &cloudcp.StorageGuardrailReport{
			Enabled: true,
			OK:      true,
		},
	}
}

func healthyProviderMSPBackupStatus(now time.Time) *providerMSPBackupStatus {
	return &providerMSPBackupStatus{
		Directory:           "/data/backups/provider-msp",
		LatestPath:          "/data/backups/provider-msp/provider-msp-backup-20260602T100000Z.tar.gz",
		LatestCreatedAt:     now.Add(-2 * time.Hour),
		LatestModifiedAt:    now.Add(-time.Hour),
		LatestBytes:         4096,
		Verified:            true,
		LicenseIncluded:     true,
		RegistryTenantCount: 1,
		RuntimeTenantCount:  1,
	}
}

func createProviderMSPStatusTenant(t *testing.T, cfg *cloudcp.CPConfig, tenant *registry.Tenant) {
	t.Helper()
	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	defer reg.Close()
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create(%s): %v", tenant.ID, err)
	}
}
