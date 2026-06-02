package cloudcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	runtimeconfig "github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
)

func TestProviderMSPRecoveryDryRunPlansDegradedWorkspaces(t *testing.T) {
	cfg := testProviderMSPBackupConfig(t)
	now := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	reg := seedProviderMSPRecoveryTenants(t, cfg, now)
	reg.Close()

	report, err := recoverProviderMSPWorkspacesWithDependencies(context.Background(), cfg, ProviderMSPRecoveryOptions{
		AllDegraded: true,
		DryRun:      true,
	}, providerMSPRecoveryDependencies{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RecoverProviderMSPWorkspaces dry-run: %v", err)
	}
	if !report.OK || !report.DryRun {
		t.Fatalf("report status = ok %t dry-run %t", report.OK, report.DryRun)
	}
	if report.RecoverCount != 3 || report.SkippedCount != 2 {
		t.Fatalf("counts = recover %d skipped %d", report.RecoverCount, report.SkippedCount)
	}
	items := providerMSPRecoveryItemsByTenant(report.Items)
	for tenantID, wantReason := range map[string]string{
		"t-FAILED":    "workspace is failed",
		"t-STUCK":     "workspace is stuck in provisioning",
		"t-UNHEALTHY": "workspace health check is failing",
	} {
		item := items[tenantID]
		if item.Action != providerMSPRecoveryActionRecover || item.Reason != wantReason {
			t.Fatalf("%s item = %#v, want recover reason %q", tenantID, item, wantReason)
		}
	}
	if items["t-RECENT"].Action != providerMSPRecoveryActionSkip || !strings.Contains(items["t-RECENT"].Reason, "provisioning timeout") {
		t.Fatalf("recent provisioning item = %#v", items["t-RECENT"])
	}
	if items["t-HEALTHY"].Action != providerMSPRecoveryActionSkip || !strings.Contains(items["t-HEALTHY"].Reason, "healthy") {
		t.Fatalf("healthy item = %#v", items["t-HEALTHY"])
	}
}

func TestProviderMSPRecoveryReactivatesRecoveredWorkspace(t *testing.T) {
	cfg := testProviderMSPBackupConfig(t)
	now := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	reg := seedProviderMSPRecoveryTenants(t, cfg, now)
	reg.Close()
	writeProviderMSPRecoveryTenantData(t, cfg, "t-FAILED")

	var rolloutTenantID string
	report, err := recoverProviderMSPWorkspacesWithDependencies(context.Background(), cfg, ProviderMSPRecoveryOptions{
		TenantIDs: []string{"t-FAILED"},
		RunID:     "recovery-test",
	}, providerMSPRecoveryDependencies{
		Now: func() time.Time { return now },
		RolloutTenantRuntime: func(_ context.Context, _ *CPConfig, opts TenantRuntimeRolloutOptions) (*TenantRuntimeRolloutResult, error) {
			rolloutTenantID = opts.TenantID
			return &TenantRuntimeRolloutResult{
				TenantID:          opts.TenantID,
				ActiveContainerID: "container-recovered",
				ActiveImageRef:    "pulse:test",
				ActiveImageID:     "sha256:recovered",
				RestoredMissing:   true,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("RecoverProviderMSPWorkspaces: %v", err)
	}
	if !report.OK || report.RecoveredCount != 1 || report.ErrorCount != 0 {
		t.Fatalf("report = %#v", report)
	}
	if rolloutTenantID != "t-FAILED" {
		t.Fatalf("rollout tenant id = %q, want t-FAILED", rolloutTenantID)
	}
	item := report.Items[0]
	if !item.Recovered || item.ActiveContainerID != "container-recovered" || !item.RestoredMissing {
		t.Fatalf("recovery item = %#v", item)
	}

	reloaded := getProviderMSPRecoveryTenant(t, cfg, "t-FAILED")
	if reloaded.State != registry.TenantStateActive || !reloaded.HealthCheckOK || reloaded.LastHealthCheck == nil {
		t.Fatalf("reloaded tenant = %#v", reloaded)
	}
}

func TestProviderMSPRecoveryRefusesMissingTenantData(t *testing.T) {
	cfg := testProviderMSPBackupConfig(t)
	now := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	reg := seedProviderMSPRecoveryTenants(t, cfg, now)
	reg.Close()

	rolloutCalled := false
	report, err := recoverProviderMSPWorkspacesWithDependencies(context.Background(), cfg, ProviderMSPRecoveryOptions{
		TenantIDs: []string{"t-FAILED"},
	}, providerMSPRecoveryDependencies{
		Now: func() time.Time { return now },
		RolloutTenantRuntime: func(context.Context, *CPConfig, TenantRuntimeRolloutOptions) (*TenantRuntimeRolloutResult, error) {
			rolloutCalled = true
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected recovery to fail when tenant data is missing")
	}
	if rolloutCalled {
		t.Fatal("rollout should not be called without tenant data")
	}
	if report == nil || report.OK || report.ErrorCount != 1 {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Items[0].Error, "provider MSP tenant data dir unavailable") {
		t.Fatalf("item error = %q", report.Items[0].Error)
	}
}

func seedProviderMSPRecoveryTenants(t *testing.T, cfg *CPConfig, now time.Time) *registry.TenantRegistry {
	t.Helper()
	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	tenants := []*registry.Tenant{
		{ID: "t-FAILED", DisplayName: "Failed", State: registry.TenantStateFailed, CreatedAt: now.Add(-time.Hour), HealthCheckOK: false},
		{ID: "t-STUCK", DisplayName: "Stuck", State: registry.TenantStateProvisioning, CreatedAt: now.Add(-time.Hour), HealthCheckOK: false},
		{ID: "t-RECENT", DisplayName: "Recent", State: registry.TenantStateProvisioning, CreatedAt: now.Add(-time.Minute), HealthCheckOK: false},
		{ID: "t-UNHEALTHY", DisplayName: "Unhealthy", State: registry.TenantStateActive, CreatedAt: now.Add(-time.Hour), HealthCheckOK: false},
		{ID: "t-HEALTHY", DisplayName: "Healthy", State: registry.TenantStateActive, CreatedAt: now.Add(-time.Hour), HealthCheckOK: true},
	}
	for _, tenant := range tenants {
		if err := reg.Create(tenant); err != nil {
			t.Fatalf("Create(%s): %v", tenant.ID, err)
		}
	}
	return reg
}

func writeProviderMSPRecoveryTenantData(t *testing.T, cfg *CPConfig, tenantID string) {
	t.Helper()
	tenantDataDir := filepath.Join(cfg.TenantsDir(), tenantID)
	secretsDir := filepath.Join(tenantDataDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		t.Fatalf("create tenant secrets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "handoff.key"), []byte("handoff-key"), 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tenantDataDir, cloudauth.HandoffKeyFile), []byte("cloud-handoff-key"), 0o600); err != nil {
		t.Fatalf("write cloud handoff key: %v", err)
	}
	org := &models.Organization{
		ID:          tenantID,
		DisplayName: tenantID,
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
	}
	if err := runtimeconfig.NewMultiTenantPersistence(tenantDataDir).SaveOrganization(org); err != nil {
		t.Fatalf("save tenant organization: %v", err)
	}
}

func getProviderMSPRecoveryTenant(t *testing.T, cfg *CPConfig, tenantID string) *registry.Tenant {
	t.Helper()
	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	defer reg.Close()
	tenant, err := reg.Get(tenantID)
	if err != nil {
		t.Fatalf("Get(%s): %v", tenantID, err)
	}
	if tenant == nil {
		t.Fatalf("tenant %s missing", tenantID)
	}
	return tenant
}

func providerMSPRecoveryItemsByTenant(items []ProviderMSPRecoveryItem) map[string]ProviderMSPRecoveryItem {
	result := make(map[string]ProviderMSPRecoveryItem, len(items))
	for _, item := range items {
		result[item.TenantID] = item
	}
	return result
}
