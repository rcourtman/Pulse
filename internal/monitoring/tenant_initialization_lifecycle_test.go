package monitoring

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Admission owns construction as well as the loop. A bounded stop/deletion
// must retain its guard and data while initialization is still in flight.
func TestTenantInitializationRetainsLifecycleOwnership(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		name := "stop"
		if deleting {
			name = "deletion"
		}
		t.Run(name, func(t *testing.T) {
			mtp, p := newTestTenantPersistence(t)
			if err := mtp.SaveOrganization(&models.Organization{ID: "starting", DisplayName: "Starting"}); err != nil {
				t.Fatal(err)
			}
			tenant, err := mtp.GetPersistence("starting")
			if err != nil {
				t.Fatal(err)
			}
			saved := []config.PVEInstance{{Name: "saved", Host: "https://saved.invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: "saved-pin"}}
			if err := tenant.SaveNodesConfig(saved, nil, nil); err != nil {
				t.Fatal(err)
			}
			nodesBefore, err := os.ReadFile(filepath.Join(tenant.GetConfigDir(), "nodes.enc"))
			if err != nil {
				t.Fatal(err)
			}
			manager := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
			t.Cleanup(manager.Stop)
			entered := make(chan *Monitor, 1)
			release := make(chan struct{})
			var once sync.Once
			manager.SetMonitorInitializer(func(m *Monitor) { entered <- m; <-release })
			initialized := make(chan error, 1)
			go func() { _, err := manager.GetMonitor("starting"); initialized <- err }()
			t.Cleanup(func() { once.Do(func() { close(release) }); <-initialized })
			var pending *Monitor
			select {
			case pending = <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("initializer did not enter")
			}
			previous, previousDeletion := tenantMonitorShutdownTimeout, tenantDeletionShutdownTimeout
			tenantMonitorShutdownTimeout, tenantDeletionShutdownTimeout = 20*time.Millisecond, 20*time.Millisecond
			t.Cleanup(func() { tenantMonitorShutdownTimeout, tenantDeletionShutdownTimeout = previous, previousDeletion })
			shutdown := make(chan error, 1)
			go func() {
				if deleting {
					shutdown <- manager.BeginTenantDeletion("starting")
				} else {
					shutdown <- manager.stop()
				}
			}()
			var shutdownErr error
			select {
			case shutdownErr = <-shutdown:
			case <-time.After(2 * time.Second):
				once.Do(func() { close(release) })
				<-shutdown
				t.Fatal("lifecycle shutdown waited beyond its budget for the initializer")
			}
			if shutdownErr == nil {
				t.Fatal("in-flight constructor was reported as fully stopped")
			}
			manager.FinishTenantDeletion("starting")
			manager.mu.RLock()
			_, guarded := manager.tenantDeleting["starting"]
			done, owned := manager.tenantDone["starting"]
			manager.mu.RUnlock()
			if !owned || !guarded {
				t.Fatal("incomplete initialization lost its lifetime or deletion guard")
			}
			if m, err := manager.GetMonitor("starting"); m != nil || err == nil {
				t.Fatal("incomplete initialization reopened admission")
			}
			if !mtp.OrgExists("starting") {
				t.Fatal("incomplete initialization removed saved tenant")
			}
			once.Do(func() { close(release) })
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("released constructor did not complete")
			}
			if pending.getRuntimeContext() != nil {
				t.Error("cancelled constructor started a monitoring loop")
			}
			if m, ok := manager.PeekMonitor("starting"); m != nil || ok {
				t.Error("cancelled constructor was exposed as a live runtime")
			}
			manager.mu.RLock()
			retained := manager.monitors["starting"]
			manager.mu.RUnlock()
			if retained != pending {
				t.Fatal("finished cancelled constructor lost its cleanup owner")
			}
			if deleting {
				err = manager.BeginTenantDeletion("starting")
			} else {
				err = manager.stop()
			}
			if err != nil {
				t.Fatalf("completed constructor could not be cleaned up: %v", err)
			}
			manager.mu.RLock()
			_, owned = manager.tenantDone["starting"]
			_, retainedEntry := manager.monitors["starting"]
			manager.mu.RUnlock()
			if owned || retainedEntry {
				t.Error("completed cleanup retained a closed runtime")
			}
			nodesAfter, err := os.ReadFile(filepath.Join(tenant.GetConfigDir(), "nodes.enc"))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(nodesBefore, nodesAfter) {
				t.Error("lifecycle cleanup changed the saved encrypted inventory")
			}
			if deleting {
				manager.FinishTenantDeletion("starting")
				// The old constructor result has already returned; a distinct runtime
				// may now recover the same persisted inventory through normal admission.
				manager.SetMonitorInitializer(nil)
				recovered, err := manager.GetMonitor("starting")
				if err != nil {
					t.Fatal(err)
				}
				if recovered == pending || !reflect.DeepEqual(recovered.GetConfig().PVEInstances, saved) {
					t.Error("ordinary admission did not recover saved intent in a distinct runtime")
				}
			}
		})
	}
}

