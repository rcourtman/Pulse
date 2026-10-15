package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// Enrichment's previous-state views intentionally carry only selected fields.
// Failed enumeration needs the complete last accepted source tuple instead.
// Keep canonical membership/coordinate selection; do not carry a merged linked
// agent's metric back into Proxmox ownership or renew any observation time.
func (m *Monitor) retainedGuestsForFailedNodes(instance string, nodes map[string]struct{}, previous previousGuestContext) ([]models.VM, []models.Container) {
	vms, containers := m.state.GuestObservationsForNodes(instance, nodes)
	fullVMs := make(map[string]models.VM, len(vms))
	fullContainers := make(map[string]models.Container, len(containers))
	for _, vm := range vms {
		fullVMs[makeGuestID(instance, vm.Node, vm.VMID)] = vm
	}
	for _, ct := range containers {
		fullContainers[makeGuestID(instance, ct.Node, ct.VMID)] = ct
	}
	var retainedVMs []models.VM
	var retainedContainers []models.Container
	for _, vm := range previous.vms {
		if _, failed := nodes[vm.Node]; !failed {
			continue
		}
		if full, exists := fullVMs[makeGuestID(instance, vm.Node, vm.VMID)]; exists {
			vm = full
		}
		retainedVMs = append(retainedVMs, vm)
	}
	for _, ct := range previous.containers {
		if _, failed := nodes[ct.Node]; !failed {
			continue
		}
		if full, exists := fullContainers[makeGuestID(instance, ct.Node, ct.VMID)]; exists {
			ct = full
		}
		retainedContainers = append(retainedContainers, ct)
	}
	return retainedVMs, retainedContainers
}
