package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

type scopedDiskHistoryFetcher struct {
	truenas.FixtureFetcher
	points []truenas.TimeSeriesPoint
	err    error
	onRead func()
}

func (f *scopedDiskHistoryFetcher) DiskTemperatureHistory(context.Context, []string, time.Duration) (map[string][]truenas.TimeSeriesPoint, error) {
	if f.onRead != nil {
		f.onRead()
	}
	return map[string][]truenas.TimeSeriesPoint{"sdb": f.points}, f.err
}

func TestTrueNASDiskHistoryScopeAndRevocation(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	points := []truenas.TimeSeriesPoint{{Timestamp: time.Now().UTC().Add(-time.Minute), Value: 33.25}}
	fixtures := truenas.DefaultFixtures()
	fixtures.Disks = []truenas.Disk{{Name: "sdb", Serial: "shared-serial"}}
	fetcherA := &scopedDiskHistoryFetcher{FixtureFetcher: truenas.FixtureFetcher{Snapshot: fixtures}, points: points, err: errors.New("synthetic missing peer series")}
	fetcherB := &scopedDiskHistoryFetcher{FixtureFetcher: truenas.FixtureFetcher{Snapshot: fixtures}, points: []truenas.TimeSeriesPoint{{Timestamp: points[0].Timestamp, Value: 50}}}
	providerA, providerB := truenas.NewLiveProvider(fetcherA), truenas.NewLiveProvider(fetcherB)
	for _, provider := range []*truenas.Provider{providerA, providerB} {
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	poller := NewTrueNASPoller(nil, time.Minute, nil)
	poller.providersByOrg = map[string]map[string]*truenas.Provider{"org-a": {"conn": providerA}, "org-b": {"conn": providerB}}
	for org, want := range map[string]float64{"org-a": 33.25, "org-b": 50} {
		got := poller.PhysicalDiskTemperatureHistory(nil, org, time.Hour)
		if len(got) != 1 || len(got["shared-serial"]) != 1 || got["shared-serial"][0].Value != want || !got["shared-serial"][0].Timestamp.Equal(points[0].Timestamp) {
			t.Fatalf("tenant/partial/native time lost: org %s %+v", org, got)
		}
	}
	if got := poller.PhysicalDiskTemperatureHistory(nil, "org-missing", time.Hour); len(got) != 0 {
		t.Fatalf("unknown tenant received history: %+v", got)
	}
	poller.providersByOrg["org-a"]["second-conn"] = providerB
	if got := poller.PhysicalDiskTemperatureHistory(nil, "org-a", time.Hour); len(got) != 0 {
		t.Fatalf("ambiguous metric target picked an arbitrary connection: %+v", got)
	}
	delete(poller.providersByOrg["org-a"], "second-conn")
	for _, change := range []string{"remove", "replace"} {
		poller.providersByOrg["org-a"]["conn"] = providerA
		fetcherA.onRead = func() {
			poller.mu.Lock()
			defer poller.mu.Unlock()
			if change == "remove" {
				delete(poller.providersByOrg["org-a"], "conn")
			} else {
				poller.providersByOrg["org-a"]["conn"] = providerB
			}
		}
		if got := poller.PhysicalDiskTemperatureHistory(nil, "org-a", time.Hour); len(got) != 0 {
			t.Fatalf("%s while reading served revoked history: %+v", change, got)
		}
	}
	truenas.SetFeatureEnabled(false)
	if got := poller.PhysicalDiskTemperatureHistory(nil, "org-b", time.Hour); len(got) != 0 {
		t.Fatalf("disabled source received history: %+v", got)
	}
}
