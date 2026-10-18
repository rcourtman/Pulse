package models

// GuestObservationsForNodes detaches the last accepted producer tuples for one
// connection's failed nodes. It is not a canonical/merged consumer projection:
// missing enumeration must not rebuild memory, disk or CPU from partial views.
// Copy only these guests, not unrelated hosts, disks or Kubernetes inventories.
func (s *State) GuestObservationsForNodes(instance string, nodes map[string]struct{}) ([]VM, []Container) {
	if s == nil || len(nodes) == 0 {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var vms []VM
	var containers []Container
	for _, vm := range s.VMs {
		if _, selected := nodes[vm.Node]; !selected || vm.Instance != instance {
			continue
		}
		copied := cloneVM(vm)
		copied.OnBoot = cloneBoolPtr(vm.OnBoot)
		vms = append(vms, copied)
	}
	for _, ct := range s.Containers {
		if _, selected := nodes[ct.Node]; !selected || ct.Instance != instance {
			continue
		}
		copied := cloneContainer(ct)
		copied.OnBoot = cloneBoolPtr(ct.OnBoot)
		containers = append(containers, copied)
	}
	return vms, containers
}
