package health

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()

	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	t.Cleanup(func() {
		if err := reg.Close(); err != nil {
			t.Fatalf("close registry: %v", err)
		}
	})

	return reg
}

func createTenant(t *testing.T, reg *registry.TenantRegistry, tenant *registry.Tenant) {
	t.Helper()
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("create tenant %q: %v", tenant.ID, err)
	}
}

func TestStuckProvisioningCleanupTransitionsOnlyStaleProvisioningTenants(t *testing.T) {
	reg := newTestRegistry(t)
	cleanup := NewStuckProvisioningCleanup(reg)

	now := time.Now().UTC()

	createTenant(t, reg, &registry.Tenant{
		ID:        "tenant-stale",
		State:     registry.TenantStateProvisioning,
		CreatedAt: now.Add(-provisioningTimeout - 2*time.Minute),
	})
	createTenant(t, reg, &registry.Tenant{
		ID:        "tenant-recent",
		State:     registry.TenantStateProvisioning,
		CreatedAt: now.Add(-provisioningTimeout + 2*time.Minute),
	})
	createTenant(t, reg, &registry.Tenant{
		ID:        "tenant-active",
		State:     registry.TenantStateActive,
		CreatedAt: now.Add(-provisioningTimeout - 2*time.Minute),
	})

	cleanup.cleanup(context.Background())

	stale, err := reg.Get("tenant-stale")
	if err != nil {
		t.Fatalf("get stale tenant: %v", err)
	}
	if stale.State != registry.TenantStateFailed {
		t.Fatalf("expected stale tenant state %q, got %q", registry.TenantStateFailed, stale.State)
	}

	recent, err := reg.Get("tenant-recent")
	if err != nil {
		t.Fatalf("get recent tenant: %v", err)
	}
	if recent.State != registry.TenantStateProvisioning {
		t.Fatalf("expected recent tenant state %q, got %q", registry.TenantStateProvisioning, recent.State)
	}

	active, err := reg.Get("tenant-active")
	if err != nil {
		t.Fatalf("get active tenant: %v", err)
	}
	if active.State != registry.TenantStateActive {
		t.Fatalf("expected active tenant state %q, got %q", registry.TenantStateActive, active.State)
	}
}

func TestStuckProvisioningCleanupReturnsOnCanceledContext(t *testing.T) {
	reg := newTestRegistry(t)
	cleanup := NewStuckProvisioningCleanup(reg)

	createTenant(t, reg, &registry.Tenant{
		ID:        "tenant-stale",
		State:     registry.TenantStateProvisioning,
		CreatedAt: time.Now().UTC().Add(-provisioningTimeout - time.Minute),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cleanup.cleanup(ctx)

	tenant, err := reg.Get("tenant-stale")
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if tenant.State != registry.TenantStateProvisioning {
		t.Fatalf("expected tenant state %q, got %q", registry.TenantStateProvisioning, tenant.State)
	}
}

func TestNewMonitorAppliesDefaultConfig(t *testing.T) {
	monitor := NewMonitor(nil, nil, MonitorConfig{})

	if monitor.cfg.Interval != 60*time.Second {
		t.Fatalf("expected default interval 60s, got %s", monitor.cfg.Interval)
	}
	if monitor.cfg.FailThreshold != 3 {
		t.Fatalf("expected default fail threshold 3, got %d", monitor.cfg.FailThreshold)
	}
}

func TestMonitorRunStopsWhenContextCanceled(t *testing.T) {
	monitor := NewMonitor(nil, nil, MonitorConfig{Interval: 10 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected monitor run loop to stop after context cancellation")
	}
}
