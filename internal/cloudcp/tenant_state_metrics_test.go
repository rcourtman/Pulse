package cloudcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newMetricsTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()

	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func createMetricsTestTenant(t *testing.T, reg *registry.TenantRegistry, id string, state registry.TenantState) {
	t.Helper()

	if err := reg.Create(&registry.Tenant{
		ID:          id,
		Email:       id + "@example.com",
		DisplayName: id,
		State:       state,
	}); err != nil {
		t.Fatalf("Create(%s): %v", id, err)
	}
}

func tenantStateGaugeValue(state registry.TenantState) float64 {
	return testutil.ToFloat64(cpmetrics.TenantsByState.WithLabelValues(string(state)))
}

func TestUpdateTenantStateGauges_KnownAndUnexpectedStates(t *testing.T) {
	reg := newMetricsTestRegistry(t)

	unexpectedState := registry.TenantState("unexpected_state_label")

	createMetricsTestTenant(t, reg, "t-0000000001", registry.TenantStateActive)
	createMetricsTestTenant(t, reg, "t-0000000002", registry.TenantStateActive)
	createMetricsTestTenant(t, reg, "t-0000000003", registry.TenantStateFailed)
	createMetricsTestTenant(t, reg, "t-0000000004", unexpectedState)

	// Seed stale values to verify this function overwrites known labels.
	cpmetrics.TenantsByState.WithLabelValues(string(registry.TenantStateSuspended)).Set(99)
	cpmetrics.TenantsByState.WithLabelValues(string(registry.TenantStateDeleted)).Set(42)

	updateTenantStateGauges(reg)

	wantKnown := map[registry.TenantState]float64{
		registry.TenantStateProvisioning: 0,
		registry.TenantStateActive:       2,
		registry.TenantStateSuspended:    0,
		registry.TenantStateCanceled:     0,
		registry.TenantStateDeleting:     0,
		registry.TenantStateDeleted:      0,
		registry.TenantStateFailed:       1,
	}
	for state, want := range wantKnown {
		if got := tenantStateGaugeValue(state); got != want {
			t.Fatalf("state %q gauge = %v, want %v", state, got, want)
		}
	}

	if got := tenantStateGaugeValue(unexpectedState); got != 1 {
		t.Fatalf("unexpected state gauge = %v, want 1", got)
	}
}

func TestUpdateTenantStateGauges_RegistryErrorDoesNotMutateGauges(t *testing.T) {
	reg := newMetricsTestRegistry(t)

	cpmetrics.TenantsByState.WithLabelValues(string(registry.TenantStateActive)).Set(7)
	if err := reg.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	updateTenantStateGauges(reg)

	if got := tenantStateGaugeValue(registry.TenantStateActive); got != 7 {
		t.Fatalf("active gauge after error = %v, want 7", got)
	}
}

func TestRunTenantStateMetrics_PrimesOnStartupBeforeExit(t *testing.T) {
	reg := newMetricsTestRegistry(t)

	createMetricsTestTenant(t, reg, "t-0000000011", registry.TenantStateActive)
	createMetricsTestTenant(t, reg, "t-0000000012", registry.TenantStateDeleted)

	cpmetrics.TenantsByState.WithLabelValues(string(registry.TenantStateActive)).Set(0)
	cpmetrics.TenantsByState.WithLabelValues(string(registry.TenantStateDeleted)).Set(0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runTenantStateMetrics(ctx, reg)

	checks := map[registry.TenantState]float64{
		registry.TenantStateActive:  1,
		registry.TenantStateDeleted: 1,
	}
	for state, want := range checks {
		if got := tenantStateGaugeValue(state); got != want {
			t.Fatalf("after runTenantStateMetrics, state %s gauge = %v, want %v", fmt.Sprint(state), got, want)
		}
	}
}
