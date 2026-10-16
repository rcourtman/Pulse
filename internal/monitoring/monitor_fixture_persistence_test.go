package monitoring

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A fixture must not restore or overwrite the suite's ambient alert history,
// nor leave alerts for the next fixture. Keep normal persistence/restore enabled:
// live startup must still retain an unobserved host's saved alert.
func TestNewTestMonitorIsolatesAlertPersistence(t *testing.T) {
	sharedDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", sharedDir)
	raiseOffline := func(t *testing.T, manager *alerts.Manager, id string) {
		t.Helper()
		cfg := manager.GetConfig()
		cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
		manager.UpdateConfig(cfg)
		host := models.DockerHost{ID: id, DisplayName: id}
		for i := 0; i < 3; i++ {
			manager.HandleDockerHostOffline(host)
		}
		if active := manager.GetActiveAlerts(); len(active) != 1 || active[0].ResourceID != "docker:"+id {
			t.Fatalf("offline fixture did not open its own alert: %+v", active)
		}
		if err := manager.SaveActiveAlerts(); err != nil {
			t.Fatal(err)
		}
	}
	seed := alerts.NewManagerWithDataDir(sharedDir)
	t.Cleanup(seed.Stop)
	raiseOffline(t, seed, "ambient-persisted-host")
	seed.Stop()

	// Control: the default constructor must still restore ambient live history.
	restored := alerts.NewManager()
	t.Cleanup(restored.Stop)
	if active := restored.GetActiveAlerts(); len(active) != 1 || active[0].ResourceID != "docker:ambient-persisted-host" {
		t.Fatalf("default manager lost persisted live alert: %+v", active)
	}
	restored.Stop()
	sharedPath := filepath.Join(sharedDir, "alerts", "active-alerts.json")
	before, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			m := newTestMonitor(t)
			if active := m.alertManager.GetActiveAlerts(); len(active) != 0 {
				t.Fatalf("new fixture restored another fixture's alerts: %+v", active)
			}
			raiseOffline(t, m.alertManager, "local-"+name)
		}) // Stop flushes the first fixture before the second is constructed.
	}
	if after, err := os.ReadFile(sharedPath); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("fixture changed ambient alert persistence: read error %v", err)
	}
}
