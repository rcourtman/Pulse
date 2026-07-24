package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type vmMemoryTrustStubClient struct {
	*stubPVEClient
	vms                 []proxmox.VM
	containers          []proxmox.Container
	vmStatus            *proxmox.VMStatus
	vmRRDPoints         []proxmox.GuestRRDPoint
	lxcRRDPoints        []proxmox.GuestRRDPoint
	vmAgentMemAvailable uint64
	vmRRDCalls          int
	vmAgentMemCalls     int
}

func TestCollectVMsWithNodesRetainsFailedNodeRuntimeState(t *testing.T) {
	monitor := newTestPVEMonitor("lab")
	defer monitor.alertManager.Stop()
	defer monitor.notificationMgr.Stop()
	monitor.state.UpdateVMsForInstance("lab", []models.VM{{
		ID:       "lab:node-b:201",
		VMID:     201,
		Name:     "database",
		Node:     "node-b",
		Instance: "lab",
		Status:   "running",
	}})

	client := &partialNodeGuestClient{
		stubPVEClient: &stubPVEClient{},
		failedNodes:   map[string]bool{"node-b": true},
	}
	vms := monitor.collectVMsWithNodes(
		context.Background(),
		"lab",
		"",
		false,
		client,
		[]proxmox.Node{{Node: "node-b", Status: "online"}},
		map[string]string{"node-b": "online"},
	)

	if len(vms) != 1 || vms[0].ID != "lab:node-b:201" || vms[0].Status != "running" {
		t.Fatalf("failed-node VM continuity = %+v, want retained source ID and running state", vms)
	}
}

func (s *vmMemoryTrustStubClient) GetVMs(ctx context.Context, node string) ([]proxmox.VM, error) {
	return s.vms, nil
}

func (s *vmMemoryTrustStubClient) GetContainers(ctx context.Context, node string) ([]proxmox.Container, error) {
	return s.containers, nil
}

func (s *vmMemoryTrustStubClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return s.vmStatus, nil
}

func (s *vmMemoryTrustStubClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	s.vmRRDCalls++
	return s.vmRRDPoints, nil
}

func (s *vmMemoryTrustStubClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return s.lxcRRDPoints, nil
}

func (s *vmMemoryTrustStubClient) GetVMMemAvailableFromAgent(ctx context.Context, node string, vmid int) (uint64, error) {
	s.vmAgentMemCalls++
	return s.vmAgentMemAvailable, nil
}

func TestPollPVENodeMemoryTrustCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name          string
		nodeStatus    *proxmox.NodeStatus
		rrdPoints     []proxmox.NodeRRDPoint
		wantSource    string
		wantFallback  string
		wantUsed      uint64
		wantRawSource string
		wantRRDUsed   uint64
		wantUnknown   bool
	}{
		{
			name: "missing MemAvailable derives from free+buffers+cached",
			nodeStatus: &proxmox.NodeStatus{
				Memory: &proxmox.MemoryStatus{
					Total:   32 * gib,
					Used:    26 * gib,
					Free:    2 * gib,
					Buffers: 3 * gib,
					Cached:  7 * gib,
				},
			},
			wantSource:    "derived-free-buffers-cached",
			wantFallback:  "",
			wantUsed:      20 * gib,
			wantRawSource: "node-status",
		},
		{
			name: "proxmox 8.4 field drift derives from total-minus-used gap",
			nodeStatus: &proxmox.NodeStatus{
				Memory: &proxmox.MemoryStatus{
					Total: 134794743808,
					Used:  107351023616,
					Free:  6471057408,
				},
			},
			wantSource:    "derived-total-minus-used",
			wantFallback:  "node-status-total-minus-used",
			wantUsed:      107351023616,
			wantRawSource: "node-status-total-minus-used",
		},
		{
			name: "RRD memused wins over MemFree and cache-inclusive status used",
			nodeStatus: &proxmox.NodeStatus{
				Memory: &proxmox.MemoryStatus{
					Total: 8 * gib,
					Used:  15 * gib / 2,
					Free:  gib / 2,
				},
			},
			rrdPoints:     []proxmox.NodeRRDPoint{{MemUsed: floatPtr(float64(6 * gib))}},
			wantSource:    "rrd-memused",
			wantFallback:  "rrd-memused",
			wantUsed:      6 * gib,
			wantRawSource: "rrd-memused",
			wantRRDUsed:   6 * gib,
		},
		{
			name: "explicit available wins over conflicting RRD",
			nodeStatus: &proxmox.NodeStatus{
				Memory: &proxmox.MemoryStatus{
					Total:     8 * gib,
					Used:      15 * gib / 2,
					Free:      gib / 2,
					Available: 2 * gib,
				},
			},
			rrdPoints:     []proxmox.NodeRRDPoint{{MemUsed: floatPtr(float64(7 * gib))}},
			wantSource:    "available-field",
			wantUsed:      6 * gib,
			wantRawSource: "node-status",
		},
		{
			name: "MemFree without cache evidence stays unknown",
			nodeStatus: &proxmox.NodeStatus{
				Memory: &proxmox.MemoryStatus{
					Total: 8 * gib,
					Used:  15 * gib / 2,
					Free:  gib / 2,
				},
			},
			wantSource:    "unavailable",
			wantFallback:  "cache-aware-memory-unavailable",
			wantRawSource: "cache-aware-unavailable",
			wantUnknown:   true,
		},
		{
			name: "conflicting cache components above total stay unknown",
			nodeStatus: &proxmox.NodeStatus{
				Memory: &proxmox.MemoryStatus{
					Total:   8 * gib,
					Used:    4 * gib,
					Free:    4 * gib,
					Buffers: 3 * gib,
					Cached:  3 * gib,
				},
			},
			wantSource:    "unavailable",
			wantFallback:  "cache-aware-memory-unavailable",
			wantRawSource: "cache-aware-unavailable",
			wantUnknown:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &stubPVEClient{nodeStatus: tt.nodeStatus, rrdPoints: tt.rrdPoints}
			node := proxmox.Node{
				Node:   "node1",
				Status: "online",
				MaxMem: tt.nodeStatus.Memory.Total,
				Mem:    tt.nodeStatus.Memory.Used,
				MaxCPU: 8,
			}

			modelNode, _, _, err := mon.pollPVENode(context.Background(), "test", &mon.config.PVEInstances[0], client, node, "healthy", nil, nil)
			if err != nil {
				t.Fatalf("pollPVENode() error = %v", err)
			}
			if got := uint64(modelNode.Memory.Used); got != tt.wantUsed {
				t.Fatalf("modelNode.Memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if modelNode.Memory.UsageUnavailable != tt.wantUnknown {
				t.Fatalf("modelNode.Memory.UsageUnavailable = %t, want %t", modelNode.Memory.UsageUnavailable, tt.wantUnknown)
			}

			snap := mon.nodeSnapshots[makeNodeSnapshotKey("test", "node1")]
			if snap.MemorySource != tt.wantSource {
				t.Fatalf("snapshot.MemorySource = %q, want %q", snap.MemorySource, tt.wantSource)
			}
			if snap.FallbackReason != tt.wantFallback {
				t.Fatalf("snapshot.FallbackReason = %q, want %q", snap.FallbackReason, tt.wantFallback)
			}
			if snap.Raw.ProxmoxMemorySource != tt.wantRawSource {
				t.Fatalf("snapshot.Raw.ProxmoxMemorySource = %q, want %q", snap.Raw.ProxmoxMemorySource, tt.wantRawSource)
			}
			if snap.Raw.RRDUsed != tt.wantRRDUsed {
				t.Fatalf("snapshot.Raw.RRDUsed = %d, want %d", snap.Raw.RRDUsed, tt.wantRRDUsed)
			}
		})
	}
}

