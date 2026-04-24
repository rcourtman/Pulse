package cloudcp

import (
	"context"
	"strings"
	"testing"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

type fakeStorageDockerUsage struct {
	usage *cpDocker.DiskUsageSnapshot
	err   error
}

func (f fakeStorageDockerUsage) DiskUsage(context.Context) (*cpDocker.DiskUsageSnapshot, error) {
	return f.usage, f.err
}

func TestCheckStorageGuardrailsPassesWithAvailableStorageAndBoundedBuildCache(t *testing.T) {
	dir := t.TempDir()
	cfg := &CPConfig{
		StorageGuardrailsEnabled:        true,
		StorageRootPath:                 dir,
		StorageDataPath:                 dir,
		StorageDockerPath:               dir,
		StorageMinRootAvailableBytes:    1,
		StorageMinDataAvailableBytes:    1,
		StorageMinDockerAvailableBytes:  1,
		StorageMaxDockerBuildCacheBytes: 1024,
	}

	report, err := CheckStorageGuardrails(context.Background(), cfg, fakeStorageDockerUsage{
		usage: &cpDocker.DiskUsageSnapshot{
			BuildCache: cpDocker.DiskUsageClass{TotalSize: 512, Reclaimable: 128},
		},
	})
	if err != nil {
		t.Fatalf("CheckStorageGuardrails: %v", err)
	}
	if !report.OK {
		t.Fatalf("report.OK = false, failures = %v", report.Failures)
	}
}

func TestCheckStorageGuardrailsFailsWhenFilesystemFallsBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	cfg := &CPConfig{
		StorageGuardrailsEnabled:        true,
		StorageRootPath:                 dir,
		StorageDataPath:                 dir,
		StorageDockerPath:               dir,
		StorageMinRootAvailableBytes:    1 << 60,
		StorageMinDataAvailableBytes:    1,
		StorageMinDockerAvailableBytes:  1,
		StorageMaxDockerBuildCacheBytes: 1024,
	}

	report, err := CheckStorageGuardrails(context.Background(), cfg, fakeStorageDockerUsage{
		usage: &cpDocker.DiskUsageSnapshot{},
	})
	if err != nil {
		t.Fatalf("CheckStorageGuardrails: %v", err)
	}
	if report.OK {
		t.Fatal("report.OK = true, want false")
	}
	if got := strings.Join(report.Failures, "; "); !strings.Contains(got, "root path") || !strings.Contains(got, "below") {
		t.Fatalf("failures = %q, want root threshold failure", got)
	}
}

func TestCheckStorageGuardrailsFailsWhenDockerBuildCacheExceedsThreshold(t *testing.T) {
	dir := t.TempDir()
	cfg := &CPConfig{
		StorageGuardrailsEnabled:        true,
		StorageRootPath:                 dir,
		StorageDataPath:                 dir,
		StorageDockerPath:               dir,
		StorageMinRootAvailableBytes:    1,
		StorageMinDataAvailableBytes:    1,
		StorageMinDockerAvailableBytes:  1,
		StorageMaxDockerBuildCacheBytes: 1024,
	}

	report, err := CheckStorageGuardrails(context.Background(), cfg, fakeStorageDockerUsage{
		usage: &cpDocker.DiskUsageSnapshot{
			BuildCache: cpDocker.DiskUsageClass{TotalSize: 2048, Reclaimable: 2048},
		},
	})
	if err != nil {
		t.Fatalf("CheckStorageGuardrails: %v", err)
	}
	if report.OK {
		t.Fatal("report.OK = true, want false")
	}
	if got := strings.Join(report.Failures, "; "); !strings.Contains(got, "docker build cache") || !strings.Contains(got, "above") {
		t.Fatalf("failures = %q, want build cache threshold failure", got)
	}
}

func TestFindStaleProofTenantsUsesConfiguredMatchersAndAge(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	fresh := now.Add(-1 * time.Hour)
	tenants := []*registry.Tenant{
		{ID: "t-OLDPROOF", AccountID: "a_ga_canary_20260424", Email: "proof@example.com", State: registry.TenantStateActive, CreatedAt: old},
		{ID: "t-FRESH", AccountID: "a_ga_canary_20260424", Email: "proof@example.com", State: registry.TenantStateActive, CreatedAt: fresh},
		{ID: "t-CUSTOMER", AccountID: "a_customer", Email: "owner@customer.test", State: registry.TenantStateActive, CreatedAt: old},
	}

	stale := findStaleProofTenants(tenants, []string{"canary", "proof"}, 24*time.Hour, now)
	if len(stale) != 1 {
		t.Fatalf("len(stale) = %d, want 1 (%v)", len(stale), stale)
	}
	if stale[0].TenantID != "t-OLDPROOF" {
		t.Fatalf("stale[0].TenantID = %q, want t-OLDPROOF", stale[0].TenantID)
	}
}

func TestFindOrphanPaidHostedEntitlementsFlagsMissingTenants(t *testing.T) {
	issuedAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	entitlements := []*registry.HostedEntitlement{
		{ID: "paid:t-ACTIVE", Kind: registry.HostedEntitlementKindPaid, TenantID: "t-ACTIVE", IssuedAt: issuedAt},
		{ID: "paid:t-MISSING", Kind: registry.HostedEntitlementKindPaid, TenantID: "t-MISSING", IssuedAt: issuedAt.Add(-time.Hour)},
		{ID: "trial:req", Kind: registry.HostedEntitlementKindTrial, TenantID: "", IssuedAt: issuedAt},
	}

	orphaned := findOrphanPaidHostedEntitlements(entitlements, map[string]struct{}{
		"t-ACTIVE": {},
	})
	if len(orphaned) != 1 {
		t.Fatalf("len(orphaned) = %d, want 1 (%v)", len(orphaned), orphaned)
	}
	if orphaned[0].EntitlementID != "paid:t-MISSING" {
		t.Fatalf("orphaned[0].EntitlementID = %q, want paid:t-MISSING", orphaned[0].EntitlementID)
	}
}

func TestFindStaleProofAccountsUsesAccountAndStripeMatchers(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	fresh := now.Add(-1 * time.Hour)
	accounts := []*registry.Account{
		{ID: "a_rehearsal_old", Kind: registry.AccountKindMSP, DisplayName: "Production Rehearsal", CreatedAt: old},
		{ID: "a_customer", Kind: registry.AccountKindIndividual, DisplayName: "Customer", CreatedAt: old},
		{ID: "a_stripe_old", Kind: registry.AccountKindMSP, DisplayName: "Pulse", CreatedAt: old},
		{ID: "a_rehearsal_fresh", Kind: registry.AccountKindMSP, DisplayName: "Canary", CreatedAt: fresh},
	}
	stripeAccounts := []*registry.StripeAccount{
		{AccountID: "a_stripe_old", StripeCustomerID: "cus_msp_rehearsal_123", PlanVersion: "msp_starter"},
	}

	stale := findStaleProofAccounts(accounts, stripeAccounts, []string{"canary", "rehearsal"}, 24*time.Hour, now)
	if len(stale) != 2 {
		t.Fatalf("len(stale) = %d, want 2 (%v)", len(stale), stale)
	}
	if stale[0].AccountID != "a_rehearsal_old" || stale[1].AccountID != "a_stripe_old" {
		t.Fatalf("stale account order = %#v, want rehearsal then stripe", stale)
	}
}
