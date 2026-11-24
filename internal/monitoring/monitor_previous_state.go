package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type previousGuestContext struct {
	// The ordered slices own each poll-local row. The read-only indexes
	// point into those rows so lookups and outage carry don't store a
	// separate copy of every guest.
	vms                []models.VM
	vmsByID            map[string]*models.VM
	containers         []models.Container
	containersByID     map[string]*models.Container
	containerOCIByVMID map[int]bool
	hostAgentsByVMID   map[string]models.Host
}

func (m *Monitor) previousGuestContextForInstance(instanceName string) previousGuestContext {
	readState := m.currentModeReadState()
	var vmViews []*unifiedresources.VMView
	var containerViews []*unifiedresources.ContainerView
	if readState != nil {
		vmViews = readState.VMs()
		containerViews = readState.Containers()
	}

	// The views span every connection, but only this connection's rows
	// belong in the carry-over. Count the matching views before allocating
	// the slices so their backing arrays stay fixed while building the indexes.
	vmCount, containerCount := 0, 0
	for _, vm := range vmViews {
		if vm != nil && vm.Instance() == instanceName {
			vmCount++
		}
	}
	for _, ct := range containerViews {
		if ct != nil && ct.Instance() == instanceName {
			containerCount++
		}
	}
	ctx := previousGuestContext{
		vms:                make([]models.VM, 0, vmCount),
		vmsByID:            make(map[string]*models.VM, vmCount),
		containers:         make([]models.Container, 0, containerCount),
		containersByID:     make(map[string]*models.Container, containerCount),
		containerOCIByVMID: make(map[int]bool),
		hostAgentsByVMID:   make(map[string]models.Host),
	}
	if readState == nil {
		return ctx
	}

	for _, vm := range vmViews {
		if vm == nil || vm.Instance() != instanceName {
			continue
		}
		ctx.vms = append(ctx.vms, previousVMFromView(vm))
		row := &ctx.vms[len(ctx.vms)-1]
		if guestID := row.ID; guestID != "" {
			ctx.vmsByID[guestID] = row
			agent := models.Host{LinkedVMID: guestID, Status: "online"}
			memory, hasMemory := vm.LinkedAgentMemory()
			agent.Memory = memory
			disks, observedAt, hasDisks := vm.LinkedAgentDisks()
			agent.Disks, agent.LastSeen = guestDisksFromReadStateView(disks), observedAt
			if hasMemory || hasDisks {
				ctx.hostAgentsByVMID[guestID] = agent
			}
		}
	}

	for _, ct := range containerViews {
		if ct == nil || ct.Instance() != instanceName {
			continue
		}
		ctx.containers = append(ctx.containers, previousContainerFromView(ct))
		row := &ctx.containers[len(ctx.containers)-1]
		if guestID := row.ID; guestID != "" {
			ctx.containersByID[guestID] = row
			agent := models.Host{LinkedContainerID: guestID, Status: "online"}
			memory, hasMemory := ct.LinkedAgentMemory()
			agent.Memory = memory
			disks, observedAt, hasDisks := ct.LinkedAgentDisks()
			agent.Disks, agent.LastSeen = guestDisksFromReadStateView(disks), observedAt
			if hasMemory || hasDisks {
				ctx.hostAgentsByVMID[guestID] = agent
			}
		}
		if row.VMID > 0 && (strings.EqualFold(strings.TrimSpace(row.Type), "oci") || row.IsOCI) {
			ctx.containerOCIByVMID[row.VMID] = true
		}
	}

	for _, host := range readState.Hosts() {
		// Only an online, guest-linked host can contribute to this input.
		// Check the view before projecting it: projection clones the complete
		// disk inventory, twice, even for unrelated hosts we would discard.
		if host == nil || host.Status() != unifiedresources.StatusOnline || (host.LinkedVMID() == "" && host.LinkedContainerID() == "") {
			continue
		}
		modelHost := previousHostFromView(host)
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

	readState := m.currentModeReadState()
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
	// Only Proxmox's own guest disk reading may be carried forward as last
	// known. A linked agent's disk that filled in stays the agent's, or the
	// carry would relabel it as Proxmox's and freeze it over the agent's.
	var disk models.Disk
	if !vm.DiskFromLinkedAgent() {
		disk = models.Disk{
			Used:  vm.DiskUsed(),
			Total: vm.DiskTotal(),
			Free:  max(0, vm.DiskTotal()-vm.DiskUsed()),
			Usage: vm.DiskPercent(),
		}
	}
	disks, interfaces := vm.ProxmoxPollInventory()
	return models.VM{
		ID:                 makeGuestID(instance, node, vmid),
		Instance:           instance,
		Node:               node,
		VMID:               vmid,
		Name:               vm.Name(),
		Type:               "qemu",
		Status:             vm.RuntimeStatus(),
		IPAddresses:        vm.IPAddresses(),
		OSName:             vm.OSName(),
		OSVersion:          vm.OSVersion(),
		AgentVersion:       vm.AgentVersion(),
		Lock:               vm.Lock(),
		Disk:               disk,
		NetworkInterfaces:  interfaces,
		Disks:              disks,
		DiskStatusReason:   vm.DiskStatusReason(),
		DiskObservation:    vm.DiskObservation(),
		GuestAgentEvidence: vm.GuestAgentEvidence(),
		LastSeen:           vm.LastSeen(),
	}
}

func previousContainerFromView(ct *unifiedresources.ContainerView) models.Container {
	if ct == nil {
		return models.Container{}
	}
	instance := ct.Instance()
	node := ct.Node()
	vmid := ct.VMID()
	disks, interfaces := ct.ProxmoxPollInventory()
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
		NetworkInterfaces: interfaces,
		OSName:            ct.OSName(),
		OSTemplate:        ct.OSTemplate(),
		HasDocker:         ct.HasDocker(),
		DockerCheckedAt:   ct.DockerCheckedAt(),
		Disks:             disks,
		Lock:              ct.Lock(),
		Memory: models.Memory{
			Used:  ct.MemoryUsed(),
			Total: ct.MemoryTotal(),
			Usage: ct.MemoryPercent(), // Both the view and the poller use percentage points.
		},
	}
}

func previousHostFromView(host *unifiedresources.HostView) models.Host {
	if host == nil {
		return models.Host{}
	}
	disks, _, _ := host.CurrentAgentDisks()
	observedAt := host.LastSeen()
	if status, ok := host.SourceStatus(unifiedresources.SourceAgent); ok {
		observedAt = status.LastSeen
	}
	memory, hasMemory := host.AgentMemory()
	if !hasMemory {
		memory = models.Memory{}
	}
	return models.Host{
		ID:                host.ID(),
		Hostname:          host.Hostname(),
		Status:            string(host.Status()),
		LinkedVMID:        host.LinkedVMID(),
		LinkedContainerID: host.LinkedContainerID(),
		LastSeen:          observedAt,
		Disks:             guestDisksFromReadStateView(disks),
		Memory:            memory,
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
			Usage: node.MemoryPercent(), // Both the view and the poller use percentage points.
		},
	}
}
