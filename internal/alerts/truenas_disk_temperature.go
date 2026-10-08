package alerts

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
