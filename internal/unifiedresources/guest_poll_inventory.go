package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// ProxmoxPollInventory projects the previous disk and interface evidence into
// detached poller rows without first cloning the intermediate view slices.
func (v VMView) ProxmoxPollInventory() ([]models.Disk, []models.GuestNetworkInterface) {
	return proxmoxPollInventory(v.r)
}

// ProxmoxPollInventory is the container counterpart of VMView.ProxmoxPollInventory.
func (v ContainerView) ProxmoxPollInventory() ([]models.Disk, []models.GuestNetworkInterface) {
	return proxmoxPollInventory(v.r)
}

func proxmoxPollInventory(r *Resource) (disks []models.Disk, interfaces []models.GuestNetworkInterface) {
	if r == nil || r.Proxmox == nil {
		return nil, nil
	}
	if source := r.Proxmox.Disks; len(source) > 0 {
		disks = make([]models.Disk, len(source))
		for i, disk := range source {
			disks[i] = models.Disk{
				Total: disk.Total, Used: disk.Used, Free: disk.Free, Usage: disk.Usage,
				Mountpoint: disk.Mountpoint, Type: disk.Filesystem, Device: disk.Device,
			}
		}
	}
	if source := r.Proxmox.NetworkInterfaces; len(source) > 0 {
		interfaces = make([]models.GuestNetworkInterface, len(source))
		for i, iface := range source {
			interfaces[i] = models.GuestNetworkInterface{
				Name:      iface.Name,
				MAC:       iface.MAC,
				Addresses: append([]string(nil), iface.Addresses...),
				RXBytes:   max(0, int64(iface.RXBytes)),
				TXBytes:   max(0, int64(iface.TXBytes)),
				Bridge:    iface.Bridge,
				VLAN:      iface.VLAN,
				Firewall:  iface.Firewall,
			}
		}
	}
	return disks, interfaces
}
