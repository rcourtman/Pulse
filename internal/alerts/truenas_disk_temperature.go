package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// trueNASDiskTemperatureDefaultNoLock returns the default tier of a TrueNAS
// disk's temperature threshold. A TrueNAS-wide value the user saved under
// TrueNAS Disks applies to every TrueNAS disk, as a host Disk Temp override
// does for an agent's disks. Without one the disk follows the disk
// temperature policy for its type, the threshold DiskTemperatureThreshold
// returns, so its alert agrees with the TrueNAS storage table and Patrol. A
// per-disk override still beats both. Callers must hold m.mu.
func (m *Manager) trueNASDiskTemperatureDefaultNoLock(diskType string, typeUnknown bool) *HysteresisThreshold {
	if explicit := m.config.TrueNASDiskDefaults.Temperature; explicit != nil {
		return cloneThreshold(explicit)
	}
	if typeUnknown {
		return cloneThreshold(m.lowestHostDiskTemperatureThresholdNoLock(m.config.AgentDefaults.DiskTemperature))
	}
	return cloneThreshold(m.hostDiskTemperatureThresholdNoLock(m.config.AgentDefaults.DiskTemperature, false, diskType))
}

// IsTrueNASDiskResource reports whether the unified evaluator judges a
// resource as a TrueNAS disk (truenas-disk), whose temperature alert resolves
// its threshold through TrueNASDiskTemperatureThreshold rather than through
// the host agent that reports it.
func IsTrueNASDiskResource(resource unifiedresources.Resource) bool {
	typeKey, ok := unifiedAlertResourceType(resource)
	return ok && typeKey == "truenas-disk"
}

// TrueNASDiskTemperatureThreshold is the disk heat policy for one TrueNAS
// disk, resolved by the same effectiveAlertPolicyNoLock tiers its temperature
// alert uses: an override stored under the disk's resource ID, then the
// TrueNAS-wide TrueNAS Disks value, then the per-type policy. An override or
// TrueNAS Disks default that switches the disk's alerts off leaves no
// threshold (nil); a non-positive trigger is off too. Like
// HostDiskTemperatureThreshold it ignores the global and per-platform alert
// switches, which silence the alert without changing the policy. A nil
// manager resolves the factory configuration, where no TrueNAS-wide value is
// set.
func (m *Manager) TrueNASDiskTemperatureThreshold(resourceID, diskType string) *HysteresisThreshold {
	if m == nil {
		return DefaultDiskTemperatureThreshold(diskType)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	thresholds := m.effectiveAlertPolicyNoLock(alertPolicyQuery{
		TypeKey:    "truenas-disk",
		ResourceID: unifiedresources.CanonicalResourceID(resourceID),
		DiskType:   strings.ToLower(strings.TrimSpace(diskType)),
	}).Thresholds
	if thresholds.Disabled {
		return nil
	}
	return cloneThreshold(thresholds.Temperature)
}
