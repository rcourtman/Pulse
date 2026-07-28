package hostmetrics

import (
	"context"
	"math"
	"testing"
	"time"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
)

// Issue #1648: a 1-second spot sample once per report interval overstates CPU
// on mostly-idle guests. CPU usage must be computed as the busy/total delta of
// the cumulative counters across the whole gap between collections, with the
// spot sample only as a fallback for the first collection and counter resets.
func TestIssue1648CPUUsageIntervalDelta(t *testing.T) {
	origCPUPercent := cpuPercent
	origCPUTimes := cpuTimes
	t.Cleanup(func() {
		cpuPercent = origCPUPercent
		cpuTimes = origCPUTimes
	})

	// What a 1-second window would read on the idle 2-vCPU guest from the
	// report: a short burst makes the instant view claim ~28% CPU.
	const spotReading = 28.4
	spotCalls := 0
	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		spotCalls++
		return []float64{spotReading}, nil
	}

	// Cumulative counter samples 30s apart on a 2-vCPU guest (60 CPU-seconds
	// per interval). Between samples[0] and samples[1] only 3 of those 60
	// seconds are busy, so the interval average is 5%. Guest time advances
	// inside user and must not count as busy on top of it.
	samples := []gocpu.TimesStat{
		{CPU: "cpu-total", User: 120, Nice: 1, System: 60, Idle: 5000, Iowait: 12, Irq: 2, Softirq: 3, Steal: 1, Guest: 10},
		{CPU: "cpu-total", User: 121.5, Nice: 1.25, System: 61, Idle: 5056.5, Iowait: 12.5, Irq: 2, Softirq: 3, Steal: 1.25, Guest: 10.5},
		// Counters restart far below the previous sample (reboot or migration).
		{CPU: "cpu-total", User: 1, System: 0.5, Idle: 20},
		// One interval after the reset: 1 busy second out of 60.
		{CPU: "cpu-total", User: 1.5, System: 1, Idle: 78, Iowait: 1},
	}
	sampleIdx := 0
	cpuTimes = func(ctx context.Context, percpu bool) ([]gocpu.TimesStat, error) {
		s := samples[sampleIdx]
		sampleIdx++
		return []gocpu.TimesStat{s}, nil
	}

	tracker := &cpuUsageTracker{}
	ctx := context.Background()

	// First collection has no baseline and must fall back to the spot sample.
	usage, err := tracker.collect(ctx)
	if err != nil {
		t.Fatalf("first collect: %v", err)
	}
	if usage != spotReading {
		t.Fatalf("first collect should use spot fallback, got %v want %v", usage, spotReading)
	}
	if spotCalls != 1 {
		t.Fatalf("expected 1 spot sample after first collect, got %d", spotCalls)
	}

	// Second collection must average across the interval, not re-run the
	// blocking spot sample that would have reported ~28%.
	usage, err = tracker.collect(ctx)
	if err != nil {
		t.Fatalf("second collect: %v", err)
	}
	if math.Abs(usage-5.0) > 1e-9 {
		t.Fatalf("interval delta usage = %v, want 5.0 (spot window would claim %v)", usage, spotReading)
	}
	if spotCalls != 1 {
		t.Fatalf("interval delta must not invoke the blocking spot sample, got %d calls", spotCalls)
	}

	// A counter reset makes the delta negative; that collection falls back to
	// the spot sample instead of reporting garbage.
	usage, err = tracker.collect(ctx)
	if err != nil {
		t.Fatalf("post-reset collect: %v", err)
	}
	if usage != spotReading {
		t.Fatalf("counter reset should use spot fallback, got %v want %v", usage, spotReading)
	}
	if spotCalls != 2 {
		t.Fatalf("expected 2 spot samples after counter reset, got %d", spotCalls)
	}

	// The reset sample rebaselines the tracker, so the next collection
	// resumes interval averaging: 1 busy CPU-second out of 60.
	usage, err = tracker.collect(ctx)
	if err != nil {
		t.Fatalf("post-rebaseline collect: %v", err)
	}
	if math.Abs(usage-100.0/60.0) > 1e-9 {
		t.Fatalf("post-rebaseline usage = %v, want %v", usage, 100.0/60.0)
	}
	if spotCalls != 2 {
		t.Fatalf("rebaselined delta must not invoke the spot sample, got %d calls", spotCalls)
	}
}
