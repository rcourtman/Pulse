package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type previousGuestContext struct {
	vms                []models.VM
	containers         []models.Container
	containerOCIByVMID map[int]bool
	hostAgentsByVMID   map[string]models.Host
}

func (m *Monitor) previousGuestContextForInstance(instanceName string) previousGuestContext {
	ctx := previousGuestContext{
		vms:                make([]models.VM, 0),
		containers:         make([]models.Container, 0),
		containerOCIByVMID: make(map[int]bool),
		hostAgentsByVMID:   make(map[string]models.Host),
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return ctx
	}

	for _, vm := range readState.VMs() {
		if vm == nil || vm.Instance() != instanceName {
			continue
		}
		ctx.vms = append(ctx.vms, previousVMFromView(vm))
	}

	for _, ct := range readState.Containers() {
		if ct == nil || ct.Instance() != instanceName {
			continue
		}
		container := previousContainerFromView(ct)
		ctx.containers = append(ctx.containers, container)
		if container.VMID > 0 && (strings.EqualFold(strings.TrimSpace(container.Type), "oci") || container.IsOCI) {
			ctx.containerOCIByVMID[container.VMID] = true
		}
	}

	for _, host := range readState.Hosts() {
		if host == nil {
			continue
		}
		modelHost := previousHostFromView(host)
		if modelHost.LinkedVMID == "" || modelHost.Status != "online" || modelHost.Memory.Total <= 0 {
			continue
		}
		ctx.hostAgentsByVMID[modelHost.LinkedVMID] = modelHost
	}

	return ctx
}

func (m *Monitor) previousNodesForInstance(instanceName string) (map[string]models.Memory, []models.Node) {
	prevNodeMemory := make(map[string]models.Memory)
	prevInstanceNodes := make([]models.Node, 0)

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return prevNodeMemory, prevInstanceNodes
	}

	for _, existingNode := range readState.Nodes() {
		if existingNode == nil || existingNode.Instance() != instanceName {
			continue
		}
		modelNode := previousNodeFromView(existingNode)
		prevNodeMemory[modelNode.ID] = modelNode.Memory
		prevInstanceNodes = append(prevInstanceNodes, modelNode)
	}
	return prevNodeMemory, prevInstanceNodes
}

func previousVMFromView(vm *unifiedresources.VMView) models.VM {
	if vm == nil {
		return models.VM{}
	}
	return models.VM{
		ID:       vm.ID(),
		Instance: vm.Instance(),
		Node:     vm.Node(),
		VMID:     vm.VMID(),
		Name:     vm.Name(),
		Status:   string(vm.Status()),
		LastSeen: vm.LastSeen(),
	}
}

func previousContainerFromView(ct *unifiedresources.ContainerView) models.Container {
	if ct == nil {
		return models.Container{}
	}
	return models.Container{
		ID:       ct.ID(),
		Instance: ct.Instance(),
		Node:     ct.Node(),
		VMID:     ct.VMID(),
		Name:     ct.Name(),
		Status:   string(ct.Status()),
		Type:     ct.ContainerType(),
		IsOCI:    ct.IsOCI(),
		LastSeen: ct.LastSeen(),
		Memory: models.Memory{
			Used:  ct.MemoryUsed(),
			Total: ct.MemoryTotal(),
			Usage: ct.MemoryPercent() / 100,
		},
	}
}

func previousHostFromView(host *unifiedresources.HostView) models.Host {
	if host == nil {
		return models.Host{}
	}
	return models.Host{
		ID:         host.ID(),
		Hostname:   host.Hostname(),
		Status:     string(host.Status()),
		LinkedVMID: host.LinkedVMID(),
		LastSeen:   host.LastSeen(),
		Memory: models.Memory{
			Used:  host.MemoryUsed(),
			Total: host.MemoryTotal(),
			Usage: host.MemoryPercent() / 100,
		},
	}
}

func previousNodeFromView(node *unifiedresources.NodeView) models.Node {
	if node == nil {
		return models.Node{}
	}
	return models.Node{
		ID:              node.ID(),
		Name:            node.NodeName(),
		DisplayName:     node.Name(),
		Instance:        node.Instance(),
		Host:            node.HostURL(),
		Status:          string(node.Status()),
		Uptime:          node.Uptime(),
		IsClusterMember: node.IsClusterMember(),
		LastSeen:        node.LastSeen(),
		LoadAverage:     node.LoadAverage(),
		PVEVersion:      node.PVEVersion(),
		KernelVersion:   node.KernelVersion(),
		Memory: models.Memory{
			Used:  node.MemoryUsed(),
			Total: node.MemoryTotal(),
			Usage: node.MemoryPercent() / 100,
		},
	}
}
