package api

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRBACMetrics_ManagerGaugeTracksLifecycle(t *testing.T) {
	baseDir := t.TempDir()
	createOrgDir(t, baseDir, "org-a")
	createOrgDir(t, baseDir, "org-b")

	provider := NewTenantRBACProvider(baseDir)

	ensureRBACMetrics()
	startingValue := testutil.ToFloat64(rbacManagersActive)

	if _, err := provider.GetManager("org-a"); err != nil {
		t.Fatalf("GetManager(org-a) failed: %v", err)
	}
	if _, err := provider.GetManager("org-b"); err != nil {
		t.Fatalf("GetManager(org-b) failed: %v", err)
	}

	got := testutil.ToFloat64(rbacManagersActive)
	if got != startingValue+2 {
		t.Fatalf("active managers after 2 creations = %v, want %v", got, startingValue+2)
	}

	if err := provider.RemoveTenant("org-a"); err != nil {
		t.Fatalf("RemoveTenant(org-a) failed: %v", err)
	}

	got = testutil.ToFloat64(rbacManagersActive)
	if got != startingValue+1 {
		t.Fatalf("active managers after remove = %v, want %v", got, startingValue+1)
	}

	if err := provider.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	got = testutil.ToFloat64(rbacManagersActive)
	if got != 0 {
		t.Fatalf("active managers after close = %v, want 0", got)
	}
}

func TestRBACMetrics_RoleMutationRecording(t *testing.T) {
	ensureRBACMetrics()
	counter, err := rbacRoleMutations.GetMetricWithLabelValues("create")
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(create) failed: %v", err)
	}

	before := testutil.ToFloat64(counter)
	RecordRBACRoleMutation("create")
	after := testutil.ToFloat64(counter)

	if after != before+1 {
		t.Fatalf("create mutation counter delta = %v, want 1", after-before)
	}
}

func TestRBACMetrics_IntegrityCheckRecording(t *testing.T) {
	ensureRBACMetrics()
	counter, err := rbacIntegrityChecks.GetMetricWithLabelValues("healthy")
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(healthy) failed: %v", err)
	}

	before := testutil.ToFloat64(counter)
	RecordRBACIntegrityCheck("healthy")
	after := testutil.ToFloat64(counter)

	if after != before+1 {
		t.Fatalf("healthy integrity counter delta = %v, want 1", after-before)
	}
}