func TestPollPVENodePreservesPreviousSnapshotDuringTransientFallback(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	client := &stubPVEClient{
		nodeStatus: &proxmox.NodeStatus{
			Memory: &proxmox.MemoryStatus{
				Total:     16 * gib,
				Used:      12 * gib,
				Free:      1 * gib,
				Available: 8 * gib,
			},
		},
	}
	node := proxmox.Node{
		Node:   "node1",
		Status: "online",
		MaxMem: 16 * gib,
		Mem:    15 * gib,
		MaxCPU: 8,
	}

	first, _, _, err := mon.pollPVENode(context.Background(), "test", &mon.config.PVEInstances[0], client, node, "healthy", nil, nil)
	if err != nil {
		t.Fatalf("first pollPVENode() error = %v", err)
	}
	if first.Memory.Used != int64(8*gib) {
		t.Fatalf("first.Memory.Used = %d, want %d", first.Memory.Used, 8*gib)
	}

	client.nodeStatus = nil
	second, _, _, err := mon.pollPVENode(
		context.Background(),
		"test",
		&mon.config.PVEInstances[0],
		client,
		node,
		"healthy",
		map[string]models.Memory{first.ID: first.Memory},
		nil,
	)
	if err != nil {
		t.Fatalf("second pollPVENode() error = %v", err)
	}
	if second.Memory.Used != first.Memory.Used {
		t.Fatalf("second.Memory.Used = %d, want preserved %d", second.Memory.Used, first.Memory.Used)
	}

	snap := mon.nodeSnapshots[makeNodeSnapshotKey("test", "node1")]
	if snap.MemorySource != "previous-snapshot" {
		t.Fatalf("snapshot.MemorySource = %q, want previous-snapshot", snap.MemorySource)
	}
	if snap.FallbackReason != "preserved-previous-snapshot" {
		t.Fatalf("snapshot.FallbackReason = %q, want preserved-previous-snapshot", snap.FallbackReason)
	}
	if snap.Memory.Used != first.Memory.Used {
		t.Fatalf("snapshot.Memory.Used = %d, want preserved %d", snap.Memory.Used, first.Memory.Used)
	}

	third, _, _, err := mon.pollPVENode(
		context.Background(),
		"test",
		&mon.config.PVEInstances[0],
		client,
		node,
		"healthy",
		map[string]models.Memory{second.ID: second.Memory},
		nil,
	)
	if err != nil {
		t.Fatalf("third pollPVENode() error = %v", err)
	}
	if !third.Memory.UsageUnavailable {
		t.Fatalf("third.Memory = %+v, want honest unavailable state after bounded carry-forward", third.Memory)
	}
}

func TestGuestDiskTrustCharacterizationCarriesForwardRecentSnapshot(t *testing.T) {
	now := time.Now()
	prev := &models.VM{
		Type:         "qemu",
		Status:       "running",
		LastSeen:     now.Add(-time.Minute),
		AgentVersion: "8.2.0",
		Disk: models.Disk{
			Total: 1000,
			Used:  400,
			Free:  600,
			Usage: 40,
		},
		Disks: []models.Disk{
			{Total: 1000, Used: 400, Free: 600, Usage: 40, Mountpoint: "/", Type: "ext4", Device: "/dev/vda"},
		},
	}

	total, used, free, usage, disks, reason := stabilizeGuestLowTrustDisk(
		prev,
		"running",
		1000,
		0,
		1000,
		-1,
		nil,
		"no-filesystems",
		false,
		now,
	)

	if total != 1000 || used != 400 || free != 600 || usage != 40 {
		t.Fatalf("unexpected carried-forward disk summary: total=%d used=%d free=%d usage=%.2f", total, used, free, usage)
	}
	if len(disks) != 1 || disks[0].Device != "/dev/vda" {
		t.Fatalf("expected previous individual disks to be preserved, got %#v", disks)
	}
	if reason != "prev-no-filesystems" {
		t.Fatalf("reason = %q, want prev-no-filesystems", reason)
	}
}

