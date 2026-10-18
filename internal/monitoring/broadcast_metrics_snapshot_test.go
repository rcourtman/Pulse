package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func projectionFixtureGraph(t testing.TB) mock.FixtureGraph {
	t.Helper()
	previousEnabled, previousConfig := mock.IsMockEnabled(), mock.GetConfig()
	t.Cleanup(func() {
		mustSetMockEnabled(t, false)
		mock.SetMockConfig(previousConfig)
		mustSetMockEnabled(t, previousEnabled)
	})
	mustSetMockEnabled(t, false)
	cfg := mock.DefaultConfig
	cfg.UpdateInterval = 5 * time.Minute
	mock.SetMockConfig(cfg)
	mustSetMockEnabled(t, true)
	graph := mock.CurrentFixtureGraph()
	mustSetMockEnabled(t, false)
	return graph
}

func projectionFixtureRegistry(graph mock.FixtureGraph) *unifiedresources.ResourceRegistry {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(graph.State)
	for _, source := range []unifiedresources.DataSource{unifiedresources.SourceTrueNAS, unifiedresources.SourceVMware, unifiedresources.SourceAvailability} {
		rr.IngestRecords(source, graph.SupplementalRecords(source))
	}
	return rr
}

// The original full projection, including target resolution after host
// coalescing. Compare all wire fields/catalogues, not selected metric values.
func legacyFrontendProjectionForTest(m *Monitor, snapshot models.StateSnapshot, resources []unifiedresources.Resource, resolver MetricsTargetResourceStore, now time.Time) models.StateFrontend {
	rows := unifiedresources.CoalescePresentationHostResources(resources)
	rows = m.applyPersistedMetadataToUnifiedResources(rows)
	rows = unifiedresources.AttachResourceHealth(rows, resourceHealthAlerts(snapshot.ActiveAlerts), now)
	projected, catalogs := convertResourcesForBroadcastReference(rows, resolver)
	out := snapshot.ToFrontend()
	out.Resources = projected
	out.CapabilityCatalog = catalogs.capabilities
	out.PolicyCatalog = catalogs.policies
	out.AISafeSummaryCatalog = catalogs.aiSafeSummaries
	out.ConnectedInfrastructure = buildConnectedInfrastructure(rows, snapshot)
	return out
}

func TestBroadcastMetricsSnapshotConnectedContent(t *testing.T) {
	graph := projectionFixtureGraph(t)
	rr := projectionFixtureRegistry(graph)
	adapter := unifiedresources.NewMonitorAdapter(rr)
	store := &broadcastProjectionCountingStore{MonitorAdapter: adapter}
	m := &Monitor{resourceStore: store}
	resources := adapter.GetAll()
	if len(resources) < 1000 {
		t.Fatalf("demo fixture too small: %d", len(resources))
	}
	sources := map[unifiedresources.DataSource]bool{}
	for _, r := range resources {
		for _, source := range r.Sources {
			sources[source] = true
		}
	}
	for _, source := range []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourceAgent, unifiedresources.SourceDocker, unifiedresources.SourcePBS, unifiedresources.SourcePMG, unifiedresources.SourceK8s, unifiedresources.SourceTrueNAS, unifiedresources.SourceVMware, unifiedresources.SourceAvailability} {
		if !sources[source] {
			t.Fatalf("demo omits provider %s", source)
		}
	}
	assertEqual := func(stage string) {
		t.Helper()
		evaluationTime := time.Now().UTC()
		before, err := json.Marshal(adapter.GetAll())
		if err != nil {
			t.Fatal(err)
		}
		want := legacyFrontendProjectionForTest(m, graph.State, adapter.GetAll(), adapter, evaluationTime)
		// The parent's unified view fills a missing store watermark from the
		// listed resources. Preserve that full-state header in the oracle too.
		if freshness := latestUnifiedResourceLastSeen(adapter.GetAll()); !freshness.IsZero() {
			want.LastUpdate = freshness.UnixMilli()
		}
		got := m.buildBroadcastFrontendStateFromSnapshotWithClock(graph.State, m.mockModeFence.begin(), func() time.Time { return evaluationTime })
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if string(wantJSON) != string(gotJSON) {
			first := 0
			for first < len(wantJSON) && first < len(gotJSON) && wantJSON[first] == gotJSON[first] {
				first++
			}
			start := max(0, first-80)
			t.Fatalf("%s: complete frontend/catalogue/infrastructure JSON differs at byte %d: want %q, got %q (bytes %d/%d)",
				stage, first, wantJSON[start:min(len(wantJSON), first+160)], gotJSON[start:min(len(gotJSON), first+160)], len(wantJSON), len(gotJSON))
		}
		after, _ := json.Marshal(adapter.GetAll())
		if string(before) != string(after) {
			t.Fatalf("%s: projection changed source data", stage)
		}
		t.Logf("%s: %d complete resources, %d frontend rows, %d wire bytes equal legacy projection", stage, len(adapter.GetAll()), len(got.Resources), len(gotJSON))
	}
	assertEqual("cold-list-only")
	rr.Workloads()
	assertEqual("typed-views-clean")
	// Same timestamps, new payload, policy and URL. No time-only invalidation.
	resources = adapter.GetAll()
	resources[0].Name = "changed-resource"
	resources[0].Tags = []string{"customer-data"}
	resources[0].CustomURL = "https://example.invalid/changed"
	rr.IngestResources(resources)
	assertEqual("dirty-same-time")
}

