package host

import "errors"

const (
	ProxmoxLXCCollectionComplete = "complete"
	ProxmoxLXCCollectionPartial  = "partial"
	ProxmoxLXCMaxContainers      = 128
)

// ValidateCollection checks the shared helper/report completeness boundary.
// Legacy omission is complete-only. Global unavailability has no inventory;
// partial (including all-failed) inventories identify each omitted container.
// Mount/value and node/name admission remain with the monitoring consumer.
func (i *ProxmoxLXCInventory) ValidateCollection() error {
	invalid := errors.New("invalid Proxmox LXC filesystem completeness")
	if i == nil {
		return invalid
	}
	if i.Status == "" {
		// Keep legacy row-by-row admission and truncation unchanged.
		if len(i.OmittedVMIDs) != 0 {
			return invalid
		}
		return nil
	}
	if len(i.Containers)+len(i.OmittedVMIDs) > ProxmoxLXCMaxContainers {
		return invalid
	}
	switch i.Status {
	case ProxmoxLXCCollectionComplete:
		if len(i.OmittedVMIDs) != 0 {
			return invalid
		}
	case ProxmoxLXCCollectionPartial:
		if len(i.OmittedVMIDs) == 0 {
			return invalid
		}
	default:
		return invalid
	}
	seen := make(map[int]struct{}, len(i.Containers)+len(i.OmittedVMIDs))
	admit := func(vmid int) bool {
		if vmid < 100 || vmid > 999999999 {
			return false
		}
		if _, duplicate := seen[vmid]; duplicate {
			return false
		}
		seen[vmid] = struct{}{}
		return true
	}
	for _, container := range i.Containers {
		if !admit(container.VMID) {
			return invalid
		}
	}
	for _, vmid := range i.OmittedVMIDs {
		if !admit(vmid) {
			return invalid
		}
	}
	return nil
}