func TestTenantInitializationCoalescesConcurrentLookups(t *testing.T) {
	mtp, p := newTestTenantPersistence(t)
	if err := mtp.SaveOrganization(&models.Organization{ID: "starting", DisplayName: "Starting"}); err != nil {
		t.Fatal(err)
	}
	manager := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
	t.Cleanup(manager.Stop)
	var called atomic.Int32
	manager.SetMonitorInitializer(func(*Monitor) { called.Add(1) })
	const n = 24
	start := make(chan struct{})
	results := make(chan *Monitor, n)
	var readers sync.WaitGroup
	for range n {
		readers.Go(func() {
			<-start
			m, err := manager.GetMonitor("starting")
			if err != nil {
				t.Errorf("concurrent lookup: %v", err)
			}
			results <- m
		})
	}
	close(start)
	readers.Wait()
	close(results)
	var first *Monitor
	for m := range results {
		if m == nil {
			t.Fatal("lookup returned no monitor")
		}
		if first == nil {
			first = m
		}
		if m != first {
			t.Error("concurrent lookups opened multiple runtimes")
		}
	}
	if called.Load() != 1 {
		t.Errorf("constructor callbacks=%d, want 1", called.Load())
	}
}

func TestTenantInitializationErrorReleasesAdmission(t *testing.T) {
	mtp, p := newTestTenantPersistence(t)
	if err := mtp.SaveOrganization(&models.Organization{ID: "starting", DisplayName: "Starting"}); err != nil {
		t.Fatal(err)
	}
	tenant, err := mtp.GetPersistence("starting")
	if err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(tenant.GetConfigDir(), "api_tokens.json")
	if err := os.WriteFile(tokenPath, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
	t.Cleanup(manager.Stop)
	if m, err := manager.GetMonitor("starting"); m != nil || err == nil {
		t.Fatal("unreadable scoped configuration was admitted")
	}
	if len(manager.tenantDone) != 0 || len(manager.tenantCancel) != 0 || len(manager.monitors) != 0 {
		t.Fatal("failed constructor retained an unfinished admission")
	}
	if err := tenant.SaveAPITokens(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.GetMonitor("starting"); err != nil {
		t.Fatalf("corrected saved configuration could not be read: %v", err)
	}
}

func TestTenantInitializationDependencyUpdateIsNotLost(t *testing.T) {
	mtp, p := newTestTenantPersistence(t)
	if err := mtp.SaveOrganization(&models.Organization{ID: "starting", DisplayName: "Starting"}); err != nil {
		t.Fatal(err)
	}
	manager := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
	t.Cleanup(manager.Stop)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	manager.SetMonitorInitializer(func(*Monitor) { close(entered); <-release })
	initialized := make(chan error, 1)
	go func() { _, err := manager.GetMonitor("starting"); initialized <- err }()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		if err := <-initialized; err != nil {
			t.Errorf("initialization: %v", err)
		}
	})
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("initializer did not enter")
	}
	var applied atomic.Int32
	updated := make(chan struct{})
	go func() {
		manager.SetMonitorInitializer(func(*Monitor) { applied.Add(1) })
		close(updated)
	}()
	releaseOnce.Do(func() { close(release) })
	select {
	case <-updated:
	case <-time.After(5 * time.Second):
		t.Fatal("dependency update did not finish after initialization")
	}
	if applied.Load() != 1 {
		t.Error("dependency update missed or repeated the admitted runtime")
	}
}
