package metrics

import (
	"fmt"
	"testing"
	"time"

	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
)

func TestStoreExactWindowBucketEligibilityAndBoundaries(t *testing.T) {
	db := newPlanTestDB(t)
	store := &Store{db: pdb.Wrap(db, "exact-window-test")}

	dayStart := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	insert := func(id, metric string, tier Tier, ts time.Time, value, low, high float64) {
		t.Helper()
		_, err := db.Exec(
			`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value,min_value,max_value) VALUES ('node',?,?,?,?,?,?,?)`,
			id, metric, tier, ts.Unix(), value, low, high,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Only daily rollup survives for node-daily.
	insert("node-daily", "cpu", TierDaily, dayStart, 55, 10, 99)
	insert("node-daily", "cpu", TierDaily, dayStart.Add(24*time.Hour), 60, 15, 95)

	// 2-hour zoom on daily-only data: neither midnight-aligned nor midday should return the daily aggregate.
	for _, tc := range []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"midnight_2h", dayStart, dayStart.Add(2 * time.Hour)},
		{"midday_2h", dayStart.Add(12 * time.Hour), dayStart.Add(14 * time.Hour)},
		{"partial_day_23h59m", dayStart, dayStart.Add(24*time.Hour - time.Second)},
	} {
		for _, step := range []int64{0, 60} {
			t.Run(fmt.Sprintf("%s_step_%d", tc.name, step), func(t *testing.T) {
				pts, err := store.QueryExact("node", "node-daily", "cpu", tc.start, tc.end, step)
				if err != nil {
					t.Fatalf("QueryExact error: %v", err)
				}
				if len(pts) != 0 {
					t.Fatalf("expected 0 points for %s (daily-only), got %+v", tc.name, pts)
				}
			})
		}
	}

	// 48-hour custom zoom covering complete UTC days includes the complete daily buckets.
	for _, step := range []int64{0, 3600} {
		t.Run(fmt.Sprintf("full_48h_daily_step_%d", step), func(t *testing.T) {
			pts, err := store.QueryExact("node", "node-daily", "cpu", dayStart, dayStart.Add(48*time.Hour), step)
			if err != nil {
				t.Fatalf("QueryExact error: %v", err)
			}
			if len(pts) != 2 {
				t.Fatalf("expected 2 daily points, got %+v", pts)
			}
		})
	}

	// Fractional millisecond boundaries and mixed-tier eligibility.
	base := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	insert("node-mixed", "cpu", TierHourly, base, 999, 999, 999)                            // covers [12:00, 13:00)
	insert("node-mixed", "cpu", TierMinute, base.Add(time.Minute), 40, 35, 45)              // covers [12:01, 12:02)
	insert("node-mixed", "cpu", TierRaw, base, 11, 11, 11)                                  // at 12:00:00.000
	insert("node-mixed", "cpu", TierRaw, base.Add(time.Second), 22, 22, 22)                 // at 12:00:01.000
	insert("node-mixed", "cpu", TierRaw, base.Add(30*time.Second), 33, 33, 33)              // at 12:00:30.000
	insert("node-mixed", "cpu", TierRaw, base.Add(2*time.Minute+5*time.Second), 55, 55, 55) // at 12:02:05.000
	insert("node-mixed", "cpu", TierRaw, base.Add(2*time.Minute+6*time.Second), 66, 66, 66) // at 12:02:06.000

	// Window [12:00:00.750, 12:02:05.250]:
	// - Raw at 12:00:00.000 MUST be excluded (ceil startSec = 12:00:01).
	// - Raw at 12:02:06.000 MUST be excluded (floor endSec = 12:02:05).
	// - Hourly at 12:00:00 MUST be excluded AND MUST NOT suppress raw/minute rows.
	// - Minute at 12:01:00 fits completely inside [12:00:01, 12:02:05] -> included.
	fracStart := base.Add(750 * time.Millisecond)
	fracEnd := base.Add(2*time.Minute + 5*time.Second + 250*time.Millisecond)
	pts, err := store.QueryExact("node", "node-mixed", "cpu", fracStart, fracEnd, 0)
	if err != nil {
		t.Fatalf("QueryExact fractional error: %v", err)
	}
	wantTimes := []time.Time{
		base.Add(time.Second),
		base.Add(30 * time.Second),
		base.Add(time.Minute),
		base.Add(2*time.Minute + 5*time.Second),
	}
	if len(pts) != len(wantTimes) {
		t.Fatalf("got %d points %+v, want %d", len(pts), pts, len(wantTimes))
	}
	for i, wt := range wantTimes {
		if !pts[i].Timestamp.Equal(wt) {
			t.Fatalf("point %d ts = %v, want %v", i, pts[i].Timestamp, wt)
		}
	}

	// Partial first and last display buckets when stepSecs > 1 must clamp their
	// representative timestamp inside [start, end] without dropping values/extrema.
	bucketed, err := store.QueryExact("node", "node-mixed", "cpu", fracStart, fracEnd, 60)
	if err != nil {
		t.Fatalf("QueryExact bucketed error: %v", err)
	}
	if len(bucketed) != 3 {
		t.Fatalf("expected 3 buckets, got %+v", bucketed)
	}
	for i, p := range bucketed {
		if p.Timestamp.Before(fracStart) || p.Timestamp.After(fracEnd) {
			t.Fatalf("bucket %d timestamp %v outside [%v, %v]", i, p.Timestamp, fracStart, fracEnd)
		}
	}
	// Last bucket contains only 55 at 12:02:05; epoch midpoint would be 12:02:30 (> fracEnd),
	// so it must clamp to 12:02:05 and preserve value 55.
	last := bucketed[len(bucketed)-1]
	if last.Value != 55 || last.Min != 55 || last.Max != 55 {
		t.Fatalf("last bucket lost value/extrema: %+v", last)
	}
	if !last.Timestamp.Equal(base.Add(2*time.Minute + 5*time.Second)) {
		t.Fatalf("last bucket timestamp = %v, want clamped %v", last.Timestamp, base.Add(2*time.Minute+5*time.Second))
	}

	// Hourly bucket boundary: [12:00:00.000, 12:59:59.999] excludes hourly at 12:00:00;
	// [12:00:00.000, 13:00:00.000] includes it.
	insert("node-hourly", "cpu", TierHourly, base, 77, 50, 90)
	excluded, err := store.QueryExact("node", "node-hourly", "cpu", base, base.Add(time.Hour-time.Millisecond), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(excluded) != 0 {
		t.Fatalf("expected hourly bucket excluded at 59m59.999s, got %+v", excluded)
	}
	included, err := store.QueryExact("node", "node-hourly", "cpu", base, base.Add(time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(included) != 1 || included[0].Value != 77 {
		t.Fatalf("expected hourly bucket included at exact 1h, got %+v", included)
	}

	// Sub-millisecond nanosecond ceiling: 12:00:00.000000001 must exclude a raw row at 12:00:00.
	subMsStart := base.Add(time.Nanosecond)
	subMsEnd := base.Add(time.Second)
	subMsPts, err := store.QueryExact("node", "node-mixed", "cpu", subMsStart, subMsEnd, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(subMsPts) != 1 || !subMsPts[0].Timestamp.Equal(base.Add(time.Second)) {
		t.Fatalf("sub-millisecond start admitted out-of-window row: got %+v", subMsPts)
	}
}
