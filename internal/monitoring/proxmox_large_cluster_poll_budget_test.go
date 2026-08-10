package monitoring

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type largeClusterBudgetClient struct {
	*stubPVEClient
	resources            []proxmox.ClusterResource
	vmStatusCalls        atomic.Int64
	containerStatusCalls atomic.Int64
}

const largeClusterGuestCount = 500

func largeClusterResources() []proxmox.ClusterResource {
	resources := make([]proxmox.ClusterResource, 0, largeClusterGuestCount)
	for i := 0; i < largeClusterGuestCount; i++ {
		guestType := "qemu"
		if i%5 == 0 {
			guestType = "lxc"
		}
		resources = append(resources, proxmox.ClusterResource{
			Type:    guestType,
			Node:    fmt.Sprintf("node-%d", i%7),
			VMID:    1000 + i,
			Name:    fmt.Sprintf("vm-%d", i),
			Status:  "running",
			MaxCPU:  2,
			MaxMem:  2 * 1024 * 1024 * 1024,
			MaxDisk: 32 * 1024 * 1024 * 1024,
		})
	}
	return resources
}

func (c *largeClusterBudgetClient) GetClusterResources(context.Context, string) ([]proxmox.ClusterResource, error) {
	return c.resources, nil
}

func (c *largeClusterBudgetClient) GetVMStatus(ctx context.Context, _ string, _ int) (*proxmox.VMStatus, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		c.vmStatusCalls.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

func (c *largeClusterBudgetClient) GetContainerStatus(ctx context.Context, _ string, _ int) (*proxmox.Container, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		c.containerStatusCalls.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

func TestLargePVEClusterPublishesCompleteGenerationWhenEnrichmentBudgetExpires(t *testing.T) {
	monitor := newTestPVEMonitor("large-cluster")
	defer monitor.alertManager.Stop()
	defer monitor.notificationMgr.Stop()
	monitor.guestAgentWorkSlots = make(chan struct{}, defaultGuestAgentVMMaxConcurrent)
	monitor.state.UpdateGuestsForInstance(
		"large-cluster",
		[]models.VM{{
			ID:           "large-cluster:node-1:1001",
			VMID:         1001,
			Name:         "vm-1",
			Node:         "node-1",
			Instance:     "large-cluster",
			Status:       "running",
			IPAddresses:  []string{"10.0.0.11"},
			OSName:       "Debian",
			AgentVersion: "1.2.3",
		}},
		[]models.Container{{
			ID:              "large-cluster:node-0:1000",
			VMID:            1000,
			Name:            "vm-0",
			Node:            "node-0",
			Instance:        "large-cluster",
			Status:          "running",
			IPAddresses:     []string{"10.0.0.10"},
			OSName:          "Ubuntu",
			HasDocker:       true,
			DockerCheckedAt: time.Now(),
		}},
	)

	client := &largeClusterBudgetClient{
		stubPVEClient: &stubPVEClient{},
		resources:     largeClusterResources(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	started := time.Now()
	err := monitor.pollGuestsWithFallback(
		ctx,
		"large-cluster",
		&config.PVEInstance{MonitorVMs: true, MonitorContainers: true},
		client,
		nil,
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("pollGuestsWithFallback() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded guest poll took %v, want under 1s", elapsed)
	}

	snapshot := monitor.GetState()
	if got := len(snapshot.VMs); got != 400 {
		t.Fatalf("published VM generation has %d guests, want 400", got)
	}
	if got := len(snapshot.Containers); got != 100 {
		t.Fatalf("published container generation has %d guests, want 100", got)
	}
	if got := client.vmStatusCalls.Load(); got != 0 {
		t.Fatalf("expired enrichment budget made %d VM detail calls, want 0", got)
	}
	if got := client.containerStatusCalls.Load(); got != 0 {
		t.Fatalf("expired enrichment budget made %d container detail calls, want 0", got)
	}
	for i, vm := range snapshot.VMs {
		if vm.ID == "" || vm.VMID == 0 || vm.Instance != "large-cluster" {
			t.Fatalf("VM %d lost authoritative cluster identity: %+v", i, vm)
		}
	}
	if vm := findVMByID(snapshot.VMs, 1001); vm == nil || len(vm.IPAddresses) != 1 || vm.OSName != "Debian" || vm.AgentVersion != "1.2.3" {
		t.Fatalf("budgeted VM lost previous optional enrichment: %+v", vm)
	}
	if container := findContainerByID(snapshot.Containers, 1000); container == nil || len(container.IPAddresses) != 1 || container.OSName != "Ubuntu" || !container.HasDocker {
		t.Fatalf("budgeted container lost previous optional enrichment: %+v", container)
	}
}

func findVMByID(vms []models.VM, vmid int) *models.VM {
	for i := range vms {
		if vms[i].VMID == vmid {
			return &vms[i]
		}
	}
	return nil
}

func findContainerByID(containers []models.Container, vmid int) *models.Container {
	for i := range containers {
		if containers[i].VMID == vmid {
			return &containers[i]
		}
	}
	return nil
}

func TestLargePVEClusterCoreSuccessKeepsConnectionHealthy(t *testing.T) {
	nodes := make([]proxmox.Node, 0, 7)
	for i := 0; i < 7; i++ {
		nodes = append(nodes, proxmox.Node{Node: fmt.Sprintf("node-%d", i), Status: "online"})
	}
	client := &largeClusterBudgetClient{
		stubPVEClient: &stubPVEClient{nodes: nodes, nodeStatus: &proxmox.NodeStatus{}},
		resources:     largeClusterResources(),
	}
	monitor := newTestPVEMonitor("large-cluster")
	defer monitor.alertManager.Stop()
	defer monitor.notificationMgr.Stop()
	monitor.guestAgentWorkSlots = make(chan struct{}, defaultGuestAgentVMMaxConcurrent)
	monitor.config.PVEInstances[0].MonitorVMs = true
	monitor.config.PVEInstances[0].MonitorContainers = true

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	monitor.pollPVEInstance(ctx, "large-cluster", client)

	key := schedulerKey(InstanceTypePVE, "large-cluster")
	monitor.mu.RLock()
	status := monitor.pollStatusMap[key]
	monitor.mu.RUnlock()
	if status == nil {
		t.Fatal("PVE poll did not record connection health")
	}
	if status.LastSuccess.IsZero() || !status.LastErrorAt.IsZero() || status.LastErrorMessage != "" {
		t.Fatalf("core inventory was not recorded as healthy: %+v", *status)
	}
	snapshot := monitor.GetState()
	if got := len(snapshot.VMs) + len(snapshot.Containers); got != largeClusterGuestCount {
		t.Fatalf("published generation has %d guests, want %d", got, largeClusterGuestCount)
	}
}

func TestPVEGuestEnrichmentBudgetReservesPollTail(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelParent()

	bounded, cancelBounded := pveGuestEnrichmentContext(parent)
	defer cancelBounded()
	deadline, ok := bounded.Deadline()
	if !ok {
		t.Fatal("enrichment context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 59*time.Second || remaining > pveGuestEnrichmentMaxDuration {
		t.Fatalf("enrichment budget = %v, want approximately %v", remaining, pveGuestEnrichmentMaxDuration)
	}
}
