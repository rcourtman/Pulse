package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type slowGuestAgentClusterClient struct {
	stubPVEClient
	resources []proxmox.ClusterResource
	fsDelay   time.Duration
}

type emptyFSInfoClusterClient struct {
	stubPVEClient
	resources []proxmox.ClusterResource
}

type repeatedLowTrustMemoryClusterClient struct {
	stubPVEClient
	resources  []proxmox.ClusterResource
	vmStatuses map[int]*proxmox.VMStatus
}

type rotatingGuestAgentClusterClient struct {
	stubPVEClient
	resources []proxmox.ClusterResource
	fsDelay   time.Duration
}

type transientStatusFailureClusterClient struct {
	stubPVEClient
	resources []proxmox.ClusterResource
}

type healthyGuestLowTrustMemoryClusterClient struct {
	stubPVEClient
	resources []proxmox.ClusterResource
}

type windowsDriveClusterClient struct {
	stubPVEClient
	resources []proxmox.ClusterResource
	status    *proxmox.VMStatus
	fsInfo    []proxmox.VMFileSystem
}

type issue1319SaturatedMemoryClusterClient struct {
	stubPVEClient
	resources    []proxmox.ClusterResource
	status       *proxmox.VMStatus
	memAvailable uint64
	rrdPoints    []proxmox.GuestRRDPoint
	memCalls     int
	rrdCalls     int
}

func (c *slowGuestAgentClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *slowGuestAgentClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return &proxmox.VMStatus{
		MaxMem: 8 * 1024,
		Mem:    4 * 1024,
		Agent:  proxmox.VMAgentField{Value: 1},
	}, nil
}

