package configapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Exercise the existing HTTP producer, real reload handshake and a genuinely
// admitted monitor callback. The callback is released and both managers are
// joined even when the old implementation incorrectly reports success.
func TestNodeRemovalKeepsIncompleteReloadUnavailable(t *testing.T) {
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
	instances := []config.PVEInstance{
		{Name: "remove-me", Host: "https://removed.invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: "removed-pin"},
		{Name: "keep-me", Host: "https://kept.invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: "kept-pin"},
	}
	if err := p.SaveNodesConfig(instances, nil, nil); err != nil {
		t.Fatal(err)
	}
	rm, err := monitoring.NewReloadableMonitor(cfg, persistence, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rm.Start(ctx)
	retired := rm.GetMultiTenantMonitor()
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	retired.SetMonitorInitializer(func(m *monitoring.Monitor) {
		m.SetConnectionsSnapshotLister(func() []alerts.ConnectionSnapshot {
			enterOnce.Do(func() { close(entered) })
			<-release
			return nil
		})
	})
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		retired.Stop()
		rm.Stop()
		cancel()
	})
	monitor := rm.GetMonitor()
	if monitor == nil {
		t.Fatal("default monitor unavailable")
	}
	// Avoid the handler constructor's unrelated periodic token cleanup worker.
	h := &ConfigHandlers{
		mtPersistence: persistence, mtMonitor: retired, defaultConfig: cfg,
		defaultMonitor: monitor, defaultPersistence: p, runtime: defaultRuntimeDependencies(),
	}
	rebind := func() {
		h.SetMonitor(rm.GetMonitor())
		h.SetMultiTenantMonitor(rm.GetMultiTenantMonitor())
		h.SetConfig(rm.GetConfig())
	}
	h.reloadFunc = func() error {
		if err := rm.Reload(); err != nil {
			return err
		}
		rebind()
		return nil
	}
	select {
	case <-entered:
	case <-time.After(20 * time.Second):
		t.Fatal("connection check did not enter its owned callback")
	}
	server := httptest.NewServer(http.HandlerFunc(h.HandleDeleteNode))
	t.Cleanup(server.Close)
	request, err := http.NewRequest(http.MethodDelete, server.URL+"/api/config/nodes/pve:remove-me", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read/close removal response: %v / %v", readErr, closeErr)
	}
	unchangedOwner := rm.GetMultiTenantMonitor() == retired
	wire, err := json.Marshal(map[string]any{
		"phase": "incomplete-removal", "status": response.StatusCode, "body": string(body),
		"retainedOwner": unchangedOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RELOAD_WIRE %s", wire)
	if response.StatusCode != http.StatusInternalServerError || string(body) != "Configuration saved but failed to apply changes\n" {
		t.Errorf("incomplete removal returned %d: %s", response.StatusCode, body)
	}
	if !unchangedOwner {
		t.Error("node removal replaced a runtime whose accepted check was still running")
	}
	saved, err := p.LoadNodesConfig()
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || !reflect.DeepEqual(saved.PVEInstances, instances[1:]) {
		t.Fatal("saved removal lost the remaining endpoint, pause setting or TLS trust")
	}
	if m, err := retired.GetMonitor("default"); m != nil || err == nil {
		t.Error("incomplete runtime reopened admission")
	}

	// Complete the independently owned work, then use the ordinary reload path
	// to recover the saved intent. No second delete or customer restart is used.
	releaseOnce.Do(func() { close(release) })
	if err := rm.Reload(); err != nil {
		t.Fatal(err)
	}
	rebind()
	if rm.GetMultiTenantMonitor() == retired || rm.GetConfig() != cfg {
		t.Fatal("completed drain did not recover through a distinct owner and canonical config")
	}
	rec := httptest.NewRecorder()
	h.HandleGetNodes(rec, httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil))
	var nodes []NodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &nodes); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(nodes) != 1 || nodes[0].Name != "keep-me" || nodes[0].Host != instances[1].Host {
		t.Fatalf("recovered inventory = %d %s", rec.Code, rec.Body.String())
	}
	wire, err = json.Marshal(map[string]any{"phase": "recovered", "status": rec.Code, "body": rec.Body.String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RELOAD_WIRE %s", wire)
}