func TestHandleClusterVMResourceMemoryTrustCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name           string
		status         *proxmox.VMStatus
		rrdAvailable   uint64
		rrdUsed        uint64
		agentAvailable uint64
		wantSource     string
		wantUsed       uint64
		wantAvailable  uint64
		wantGap        uint64
		wantUnknown    bool
	}{
		{
			name: "cache inflated Linux VM usage prefers RRD memavailable fallback",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			rrdAvailable:  4 * gib,
			wantSource:    "rrd-memavailable",
			wantUsed:      4 * gib,
			wantAvailable: 4 * gib,
		},
		{
			name: "missing MemAvailable derives from free buffers cached",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total:   8 * gib,
					Free:    1 * gib,
					Buffers: gib / 2,
					Cached:  2 * gib,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			wantSource: "derived-free-buffers-cached",
			wantUsed:   uint64(8*gib) - (uint64(gib) + uint64(gib/2) + uint64(2*gib)),
		},
		{
			name: "missing Buffers and Cached derives from total minus used gap",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
					Used:  5 * gib,
					Free:  gib / 2,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			wantSource: "derived-total-minus-used",
			wantUsed:   5 * gib,
			wantGap:    3 * gib,
		},
		{
			name: "partial Linux meminfo uses RRD memused",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
					Free:  gib / 2,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			rrdUsed:    5 * gib,
			wantSource: "rrd-memused",
			wantUsed:   5 * gib,
		},
		{
			name: "partial Linux meminfo without cache aware fallback stays unknown",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    15 * gib / 2,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
					Free:  gib / 2,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			wantSource:  "unavailable",
			wantUnknown: true,
		},
		{
			name: "MemAvailable conflicting with the selected capacity stays unknown",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total:     16 * gib,
					Available: 12 * gib,
				},
			},
			wantSource:  "unavailable",
			wantUnknown: true,
		},
		{
			name: "reclaimable components above the selected capacity stay unknown",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total:   8 * gib,
					Free:    2 * gib,
					Buffers: 2 * gib,
					Cached:  5 * gib,
				},
			},
			wantSource:  "unavailable",
			wantUnknown: true,
		},
		{
			name: "guest agent availability above the selected capacity stays unknown",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
					Free:  gib / 2,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			agentAvailable: 9 * gib,
			wantSource:     "unavailable",
			wantUnknown:    true,
		},
		{
			name: "RRD available wins over delayed total minus used estimate",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
					Used:  5 * gib,
					Free:  gib / 2,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			rrdAvailable:  4 * gib,
			wantSource:    "rrd-memavailable",
			wantUsed:      4 * gib,
			wantAvailable: 4 * gib,
			wantGap:       3 * gib,
		},
		{
			name: "materially inconsistent status memory prefers freemem fallback",
			status: &proxmox.VMStatus{
				Status:  "running",
				MaxMem:  8 * gib,
				Mem:     7920 * 1024 * 1024,
				FreeMem: 5 * gib,
			},
			wantSource: "status-freemem",
			wantUsed:   3 * gib,
		},
		{
			name: "ballooned VM derives freemem fallback from balloon total",
			status: &proxmox.VMStatus{
				Status:  "running",
				MaxMem:  8 * gib,
				Mem:     4 * gib,
				Balloon: 4 * gib,
				FreeMem: 1 * gib,
			},
			wantSource: "status-freemem",
			wantUsed:   3 * gib,
		},
		{
			name: "saturated VM derives freemem fallback from ballooninfo",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    8 * gib,
				BalloonInfo: &proxmox.VMBalloonInfo{
					FreeMem:  5 * gib,
					TotalMem: 8 * gib,
				},
			},
			wantSource: "status-freemem",
			wantUsed:   3 * gib,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &vmMemoryTrustStubClient{
				stubPVEClient:       &stubPVEClient{},
				vmStatus:            tt.status,
				vmAgentMemAvailable: tt.agentAvailable,
			}
			if tt.rrdAvailable > 0 {
				client.vmRRDPoints = []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(tt.rrdAvailable))}}
			} else if tt.rrdUsed > 0 {
				client.vmRRDPoints = []proxmox.GuestRRDPoint{{MemUsed: floatPtr(float64(tt.rrdUsed))}}
			}

			res := proxmox.ClusterResource{
				ID:     "qemu/101",
				Type:   "qemu",
				Node:   "node1",
				Name:   "vm-101",
				Status: "running",
				VMID:   101,
				MaxMem: tt.status.MaxMem,
				Mem:    tt.status.Mem,
				MaxCPU: 4,
			}

			vm, ok := mon.handleClusterVMResource(context.Background(), "test", res, makeGuestID("test", "node1", 101), client, nil, nil)
			if !ok {
				t.Fatal("handleClusterVMResource() returned ok=false")
			}
			if got := uint64(vm.Memory.Used); got != tt.wantUsed {
				t.Fatalf("vm.Memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if vm.Memory.UsageUnavailable != tt.wantUnknown {
				t.Fatalf("vm.Memory.UsageUnavailable = %t, want %t", vm.Memory.UsageUnavailable, tt.wantUnknown)
			}

			snap := mon.guestSnapshots[makeGuestSnapshotKey("test", "qemu", "node1", 101)]
			if snap.MemorySource != tt.wantSource {
				t.Fatalf("snapshot.MemorySource = %q, want %q", snap.MemorySource, tt.wantSource)
			}
			if tt.wantSource == "derived-total-minus-used" && snap.FallbackReason != "derived-total-minus-used" {
				t.Fatalf("snapshot.FallbackReason = %q, want derived-total-minus-used", snap.FallbackReason)
			}
			if tt.wantAvailable > 0 && snap.Raw.RRDMemAvailable != tt.wantAvailable {
				t.Fatalf("snapshot.Raw.RRDMemAvailable = %d, want %d", snap.Raw.RRDMemAvailable, tt.wantAvailable)
			}
			if tt.rrdUsed > 0 && snap.Raw.RRDMemUsed != tt.rrdUsed {
				t.Fatalf("snapshot.Raw.RRDMemUsed = %d, want %d", snap.Raw.RRDMemUsed, tt.rrdUsed)
			}
			if tt.wantGap > 0 && snap.Raw.MemInfoTotalMinusUsed != tt.wantGap {
				t.Fatalf("snapshot.Raw.MemInfoTotalMinusUsed = %d, want %d", snap.Raw.MemInfoTotalMinusUsed, tt.wantGap)
			}
		})
	}
}

