package alerts

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// proxmoxDiskTemperatureMetric is the metric of a Proxmox physical disk's
// temperature alert, the same metric CheckHost raises for an agent's disks.
const proxmoxDiskTemperatureMetric = "diskTemperature"

// proxmoxDiskResourceType is the resource type PVE disk alerts carry in their
// spec and metadata (see CheckDiskHealth).
const proxmoxDiskResourceType = "proxmox-disk"

// ProxmoxDiskTemperatureReading is one Proxmox physical disk's temperature as
// the PVE disk poller judged it.
type ProxmoxDiskTemperatureReading struct {
	Disk proxmox.Disk
	// Celsius is the reading collected this poll, or 0 when there is none:
	// no sensor reports the disk, it is in standby, or normalization kept a
	// last-known value it did not collect now.
	Celsius int
	// AgentOwned says a linked Pulse agent lists the disk in its SMART
	// report, so the agent's CheckHost owns its temperature alert.
	AgentOwned bool
	// Excluded says the linked agent's --disk-exclude patterns match the
	// device, which excludes it from disk alerts.
	Excluded bool
}

// CheckProxmoxDiskTemperature raises a Proxmox physical disk's temperature
// alert under the disk temperature policy (DiskTemperatureThreshold): the
// same per-type thresholds that judge an agent's disks, TrueNAS disks and the
// Physical Disks verdict. Without it a disk that only the node's sensors
// report could read Running Hot and never alert.
//
// A disk a linked agent lists is the agent's: its open alert here closes as
// moved to the agent rather than recovered, so the disk never alerts twice.
// An excluded disk, or a policy switched off by the agent Disk Temp default,
// closes its alert. A disk with no current reading keeps its alert as it is,
// since no reading shows it cooled.
func (m *Manager) CheckProxmoxDiskTemperature(instance, node string, reading ProxmoxDiskTemperatureReading) {
	if m == nil {
		return
	}
	disk := reading.Disk
	resourceID := proxmoxDiskCanonicalResourceID(instance, node, disk.DevPath)
	resourceName := fmt.Sprintf("%s (%s)", disk.Model, disk.DevPath)
	diskType := strings.ToLower(strings.TrimSpace(disk.Type))

	m.mu.RLock()
	enabled := m.config.Enabled
	threshold := m.proxmoxDiskTemperatureThresholdNoLock(diskType, false)
	m.mu.RUnlock()
	if !enabled {
		return
	}

	if reading.AgentOwned || reading.Excluded || threshold == nil || threshold.Trigger <= 0 {
		spec, err := buildCanonicalMetricSpec(resourceID, resourceName, unifiedresources.ResourceType(proxmoxDiskResourceType), proxmoxDiskTemperatureMetric, nil)
		if err != nil {
			return
		}
		var resolution *AlertResolution
		if reading.AgentOwned && !reading.Excluded {
			resolution = &AlertResolution{Reason: AlertResolutionMovedToAgent}
		}
		m.releaseCanonicalMetricAlert(spec, resourceName, node, instance, proxmoxDiskResourceType, 0, resolution)
		return
	}
	if reading.Celsius <= 0 {
		return
	}

	spec, err := buildCanonicalMetricSpec(resourceID, resourceName, unifiedresources.ResourceType(proxmoxDiskResourceType), proxmoxDiskTemperatureMetric, threshold)
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resourceID).
			Str("node", node).
			Str("disk", disk.DevPath).
			Msg("Skipping invalid canonical Proxmox disk temperature metric spec")
		return
	}
	metadata := proxmoxDiskAlertMetadata(disk)
	metadata["metric"] = proxmoxDiskTemperatureMetric
	// A config save re-judges the open alert against its disk type.
	metadata["diskType"] = diskType
	value := float64(reading.Celsius)
	m.checkMetricWithCanonicalSpec(spec, resourceName, node, instance, proxmoxDiskResourceType, value, threshold, &metricOptions{
		Metadata: metadata,
		Message:  fmt.Sprintf("Disk temperature at %s", formatMetricValue(value, "°C")),
	})
}

// proxmoxDiskTemperatureThresholdNoLock is the disk temperature policy for a
// Proxmox disk of diskType, or the lowest per-type trigger for an alert whose
// disk type was never recorded, so a config save never resolves an alert the
// next poll raises again. Callers must hold m.mu.
func (m *Manager) proxmoxDiskTemperatureThresholdNoLock(diskType string, typeUnknown bool) *HysteresisThreshold {
	if typeUnknown {
		return cloneThreshold(m.lowestHostDiskTemperatureThresholdNoLock(m.config.AgentDefaults.DiskTemperature))
	}
	return cloneThreshold(m.hostDiskTemperatureThresholdNoLock(m.config.AgentDefaults.DiskTemperature, false, diskType))
}
