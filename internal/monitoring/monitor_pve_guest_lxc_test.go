package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubPVEClientLXCStatus struct {
	stubPVEClient

	containerStatus *proxmox.Container
	statusCalls     int
}

func (s *stubPVEClientLXCStatus) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	s.statusCalls++
	return s.containerStatus, nil
}

func TestMergeContainerRuntimeCounters_PrefersHigherStatusCounters(t *testing.T) {
	t.Parallel()

	current := IOMetrics{
		DiskRead:   0,
		DiskWrite:  8,
		NetworkIn:  12,
		NetworkOut: 0,
		Timestamp:  time.Unix(0, 0),
	}

	merged := mergeContainerRuntimeCounters(current, &proxmox.Container{
		DiskRead:  128,
		DiskWrite: 4,
		NetIn:     10,
		NetOut:    256,
	})

	if merged.DiskRead != 128 {
		t.Fatalf("expected DiskRead to upgrade from status snapshot, got %d", merged.DiskRead)
	}
	if merged.DiskWrite != 8 {
		t.Fatalf("expected DiskWrite to preserve the higher baseline counter, got %d", merged.DiskWrite)
	}
	if merged.NetworkIn != 12 {
		t.Fatalf("expected NetworkIn to preserve the higher baseline counter, got %d", merged.NetworkIn)
	}
	if merged.NetworkOut != 256 {
		t.Fatalf("expected NetworkOut to upgrade from status snapshot, got %d", merged.NetworkOut)
	}
}

func TestBuildContainerFromClusterResource_UsesContainerStatusCountersForRates(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{rateTracker: NewRateTracker()}
	client := &stubPVEClientLXCStatus{
		containerStatus: &proxmox.Container{
			Status:    "running",
			DiskRead:  4096,
			DiskWrite: 2048,
			NetIn:     1024,
			NetOut:    512,
		},
	}

	resource := proxmox.ClusterResource{
		Type:    "lxc",
		Node:    "pve-a",
		Name:    "cache-ct",
		Status:  "running",
		VMID:    202,
		MaxCPU:  2,
		MaxMem:  4096,
		Mem:     2048,
		MaxDisk: 32 * 1024 * 1024 * 1024,
		Disk:    8 * 1024 * 1024 * 1024,
	}

	if _, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	); !ok {
		t.Fatal("expected first container sample to be built")
	}

	time.Sleep(20 * time.Millisecond)

	client.containerStatus = &proxmox.Container{
		Status:    "running",
		DiskRead:  8192,
		DiskWrite: 4096,
		NetIn:     2048,
		NetOut:    1024,
	}

	container, _, _, _, ok := monitor.buildContainerFromClusterResource(
		context.Background(),
		"cluster-a",
		resource,
		client,
		map[int]bool{},
	)
	if !ok {
		t.Fatal("expected second container sample to be built")
	}
	if client.statusCalls < 2 {
		t.Fatalf("expected container status to be queried for running LXC samples, got %d calls", client.statusCalls)
	}
	if container.DiskRead <= 0 {
		t.Fatalf("expected DiskRead rate from container status counters, got %d", container.DiskRead)
	}
	if container.DiskWrite <= 0 {
		t.Fatalf("expected DiskWrite rate from container status counters, got %d", container.DiskWrite)
	}
	if container.NetworkIn <= 0 {
		t.Fatalf("expected NetworkIn rate from container status counters, got %d", container.NetworkIn)
	}
	if container.NetworkOut <= 0 {
		t.Fatalf("expected NetworkOut rate from container status counters, got %d", container.NetworkOut)
	}
}
