package metrics

import (
	"fmt"
	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
	"reflect"
	"testing"
	"time"
)

func TestStoreRetainedTierCoverage(t *testing.T) {
	db := newPlanTestDB(t)
	store := &Store{db: pdb.Wrap(db, "tier-coverage-test")}
	base := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	insert := func(id, metric string, tier Tier, offset time.Duration, value, low, high float64) {
		t.Helper()
		_, err := db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value,min_value,max_value) VALUES ('node',?,?,?,?,?,?,?)`, id, metric, tier, base.Add(offset).Unix(), value, low, high)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Raw observations before, within and after aggregate buckets. The raw
	// overlap deliberately differs so double-counting is visible in the mean.
	insert("a", "cpu", TierMinute, 0, 10, 2, 90)
	insert("a", "cpu", TierMinute, 2*time.Minute, 30, 25, 40)
	insert("a", "cpu", TierRaw, -time.Minute, 5, 5, 5)
	insert("a", "cpu", TierRaw, 20*time.Second, 999, 999, 999)
	insert("a", "cpu", TierRaw, time.Minute, 20, 20, 20)
	insert("a", "cpu", TierRaw, 3*time.Minute, 50, 50, 50)
	insert("a", "memory", TierRaw, 3*time.Minute, 88, 88, 88)
	// The older hourly fallback remains useful, but the overlapping hour is
	// indivisible and must not be mixed with the preferred minute/raw evidence.
	insert("a", "cpu", TierHourly, -2*time.Hour, 7, 1, 9)
	insert("a", "cpu", TierHourly, 0, 777, 777, 777)
	insert("b", "cpu", TierRaw, 20*time.Second, 60, 60, 60)
	insert("b", "memory", TierMinute, 0, 70, 65, 75)
	insert("b", "memory", TierRaw, 3*time.Minute, 80, 80, 80)
	// Same ID in another resource family cannot shadow or contribute evidence.
	_, err := db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value) VALUES ('vm','a','cpu','minute',?,1234)`, base.Add(time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	start, end := base.Add(-23*time.Hour), base.Add(4*time.Minute)
	want := []MetricPoint{
		{base.Add(-2 * time.Hour), 7, 1, 9}, {base.Add(-time.Minute), 5, 5, 5},
		{base, 10, 2, 90}, {base.Add(time.Minute), 20, 20, 20},
		{base.Add(2 * time.Minute), 30, 25, 40}, {base.Add(3 * time.Minute), 50, 50, 50},
	}
	for _, step := range []int64{0, 120} {
		t.Run(fmt.Sprintf("step_%d", step), func(t *testing.T) {
			batch, err := store.QueryAllBatch("node", []string{"a", "b", "a", "missing"}, start, end, step)
			if err != nil {
				t.Fatal(err)
			}
			filtered, err := store.QueryMetricTypesBatch("node", []string{"a", "b"}, []string{"cpu"}, start, end, step)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"a", "b"} {
				all, err := store.QueryAll("node", id, start, end, step)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(all, batch[id]) {
					t.Fatalf("batch differs for %s: %v != %v", id, batch[id], all)
				}
				for _, metric := range []string{"cpu", "memory"} {
					single, err := store.Query("node", id, metric, start, end, step)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(single, all[metric]) {
						t.Fatalf("single differs for %s/%s", id, metric)
					}
				}
				if len(filtered[id]) != 1 || !reflect.DeepEqual(filtered[id]["cpu"], all["cpu"]) {
					t.Fatalf("filtered mismatch: %+v", filtered)
				}
			}
			got := batch["a"]["cpu"]
			expected := want
			if step == 120 {
				expected = []MetricPoint{{base.Add(-119 * time.Minute), 7, 1, 9}, {base.Add(-time.Minute), 5, 5, 5}, {base.Add(time.Minute), 15, 2, 90}, {base.Add(3 * time.Minute), 40, 25, 50}}
			}
			if len(got) != len(expected) {
				t.Fatalf("got %+v\nwant %+v", got, expected)
			}
			for i, p := range got {
				want := expected[i]
				if !p.Timestamp.Equal(want.Timestamp) || p.Value != want.Value || p.Min != want.Min || p.Max != want.Max {
					t.Fatalf("point %d: got %+v, want %+v", i, p, want)
				}
			}
			if _, ok := batch["missing"]; ok {
				t.Fatal("invented missing resource")
			}
		})
	}
	// On a short raw-preferred window the real raw sample wins, rather than
	// its encompassing minute/hour buckets. With no raw in a minute, retain it.
	short, err := store.Query("node", "a", "cpu", base, base.Add(4*time.Minute), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(short) != 4 || short[0].Value != 999 || short[2].Value != 30 {
		t.Fatalf("raw precedence: %+v", short)
	}
	// An aggregate outside the requested timestamp window cannot suppress a
	// raw point inside it, even when they share a nominal retention bucket.
	clipped, err := store.Query("node", "a", "cpu", base.Add(10*time.Second), base.Add(20*time.Second), 0)
	if err != nil || len(clipped) != 1 || clipped[0].Value != 999 {
		t.Fatalf("clipped boundary: %+v, %v", clipped, err)
	}
}

func TestStoreRetainedDailyAndHourlyCoverage(t *testing.T) {
	db := newPlanTestDB(t)
	store := &Store{db: pdb.Wrap(db, "tier-coverage-test")}
	base := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		tier   Tier
		offset time.Duration
		value  float64
	}{
		{TierDaily, -24 * time.Hour, 10}, {TierHourly, -23 * time.Hour, 999},
		{TierHourly, 0, 20}, {TierMinute, 30 * time.Minute, 999},
		{TierMinute, time.Hour, 30}, {TierRaw, time.Hour + 20*time.Second, 999},
		{TierRaw, time.Hour + time.Minute, 40},
	} {
		_, err := db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value) VALUES ('node','a','cpu',?,?,?)`, row.tier, base.Add(row.offset).Unix(), row.value)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.Query("node", "a", "cpu", base.Add(-30*24*time.Hour), base.Add(2*time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("coverage: %+v", got)
	}
	for i, p := range got {
		if p.Value != float64((i+1)*10) {
			t.Fatalf("precedence: %+v", got)
		}
	}
}

func TestStoreRollupPreservesRetainedExtrema(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%v", legacy), func(t *testing.T) {
			db := newPlanTestDB(t)
			store := &Store{db: pdb.Wrap(db, "tier-coverage-test")}
			base := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
			_, err := db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value,min_value,max_value) VALUES ('node','a','cpu','minute',?,10,1,99),('node','a','cpu','minute',?,20,5,35)`, base.Unix(), base.Add(time.Minute).Unix())
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				store.rollupCandidate("node", "a", "cpu", TierMinute, TierHourly, 3600, base.Unix(), base.Add(time.Hour).Unix())
			} else if !store.rollupTierWindow(TierMinute, TierHourly, 3600, base.Unix(), base.Add(time.Hour).Unix()) {
				t.Fatal("rollup failed")
			}
			var value, low, high float64
			if err := db.QueryRow(`SELECT value,min_value,max_value FROM metrics WHERE tier='hourly'`).Scan(&value, &low, &high); err != nil {
				t.Fatal(err)
			}
			if value != 15 || low != 1 || high != 99 {
				t.Fatalf("lost retained extrema: %v %v %v", value, low, high)
			}
		})
	}
}
