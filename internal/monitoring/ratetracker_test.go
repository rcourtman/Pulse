package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func fullCounterSample(at time.Time, diskRead, diskWrite, networkIn, networkOut int64) models.IOMetrics {
	return models.IOMetrics{
		DiskRead:   diskRead,
		DiskWrite:  diskWrite,
		NetworkIn:  networkIn,
		NetworkOut: networkOut,
		Timestamp:  at,
	}
}

func TestRateTrackerConstantRatesUseActualElapsedTime(t *testing.T) {
	for _, interval := range []time.Duration{30 * time.Second, 60 * time.Second, 90 * time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			tracker := NewRateTracker()
			start := time.Unix(1_700_000_000, 0)
			tracker.CalculateRates("guest", fullCounterSample(start, 1_000, 2_000, 3_000, 4_000))

			seconds := int64(interval / time.Second)
			read, write, in, out := tracker.CalculateRates(
				"guest",
				fullCounterSample(
					start.Add(interval),
					1_000+seconds*1_024,
					2_000+seconds*2_048,
					3_000+seconds*4_096,
					4_000+seconds*8_192,
				),
			)
			if read != 1_024 || write != 2_048 || in != 4_096 || out != 8_192 {
				t.Fatalf("rates = (%v, %v, %v, %v), want (1024, 2048, 4096, 8192)", read, write, in, out)
			}
		})
	}
}

func TestRateTrackerBurstUsesAdjacentObservationInterval(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	tracker.CalculateRates("guest", fullCounterSample(start, 0, 0, 0, 0))

	read, _, _, _ := tracker.CalculateRates("guest", fullCounterSample(start.Add(90*time.Second), 90_000, 0, 0, 0))
	if read != 1_000 {
		t.Fatalf("burst rate = %v, want 1000 B/s", read)
	}

	read, _, _, _ = tracker.CalculateRates("guest", fullCounterSample(start.Add(180*time.Second), 90_000, 0, 0, 0))
	if read != 0 {
		t.Fatalf("idle interval rate = %v, want valid zero", read)
	}
}

func TestRateTrackerMissingAndExplicitZeroAreDistinct(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	diskOnly := models.IOCounterPresence{Explicit: true, DiskRead: true}
	tracker.CalculateRates("guest", models.IOMetrics{
		DiskRead:  0,
		Timestamp: start,
		Presence:  diskOnly,
	})

	read, write, in, out := tracker.CalculateRates("guest", models.IOMetrics{
		DiskRead:  0,
		Timestamp: start.Add(60 * time.Second),
		Presence:  diskOnly,
	})
	if read != 0 {
		t.Fatalf("explicit zero rate = %v, want 0", read)
	}
	if write != -1 || in != -1 || out != -1 {
		t.Fatalf("missing rates = (%v, %v, %v), want unknown", write, in, out)
	}
}

func TestRateTrackerPartialSampleKeepsIndependentBaseline(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	tracker.CalculateRates("guest", fullCounterSample(start, 0, 0, 0, 0))

	read, write, in, out := tracker.CalculateRates("guest", models.IOMetrics{
		DiskRead:  60_000,
		Timestamp: start.Add(60 * time.Second),
		Presence: models.IOCounterPresence{
			Explicit: true,
			DiskRead: true,
		},
	})
	if read != 1_000 || write != -1 || in != -1 || out != -1 {
		t.Fatalf("partial rates = (%v, %v, %v, %v)", read, write, in, out)
	}

	_, write, _, _ = tracker.CalculateRates("guest", models.IOMetrics{
		DiskWrite: 180_000,
		Timestamp: start.Add(90 * time.Second),
		Presence: models.IOCounterPresence{
			Explicit:  true,
			DiskWrite: true,
		},
	})
	if write != 2_000 {
		t.Fatalf("disk write rate = %v, want 2000 over its 90s observation gap", write)
	}
}

func TestRateTrackerUsesEachCounterReceiptTimeForPartialSources(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	tracker.CalculateRates("guest", fullCounterSample(start, 0, 0, 0, 0))

	read, write, _, _ := tracker.CalculateRates("guest", models.IOMetrics{
		DiskRead:  30_000,
		DiskWrite: 180_000,
		Timestamp: start.Add(90 * time.Second),
		Presence: models.IOCounterPresence{
			Explicit:  true,
			DiskRead:  true,
			DiskWrite: true,
		},
		ObservedAt: models.IOCounterObservationTimes{
			DiskRead:  start.Add(30 * time.Second),
			DiskWrite: start.Add(90 * time.Second),
		},
	})
	if read != 1_000 || write != 2_000 {
		t.Fatalf("rates = (%v, %v), want per-counter receipt rates (1000, 2000)", read, write)
	}
}

