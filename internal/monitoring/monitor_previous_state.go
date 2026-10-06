package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type previousGuestContext struct {
	vms                []models.VM
	vmsByID            map[string]models.VM
	containers         []models.Container
	containersByID     map[string]models.Container
	containerOCIByVMID map[int]bool
	hostAgentsByVMID   map[string]models.Host
}

func (m *Monitor) previousGuestContextForInstance(instanceName string) previousGuestContext {
	ctx := previousGuestContext{
		vms:                make([]models.VM, 0),
		vmsByID:            make(map[string]models.VM),
		containers:         make([]models.Container, 0),
		containersByID:     make(map[string]models.Container),
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
		modelVM := previousVMFromView(vm)
		ctx.vms = append(ctx.vms, modelVM)
		if modelVM.ID != "" {
			ctx.vmsByID[modelVM.ID] = modelVM
		}
		guestID := makeGuestID(modelVM.Instance, modelVM.Node, modelVM.VMID)
		if guestID != "" {
			ctx.vmsByID[guestID] = modelVM
			if memory, ok := vm.LinkedAgentMemory(); ok {
				ctx.hostAgentsByVMID[guestID] = models.Host{LinkedVMID: guestID, Status: "online", Memory: memory}
			}
		}
	}

	for _, ct := range readState.Containers() {
		if ct == nil || ct.Instance() != instanceName {
			continue
		}
		container := previousContainerFromView(ct)
		ctx.containers = append(ctx.containers, container)
		if container.ID != "" {
			ctx.containersByID[container.ID] = container
		}
		guestID := makeGuestID(container.Instance, container.Node, container.VMID)
		if guestID != "" {
			ctx.containersByID[guestID] = container
			if memory, ok := ct.LinkedAgentMemory(); ok {
				ctx.hostAgentsByVMID[guestID] = models.Host{LinkedVMID: guestID, Status: "online", Memory: memory}
			}
		}
		if container.VMID > 0 && (strings.EqualFold(strings.TrimSpace(container.Type), "oci") || container.IsOCI) {
			ctx.containerOCIByVMID[container.VMID] = true
		}
	}

	for _, host := range readState.Hosts() {
		if host == nil {
			continue
		}
		modelHost := previousHostFromView(host)
		if modelHost.Status != "online" {
			continue
		}
		if modelHost.LinkedVMID != "" {
			ctx.hostAgentsByVMID[modelHost.LinkedVMID] = modelHost
		}
		if modelHost.LinkedContainerID != "" {
			ctx.hostAgentsByVMID[modelHost.LinkedContainerID] = modelHost
		}
	}

	return ctx
}

func (m *Monitor) previousNodesForInstance(instanceName string) []models.Node {
	prevInstanceNodes := make([]models.Node, 0)

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return prevInstanceNodes
	}

	// The registry keeps no copy of the identity stamps pollPVENode derives
	// from configuration, so restore them the same way. Nodes preserved through
	// an instance outage are written back to state, and without these the
	// registry would derive a different canonical identity for them.
	instanceCfg := m.getInstanceConfig(instanceName)
	providerScoped := m.pveNodeUsesProviderScopedIdentity(instanceName, instanceCfg)
	for _, existingNode := range readState.Nodes() {
		if existingNode == nil || existingNode.Instance() != instanceName {
			continue
		}
		modelNode := previousNodeFromView(existingNode)
		modelNode.ProviderScopedIdentity = providerScoped
		modelNode.NativeNameAliases = config.PVEClusterNodeNativeAliases(instanceCfg, modelNode.Name)
		modelNode.TLSFingerprint = pveNodeTLSFingerprint(instanceCfg, modelNode.Name)
		prevInstanceNodes = append(prevInstanceNodes, modelNode)
	}
	return prevInstanceNodes
}

