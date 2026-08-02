package truenas

import (
	"testing"
	"time"
)

// Issue #1668: TrueNAS reserves most RAM for the ZFS ARC, and the kernel's
// MemAvailable does not count ARC even though it shrinks under pressure, so
// memory derived from total-available read ~95% used on every ZFS system.
// ARC must be treated as reclaimable cache.

func TestParseSystemTelemetryReadsARCSize(t *testing.T) {
	telemetry := parseSystemTelemetry(map[string]any{
		"memory": map[string]any{
			"physical_memory_total":     int64(68719476736),
			"physical_memory_available": int64(3435973836),
			"arc_size":                  int64(51539607552),
		},
	}, 2, time.Now())

	if telemetry.MemoryTotalBytes != 68719476736 {
		t.Fatalf("MemoryTotalBytes = %d", telemetry.MemoryTotalBytes)
	}
	if telemetry.ARCSizeBytes != 51539607552 {
		t.Fatalf("ARCSizeBytes = %d, want 51539607552", telemetry.ARCSizeBytes)
	}
}

func TestTrueNASEffectiveMemoryUsedExcludesARC(t *testing.T) {
	system := SystemInfo{
		MemoryTotalBytes:     64 << 30,
		MemoryAvailableBytes: 3 << 30,
		ARCSizeBytes:         48 << 30,
	}

	used := trueNASEffectiveMemoryUsed(system)
	want := int64(64<<30 - 3<<30 - 48<<30)
	if used != want {
		t.Fatalf("effective used = %d, want %d", used, want)
	}
	if cache := trueNASReclaimableARC(system); cache != 48<<30 {
		t.Fatalf("reclaimable ARC = %d, want %d", cache, int64(48<<30))
	}

	// used + cache + free must not exceed total.
	if used+trueNASReclaimableARC(system)+system.MemoryAvailableBytes > system.MemoryTotalBytes {
		t.Fatal("used + cache + free exceeds total")
	}
}

func TestTrueNASEffectiveMemoryUsedWithoutARCUnchanged(t *testing.T) {
	system := SystemInfo{
		MemoryTotalBytes:     64 << 30,
		MemoryAvailableBytes: 32 << 30,
	}
	if used := trueNASEffectiveMemoryUsed(system); used != 32<<30 {
		t.Fatalf("effective used without ARC = %d, want %d", used, int64(32<<30))
	}
	if cache := trueNASReclaimableARC(system); cache != 0 {
		t.Fatalf("reclaimable ARC without arc_size = %d, want 0", cache)
	}
}

func TestTrueNASReclaimableARCClampedToHeadroom(t *testing.T) {
	// An inconsistent snapshot (ARC larger than total-available) must not
	// drive used negative or let used+cache+free exceed total.
	system := SystemInfo{
		MemoryTotalBytes:     64 << 30,
		MemoryAvailableBytes: 60 << 30,
		ARCSizeBytes:         48 << 30,
	}
	if cache := trueNASReclaimableARC(system); cache != 4<<30 {
		t.Fatalf("clamped ARC = %d, want %d", cache, int64(4<<30))
	}
	if used := trueNASEffectiveMemoryUsed(system); used != 0 {
		t.Fatalf("effective used = %d, want 0", used)
	}
}

func TestSystemMemoryPercentHistorySubtractsARC(t *testing.T) {
	ts1 := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	ts2 := ts1.Add(time.Hour)
	history := &SystemMetricHistory{
		MemoryUsedBytes: []TimeSeriesPoint{
			{Timestamp: ts1, Value: 60 << 30},
			{Timestamp: ts2, Value: 62 << 30},
		},
		ARCSizeBytes: []TimeSeriesPoint{
			{Timestamp: ts1, Value: 48 << 30},
			{Timestamp: ts2, Value: 50 << 30},
		},
	}

	points := systemMemoryPercentHistory(history, 64<<30)
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	wantFirst := (float64(60<<30) - float64(48<<30)) / float64(64<<30) * 100
	if diff := points[0].Value - wantFirst; diff > 0.01 || diff < -0.01 {
		t.Fatalf("first point = %.2f, want %.2f", points[0].Value, wantFirst)
	}
}

func TestSystemMemoryPercentHistoryWithoutARCUnchanged(t *testing.T) {
	ts := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	history := &SystemMetricHistory{
		MemoryUsedBytes: []TimeSeriesPoint{{Timestamp: ts, Value: 32 << 30}},
	}
	points := systemMemoryPercentHistory(history, 64<<30)
	if len(points) != 1 || points[0].Value != 50 {
		t.Fatalf("expected unchanged 50%%, got %+v", points)
	}
}
