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
