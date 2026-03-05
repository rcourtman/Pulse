package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

type previousGuestContext struct {
	vms                []models.VM
	containers         []models.Container
	containerOCIByVMID map[int]bool
	hostAgentsByVMID   map[string]models.Host
}

func (m *Monitor) previousGuestContextForInstance(instanceName string) previousGuestContext {
	snapshot := m.GetState()

	ctx := previousGuestContext{
		vms:                make([]models.VM, 0),
		containers:         make([]models.Container, 0),
		containerOCIByVMID: make(map[int]bool),
		hostAgentsByVMID:   make(map[string]models.Host),
	}

	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName {
			ctx.vms = append(ctx.vms, vm)
		}
	}

	for _, ct := range snapshot.Containers {
		if ct.Instance != instanceName {
			continue
		}
		ctx.containers = append(ctx.containers, ct)
		if ct.VMID > 0 && (ct.Type == "oci" || ct.IsOCI) {
			ctx.containerOCIByVMID[ct.VMID] = true
		}
	}

	for _, host := range snapshot.Hosts {
		if host.LinkedVMID == "" || host.Status != "online" || host.Memory.Total <= 0 {
			continue
		}
		ctx.hostAgentsByVMID[host.LinkedVMID] = host
	}

	return ctx
}

func (m *Monitor) previousNodesForInstance(instanceName string) (map[string]models.Memory, []models.Node) {
	snapshot := m.GetState()
	prevNodeMemory := make(map[string]models.Memory)
	prevInstanceNodes := make([]models.Node, 0)
	for _, existingNode := range snapshot.Nodes {
		if existingNode.Instance != instanceName {
			continue
		}
		prevNodeMemory[existingNode.ID] = existingNode.Memory
		prevInstanceNodes = append(prevInstanceNodes, existingNode)
	}
	return prevNodeMemory, prevInstanceNodes
}
