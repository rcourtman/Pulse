package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

// smartTemperatureDiskTypes are the disk types whose temperature reaches a
// Proxmox node only through SMART: the transports a host agent or the
// pulse-sensors wrapper reports (sata, sas) and the form factors Proxmox's own
// disk inventory reports for those same disks (hdd, ssd). Proxmox also types
// a non-rotational USB device ssd. NVMe temperatures come from kernel hwmon
// and arrive even on a legacy setup.
var smartTemperatureDiskTypes = map[string]struct{}{
	"sata": {},
	"sas":  {},
	"hdd":  {},
	"ssd":  {},
}

// refreshProxmoxSensorSetupLocked derives ProxmoxData.SensorSetupOutdated on
// every Proxmox node, as an explicit true or false. SSH temperature monitoring
// set up before v6.0.0-rc.6 locks the node's key to `sensors -j`: the payload
// still parses and carries CPU and NVMe temperatures, but SMART disk
// temperatures can never arrive, so they stay blank with no error anywhere.
// The flag is data-gated three ways: the node's last temperature collection
// succeeded, its payload was the legacy format, and a disk under it waits on a
// SMART temperature it does not have now. A last known temperature kept for a
// disk that could not be read now (standby, a host agent past its lease) is not
// a current one (diskinventory.TemperatureCollected), while a disk whose
// temperature arrives another way, such as a linked host agent, falls out.
// Disks count through their canonical parent, so an agent the registry has not
// linked to the node contributes none. Deriving the flag here lets every
// surface that lists nodes read it without loading the node's disk inventory.
// It must run after buildChildCounts, which resolves each disk's canonical
// parent node.
func (rr *ResourceRegistry) refreshProxmoxSensorSetupLocked() {
	waiting := make(map[string]struct{})
	for _, resource := range rr.resources {
		if resource == nil {
			continue
		}
		if resource.Proxmox != nil {
			// A fresh pointer every time: a facet copied by an earlier merge or
			// clone must not see this generation's verdict.
			resource.Proxmox.SensorSetupOutdated = nil
			if CanonicalResourceType(resource.Type) == ResourceTypeAgent {
				resource.Proxmox.SensorSetupOutdated = sensorSetupVerdict(false)
			}
		}
		if resource.ParentID != nil && physicalDiskWaitsOnSMARTTemperature(resource) {
			waiting[*resource.ParentID] = struct{}{}
		}
	}
	for parentID := range waiting {
		node := rr.resources[parentID]
		if node == nil || node.Proxmox == nil || CanonicalResourceType(node.Type) != ResourceTypeAgent {
			continue
		}
		details := node.Proxmox.TemperatureDetails
		node.Proxmox.SensorSetupOutdated = sensorSetupVerdict(details != nil && details.Available && details.LegacySensorsFormat)
	}
}

func sensorSetupVerdict(outdated bool) *bool {
	return &outdated
}

func physicalDiskWaitsOnSMARTTemperature(resource *Resource) bool {
	if CanonicalResourceType(resource.Type) != ResourceTypePhysicalDisk || resource.PhysicalDisk == nil {
		return false
	}
	disk := resource.PhysicalDisk
	if _, ok := smartTemperatureDiskTypes[strings.ToLower(strings.TrimSpace(disk.DiskType))]; !ok {
		return false
	}
	return !diskinventory.TemperatureCollected(disk.Temperature, disk.Collection)
}
