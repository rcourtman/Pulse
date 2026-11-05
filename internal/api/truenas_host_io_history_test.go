package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// This fixture models CORE's REST graph contract, not a reporter's appliance.
// Live disjoint disk samples survive individually but cannot form a host sum.
// History phases independently control whether a complete native sum exists.
type hostIORESTFixture struct {
	server *httptest.Server
	phase  atomic.Int32
	reads  atomic.Int32
	end    atomic.Int64
}

func newHostIORESTFixture(t *testing.T) *hostIORESTFixture {
	t.Helper()
	f := &hostIORESTFixture{}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			fmt.Fprint(w, `{"hostname":"same-core-name","version":"TrueNAS-13.0-U6.8","cores":8,"physmem":17179869184}`)
		case "/api/v2.0/pool":
			fmt.Fprint(w, `[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`)
		case "/api/v2.0/disk":
			fmt.Fprint(w, `[{"identifier":"disk-a","name":"da0","serial":"host-io-a","type":"HDD","size":1000},{"identifier":"disk-b","name":"da1","serial":"host-io-b","type":"HDD","size":1000}]`)
		case "/api/v2.0/pool/dataset", "/api/v2.0/alert/list":
			fmt.Fprint(w, `[]`)
		case "/api/v2.0/reporting/graphs":
			if f.phase.Load() == 2 {
				fmt.Fprint(w, `[{"name":"disk","identifiers":["da0","da1","da2"]}]`)
			} else {
				fmt.Fprint(w, `[{"name":"disk","identifiers":["da0","da1"]}]`)
			}
		case "/api/v2.0/reporting/get_data":
			f.reads.Add(1)
			var request struct {
				Graphs []struct{ Name, Identifier string }
				Query  struct{ Start, End int64 } `json:"reporting_query"`
			}
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&request) != nil || request.Query.End <= request.Query.Start {
				http.Error(w, "invalid synthetic reporting request", 400)
				return
			}
			f.end.Store(request.Query.End)
			var responses []map[string]any
			for _, graph := range request.Graphs {
				start, step := request.Query.End-3540, int64(1160)
				var legend []string
				var row []any
				switch graph.Name {
				case "cpu":
					legend, row = []string{"usage"}, []any{12.}
				case "disk":
					legend, row = []string{"disk_octets_read", "disk_octets_write"}, []any{100., 200.}
					if graph.Identifier == "da1" {
						row = []any{1000., 2000.}
					}
					if f.phase.Load() == 3 {
						row = []any{0., 0.}
					}
					if f.phase.Load() == 4 && graph.Identifier == "da1" {
						row[0] = nil
					}
				default:
					continue
				}
				rows := []any{row, row, row, row}
				if graph.Identifier == "da2" {
					rows = nil
				}
				if f.phase.Load() == 0 && graph.Name == "disk" {
					start, step, rows = request.Query.End-20, 10, []any{row, row}
					if graph.Identifier == "da1" {
						start += 5
					}
				}
				responses = append(responses, map[string]any{"name": graph.Name, "identifier": graph.Identifier, "legend": legend, "start": start, "end": request.Query.End, "step": step, "data": rows})
			}
			if err := json.NewEncoder(w).Encode(responses); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *hostIORESTFixture) provider(t *testing.T, connection string) *truenas.Provider {
	t.Helper()
	client, err := truenas.NewClient(truenas.ClientConfig{Host: f.server.URL, APIKey: "synthetic-host-io-key", InsecureSkipVerify: true, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(f.server.Certificate().Raw))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	p := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, connection)
	if err := p.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

type hostIOHistoryPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
}

