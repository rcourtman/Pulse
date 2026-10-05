package truenas

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type diskHistoryResultFetcher struct {
	FixtureFetcher
	history      map[string][]TimeSeriesPoint
	err          error
	beforeReturn func()
	queried      []string
}

func (f *diskHistoryResultFetcher) DiskTemperatureHistory(_ context.Context, ids []string, _ time.Duration) (map[string][]TimeSeriesPoint, error) {
	f.queried = append([]string(nil), ids...)
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	return f.history, f.err
}

func TestDiskHistoryProviderPreservesPartialAndCurrentIdentity(t *testing.T) {
	now := time.Now().UTC()
	points := []TimeSeriesPoint{{Timestamp: now.Add(-time.Minute), Value: 32.25}}
	missing := errors.New("synthetic missing-series error")
	fixtures := DefaultFixtures()
	fixtures.Disks = []Disk{{Name: "sdb", Serial: "CURRENT-SERIAL"}}
	fetcher := &diskHistoryResultFetcher{FixtureFetcher: FixtureFetcher{Snapshot: fixtures}, history: map[string][]TimeSeriesPoint{"sdb": points, "unlisted": points}, err: missing}
	provider := NewLiveProvider(fetcher)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := provider.PhysicalDiskTemperatureHistory(context.Background(), time.Hour)
	if !errors.Is(err, missing) || len(got) != 1 || !reflect.DeepEqual(got["CURRENT-SERIAL"], points) || !reflect.DeepEqual(fetcher.queried, []string{"sdb"}) {
		t.Fatalf("canonical partial identity/error lost: %+v err %v query %+v", got, err, fetcher.queried)
	}
	got["CURRENT-SERIAL"][0].Value = 999
	if points[0].Value != 32.25 {
		t.Fatal("provider history aliases fetcher response")
	}
	fetcher.beforeReturn = func() {
		fetcher.Snapshot.Disks = []Disk{{Name: "sdb", Serial: "REPLACEMENT-SERIAL"}}
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := provider.PhysicalDiskTemperatureHistory(context.Background(), time.Hour); len(got) != 0 || !errors.Is(err, missing) {
		t.Fatalf("in-flight replaced disk served old samples: %+v err %v", got, err)
	}
}

func TestDiskHistoryProviderRejectsAmbiguousAliases(t *testing.T) {
	for _, disks := range [][]Disk{
		{{Name: "sda", Serial: "duplicate"}, {Name: "sdb", Serial: "duplicate"}},
		{{Name: "sda", Serial: "serial-a"}, {Name: "sda", Serial: "serial-b"}},
	} {
		fixtures := DefaultFixtures()
		fixtures.Disks = disks
		fetcher := &diskHistoryResultFetcher{FixtureFetcher: FixtureFetcher{Snapshot: fixtures}, history: map[string][]TimeSeriesPoint{"sda": {{Timestamp: time.Now(), Value: 30}}, "sdb": {{Timestamp: time.Now(), Value: 50}}}}
		provider := NewLiveProvider(fetcher)
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got, err := provider.PhysicalDiskTemperatureHistory(context.Background(), time.Hour); len(got) != 0 || err != nil {
			t.Fatalf("ambiguous inventory attached arbitrary History: %+v err %v", got, err)
		}
	}
}
