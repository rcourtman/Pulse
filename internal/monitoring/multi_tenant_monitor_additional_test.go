package monitoring

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestMultiTenantMonitorRemoveTenantCancelsTenantRuntime(t *testing.T) {
	mtp, _ := newTestTenantPersistence(t)
	baseCfg := &config.Config{DataPath: t.TempDir()}
	mtm := NewMultiTenantMonitor(baseCfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "org-stop",
		DisplayName: "Org Stop",
	}); err != nil {
		t.Fatalf("SaveOrganization(org-stop) error = %v", err)
	}

	monitor, err := mtm.GetMonitor("org-stop")
	if err != nil {
		t.Fatalf("GetMonitor(org-stop) error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for monitor.runtimeCtx == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if monitor.runtimeCtx == nil {
		t.Fatal("expected tenant monitor runtime context to be initialized")
	}

	runtimeDone := monitor.runtimeCtx.Done()
	select {
	case <-runtimeDone:
		t.Fatal("expected tenant runtime context to be active before removal")
	default:
	}

	mtm.RemoveTenant("org-stop")

	select {
	case <-runtimeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected RemoveTenant to cancel the tenant runtime context")
	}

	if _, ok := mtm.monitors["org-stop"]; ok {
		t.Fatal("expected org-stop to be removed from monitor cache")
	}
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

func TestMultiTenantMonitorGetMonitorRejectsEmptyOrgID(t *testing.T) {
	mtm := &MultiTenantMonitor{}
	if _, err := mtm.GetMonitor("   "); err == nil {
		t.Fatal("expected error for empty org ID")
	}
}

func TestMultiTenantMonitorGetMonitorRejectsUnknownTenant(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	baseCfg := &config.Config{DataPath: baseDir}
	mtm := NewMultiTenantMonitor(baseCfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	if _, err := mtm.GetMonitor("org-missing"); err == nil {
		t.Fatal("expected error for unknown org")
	}

	orgDir := filepath.Join(baseDir, "orgs", "org-missing")
	if _, err := os.Stat(orgDir); !os.IsNotExist(err) {
		t.Fatalf("expected unknown org monitor lookup to avoid creating %s, stat err = %v", orgDir, err)
	}
}

func TestMultiTenantMonitorPeekMonitorTrimsOrgID(t *testing.T) {
	expected := &Monitor{}
	mtm := &MultiTenantMonitor{
		monitors: map[string]*Monitor{
			"org-a": expected,
		},
	}

	got, ok := mtm.PeekMonitor("  org-a  ")
	if !ok {
		t.Fatal("expected trimmed org ID lookup to succeed")
	}
	if got != expected {
		t.Fatalf("expected same monitor pointer, got %p want %p", got, expected)
	}
}

func TestMultiTenantMonitorSetMonitorInitializerAppliesToExisting(t *testing.T) {
	first := &Monitor{}
	second := &Monitor{}
	mtm := &MultiTenantMonitor{
		monitors: map[string]*Monitor{
			"org-1": first,
			"org-2": second,
		},
	}

	var called atomic.Int32
	mtm.SetMonitorInitializer(func(m *Monitor) {
		if m != nil {
			called.Add(1)
		}
	})

	if called.Load() != 2 {
		t.Fatalf("expected initializer to run for existing monitors, got %d", called.Load())
	}
}

func TestMultiTenantMonitorSetMonitorInitializerAppliesToNewMonitor(t *testing.T) {
	mtp, _ := newTestTenantPersistence(t)
	baseCfg := &config.Config{DataPath: t.TempDir()}
	mtm := NewMultiTenantMonitor(baseCfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	var called atomic.Int32
	mtm.SetMonitorInitializer(func(m *Monitor) {
		if m != nil {
			called.Add(1)
		}
	})

	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "org-init",
		DisplayName: "Org Init",
	}); err != nil {
		t.Fatalf("SaveOrganization(org-init) error = %v", err)
	}

	if _, err := mtm.GetMonitor("org-init"); err != nil {
		t.Fatalf("GetMonitor(org-init) error = %v", err)
	}
	if called.Load() != 1 {
		t.Fatalf("expected initializer to run for new monitor, got %d", called.Load())
	}
}
