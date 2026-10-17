package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

// A full monitor reload (node auto-registration, settings that need one)
// replaces the default monitor and reattaches it with SetMonitor and
// SetMultiTenantMonitor, in that order, exactly as pkg/server does. The
// replacement must keep raising connection-degraded alerts from its own poll
// loop: a lister installed only on the startup monitor left the new one with
// none, so wedged connections stayed silent until restart.
func TestConnectionAlertsSurviveMonitorReload(t *testing.T) {
	setMockModeForTest(t, false)
	t.Setenv(truenas.FeatureTrueNAS, "true")
	setTrueNASFeatureForTest(t, true)

	nas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = w.Write([]byte(`{"hostname":"fixture-nas","version":"TrueNAS-SCALE-24.10.2","uptime_seconds":100}`))
		case "/api/v2.0/pool":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"fixture required inventory denied"}`))
		case "/api/v2.0/pool/dataset", "/api/v2.0/disk", "/api/v2.0/alert/list":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(nas.Close)

	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, PVEPollingInterval: 10 * time.Second}
	persistence := config.NewConfigPersistence(dataDir)
	tn := trueNASInstanceFromRawURL(t, "fixture-nas", nas.URL, true)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{tn}); err != nil {
		t.Fatal(err)
	}

	startup, err := monitoring.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(startup.Stop)
	router := NewRouter(cfg, startup, nil, nil, nil, "fixture")
	cleanupTestRouter(t, router)

	// The reload sequence from pkg/server refreshRouterAfterMonitorReload.
	reloaded := monitoring.NewMultiTenantMonitor(cfg, config.NewMultiTenantPersistence(dataDir), nil)
	t.Cleanup(reloaded.Stop)
	replacement, err := reloaded.GetMonitor("default")
	if err != nil {
		t.Fatal(err)
	}
	if replacement == startup {
		t.Fatal("reload fixture must produce a new default monitor")
	}
	router.SetMonitor(replacement)
	router.SetMultiTenantMonitor(reloaded)

	nasID := "truenas:" + tn.ID
	manager := replacement.GetAlertManager()
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		for _, alert := range manager.GetActiveAlerts() {
			if alert.Type == "connection-degraded" && alert.ResourceID == nasID {
				t.Logf("reloaded monitor raised %s for the wedged connection", alert.Level)
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the reloaded default monitor never evaluated connection-degraded alerts")
}
