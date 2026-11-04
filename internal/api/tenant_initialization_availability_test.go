package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Use the actual tenant guard and connections producer, not a mock manager.
// A pending initializer in one org cannot hold another org's saved ledger.
func TestTenantInitializationKeepsLiveConnectionsAvailable(t *testing.T) {
	dir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(dir)
	for _, id := range []string{"live", "starting"} {
		if err := mtp.SaveOrganization(&models.Organization{ID: id, DisplayName: id}); err != nil {
			t.Fatal(err)
		}
		p, err := mtp.GetPersistence(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.SaveNodesConfig([]config.PVEInstance{{Name: id + "-pve", Host: "https://" + id + ".invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: id + "-pin"}}, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir}
	manager := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(manager.Stop)
	live, err := manager.GetMonitor("live")
	if err != nil {
		t.Fatal(err)
	}
	saved := live.GetConfig().DeepCopy()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	manager.SetMonitorInitializer(func(m *monitoring.Monitor) {
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
		t.Fatal("tenant initializer did not enter its owned callback")
	}
	router := &Router{config: cfg, mtMonitor: manager}
	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return live.GetConfig() },
		func(context.Context) *config.ConfigPersistence { return live.GetConfigPersistence() },
		router.getTenantMonitor,
	)
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(handler.HandleList))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		guarded.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "live")))
	}))
	t.Cleanup(server.Close)
	type result struct {
		status int
		body   []byte
		err    error
	}
	response := make(chan result, 1)
	go func() {
		res, err := server.Client().Get(server.URL + "/api/connections")
		if err != nil {
			response <- result{err: err}
			return
		}
		body, readErr := io.ReadAll(res.Body)
		closeErr := res.Body.Close()
		if readErr == nil {
			readErr = closeErr
		}
		response <- result{status: res.StatusCode, body: body, err: readErr}
	}()
	var got result
	available := false
	select {
	case got = <-response:
		available = true
	case <-time.After(2 * time.Second):
		t.Error("live connections HTTP response waited for another tenant's initialization")
	}
	releaseOnce.Do(func() { close(release) })
	if !available {
		got = <-response
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	wire, err := json.Marshal(map[string]any{"availableWhileBlocked": available, "status": got.status, "body": string(got.body)})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TENANT_INITIALIZATION_WIRE %s", wire)
	if got.status != http.StatusOK || strings.Contains(string(got.body), "starting.invalid") {
		t.Fatalf("tenant response = %d: %s", got.status, got.body)
	}
	var list ConnectionsListResponse
	if err := json.Unmarshal(got.body, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Connections) != 1 || list.Connections[0].Name != "live-pve" || list.Connections[0].Address != "https://live.invalid:8006" || list.Connections[0].State != ConnectionStatePaused {
		t.Fatalf("saved ledger not preserved: %s", got.body)
	}
	if !reflect.DeepEqual(saved, live.GetConfig().DeepCopy()) {
		t.Error("another tenant's startup changed live configuration or TLS trust")
	}
}