func (c *slowGuestAgentClusterClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	select {
	case <-time.After(c.fsDelay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return []proxmox.VMFileSystem{{
		Mountpoint: "/",
		Type:       "ext4",
		TotalBytes: 100 * 1024 * 1024 * 1024,
		UsedBytes:  40 * 1024 * 1024 * 1024,
		Disk:       "/dev/vda",
	}}, nil
}

func (c *emptyFSInfoClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *emptyFSInfoClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return &proxmox.VMStatus{
		MaxMem: 8 * 1024,
		Mem:    4 * 1024,
		Agent:  proxmox.VMAgentField{Value: 1},
	}, nil
}

func (c *emptyFSInfoClusterClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return []proxmox.VMFileSystem{}, nil
}

func (c *repeatedLowTrustMemoryClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *repeatedLowTrustMemoryClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	if status, ok := c.vmStatuses[vmid]; ok {
		return status, nil
	}
	return nil, nil
}

func (c *rotatingGuestAgentClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *rotatingGuestAgentClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return &proxmox.VMStatus{
		MaxMem: 8 * 1024,
		Mem:    4 * 1024,
		Agent:  proxmox.VMAgentField{Value: 1},
	}, nil
}

func (c *rotatingGuestAgentClusterClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	select {
	case <-time.After(c.fsDelay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return []proxmox.VMFileSystem{{
		Mountpoint: "/",
		Type:       "ext4",
		TotalBytes: 100 * 1024 * 1024 * 1024,
		UsedBytes:  40 * 1024 * 1024 * 1024,
		Disk:       "/dev/vda",
	}}, nil
}

func (c *rotatingGuestAgentClusterClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return nil, nil
}

func (c *rotatingGuestAgentClusterClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return nil, nil
}

func (c *rotatingGuestAgentClusterClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "", nil
}

func (c *transientStatusFailureClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *transientStatusFailureClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return nil, context.DeadlineExceeded
}

func (c *transientStatusFailureClusterClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return []proxmox.VMFileSystem{{
		Mountpoint: "/",
		Type:       "ext4",
		TotalBytes: 100 * 1024 * 1024 * 1024,
		UsedBytes:  40 * 1024 * 1024 * 1024,
		Disk:       "/dev/vda",
	}}, nil
}

func (c *transientStatusFailureClusterClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return []proxmox.VMNetworkInterface{
		{
			Name:         "Ethernet0",
			HardwareAddr: "00:11:22:33:44:55",
			IPAddresses: []proxmox.VMIpAddress{
				{Address: "192.168.1.50", Prefix: 24},
			},
		},
	}, nil
}

func (c *transientStatusFailureClusterClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{
		"pretty-name": "Ubuntu 24.04",
		"version":     "24.04",
	}, nil
}

func (c *transientStatusFailureClusterClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "8.2.0", nil
}

func (c *transientStatusFailureClusterClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	return 5 * 1024, nil
}

func (c *healthyGuestLowTrustMemoryClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *healthyGuestLowTrustMemoryClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	const total = uint64(8 << 30)
	return &proxmox.VMStatus{
		Status: "running",
		Agent:  proxmox.VMAgentField{Value: 1},
		MaxMem: total,
		Mem:    total,
	}, nil
}

func (c *healthyGuestLowTrustMemoryClusterClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return []proxmox.VMNetworkInterface{
		{
			Name:         "Ethernet0",
			HardwareAddr: "00:11:22:33:44:55",
			IPAddresses: []proxmox.VMIpAddress{
				{Address: "192.168.1.50", Prefix: 24},
			},
		},
	}, nil
}

func (c *healthyGuestLowTrustMemoryClusterClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{
		"name":           "Ubuntu",
		"version-id":     "24.04",
		"pretty-name":    "Ubuntu 24.04",
		"version":        "24.04",
		"kernel-release": "6.8.0",
	}, nil
}

func (c *healthyGuestLowTrustMemoryClusterClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "8.2.0", nil
}

func (c *healthyGuestLowTrustMemoryClusterClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	return 0, context.DeadlineExceeded
}

func (c *windowsDriveClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *windowsDriveClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	if c.status != nil {
		return c.status, nil
	}
	return &proxmox.VMStatus{
		Status: "running",
		MaxMem: 8 * 1024,
		Mem:    4 * 1024,
		Agent:  proxmox.VMAgentField{Value: 1},
	}, nil
}

func (c *windowsDriveClusterClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	if c.fsInfo != nil {
		return c.fsInfo, nil
	}
	return []proxmox.VMFileSystem{
		{
			Mountpoint: "C:",
			Type:       "NTFS",
			TotalBytes: 100 * 1024 * 1024 * 1024,
			UsedBytes:  57 * 1024 * 1024 * 1024,
			Disk:       "C:",
		},
		{
			Mountpoint: "System Reserved",
			Type:       "NTFS",
			TotalBytes: 500 * 1024 * 1024,
			UsedBytes:  150 * 1024 * 1024,
			Disk:       "system-reserved",
		},
	}, nil
}

func (c *issue1319SaturatedMemoryClusterClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *issue1319SaturatedMemoryClusterClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return c.status, nil
}

func (c *issue1319SaturatedMemoryClusterClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	c.memCalls++
	return c.memAvailable, nil
}

func (c *issue1319SaturatedMemoryClusterClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	c.rrdCalls++
	return c.rrdPoints, nil
}

func TestGuestAgentFSInfoBudgetHonorsConfiguredTimeouts(t *testing.T) {
	t.Parallel()

	m := &Monitor{
		guestAgentFSInfoTimeout: 15 * time.Second,
		guestAgentRetries:       1,
	}

	budget := m.guestAgentFSInfoBudget()
	if budget < 30*time.Second {
		t.Fatalf("guestAgentFSInfoBudget() = %s, want at least 30s", budget)
	}
}

func TestRotateIndexedClusterResources(t *testing.T) {
	t.Parallel()

	original := []indexedClusterResource{
		{order: 0, resource: proxmox.ClusterResource{VMID: 100}},
		{order: 1, resource: proxmox.ClusterResource{VMID: 101}},
		{order: 2, resource: proxmox.ClusterResource{VMID: 102}},
	}

	rotated := rotateIndexedClusterResources(original, 1)
	if got := []int{rotated[0].resource.VMID, rotated[1].resource.VMID, rotated[2].resource.VMID}; got[0] != 101 || got[1] != 102 || got[2] != 100 {
		t.Fatalf("rotateIndexedClusterResources(..., 1) VMIDs = %v, want [101 102 100]", got)
	}

	if original[0].resource.VMID != 100 || original[1].resource.VMID != 101 || original[2].resource.VMID != 102 {
		t.Fatal("rotateIndexedClusterResources should not mutate the original slice")
	}
}

func TestPollVMsAndContainersEfficientCompletesDiskQueriesWithinPollBudget(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &slowGuestAgentClusterClient{
		fsDelay: 60 * time.Millisecond,
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 101, Name: "vm101", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 102, Name: "vm102", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 103, Name: "vm103", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentFSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentNetworkTimeout = 250 * time.Millisecond
	mon.guestAgentOSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentVersionTimeout = 250 * time.Millisecond
	mon.guestAgentRetries = 0
	mon.guestAgentWorkSlots = make(chan struct{}, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
	defer cancel()

	if ok := mon.pollVMsAndContainersEfficient(ctx, "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 4 {
		t.Fatalf("expected 4 VMs, got %d", len(state.VMs))
	}
	for _, vm := range state.VMs {
		if vm.Disk.Total <= 0 || vm.Disk.Usage <= 0 {
			t.Fatalf("expected guest-agent disk data for %s, got total=%d usage=%.2f", vm.Name, vm.Disk.Total, vm.Disk.Usage)
		}
	}
}

func TestPollVMsAndContainersEfficientRotatesGuestAgentPriorityAcrossPolls(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &rotatingGuestAgentClusterClient{
		fsDelay: 60 * time.Millisecond,
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 101, Name: "vm101", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 102, Name: "vm102", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 1)
	mon.guestAgentFSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentNetworkTimeout = 250 * time.Millisecond
	mon.guestAgentOSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentVersionTimeout = 250 * time.Millisecond
	mon.guestAgentRetries = 0

	checkResolved := func(expectedVMID int) {
		state := mon.state.GetSnapshot()
		if len(state.VMs) != 3 {
			t.Fatalf("expected 3 VMs, got %d", len(state.VMs))
		}

		vmByID := make(map[int]models.VM, len(state.VMs))
		for _, vm := range state.VMs {
			vmByID[vm.VMID] = vm
		}

		if vmByID[expectedVMID].Disk.Usage <= 0 {
			t.Fatalf("expected VM %d to get a real disk reading, got usage=%.2f reason=%q", expectedVMID, vmByID[expectedVMID].Disk.Usage, vmByID[expectedVMID].DiskStatusReason)
		}
	}

	for _, expectedVMID := range []int{100, 101, 102} {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
		if ok := mon.pollVMsAndContainersEfficient(ctx, "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
			cancel()
			t.Fatal("pollVMsAndContainersEfficient() returned false")
		}
		cancel()
		checkResolved(expectedVMID)
	}
}

func TestPollVMsAndContainersEfficientPreservesCachedGuestMetadataWhenStatusUnavailable(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &transientStatusFailureClusterClient{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = map[string]guestMetadataCacheEntry{
		guestMetadataCacheKey("pve1", "node1", 100): {
			ipAddresses: []string{"192.168.1.50"},
			networkInterfaces: []models.GuestNetworkInterface{
				{Name: "Ethernet0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.50"}},
			},
			osName:       "Windows",
			osVersion:    "Server 2022",
			agentVersion: "8.2.0",
			fetchedAt:    time.Now(),
		},
	}
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentFSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentNetworkTimeout = 250 * time.Millisecond
	mon.guestAgentOSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentVersionTimeout = 250 * time.Millisecond
	mon.guestAgentRetries = 0
	mon.guestAgentWorkSlots = make(chan struct{}, 1)

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if len(vm.IPAddresses) != 1 || vm.IPAddresses[0] != "192.168.1.50" {
		t.Fatalf("expected cached IPs to be preserved, got %#v", vm.IPAddresses)
	}
	if len(vm.NetworkInterfaces) != 1 || vm.NetworkInterfaces[0].Name != "Ethernet0" {
		t.Fatalf("expected cached interfaces to be preserved, got %#v", vm.NetworkInterfaces)
	}
	if vm.OSName != "Windows" || vm.OSVersion != "Server 2022" {
		t.Fatalf("expected cached OS info to be preserved, got %q %q", vm.OSName, vm.OSVersion)
	}
	if vm.AgentVersion != "8.2.0" {
		t.Fatalf("expected cached agent version to be preserved, got %q", vm.AgentVersion)
	}
}

func TestPollVMsAndContainersEfficientContinuesGuestAgentQueriesAfterTransientStatusFailure(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &transientStatusFailureClusterClient{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: 8 * 1024, Mem: 8 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentFSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentNetworkTimeout = 250 * time.Millisecond
	mon.guestAgentOSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentVersionTimeout = 250 * time.Millisecond
	mon.guestAgentRetries = 0
	mon.guestAgentWorkSlots = make(chan struct{}, 1)

	mon.state.UpdateVMsForInstance("pve1", []models.VM{
		{
			ID:           makeGuestID("pve1", "node1", 100),
			VMID:         100,
			Name:         "vm100",
			Node:         "node1",
			Instance:     "pve1",
			Type:         "qemu",
			Status:       "running",
			AgentVersion: "8.1.0",
			NetworkInterfaces: []models.GuestNetworkInterface{
				{Name: "Ethernet0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.50"}},
			},
			LastSeen: time.Now(),
		},
	})

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.MemorySource != "guest-agent-meminfo" {
		t.Fatalf("expected guest-agent memory fallback after status failure, got %q", vm.MemorySource)
	}
	if vm.Disk.Usage != 40 {
		t.Fatalf("expected live guest-agent disk usage after status failure, got %.2f", vm.Disk.Usage)
	}
	if vm.DiskStatusReason != "" {
		t.Fatalf("expected empty disk status reason, got %q", vm.DiskStatusReason)
	}
	if len(vm.Disks) != 1 || vm.Disks[0].Device != "/dev/vda" {
		t.Fatalf("expected live guest-agent disk inventory, got %#v", vm.Disks)
	}
	if len(vm.NetworkInterfaces) != 1 || vm.NetworkInterfaces[0].Name != "Ethernet0" {
		t.Fatalf("expected refreshed network interfaces, got %#v", vm.NetworkInterfaces)
	}
	if vm.AgentVersion != "8.2.0" {
		t.Fatalf("expected refreshed agent version, got %q", vm.AgentVersion)
	}
}

func TestPollVMsAndContainersEfficientKeepsPreviousMemoryForHealthyGuestAfterRepeatedLowTrustFullUsage(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const total = uint64(8 << 30)
	const trustedUsed = uint64(3 << 30)

	client := &healthyGuestLowTrustMemoryClusterClient{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: total, Mem: total, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentFSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentNetworkTimeout = 250 * time.Millisecond
	mon.guestAgentOSInfoTimeout = 250 * time.Millisecond
	mon.guestAgentVersionTimeout = 250 * time.Millisecond
	mon.guestAgentRetries = 0
	mon.guestAgentWorkSlots = make(chan struct{}, 1)

	mon.state.UpdateVMsForInstance("pve1", []models.VM{
		{
			ID:           makeGuestID("pve1", "node1", 100),
			VMID:         100,
			Name:         "vm100",
			Node:         "node1",
			Instance:     "pve1",
			Type:         "qemu",
			Status:       "running",
			MemorySource: "guest-agent-meminfo",
			Memory: models.Memory{
				Total: int64(total),
				Used:  int64(trustedUsed),
				Free:  int64(total - trustedUsed),
				Usage: safePercentage(float64(trustedUsed), float64(total)),
			},
			LastSeen: time.Now(),
		},
	})

	for i := 0; i < 2; i++ {
		if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
			t.Fatalf("pollVMsAndContainersEfficient() returned false on pass %d", i+1)
		}
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.MemorySource != "previous-snapshot" {
		t.Fatalf("memory source = %q, want previous-snapshot", vm.MemorySource)
	}
	if vm.Memory.Used != int64(trustedUsed) {
		t.Fatalf("memory used = %d, want preserved %d", vm.Memory.Used, trustedUsed)
	}
	if len(vm.NetworkInterfaces) != 1 || vm.NetworkInterfaces[0].Name != "Ethernet0" {
		t.Fatalf("expected guest agent network metadata to confirm healthy guest, got %#v", vm.NetworkInterfaces)
	}
}

func TestPollVMsAndContainersEfficientCarriesForwardPreviousIndividualDisks(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &emptyFSInfoClusterClient{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: 8 * 1024, Mem: 4 * 1024, MaxDisk: 100 * 1024 * 1024 * 1024, MaxCPU: 4},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	prevVM := models.VM{
		ID:       makeGuestID("pve1", "node1", 100),
		VMID:     100,
		Name:     "vm100",
		Node:     "node1",
		Instance: "pve1",
		Type:     "qemu",
		Status:   "running",
		Disk: models.Disk{
			Total: 100 * 1024 * 1024 * 1024,
			Used:  40 * 1024 * 1024 * 1024,
			Free:  60 * 1024 * 1024 * 1024,
			Usage: 40,
		},
		Disks: []models.Disk{
			{
				Total:      100 * 1024 * 1024 * 1024,
				Used:       40 * 1024 * 1024 * 1024,
				Free:       60 * 1024 * 1024 * 1024,
				Usage:      40,
				Mountpoint: "/",
				Type:       "ext4",
				Device:     "/dev/vda",
			},
		},
	}
	mon.state.UpdateVMs([]models.VM{prevVM})

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if len(vm.Disks) != 1 {
		t.Fatalf("expected previous individual disks to be preserved, got %#v", vm.Disks)
	}
	if vm.Disks[0].Mountpoint != "/" || vm.Disks[0].Device != "/dev/vda" {
		t.Fatalf("unexpected carried-forward disk data: %#v", vm.Disks[0])
	}
	if vm.Disk.Usage != 40 {
		t.Fatalf("expected aggregate disk usage to be carried forward, got %.2f", vm.Disk.Usage)
	}
	if vm.DiskStatusReason != "prev-no-filesystems" {
		t.Fatalf("expected carried-forward disk status reason, got %q", vm.DiskStatusReason)
	}
}

func TestPollVMsAndContainersEfficientMarksDiskUnknownUntilGuestAgentFilesystemDataArrives(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &emptyFSInfoClusterClient{
		resources: []proxmox.ClusterResource{
			{
				Type:    "qemu",
				Node:    "node1",
				VMID:    100,
				Name:    "vm100",
				Status:  "running",
				MaxMem:  8 * 1024,
				Mem:     4 * 1024,
				Disk:    57 * 1024 * 1024 * 1024,
				MaxDisk: 100 * 1024 * 1024 * 1024,
				MaxCPU:  4,
			},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.Disk.Usage != -1 {
		t.Fatalf("expected aggregate disk usage to remain unknown, got %.2f", vm.Disk.Usage)
	}
	if vm.DiskStatusReason != "no-filesystems" {
		t.Fatalf("expected disk status reason %q, got %q", "no-filesystems", vm.DiskStatusReason)
	}

	guestMetrics := mon.metricsHistory.GetGuestMetrics(vm.ID, "disk", time.Hour)
	if len(guestMetrics) != 0 {
		t.Fatalf("expected no disk metric samples while disk usage is unknown, got %#v", guestMetrics)
	}
}

func TestPollVMsAndContainersEfficientUsesLinkedHostAgentDiskFallback(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &emptyFSInfoClusterClient{
		resources: []proxmox.ClusterResource{
			{
				Type:    "qemu",
				Node:    "node1",
				VMID:    100,
				Name:    "vm100",
				Status:  "running",
				MaxMem:  8 * 1024,
				Mem:     4 * 1024,
				MaxDisk: 100 * 1024 * 1024 * 1024,
				MaxCPU:  4,
			},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	mon.state.UpsertHost(models.Host{
		ID:         "host-100",
		Hostname:   "vm100-agent",
		Status:     "online",
		LinkedVMID: makeGuestID("pve1", "node1", 100),
		Disks: []models.Disk{
			{
				Total:      100 * 1024 * 1024 * 1024,
				Used:       57 * 1024 * 1024 * 1024,
				Free:       43 * 1024 * 1024 * 1024,
				Usage:      57,
				Mountpoint: "C:",
				Type:       "NTFS",
				Device:     "C:",
			},
		},
	})

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.Disk.Usage != 57 {
		t.Fatalf("expected linked host-agent disk usage, got %.2f", vm.Disk.Usage)
	}
	if vm.DiskStatusReason != "" {
		t.Fatalf("expected cleared disk status reason, got %q", vm.DiskStatusReason)
	}
	if len(vm.Disks) != 1 || vm.Disks[0].Mountpoint != "C:" {
		t.Fatalf("expected linked host-agent disk inventory, got %#v", vm.Disks)
	}
}

func TestPollVMsAndContainersEfficientPrefersLinkedHostAgentDiskInventoryOverPartialGuestAgentFilesystems(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &slowGuestAgentClusterClient{
		resources: []proxmox.ClusterResource{
			{
				Type:    "qemu",
				Node:    "node1",
				VMID:    100,
				Name:    "pbs01",
				Status:  "running",
				MaxMem:  8 * 1024,
				Mem:     4 * 1024,
				MaxDisk: 300 * 1024 * 1024 * 1024,
				MaxCPU:  4,
			},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	mon.state.UpsertHost(models.Host{
		ID:         "host-pbs",
		Hostname:   "pbs01",
		Status:     "online",
		LinkedVMID: makeGuestID("pve1", "node1", 100),
		Disks: []models.Disk{
			{
				Total:      100 * 1024 * 1024 * 1024,
				Used:       57 * 1024 * 1024 * 1024,
				Free:       43 * 1024 * 1024 * 1024,
				Usage:      57,
				Mountpoint: "/",
				Type:       "ext4",
				Device:     "/dev/vda2",
			},
			{
				Total:      200 * 1024 * 1024 * 1024,
				Used:       120 * 1024 * 1024 * 1024,
				Free:       80 * 1024 * 1024 * 1024,
				Usage:      60,
				Mountpoint: "/mnt/datastore/pbs01rep01",
				Type:       "zfs",
				Device:     "rpool/pbs01rep01",
			},
		},
	})

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.Disk.Usage != 57 {
		t.Fatalf("expected linked host-agent root disk summary, got %.2f", vm.Disk.Usage)
	}
	if len(vm.Disks) != 2 {
		t.Fatalf("expected linked host-agent disk inventory, got %#v", vm.Disks)
	}
	if vm.Disks[1].Mountpoint != "/mnt/datastore/pbs01rep01" || vm.Disks[1].Type != "zfs" {
		t.Fatalf("expected linked host-agent ZFS datastore disk, got %#v", vm.Disks[1])
	}
}

func TestPollVMsAndContainersEfficientKeepsNormalizedWindowsDriveRoots(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &windowsDriveClusterClient{
		resources: []proxmox.ClusterResource{
			{
				Type:    "qemu",
				Node:    "node1",
				VMID:    100,
				Name:    "win100",
				Status:  "running",
				MaxMem:  8 * 1024,
				Mem:     4 * 1024,
				Disk:    0,
				MaxDisk: 100 * 1024 * 1024 * 1024,
				MaxCPU:  4,
			},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.DiskStatusReason != "" {
		t.Fatalf("expected empty disk status reason, got %q", vm.DiskStatusReason)
	}
	if len(vm.Disks) != 1 {
		t.Fatalf("expected 1 usable Windows disk, got %#v", vm.Disks)
	}
	if vm.Disks[0].Mountpoint != "C:" {
		t.Fatalf("expected normalized Windows drive root to be preserved, got %q", vm.Disks[0].Mountpoint)
	}
	if vm.Disk.Usage <= 0 {
		t.Fatalf("expected Windows guest disk usage to be populated, got %.2f", vm.Disk.Usage)
	}
}

func TestPollVMsAndContainersEfficientUsesGuestAgentMemAvailableForIssue1319SaturatedStatus(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	guestAvailable := uint64(4008068 * 1024)
	total := uint64(17179869184)

	client := &issue1319SaturatedMemoryClusterClient{
		resources: []proxmox.ClusterResource{{
			Type:   "qemu",
			Node:   "pve2",
			VMID:   164,
			Name:   "OC-SECURE-004",
			Status: "running",
			MaxMem: total,
			Mem:    17231028224,
			MaxCPU: 4,
		}},
		status: &proxmox.VMStatus{
			Status: "running",
			MaxMem: total,
			Mem:    17231028224,
			Agent:  proxmox.VMAgentField{Value: 1},
		},
		memAvailable: guestAvailable,
		rrdPoints:    []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(512 * 1024 * 1024))}},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"pve2": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.MemorySource != "guest-agent-meminfo" {
		t.Fatalf("MemorySource = %q, want guest-agent-meminfo", vm.MemorySource)
	}
	if got, want := uint64(vm.Memory.Used), total-guestAvailable; got != want {
		t.Fatalf("vm.Memory.Used = %d, want %d", got, want)
	}
	if client.memCalls != 1 {
		t.Fatalf("expected guest-agent meminfo to be queried once, got %d", client.memCalls)
	}
	if client.rrdCalls != 0 {
		t.Fatalf("expected guest-agent meminfo to prevent RRD fallback, got %d RRD calls", client.rrdCalls)
	}

	snapshot := mon.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "pve2", 164)]
	if snapshot.Raw.GuestAgentMemAvailable != guestAvailable {
		t.Fatalf("snapshot.Raw.GuestAgentMemAvailable = %d, want %d", snapshot.Raw.GuestAgentMemAvailable, guestAvailable)
	}
}

func TestPollVMsAndContainersEfficientCountsIssue1319WindowsVolumePayload(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &windowsDriveClusterClient{
		resources: []proxmox.ClusterResource{{
			Type:    "qemu",
			Node:    "pve7",
			VMID:    116,
			Name:    "win116",
			Status:  "running",
			MaxMem:  8 * 1024,
			Mem:     4 * 1024,
			Disk:    0,
			MaxDisk: 100 * 1024 * 1024 * 1024,
			MaxCPU:  4,
		}},
		fsInfo: []proxmox.VMFileSystem{
			{Mountpoint: "System Reserved", Type: "FAT32", Disk: `\\.\PhysicalDrive0`},
			{Mountpoint: `F:\`, Type: "NTFS", TotalBytes: 3298516004864, UsedBytes: 2671768784896, Disk: `\\.\PhysicalDrive2`},
			{Mountpoint: `E:\`, Type: "NTFS", TotalBytes: 9565733122048, UsedBytes: 8126376873984, Disk: `\\.\PhysicalDrive1`},
			{Mountpoint: `C:\`, Type: "NTFS", TotalBytes: 267789529088, UsedBytes: 195096502272, Disk: `\\.\PhysicalDrive0`},
			{Mountpoint: "System Reserved", Type: "NTFS", Disk: `\\.\PhysicalDrive0`},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 2)

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"pve7": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.DiskStatusReason != "" {
		t.Fatalf("expected empty disk status reason, got %q", vm.DiskStatusReason)
	}
	if len(vm.Disks) != 3 {
		t.Fatalf("expected 3 usable Windows volumes, got %#v", vm.Disks)
	}
	wantTotal := int64(3298516004864 + 9565733122048 + 267789529088)
	wantUsed := int64(2671768784896 + 8126376873984 + 195096502272)
	if vm.Disk.Total != wantTotal || vm.Disk.Used != wantUsed {
		t.Fatalf("unexpected aggregate disk totals: got total=%d used=%d want total=%d used=%d", vm.Disk.Total, vm.Disk.Used, wantTotal, wantUsed)
	}

	mountpoints := map[string]bool{}
	for _, disk := range vm.Disks {
		mountpoints[disk.Mountpoint] = true
	}
	for _, mountpoint := range []string{`C:\`, `E:\`, `F:\`} {
		if !mountpoints[mountpoint] {
			t.Fatalf("expected mountpoint %q in usable Windows disks, got %#v", mountpoint, vm.Disks)
		}
	}
}

func TestPollVMsAndContainersEfficientStabilizesSuspiciousRepeatedLowTrustMemory(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const total = uint64(8 << 30)
	client := &repeatedLowTrustMemoryClusterClient{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: total, Mem: total, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 101, Name: "vm101", Status: "running", MaxMem: total, Mem: total, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 102, Name: "vm102", Status: "running", MaxMem: total, Mem: total, MaxCPU: 4},
			{Type: "qemu", Node: "node1", VMID: 103, Name: "vm103", Status: "running", MaxMem: total, Mem: 2 << 30, MaxCPU: 4},
		},
		vmStatuses: map[int]*proxmox.VMStatus{
			100: {Status: "running", MaxMem: total, Mem: total, Balloon: 2 << 30, Agent: proxmox.VMAgentField{Value: 1}},
			101: {Status: "running", MaxMem: total, Mem: total, Agent: proxmox.VMAgentField{Value: 1}},
			102: {Status: "running", MaxMem: total, Mem: total, Agent: proxmox.VMAgentField{Value: 1}},
			103: {Status: "running", MaxMem: total, Mem: 2 << 30, Agent: proxmox.VMAgentField{Value: 0}},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 4)

	now := time.Now()
	mon.state.UpdateVMs([]models.VM{
		{
			ID:           makeGuestID("pve1", "node1", 100),
			VMID:         100,
			Name:         "vm100",
			Node:         "node1",
			Instance:     "pve1",
			Type:         "qemu",
			Status:       "running",
			MemorySource: "rrd-memavailable",
			Memory:       models.Memory{Total: int64(total), Used: 3 << 30, Free: 5 << 30, Usage: safePercentage(float64(3<<30), float64(total))},
			LastSeen:     now,
		},
		{
			ID:           makeGuestID("pve1", "node1", 101),
			VMID:         101,
			Name:         "vm101",
			Node:         "node1",
			Instance:     "pve1",
			Type:         "qemu",
			Status:       "running",
			MemorySource: "guest-agent-meminfo",
			Memory:       models.Memory{Total: int64(total), Used: 4 << 30, Free: 4 << 30, Usage: 50},
			LastSeen:     now,
		},
		{
			ID:           makeGuestID("pve1", "node1", 102),
			VMID:         102,
			Name:         "vm102",
			Node:         "node1",
			Instance:     "pve1",
			Type:         "qemu",
			Status:       "running",
			MemorySource: "previous-snapshot",
			Memory:       models.Memory{Total: int64(total), Used: 5 << 30, Free: 3 << 30, Usage: 62.5},
			LastSeen:     now,
		},
	})

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 4 {
		t.Fatalf("expected 4 VMs, got %d", len(state.VMs))
	}

	vmByID := make(map[int]models.VM, len(state.VMs))
	for _, vm := range state.VMs {
		vmByID[vm.VMID] = vm
	}

	if vmByID[100].MemorySource != "previous-snapshot" || vmByID[100].Memory.Used != 3<<30 {
		t.Fatalf("vm100 memory = %#v source=%q, want preserved previous reading", vmByID[100].Memory, vmByID[100].MemorySource)
	}
	if vmByID[100].Memory.Balloon != 2<<30 {
		t.Fatalf("vm100 balloon = %d, want current balloon", vmByID[100].Memory.Balloon)
	}
	if vmByID[101].MemorySource != "previous-snapshot" || vmByID[101].Memory.Used != 4<<30 {
		t.Fatalf("vm101 memory = %#v source=%q, want preserved previous reading", vmByID[101].Memory, vmByID[101].MemorySource)
	}
	if vmByID[102].MemorySource != "previous-snapshot" || vmByID[102].Memory.Used != 5<<30 {
		t.Fatalf("vm102 memory = %#v source=%q, want chained preserved reading", vmByID[102].Memory, vmByID[102].MemorySource)
	}
	if vmByID[103].MemorySource != "status-mem" || vmByID[103].Memory.Used != 2<<30 {
		t.Fatalf("vm103 memory = %#v source=%q, want unaffected current reading", vmByID[103].Memory, vmByID[103].MemorySource)
	}

	snapshotKey := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
	mon.diagMu.RLock()
	snapshot, ok := mon.guestSnapshots[snapshotKey]
	stabilizedSnapshot := mon.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "node1", 102)]
	mon.diagMu.RUnlock()
	if !ok {
		t.Fatal("expected guest snapshot for vm100")
	}
	if snapshot.MemorySource != "previous-snapshot" || snapshot.Memory.Used != 3<<30 {
		t.Fatalf("snapshot memory = %#v source=%q, want preserved previous reading", snapshot.Memory, snapshot.MemorySource)
	}
	if !snapshotHasNote(stabilizedSnapshot.Notes, "preserved-previous-memory-after-repeated-low-trust-pattern") &&
		!snapshotHasNote(stabilizedSnapshot.Notes, "preserved-previous-memory-for-healthy-guest-low-trust-full-usage") {
		t.Fatalf("vm102 snapshot notes = %#v, want preservation note", stabilizedSnapshot.Notes)
	}
}

func TestPollVMsAndContainersEfficientTreatsAvailableGuestAgentAsHealthyForMemoryCarryForward(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const total = uint64(8 << 30)
	client := &repeatedLowTrustMemoryClusterClient{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", Node: "node1", VMID: 100, Name: "vm100", Status: "running", MaxMem: total, Mem: total, MaxCPU: 4},
		},
		vmStatuses: map[int]*proxmox.VMStatus{
			100: {Status: "running", MaxMem: total, Mem: total, Agent: proxmox.VMAgentField{Value: 1}},
		},
	}

	mon := newTestPVEMonitor("pve1")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.rateTracker = NewRateTracker()
	mon.guestMetadataCache = make(map[string]guestMetadataCacheEntry)
	mon.guestMetadataLimiter = make(map[string]time.Time)
	mon.vmRRDMemCache = make(map[string]rrdMemCacheEntry)
	mon.vmAgentMemCache = make(map[string]agentMemCacheEntry)
	mon.guestAgentWorkSlots = make(chan struct{}, 4)

	now := time.Now()
	mon.state.UpdateVMs([]models.VM{
		{
			ID:           makeGuestID("pve1", "node1", 100),
			VMID:         100,
			Name:         "vm100",
			Node:         "node1",
			Instance:     "pve1",
			Type:         "qemu",
			Status:       "running",
			MemorySource: "previous-snapshot",
			Memory:       models.Memory{Total: int64(total), Used: 3 << 30, Free: 5 << 30, Usage: safePercentage(float64(3<<30), float64(total))},
			LastSeen:     now,
		},
	})

	if ok := mon.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"}); !ok {
		t.Fatal("pollVMsAndContainersEfficient() returned false")
	}

	state := mon.state.GetSnapshot()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}

	vm := state.VMs[0]
	if vm.MemorySource != "previous-snapshot" || vm.Memory.Used != 3<<30 {
		t.Fatalf("vm memory = %#v source=%q, want preserved previous reading", vm.Memory, vm.MemorySource)
	}
}
