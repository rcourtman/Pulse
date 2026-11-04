package monitoring

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// An incomplete drain is not a successful reload. Replacing the manager here
// loses ownership of its retained loops and can open the same saved state twice.
// These controls use completion channels only: no provider or database is run.
func TestReloadRetainsIncompleteRuntimeOwnership(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	cfg, err := config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	persistence := config.NewMultiTenantPersistence(cfg.DataPath)
	p, err := persistence.GetPersistence("default")
	if err != nil {
		t.Fatal(err)
	}
	saved := []config.PVEInstance{{Name: "kept", Host: "https://kept.invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: "saved-pin", TokenName: "pulse@pve!monitor", TokenValue: "synthetic-fixture-token"}}
	if err := p.SaveNodesConfig(saved, nil, nil); err != nil {
		t.Fatal(err)
	}
	before := cfg.DeepCopy()
	rm, err := NewReloadableMonitor(cfg, persistence, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rm.Start(ctx)
	retired := rm.GetMultiTenantMonitor()
	previous := tenantMonitorShutdownTimeout
	tenantMonitorShutdownTimeout = time.Millisecond
	release := make(chan struct{})
	completed := make(chan struct{})
	close(completed)
	// Nil monitors isolate the manager's loop ownership from store cleanup.
	for _, orgID := range []string{"stalled-a", "stalled-b"} {
		retired.monitors[orgID] = nil
		retired.tenantDone[orgID] = release
	}
	retired.monitors["completed"] = nil
	retired.tenantDone["completed"] = completed
	t.Cleanup(func() {
		close(release)
		retired.Stop()
		rm.Stop()
		cancel()
		tenantMonitorShutdownTimeout = previous
	})

	err = rm.Reload()
	if err == nil {
		t.Error("reload reported success while two tenant loops still owned their runtime")
	} else {
		for _, orgID := range []string{"stalled-a", "stalled-b"} {
			if !strings.Contains(err.Error(), orgID) {
				t.Errorf("shutdown did not report retained owner %q: %v", orgID, err)
			}
		}
	}
	if rm.GetMultiTenantMonitor() != retired {
		t.Error("reload discarded the incomplete manager")
	}
	if !reflect.DeepEqual(cfg.DeepCopy(), before) {
		t.Error("incomplete reload published the replacement configuration")
	}
	if _, exists := retired.monitors["completed"]; exists {
		t.Error("a failed tenant prevented independent completed cleanup")
	}
	if len(retired.monitors) != 2 {
		t.Errorf("retained %d owners, want exactly the two incomplete loops", len(retired.monitors))
	}
	if monitor, err := retired.GetMonitor("default"); monitor != nil || err == nil {
		t.Error("incomplete manager reopened runtime admission")
	}
}
