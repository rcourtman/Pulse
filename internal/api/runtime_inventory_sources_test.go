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
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
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

// mockMonitorWithStaleView returns a mock-mode monitor whose estate view was
// built before the fixture last ticked, and the structure read state it holds.
func mockMonitorWithStaleView(t *testing.T) (*monitoring.Monitor, *config.Config) {
	t.Helper()
	previous := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	estate := mock.DefaultConfig
	estate.NodeCount = 4
	estate.UpdateInterval = time.Second
	mock.SetMockConfig(estate)
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() {
		_ = mock.SetEnabled(false)
		mock.SetMockConfig(previousConfig)
		_ = mock.SetEnabled(previous)
	})

	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	t.Cleanup(monitor.Stop)

	monitor.GetUnifiedReadStateOrSnapshot()
	tick := mock.FixtureDataVersion()
	deadline := time.Now().Add(5 * time.Minute)
	for mock.FixtureDataVersion() <= tick {
		if time.Now().After(deadline) {
			t.Fatal("the mock fixture did not tick")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return monitor, cfg
}

// The connection-degraded alert feed keeps the platform rows (PVE, PBS, PMG,
// VMware, TrueNAS) and drops every agent row, so it has no use for the
// monitor's hosts or PBS instances. In mock mode each of those reads rebuilds
// the whole estate view once a metric tick has made it stale, and the feed
// runs on startup's critical path.
func TestAlertConnectionSnapshotsDoNotRebuildTheMockViewAfterFixtureTicks(t *testing.T) {
	monitor, cfg := mockMonitorWithStaleView(t)
	warm := monitor.GetUnifiedStructureReadState()

	buildAlertConnectionSnapshotsWithRuntimeSources(context.Background(), cfg, nil, monitor, aggregatorRuntimeSources{orgID: "default"})
	if monitor.GetUnifiedStructureReadState() != warm {
		t.Fatal("the alert connection feed rebuilt the unified view after a metric tick; it reads no host or PBS instance")
	}

	// Control: the connections endpoint does read them, and the test would
	// pass vacuously if a rebuild left no trace.
	buildAggregatorInputsWithRuntimeSources(context.Background(), cfg, nil, monitor, aggregatorRuntimeSources{orgID: "default"})
	if monitor.GetUnifiedStructureReadState() == warm {
		t.Fatal("reading the hosts did not rebuild the stale view, so this test cannot see a rebuild")
	}
}

// Leaving the hosts and PBS instances out must not change a platform row. The
// lean inputs are the full ones with exactly what platformRowsOnly omits taken
// away, so a supplemental fixture tick between two reads cannot make the rows
// differ for a reason other than the omission.
func TestAlertConnectionSnapshotsMatchTheFullAggregatorOnPlatformRows(t *testing.T) {
	monitor, cfg := mockMonitorWithStaleView(t)

	full := buildAggregatorInputsWithRuntimeSources(context.Background(), cfg, nil, monitor, aggregatorRuntimeSources{orgID: "default"})
	lean := buildAggregatorInputsWithRuntimeSources(context.Background(), cfg, nil, monitor, aggregatorRuntimeSources{orgID: "default", platformRowsOnly: true})

	if len(full.hosts) == 0 {
		t.Fatal("the mock estate lists no host, so the comparison proves nothing")
	}
	if len(lean.hosts) != 0 || lean.agentDesiredConfigs != nil || lean.pbsReportedNodeNames != nil {
		t.Fatalf("platformRowsOnly kept hosts=%d desiredConfigs=%v pbsNodeNames=%v", len(lean.hosts), lean.agentDesiredConfigs, lean.pbsReportedNodeNames)
	}

	stripped := full
	stripped.hosts = lean.hosts
	stripped.agentDesiredConfigs = nil
	stripped.pbsReportedNodeNames = nil
	want := snapshotConnectionsForAlerts(buildConnections(full))
	got := snapshotConnectionsForAlerts(buildConnections(stripped))
	if len(want) == 0 {
		t.Fatal("the mock estate yields no platform connection")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("platform connections changed without the hosts:\n got %+v\nwant %+v", got, want)
	}
}

// The patrol resolver turns a finding's resource reference into a canonical ID.
// It reads identity only, so in mock mode it must not rebuild the estate view
// after a metric tick (it runs once per finding).
func TestPatrolOperatorStateResolverDoesNotRebuildTheMockViewAfterFixtureTicks(t *testing.T) {
	monitor, cfg := mockMonitorWithStaleView(t)
	warm := monitor.GetUnifiedStructureReadState()

	router := &Router{
		resourceHandlers: NewResourceHandlers(&config.Config{DataPath: cfg.DataPath}),
		monitor:          monitor,
	}
	t.Cleanup(router.ShutdownResourceStores)
	provider := router.patrolResourceOperatorStateProvider("default")
	provider.OperatorStateProjection("some-finding-resource", time.Now())

	if monitor.GetUnifiedStructureReadState() != warm {
		t.Fatal("the patrol operator-state resolver rebuilt the unified view after a metric tick; it only resolves identity")
	}
}

// Mock mode drops the configured PVE, PBS and PMG instances from the ledger, so
// their rows are checked here with mock mode off: the platform rows an alert
// is raised from are the same with and without the hosts and PBS reads.
func TestAlertConnectionSnapshotsKeepConfiguredPlatformRowsWithoutTheHosts(t *testing.T) {
	previousMock := mock.IsMockEnabled()
	if err := mock.SetEnabled(false); err != nil {
		t.Fatalf("disable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previousMock) })
	monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	t.Cleanup(monitor.Stop)
	cfg := &config.Config{
		DataPath:     t.TempDir(),
		PVEInstances: []config.PVEInstance{{Name: "pve-lab", Host: "https://pve-lab.example:8006"}},
		PBSInstances: []config.PBSInstance{{Name: "pbs-lab", Host: "https://pbs-lab.example:8007"}},
		PMGInstances: []config.PMGInstance{{Name: "pmg-lab", Host: "https://pmg-lab.example:8006"}},
	}

	full := buildAggregatorInputsWithRuntimeSources(context.Background(), cfg, nil, monitor, aggregatorRuntimeSources{orgID: "default"})
	lean := buildAggregatorInputsWithRuntimeSources(context.Background(), cfg, nil, monitor, aggregatorRuntimeSources{orgID: "default", platformRowsOnly: true})
	lean.now = full.now

	want := snapshotConnectionsForAlerts(buildConnections(full))
	got := snapshotConnectionsForAlerts(buildConnections(lean))
	if len(want) != 3 {
		t.Fatalf("expected the PVE, PBS and PMG rows, got %+v", want)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("platform connections changed without the hosts:\n got %+v\nwant %+v", got, want)
	}
}
