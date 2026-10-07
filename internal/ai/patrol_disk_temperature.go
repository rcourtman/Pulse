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
	var trigger, clear float64
	if provider != nil {
		trigger, clear = provider.GetDiskTemperatureThreshold(host, diskType)
	} else if threshold := alerts.DefaultDiskTemperatureThreshold(diskType); threshold != nil {
		trigger, clear = threshold.Trigger, threshold.Clear
	}
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
// no agent reports gets the hostless policy.
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
