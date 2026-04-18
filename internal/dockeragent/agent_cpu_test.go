package dockeragent

import (
	"math"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/rs/zerolog"
)

func TestCalculateContainerCPUPercent(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("manual delta across two calls", func(t *testing.T) {
		agent := &Agent{
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
			cpuCount:         2,
		}

		// First call: stores sample, returns 0
		stats1 := containertypes.StatsResponse{
			Read: time.Now().Add(-time.Second),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
				OnlineCPUs:  2,
			},
		}
		got := agent.calculateContainerCPUPercent("container-123456", stats1)
		if got != 0 {
			t.Fatalf("first call: expected 0, got %f", got)
		}
		if _, ok := agent.prevContainerCPU["container-123456"]; !ok {
			t.Fatal("expected sample to be stored after first call")
		}

		// Second call: computes delta from stored sample
		stats2 := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 2000,
				OnlineCPUs:  2,
			},
		}
		got = agent.calculateContainerCPUPercent("container-123456", stats2)
		if got <= 0 {
			t.Fatalf("second call: expected percent > 0, got %f", got)
		}
	})

	t.Run("first manual sample returns zero", func(t *testing.T) {
		agent := &Agent{
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
		}
		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
			},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if got != 0 {
			t.Fatalf("expected 0, got %f", got)
		}
		if _, ok := agent.prevContainerCPU["container-123456"]; !ok {
			t.Fatal("expected sample to be stored")
		}
	})

	t.Run("manual system delta uses previous sample", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  100,
					systemUsage: 1000,
					onlineCPUs:  2,
					read:        time.Now().Add(-time.Second),
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 2000,
				OnlineCPUs:  0,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 2000,
			},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if got <= 0 {
			t.Fatalf("expected percent > 0, got %f", got)
		}
	})

	t.Run("manual time delta fallback", func(t *testing.T) {
		agent := &Agent{
			logger:   logger,
			cpuCount: 4,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  100,
					systemUsage: 1000,
					onlineCPUs:  0,
					read:        time.Now().Add(-2 * time.Second),
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1000,
				OnlineCPUs:  0,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1000,
			},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if got <= 0 {
			t.Fatalf("expected percent > 0, got %f", got)
		}
	})

	t.Run("counter reset uses current total", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  200,
					systemUsage: 1000,
					onlineCPUs:  2,
					read:        time.Now().Add(-time.Second),
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 50},
				SystemUsage: 1200,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 50},
				SystemUsage: 1200,
			},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if got <= 0 {
			t.Fatalf("expected percent > 0, got %f", got)
		}
	})

	t.Run("no online CPUs returns zero", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  100,
					systemUsage: 1000,
					onlineCPUs:  0,
					read:        time.Now().Add(-time.Second),
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1100,
				OnlineCPUs:  0,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1100,
			},
		}

		if got := agent.calculateContainerCPUPercent("container-123456", stats); got != 0 {
			t.Fatalf("expected 0, got %f", got)
		}
	})

	t.Run("elapsed non-positive returns zero", func(t *testing.T) {
		now := time.Now()
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  100,
					systemUsage: 1000,
					onlineCPUs:  2,
					read:        now,
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: now,
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 900,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 900,
			},
		}

		if got := agent.calculateContainerCPUPercent("container-123456", stats); got != 0 {
			t.Fatalf("expected 0, got %f", got)
		}
	})

	t.Run("total delta zero returns zero", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  200,
					systemUsage: 1000,
					onlineCPUs:  2,
					read:        time.Now().Add(-time.Second),
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1100,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1100,
			},
		}

		if got := agent.calculateContainerCPUPercent("container-123456", stats); got != 0 {
			t.Fatalf("expected 0, got %f", got)
		}
	})

	t.Run("no valid delta returns zero", func(t *testing.T) {
		agent := &Agent{
			logger: logger,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  100,
					systemUsage: 1000,
					onlineCPUs:  0,
					read:        time.Time{},
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: time.Time{},
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 1000,
				OnlineCPUs:  0,
			},
			PreCPUStats: containertypes.CPUStats{},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if got != 0 {
			t.Fatalf("expected 0, got %f", got)
		}
	})

	t.Run("ignores stale precpu stats", func(t *testing.T) {
		agent := &Agent{
			logger:           logger,
			prevContainerCPU: make(map[string]cpuSample),
			cpuCount:         2,
		}

		// Even with valid-looking PreCPUStats, we use manual tracking.
		// First call stores sample and returns 0.
		stats := containertypes.StatsResponse{
			Read: time.Now(),
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 200},
				SystemUsage: 2000,
				OnlineCPUs:  2,
			},
			PreCPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 1000,
			},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if got != 0 {
			t.Fatalf("expected 0 on first call (manual tracking), got %f", got)
		}
	})

	t.Run("podman uses wall clock delta", func(t *testing.T) {
		now := time.Now()
		agent := &Agent{
			logger:  logger,
			runtime: RuntimePodman,
			prevContainerCPU: map[string]cpuSample{
				"container-123456": {
					totalUsage:  1_000_000_000,
					systemUsage: 1_000,
					onlineCPUs:  16,
					read:        now.Add(-time.Second),
				},
			},
		}

		stats := containertypes.StatsResponse{
			Read: now,
			CPUStats: containertypes.CPUStats{
				CPUUsage:    containertypes.CPUUsage{TotalUsage: 1_250_000_000},
				SystemUsage: 2_000,
				OnlineCPUs:  16,
			},
		}

		got := agent.calculateContainerCPUPercent("container-123456", stats)
		if math.Abs(got-25.0) > 0.01 {
			t.Fatalf("expected podman cpu percent near 25.0, got %f", got)
		}
	})
}

func TestDecodeContainerStatsPayloadExtractsPodmanCPU(t *testing.T) {
	payload := []byte(`{
		"read":"2026-04-09T12:00:00Z",
		"cpu_stats":{
			"cpu_usage":{"total_usage":123456789},
			"system_cpu_usage":987654321,
			"online_cpus":16,
			"cpu":0.32,
			"throttling_data":{}
		},
		"precpu_stats":{},
		"memory_stats":{"usage":1000,"limit":2000,"stats":{"cache":100}},
		"blkio_stats":{"io_service_bytes_recursive":[]}
	}`)

	stats, podmanCPU, err := decodeContainerStatsPayload(payload)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if stats.CPUStats.CPUUsage.TotalUsage != 123456789 {
		t.Fatalf("expected total usage 123456789, got %d", stats.CPUStats.CPUUsage.TotalUsage)
	}
	if podmanCPU == nil {
		t.Fatal("expected podman cpu percent to be extracted")
	}
	if math.Abs(*podmanCPU-0.32) > 0.000001 {
		t.Fatalf("expected podman cpu percent 0.32, got %f", *podmanCPU)
	}
}
