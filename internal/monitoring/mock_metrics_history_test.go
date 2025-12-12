package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestSeedMockMetricsHistory_PopulatesSeries(t *testing.T) {
	now := time.Now()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Status: "online",
				CPU:    0.33,
				Memory: models.Memory{Usage: 62, Total: 128 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 41, Total: 1024, Used: 512},
			},
		},
		VMs: []models.VM{
			{
				ID:     "vm-100",
				Status: "running",
				CPU:    0.21,
				Memory: models.Memory{Usage: 47, Total: 8 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 28, Total: 1024, Used: 256},
			},
		},
		Containers: []models.Container{
			{
				ID:     "ct-200",
				Status: "running",
				CPU:    0.09,
				Memory: models.Memory{Usage: 53, Total: 2 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 17, Total: 512, Used: 128},
			},
		},
		Storage: []models.Storage{
			{
				ID:     "local",
				Status: "available",
				Total:  1000,
				Used:   420,
				Free:   580,
				Usage:  42,
			},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "host-1",
				Status:   "online",
				CPUUsage: 22.5,
				Memory:   models.Memory{Usage: 58, Total: 16 * 1024 * 1024 * 1024},
				Disks: []models.Disk{
					{Total: 1000, Used: 600, Usage: 60},
				},
				Containers: []models.DockerContainer{
					{
						ID:                  "cont-1",
						State:               "running",
						CPUPercent:          3.3,
						MemoryPercent:       11.2,
						WritableLayerBytes:  10,
						RootFilesystemBytes: 100,
					},
				},
			},
		},
	}

	mh := NewMetricsHistory(1000, 24*time.Hour)
	seedMockMetricsHistory(mh, state, now, time.Hour, 30*time.Second)

	nodeCPU := mh.GetNodeMetrics("node-1", "cpu", time.Hour)
	if len(nodeCPU) < 10 {
		t.Fatalf("expected seeded node cpu points, got %d", len(nodeCPU))
	}
	if got, want := nodeCPU[len(nodeCPU)-1].Value, state.Nodes[0].CPU*100; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last node cpu point to match current, got=%v want=%v", got, want)
	}

	vmCPU := mh.GetGuestMetrics("vm-100", "cpu", time.Hour)
	if len(vmCPU) < 10 {
		t.Fatalf("expected seeded vm cpu points, got %d", len(vmCPU))
	}
	if got, want := vmCPU[len(vmCPU)-1].Value, state.VMs[0].CPU*100; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last vm cpu point to match current, got=%v want=%v", got, want)
	}

	dockerCPU := mh.GetGuestMetrics("docker:cont-1", "cpu", time.Hour)
	if len(dockerCPU) < 10 {
		t.Fatalf("expected seeded docker container cpu points, got %d", len(dockerCPU))
	}
	if got, want := dockerCPU[len(dockerCPU)-1].Value, state.DockerHosts[0].Containers[0].CPUPercent; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last docker cpu point to match current, got=%v want=%v", got, want)
	}
}

