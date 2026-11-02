package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

// Different shapes, not multiples of one trajectory, catch per-pool aliasing.
// CORE wire responses are synthetic; all ingestion, persistence and HTTP code
// is the assigned source. No customer appliance/state is changed.
func TestCOREIndependentDiskTrajectories(t *testing.T) {
	old := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(old) })
	oldMulti := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(oldMulti) })
	t.Setenv("PULSE_DEV", "true")
	var cycle atomic.Int64
	// Use real present-second samples in the two-step native freshness window.
	start := time.Now().UTC().Truncate(time.Second).Unix() - 55
	reads := [][]any{{1., 9., 4., 8., 2., 0.}, {8., 2., 0., 7., 5., 1.}, {2., 0., 6., 1., 9., 4.}, {9., 4., 1., 0., 3., 7.}}
	writes := [][]any{{4., 1., 8., 2., 0., 6.}, {0., 6., 3., 9., 1., 5.}, {7., 3., 0., 8., 2., 1.}, {1., 8., 4., 2., 6., 0.}}
	providers := map[string]*truenas.Provider{}
	for connection := 0; connection < 2; connection++ {
		connection := connection
		name := fmt.Sprintf("core-%d", connection)
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v2.0/system/info":
				fmt.Fprint(w, `{"hostname":"namesake-core","version":"TrueNAS-13.0-U6.1","cores":4,"physmem":17179869184}`)
			case "/api/v2.0/pool":
				fmt.Fprint(w, `[{"id":1,"name":"tank","status":"ONLINE","size":10000,"allocated":4000,"free":6000},{"id":2,"name":"boot-pool","status":"ONLINE","size":2000,"allocated":500,"free":1500}]`)
			case "/api/v2.0/disk":
				fmt.Fprintf(w, `[{"identifier":"{da0}","name":"da0","serial":"fixture-%d-da0","model":"Main A","pool":"tank","size":1000},{"identifier":"{da1}","name":"da1","serial":"fixture-%d-da1","model":"Main B","pool":"tank","size":1000},{"identifier":"{da2}","name":"da2","serial":"UNKNOWN","model":"Boot A","pool":"boot-pool","size":1000},{"identifier":"{da3}","name":"da3","model":"Boot B","pool":"boot-pool","size":1000}]`, connection, connection)
			case "/api/v2.0/disk/temperatures":
				fmt.Fprint(w, `{"da0":30,"da1":31,"da2":32,"da3":33}`)
			case "/api/v2.0/pool/dataset", "/api/v2.0/alert/list":
				fmt.Fprint(w, `[]`)
			case "/api/v2.0/reporting/graphs":
				fmt.Fprint(w, `[{"name":"disk","identifiers":["da0","da1","da2","da3"]}]`)
			case "/api/v2.0/reporting/get_data":
				var q struct {
					Graphs []struct{ Name, Identifier string }
					Query  struct{ Start, End int64 } `json:"reporting_query"`
				}
				if json.NewDecoder(r.Body).Decode(&q) != nil || q.Query.End <= q.Query.Start {
					http.Error(w, "invalid fixture query", 400)
					return
				}
				at := start + cycle.Load()*10
				replies := []map[string]any{}
				for _, g := range q.Graphs {
					switch g.Name {
					case "disk":
						var index int
						if _, err := fmt.Sscanf(g.Identifier, "da%d", &index); err != nil || index < 0 || index > 3 {
							t.Errorf("unrecognised fixture device %q", g.Identifier)
							continue
						}
						read, write := reads[index][cycle.Load()], writes[index][cycle.Load()]
						// Directional absence is not zero and must not copy another disk/pool.
						if cycle.Load() == 3 && index == 1 {
							write = nil
						}
						if connection == 1 {
							if read != nil {
								read = read.(float64)*3 + 17
							}
							if write != nil {
								write = write.(float64)*2 + 11
							}
						}
						replies = append(replies, map[string]any{"name": "disk", "identifier": g.Identifier, "start": at, "end": at, "step": 60, "legend": []string{"disk_octets_read", "disk_octets_write"}, "data": [][]any{{read, write}}})
					case "cpu":
						replies = append(replies, map[string]any{"name": "cpu", "start": at, "end": at, "step": 60, "legend": []string{"usage"}, "data": [][]any{{12.}}})
					}
				}
				// The native batch need not return requested device order.
				for i, j := 0, len(replies)-1; i < j; i, j = i+1, j-1 {
					replies[i], replies[j] = replies[j], replies[i]
				}
				_ = json.NewEncoder(w).Encode(replies)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)
		client, err := truenas.NewClient(truenas.ClientConfig{Host: server.URL, APIKey: "synthetic-distinct-key", Fingerprint: fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw)), InsecureSkipVerify: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(client.Close)
		providers[name] = truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, name)
	}
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	cached := map[string][]unifiedresources.IngestRecord{}
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{"org-distinct": providers})
	monitor := &monitoring.Monitor{}
	monitor.SetOrgID("org-distinct")
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(1024, 24*time.Hour))
	setUnexportedField(t, monitor, "metricsStore", store)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	for c := int64(0); c < 6; c++ {
		cycle.Store(c)
		for name, p := range providers {
			if err := p.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			cached[name] = p.Records()
		}
		setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{"org-distinct": cached})
		monitor.SetResourceStoreAndProviders(adapter, map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller})
		monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	}
	store.Flush()
	const raw = "distinct-reader.1234567890123456789"
	token := newTokenRecord(t, raw, []string{config.ScopeMonitoringRead}, nil)
	token.OrgID = "org-distinct"
	cfg := newTestConfigWithTokens(t, token)
	cfg.AuthUser, cfg.AuthPass = "fixture-admin", "synthetic-unused-hash"
	for _, org := range []string{"org-distinct", "org-other"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataPath, "orgs", org), 0700); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouter(cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	mtm := monitoring.NewMultiTenantMonitor(cfg, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"org-distinct": monitor, "org-other": {}})
	router.mtMonitor = mtm
	router.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, monitor))
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	request := func(org, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-API-Token", raw)
		r.Header.Set("X-Pulse-Org-ID", org)
		w := httptest.NewRecorder()
		router.Handler().ServeHTTP(w, r)
		return w
	}
	resourceReply := request("org-distinct", "/api/resources?type=agent,storage,physical_disk&source=truenas&limit=100")
	var resources ResourcesResponse
	if resourceReply.Code != 200 || json.Unmarshal(resourceReply.Body.Bytes(), &resources) != nil {
		t.Fatalf("resources %d %s", resourceReply.Code, resourceReply.Body.String())
	}
	t.Logf("DISTINCT_RESOURCES %s", resourceReply.Body.String())
	hosts := map[string]bool{}
	for _, r := range resources.Data {
		if r.Type != unifiedresources.ResourceTypeAgent {
			continue
		}
		target := r.MetricsTarget
		if target == nil || target.ResourceType != "agent" || hosts[target.ResourceID] {
			t.Fatalf("host borrowed connection history: %+v", r)
		}
		hosts[target.ResourceID] = true
		wantRead, wantWrite := 12., 12.
		if target.ResourceID == "core-1" {
			wantRead, wantWrite = 104, 68
		}
		if r.Metrics == nil || r.Metrics.DiskRead == nil || r.Metrics.DiskWrite == nil || r.Metrics.DiskRead.Value != wantRead || r.Metrics.DiskWrite.Value != wantWrite {
			t.Fatalf("host total from another connection: %+v", r)
		}
		for _, range_ := range []string{"1h", "24h"} {
			reply := request("org-distinct", "/api/metrics-store/history?resourceType=agent&resourceId="+url.QueryEscape(target.ResourceID)+"&range="+range_+"&maxPoints=240")
			if reply.Code != 200 {
				t.Fatalf("host history %d %s", reply.Code, reply.Body.String())
			}
			t.Logf("DISTINCT_HOST_HISTORY %s", reply.Body.String())
		}
	}
	if len(hosts) != 2 || !hosts["core-0"] || !hosts["core-1"] {
		t.Fatalf("same-named connections collapsed: %+v", hosts)
	}
	count := 0
	seen := map[string]bool{}
	shapes := map[string][]float64{}
	for _, r := range resources.Data {
		if r.Type != unifiedresources.ResourceTypePhysicalDisk {
			continue
		}
		count++
		target := r.MetricsTarget
		if target == nil || target.ResourceType != "disk" || seen[target.ResourceID] {
			t.Fatalf("shared/missing disk target: %+v", r)
		}
		seen[target.ResourceID] = true
		var index int
		fmt.Sscanf(r.Name, "da%d", &index)
		connection := 0
		if len(target.ResourceID) > 0 && (target.ResourceID == "fixture-1-da0" || target.ResourceID == "fixture-1-da1" || target.ResourceID == "system:core-1:disk:da2" || target.ResourceID == "system:core-1:disk:da3") {
			connection = 1
		}
		// Source coordinates identify the owning connection even for placeholder serials.
		for name, p := range providers {
			for _, rec := range p.Records() {
				if rec.Resource.Type == unifiedresources.ResourceTypePhysicalDisk && rec.Resource.Name == r.Name {
					if unifiedresources.PhysicalDiskMetaMetricID(rec.Resource.PhysicalDisk, rec.SourceID) == target.ResourceID && name == "core-1" {
						connection = 1
					}
				}
			}
		}
		uri := "/api/metrics-store/history?resourceType=disk&resourceId=" + url.QueryEscape(target.ResourceID) + "&range=1h&maxPoints=240"
		w := request("org-distinct", uri)
		var reply struct {
			ResourceID string
			Metrics    map[string][]hostIOHistoryPoint
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &reply) != nil || reply.ResourceID != target.ResourceID {
			t.Fatalf("history %d %s", w.Code, w.Body.String())
		}
		for kind, trajectory := range map[string][]any{"diskread": reads[index], "diskwrite": writes[index]} {
			type bucket struct {
				sum, min, max float64
				n             int
			}
			b := map[int64]bucket{}
			for c, v := range trajectory {
				if kind == "diskwrite" && index == 1 && c == 3 {
					continue
				}
				value := v.(float64)
				if connection == 1 {
					if kind == "diskread" {
						value = value*3 + 17
					} else {
						value = value*2 + 11
					}
				}
				ts := (start + int64(c)*10) / 15 * 15
				entry, ok := b[ts]
				if !ok {
					entry.min, entry.max = value, value
				}
				entry.sum += value
				entry.n++
				if value < entry.min {
					entry.min = value
				}
				if value > entry.max {
					entry.max = value
				}
				b[ts] = entry
			}
			times := []int64{}
			for ts := range b {
				times = append(times, ts)
			}
			sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
			points := reply.Metrics[kind]
			if len(points) != len(times) {
				t.Fatalf("%s %s points %v expected buckets %v", target.ResourceID, kind, points, times)
			}
			for i, ts := range times {
				want := b[ts]
				if points[i].Timestamp != (ts+7)*1000 || points[i].Value != want.sum/float64(want.n) || points[i].Min != want.min || points[i].Max != want.max {
					t.Fatalf("copied or wrong trajectory %s %s[%d] = %+v want %+v", target.ResourceID, kind, i, points[i], want)
				}
				shapes[target.ResourceID+kind] = append(shapes[target.ResourceID+kind], points[i].Value)
			}
		}
		t.Logf("DISTINCT_HISTORY %s", w.Body.String())
		day := request("org-distinct", "/api/metrics-store/history?resourceType=disk&resourceId="+url.QueryEscape(target.ResourceID)+"&range=24h&maxPoints=240")
		if day.Code != 200 {
			t.Fatal(day.Body.String())
		}
		t.Logf("DISTINCT_HISTORY %s", day.Body.String())
		if w := request("org-other", uri); w.Code != 403 {
			t.Fatalf("cross-tenant history %d", w.Code)
		}
	}
	if count != 8 {
		t.Fatalf("disk count %d, want 8", count)
	}
	for a, x := range shapes {
		for b, y := range shapes {
			if a != b && reflect.DeepEqual(x, y) {
				t.Fatalf("fixture trajectories not independent: %s %s", a, b)
			}
		}
	}
	if cfg.RemoveAPIToken(token.ID) == nil {
		t.Fatal("token missing")
	}
	if w := request("org-distinct", "/api/metrics-store/history?resourceType=disk&resourceId=fixture-0-da0&range=1h"); w.Code != 401 {
		t.Fatal("withdrawn token history visible")
	}
}