func TestHandleClusterVMResourcePrefersGuestAgentMemAvailableForSaturatedIssue1319Status(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		vmStatus: &proxmox.VMStatus{
			Status: "running",
			MaxMem: 16 * gib,
			Mem:    16*gib + 512*1024*1024,
			Agent:  proxmox.VMAgentField{Value: 1},
		},
		vmRRDPoints:         []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(512 * 1024 * 1024))}},
		vmAgentMemAvailable: 4 * gib,
	}
	res := proxmox.ClusterResource{
		ID:     "qemu/164",
		Type:   "qemu",
		Node:   "pve2",
		Name:   "linux-vm",
		Status: "running",
		VMID:   164,
		MaxMem: 16 * gib,
		Mem:    16*gib + 512*1024*1024,
		MaxCPU: 4,
	}

	vm, ok := mon.handleClusterVMResource(context.Background(), "cluster-a", res, makeGuestID("cluster-a", "pve2", 164), client, nil, nil)
	if !ok {
		t.Fatal("handleClusterVMResource() returned ok=false")
	}
	if got := uint64(vm.Memory.Used); got != 12*gib {
		t.Fatalf("vm.Memory.Used = %d, want %d", got, uint64(12*gib))
	}

	snap := mon.guestSnapshots[makeGuestSnapshotKey("cluster-a", "qemu", "pve2", 164)]
	if snap.MemorySource != "guest-agent-meminfo" {
		t.Fatalf("snapshot.MemorySource = %q, want guest-agent-meminfo", snap.MemorySource)
	}
	if snap.Raw.GuestAgentMemAvailable != 4*gib {
		t.Fatalf("snapshot.Raw.GuestAgentMemAvailable = %d, want %d", snap.Raw.GuestAgentMemAvailable, uint64(4*gib))
	}
	if client.vmAgentMemCalls != 1 {
		t.Fatalf("expected guest agent meminfo to be queried once, got %d calls", client.vmAgentMemCalls)
	}
	if client.vmRRDCalls != 0 {
		t.Fatalf("expected saturated guest-agent memory path to skip RRD, got %d RRD calls", client.vmRRDCalls)
	}
}

func TestPollVMsWithNodesPreservesProxmoxPool(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		vms: []proxmox.VM{{
			VMID:   101,
			Name:   "vm-101",
			Node:   "node1",
			Pool:   "prod-vms",
			Status: "stopped",
			MaxMem: 8 * 1024,
			Mem:    2 * 1024,
			CPUs:   2,
		}},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeEffectiveStatus := map[string]string{"node1": "online"}
	mon.pollVMsWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

	vms := mon.state.GetSnapshot().VMs
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	if got := vms[0].Pool; got != "prod-vms" {
		t.Fatalf("vm pool = %q, want %q", got, "prod-vms")
	}
}

func TestPollVMsWithNodesRecordsQEMUTemplateBackupInventoryReadiness(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		vms: []proxmox.VM{
			{
				VMID:     900,
				Name:     "tmpl-900",
				Node:     "node1",
				Status:   "stopped",
				Template: 1,
				MaxMem:   8 * 1024,
				CPUs:     2,
			},
			{
				VMID:   101,
				Name:   "vm-101",
				Node:   "node1",
				Status: "stopped",
				MaxMem: 8 * 1024,
				Mem:    2 * 1024,
				CPUs:   2,
			},
		},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeEffectiveStatus := map[string]string{"node1": "online"}
	mon.pollVMsWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

	vms := mon.state.GetSnapshot().VMs
	if len(vms) != 1 {
		t.Fatalf("expected only non-template VM in runtime state, got %d", len(vms))
	}
	if got := vms[0].VMID; got != 101 {
		t.Fatalf("runtime VMID = %d, want 101", got)
	}

	scope := mon.backupInventoryScopeForAlerts()
	if scope == nil {
		t.Fatal("expected backup inventory scope")
	}
	if !scope.PVEOrphanInventoryReady["test"]["qemu"] {
		t.Fatalf("expected qemu backup orphan inventory readiness for test instance")
	}
	templateSubject := pveBackupTemplateSubjectKey("test", "qemu", "node1", 900)
	if _, ok := scope.PVETemplateSubjects[templateSubject]; !ok {
		t.Fatalf("expected template subject %q in backup inventory scope", templateSubject)
	}
}

func TestPollVMsWithNodesMemoryTrustCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name          string
		status        *proxmox.VMStatus
		rrdAvailable  uint64
		wantSource    string
		wantUsed      uint64
		wantAvailable uint64
	}{
		{
			name: "cache inflated Linux VM usage prefers RRD memavailable fallback",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			rrdAvailable:  4 * gib,
			wantSource:    "rrd-memavailable",
			wantUsed:      4 * gib,
			wantAvailable: 4 * gib,
		},
		{
			name: "missing Buffers and Cached derives from total minus used gap",
			status: &proxmox.VMStatus{
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MemInfo: &proxmox.VMMemInfo{
					Total: 8 * gib,
					Used:  5 * gib,
					Free:  gib / 2,
				},
				Agent: proxmox.VMAgentField{Value: 1},
			},
			wantSource: "derived-total-minus-used",
			wantUsed:   5 * gib,
		},
		{
			name: "materially inconsistent status memory prefers freemem fallback",
			status: &proxmox.VMStatus{
				Status:  "running",
				MaxMem:  8 * gib,
				Mem:     7920 * 1024 * 1024,
				FreeMem: 5 * gib,
			},
			wantSource: "status-freemem",
			wantUsed:   3 * gib,
		},
		{
			name: "ballooned VM derives freemem fallback from balloon total",
			status: &proxmox.VMStatus{
				Status:  "running",
				MaxMem:  8 * gib,
				Mem:     4 * gib,
				Balloon: 4 * gib,
				FreeMem: 1 * gib,
			},
			wantSource: "status-freemem",
			wantUsed:   3 * gib,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &vmMemoryTrustStubClient{
				stubPVEClient: &stubPVEClient{},
				vms: []proxmox.VM{{
					VMID:   101,
					Name:   "vm-101",
					Node:   "node1",
					Status: "running",
					MaxMem: tt.status.MaxMem,
					Mem:    tt.status.Mem,
					CPUs:   2,
				}},
				vmStatus: tt.status,
			}
			if tt.rrdAvailable > 0 {
				client.vmRRDPoints = []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(tt.rrdAvailable))}}
			}

			nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
			nodeEffectiveStatus := map[string]string{"node1": "online"}
			mon.pollVMsWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

			key := makeGuestSnapshotKey("test", "qemu", "node1", 101)
			snap, ok := mon.guestSnapshots[key]
			if !ok {
				t.Fatalf("expected guest snapshot %q to be recorded", key)
			}
			if snap.MemorySource != tt.wantSource {
				t.Fatalf("snapshot.MemorySource = %q, want %q", snap.MemorySource, tt.wantSource)
			}
			if got := uint64(snap.Memory.Used); got != tt.wantUsed {
				t.Fatalf("snapshot.Memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if tt.wantAvailable > 0 && snap.Raw.RRDMemAvailable != tt.wantAvailable {
				t.Fatalf("snapshot.Raw.RRDMemAvailable = %d, want %d", snap.Raw.RRDMemAvailable, tt.wantAvailable)
			}
			if tt.wantSource == "derived-total-minus-used" && snap.FallbackReason != "derived-total-minus-used" {
				t.Fatalf("snapshot.FallbackReason = %q, want derived-total-minus-used", snap.FallbackReason)
			}
		})
	}
}

func TestPollVMsWithNodes_SkipsNativeGuestMetricWritesInMockMode(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		vms: []proxmox.VM{{
			VMID:   101,
			Name:   "vm-101",
			Node:   "node1",
			Status: "running",
			MaxMem: 8 * 1024,
			Mem:    3 * 1024,
			CPUs:   2,
			CPU:    0.42,
		}},
		vmStatus: &proxmox.VMStatus{
			Status: "running",
			MaxMem: 8 * 1024,
			Mem:    3 * 1024,
			Agent:  proxmox.VMAgentField{Value: 1},
		},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeEffectiveStatus := map[string]string{"node1": "online"}
	mon.pollVMsWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

	vms := mon.state.GetSnapshot().VMs
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}

	if got := mon.metricsHistory.GetGuestMetrics(vms[0].ID, "cpu", time.Hour); len(got) != 0 {
		t.Fatalf("expected mock mode to skip native VM metrics history writes, got %+v", got)
	}
	if got := mon.metricsHistory.GetGuestMetrics(vms[0].ID, "memory", time.Hour); len(got) != 0 {
		t.Fatalf("expected mock mode to skip native VM memory history writes, got %+v", got)
	}
}

