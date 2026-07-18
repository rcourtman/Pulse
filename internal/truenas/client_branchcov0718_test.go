package truenas

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// timeoutError is a minimal net.Error implementation used to drive the
// Timeout()/Temporary() branches of isTimeoutError deterministically.
type timeoutError struct {
	timeout   bool
	temporary bool
	msg       string
}

func (e *timeoutError) Error() string { return e.msg }
func (e *timeoutError) Timeout() bool { return e.timeout }
func (e *timeoutError) Temporary() bool {
	return e.temporary
}

// wrappedError chains a timeout error so we can verify errors.As walks the
// chain inside isTimeoutError.
type wrappedError struct {
	inner error
}

func (e *wrappedError) Error() string { return "wrapped: " + e.inner.Error() }
func (e *wrappedError) Unwrap() error { return e.inner }

func TestIsTimeoutErrorCoversBranches(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error returns false", err: nil, want: false},
		{name: "plain non-network error returns false", err: fmt.Errorf("disk full"), want: false},
		{name: "network error with timeout flag returns true", err: &timeoutError{timeout: true, temporary: true, msg: "i/o timeout"}, want: true},
		{name: "network error without timeout flag returns false", err: &timeoutError{timeout: false, temporary: true, msg: "connection reset"}, want: false},
		{name: "wrapped network timeout still detected via errors.As", err: &wrappedError{inner: &timeoutError{timeout: true, msg: "deadline exceeded"}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTimeoutError(tt.err)
			if got != tt.want {
				t.Fatalf("isTimeoutError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestSplitSnapshotNameCoversShapes(t *testing.T) {
	tests := []struct {
		name        string
		full        string
		wantDataset string
		wantSnap    string
	}{
		{name: "empty string", full: "", wantDataset: "", wantSnap: ""},
		{name: "whitespace only collapses to empty", full: "   ", wantDataset: "", wantSnap: ""},
		{name: "no at sign", full: "tank/media", wantDataset: "", wantSnap: ""},
		{name: "simple dataset at snapshot", full: "tank@auto-2024-01-01", wantDataset: "tank", wantSnap: "auto-2024-01-01"},
		{name: "nested dataset at snapshot", full: "tank/media/movies@manual-1", wantDataset: "tank/media/movies", wantSnap: "manual-1"},
		{name: "only at sign returns empty parts", full: "@", wantDataset: "", wantSnap: ""},
		{name: "first at sign wins when multiple present", full: "tank@snap@extra", wantDataset: "tank", wantSnap: "snap@extra"},
		{name: "leading and trailing whitespace is trimmed", full: "  tank/media @ snap  ", wantDataset: "tank/media", wantSnap: "snap"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset, snap := splitSnapshotName(tt.full)
			if dataset != tt.wantDataset || snap != tt.wantSnap {
				t.Fatalf("splitSnapshotName(%q) = (%q, %q), want (%q, %q)", tt.full, dataset, snap, tt.wantDataset, tt.wantSnap)
			}
		})
	}
}

func TestDatasetFromSharePathCoversShapes(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty path", path: "", want: ""},
		{name: "whitespace only", path: "   ", want: ""},
		{name: "EXTERNAL sentinel rejected case-insensitively", path: "external", want: ""},
		{name: "EXTERNAL uppercase rejected", path: "EXTERNAL", want: ""},
		{name: "bare mnt prefix only", path: "/mnt/", want: ""},
		{name: "pool only under mnt", path: "/mnt/tank", want: "tank"},
		{name: "pool and dataset under mnt", path: "/mnt/tank/media", want: "tank/media"},
		{name: "deep path truncated to pool and dataset", path: "/mnt/tank/media/movies/action", want: "tank/media"},
		{name: "trailing slashes trimmed", path: "/mnt/tank/media/", want: "tank/media"},
		{name: "no mnt prefix keeps raw value", path: "tank/apps", want: "tank/apps"},
		{name: "surrounding whitespace trimmed", path: "  /mnt/tank/apps  ", want: "tank/apps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := datasetFromSharePath(tt.path)
			if got != tt.want {
				t.Fatalf("datasetFromSharePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPoolFromSharePathCoversShapes(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty path returns empty pool", path: "", want: ""},
		{name: "mnt only returns empty pool", path: "/mnt/", want: ""},
		{name: "EXTERNAL sentinel returns empty pool", path: "EXTERNAL", want: ""},
		{name: "pool only yields pool", path: "/mnt/tank", want: "tank"},
		{name: "pool and dataset yields pool only", path: "/mnt/tank/media", want: "tank"},
		{name: "deep path still yields root pool", path: "/mnt/tank/media/movies", want: "tank"},
		{name: "dataset path without slash echoes back as pool", path: "/mnt/tank", want: "tank"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := poolFromSharePath(tt.path)
			if got != tt.want {
				t.Fatalf("poolFromSharePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractFirstReportingFloatValueCoversShapes(t *testing.T) {
	tests := []struct {
		name      string
		raw       any
		wantValue float64
		wantOK    bool
	}{
		{name: "nil returns false", raw: nil, wantValue: 0, wantOK: false},
		{name: "empty slice returns false", raw: []any{}, wantValue: 0, wantOK: false},
		{name: "slice of non-numeric strings returns false", raw: []any{"abc", "def"}, wantValue: 0, wantOK: false},
		{name: "slice with first float64 wins", raw: []any{float64(1.5), float64(2.5)}, wantValue: 1.5, wantOK: true},
		{name: "slice skips non-numeric leading entries", raw: []any{"junk", float64(7.25)}, wantValue: 7.25, wantOK: true},
		{name: "nested slice recurses to first float", raw: []any{[]any{"x", float64(9.5)}}, wantValue: 9.5, wantOK: true},
		{name: "deeply nested slice recurses to first float", raw: []any{[]any{[]any{float64(42.0)}}}, wantValue: 42.0, wantOK: true},
		{name: "empty map returns false", raw: map[string]any{}, wantValue: 0, wantOK: false},
		{name: "single-entry map returns that value", raw: map[string]any{"cpu": float64(3.14)}, wantValue: 3.14, wantOK: true},
		{name: "map recurses into nested map for value", raw: map[string]any{"agg": map[string]any{"mean": float64(11.0)}}, wantValue: 11.0, wantOK: true},
		{name: "bare float64 default branch returns value", raw: float64(2.5), wantValue: 2.5, wantOK: true},
		{name: "bare int default branch returns value as float", raw: int(8), wantValue: 8.0, wantOK: true},
		{name: "numeric string default branch parses", raw: "12.5", wantValue: 12.5, wantOK: true},
		{name: "non-numeric string default branch returns false", raw: "not-a-number", wantValue: 0, wantOK: false},
		{name: "non-numeric unhandled type returns false", raw: struct{ X int }{X: 1}, wantValue: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := extractFirstReportingFloatValue(tt.raw)
			if gotOK != tt.wantOK {
				t.Fatalf("extractFirstReportingFloatValue(%v) ok = %v, want %v", tt.raw, gotOK, tt.wantOK)
			}
			if gotOK && gotValue != tt.wantValue {
				t.Fatalf("extractFirstReportingFloatValue(%v) = %v, want %v", tt.raw, gotValue, tt.wantValue)
			}
		})
	}
}

func TestMergeSystemTelemetryCoversAllBranches(t *testing.T) {
	t.Run("nil system is a no-op", func(t *testing.T) {
		telemetry := &SystemInfo{CPUCount: 8, CPUPercent: 12.5}
		mergeSystemTelemetry(nil, telemetry)
	})

	t.Run("nil telemetry is a no-op preserving system values", func(t *testing.T) {
		system := &SystemInfo{Hostname: "nas", CPUCount: 4, MemoryTotalBytes: 1024, CPUPercent: 50}
		mergeSystemTelemetry(system, nil)
		if system.CPUCount != 4 || system.MemoryTotalBytes != 1024 || system.CPUPercent != 50 {
			t.Fatalf("mergeSystemTelemetry with nil telemetry mutated system: %+v", system)
		}
	})

	t.Run("both nil pointers is a no-op", func(t *testing.T) {
		mergeSystemTelemetry(nil, nil)
	})

	t.Run("positive telemetry fields override system fields", func(t *testing.T) {
		system := &SystemInfo{CPUCount: 4, MemoryTotalBytes: 1024, MemoryAvailableBytes: 512}
		telemetry := &SystemInfo{
			CPUCount:             16,
			MemoryTotalBytes:     8192,
			MemoryAvailableBytes: 4096,
		}
		mergeSystemTelemetry(system, telemetry)
		if system.CPUCount != 16 {
			t.Fatalf("CPUCount = %d, want 16", system.CPUCount)
		}
		if system.MemoryTotalBytes != 8192 {
			t.Fatalf("MemoryTotalBytes = %d, want 8192", system.MemoryTotalBytes)
		}
		if system.MemoryAvailableBytes != 4096 {
			t.Fatalf("MemoryAvailableBytes = %d, want 4096", system.MemoryAvailableBytes)
		}
	})

	t.Run("zero or empty telemetry fields do not override system fields", func(t *testing.T) {
		system := &SystemInfo{CPUCount: 4, MemoryTotalBytes: 1024, MemoryAvailableBytes: 512, IntervalSeconds: 5}
		telemetry := &SystemInfo{
			CPUCount:             0,
			MemoryTotalBytes:     0,
			MemoryAvailableBytes: 0,
			IntervalSeconds:      0,
		}
		mergeSystemTelemetry(system, telemetry)
		if system.CPUCount != 4 {
			t.Fatalf("CPUCount = %d, want preserved 4", system.CPUCount)
		}
		if system.MemoryTotalBytes != 1024 {
			t.Fatalf("MemoryTotalBytes = %d, want preserved 1024", system.MemoryTotalBytes)
		}
		if system.MemoryAvailableBytes != 512 {
			t.Fatalf("MemoryAvailableBytes = %d, want preserved 512", system.MemoryAvailableBytes)
		}
		if system.IntervalSeconds != 5 {
			t.Fatalf("IntervalSeconds = %d, want preserved 5", system.IntervalSeconds)
		}
	})

	t.Run("rate fields are always copied regardless of zero", func(t *testing.T) {
		system := &SystemInfo{CPUPercent: 90, NetInRate: 1, NetOutRate: 2, DiskReadRate: 3, DiskWriteRate: 4}
		telemetry := &SystemInfo{CPUPercent: 0, NetInRate: 0, NetOutRate: 0, DiskReadRate: 0, DiskWriteRate: 0}
		mergeSystemTelemetry(system, telemetry)
		if system.CPUPercent != 0 || system.NetInRate != 0 || system.NetOutRate != 0 || system.DiskReadRate != 0 || system.DiskWriteRate != 0 {
			t.Fatalf("rate fields not overwritten with zero: %+v", system)
		}
	})

	t.Run("positive telemetry rate fields are copied through", func(t *testing.T) {
		system := &SystemInfo{}
		telemetry := &SystemInfo{CPUPercent: 42.5, NetInRate: 100, NetOutRate: 200, DiskReadRate: 300, DiskWriteRate: 400}
		mergeSystemTelemetry(system, telemetry)
		if system.CPUPercent != 42.5 {
			t.Fatalf("CPUPercent = %v, want 42.5", system.CPUPercent)
		}
		if system.NetInRate != 100 || system.NetOutRate != 200 {
			t.Fatalf("Net rates = (%v, %v), want (100, 200)", system.NetInRate, system.NetOutRate)
		}
		if system.DiskReadRate != 300 || system.DiskWriteRate != 400 {
			t.Fatalf("Disk rates = (%v, %v), want (300, 400)", system.DiskReadRate, system.DiskWriteRate)
		}
	})

	t.Run("non-empty temperature map is cloned into system", func(t *testing.T) {
		system := &SystemInfo{}
		temps := map[string]float64{"cpu_package": 55.0, "core_0": 52.0}
		telemetry := &SystemInfo{TemperatureCelsius: temps}
		mergeSystemTelemetry(system, telemetry)
		if system.TemperatureCelsius == nil {
			t.Fatalf("expected cloned temperature map, got nil")
		}
		if len(system.TemperatureCelsius) != 2 {
			t.Fatalf("expected 2 temperature entries, got %d", len(system.TemperatureCelsius))
		}
		if v, ok := system.TemperatureCelsius["cpu_package"]; !ok || v != 55.0 {
			t.Fatalf("cpu_package entry = (%v, %v), want present 55.0", v, ok)
		}
		system.TemperatureCelsius["cpu_package"] = 999
		if temps["cpu_package"] == 999 {
			t.Fatal("expected system temperature map to be a clone, not the same reference")
		}
	})

	t.Run("empty temperature map is not copied", func(t *testing.T) {
		system := &SystemInfo{TemperatureCelsius: map[string]float64{"existing": 1.0}}
		telemetry := &SystemInfo{TemperatureCelsius: map[string]float64{}}
		mergeSystemTelemetry(system, telemetry)
		if _, ok := system.TemperatureCelsius["existing"]; !ok {
			t.Fatalf("expected existing temperature map to be preserved, got %+v", system.TemperatureCelsius)
		}
	})

	t.Run("non-zero collected_at is copied", func(t *testing.T) {
		system := &SystemInfo{}
		now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
		telemetry := &SystemInfo{CollectedAt: now}
		mergeSystemTelemetry(system, telemetry)
		if !system.CollectedAt.Equal(now) {
			t.Fatalf("CollectedAt = %v, want %v", system.CollectedAt, now)
		}
	})

	t.Run("zero collected_at is not copied", func(t *testing.T) {
		original := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		system := &SystemInfo{CollectedAt: original}
		telemetry := &SystemInfo{CollectedAt: time.Time{}}
		mergeSystemTelemetry(system, telemetry)
		if !system.CollectedAt.Equal(original) {
			t.Fatalf("CollectedAt = %v, want preserved %v", system.CollectedAt, original)
		}
	})

	t.Run("positive interval seconds is copied", func(t *testing.T) {
		system := &SystemInfo{IntervalSeconds: 0}
		telemetry := &SystemInfo{IntervalSeconds: 2}
		mergeSystemTelemetry(system, telemetry)
		if system.IntervalSeconds != 2 {
			t.Fatalf("IntervalSeconds = %d, want 2", system.IntervalSeconds)
		}
	})
}

func TestAppImagesFromContainersCoversShapes(t *testing.T) {
	t.Run("nil containers returns nil", func(t *testing.T) {
		if got := appImagesFromContainers(nil); got != nil {
			t.Fatalf("appImagesFromContainers(nil) = %#v, want nil", got)
		}
	})

	t.Run("empty containers returns nil", func(t *testing.T) {
		if got := appImagesFromContainers([]AppContainer{}); got != nil {
			t.Fatalf("appImagesFromContainers([]) = %#v, want nil", got)
		}
	})

	t.Run("containers with blank images are skipped", func(t *testing.T) {
		containers := []AppContainer{
			{ID: "a", Image: ""},
			{ID: "b", Image: "   "},
		}
		got := appImagesFromContainers(containers)
		if len(got) != 0 {
			t.Fatalf("appImagesFromContainers() = %#v, want empty slice", got)
		}
	})

	t.Run("images are gathered preserving order and trimming whitespace", func(t *testing.T) {
		containers := []AppContainer{
			{ID: "a", Image: "  alpine:3.20  "},
			{ID: "b", Image: ""},
			{ID: "c", Image: "nginx:latest"},
		}
		got := appImagesFromContainers(containers)
		want := []string{"alpine:3.20", "nginx:latest"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("appImagesFromContainers() = %#v, want %#v", got, want)
		}
	})

	t.Run("duplicate images are preserved at this layer", func(t *testing.T) {
		containers := []AppContainer{
			{ID: "a", Image: "redis:7"},
			{ID: "b", Image: "redis:7"},
		}
		got := appImagesFromContainers(containers)
		if len(got) != 2 || got[0] != "redis:7" || got[1] != "redis:7" {
			t.Fatalf("appImagesFromContainers() = %#v, want two redis:7 entries", got)
		}
	})
}

func TestAppVolumesFromContainersCoversShapes(t *testing.T) {
	t.Run("nil containers returns nil", func(t *testing.T) {
		if got := appVolumesFromContainers(nil); got != nil {
			t.Fatalf("appVolumesFromContainers(nil) = %#v, want nil", got)
		}
	})

	t.Run("empty containers returns nil", func(t *testing.T) {
		if got := appVolumesFromContainers([]AppContainer{}); got != nil {
			t.Fatalf("appVolumesFromContainers([]) = %#v, want nil", got)
		}
	})

	t.Run("containers with no volume mounts produce empty result", func(t *testing.T) {
		containers := []AppContainer{
			{ID: "a"},
			{ID: "b"},
		}
		got := appVolumesFromContainers(containers)
		if len(got) != 0 {
			t.Fatalf("appVolumesFromContainers() = %#v, want empty", got)
		}
	})

	t.Run("volumes from multiple containers are concatenated in order", func(t *testing.T) {
		containers := []AppContainer{
			{ID: "a", VolumeMounts: []AppVolume{{Source: "/host/a", Destination: "/data/a", Mode: "rw", Type: "bind"}}},
			{ID: "b", VolumeMounts: []AppVolume{
				{Source: "/host/b1", Destination: "/data/b1"},
				{Source: "/host/b2", Destination: "/data/b2"},
			}},
		}
		got := appVolumesFromContainers(containers)
		if len(got) != 3 {
			t.Fatalf("appVolumesFromContainers() len = %d, want 3 (%#v)", len(got), got)
		}
		if got[0].Source != "/host/a" || got[1].Source != "/host/b1" || got[2].Source != "/host/b2" {
			t.Fatalf("appVolumesFromContainers() = %#v, want sources in container order", got)
		}
	})

	t.Run("duplicate volumes across containers are NOT deduped at this layer", func(t *testing.T) {
		// This deliberately documents the function's actual behaviour: dedup is
		// the caller's responsibility (see dedupeAppVolumes, invoked from
		// parseAppsWithStats). Asserting it here keeps the test honest and
		// guards against an accidental behaviour change.
		dupe := AppVolume{Source: "/host/shared", Destination: "/data/shared", Mode: "rw", Type: "bind"}
		containers := []AppContainer{
			{ID: "a", VolumeMounts: []AppVolume{dupe}},
			{ID: "b", VolumeMounts: []AppVolume{dupe}},
		}
		got := appVolumesFromContainers(containers)
		if len(got) != 2 {
			t.Fatalf("appVolumesFromContainers() len = %d, want 2 (no dedup at this layer)", len(got))
		}
		if got[0] != dupe || got[1] != dupe {
			t.Fatalf("appVolumesFromContainers() = %#v, want both volumes preserved", got)
		}
	})
}

// Compile-time guards: ensure the test helpers we built keep satisfying the
// interfaces the production code targets via errors.As / type switches.
var (
	_ net.Error = (*timeoutError)(nil)
	_ error     = (*wrappedError)(nil)
)
