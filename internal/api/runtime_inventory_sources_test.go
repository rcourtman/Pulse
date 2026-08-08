package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestRuntimeInventorySourcesProjectsOnlyBlockingWorkloadCoverage(t *testing.T) {
	sources := runtimeInventorySources([]Connection{
		{
			ID:       "pve:blocked",
			Type:     ConnectionTypePVE,
			Name:     "Blocked PVE",
			State:    ConnectionStateStale,
			Enabled:  true,
			Surfaces: []string{"backups", "containers", "storage", "vms"},
			Scope:    map[string]bool{"containers": false, "storage": true, "vms": true},
		},
		{
			ID:       "vmware:healthy",
			Type:     ConnectionTypeVMware,
			Name:     "Healthy vCenter",
			State:    ConnectionStateActive,
			Enabled:  true,
			Surfaces: []string{"vms"},
		},
		{
			ID:       "docker:disabled",
			Type:     ConnectionTypeDocker,
			Name:     "Disabled Docker",
			State:    ConnectionStateUnreachable,
			Enabled:  false,
			Surfaces: []string{"containers"},
		},
		{
			ID:       "pbs:blocked",
			Type:     ConnectionTypePBS,
			Name:     "Blocked PBS",
			State:    ConnectionStateUnreachable,
			Enabled:  true,
			Surfaces: []string{"backups"},
		},
	})

	want := []RuntimeInventorySource{{
		Type:     ConnectionTypePVE,
		Name:     "Blocked PVE",
		State:    ConnectionStateStale,
		Surfaces: []string{"vms"},
	}}
	if !reflect.DeepEqual(sources, want) {
		t.Fatalf("runtime inventory sources = %#v, want %#v", sources, want)
	}
}

func TestRuntimeInventorySourcesNormalizesCredentialFailureWithoutPublishingFleet(t *testing.T) {
	expiredAt := time.Now().UTC().Add(-time.Hour)
	sources := runtimeInventorySources([]Connection{
		{
			Type:     ConnectionTypeVMware,
			Name:     "vCenter",
			State:    ConnectionStateActive,
			Enabled:  true,
			Surfaces: []string{"vms", "vms", "storage"},
			Fleet: ConnectionFleetGovernance{
				CredentialHealth: &ConnectionFleetCredentialHealth{
					Status:    "expired",
					ExpiresAt: &expiredAt,
				},
			},
		},
	})

	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}
	if sources[0].State != ConnectionStateUnauthorized {
		t.Fatalf("state = %q, want normalized unauthorized", sources[0].State)
	}
	if !reflect.DeepEqual(sources[0].Surfaces, []string{"vms"}) {
		t.Fatalf("surfaces = %#v, want workload-only deduplicated coverage", sources[0].Surfaces)
	}
}

func TestRuntimeInventorySourceWireShapeIsAnExactWhitelist(t *testing.T) {
	payload, err := json.Marshal(RuntimeInventorySource{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]struct{}{
		"type":     {},
		"name":     {},
		"state":    {},
		"surfaces": {},
	}
	if len(fields) != len(want) {
		t.Fatalf("wire fields = %v, want exactly %v", fields, want)
	}
	for field := range fields {
		if _, allowed := want[field]; !allowed {
			t.Fatalf("monitoring projection grew unapproved field %q", field)
		}
	}
}

func TestRuntimeInventorySourcesOmitAdministrativeAndSensitiveFacts(t *testing.T) {
	now := time.Now().UTC()
	sources := runtimeInventorySources([]Connection{{
		ID:          "vmware:secret-internal-id",
		Type:        ConnectionTypeVMware,
		Name:        "Primary vCenter",
		Address:     "https://vcenter.corp.local:443",
		HostAliases: []string{"10.0.1.5", "vcenter-old.corp.local"},
		State:       ConnectionStateUnreachable,
		StateReason: `Get "https://vcenter.corp.local/sdk": dial tcp 10.0.1.5:443: i/o timeout`,
		Enabled:     true,
		Surfaces:    []string{"vms", "storage"},
		LastSeen:    &now,
		LastError: &ConnectionError{
			At:      now,
			Message: "credential token secret-value rejected",
		},
		AgentIdentity: &ConnectionAgentIdentity{
			Hostname: "collector-01",
			ReportIP: "10.0.1.9",
			OSName:   "Debian",
		},
		AgentVersion: "6.2.0",
		Fleet: ConnectionFleetGovernance{
			CommandPolicy: &ConnectionFleetCommandPolicy{Reason: "privileged policy detail"},
		},
	}})

	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}
	payload, err := json.Marshal(sources[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{
		"secret-internal-id",
		"vcenter.corp.local",
		"10.0.1.5",
		"secret-value",
		"collector-01",
		"10.0.1.9",
		"Debian",
		"6.2.0",
		"privileged policy detail",
		"storage",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("monitoring projection leaked %q: %s", forbidden, payload)
		}
	}
	if !strings.Contains(string(payload), "Primary vCenter") {
		t.Fatalf("projection omitted the operator-facing source label: %s", payload)
	}
}

func TestRuntimeInventorySourcesHandlerFailsClosedWhenUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/inventory-sources", nil)
	rec := httptest.NewRecorder()
	var handler *ConnectionsHandlers
	handler.HandleRuntimeInventorySources(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (%s)", rec.Code, rec.Body.String())
	}
}

func TestRuntimeInventorySourcesRouteAuthorizationKeepsAdminLedgerPrivate(t *testing.T) {
	cfg := &config.Config{
		DataPath:            t.TempDir(),
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	request := func(path, roles string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
		req.Header.Set(cfg.ProxyAuthUserHeader, "alice")
		req.Header.Set(cfg.ProxyAuthRoleHeader, roles)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	if rec := request("/api/runtime/inventory-sources", "viewer"); rec.Code != http.StatusOK {
		t.Fatalf("viewer runtime projection = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if rec := request("/api/connections", "viewer"); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer admin ledger = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if rec := request("/api/runtime/inventory-sources", "viewer|admin"); rec.Code != http.StatusOK {
		t.Fatalf("admin runtime projection = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if rec := request("/api/connections", "viewer|admin"); rec.Code != http.StatusOK {
		t.Fatalf("admin ledger = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
}

func TestRuntimeInventorySourcesRouteRequiresAuthenticationAndMonitoringScope(t *testing.T) {
	monitoringToken := "runtime-inventory-monitoring.12345678"
	settingsToken := "runtime-inventory-settings.12345678"
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, monitoringToken, []string{config.ScopeMonitoringRead}, nil),
		newTokenRecord(t, settingsToken, []string{config.ScopeSettingsRead}, nil),
	)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	request := func(rawToken string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/inventory-sources", nil)
		if rawToken != "" {
			req.Header.Set("X-API-Token", rawToken)
		}
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	if rec := request(""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
	if rec := request(settingsToken); rec.Code != http.StatusForbidden {
		t.Fatalf("settings-only token status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if rec := request(monitoringToken); rec.Code != http.StatusOK {
		t.Fatalf("monitoring token status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
}

func TestRuntimeInventorySourcesHandlerRejectsNonGET(t *testing.T) {
	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return nil },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return nil },
	)
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/inventory-sources", nil)
	rec := httptest.NewRecorder()
	handler.HandleRuntimeInventorySources(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (%s)", rec.Code, rec.Body.String())
	}
}
