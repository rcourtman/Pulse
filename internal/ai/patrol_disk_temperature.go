package ai

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// diskTemperatureLimits is the alert disk temperature policy for one disk,
// the same policy disk temperature alerts and the Physical Disks Health
// verdict judge heat by. A disk is hot at the trigger and stays hot until it
// cools to the clear value. A zero trigger means disk temperature alerting is
// off, so no reading counts as heat.
type diskTemperatureLimits struct {
	trigger float64
	clear   float64
}

// diskTemperatureLimitsFor resolves the policy for a disk of the given type
// that the given host agent reports from the user's alert configuration, so
// that host's Disk Temp override applies, or from the factory alert
// configuration when Patrol has no threshold provider.
func diskTemperatureLimitsFor(provider ThresholdProvider, host alerts.DiskTemperatureHost, diskType string) diskTemperatureLimits {
	if provider != nil {
		return newDiskTemperatureLimits(provider.GetDiskTemperatureThreshold(host, diskType))
	}
	return factoryDiskTemperatureLimits(diskType)
}

// trueNASDiskTemperatureLimitsFor resolves the policy for one TrueNAS disk the
// way its temperature alert does, from the user's alert configuration, so the
// disk's own override and the TrueNAS-wide value apply, or from the factory
// alert configuration when Patrol has no threshold provider.
func trueNASDiskTemperatureLimitsFor(provider ThresholdProvider, resourceID, diskType string) diskTemperatureLimits {
	if provider != nil {
		return newDiskTemperatureLimits(provider.GetTrueNASDiskTemperatureThreshold(resourceID, diskType))
	}
	return factoryDiskTemperatureLimits(diskType)
}

// physicalDiskTemperatureLimits resolves the policy for one physical disk
// resource under the alert that judges its heat. A TrueNAS disk has its own
// temperature alert, judged by the disk's override, the TrueNAS-wide value and
// the per-type policy; its TrueNAS system carries a synthetic agent ID, so the
// host walk would wrongly treat that system as a host agent. Any other disk is
// judged as a SMART disk of the host agent that reports it.
func physicalDiskTemperatureLimits(provider ThresholdProvider, disk unifiedresources.Resource, owners map[string]unifiedresources.Resource) diskTemperatureLimits {
	diskType := ""
	if disk.PhysicalDisk != nil {
		diskType = disk.PhysicalDisk.DiskType
	}
	if alerts.IsTrueNASDiskResource(disk) {
		return trueNASDiskTemperatureLimitsFor(provider, disk.ID, diskType)
	}
	return diskTemperatureLimitsFor(provider, physicalDiskTemperatureHost(disk, owners), diskType)
}

// factoryDiskTemperatureLimits is the policy for a disk of the given type
// under the factory alert configuration.
func factoryDiskTemperatureLimits(diskType string) diskTemperatureLimits {
	if threshold := alerts.DefaultDiskTemperatureThreshold(diskType); threshold != nil {
		return newDiskTemperatureLimits(threshold.Trigger, threshold.Clear)
	}
	return diskTemperatureLimits{}
}

// newDiskTemperatureLimits builds limits from an alert trigger and clear
// value. A non-positive trigger is off; a clear value missing or above the
// trigger leaves no band below it.
func newDiskTemperatureLimits(trigger, clear float64) diskTemperatureLimits {
	if trigger <= 0 {
		return diskTemperatureLimits{}
	}
	if clear <= 0 || clear > trigger {
		clear = trigger
	}
	return diskTemperatureLimits{trigger: trigger, clear: clear}
}

// hot reports a reading at or above the alert trigger.
func (l diskTemperatureLimits) hot(temperature int) bool {
	return l.trigger > 0 && temperature > 0 && float64(temperature) >= l.trigger
}

// cooled reports a reading at or below the clear value, where a disk
// temperature alert recovers. With no band below the trigger, a reading at
// the trigger still fires, so only a reading under it has cooled.
func (l diskTemperatureLimits) cooled(temperature int) bool {
	if l.trigger <= 0 || temperature <= 0 {
		return true
	}
	if l.clear >= l.trigger {
		return float64(temperature) < l.trigger
	}
	return float64(temperature) <= l.clear
}

// physicalDiskOwnerIndex indexes, by resource ID, the resources a physical disk
// can reach its reporting host agent through: agent-bearing machines and the
// storage pools disks hang off.
func physicalDiskOwnerIndex(urp UnifiedResourceProvider) map[string]unifiedresources.Resource {
	index := make(map[string]unifiedresources.Resource)
	for _, resource := range urp.GetAll() {
		if resource.Agent != nil || unifiedresources.CanonicalResourceType(resource.Type) == unifiedresources.ResourceTypeStorage {
			index[resource.ID] = resource
		}
	}
	return index
}

// physicalDiskTemperatureHost returns the host agent that reports a physical
// disk, the host its disk temperature alerts are evaluated under: the agent on
// the machine the disk is parented to or, for a disk parented to a storage
// pool (Unraid array and cache disks), the agent on the pool's machine. A disk
// no agent reports gets the hostless policy. TrueNAS disks do not come here;
// physicalDiskTemperatureLimits judges them by their own alert.
func physicalDiskTemperatureHost(disk unifiedresources.Resource, owners map[string]unifiedresources.Resource) alerts.DiskTemperatureHost {
	parentID := disk.ParentID
	for hops := 0; parentID != nil && hops < 2; hops++ {
		parent, ok := owners[strings.TrimSpace(*parentID)]
		if !ok {
			break
		}
		if parent.Agent != nil && strings.TrimSpace(parent.Agent.AgentID) != "" {
			return alerts.DiskTemperatureHost{
				ID:                strings.TrimSpace(parent.Agent.AgentID),
				LinkedNodeID:      parent.Agent.LinkedNodeID,
				LinkedVMID:        parent.Agent.LinkedVMID,
				LinkedContainerID: parent.Agent.LinkedContainerID,
			}
		}
		if unifiedresources.CanonicalResourceType(parent.Type) != unifiedresources.ResourceTypeStorage {
			break
		}
		parentID = parent.ParentID
	}
	return alerts.DiskTemperatureHost{}
}
