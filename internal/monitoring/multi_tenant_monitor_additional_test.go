package monitoring

import (
	"testing"
	"time"
)

func TestMultiTenantMonitorRemoveTenant(t *testing.T) {
	monitor := &Monitor{}
	mtm := &MultiTenantMonitor{
		monitors: map[string]*Monitor{
			"org-1": monitor,
		},
	}

	mtm.RemoveTenant("org-1")
	if _, ok := mtm.monitors["org-1"]; ok {
		t.Fatalf("expected org-1 to be removed")
	}

	// Ensure removal of missing orgs is a no-op.
	mtm.RemoveTenant("missing")
}

func TestMultiTenantMonitorGetMonitor_MetricsIsolationByTenant(t *testing.T) {
	orgAMonitor := &Monitor{
		metricsHistory: NewMetricsHistory(32, time.Hour),
	}
	orgAMonitor.SetOrgID("org-a")

	orgBMonitor := &Monitor{
		metricsHistory: NewMetricsHistory(32, time.Hour),
	}
	orgBMonitor.SetOrgID("org-b")

	mtm := &MultiTenantMonitor{
		monitors: map[string]*Monitor{
			"org-a": orgAMonitor,
			"org-b": orgBMonitor,
		},
	}

	gotA, err := mtm.GetMonitor("org-a")
	if err != nil {
		t.Fatalf("unexpected error getting org-a monitor: %v", err)
	}
	gotB, err := mtm.GetMonitor("org-b")
	if err != nil {
		t.Fatalf("unexpected error getting org-b monitor: %v", err)
	}

	if gotA == gotB {
		t.Fatalf("expected distinct monitor instances per tenant")
	}
	if gotA.GetOrgID() != "org-a" {
		t.Fatalf("expected org-a monitor org id, got %q", gotA.GetOrgID())
	}
	if gotB.GetOrgID() != "org-b" {
		t.Fatalf("expected org-b monitor org id, got %q", gotB.GetOrgID())
	}

	now := time.Now()
	gotA.metricsHistory.AddGuestMetric("vm-1", "cpu", 73.5, now)

	aMetrics := gotA.GetGuestMetrics("vm-1", time.Hour)
	bMetrics := gotB.GetGuestMetrics("vm-1", time.Hour)
	if len(aMetrics["cpu"]) != 1 {
		t.Fatalf("expected org-a cpu series length 1, got %d", len(aMetrics["cpu"]))
	}
	if len(bMetrics["cpu"]) != 0 {
		t.Fatalf("expected org-b cpu series to remain empty, got %d points", len(bMetrics["cpu"]))
	}
}
