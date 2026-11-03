package monitoring

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type batchLifetimeHistoryFetcher struct {
	truenas.FixtureFetcher
	onRead func(context.Context)
	value  float64
}

func (f *batchLifetimeHistoryFetcher) SystemMetricHistory(ctx context.Context, _ time.Duration) (*truenas.SystemMetricHistory, error) {
	if f.onRead != nil {
		f.onRead(ctx)
	}
	return &truenas.SystemMetricHistory{CPUPercent: []truenas.TimeSeriesPoint{{Timestamp: time.Now().UTC().Add(-time.Minute), Value: f.value}}}, nil
}

// The provider map is deliberately unordered. Whichever connection is read
// first loses ownership while the second native read is outstanding. Checking
// each provider only when its own read finishes cannot fence the whole result.
func TestTrueNASHostHistoryBatchRechecksEarlierOwners(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	for _, surface := range []string{"native", "chart-batch"} {
		for _, action := range []string{"remove", "replace", "disable"} {
			t.Run(surface+"/"+action, func(t *testing.T) {
				truenas.SetFeatureEnabled(true)
				poller := NewTrueNASPoller(nil, time.Minute, nil)
				fetchers := map[string]*batchLifetimeHistoryFetcher{}
				makeProvider := func(id string, value float64) *truenas.Provider {
					fixture := truenas.DefaultFixtures()
					fixture.System.Hostname = "namesake"
					f := &batchLifetimeHistoryFetcher{FixtureFetcher: truenas.FixtureFetcher{Snapshot: fixture}, value: value}
					p := truenas.NewLiveProviderForConnection(f, id)
					if err := p.Refresh(context.Background()); err != nil {
						t.Fatal(err)
					}
					fetchers[id] = f
					return p
				}
				poller.providersByOrg["org-current"] = map[string]*truenas.Provider{"a": makeProvider("a", 12), "b": makeProvider("b", 34)}
				replacement := makeProvider("replacement", 99)
				// A peer tenant with the same connection keys must never be selected or
				// removed as a side effect of withdrawing this tenant's result.
				other := makeProvider("other", 78)
				poller.providersByOrg["org-other"] = map[string]*truenas.Provider{"a": other}
				var calls atomic.Int32
				first := make(chan string, 1)
				entered := make(chan struct{})
				release := make(chan struct{})
				for _, id := range []string{"a", "b"} {
					id := id
					fetchers[id].onRead = func(ctx context.Context) {
						switch calls.Add(1) {
						case 1:
							first <- id
						case 2:
							close(entered)
							select {
							case <-release:
							case <-ctx.Done():
							}
						}
					}
				}
				monitor := &Monitor{metricsHistory: NewMetricsHistory(32, time.Hour), supplementalProviders: map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller}}
				monitor.SetOrgID("org-current")
				read := func() map[string]map[string][]MetricPoint {
					if surface == "native" {
						return poller.GuestMetricHistory(nil, "org-current", "agent", time.Hour)
					}
					return monitor.GetGuestMetricsForChartBatch("agent", []GuestChartRequest{{InMemoryKey: "agent:a", SQLResourceID: "a"}, {InMemoryKey: "agent:b", SQLResourceID: "b"}}, time.Hour, "cpu")
				}
				done := make(chan map[string]map[string][]MetricPoint, 1)
				go func() { done <- read() }()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					close(release)
					t.Fatal("second native read did not begin")
				}
				revoked := <-first
				survivor := "a"
				if revoked == survivor {
					survivor = "b"
				}
				poller.mu.Lock()
				switch action {
				case "remove":
					delete(poller.providersByOrg["org-current"], revoked)
				case "replace":
					poller.providersByOrg["org-current"][revoked] = replacement
				case "disable":
					truenas.SetFeatureEnabled(false)
				}
				poller.mu.Unlock()
				close(release)
				result := <-done
				if len(result[revoked]["cpu"]) != 0 {
					t.Fatalf("%s earlier connection retained native history after %s: %+v", revoked, action, result)
				}
				if action == "disable" {
					if len(result[survivor]["cpu"]) != 0 {
						t.Fatalf("disabled source retained later history: %+v", result)
					}
				} else if got := result[survivor]["cpu"]; len(got) != 1 || got[0].Value == 99 {
					t.Fatalf("surviving connection lost or borrowed history: %+v", result)
				}
				poller.mu.Lock()
				if poller.providersByOrg["org-other"]["a"] != other {
					t.Fatal("withdrawal changed peer tenant")
				}
				for _, id := range []string{"a", "b"} {
					fetchers[id].onRead = nil
				}
				if action == "remove" {
					poller.providersByOrg["org-current"][revoked] = replacement
				}
				poller.mu.Unlock()
				truenas.SetFeatureEnabled(true)
				fresh := read()
				// The replacement's own host identity is different: it is visible to a
				// native fleet read, never attached to the old chart target.
				if surface == "native" && action != "disable" {
					got := fresh["replacement"]["cpu"]
					if len(got) != 1 || got[0].Value != 99 {
						t.Fatalf("fresh native read did not recover replacement: %+v", fresh)
					}
				}
				if action == "disable" && len(fresh[survivor]["cpu"]) != 1 {
					t.Fatalf("fresh read after enable did not recover: %+v", fresh)
				}
			})
		}
	}
}
