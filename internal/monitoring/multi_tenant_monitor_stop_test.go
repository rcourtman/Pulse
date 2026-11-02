package monitoring

import (
	"crypto/sha256"
	"io/fs"
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

// Stop retires the owner, not just the currently materialized tenant loops.
// In particular, a handler holding the old manager through a reload must not
// initialize a previously unused tenant or reload the default config into it.
func TestMultiTenantMonitorStopRejectsLazyRecreation(t *testing.T) {
	mtp, persistence := newTestTenantPersistence(t)
	if err := mtp.SaveOrganization(&models.Organization{ID: "tenant-a", DisplayName: "Tenant A"}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		DataPath:     persistence.GetConfigDir(),
		PVEInstances: []config.PVEInstance{{Name: "saved-primary", Host: "https://primary.invalid:8006", Disabled: true}},
		APITokens:    []config.APITokenRecord{{ID: "primary-token", Hash: "synthetic-primary-hash"}},
	}
	before := cfg.DeepCopy()
	snapshot := func() map[string][32]byte {
		result := make(map[string][32]byte)
		err := filepath.WalkDir(mtp.BaseDataDir(), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				result[path] = [32]byte{}
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = sha256.Sum256(data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	filesBefore := snapshot()
	mtm := NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	var initialized atomic.Int32
	mtm.SetMonitorInitializer(func(*Monitor) { initialized.Add(1) })
	mtm.Stop()
	mtm.Stop() // Repeated shutdown must not reopen admission.

	for _, orgID := range []string{"default", "tenant-a", "not-provisioned", " tenant-a "} {
		if monitor, err := mtm.GetMonitor(orgID); monitor != nil || err == nil {
			t.Errorf("retired manager admitted %q: monitor present=%v, err=%v", orgID, monitor != nil, err)
		}
		if monitor, ok := mtm.PeekMonitor(orgID); monitor != nil || ok {
			t.Errorf("retired manager exposed cached monitor for %q", orgID)
		}
	}
	if initialized.Load() != 0 {
		t.Errorf("retired manager initialized %d runtimes", initialized.Load())
	}
	if !reflect.DeepEqual(cfg.DeepCopy(), before) {
		t.Error("retired lookup changed the canonical config")
	}
	// A rejected lookup must not create stores/writers or change saved files.
	if !reflect.DeepEqual(snapshot(), filesBefore) {
		t.Error("retired lookup created or changed persisted runtime material")
	}

}

func TestMultiTenantMonitorStopFencesRetainedRuntimes(t *testing.T) {
	mtm := NewMultiTenantMonitor(&config.Config{}, nil, nil)
	// Retain a loop deliberately, without starting stores or provider work.
	// The global cancellation signal proves Stop has acquired the same lock
	// as admission; the per-tenant done signal still prevents cleanup.
	done := make(chan struct{})
	mtm.monitors["tenant-a"] = &Monitor{}
	mtm.monitors["tenant-b"] = &Monitor{}
	mtm.tenantDone["tenant-a"] = done
	mtm.tenantDone["tenant-b"] = done
	stopped := make(chan struct{})
	go func() {
		mtm.Stop()
		close(stopped)
	}()
	t.Cleanup(func() { close(done); <-stopped })
	select {
	case <-mtm.globalCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not close admission")
	}
	var readers sync.WaitGroup
	for range 32 {
		readers.Go(func() {
			for _, orgID := range []string{"tenant-a", "tenant-b", "default", "new-tenant"} {
				if monitor, err := mtm.GetMonitor(orgID); monitor != nil || err == nil {
					t.Errorf("lookup admitted %q after shutdown began", orgID)
				}
				if monitor, ok := mtm.PeekMonitor(orgID); monitor != nil || ok {
					t.Errorf("peek exposed %q after shutdown began", orgID)
				}
			}
		})
	}
	readers.Wait()
	mtm.mu.RLock()
	retained := len(mtm.monitors)
	mtm.mu.RUnlock()
	if retained != 2 {
		t.Errorf("shutdown lost runtime ownership before loop completion: retained=%d", retained)
	}
}

func TestMultiTenantMonitorStopRetainsFenceAfterIncompleteCleanup(t *testing.T) {
	previous := tenantMonitorShutdownTimeout
	tenantMonitorShutdownTimeout = time.Millisecond
	t.Cleanup(func() { tenantMonitorShutdownTimeout = previous })
	mtm := NewMultiTenantMonitor(&config.Config{}, nil, nil)
	done := make(chan struct{})
	mtm.monitors["tenant-a"] = &Monitor{}
	mtm.tenantDone["tenant-a"] = done
	mtm.Stop()
	if len(mtm.monitors) != 1 {
		t.Fatal("incomplete shutdown lost its runtime owner")
	}
	if monitor, ok := mtm.PeekMonitor("tenant-a"); monitor != nil || ok {
		t.Error("incomplete shutdown exposed its retained runtime")
	}
	mtm.FinishTenantDeletion("tenant-a")
	if monitor, err := mtm.GetMonitor("tenant-a"); monitor != nil || err == nil {
		t.Error("deletion cleanup reopened the retired manager")
	}
	close(done)
	mtm.Stop() // The admission fence must not prevent finishing owned cleanup.
	if len(mtm.monitors) != 0 {
		t.Error("completed loop was not cleaned up on repeated Stop")
	}
	if monitor, err := mtm.GetMonitor("default"); monitor != nil || err == nil {
		t.Error("completed cleanup reopened the retired manager")
	}
}
