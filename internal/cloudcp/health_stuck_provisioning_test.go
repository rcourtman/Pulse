package cloudcp

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestInspectStuckProvisioningUsesCleanupThreshold(t *testing.T) {
	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	defer reg.Close()

	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	for _, tenant := range []*registry.Tenant{
		{
			ID:        "t-STUCK",
			State:     registry.TenantStateProvisioning,
			CreatedAt: now.Add(-provisioningTimeout - time.Second),
		},
		{
			ID:        "t-RECENT",
			State:     registry.TenantStateProvisioning,
			CreatedAt: now.Add(-provisioningTimeout + time.Second),
		},
		{
			ID:            "t-UNHEALTHY",
			State:         registry.TenantStateActive,
			CreatedAt:     now.Add(-time.Hour),
			HealthCheckOK: false,
		},
	} {
		if err := reg.Create(tenant); err != nil {
			t.Fatalf("Create(%s): %v", tenant.ID, err)
		}
	}

	report, err := InspectStuckProvisioning(reg, now)
	if err != nil {
		t.Fatalf("InspectStuckProvisioning: %v", err)
	}
	if report == nil {
		t.Fatal("report = nil")
	}
	if report.Timeout != provisioningTimeout {
		t.Fatalf("Timeout = %s, want %s", report.Timeout, provisioningTimeout)
	}
	if report.Count != 1 {
		t.Fatalf("Count = %d, want 1", report.Count)
	}
	if len(report.TenantIDs) != 1 || report.TenantIDs[0] != "t-STUCK" {
		t.Fatalf("TenantIDs = %#v, want [t-STUCK]", report.TenantIDs)
	}
}
