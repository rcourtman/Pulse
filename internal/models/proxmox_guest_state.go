package models

import "time"

// UpdateGuestsForInstance replaces the VM and container projections for one
// Proxmox instance under a single state lock. Pollers collect and enrich both
// guest kinds before calling this method so readers cannot observe a mixed
// generation while a refresh is in flight.
func (s *State) UpdateGuestsForInstance(instanceName string, vms []VM, containers []Container) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.VMs = updateSliceByInstanceWithBackup(
		s.VMs, vms, instanceName,
		func(vm VM) string { return vm.ID },
		func(vm VM) string { return vm.Instance },
		func(vm VM) int { return vm.VMID },
		func(vm VM) time.Time { return vm.LastBackup },
		func(vm VM, t time.Time) VM { vm.LastBackup = t; return vm },
		cloneVM,
		func(items []VM, i, j int) bool { return items[i].VMID < items[j].VMID },
	)
	s.Containers = updateSliceByInstanceWithBackup(
		s.Containers, containers, instanceName,
		func(ct Container) string { return ct.ID },
		func(ct Container) string { return ct.Instance },
		func(ct Container) int { return ct.VMID },
		func(ct Container) time.Time { return ct.LastBackup },
		func(ct Container, t time.Time) Container { ct.LastBackup = t; return ct },
		cloneContainer,
		func(items []Container, i, j int) bool { return items[i].VMID < items[j].VMID },
	)
	s.LastUpdate = time.Now()
}
