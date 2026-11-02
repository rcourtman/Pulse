package monitoring

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

type targetedTrueNASHistoryFetcher struct {
	controllableTrueNASHistoryFetcher
	reads   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (f *targetedTrueNASHistoryFetcher) SystemMetricHistory(ctx context.Context, duration time.Duration) (*truenas.SystemMetricHistory, error) {
	f.reads.Add(1)
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	return f.controllableTrueNASHistoryFetcher.SystemMetricHistory(ctx, duration)
}

func TestTrueNASHostHistoryTargetAndLifetime(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	newProvider := func(connection string) (*truenas.Provider, *targetedTrueNASHistoryFetcher) {
		fixture := truenas.DefaultFixtures()
		fixture.System.Hostname = "same-name"
		f := &targetedTrueNASHistoryFetcher{controllableTrueNASHistoryFetcher: controllableTrueNASHistoryFetcher{snapshot: &fixture, systemHistory: &truenas.SystemMetricHistory{DiskReadRate: []truenas.TimeSeriesPoint{{Timestamp: time.Now().Add(-time.Minute), Value: 0}}}}}
		p := truenas.NewLiveProviderForConnection(f, connection)
		if err := p.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		return p, f
	}
	target, targetFetcher := newProvider("target")
	other, otherFetcher := newProvider("other")
	poller := NewTrueNASPoller(nil, time.Minute, nil)
	poller.providersByOrg["org-a"] = map[string]*truenas.Provider{"target": target, "other": other}
	for _, test := range []struct{ org, kind, id string }{{"org-b", "agent", "target"}, {"org-a", "disk", "target"}, {"org-a", "agent", "missing"}, {"org-a", "agent", "same-name"}, {"org-a", "agent", ""}} {
		if result := poller.GuestMetricHistoryForResource(nil, test.org, test.kind, test.id, time.Hour); len(result) != 0 || targetFetcher.reads.Load() != 0 || otherFetcher.reads.Load() != 0 {
			t.Fatalf("nonmatching target reached native history: %+v", test)
		}
	}
	result := poller.GuestMetricHistoryForResource(nil, "org-a", "agent", "target", time.Hour)
	if len(result["diskread"]) != 1 || result["diskread"][0].Value != 0 || targetFetcher.reads.Load() != 1 || otherFetcher.reads.Load() != 0 {
		t.Fatalf("exact target/zero lost or other same-name host read: %+v", result)
	}
	for _, action := range []string{"remove", "replace", "disable"} {
		t.Run(action, func(t *testing.T) {
			p, f := newProvider("target")
			f.entered, f.release = make(chan struct{}), make(chan struct{})
			poller.mu.Lock()
			poller.providersByOrg["org-a"]["target"] = p
			poller.mu.Unlock()
			truenas.SetFeatureEnabled(true)
			done := make(chan map[string][]MetricPoint, 1)
			go func() { done <- poller.GuestMetricHistoryForResource(nil, "org-a", "agent", "target", time.Hour) }()
			<-f.entered
			poller.mu.Lock()
			switch action {
			case "remove":
				delete(poller.providersByOrg["org-a"], "target")
			case "replace":
				poller.providersByOrg["org-a"]["target"] = target
			case "disable":
				truenas.SetFeatureEnabled(false)
			}
			poller.mu.Unlock()
			close(f.release)
			if result := <-done; len(result) != 0 {
				t.Fatalf("withdrawn provider returned native history: %+v", result)
			}
		})
	}
}
