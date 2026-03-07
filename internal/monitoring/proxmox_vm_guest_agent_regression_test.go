package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type guestAgentPanicPVEClient struct {
	mockPVEClient
}

func (guestAgentPanicPVEClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return []proxmox.ClusterResource{
		{
			Type:    "qemu",
			Node:    "node1",
			VMID:    100,
			Name:    "broken-agent",
			Status:  "running",
			MaxMem:  4 * 1024,
			Mem:     2 * 1024,
			MaxDisk: 100 * 1024,
			MaxCPU:  2,
		},
		{
			Type:    "qemu",
			Node:    "node1",
			VMID:    101,
			Name:    "healthy-after-broken",
			Status:  "running",
			MaxMem:  4 * 1024,
			Mem:     2 * 1024,
			MaxDisk: 100 * 1024,
			MaxCPU:  2,
		},
	}, nil
}

func (guestAgentPanicPVEClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return &proxmox.VMStatus{
		MaxMem: 4 * 1024,
		Mem:    2 * 1024,
		Agent:  proxmox.VMAgentField{Value: 1},
	}, nil
}

func (guestAgentPanicPVEClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}

func (guestAgentPanicPVEClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	if vmid == 100 {
		panic("simulated guest agent parser failure")
	}

	return []proxmox.VMFileSystem{
		{
			Mountpoint: "/",
			Type:       "ext4",
			TotalBytes: 100 * 1024,
			UsedBytes:  50 * 1024,
			Disk:       "/dev/vda",
		},
	}, nil
}

func (guestAgentPanicPVEClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return nil, nil
}

func (guestAgentPanicPVEClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (guestAgentPanicPVEClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "", nil
}

func (guestAgentPanicPVEClient) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return nil, nil
}

func TestPollVMsAndContainersEfficient_GuestAgentFailureDoesNotSkipLaterVMs(t *testing.T) {
	t.Parallel()

	monitor := &Monitor{
		state:                models.NewState(),
		alertManager:         alerts.NewManager(),
		metricsHistory:       NewMetricsHistory(16, time.Minute),
		rateTracker:          NewRateTracker(),
		vmRRDMemCache:        make(map[string]rrdMemCacheEntry),
		vmAgentMemCache:      make(map[string]agentMemCacheEntry),
		guestMetadataCache:   make(map[string]guestMetadataCacheEntry),
		guestMetadataLimiter: make(map[string]time.Time),
	}

	ok := monitor.pollVMsAndContainersEfficient(
		context.Background(),
		"pve-test",
		"",
		false,
		&guestAgentPanicPVEClient{},
		map[string]string{"node1": "online"},
	)
	if !ok {
		t.Fatalf("pollVMsAndContainersEfficient() returned false, want true")
	}

	vms := monitor.state.VMs
	if len(vms) != 2 {
		t.Fatalf("expected 2 VMs after polling, got %d", len(vms))
	}

	found := make(map[int]bool, len(vms))
	for _, vm := range vms {
		found[vm.VMID] = true
	}

	if !found[100] {
		t.Fatalf("broken guest-agent VM 100 was dropped from state")
	}
	if !found[101] {
		t.Fatalf("VM 101 was skipped after VM 100 guest-agent failure")
	}
}