func previousVMFromView(vm *unifiedresources.VMView) models.VM {
	if vm == nil {
		return models.VM{}
	}
	instance := vm.Instance()
	node := vm.Node()
	vmid := vm.VMID()
	return models.VM{
		ID:           makeGuestID(instance, node, vmid),
		Instance:     instance,
		Node:         node,
		VMID:         vmid,
		Name:         vm.Name(),
		Type:         "qemu",
		Status:       vm.RuntimeStatus(),
		IPAddresses:  vm.IPAddresses(),
		OSName:       vm.OSName(),
		OSVersion:    vm.OSVersion(),
		AgentVersion: vm.AgentVersion(),
		Lock:         vm.Lock(),
		Disk: models.Disk{
			Used:  vm.DiskUsed(),
			Total: vm.DiskTotal(),
			Free:  max(0, vm.DiskTotal()-vm.DiskUsed()),
			Usage: vm.DiskPercent(),
		},
		NetworkInterfaces: guestNetworkInterfacesFromReadStateView(vm.NetworkInterfaces()),
		Disks:             guestDisksFromReadStateView(vm.Disks()),
		DiskStatusReason:  vm.DiskStatusReason(),
		LastSeen:          vm.LastSeen(),
	}
}

func previousContainerFromView(ct *unifiedresources.ContainerView) models.Container {
	if ct == nil {
		return models.Container{}
	}
	instance := ct.Instance()
	node := ct.Node()
	vmid := ct.VMID()
	return models.Container{
		ID:                makeGuestID(instance, node, vmid),
		Instance:          instance,
		Node:              node,
		VMID:              vmid,
		Name:              ct.Name(),
		Status:            ct.RuntimeStatus(),
		Type:              ct.ContainerType(),
		IsOCI:             ct.IsOCI(),
		LastSeen:          ct.LastSeen(),
		IPAddresses:       ct.IPAddresses(),
		NetworkInterfaces: guestNetworkInterfacesFromReadStateView(ct.NetworkInterfaces()),
		OSName:            ct.OSName(),
		OSTemplate:        ct.OSTemplate(),
		HasDocker:         ct.HasDocker(),
		DockerCheckedAt:   ct.DockerCheckedAt(),
		Disks:             guestDisksFromReadStateView(ct.Disks()),
		Lock:              ct.Lock(),
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
		ID:                host.ID(),
		Hostname:          host.Hostname(),
		Status:            string(host.Status()),
		LinkedVMID:        host.LinkedVMID(),
		LinkedContainerID: host.LinkedContainerID(),
		LastSeen:          host.LastSeen(),
		Disks:             guestDisksFromReadStateView(host.Disks()),
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
	// The poller keys nodes by their Proxmox source ID (nodeLastOnline, the
	// temperature carry, state writes). The unified resource ID is a separate
	// registry key, so returning it would match nothing, and writing it back
	// during an outage would re-key the node on every failed poll.
	id := node.SourceID()
	if id == "" {
		id = node.ID()
	}
	return models.Node{
		ID:                id,
		NodeIdentity:      id,
		Name:              node.NodeName(),
		DisplayName:       node.Name(),
		Instance:          node.Instance(),
		Host:              node.HostURL(),
		Status:            string(node.Status()),
		Type:              "node",
		Uptime:            node.Uptime(),
		IsClusterMember:   node.IsClusterMember(),
		ClusterName:       node.ClusterName(),
		LastSeen:          node.LastSeen(),
		LoadAverage:       node.LoadAverage(),
		PVEVersion:        node.PVEVersion(),
		KernelVersion:     node.KernelVersion(),
		NetworkInterfaces: hostNetworkInterfacesFromReadStateView(node.NetworkInterfaces()),
		Temperature:       node.TemperatureDetails(),
		LinkedAgentID:     node.LinkedAgentID(),
		Memory: models.Memory{
			Used:  node.MemoryUsed(),
			Total: node.MemoryTotal(),
			Usage: node.MemoryPercent() / 100,
		},
	}
}
