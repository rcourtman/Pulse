package proxmox

import (
	"testing"
)

// Contradictory counters must not become a trusted zero-usage sample. These are
// synthetic parser inputs, not a replay or diagnosis of a native guest failure.
func TestParseLinuxMemoryAvailabilityRejectsContradictoryEvidence(t *testing.T) {
	for name, content := range map[string]string{
		"derived-over-total":                      "MemTotal: 8192 kB\nMemFree: 6144 kB\nBuffers: 1024 kB\nCached: 4096 kB\n",
		"free-over-total-with-shmem":              "MemTotal: 8192 kB\nMemFree: 12288 kB\nBuffers: 0 kB\nCached: 1024 kB\nShmem: 6144 kB\n",
		"buffers-over-total":                      "MemTotal: 8192 kB\nMemFree: 1024 kB\nBuffers: 12288 kB\nCached: 1024 kB\n",
		"cached-over-total":                       "MemTotal: 8192 kB\nMemFree: 1024 kB\nBuffers: 0 kB\nCached: 12288 kB\n",
		"reclaimable-over-total":                  "MemTotal: 8192 kB\nMemFree: 1024 kB\nBuffers: 0 kB\nCached: 1024 kB\nSReclaimable: 12288 kB\n",
		"shmem-over-total":                        "MemTotal: 8192 kB\nMemFree: 1024 kB\nBuffers: 0 kB\nCached: 1024 kB\nShmem: 12288 kB\n",
		"explicit-available-with-impossible-free": "MemTotal: 8192 kB\nMemAvailable: 4096 kB\nMemFree: 12288 kB\n",
		"derived-sum-overflow":                    "MemTotal: 18014398509481983 kB\nMemFree: 18014398509481982 kB\nBuffers: 0 kB\nCached: 2 kB\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseLinuxMemoryAvailability(content, false)
			if err == nil || got != (LinuxMemoryAvailability{}) {
				t.Errorf("contradictory sample acquired memory authority: source=%q available=%d total=%d error=%v", got.Source, got.EffectiveAvailable, got.Total, err)
			}
		})
	}
}

func TestParseLinuxMemoryAvailabilityPreservesValidBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, content, source string
		available             uint64
	}{
		{"old-kernel", "MemTotal: 8192 kB\nMemFree: 1024 kB\nBuffers: 512 kB\nCached: 2048 kB\nSReclaimable: 512 kB\nShmem: 256 kB\n", "meminfo-derived", 3840 * 1024},
		{"shared-subtracted-before-bound", "MemTotal: 8192 kB\nMemFree: 4096 kB\nBuffers: 0 kB\nCached: 6144 kB\nShmem: 4096 kB\n", "meminfo-derived", 6144 * 1024},
		{"derived-exact-total-measured-zero-used", "MemTotal: 8192 kB\nMemFree: 4096 kB\nBuffers: 0 kB\nCached: 4096 kB\n", "meminfo-derived", 8192 * 1024},
		{"shared-full-pressure", "MemTotal: 8192 kB\nMemFree: 0 kB\nBuffers: 0 kB\nCached: 1024 kB\nShmem: 1024 kB\n", "meminfo-derived", 0},
		{"explicit-zero-available", "MemTotal: 8192 kB\nMemAvailable: 0 kB\nMemFree: 0 kB\n", "meminfo-available", 0},
		{"explicit-available-does-not-use-estimate", "MemTotal: 8192 kB\nMemAvailable: 4096 kB\nMemFree: 4096 kB\nCached: 6144 kB\nSwapTotal: 16384 kB\nSwapFree: 12288 kB\n", "meminfo-available", 4096 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLinuxMemoryAvailability(tc.content, false)
			if err != nil || got.Source != tc.source || got.EffectiveAvailable != tc.available || got.Total != 8192*1024 {
				t.Errorf("valid boundary changed: %+v error=%v", got, err)
			}
		})
	}
}