func TestRecordGuestMetric_SkipsNativeWritesInMockMode(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	now := time.Now().UTC()
	mon.recordGuestMetric(
		"vm",
		"test:node1:101",
		42,
		48,
		51,
		1024,
		512,
		256,
		128,
		now,
	)

	if got := mon.metricsHistory.GetGuestMetrics("test:node1:101", "cpu", time.Hour); len(got) != 0 {
		t.Fatalf("expected mock mode to skip helper cpu history writes, got %+v", got)
	}
	if got := mon.metricsHistory.GetGuestMetrics("test:node1:101", "memory", time.Hour); len(got) != 0 {
		t.Fatalf("expected mock mode to skip helper memory history writes, got %+v", got)
	}
}

func TestRecordGuestMetricPreservesHistoryWhenLiveMemoryIsUnavailable(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	now := time.Now().UTC()
	mon.recordGuestMetric("vm", "test:node1:101", 20, 76, 40, -1, -1, -1, -1, now.Add(-time.Minute))
	mon.recordGuestMetric("vm", "test:node1:101", 25, -1, 42, -1, -1, -1, -1, now)

	points := mon.metricsHistory.GetGuestMetrics("test:node1:101", "memory", time.Hour)
	if len(points) != 1 {
		t.Fatalf("memory history points = %d, want 1 trusted point", len(points))
	}
	if points[0].Value != 76 {
		t.Fatalf("memory history value = %.2f, want 76", points[0].Value)
	}
}

func TestHandleClusterContainerResourceMemoryTrustCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name         string
		res          proxmox.ClusterResource
		lxcRRDPoints []proxmox.GuestRRDPoint
		wantSource   string
		wantUsed     uint64
		wantAvail    uint64
		wantStatus   string
		wantUnknown  bool
	}{
		{
			name: "cache inflated LXC usage prefers RRD memavailable fallback",
			res: proxmox.ClusterResource{
				ID:     "lxc/201",
				Type:   "lxc",
				Node:   "node1",
				Name:   "ct-201",
				Status: "running",
				VMID:   201,
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MaxCPU: 4,
			},
			lxcRRDPoints: []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(3 * gib))}},
			wantSource:   "rrd-memavailable",
			wantUsed:     5 * gib,
			wantAvail:    3 * gib,
			wantStatus:   "running",
		},
		{
			name: "missing memavailable falls back to RRD memused",
			res: proxmox.ClusterResource{
				ID:     "lxc/202",
				Type:   "lxc",
				Node:   "node1",
				Name:   "ct-202",
				Status: "running",
				VMID:   202,
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MaxCPU: 4,
			},
			lxcRRDPoints: []proxmox.GuestRRDPoint{{MemUsed: floatPtr(float64(6 * gib))}},
			wantSource:   "rrd-memused",
			wantUsed:     6 * gib,
			wantStatus:   "running",
		},
		{
			name: "explicit zero RRD memavailable is valid full usage",
			res: proxmox.ClusterResource{
				ID:     "lxc/205",
				Type:   "lxc",
				Node:   "node1",
				Name:   "ct-205",
				Status: "running",
				VMID:   205,
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MaxCPU: 4,
			},
			lxcRRDPoints: []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(0)}},
			wantSource:   "rrd-memavailable",
			wantUsed:     8 * gib,
			wantStatus:   "running",
		},
		{
			name: "running LXC without RRD memory stays unknown",
			res: proxmox.ClusterResource{
				ID:     "lxc/204",
				Type:   "lxc",
				Node:   "node1",
				Name:   "ct-204",
				Status: "running",
				VMID:   204,
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				MaxCPU: 4,
			},
			wantSource:  "unavailable",
			wantStatus:  "running",
			wantUnknown: true,
		},
		{
			name: "stopped LXC reports powered off memory",
			res: proxmox.ClusterResource{
				ID:     "lxc/203",
				Type:   "lxc",
				Node:   "node1",
				Name:   "ct-203",
				Status: "stopped",
				VMID:   203,
				MaxMem: 8 * gib,
				Mem:    2 * gib,
				MaxCPU: 4,
			},
			wantSource: "powered-off",
			wantUsed:   0,
			wantStatus: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &vmMemoryTrustStubClient{
				stubPVEClient: &stubPVEClient{},
			}
			client.lxcRRDPoints = tt.lxcRRDPoints

			container, ok := mon.handleClusterContainerResource(
				context.Background(),
				"test",
				tt.res,
				makeGuestID("test", "node1", tt.res.VMID),
				client,
				nil,
			)
			if !ok {
				t.Fatal("handleClusterContainerResource() returned ok=false")
			}
			if container.Status != tt.wantStatus {
				t.Fatalf("container.Status = %q, want %q", container.Status, tt.wantStatus)
			}
			if got := uint64(container.Memory.Used); got != tt.wantUsed {
				t.Fatalf("container.Memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if container.Memory.UsageUnavailable != tt.wantUnknown {
				t.Fatalf("container.Memory.UsageUnavailable = %t, want %t", container.Memory.UsageUnavailable, tt.wantUnknown)
			}

			key := makeGuestSnapshotKey("test", container.Type, "node1", tt.res.VMID)
			snap, ok := mon.guestSnapshots[key]
			if !ok {
				t.Fatalf("expected guest snapshot %q to be recorded", key)
			}
			if snap.MemorySource != tt.wantSource {
				t.Fatalf("snapshot.MemorySource = %q, want %q", snap.MemorySource, tt.wantSource)
			}
			if got := uint64(snap.Memory.Used); got != tt.wantUsed {
				t.Fatalf("snapshot.Memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if tt.wantAvail > 0 && snap.Raw.RRDMemAvailable != tt.wantAvail {
				t.Fatalf("snapshot.Raw.RRDMemAvailable = %d, want %d", snap.Raw.RRDMemAvailable, tt.wantAvail)
			}
		})
	}
}