func TestBroadcastMetricsSnapshotHealthClockBoundary(t *testing.T) {
	m, adapter, id := newReadStateCloneTestMonitor(t, 1)
	rows := adapter.GetAll()
	original := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	rows[0].LastSeen = original
	rows[0].SourceStatus = map[unifiedresources.DataSource]unifiedresources.SourceStatus{
		unifiedresources.SourceProxmox: {Status: "stale", LastSeen: original},
	}
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestResources(rows)
	m.resourceStore = &broadcastProjectionCountingStore{MonitorAdapter: unifiedresources.NewMonitorAdapter(rr)}
	for _, tc := range []struct {
		age    time.Duration
		detail string
	}{{59 * time.Second, "0m"}, {61 * time.Second, "1m"}} {
		calls := 0
		got := m.buildBroadcastFrontendStateFromSnapshotWithClock(models.EmptyStateSnapshot(), m.mockModeFence.begin(), func() time.Time {
			calls++
			return original.Add(tc.age)
		})
		if calls != 1 || len(got.Resources) != 1 || got.Resources[0].ID != id || got.Resources[0].LastSeen != original.UnixMilli() {
			t.Fatal("health clock changed capture identity/original time or was not sampled once")
		}
		var health unifiedresources.ResourceHealth
		if err := json.Unmarshal(got.Resources[0].Health, &health); err != nil {
			t.Fatal(err)
		}
		if health.Verdict != unifiedresources.HealthStale || len(health.Reasons) != 1 || health.Reasons[0].Code != "telemetry_stale" || health.Reasons[0].Detail != tc.detail {
			t.Fatalf("health age must advance across the minute boundary: %+v", health)
		}
	}
}

func TestBroadcastMetricsSnapshotOwnsTargetsBeforeLiveReplacement(t *testing.T) {
	m, adapter, _ := newReadStateCloneTestMonitor(t, 4)
	view := m.currentUnifiedStateView()
	if view.metricsTargets == nil {
		t.Fatal("broadcast did not capture a bulk resolver")
	}
	old := view.resources[0]
	want := adapter.MetricsTargetForResource(old.ID)
	adapter.PopulateFromSnapshot(models.EmptyStateSnapshot())
	if got := view.metricsTargets.MetricsTargetForResource(old.ID); !reflect.DeepEqual(got, want) {
		t.Fatal("captured resources were paired with a later generation's target")
	}
	if got := view.metricsTargets.MetricsTargetForResource("unknown"); got != nil {
		t.Fatal("unknown target fell through to live data")
	}
	copyTarget := view.metricsTargets.MetricsTargetForResource(old.ID)
	copyTarget.ResourceID = "caller-edit"
	if got := view.metricsTargets.MetricsTargetForResource(old.ID); got.ResourceID == "caller-edit" {
		t.Fatal("caller mutated the captured resolver")
	}
}

