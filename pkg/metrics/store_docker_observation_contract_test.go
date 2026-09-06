package metrics

import (
	"fmt"
	"testing"
	"time"
)

func TestDockerObservationContractSeparatesLegacyAcrossRetainedReads(t *testing.T) {
	for _, family := range []string{"dockerContainer", "docker"} {
		t.Run(family, func(t *testing.T) {
			store, err := NewStore(DefaultConfig(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			ts := time.Now().Add(-10 * time.Minute).Truncate(time.Minute)
			// Seed the old physical schema directly. New public writes must not grant
			// these ambiguous historical rows an observation provenance retroactively.
			for _, id := range []string{"a", "b", "legacy-only"} {
				for _, metric := range []string{"disk", "diskread", "diskwrite"} {
					for _, tier := range []Tier{TierRaw, TierMinute, TierHourly} {
						_, err := store.db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,value,timestamp,tier) VALUES(?,?,?,?,?,?)`, normalizeMetricResourceType(family), id, metric, 99, ts.Unix(), string(tier))
						if err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			for _, id := range []string{"a", "b"} {
				// Exercise the buffered, synchronous and bounded writer entry points.
				store.Write(family, id, "diskread", 0, ts)
				store.WriteBatchSync([]WriteMetric{{ResourceType: family, ResourceID: id, MetricType: "diskwrite", Value: 0, Timestamp: ts, Tier: TierRaw}})
				store.WriteBatchBounded([]WriteMetric{{ResourceType: family, ResourceID: id, MetricType: "disk", Value: 25, Timestamp: ts, Tier: TierRaw}})
				store.Write(family, id, "cpu", 0, ts)
			}
			store.Flush()
			// Both generations roll up independently. Old minute/hourly data must
			// neither replace the corrected zeros nor contaminate their averages.
			if !store.rollupTierWindow(TierRaw, TierMinute, 60, ts.Unix()-60, ts.Unix()+120) {
				t.Fatal("rollup failed")
			}
			coverage, err := store.MaxTimestampsForTier(TierRaw)
			if err != nil {
				t.Fatal(err)
			}
			if !coverage[NormalizedSeriesKey(family, "a", "diskread")].Equal(ts) {
				t.Fatalf("physical coverage mismatch: %+v", coverage)
			}
			for _, step := range []int64{0, 60} {
				t.Run(fmt.Sprintf("step-%d", step), func(t *testing.T) {
					start, end := ts.Add(-2*time.Hour), ts.Add(time.Hour)
					assert := func(series map[string][]MetricPoint, legacy bool) {
						t.Helper()
						for _, metric := range []string{"disk", "diskread", "diskwrite"} {
							points := series[metric]
							if legacy {
								if len(points) > 0 {
									t.Fatalf("legacy %s leaked: %+v", metric, points)
								}
								continue
							}
							want := 0.0
							if metric == "disk" {
								want = 25
							}
							if len(points) != 1 || points[0].Value != want || points[0].Min != want || points[0].Max != want {
								t.Fatalf("%s observation contaminated: %+v", metric, points)
							}
							if _, exists := series[metric+".observed"]; exists {
								t.Fatalf("physical metric leaked: %+v", series)
							}
						}
					}
					all, err := store.QueryAll(family, "a", start, end, step)
					if err != nil {
						t.Fatal(err)
					}
					assert(all, false)
					if len(all["cpu"]) != 1 || all["cpu"][0].Value != 0 {
						t.Fatalf("unrelated zero lost: %+v", all)
					}
					batch, err := store.QueryAllBatch(family, []string{"a", "b", "legacy-only"}, start, end, step)
					if err != nil {
						t.Fatal(err)
					}
					assert(batch["a"], false)
					assert(batch["b"], false)
					assert(batch["legacy-only"], true)
					selected, err := store.QueryMetricTypesBatch(family, []string{"a", "b", "legacy-only"}, []string{"disk", "diskread", "diskwrite"}, start, end, step)
					if err != nil {
						t.Fatal(err)
					}
					assert(selected["a"], false)
					assert(selected["b"], false)
					assert(selected["legacy-only"], true)
					for _, metric := range []string{"disk", "diskread", "diskwrite"} {
						points, err := store.Query(family, "a", metric, start, end, step)
						if err != nil {
							t.Fatal(err)
						}
						if len(points) != 1 || points[0].Value != all[metric][0].Value {
							t.Fatalf("selected %s differs: %+v", metric, points)
						}
						old, err := store.Query(family, "legacy-only", metric, start, end, step)
						if err != nil || len(old) != 0 {
							t.Fatalf("legacy selected %s leaked: %+v %v", metric, old, err)
						}
					}
				})
			}
			var legacyCount int
			err = store.db.QueryRow(`SELECT COUNT(*) FROM metrics WHERE resource_type=? AND metric_type IN ('disk','diskread','diskwrite')`, normalizeMetricResourceType(family)).Scan(&legacyCount)
			if err != nil || legacyCount != 27 {
				t.Fatalf("legacy history was changed: count=%d err=%v", legacyCount, err)
			}
		})
	}
}

func TestDockerObservationContractLeavesOtherFamiliesUnchanged(t *testing.T) {
	store, err := NewStore(DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ts := time.Now().Truncate(time.Second)
	for _, family := range []string{"vm", "ct", "agent", "dockerHost", "disk", "storage", "k8s"} {
		for _, metric := range []string{"disk", "diskread", "diskwrite"} {
			store.WriteWithTier(family, "one", metric, 0, ts, TierRaw)
		}
	}
	store.Flush()
	for _, family := range []string{"vm", "ct", "agent", "dockerHost", "disk", "storage", "k8s"} {
		all, err := store.QueryAll(family, "one", ts.Add(-time.Second), ts.Add(time.Second), 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, metric := range []string{"disk", "diskread", "diskwrite"} {
			if len(all[metric]) != 1 || all[metric][0].Value != 0 {
				t.Fatalf("%s/%s zero changed: %+v", family, metric, all)
			}
			if NormalizedSeriesKey(family, "one", metric).MetricType != metric {
				t.Fatal("unrelated physical key changed")
			}
		}
	}
}