// Exercise native CORE collection -> canonical ingestion -> persisted HTTP,
// including the drawer's all-metric query with unrelated covered CPU history.
func TestTrueNASHostIOHistoryWithPartialStore(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	previousMulti := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(previousMulti) })
	t.Setenv("PULSE_DEV", "true")
	f, other := newHostIORESTFixture(t), newHostIORESTFixture(t)
	provider, otherProvider := f.provider(t, "host-io-target"), other.provider(t, "other-target")
	snapshot := provider.Snapshot()
	if snapshot.System.Telemetry.DiskRead || snapshot.System.Telemetry.DiskWrite || len(snapshot.System.DiskReadRates) != 2 {
		t.Fatal("fixture did not establish complete per-disk readings with an absent host total")
	}
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{"org-host-io": {"host-io-target": provider, "other-target": otherProvider}})
	setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{"org-host-io": {"host-io-target": provider.Records()}})
	monitor := &monitoring.Monitor{}
	monitor.SetOrgID("org-host-io")
	setUnexportedField(t, monitor, "state", models.NewState())
	ring := monitoring.NewMetricsHistory(1024, 24*time.Hour)
	setUnexportedField(t, monitor, "metricsHistory", ring)
	setUnexportedField(t, monitor, "metricsStore", store)
	monitor.SetResourceStoreAndProviders(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)), map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller})
	store.Flush()
	now := time.Now().UTC().Truncate(time.Second)
	// Simulate an established CPU ring. That must not certify an absent I/O series.
	for _, offset := range []time.Duration{-55 * time.Minute, -30 * time.Minute, -time.Minute} {
		ring.AddGuestMetric("agent:host-io-target", "cpu", 19, now.Add(offset))
	}
	const token = "host-io-reader.1234567890123456789"
	readToken := newTokenRecord(t, token, []string{config.ScopeMonitoringRead}, nil)
	readToken.OrgID = "org-host-io"
	const wrongScope = "host-io-settings.1234567890123456789"
	scopeToken := newTokenRecord(t, wrongScope, []string{config.ScopeSettingsRead}, nil)
	scopeToken.OrgID = "org-host-io"
	cfg := newTestConfigWithTokens(t, readToken, scopeToken)
	cfg.AuthUser, cfg.AuthPass = "fixture-admin", "synthetic-unused-password-hash"
	for _, org := range []string{"org-host-io", "org-other"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataPath, "orgs", org), 0700); err != nil {
			t.Fatal(err)
		}
	}
	router := newTestRouter(t, cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	mtm := monitoring.NewMultiTenantMonitor(cfg, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"org-host-io": monitor, "org-other": {}})
	router.mtMonitor = mtm
	router.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, monitor))
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	request := func(raw, org, resourceType, id, metric string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType="+resourceType+"&resourceId="+id+"&metric="+metric+"&range=1h&maxPoints=240", nil)
		req.Header.Set("X-Pulse-Org-ID", org)
		if raw != "" {
			req.Header.Set("X-API-Token", raw)
		}
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}
	f.reads.Store(0)
	other.reads.Store(0)
	for _, test := range []struct {
		raw, org string
		want     int
	}{{"", "org-host-io", 401}, {wrongScope, "org-host-io", 403}, {token, "org-other", 403}} {
		if rec := request(test.raw, test.org, "agent", "host-io-target", ""); rec.Code != test.want || f.reads.Load() != 0 || other.reads.Load() != 0 {
			t.Fatalf("auth/scope/tenant gate: status %d want %d reads %d/%d", rec.Code, test.want, f.reads.Load(), other.reads.Load())
		}
	}
	// Actual per-disk persistence is independent of the native host fallback.
	for _, serial := range []string{"host-io-a", "host-io-b"} {
		rec := request(token, "org-host-io", "disk", serial, "diskread")
		var result struct{ Points []hostIOHistoryPoint }
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &result) != nil || len(result.Points) == 0 {
			t.Fatalf("individual disk lost persisted I/O: %d %s", rec.Code, rec.Body.String())
		}
	}
	if f.reads.Load() != 0 || other.reads.Load() != 0 {
		t.Fatal("persisted per-disk I/O queried native host History")
	}
	var storedCPU []hostIOHistoryPoint
	for _, test := range []struct {
		name        string
		phase       int32
		read, write bool
		value       float64
	}{{"complete", 1, true, true, 1100}, {"empty-member", 2, false, false, 0}, {"measured-zero", 3, true, true, 0}, {"write-only", 4, false, true, 0}, {"disjoint", 0, false, false, 0}} {
		t.Run(test.name, func(t *testing.T) {
			f.phase.Store(test.phase)
			before := f.reads.Load()
			rec := request(token, "org-host-io", "agent", "host-io-target", "")
			var result struct {
				Metrics map[string][]hostIOHistoryPoint
				Source  *string
			}
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &result) != nil {
				t.Fatalf("host history %d %s", rec.Code, rec.Body.String())
			}
			if (len(result.Metrics["diskread"]) > 0) != test.read || (len(result.Metrics["diskwrite"]) > 0) != test.write {
				t.Fatalf("host I/O completeness: read=%d write=%d want %t/%t", len(result.Metrics["diskread"]), len(result.Metrics["diskwrite"]), test.read, test.write)
			}
			if test.read || test.write {
				if result.Source != nil {
					t.Fatal("mixed native/stored history claimed a single source")
				}
			} else if result.Source == nil || *result.Source != "store" {
				t.Fatal("unchanged stored history lost its source hint")
			}
			if f.reads.Load()-before != 1 || other.reads.Load() != 0 {
				t.Fatalf("host fallback read other appliance or lost its one-request bound: reads %d/%d", f.reads.Load()-before, other.reads.Load())
			}
			if len(result.Metrics["cpu"]) != 1 || result.Metrics["cpu"][0].Value != 12 {
				t.Fatalf("host fallback replaced stored CPU: %+v", result.Metrics["cpu"])
			}
			if storedCPU == nil {
				storedCPU = result.Metrics["cpu"]
			} else if result.Metrics["cpu"][0] != storedCPU[0] {
				t.Fatal("native supplement changed stored CPU time/bounds")
			}
			for _, metric := range []string{"diskread", "diskwrite"} {
				for i, point := range result.Metrics[metric] {
					value := test.value
					if metric == "diskwrite" && test.phase != 3 {
						value = 2200
					}
					ts := f.end.Load() - 3540 + int64(i)*1160
					if point.Value != value || point.Min != value || point.Max != value || point.Timestamp != (ts/15*15+7)*1000 {
						t.Fatalf("native rate/time/bucket/bounds changed: %s %+v", metric, point)
					}
				}
			}
			t.Logf("HOST_IO_BROWSER %s %s", test.name, rec.Body.String())
		})
	}
	// Single-series reads must qualify that series, not a covered CPU sibling.
	f.phase.Store(1)
	before := f.reads.Load()
	rec := request(token, "org-host-io", "agent", "host-io-target", "diskread")
	var single struct{ Points []hostIOHistoryPoint }
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &single) != nil || len(single.Points) != 4 || f.reads.Load()-before != 1 || other.reads.Load() != 0 {
		t.Fatalf("single host I/O hidden by covered CPU: status %d points %d reads %d/%d", rec.Code, len(single.Points), f.reads.Load()-before, other.reads.Load())
	}
	// Existing stored directions are not replaced by a larger native series.
	ioTime := now.Add(-20 * time.Second)
	store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "agent", ResourceID: "host-io-target", MetricType: "diskread", Value: 777, Timestamp: ioTime, Tier: metrics.TierRaw}})
	rec = request(token, "org-host-io", "agent", "host-io-target", "")
	var partial struct {
		Metrics map[string][]hostIOHistoryPoint
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &partial) != nil || len(partial.Metrics["diskread"]) != 1 || partial.Metrics["diskread"][0].Value != 777 || len(partial.Metrics["diskwrite"]) != 4 {
		t.Fatalf("native fill replaced stored direction: %s", rec.Body.String())
	}
	store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "agent", ResourceID: "host-io-target", MetricType: "diskwrite", Value: 888, Timestamp: ioTime, Tier: metrics.TierRaw}})
	before = f.reads.Load()
	rec = request(token, "org-host-io", "agent", "host-io-target", "")
	if rec.Code != 200 || f.reads.Load() != before || other.reads.Load() != 0 {
		t.Fatal("complete stored I/O needlessly queried native History")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if rec := request(token, "org-host-io", "agent", "host-io-target", ""); rec.Code != 500 || f.reads.Load() != before {
		t.Fatal("failed store read became successful native history")
	}
	if cfg.RemoveAPIToken(readToken.ID) == nil {
		t.Fatal("missing fixture token")
	}
	if rec := request(token, "org-host-io", "agent", "host-io-target", ""); rec.Code != 401 || f.reads.Load() != before {
		t.Fatal("withdrawn reader reached native history")
	}
}