func TestPollContainersWithNodesPreservesProxmoxPool(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		containers: []proxmox.Container{{
			VMID:   201,
			Name:   "ct-201",
			Node:   "node1",
			Pool:   "ops-lxc",
			Status: "stopped",
			MaxMem: 8 * 1024,
			Mem:    2 * 1024,
			CPUs:   2,
		}},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeEffectiveStatus := map[string]string{"node1": "online"}
	mon.pollContainersWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

	containers := mon.state.GetSnapshot().Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if got := containers[0].Pool; got != "ops-lxc" {
		t.Fatalf("container pool = %q, want %q", got, "ops-lxc")
	}
}

func TestPollContainersWithNodesMemoryTrustCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name         string
		container    proxmox.Container
		lxcRRDPoints []proxmox.GuestRRDPoint
		wantSource   string
		wantUsed     uint64
		wantAvail    uint64
	}{
		{
			name: "running LXC prefers RRD memavailable in node polling",
			container: proxmox.Container{
				VMID:   301,
				Name:   "ct-301",
				Node:   "node1",
				Status: "running",
				MaxMem: 8 * gib,
				Mem:    7 * gib,
				CPUs:   2,
			},
			lxcRRDPoints: []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(3 * gib))}},
			wantSource:   "rrd-memavailable",
			wantUsed:     5 * gib,
			wantAvail:    3 * gib,
		},
		{
			name: "stopped LXC reports powered off memory in node polling",
			container: proxmox.Container{
				VMID:   302,
				Name:   "ct-302",
				Node:   "node1",
				Status: "stopped",
				MaxMem: 8 * gib,
				Mem:    2 * gib,
				CPUs:   2,
			},
			wantSource: "powered-off",
			wantUsed:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &vmMemoryTrustStubClient{
				stubPVEClient: &stubPVEClient{},
				containers:    []proxmox.Container{tt.container},
				lxcRRDPoints:  tt.lxcRRDPoints,
			}

			nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
			nodeEffectiveStatus := map[string]string{"node1": "online"}
			mon.pollContainersWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

			key := makeGuestSnapshotKey("test", "lxc", "node1", int(tt.container.VMID))
			snap, ok := mon.guestSnapshots[key]
			if !ok {
				t.Fatalf("expected guest snapshot %q to be recorded", key)
			}
			if snap.MemorySource != tt.wantSource {
				t.Fatalf("snapshot.MemorySource = %q, want %q", snap.MemorySource, tt.wantSource)
			}
			if got := uint64(snap.Memory.Used); got != tt.wantUsed {
				t.Fatalf("snapshot.Memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if tt.wantAvail > 0 && snap.Raw.RRDMemAvailable != tt.wantAvail {
				t.Fatalf("snapshot.Raw.RRDMemAvailable = %d, want %d", snap.Raw.RRDMemAvailable, tt.wantAvail)
			}
		})
	}
}
