package monitoring

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func configureAlertShutdownFixture(t *testing.T) {
	t.Helper()
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("HOME", t.TempDir())
	t.Setenv(mockKeepRealPollingEnv, "false")
	t.Setenv("PULSE_MOCK_TRENDS_SEED_DURATION", "5m")
	t.Setenv("PULSE_MOCK_TRENDS_SAMPLE_INTERVAL", "1m")
	t.Setenv("PULSE_MOCK_SEED_METRICS_STORE", "false")
	previousEnabled, previousConfig := mock.IsMockEnabled(), mock.GetConfig()
	t.Cleanup(func() {
		if err := mock.SetEnabled(false); err != nil {
			t.Error(err)
		}
		mock.SetMockConfig(previousConfig)
		if err := mock.SetEnabled(previousEnabled); err != nil {
			t.Error(err)
		}
	})
	if err := mock.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	fixture := mock.DefaultConfig
	fixture.NodeCount = 1
	fixture.VMsPerNode, fixture.LXCsPerNode = 0, 0
	fixture.DockerHostCount, fixture.GenericHostCount, fixture.K8sClusterCount = 0, 0, 0
	fixture.RandomMetrics = false
	mock.SetMockConfig(fixture)
	if err := mock.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
}

// A synchronous lifecycle consumer holds an actual mock alert pass in flight.
// No sleeps choose the overlap; the channel proves the pass has reached the
// writer boundary before cancellation or deletion starts.
func blockFirstShutdownAlert(t *testing.T, monitor *Monitor) (<-chan struct{}, func()) {
	t.Helper()
	manager := monitor.GetAlertManager()
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationPending
	cfg.TimeThresholds = map[string]int{}
	cfg.MetricTimeThresholds = nil
	cfg.NodeDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 1, Clear: 0.5}
	manager.UpdateConfig(cfg)
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	unsubscribe := manager.SubscribeLifecycleCallback(func(alerts.LifecycleEvent) {
		enterOnce.Do(func() {
			close(entered)
			<-release
		})
	})
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unsubscribe)
	t.Cleanup(unblock)
	return entered, unblock
}

func TestMonitorStartJoinsMockAlertChecksBeforeReturning(t *testing.T) {
	configureAlertShutdownFixture(t)
	monitor, err := New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	entered, unblock := blockFirstShutdownAlert(t, monitor)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Start(ctx, nil)
	}()
	t.Cleanup(func() {
		cancel()
		unblock()
		select {
		case <-done:
			monitor.Stop()
		case <-time.After(5 * time.Second):
			t.Error("monitor did not finish after releasing the alert pass")
		}
	})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("mock pass did not reach the lifecycle writer")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("monitor reported completion while its alert pass could still write")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor failed to join its completed alert pass")
	}
	if len(monitor.GetAlertManager().GetActiveAlerts()) == 0 {
		t.Fatal("shutdown silently dropped the admitted alert pass")
	}
}

func TestMonitorStartJoinsConnectionAlertChecksBeforeReturning(t *testing.T) {
	configureAlertShutdownFixture(t)
	monitor, err := New(&config.Config{DataPath: t.TempDir(), PVEPollingInterval: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	monitor.SetConnectionsSnapshotLister(func() []alerts.ConnectionSnapshot {
		enterOnce.Do(func() { close(entered) })
		<-release
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Start(ctx, nil)
	}()
	t.Cleanup(func() {
		cancel()
		unblock()
		select {
		case <-done:
			monitor.Stop()
		case <-time.After(5 * time.Second):
			t.Error("monitor did not finish after releasing the connection lister")
		}
	})
	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		t.Fatal("connection check did not reach its registered lister")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("monitor reported completion while a connection alert check remained in flight")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor failed to join its completed connection check")
	}
}

func TestTenantDeletionRetainsBlockedAlertPassThenRemovesCompletedRuntime(t *testing.T) {
	configureAlertShutdownFixture(t)
	previousTimeout := tenantDeletionShutdownTimeout
	tenantDeletionShutdownTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tenantDeletionShutdownTimeout = previousTimeout })
	root := t.TempDir()
	persistence := config.NewMultiTenantPersistence(root)
	if err := persistence.SaveOrganization(&models.Organization{ID: "alert-owner", DisplayName: "Alert owner"}); err != nil {
		t.Fatal(err)
	}
	manager := NewMultiTenantMonitor(&config.Config{DataPath: root}, persistence, nil)
	var entered <-chan struct{}
	var unblock func()
	manager.SetMonitorInitializer(func(monitor *Monitor) {
		entered, unblock = blockFirstShutdownAlert(t, monitor)
	})
	monitor, err := manager.GetMonitor("alert-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { unblock(); manager.Stop() })
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("tenant mock pass did not reach the lifecycle writer")
	}
	// The deletion budget still refuses a live writer. A timeout must retain
	// its original owner and organization bytes, not close stores underneath it.
	if err := manager.BeginTenantDeletion("alert-owner"); err == nil {
		t.Fatal("deletion accepted an unfinished alert pass")
	}
	if got, ok := manager.PeekMonitor("alert-owner"); !ok || got != monitor {
		t.Fatal("blocked alert writer lost its runtime owner")
	}
	manager.FinishTenantDeletion("alert-owner")
	if _, err := manager.GetMonitor("alert-owner"); err == nil {
		t.Fatal("deferred cleanup released the unfinished writer's guard")
	}
	orgPath := filepath.Join(root, "orgs", "alert-owner")
	if _, err := os.Stat(filepath.Join(orgPath, "org.json")); err != nil {
		t.Fatalf("blocked runtime lost its organization data: %v", err)
	}
	unblock()
	if err := manager.BeginTenantDeletion("alert-owner"); err != nil {
		t.Fatalf("completed alert pass prevented safe removal: %v", err)
	}
	if err := persistence.DeleteOrganization("alert-owner"); err != nil {
		t.Fatal(err)
	}
	manager.FinishTenantDeletion("alert-owner")
	if _, ok := manager.PeekMonitor("alert-owner"); ok {
		t.Fatal("completed tenant runtime was retained after deletion")
	}
	if _, err := manager.GetMonitor("alert-owner"); err == nil {
		t.Fatal("deleted organization can recreate a runtime")
	}
	if _, err := os.Stat(orgPath); !os.IsNotExist(err) {
		t.Fatalf("safe deletion left an organization directory: %v", err)
	}
}
