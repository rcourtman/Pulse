package api

import (
	"math"
	"runtime"
	"testing"
)

func TestBuildSystemDiagnosticSeparatesHeapRuntimeAndRSS(t *testing.T) {
	const mib = uint64(1024 * 1024)
	memStats := runtime.MemStats{
		Alloc:        20 * mib,
		HeapInuse:    24 * mib,
		HeapIdle:     40 * mib,
		HeapReleased: 16 * mib,
		Sys:          80 * mib,
	}

	got := buildSystemDiagnostic(memStats, 96*mib, int64(230*mib))
	if got.MemoryMB != 20 || got.HeapAllocMB != 20 {
		t.Fatalf("live heap = legacy %d explicit %d, want 20", got.MemoryMB, got.HeapAllocMB)
	}
	if got.HeapInUseMB != 24 || got.HeapIdleMB != 40 || got.HeapReleasedMB != 16 {
		t.Fatalf("heap breakdown = in-use %d idle %d released %d", got.HeapInUseMB, got.HeapIdleMB, got.HeapReleasedMB)
	}
	if got.RuntimeRetainedMB != 64 {
		t.Fatalf("runtime retained = %d, want Sys - HeapReleased = 64", got.RuntimeRetainedMB)
	}
	if got.ProcessRSSMB != 96 || got.GCMemoryLimitMB != 230 {
		t.Fatalf("process/limit = RSS %d limit %d, want 96/230", got.ProcessRSSMB, got.GCMemoryLimitMB)
	}
}

func TestBuildSystemDiagnosticOmitsUnlimitedMemoryLimit(t *testing.T) {
	got := buildSystemDiagnostic(runtime.MemStats{}, 0, math.MaxInt64)
	if got.GCMemoryLimitMB != 0 {
		t.Fatalf("unlimited GC memory limit = %d, want omitted zero", got.GCMemoryLimitMB)
	}
}
