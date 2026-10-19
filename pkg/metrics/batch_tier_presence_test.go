package metrics

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
)

// Batch presence is an optimization, not evidence of complete coverage. Use
// the unpruned single-resource reader as an independent reconciliation oracle,
// then change the inventory between calls with the same parameter-count shape.
func TestRetainedBatchTierPresenceReadsCurrentScopedSnapshot(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy_%t", legacy), func(t *testing.T) {
			db := newPlanTestDB(t)
			db.SetMaxOpenConns(1)
			store := &Store{db: pdb.Wrap(db, "batch-tier-presence")}
			if !legacy {
				if err := store.migrateClusteredMetrics(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			base := time.Unix(2_000_000_040, 0).UTC().Truncate(time.Minute)
			start, end := base.Add(-4*time.Hour), base.Add(4*time.Minute)
			ids := []string{"a", "b", "missing"}
			insert := func(family, id, metric string, tier Tier, at time.Time, value float64) {
				t.Helper()
				_, err := db.Exec(`INSERT INTO metrics
					(resource_type, resource_id, metric_type, tier, timestamp, value)
					VALUES (?, ?, ?, ?, ?, ?)`, family, id, metric, tier, at.Unix(), value)
				if err != nil {
					t.Fatal(err)
				}
			}
			check := func() map[string]map[string][]MetricPoint {
				t.Helper()
				batch, err := store.QueryMetricTypesBatch("node", ids, []string{"cpu"}, start, end, 0)
				if err != nil {
					t.Fatal(err)
				}
				values, err := store.QueryMetricValuesBatch("node", ids, []string{"cpu"}, start, end, 0)
				if err != nil {
					t.Fatal(err)
				}
				assertMetricValueProjection(t, batch, values)
				for _, id := range ids {
					single, err := store.Query("node", id, "cpu", start, end, 0)
					if err != nil {
						t.Fatal(err)
					}
					if len(single) == 0 {
						if _, ok := batch[id]; ok {
							t.Fatalf("invented absent series %s: %+v", id, batch[id])
						}
					} else if !reflect.DeepEqual(single, batch[id]["cpu"]) {
						t.Fatalf("%s differs from unpruned reader: %+v != %+v", id, batch[id]["cpu"], single)
					}
				}
				return batch
			}
			if got := check(); len(got) != 0 {
				t.Fatalf("empty inventory: %+v", got)
			}
			// These preferred-tier points are outside each part of this read's
			// scope, and cannot prevent pruning the minute/hourly branches.
			insert("vm", "a", "cpu", TierMinute, base, 901)
			insert("node", "other", "cpu", TierMinute, base, 902)
			insert("node", "a", "memory", TierMinute, base, 903)
			insert("node", "a", "cpu", TierHourly, start.Add(-time.Hour), 904)
			insert("node", "a", "cpu", TierMinute, end.Add(time.Minute), 905)
			insert("node", "a", "cpu", TierRaw, base.Add(20*time.Second), 999)
			insert("node", "a", "cpu", TierRaw, base.Add(3*time.Minute), 30)
			insert("node", "b", "cpu", TierRaw, base.Add(20*time.Second), 60)
			if got := check(); len(got["a"]["cpu"]) != 2 || got["a"]["cpu"][0].Value != 999 {
				t.Fatalf("single-tier fallback: %+v", got)
			}
			// Prove this inventory selected the direct read, not merely that
			// both the optimized and old UNION happen to return equal data.
			for _, valuesOnly := range []bool{false, true} {
				shape := retainedQueryShape{resources: len(ids), metrics: 1, valuesOnly: valuesOnly, clustered: !legacy}
				shape.tiers[0] = TierRaw
				query := store.readQueryShapes[shape]
				if query == "" || strings.Contains(query, "UNION") {
					t.Fatalf("absent tiers were not pruned: %q", query)
				}
			}
			// The preferred bucket supersedes only a's overlapping raw point.
			// b's raw point and a's newer raw tail must remain, even though the
			// preferred tier is now present somewhere in the batch.
			insert("node", "a", "cpu", TierMinute, base, 10)
			insert("node", "b", "cpu", TierHourly, base.Add(-time.Hour), 5)
			got := check()
			if len(got["a"]["cpu"]) != 2 || got["a"]["cpu"][0].Value != 10 || got["a"]["cpu"][1].Value != 30 || len(got["b"]["cpu"]) != 2 {
				t.Fatalf("partial per-resource coverage was lost: %+v", got)
			}
			// Removed tiers cannot remain cached either. Reuse the query shape
			// after removing the in-scope preferred data, then change the IDs.
			if _, err := db.Exec(`DELETE FROM metrics WHERE resource_type = 'node' AND resource_id IN ('a','b') AND metric_type = 'cpu' AND tier != 'raw'`); err != nil {
				t.Fatal(err)
			}
			if got := check(); got["a"]["cpu"][0].Value != 999 {
				t.Fatalf("stale preferred presence: %+v", got)
			}
			ids = []string{"c", "d", "missing"}
			if got := check(); len(got) != 0 {
				t.Fatalf("cached inventory crossed identities: %+v", got)
			}
		})
	}
}
