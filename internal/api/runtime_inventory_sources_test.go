package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
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

func TestRuntimeInventorySourcesProjectsDegradedCompletenessWithoutRawErrors(t *testing.T) {
	sources := runtimeInventorySources([]Connection{{
		Type:     ConnectionTypeVMware,
		Name:     "Production vCenter",
		State:    ConnectionStateActive,
		Enabled:  true,
		Surfaces: []string{"vms"},
		inventoryCompleteness: &RuntimeInventoryCompleteness{
			State:      "degraded",
			IssueCount: 3,
			Issues: []RuntimeInventoryCompletenessIssue{{
				Stage:       "tags",
				Category:    "permission",
				Occurrences: 3,
			}},
		},
	}})

	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}
	if sources[0].State != ConnectionStateActive || sources[0].Completeness == nil {
		t.Fatalf("source = %#v, want active source with degraded completeness", sources[0])
	}
	if sources[0].Completeness.IssueCount != 3 || sources[0].Completeness.Issues[0].Stage != "tags" {
		t.Fatalf("completeness = %#v", sources[0].Completeness)
	}
}

func TestRuntimeVMwareInventoryCompletenessDropsMessagesAndEntityIdentity(t *testing.T) {
	completeness := runtimeVMwareInventoryCompleteness(&monitoring.VMwareConnectionObservedSummary{
		Degraded:   true,
		IssueCount: 1,
		Issues: []monitoring.VMwareConnectionObservedIssue{{
			Stage:       "guest-details",
			Category:    "permission",
			Message:     "vm-42 at https://vcenter.secret.local/sdk denied",
			Occurrences: 1,
		}},
	})
	if completeness == nil {
		t.Fatal("expected degraded completeness")
	}
	payload, err := json.Marshal(completeness)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"vm-42", "vcenter.secret.local", "denied"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("completeness leaked %q: %s", forbidden, payload)
		}
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

// countingSupplementalRecords counts how often the monitor reads it: one read
// per provider for every store pass.
type countingSupplementalRecords struct {
	reads atomic.Int32
}

func (c *countingSupplementalRecords) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	c.reads.Add(1)
	return nil
}

func newWiringTestMonitor(t *testing.T) *monitoring.Monitor {
	t.Helper()
	monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })
	return monitor
}

// Wiring a monitor attaches its store and every supplemental provider. Each
// separate wiring step published the whole estate, so a router with two
// platform providers ran three full store passes (and the first two read an
// incomplete provider set) before the listener opened.
func TestConfigureMonitorDependenciesPublishesTheEstateOnce(t *testing.T) {
	trueNAS := &countingSupplementalRecords{}
	vmware := &countingSupplementalRecords{}
	router := &Router{
		monitorResourceAdapter:  unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
		monitorSupplementalRecords: map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
			unifiedresources.SourceTrueNAS: trueNAS,
			unifiedresources.SourceVMware:  vmware,
		},
	}

	monitor := newWiringTestMonitor(t)
	router.configureMonitorDependencies(monitor)

	if got := trueNAS.reads.Load(); got != 1 {
		t.Fatalf("TrueNAS provider read %d times while wiring one monitor, want 1 store pass", got)
	}
	if got := vmware.reads.Load(); got != 1 {
		t.Fatalf("VMware provider read %d times while wiring one monitor, want 1 store pass", got)
	}

	// NewRouter, SetMonitor, SetMultiTenantMonitor and every provider change
	// wire the same monitor again; none of it has anything new to publish.
	router.configureMonitorDependencies(monitor)
	router.configureMonitorDependencies(monitor)
	if trueNAS.reads.Load() != 1 || vmware.reads.Load() != 1 {
		t.Fatalf("re-wiring the same monitor read the providers %d and %d times in all, want one store pass", trueNAS.reads.Load(), vmware.reads.Load())
	}
}

// passCountingStore counts the registry rebuilds a monitor publishes into it.
type passCountingStore struct {
	*unifiedresources.MonitorAdapter
	passes atomic.Int32
}

func (s *passCountingStore) PopulateSnapshotAndSupplemental(snapshot models.StateSnapshot, recordsBySource map[unifiedresources.DataSource][]unifiedresources.IngestRecord) {
	s.passes.Add(1)
	s.MonitorAdapter.PopulateSnapshotAndSupplemental(snapshot, recordsBySource)
}

// A mock-mode switch re-registers both platform providers; the running monitor
// must publish once for the pair, not once per source.
func TestSyncPlatformSupplementalProvidersPublishesOnceForBothSources(t *testing.T) {
	previousTrueNAS := truenas.IsFeatureEnabled()
	previousVMware := vmware.IsFeatureEnabled()
	t.Cleanup(func() {
		truenas.SetFeatureEnabled(previousTrueNAS)
		vmware.SetFeatureEnabled(previousVMware)
	})

	monitor := newWiringTestMonitor(t)
	store := &passCountingStore{MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
	monitor.SetResourceStore(store)
	router := &Router{monitor: monitor}

	before := store.passes.Load()
	router.syncPlatformSupplementalProviders(true)
	if got := store.passes.Load() - before; got != 1 {
		t.Fatalf("switching mock mode on published %d times, want 1 for the TrueNAS and VMware pair", got)
	}

	before = store.passes.Load()
	router.syncPlatformSupplementalProviders(false)
	if got := store.passes.Load() - before; got != 1 {
		t.Fatalf("switching mock mode off published %d times, want 1 for the TrueNAS and VMware pair", got)
	}
}

// The router keeps the providers for monitors it wires later, and a nil
// provider removes one without touching the other.
func TestSetMonitorSupplementalRecordsProvidersKeepsTheRouterRegistry(t *testing.T) {
	router := &Router{}
	trueNAS := &countingSupplementalRecords{}
	vmware := &countingSupplementalRecords{}
	router.setMonitorSupplementalRecordsProviders(map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
		" TrueNAS ":                   trueNAS,
		unifiedresources.SourceVMware: vmware,
		"   ":                         &countingSupplementalRecords{},
	})
	if router.monitorSupplementalRecords[unifiedresources.SourceTrueNAS] != trueNAS || router.monitorSupplementalRecords[unifiedresources.SourceVMware] != vmware {
		t.Fatalf("the router did not keep the providers for monitors created later: %v", router.monitorSupplementalRecords)
	}
	if len(router.monitorSupplementalRecords) != 2 {
		t.Fatalf("a blank source was registered: %v", router.monitorSupplementalRecords)
	}

	router.setMonitorSupplementalRecordsProviders(map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: nil,
	})
	if _, kept := router.monitorSupplementalRecords[unifiedresources.SourceTrueNAS]; kept {
		t.Fatal("a removed provider stayed registered for monitors created later")
	}
	if router.monitorSupplementalRecords[unifiedresources.SourceVMware] != vmware {
		t.Fatal("removing one provider dropped the other")
	}
}
