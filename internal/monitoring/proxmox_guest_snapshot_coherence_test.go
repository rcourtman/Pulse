package monitoring

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type partialNodeGuestClient struct {
	*stubPVEClient
	failedNodes      map[string]bool
	vmsByNode        map[string][]proxmox.VM
	containersByNode map[string][]proxmox.Container
}

func (c *partialNodeGuestClient) GetClusterResources(context.Context, string) ([]proxmox.ClusterResource, error) {
	return nil, errors.New("cluster resources unavailable")
}

func (c *partialNodeGuestClient) GetVMs(_ context.Context, node string) ([]proxmox.VM, error) {
	if c.failedNodes[node] {
		return nil, errors.New("VM enumeration unavailable")
	}
	return c.vmsByNode[node], nil
}

func (c *partialNodeGuestClient) GetContainers(_ context.Context, node string) ([]proxmox.Container, error) {
	if c.failedNodes[node] {
		return nil, errors.New("container enumeration unavailable")
	}
	return c.containersByNode[node], nil
}

func TestPollGuestsWithFallbackRetainsOnlyFailedNodeGeneration(t *testing.T) {
	monitor := newTestPVEMonitor("lab")
	defer monitor.alertManager.Stop()
	defer monitor.notificationMgr.Stop()

	monitor.state.UpdateGuestsForInstance(
		"lab",
		[]models.VM{
			{ID: "lab:node-a:101", VMID: 101, Name: "deleted-vm", Node: "node-a", Instance: "lab", Status: "running"},
			{ID: "lab:node-b:201", VMID: 201, Name: "retained-vm", Node: "node-b", Instance: "lab", Status: "running"},
		},
		[]models.Container{
			{ID: "lab:node-a:102", VMID: 102, Name: "deleted-ct", Node: "node-a", Instance: "lab", Status: "running"},
			{ID: "lab:node-b:202", VMID: 202, Name: "retained-ct", Node: "node-b", Instance: "lab", Status: "running"},
		},
	)
	monitor.state.UpdateGuestsForInstance(
		"other",
		[]models.VM{{ID: "other:node-c:301", VMID: 301, Node: "node-c", Instance: "other"}},
		[]models.Container{{ID: "other:node-c:302", VMID: 302, Node: "node-c", Instance: "other"}},
	)

	client := &partialNodeGuestClient{
		stubPVEClient:    &stubPVEClient{},
		failedNodes:      map[string]bool{"node-b": true},
		vmsByNode:        map[string][]proxmox.VM{"node-a": {}},
		containersByNode: map[string][]proxmox.Container{"node-a": {}},
	}
	nodes := []proxmox.Node{
		{Node: "node-a", Status: "online"},
		{Node: "node-b", Status: "online"},
	}
	nodeStatus := map[string]string{"node-a": "online", "node-b": "online"}
	cfg := &config.PVEInstance{MonitorVMs: true, MonitorContainers: true}

	if err := monitor.pollGuestsWithFallback(context.Background(), "lab", cfg, client, nodes, nodeStatus); err != nil {
		t.Fatalf("partial poll failed: %v", err)
	}

	snapshot := monitor.GetState()
	assertGuestIDs(t, snapshot.VMs, []string{"lab:node-b:201", "other:node-c:301"})
	assertGuestIDs(t, snapshot.Containers, []string{"lab:node-b:202", "other:node-c:302"})
	if snapshot.VMs[0].Status != "running" || snapshot.Containers[0].Status != "running" {
		t.Fatalf("failed-node power state was not retained: vm=%q container=%q", snapshot.VMs[0].Status, snapshot.Containers[0].Status)
	}

	client.failedNodes["node-b"] = false
	client.vmsByNode["node-b"] = []proxmox.VM{}
	client.containersByNode["node-b"] = []proxmox.Container{}

	if err := monitor.pollGuestsWithFallback(context.Background(), "lab", cfg, client, nodes, nodeStatus); err != nil {
		t.Fatalf("recovery poll failed: %v", err)
	}

	snapshot = monitor.GetState()
	assertGuestIDs(t, snapshot.VMs, []string{"other:node-c:301"})
	assertGuestIDs(t, snapshot.Containers, []string{"other:node-c:302"})
}

func assertGuestIDs[T models.VM | models.Container](t *testing.T, guests []T, want []string) {
	t.Helper()
	got := make([]string, 0, len(guests))
	for _, guest := range guests {
		switch typed := any(guest).(type) {
		case models.VM:
			got = append(got, typed.ID)
		case models.Container:
			got = append(got, typed.ID)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("guest ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("guest ids = %v, want %v", got, want)
		}
	}
}
