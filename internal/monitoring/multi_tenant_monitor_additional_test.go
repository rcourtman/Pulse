package monitoring

import "testing"

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