func TestBroadcastMetricsSnapshotContinuityAndLiveOverlays(t *testing.T) {
	m, adapter, _ := newReadStateCloneTestMonitor(t, 2)
	now := time.Now().UTC()
	m.hostContinuityStore = config.NewHostContinuityStore(t.TempDir(), nil)
	entry := config.HostContinuityEntry{HostID: "absent-host", Hostname: "absent.example", MachineID: "absent-machine", LastSeen: now.Add(-time.Hour)}
	if err := m.hostContinuityStore.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	view := m.unifiedStateViewWithStandaloneHostContinuity(monitorUnifiedStateView{readState: adapter})
	if len(view.resources) != 3 || view.metricsTargets == nil {
		t.Fatal("continuity projection omitted its resources or captured resolver")
	}
	var host unifiedresources.Resource
	for _, r := range view.resources {
		if r.Agent != nil && r.Agent.AgentID == entry.HostID {
			host = r
		}
		if got, want := view.metricsTargets.MetricsTargetForResource(r.ID), broadcastMetricsTargetResolver(view.readState).MetricsTargetForResource(r.ID); !reflect.DeepEqual(got, want) {
			t.Fatal("overlay target differs from overlay registry")
		}
	}
	if host.ID == "" || host.Status == unifiedresources.StatusOnline || !host.LastSeen.Equal(entry.LastSeen) {
		t.Fatal("continuity history was omitted or promoted to current")
	}
	m.hostMetadataStore = config.NewHostMetadataStore(t.TempDir(), nil)
	for _, url := range []string{"https://example.invalid/operator", ""} {
		if err := m.hostMetadataStore.Set(entry.HostID, &config.HostMetadata{CustomURL: url}); err != nil {
			t.Fatal(err)
		}
		decorated := m.applyPersistedMetadataToUnifiedResources(view.resources)
		for _, r := range decorated {
			if r.ID == host.ID && r.CustomURL != url {
				t.Fatal("operator URL change/clear was hidden by captured data")
			}
		}
	}
	for _, r := range view.resources {
		if r.Type != unifiedresources.ResourceTypeVM {
			continue
		}
		baseline := unifiedresources.EvaluateResourceHealth(r, nil, now)
		critical := unifiedresources.EvaluateResourceHealth(r, []unifiedresources.ResourceHealthAlert{{ResourceID: r.ID, Level: "critical", Type: "cpu"}}, now)
		cleared := unifiedresources.EvaluateResourceHealth(r, nil, now)
		if baseline.Verdict != unifiedresources.HealthOK || critical.Verdict != unifiedresources.HealthCritical || !reflect.DeepEqual(cleared, baseline) {
			t.Fatal("live alert/clear was hidden by captured data")
		}
	}
	if len(adapter.GetAll()) != 2 {
		t.Fatal("continuity overlay wrote back to the live registry")
	}
}

var completeProjectionSink models.StateFrontend
var completeProjectionJSONSink []byte

// Exact parent broadcast shape (one coalesce, copy-to-attach, then conversion),
// not the older two-coalesce content oracle above. The unchanged conversion,
// metadata/health decoration and snapshot projection are part of both costs.
func parentFrontendProjectionForTest(m *Monitor, snapshot models.StateSnapshot, resources []unifiedresources.Resource, resolver MetricsTargetResourceStore, now time.Time) models.StateFrontend {
	rows := unifiedresources.CoalescePresentationHostResources(resources)
	healthAlerts := resourceHealthAlerts(snapshot.ActiveAlerts)
	for i := range rows {
		m.applyPersistedMetadataToUnifiedResource(&rows[i])
		health := unifiedresources.EvaluateResourceHealth(rows[i], healthAlerts, now)
		rows[i].Health = &health
	}
	projected, catalogs := convertPresentationResourcesForBroadcast(attachBroadcastMetricsTargets(rows, resolver))
	out := snapshot.ToFrontend()
	out.Resources = projected
	out.CapabilityCatalog = catalogs.capabilities
	out.PolicyCatalog = catalogs.policies
	out.AISafeSummaryCatalog = catalogs.aiSafeSummaries
	out.ConnectedInfrastructure = buildConnectedInfrastructure(rows, snapshot)
	if freshness := latestUnifiedResourceLastSeen(resources); !freshness.IsZero() {
		out.LastUpdate = freshness.UnixMilli()
	}
	return out
}

func BenchmarkBroadcastMetricsSnapshot(b *testing.B) {
	graph := projectionFixtureGraph(b)
	for _, mode := range []string{"cold", "list-clean", "views-clean", "dirty"} {
		for _, encode := range []bool{false, true} {
			stage := "projection"
			if encode {
				stage = "json-frame"
			}
			for _, bulk := range []bool{false, true} {
				path := "point-targets"
				if bulk {
					path = "bulk-targets"
				}
				b.Run(fmt.Sprintf("%s/%s/%s", mode, stage, path), func(b *testing.B) {
					rr := projectionFixtureRegistry(graph)
					adapter := unifiedresources.NewMonitorAdapter(rr)
					store := &broadcastProjectionCountingStore{MonitorAdapter: adapter}
					m := &Monitor{resourceStore: store}
					if mode != "cold" {
						rr.List()
					}
					if mode == "views-clean" {
						rr.Workloads()
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if mode == "cold" || mode == "dirty" {
							b.StopTimer()
							if mode == "cold" {
								rr = projectionFixtureRegistry(graph)
								adapter = unifiedresources.NewMonitorAdapter(rr)
								store.MonitorAdapter = adapter
							} else {
								changed := rr.List()[0]
								changed.Tags = []string{fmt.Sprintf("edit-%d", i)}
								rr.IngestResources([]unifiedresources.Resource{changed})
							}
							b.StartTimer()
						}
						if bulk {
							completeProjectionSink = m.buildBroadcastFrontendStateFromSnapshot(graph.State, m.mockModeFence.begin())
						} else {
							completeProjectionSink = parentFrontendProjectionForTest(m, graph.State, adapter.GetAll(), adapter, time.Now().UTC())
						}
						if encode {
							var err error
							completeProjectionJSONSink, err = json.Marshal(completeProjectionSink)
							if err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		}
	}
}