func TestRateTrackerResetOrWrapRebasesCounterEpoch(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	tracker.CalculateRates("guest", fullCounterSample(start, math.MaxInt64-1_000, 50_000, 10_000, 20_000))

	read, write, in, out := tracker.CalculateRates(
		"guest",
		fullCounterSample(start.Add(30*time.Second), 500, 1_000, 100, 200),
	)
	if read != 0 || write != 0 || in != 0 || out != 0 {
		t.Fatalf("reset rates = (%v, %v, %v, %v), want zeros", read, write, in, out)
	}

	read, _, _, _ = tracker.CalculateRates("guest", fullCounterSample(start.Add(60*time.Second), 30_500, 1_000, 100, 200))
	if read != 1_000 {
		t.Fatalf("post-reset rate = %v, want 1000", read)
	}
}

func TestRateTrackerUptimeRollbackRebasesEvenWhenCounterSurpassesOldValue(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	beforeRestart := fullCounterSample(start, 1_000, 2_000, 3_000, 4_000)
	beforeRestart.SourceUptime = 10_000
	tracker.CalculateRates("guest", beforeRestart)

	afterRestart := fullCounterSample(start.Add(90*time.Second), 91_000, 182_000, 273_000, 364_000)
	afterRestart.SourceUptime = 30
	read, write, in, out := tracker.CalculateRates("guest", afterRestart)
	if read != -1 || write != -1 || in != -1 || out != -1 {
		t.Fatalf("first sample in restarted epoch = (%v, %v, %v, %v), want unknown", read, write, in, out)
	}

	next := fullCounterSample(start.Add(120*time.Second), 121_000, 242_000, 363_000, 484_000)
	next.SourceUptime = 60
	read, write, in, out = tracker.CalculateRates("guest", next)
	if read != 1_000 || write != 2_000 || in != 3_000 || out != 4_000 {
		t.Fatalf("post-restart rates = (%v, %v, %v, %v), want (1000, 2000, 3000, 4000)", read, write, in, out)
	}
}

func TestRateTrackerRejectsOutOfOrderSamplesWithoutChangingBaseline(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	tracker.CalculateRates("guest", fullCounterSample(start, 0, 0, 0, 0))
	tracker.CalculateRates("guest", fullCounterSample(start.Add(60*time.Second), 60_000, 0, 0, 0))

	read, _, _, _ := tracker.CalculateRates("guest", fullCounterSample(start.Add(30*time.Second), 90_000, 0, 0, 0))
	if read != -1 {
		t.Fatalf("out-of-order rate = %v, want unknown", read)
	}
	read, _, _, _ = tracker.CalculateRates("guest", fullCounterSample(start.Add(90*time.Second), 90_000, 0, 0, 0))
	if read != 1_000 {
		t.Fatalf("rate after rejected sample = %v, want 1000", read)
	}
}

func TestRateTrackerCleanupUsesLatestSampleEvenWhenIdleOrPartial(t *testing.T) {
	tracker := NewRateTracker()
	now := time.Now()
	tracker.CalculateRates("idle", fullCounterSample(now.Add(-2*time.Hour), 100, 100, 100, 100))
	tracker.CalculateRates("idle", models.IOMetrics{
		Timestamp: now,
		Presence:  models.IOCounterPresence{Explicit: true},
	})
	tracker.CalculateRates("stale", fullCounterSample(now.Add(-2*time.Hour), 100, 100, 100, 100))

	if removed := tracker.Cleanup(now.Add(-time.Hour)); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, ok := tracker.history["idle"]; !ok {
		t.Fatal("idle resource was removed despite a recent sample")
	}
}

func TestRateTrackerDiskBusyUsesElapsedMilliseconds(t *testing.T) {
	tracker := NewRateTracker()
	start := time.Unix(1_700_000_000, 0)
	tracker.CalculateRatesWithBusy("disk", models.IOMetrics{DiskBusy: 100, Timestamp: start})
	_, _, busy, _, _ := tracker.CalculateRatesWithBusy("disk", models.IOMetrics{
		DiskBusy:  15_100,
		Timestamp: start.Add(30 * time.Second),
	})
	if busy != 50 {
		t.Fatalf("busy = %v, want 50%%", busy)
	}
}

func TestRateTrackerClearRestoresFirstSampleUnknown(t *testing.T) {
	tracker := NewRateTracker()
	now := time.Now()
	tracker.CalculateRates("guest", fullCounterSample(now, 1, 2, 3, 4))
	tracker.Clear()
	read, write, in, out := tracker.CalculateRates("guest", fullCounterSample(now.Add(time.Second), 2, 3, 4, 5))
	if read != -1 || write != -1 || in != -1 || out != -1 {
		t.Fatalf("first rates after clear = (%v, %v, %v, %v)", read, write, in, out)
	}
}
