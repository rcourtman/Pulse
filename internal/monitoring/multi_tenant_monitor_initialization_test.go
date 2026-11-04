package monitoring

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A slow constructor or dependency callback for one organization must not
// monopolize admission to tenants that are already monitoring their estate.
func TestTenantInitializationDoesNotBlockLiveReads(t *testing.T) {
	mtp, p := newTestTenantPersistence(t)
	for _, id := range []string{"live", "starting"} {
		if err := mtp.SaveOrganization(&models.Organization{ID: id, DisplayName: id}); err != nil {
			t.Fatal(err)
		}
	}
	manager := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
	t.Cleanup(manager.Stop)
	live, err := manager.GetMonitor("live")
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	manager.SetMonitorInitializer(func(m *Monitor) {
		if m.GetOrgID() == "starting" {
			close(entered)
			<-release
		}
	})
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
		t.Fatal("tenant constructor did not enter the owned callback")
	}
	read := make(chan struct{})
	go func() {
		defer close(read)
		if got, err := manager.GetMonitor("live"); err != nil || got != live {
			t.Errorf("live lookup lost its existing runtime: %v", err)
		}
		if got, ok := manager.PeekMonitor("live"); !ok || got != live {
			t.Error("live peek lost its existing runtime")
		}
		if m, ok := manager.PeekMonitor("starting"); ok || m != nil {
			t.Error("unfinished runtime was exposed to live readers")
		}
		manager.ForEachMonitor(func(m *Monitor) {
			if m.GetOrgID() == "starting" {
				select {
				case <-release: // The original blocked reader resumes after release.
				default:
					t.Error("unfinished runtime was exposed to live readers")
				}
			}
		})
	}()
	select {
	case <-read:
	case <-time.After(2 * time.Second):
		t.Error("initializing another tenant blocked live monitor reads")
	}
	// Always release and join fixture work, including on the old lock scope.
	releaseOnce.Do(func() { close(release) })
	<-read
}
