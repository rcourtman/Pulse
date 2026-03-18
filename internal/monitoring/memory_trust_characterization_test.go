package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type vmMemoryTrustStubClient struct {
	*stubPVEClient
	vms          []proxmox.VM
	containers   []proxmox.Container
	vmStatus     *proxmox.VMStatus
	vmRRDPoints  []proxmox.GuestRRDPoint
	lxcRRDPoints []proxmox.GuestRRDPoint
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
	return s.vmRRDPoints, nil
}

func (s *vmMemoryTrustStubClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return s.lxcRRDPoints, nil
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

			modelNode, _, err := mon.pollPVENode(context.Background(), "test", &mon.config.PVEInstances[0], client, node, "healthy", nil, nil)
			if err != nil {
				t.Fatalf("pollPVENode() error = %v", err)
			}
			if got := uint64(modelNode.Memory.Used); got != tt.wantUsed {
				t.Fatalf("modelNode.Memory.Used = %d, want %d", got, tt.wantUsed)
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

	first, _, err := mon.pollPVENode(context.Background(), "test", &mon.config.PVEInstances[0], client, node, "healthy", nil, nil)
	if err != nil {
		t.Fatalf("first pollPVENode() error = %v", err)
	}
	if first.Memory.Used != int64(8*gib) {
		t.Fatalf("first.Memory.Used = %d, want %d", first.Memory.Used, 8*gib)
	}

	client.nodeStatus = nil
	second, _, err := mon.pollPVENode(
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
}

func TestHandleClusterVMResourceMemoryTrustCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name          string
		status        *proxmox.VMStatus
		rrdAvailable  uint64
		wantSource    string
		wantUsed      uint64
		wantAvailable uint64
		wantGap       uint64
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &vmMemoryTrustStubClient{
				stubPVEClient: &stubPVEClient{},
				vmStatus:      tt.status,
			}
			if tt.rrdAvailable > 0 {
				client.vmRRDPoints = []proxmox.GuestRRDPoint{{MemAvailable: floatPtr(float64(tt.rrdAvailable))}}
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

			vm, ok := mon.handleClusterVMResource(context.Background(), "test", res, makeGuestID("test", "node1", 101), client, nil)
			if !ok {
				t.Fatal("handleClusterVMResource() returned ok=false")
			}
			if got := uint64(vm.Memory.Used); got != tt.wantUsed {
				t.Fatalf("vm.Memory.Used = %d, want %d", got, tt.wantUsed)
			}

			snap := mon.guestSnapshots[makeGuestSnapshotKey("test", "qemu", "node1", 101)]
			if snap.MemorySource != tt.wantSource {
				t.Fatalf("snapshot.MemorySource = %q, want %q", snap.MemorySource, tt.wantSource)
			}
			if tt.wantSource == "derived-total-minus-used" && snap.FallbackReason != "derived-total-minus-used" {
				t.Fatalf("snapshot.FallbackReason = %q, want derived-total-minus-used", snap.FallbackReason)
			}
			if tt.wantAvailable > 0 && snap.Raw.MemInfoAvailable != tt.wantAvailable {
				t.Fatalf("snapshot.Raw.MemInfoAvailable = %d, want %d", snap.Raw.MemInfoAvailable, tt.wantAvailable)
			}
			if tt.wantGap > 0 && snap.Raw.MemInfoTotalMinusUsed != tt.wantGap {
				t.Fatalf("snapshot.Raw.MemInfoTotalMinusUsed = %d, want %d", snap.Raw.MemInfoTotalMinusUsed, tt.wantGap)
			}
		})
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
			if tt.wantAvailable > 0 && snap.Raw.MemInfoAvailable != tt.wantAvailable {
				t.Fatalf("snapshot.Raw.MemInfoAvailable = %d, want %d", snap.Raw.MemInfoAvailable, tt.wantAvailable)
			}
			if tt.wantSource == "derived-total-minus-used" && snap.FallbackReason != "derived-total-minus-used" {
				t.Fatalf("snapshot.FallbackReason = %q, want derived-total-minus-used", snap.FallbackReason)
			}
		})
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
			name: "stopped LXC keeps cluster resources source",
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
			wantSource: "cluster-resources",
			wantUsed:   2 * gib,
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
			if tt.wantAvail > 0 && snap.Raw.MemInfoAvailable != tt.wantAvail {
				t.Fatalf("snapshot.Raw.MemInfoAvailable = %d, want %d", snap.Raw.MemInfoAvailable, tt.wantAvail)
			}
		})
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
			name: "stopped LXC keeps cluster resources source in node polling",
			container: proxmox.Container{
				VMID:   302,
				Name:   "ct-302",
				Node:   "node1",
				Status: "stopped",
				MaxMem: 8 * gib,
				Mem:    2 * gib,
				CPUs:   2,
			},
			wantSource: "cluster-resources",
			wantUsed:   2 * gib,
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
			if tt.wantAvail > 0 && snap.Raw.MemInfoAvailable != tt.wantAvail {
				t.Fatalf("snapshot.Raw.MemInfoAvailable = %d, want %d", snap.Raw.MemInfoAvailable, tt.wantAvail)
			}
		})
	}
}
