package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const HostOfflineAlertType = "host-offline"

type smartCounterSnapshot struct {
	UDMACRCErrors int64
	LastObserved  time.Time
}

func hostResourceID(hostID string) string {
	trimmed := strings.TrimSpace(hostID)
	if trimmed == "" {
		return "agent:unknown"
	}
	return fmt.Sprintf("agent:%s", trimmed)
}

// Every alert CheckHost raises about something other than the machine itself
// (a filesystem, disk, array or custom sensor) targets a child resource ID,
// and the alert card's monitoring policy writes to that ID. These alerts
// name their own type so the card does not describe retiring the machine.
// Each type keeps "agent" among its alert policy keys
// (config.CanonicalResourceTypeKeys), so agent thresholds and switches still
// govern them.
const (
	hostDiskAlertResourceType    = "agent-disk"    // filesystems, disk temperatures, SMART disks
	hostStorageAlertResourceType = "agent-storage" // RAID and Unraid arrays
	hostSensorAlertResourceType  = "agent-sensor"  // custom sensors
)

// hostChildAlertMetadata clones a host's base alert metadata for an alert
// about something the agent reports rather than the machine itself.
func hostChildAlertMetadata(base map[string]interface{}, resourceType string) map[string]interface{} {
	metadata := cloneMetadata(base)
	metadata["resourceType"] = resourceType
	return metadata
}

func stripHostResourcePrefix(resourceID string) string {
	trimmed := strings.TrimSpace(resourceID)
	trimmed = strings.TrimPrefix(trimmed, "agent:")
	return strings.TrimSpace(trimmed)
}

func hostDisplayName(host models.Host) string {
	base := "Agent"
	if name := strings.TrimSpace(host.DisplayName); name != "" {
		base = name
	} else if name := strings.TrimSpace(host.Hostname); name != "" {
		base = name
	} else if host.ID != "" {
		base = host.ID
	}

	// When a host agent is linked to a Proxmox node/VM/container, qualify its
	// name so its alerts are not confused with the linked resource's alerts.
	if strings.TrimSpace(host.LinkedNodeID) != "" ||
		strings.TrimSpace(host.LinkedVMID) != "" ||
		strings.TrimSpace(host.LinkedContainerID) != "" {
		if strings.EqualFold(base, "Agent") {
			return "Host Agent"
		}
		if !strings.Contains(strings.ToLower(base), "host agent") {
			return fmt.Sprintf("%s (Host Agent)", base)
		}
	}

	return base
}

func hostInstanceName(host models.Host) string {
	if platform := strings.TrimSpace(host.Platform); platform != "" {
		return platform
	}
	if osName := strings.TrimSpace(host.OSName); osName != "" {
		return osName
	}
	return "Agent"
}

// resolveHostThresholdsNoLock resolves the effective thresholds for a host agent.
// Explicit host-agent overrides win. Otherwise, linked node/guest overrides are
// inherited so the host agent follows the logical resource it augments.
// Callers must hold m.mu when reading config through this helper.
func (m *Manager) resolveHostThresholdsNoLock(hostID, linkedNodeID, linkedVMID, linkedContainerID string) ThresholdConfig {
	base := m.defaultThresholdsForResourceType("agent")

	if override, exists := m.hostThresholdOverrideNoLock(hostID, linkedNodeID, linkedVMID, linkedContainerID); exists {
		return m.applyThresholdOverride(base, override)
	}

	return base
}

// hostThresholdOverrideNoLock returns the first override in the host-agent
// resolution chain (host, linked node, linked guest), mirroring
// resolveHostThresholdsNoLock's precedence. Callers must hold m.mu.
func (m *Manager) hostThresholdOverrideNoLock(hostID, linkedNodeID, linkedVMID, linkedContainerID string) (ThresholdConfig, bool) {
	if hostID = strings.TrimSpace(hostID); hostID != "" {
		if override, exists := m.thresholdOverrideForResourceNoLock(hostID); exists {
			return override, true
		}
	}

	if linkedNodeID = strings.TrimSpace(linkedNodeID); linkedNodeID != "" {
		if override, exists := m.thresholdOverrideForResourceNoLock(linkedNodeID); exists {
			return override, true
		}
	}

	if linkedVMID = strings.TrimSpace(linkedVMID); linkedVMID != "" {
		if override, exists := lookupGuestOverride(m.config.Overrides, nil, linkedVMID); exists {
			return override, true
		}
	}

	if linkedContainerID = strings.TrimSpace(linkedContainerID); linkedContainerID != "" {
		if override, exists := lookupGuestOverride(m.config.Overrides, nil, linkedContainerID); exists {
			return override, true
		}
	}

	return ThresholdConfig{}, false
}

// resolveHostAlertThresholdsNoLock resolves thresholds for persisted host-agent alerts.
// Alert metadata carries the link context needed to inherit node/guest overrides.
// Callers must hold m.mu when reading config through this helper.
func (m *Manager) resolveHostAlertThresholdsNoLock(alert *Alert, resourceID string) ThresholdConfig {
	hostID := stripHostResourcePrefix(resourceID)
	if idx := strings.Index(hostID, "/"); idx >= 0 {
		hostID = hostID[:idx]
	}

	linkedNodeID := ""
	linkedVMID := ""
	linkedContainerID := ""
	if alert != nil {
		if metadataHostID := metadataStringValue(alert.Metadata, "hostId"); metadataHostID != "" {
			hostID = metadataHostID
		}
		linkedNodeID = metadataStringValue(alert.Metadata, "linkedNodeId")
		linkedVMID = metadataStringValue(alert.Metadata, "linkedVmId")
		linkedContainerID = metadataStringValue(alert.Metadata, "linkedContainerId")
	}

	thresholds := m.resolveHostThresholdsNoLock(hostID, linkedNodeID, linkedVMID, linkedContainerID)
	// A disk temperature alert is judged against the per-type threshold
	// CheckHost evaluates for that disk, so a config save does not resolve an
	// alert the next report would raise again.
	if alert != nil && alert.Type == "diskTemperature" {
		override, exists := m.hostThresholdOverrideNoLock(hostID, linkedNodeID, linkedVMID, linkedContainerID)
		overridden := exists && override.DiskTemperature != nil
		if diskType, known := alert.Metadata["diskType"].(string); known || overridden {
			thresholds.DiskTemperature = m.hostDiskTemperatureThresholdNoLock(thresholds.DiskTemperature, overridden, diskType)
		} else {
			// Alerts persisted before CheckHost recorded diskType carry no
			// disk type until their next firing evaluation.
			thresholds.DiskTemperature = m.lowestHostDiskTemperatureThresholdNoLock(thresholds.DiskTemperature)
		}
	}
	return thresholds
}

func sanitizeHostComponent(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}

	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				builder.WriteRune('-')
				lastHyphen = true
			}
		}
	}

	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}

// sanitizeRAIDDevice sanitizes RAID device names for use in resource IDs.
func sanitizeRAIDDevice(device string) string {
	// Remove /dev/ prefix if present
	device = strings.TrimPrefix(device, "/dev/")
	return sanitizeHostComponent(device)
}

func hostDiskResourceIDWithPrefix(host models.Host, disk models.Disk, resourcePrefix string) (string, string) {
	label := strings.TrimSpace(disk.Mountpoint)
	if label == "" {
		label = strings.TrimSpace(disk.Device)
	}
	if label == "" {
		label = "disk"
	}
	resourceID := fmt.Sprintf("%s/disk:%s", resourcePrefix, sanitizeHostComponent(label))
	resourceName := fmt.Sprintf("%s (%s)", hostDisplayName(host), label)
	return resourceID, resourceName
}

func hostDiskResourceID(host models.Host, disk models.Disk) (string, string) {
	return hostDiskResourceIDWithPrefix(host, disk, hostResourceID(host.ID))
}

func hostSMARTDiskResourceID(host models.Host, disk models.HostDiskSMART) (string, string) {
	label := strings.TrimSpace(strings.TrimPrefix(disk.Device, "/dev/"))
	if label == "" {
		label = strings.TrimSpace(disk.Serial)
	}
	if label == "" {
		label = strings.TrimSpace(disk.WWN)
	}
	if label == "" {
		label = strings.TrimSpace(disk.Model)
	}
	if label == "" {
		label = "smart-disk"
	}

	resourceID := fmt.Sprintf("%s/disk:%s", hostResourceID(host.ID), sanitizeHostComponent(label))
	resourceName := fmt.Sprintf("%s (%s)", hostDisplayName(host), label)
	return resourceID, resourceName
}

// CheckHost evaluates host agent telemetry for alerts.
func (m *Manager) CheckHost(host models.Host) {
	if host.ID == "" {
		return
	}

	// Cache display name so host alerts show the user-configured name.
	m.UpdateNodeDisplayName("", host.Hostname, host.DisplayName)

	// Fresh telemetry marks the host as online and clears offline tracking.
	m.HandleHostOnline(host)

	// Apply the link outcome in report order, including disabled exits.
	reportSeq := m.beginHostAgentReport(host.ID)

	m.mu.RLock()
	alertsEnabled := m.config.Enabled
	disableAllAgents, _ := m.alertPolicyTypeSwitchesNoLock("agent")
	thresholds := m.resolveHostThresholdsNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	// An explicit disk temperature override (host or inherited linked-resource)
	// beats the per-type defaults in DiskTempByType.
	diskTempOverridden := false
	diskOverridden := false
	if override, exists := m.hostThresholdOverrideNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID); exists {
		diskTempOverridden = override.DiskTemperature != nil
		diskOverridden = override.Disk != nil
	}
	m.mu.RUnlock()

	// While this agent evaluates nothing, its linked node keeps its own usage
	// alerts; the link is registered below once the evaluated metrics are known.
	if !alertsEnabled {
		m.applyHostAgentNodeLink(hostAgentNodeLink{agentID: host.ID}, reportSeq)
		return
	}

	if disableAllAgents {
		m.applyHostAgentNodeLink(hostAgentNodeLink{agentID: host.ID}, reportSeq)
		// Clear any existing host alerts when all host alerts are disabled
		m.clearHostMetricAlerts(host.ID)
		m.clearHostDiskAlerts(host.ID)
		m.clearHostRAIDAlerts(host.ID)
		m.clearHostUnraidAlerts(host.ID)
		m.clearHostCustomSensorAlerts(host.ID)
		return
	}

	if thresholds.Disabled {
		m.applyHostAgentNodeLink(hostAgentNodeLink{agentID: host.ID}, reportSeq)
		m.clearHostMetricAlerts(host.ID)
		m.clearHostDiskAlerts(host.ID)
		m.clearHostRAIDAlerts(host.ID)
		m.clearHostUnraidAlerts(host.ID)
		m.clearHostCustomSensorAlerts(host.ID)
		return
	}

	resourceID := hostResourceID(host.ID)
	resourceName := hostDisplayName(host)
	nodeName := strings.TrimSpace(host.Hostname)
	instanceName := hostInstanceName(host)

	baseMetadata := map[string]interface{}{
		"resourceType":       "agent",
		alertPlatformTypeKey: string(unifiedresources.SourceAgent),
		"hostId":             host.ID,
		"hostname":           host.Hostname,
		"displayName":        host.DisplayName,
		"platform":           host.Platform,
		"osName":             host.OSName,
		"osVersion":          host.OSVersion,
		"agentVersion":       host.AgentVersion,
		"architecture":       host.Architecture,
	}
	if linkedNodeID := strings.TrimSpace(host.LinkedNodeID); linkedNodeID != "" {
		baseMetadata["linkedNodeId"] = linkedNodeID
	}
	if linkedVMID := strings.TrimSpace(host.LinkedVMID); linkedVMID != "" {
		baseMetadata["linkedVmId"] = linkedVMID
	}
	if linkedContainerID := strings.TrimSpace(host.LinkedContainerID); linkedContainerID != "" {
		baseMetadata["linkedContainerId"] = linkedContainerID
	}
	if len(host.Tags) > 0 {
		baseMetadata["tags"] = append([]string(nil), host.Tags...)
	}
	m.syncHostCustomSensorAlerts(host, nodeName, instanceName, baseMetadata)

	cpuEvaluated := false
	if thresholds.CPU != nil {
		cpuMetadata := cloneMetadata(baseMetadata)
		cpuMetadata["metric"] = "cpu"
		cpuMetadata["cpuUsagePercent"] = host.CPUUsage
		if host.CPUCount > 0 {
			cpuMetadata["cpuCount"] = host.CPUCount
		}
		spec, err := buildCanonicalMetricSpec(resourceID, resourceName, unifiedresources.ResourceTypeAgent, "cpu", thresholds.CPU)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", resourceID).
				Str("host", resourceName).
				Msg("Skipping invalid canonical host CPU metric spec")
		} else {
			cpuEvaluated = m.checkMetricWithCanonicalSpec(spec, resourceName, nodeName, instanceName, "agent", host.CPUUsage, thresholds.CPU, &metricOptions{Metadata: cpuMetadata})
		}
	} else {
		m.releaseHostUsageMetric(resourceID, resourceName, nodeName, instanceName, unifiedresources.ResourceTypeAgent, "agent", "cpu")
	}

	memoryEvaluated := false
	if thresholds.Memory != nil {
		memMetadata := cloneMetadata(baseMetadata)
		memMetadata["metric"] = "memory"
		memMetadata["memoryUsagePercent"] = host.Memory.Usage
		if host.Memory.Total > 0 {
			memMetadata["memoryTotalBytes"] = host.Memory.Total
			memMetadata["memoryUsedBytes"] = host.Memory.Used
			memMetadata["memoryFreeBytes"] = host.Memory.Free
		}
		spec, err := buildCanonicalMetricSpec(resourceID, resourceName, unifiedresources.ResourceTypeAgent, "memory", thresholds.Memory)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", resourceID).
				Str("host", resourceName).
				Msg("Skipping invalid canonical host memory metric spec")
		} else if !host.Memory.HasKnownUsage() && !spec.Disabled {
			m.interruptMetricRun(spec)
		} else {
			memoryEvaluated = m.checkMetricWithCanonicalSpec(spec, resourceName, nodeName, instanceName, "agent", host.Memory.Usage, thresholds.Memory, &metricOptions{Metadata: memMetadata})
		}
	} else if thresholds.Memory == nil {
		m.releaseHostUsageMetric(resourceID, resourceName, nodeName, instanceName, unifiedresources.ResourceTypeAgent, "agent", "memory")
	}

	if thresholds.DiskTemperature != nil && thresholds.DiskTemperature.Trigger > 0 {
		// An empty SMART list means collection failed or is unsupported for
		// this report, not that every disk left, so existing alerts are held.
		if len(host.Sensors.SMART) > 0 {
			seenDiskTemps := make(map[string]struct{}, len(host.Sensors.SMART))
			for _, disk := range host.Sensors.SMART {
				// A listed disk in standby, or without a temperature after a
				// failed probe, is still present. Its alert holds until a fresh
				// reading resolves it instead of clearing and re-raising.
				tempResourceID := hostDiskTemperatureResourceID(host.ID, disk.Device)
				seenDiskTemps[tempResourceID] = struct{}{}
				if hostDiskTemperatureObserved(disk) {
					m.mu.RLock()
					effectiveTempThreshold := m.hostDiskTemperatureThresholdNoLock(thresholds.DiskTemperature, diskTempOverridden, disk.Type)
					m.mu.RUnlock()

					tempResourceName := fmt.Sprintf("%s (%s Temp)", hostDisplayName(host), disk.Device)

					diskTempMetadata := hostChildAlertMetadata(baseMetadata, hostDiskAlertResourceType)
					diskTempMetadata["metric"] = "diskTemperature"
					diskTempMetadata["device"] = disk.Device
					diskTempMetadata["temperature"] = disk.Temperature
					diskTempMetadata["model"] = disk.Model
					diskTempMetadata["diskType"] = disk.Type
					spec, err := buildCanonicalMetricSpec(tempResourceID, tempResourceName, unifiedresources.ResourceType("agent-disk"), "diskTemperature", effectiveTempThreshold)
					if err != nil {
						log.Warn().
							Err(err).
							Str("resourceID", tempResourceID).
							Str("host", resourceName).
							Str("device", disk.Device).
							Msg("Skipping invalid canonical host disk temperature metric spec")
						m.interruptHostDiskTemperatureRun(tempResourceID)
						continue
					}

					m.checkMetricWithCanonicalSpec(spec, tempResourceName, nodeName, disk.Device, "agent", float64(disk.Temperature), effectiveTempThreshold, &metricOptions{Metadata: diskTempMetadata})
					m.rememberHostDiskTemperaturePending(host, disk, tempResourceID)
				} else {
					m.interruptHostDiskTemperatureRun(tempResourceID)
				}
			}
			// A disk missing from consecutive non-empty reports was removed,
			// replaced or renamed, so no later reading will resolve its alert.
			m.cleanupHostDiskTemperatureAlerts(host.ID, seenDiskTemps)
		} else {
			m.interruptHostDiskTemperatureRuns(host.ID)
		}
	} else {
		// Disk temperature alerting is off for this host, so no later reading
		// will resolve an alert it raised while it was on.
		m.clearHostDiskTemperatureAlerts(host.ID)
	}

	seenDisks := make(map[string]struct{}, len(host.Disks))
	// A linked node's disk metric is this agent's summary filesystem (root when
	// reported), so the agent owns it only while it evaluates that filesystem.
	summaryDiskResourceID := ""
	if summary, ok := models.SummaryDisk(host.Disks); ok {
		summaryDiskResourceID, _ = hostDiskResourceID(host, summary)
	}
	evaluatesSummaryDisk := false
	summaryDiskLive := false
	if host.LinkedNodeID != "" {
		// Preserve node ownership even when this report carries no SMART list.
		m.clearHostSMARTDiskAlerts(host.ID)
	} else {
		var seenSMARTDisks map[string]struct{}
		if len(host.Sensors.SMART) > 0 {
			seenSMARTDisks = make(map[string]struct{}, len(host.Sensors.SMART))
			for _, disk := range host.Sensors.SMART {
				diskResourceID, diskName := hostSMARTDiskResourceID(host, disk)
				seenSMARTDisks[diskResourceID] = struct{}{}
				m.syncHostSMARTDiskRiskAlerts(host, disk, diskResourceID, diskName, nodeName, instanceName, baseMetadata, thresholds)
			}
		}
		m.cleanupHostSMARTDiskAlerts(host.ID, seenSMARTDisks, thresholds)
	}

	for _, disk := range host.Disks {
		diskResourceID, diskName := hostDiskResourceID(host, disk)
		seenDisks[diskResourceID] = struct{}{}

		// Check for disk-specific override
		m.mu.RLock()
		diskOverride, hasDiskOverride := m.config.Overrides[diskResourceID]
		m.mu.RUnlock()

		// Determine the effective disk threshold
		var effectiveDiskThreshold *HysteresisThreshold
		if hasDiskOverride {
			// If disk is disabled via override, skip alerting
			if diskOverride.Disabled {
				m.releaseHostUsageMetric(diskResourceID, diskName, nodeName, instanceName, unifiedresources.ResourceType("agent-disk"), "agent-disk", "disk")
				continue
			}
			// Use disk-specific threshold if set
			if diskOverride.Disk != nil {
				effectiveDiskThreshold = ensureHysteresisThreshold(diskOverride.Disk)
			}
		}
		// Per-type override: consult DiskFillByType if hardware type is inferable
		// from the device path and no disk-specific override applied above.
		if effectiveDiskThreshold == nil && !diskOverridden && thresholds.Disk != nil && thresholds.Disk.Trigger > 0 {
			if hwType := inferDiskHardwareType(disk.Device); hwType != "" {
				m.mu.RLock()
				if th, ok := m.config.DiskFillByType[hwType]; ok {
					t := th
					effectiveDiskThreshold = &t
				}
				m.mu.RUnlock()
			}
		}
		// Fall back to host-level threshold
		if effectiveDiskThreshold == nil {
			effectiveDiskThreshold = thresholds.Disk
		}

		// Skip if no threshold configured (nil)
		// We DO NOT skip if Trigger <= 0 because we need to call checkMetric to clear any existing alerts.
		if effectiveDiskThreshold == nil {
			m.releaseHostUsageMetric(diskResourceID, diskName, nodeName, instanceName, unifiedresources.ResourceType("agent-disk"), "agent-disk", "disk")
			continue
		}

		diskMetadata := hostChildAlertMetadata(baseMetadata, hostDiskAlertResourceType)
		diskMetadata["metric"] = "disk"
		diskMetadata["mountpoint"] = disk.Mountpoint
		diskMetadata["device"] = disk.Device
		diskMetadata["diskType"] = disk.Type
		diskMetadata["diskUsagePercent"] = disk.Usage
		if disk.Total > 0 {
			diskMetadata["diskTotalBytes"] = disk.Total
			diskMetadata["diskUsedBytes"] = disk.Used
			diskMetadata["diskFreeBytes"] = disk.Free
		}
		spec, err := buildCanonicalMetricSpec(diskResourceID, diskName, unifiedresources.ResourceType("agent-disk"), "disk", effectiveDiskThreshold)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", diskResourceID).
				Str("host", resourceName).
				Str("mountpoint", disk.Mountpoint).
				Msg("Skipping invalid canonical host disk metric spec")
			continue
		}

		evaluated := m.checkMetricWithCanonicalSpec(spec, diskName, nodeName, instanceName, "agent-disk", disk.Usage, effectiveDiskThreshold, &metricOptions{Metadata: diskMetadata})
		if diskResourceID == summaryDiskResourceID {
			summaryDiskLive = liveHysteresisThreshold(effectiveDiskThreshold)
			evaluatesSummaryDisk = evaluated
		}
	}

	// Clear all disk alerts if host-level disk alerting is completely disabled and no disk-specific overrides
	if thresholds.Disk == nil || thresholds.Disk.Trigger <= 0 {
		// Only clear alerts for disks that don't have their own overrides
		m.mu.RLock()
		var disksToClear []string
		for _, disk := range host.Disks {
			diskResourceID, _ := hostDiskResourceID(host, disk)
			_, hasDiskOverride := m.config.Overrides[diskResourceID]
			if !hasDiskOverride {
				disksToClear = append(disksToClear, canonicalMetricStateID(diskResourceID, "disk"))
			}
		}
		m.mu.RUnlock()

		for _, alertID := range disksToClear {
			m.clearAlert(alertID)
		}
	}

	m.cleanupHostDiskAlerts(host, seenDisks)

	// The linked node releases the usage metrics this agent covers and keeps
	// the rest, so deduplication never leaves the machine unmonitored. A metric
	// is covered while the current config lets the agent evaluate it and the
	// agent either evaluated it this report or still holds an alert or run for
	// it: a missing reading or a warming evaluation window then keeps the
	// agent's alert as the single source instead of opening a node duplicate,
	// while an agent with no usable evidence and nothing open hands the metric
	// to the node.
	m.applyHostAgentNodeLink(hostAgentNodeLink{
		agentID:               host.ID,
		agentName:             resourceName,
		nodeID:                host.LinkedNodeID,
		cpu:                   liveHysteresisThreshold(thresholds.CPU) && (cpuEvaluated || m.hostAgentMetricOpen(resourceID, "cpu")),
		memory:                liveHysteresisThreshold(thresholds.Memory) && (memoryEvaluated || m.hostAgentMetricOpen(resourceID, "memory")),
		disk:                  summaryDiskLive && (evaluatesSummaryDisk || m.hostAgentMetricOpen(summaryDiskResourceID, "disk")),
		summaryDiskResourceID: summaryDiskResourceID,
	}, reportSeq)

	if host.Unraid != nil {
		m.syncHostUnraidStorageAlert(host, nodeName, instanceName, resourceName, baseMetadata)
	} else {
		m.clearHostUnraidAlerts(host.ID)
	}

	// Clear vendor-managed system-array alerts even when host state has already
	// been normalized to exclude them.
	m.clearVendorManagedHostRAIDAlerts(host)

	// Check RAID arrays for degraded or failed state
	if len(host.RAID) > 0 {
		for _, array := range host.RAID {
			// Skip vendor-managed system arrays that are not customer-facing storage pools.
			if storagehealth.IsVendorManagedSystemRAIDArray(host, array) {
				// Still clear any existing alerts for these devices
				raidSpecResourceID := fmt.Sprintf("%s/raid:%s", hostResourceID(host.ID), sanitizeRAIDDevice(array.Device))
				m.clearAlert(buildCanonicalStateID(raidSpecResourceID, raidSpecResourceID+"-health"))
				continue
			}

			raidResourceID := fmt.Sprintf("host-%s-raid-%s", host.ID, sanitizeRAIDDevice(array.Device))
			raidName := fmt.Sprintf("%s - %s (%s)", resourceName, array.Device, array.Level)
			raidSpecResourceID := fmt.Sprintf("%s/raid:%s", hostResourceID(host.ID), sanitizeRAIDDevice(array.Device))

			raidMetadata := hostChildAlertMetadata(baseMetadata, hostStorageAlertResourceType)
			raidMetadata["metric"] = "raid"
			raidMetadata["raidDevice"] = array.Device
			raidMetadata["raidLevel"] = array.Level
			raidMetadata["raidState"] = array.State
			if array.RequiredDevices > 0 {
				raidMetadata["raidRequiredDevices"] = array.RequiredDevices
			}
			raidMetadata["raidTotalDevices"] = array.TotalDevices
			raidMetadata["raidActiveDevices"] = array.ActiveDevices
			raidMetadata["raidFailedDevices"] = array.FailedDevices
			raidMetadata["raidSpareDevices"] = array.SpareDevices
			if array.UUID != "" {
				raidMetadata["raidUUID"] = array.UUID
			}
			if array.RebuildPercent > 0 {
				raidMetadata["raidRebuildPercent"] = array.RebuildPercent
			}
			if array.Operation != "" {
				raidMetadata["raidOperation"] = array.Operation
			}

			alertID := fmt.Sprintf("host-%s-raid-%s", host.ID, sanitizeRAIDDevice(array.Device))
			assessment := storagehealth.AssessHostRAIDArray(array)
			result, _ := m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
				SpecID:         raidSpecResourceID + "-health",
				Signal:         "host-raid",
				Codes:          raidAssessmentCodes,
				Reasons:        assessment.Reasons,
				AlertID:        alertID,
				AlertType:      "raid",
				SpecResourceID: raidSpecResourceID,
				ResourceID:     raidResourceID,
				ResourceName:   raidName,
				ResourceType:   unifiedresources.ResourceTypeAgent,
				Node:           nodeName,
				Instance:       instanceName,
				Metadata:       raidMetadata,
				MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
					message := strings.Join(storageHealthReasonSummaries(assessment.Reasons), "; ")
					switch result.State.Severity {
					case alertspecs.AlertSeverityCritical:
						return message, float64(array.FailedDevices), 0
					case alertspecs.AlertSeverityWarning:
						return message, array.RebuildPercent, 100
					default:
						return message, 0, 0
					}
				},
			})

			if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
				switch result.State.Severity {
				case alertspecs.AlertSeverityCritical:
					log.Error().
						Str("host", resourceName).
						Str("hostID", host.ID).
						Str("raidDevice", array.Device).
						Str("raidLevel", array.Level).
						Int("failedDevices", array.FailedDevices).
						Msg("CRITICAL: RAID array degraded")
				case alertspecs.AlertSeverityWarning:
					log.Warn().
						Str("host", resourceName).
						Str("hostID", host.ID).
						Str("raidDevice", array.Device).
						Str("raidLevel", array.Level).
						Float64("rebuildPercent", array.RebuildPercent).
						Msg("WARNING: RAID array rebuilding")
				}
			}
		}
	}
}

// HandleHostOnline clears offline tracking and alerts for a host agent.
func (m *Manager) HandleHostOnline(host models.Host) {
	if host.ID == "" {
		return
	}

	resourceKey := hostResourceID(host.ID)
	alertID := canonicalConnectivityStateID(resourceKey)

	m.mu.Lock()
	exists := m.hasActiveAlertNoLock(alertID)
	// The host was observed online: a healthy observation ends any
	// in-flight offline confirmation run in the core.
	m.core.ApplyDiscrete(reducer.DiscreteSignal{ResourceID: resourceKey, Key: canonicalConnectivitySpecID(resourceKey), Matched: false, ObservedAt: m.policyNow()}, reducer.DiscreteRule{})
	m.mu.Unlock()

	if exists {
		m.clearAlert(alertID)
	}
}

// HandleHostRemoved clears alerts and tracking when a host agent is deleted.
func (m *Manager) HandleHostRemoved(host models.Host) {
	if host.ID == "" {
		return
	}

	// The removed agent no longer owns its linked node's usage alerts.
	m.unregisterHostAgentNodeLink(host.ID)

	m.HandleHostOnline(host)
	m.clearHostMetricAlerts(host.ID)
	m.clearHostDiskAlerts(host.ID)
	m.clearHostRAIDAlerts(host.ID)
	m.clearHostUnraidAlerts(host.ID)
	m.clearHostCustomSensorAlerts(host.ID)
	// No later report will close the host's pending disk temperature runs or
	// reset its disk absence counts.
	m.clearHostDiskTemperatureAlerts(host.ID)
}

// HandleHostTelemetryExpired re-evaluates transient storage-operation evidence
// after the host reporting lease ends. Connectivity remains a separate signal
// evaluated by HandleHostOffline; non-transient metric and disk alerts retain
// their existing confirmation policy until that connectivity transition.
func (m *Manager) HandleHostTelemetryExpired(host models.Host) {
	if host.ID == "" {
		return
	}

	// Expiry interrupts timing, not the separate connectivity confirmation.
	m.interruptHostDiskTemperatureRuns(host.ID)
	m.resetHostSMARTDiskAbsences(host.ID)

	if host.Unraid != nil {
		unraid := *host.Unraid
		unraid.SyncAction = ""
		unraid.SyncProgress = 0
		host.Unraid = &unraid
	}
	host.RAID = append([]models.HostRAIDArray(nil), host.RAID...)
	for i := range host.RAID {
		host.RAID[i].Operation = ""
		host.RAID[i].RebuildPercent = 0
		host.RAID[i].RebuildSpeed = ""
	}

	m.mu.RLock()
	alertsEnabled := m.config.Enabled
	disableAllAgents, _ := m.alertPolicyTypeSwitchesNoLock("agent")
	thresholds := m.resolveHostThresholdsNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	m.mu.RUnlock()
	if !alertsEnabled || disableAllAgents || thresholds.Disabled {
		m.clearHostRAIDAlerts(host.ID)
		m.clearHostUnraidAlerts(host.ID)
		m.clearHostCustomSensorAlerts(host.ID)
		return
	}
	m.clearHostCustomSensorAlerts(host.ID)

	if host.Unraid == nil {
		m.clearHostUnraidAlerts(host.ID)
	} else {
		baseMetadata := map[string]interface{}{
			"resourceType":       "agent",
			alertPlatformTypeKey: string(unifiedresources.SourceAgent),
			"hostId":             host.ID,
			"hostname":           host.Hostname,
			"displayName":        host.DisplayName,
			"platform":           host.Platform,
			"osName":             host.OSName,
			"osVersion":          host.OSVersion,
			"agentVersion":       host.AgentVersion,
			"architecture":       host.Architecture,
		}
		if linkedNodeID := strings.TrimSpace(host.LinkedNodeID); linkedNodeID != "" {
			baseMetadata["linkedNodeId"] = linkedNodeID
		}
		if linkedVMID := strings.TrimSpace(host.LinkedVMID); linkedVMID != "" {
			baseMetadata["linkedVmId"] = linkedVMID
		}
		if linkedContainerID := strings.TrimSpace(host.LinkedContainerID); linkedContainerID != "" {
			baseMetadata["linkedContainerId"] = linkedContainerID
		}
		if len(host.Tags) > 0 {
			baseMetadata["tags"] = append([]string(nil), host.Tags...)
		}
		m.syncHostUnraidStorageAlert(
			host,
			strings.TrimSpace(host.Hostname),
			hostInstanceName(host),
			hostDisplayName(host),
			baseMetadata,
		)
	}

	if len(host.RAID) == 0 {
		m.clearHostRAIDAlerts(host.ID)
		return
	}
	for _, array := range host.RAID {
		assessment := storagehealth.AssessHostRAIDArray(array)
		if assessment.Level != storagehealth.RiskHealthy {
			continue
		}
		resourceID := fmt.Sprintf("%s/raid:%s", hostResourceID(host.ID), sanitizeRAIDDevice(array.Device))
		m.clearAlert(buildCanonicalStateID(resourceID, resourceID+"-health"))
	}
}

// HandleHostOffline raises an alert when a host agent stops reporting.
func (m *Manager) HandleHostOffline(host models.Host) {
	m.HandleHostOfflineWithCorrelation(host, nil)
}

// HandleHostOfflineWithCorrelation raises a host connectivity alert with
// optional verified shared-system context. The correlation affects only
// presentation; the host alert retains its independent canonical lifecycle.
func (m *Manager) HandleHostOfflineWithCorrelation(host models.Host, correlation *AlertCorrelation) {
	if host.ID == "" {
		return
	}

	// The agent is no longer actively monitoring, so its linked Proxmox node
	// resumes evaluating its own usage alerts.
	m.unregisterHostAgentNodeLink(host.ID)
	m.HandleHostTelemetryExpired(host)

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	_, disableHostsOffline := m.alertPolicyTypeSwitchesNoLock("agent")
	thresholds := m.resolveHostThresholdsNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	m.mu.RUnlock()

	resourceKey := hostResourceID(host.ID)
	alertID := canonicalConnectivityStateID(resourceKey)
	resourceName := hostDisplayName(host)
	nodeName := strings.TrimSpace(host.Hostname)
	instanceName := hostInstanceName(host)

	if disableHostsOffline {
		m.mu.Lock()
		m.mu.Unlock()
		m.clearAlert(alertID)
		return
	}

	if thresholds.Disabled || thresholds.DisableConnectivity {
		m.clearAlert(alertID)
		m.mu.Lock()
		m.mu.Unlock()
		return
	}

	spec, err := buildCanonicalConnectivitySpec(resourceKey, resourceName, unifiedresources.ResourceTypeAgent, AlertLevelCritical, 3, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("host", resourceName).
			Str("hostID", host.ID).
			Msg("Skipping invalid canonical host connectivity spec")
		return
	}

	result, ok := m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			Connectivity: &alertspecs.ConnectivityEvidence{
				Signal:    "status",
				Connected: false,
			},
		},
		AlertID:      alertID,
		AlertType:    HostOfflineAlertType,
		ResourceID:   resourceKey,
		ResourceName: resourceName,
		Node:         nodeName,
		Instance:     instanceName,
		Message:      fmt.Sprintf("Host '%s' is offline", resourceName),
		Correlation:  correlation,
		Metadata: map[string]interface{}{
			"resourceType":       "agent",
			alertPlatformTypeKey: string(unifiedresources.SourceAgent),
			"hostId":             host.ID,
			"hostname":           host.Hostname,
			"displayName":        host.DisplayName,
			"platform":           host.Platform,
			"osName":             host.OSName,
			"osVersion":          host.OSVersion,
			"linkedNodeId":       strings.TrimSpace(host.LinkedNodeID),
			"linkedVmId":         strings.TrimSpace(host.LinkedVMID),
			"linkedContainerId":  strings.TrimSpace(host.LinkedContainerID),
		},
		AddToRecent:   true,
		AddToHistory:  true,
		RateLimit:     true,
		DispatchAsync: false,
	})
	if !ok {
		return
	}
	if result.State.State == alertspecs.AlertStatePending {
		log.Debug().
			Str("host", resourceName).
			Str("hostID", host.ID).
			Int("confirmations", result.State.ConsecutiveMatches).
			Int("required", 3).
			Msg("Host agent appears offline, awaiting confirmation")
		return
	}
	if result.Transition == nil || result.Transition.Kind != alertspecs.EvaluationTransitionActivated {
		return
	}

	// Host is confirmed offline. Clear all host-scoped metrics and storage-health alerts
	// so the connectivity alert becomes the only active signal for this agent.
	m.mu.Lock()
	for _, mt := range []string{"cpu", "memory"} {
		m.clearAlertNoLock(canonicalMetricStateID(resourceKey, mt))
	}

	diskResourcePrefixes := []string{
		fmt.Sprintf("%s/disk:", resourceKey),
		hostDiskTemperatureResourcePrefix(host.ID),
	}
	raidAlertPrefix := fmt.Sprintf("host-%s-raid-", host.ID)
	var alertsToClear []string
	for activeAlertID, a := range m.activeAlerts {
		if a == nil {
			continue
		}
		matchesDiskPrefix := false
		for _, diskResourcePrefix := range diskResourcePrefixes {
			if strings.HasPrefix(a.ResourceID, diskResourcePrefix) {
				matchesDiskPrefix = true
				break
			}
		}
		if matchesDiskPrefix || strings.HasPrefix(activeAlertID, raidAlertPrefix) {
			alertsToClear = append(alertsToClear, activeAlertID)
		}
	}
	for _, staleAlertID := range alertsToClear {
		m.clearAlertNoLock(staleAlertID)
	}
	m.mu.Unlock()
	m.clearHostRAIDAlerts(host.ID)
	m.clearHostUnraidAlerts(host.ID)

	log.Error().
		Str("host", resourceName).
		Str("hostID", host.ID).
		Str("hostname", host.Hostname).
		Msg("CRITICAL: Host agent is offline")
}

func (m *Manager) clearHostMetricAlerts(hostID string, metrics ...string) {
	if hostID == "" {
		return
	}
	resourceIDs := []string{
		hostResourceID(hostID),
	}
	if len(metrics) == 0 {
		metrics = []string{"cpu", "memory"}
	}
	for _, resourceID := range resourceIDs {
		for _, metric := range metrics {
			m.releaseHostUsageMetric(resourceID, "", "", "", unifiedresources.ResourceTypeAgent, "agent", metric)
		}
	}
}

func (m *Manager) clearHostDiskAlerts(hostID string) {
	if hostID == "" {
		return
	}

	prefixes := []string{
		fmt.Sprintf("%s/disk:", hostResourceID(hostID)),
		hostDiskTemperatureResourcePrefix(hostID),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetHostSMARTDiskAbsencesNoLock(hostID)

	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert == nil {
			continue
		}
		matches := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(alert.ResourceID, prefix) {
				matches = true
				break
			}
		}
		if !matches {
			continue
		}
		m.clearAlertNoLock(alertID)
	}
}

// hostDiskTemperatureResourcePrefix is the resource ID prefix of every SMART
// disk temperature alert CheckHost raises for a host.
func hostDiskTemperatureResourcePrefix(hostID string) string {
	return hostResourceID(hostID) + "/disk_temp:"
}

// hostDiskTemperatureResourceID is the resource ID of the SMART disk
// temperature alert for one device on a host.
func hostDiskTemperatureResourceID(hostID, device string) string {
	return hostDiskTemperatureResourcePrefix(hostID) + sanitizeHostComponent(device)
}

// hostDiskTemperatureThresholdNoLock returns the threshold for one SMART disk
// given the host's resolved disk temperature threshold. Callers must hold m.mu.
func (m *Manager) hostDiskTemperatureThresholdNoLock(hostThreshold *HysteresisThreshold, overridden bool, diskType string) *HysteresisThreshold {
	return diskTemperatureThresholdForType(m.config.DiskTempByType, hostThreshold, overridden, diskType)
}

// diskTemperatureThresholdForType is the disk temperature policy. An enabled
// host threshold gives way to the disk type's DiskTempByType entry unless an
// explicit host or linked-resource override set it; a disabled one switches
// disk temperature alerting off for every type.
func diskTemperatureThresholdForType(byType map[string]HysteresisThreshold, hostThreshold *HysteresisThreshold, overridden bool, diskType string) *HysteresisThreshold {
	if hostThreshold == nil || hostThreshold.Trigger <= 0 || overridden {
		return hostThreshold
	}
	if diskType = strings.ToLower(strings.TrimSpace(diskType)); diskType != "" {
		if th, ok := byType[diskType]; ok {
			return &th
		}
	}
	return hostThreshold
}

// DiskTemperatureThreshold returns the threshold that judges a physical disk's
// temperature when no host override applies: the disk type's DiskTempByType
// entry, else the agent default. This is the canonical disk heat policy. Disk
// temperature alerts, Patrol and the Physical Disks Health verdict all judge
// heat against it; disk risk does not. The disk is hot at the trigger, and the
// clear value is where it stops being hot. A nil threshold or a non-positive
// trigger means disk temperature alerting is off.
func (m *Manager) DiskTemperatureThreshold(diskType string) *HysteresisThreshold {
	if m == nil {
		return DefaultDiskTemperatureThreshold(diskType)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneThreshold(m.hostDiskTemperatureThresholdNoLock(m.config.AgentDefaults.DiskTemperature, false, diskType))
}

// DefaultDiskTemperatureThreshold is DiskTemperatureThreshold under the
// factory alert configuration, for callers with no alert manager.
func DefaultDiskTemperatureThreshold(diskType string) *HysteresisThreshold {
	config := defaultAlertConfig()
	return cloneThreshold(diskTemperatureThresholdForType(config.DiskTempByType, config.AgentDefaults.DiskTemperature, false, diskType))
}

// lowestHostDiskTemperatureThresholdNoLock returns the lowest enabled
// threshold any SMART disk of the host can be evaluated against. An alert
// whose disk type is unknown is judged against it, so a config save never
// resolves an alert its disk type would still fire. Callers must hold m.mu.
func (m *Manager) lowestHostDiskTemperatureThresholdNoLock(hostThreshold *HysteresisThreshold) *HysteresisThreshold {
	if hostThreshold == nil || hostThreshold.Trigger <= 0 {
		return hostThreshold
	}
	lowest := hostThreshold
	for _, th := range m.config.DiskTempByType {
		if th.Trigger > 0 && th.Trigger < lowest.Trigger {
			t := th
			lowest = &t
		}
	}
	return lowest
}

// clearHostDiskTemperatureAlerts resolves every SMART disk temperature alert
// for a host.
func (m *Manager) clearHostDiskTemperatureAlerts(hostID string) {
	m.cleanupHostDiskTemperatureAlerts(hostID, nil)
}

// hostDiskTemperatureAbsenceConfirmations is how many consecutive non-empty
// SMART reports must omit a disk before its temperature alert clears. The
// Windows, FreeBSD and controller-multiplexed Linux collectors drop a disk
// whose probe fails, so one omission is not evidence the disk left.
const hostDiskTemperatureAbsenceConfirmations = 3

// cleanupHostDiskTemperatureAlerts resolves a host's SMART disk temperature
// alerts, and drops their pending threshold runs, once their resource ID has
// been missing from seen for hostDiskTemperatureAbsenceConfirmations calls in
// a row. A nil seen resolves all of them at once.
func (m *Manager) cleanupHostDiskTemperatureAlerts(hostID string, seen map[string]struct{}) {
	if hostID == "" {
		return
	}

	resourcePrefix := hostDiskTemperatureResourcePrefix(hostID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Absent resources map to the storage keys of their active alerts; a
	// resource with only a pending run maps to none.
	absent := make(map[string][]string)
	intentChanged := false
	defer func() {
		if intentChanged {
			m.saveActiveAlertsAsync("disk temperature inventory gap")
		}
	}()
	for storageKey, alert := range m.activeAlerts {
		// Only CheckHost raises alerts under this prefix. Matching on it
		// alone also catches alerts raised before the type became
		// diskTemperature ("disk_temperature", January 2026).
		if alert == nil || !strings.HasPrefix(alert.ResourceID, resourcePrefix) {
			continue
		}
		if _, exists := seen[alert.ResourceID]; exists {
			continue
		}
		absent[alert.ResourceID] = append(absent[alert.ResourceID], storageKey)
	}
	for resourceID := range m.hostDiskTemperatureTrackedResourcesNoLock() {
		if !strings.HasPrefix(resourceID, resourcePrefix) {
			continue
		}
		if _, exists := seen[resourceID]; exists {
			continue
		}
		if _, exists := absent[resourceID]; !exists {
			absent[resourceID] = nil
		}
	}

	if m.hostDiskTempAbsences == nil {
		m.hostDiskTempAbsences = make(map[string]int)
	}
	// A disk seen again, or whose state is gone, restarts its count.
	for resourceID := range m.hostDiskTempAbsences {
		if !strings.HasPrefix(resourceID, resourcePrefix) {
			continue
		}
		if _, exists := absent[resourceID]; !exists {
			delete(m.hostDiskTempAbsences, resourceID)
		}
	}
	for resourceID, storageKeys := range absent {
		if seen != nil {
			// One omission breaks sustained/recovery evidence; only confirmed
			// departure resolves a firing alert.
			intentChanged = m.interruptHostDiskTemperatureRunNoLock(resourceID) || intentChanged
			m.hostDiskTempAbsences[resourceID]++
			if m.hostDiskTempAbsences[resourceID] < hostDiskTemperatureAbsenceConfirmations {
				continue
			}
		}
		delete(m.hostDiskTempAbsences, resourceID)
		// A departed disk never sends the reading that would close its run.
		intentChanged = m.interruptHostDiskTemperatureRunNoLock(resourceID) || intentChanged
		for _, storageKey := range storageKeys {
			m.clearAlertNoLock(storageKey)
		}
	}
}

var customSensorAssessmentCodes = []string{
	"custom_sensor_warning",
	"custom_sensor_critical",
	"custom_sensor_error",
}

func (m *Manager) syncHostCustomSensorAlerts(host models.Host, nodeName, instanceName string, baseMetadata map[string]interface{}) {
	seen := make(map[string]struct{}, len(host.Sensors.Custom))
	for _, metric := range host.Sensors.Custom {
		metricID := sanitizeHostComponent(strings.TrimSpace(metric.ID))
		if metricID == "" {
			continue
		}
		resourceID := fmt.Sprintf("%s/custom:%s", hostResourceID(host.ID), metricID)
		seen[resourceID] = struct{}{}

		reasons := make([]storagehealth.Reason, 0, 1)
		valueText := "unavailable"
		if metric.Value != nil {
			valueText = fmt.Sprintf("%g", *metric.Value)
			if unit := strings.TrimSpace(metric.Unit); unit != "" {
				valueText += " " + unit
			}
		}
		switch strings.ToLower(strings.TrimSpace(metric.Status)) {
		case "critical":
			reasons = append(reasons, storagehealth.Reason{
				Code:     "custom_sensor_critical",
				Severity: storagehealth.RiskCritical,
				Summary:  fmt.Sprintf("%s is critical at %s", metric.Name, valueText),
			})
		case "warning":
			reasons = append(reasons, storagehealth.Reason{
				Code:     "custom_sensor_warning",
				Severity: storagehealth.RiskWarning,
				Summary:  fmt.Sprintf("%s is warning at %s", metric.Name, valueText),
			})
		case "error":
			if metric.AlertOnError {
				message := strings.TrimSpace(metric.Error)
				if message == "" {
					message = "collector execution failed"
				}
				reasons = append(reasons, storagehealth.Reason{
					Code:     "custom_sensor_error",
					Severity: storagehealth.RiskWarning,
					Summary:  fmt.Sprintf("%s custom sensor error: %s", metric.Name, message),
				})
			}
		}

		metadata := hostChildAlertMetadata(baseMetadata, hostSensorAlertResourceType)
		metadata["metric"] = "customSensor"
		metadata["customSensorId"] = metric.ID
		metadata["customSensorName"] = metric.Name
		metadata["customSensorGroup"] = metric.Group
		metadata["customSensorSubgroup"] = metric.Subgroup
		metadata["customSensorKind"] = metric.Kind
		metadata["customSensorUnit"] = metric.Unit
		metadata["customSensorStatus"] = metric.Status
		metadata["customSensorStale"] = metric.Stale
		if metric.Value != nil {
			metadata["customSensorValue"] = *metric.Value
		}
		if metric.Error != "" {
			metadata["customSensorError"] = metric.Error
		}
		if metric.EventAt != nil {
			metadata["customSensorEventAt"] = metric.EventAt.UTC().Format(time.RFC3339)
		}

		resourceName := fmt.Sprintf("%s - %s", hostDisplayName(host), metric.Name)
		_, _ = m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
			SpecID:         resourceID + "-health",
			Signal:         "custom-sensor",
			Codes:          customSensorAssessmentCodes,
			Reasons:        reasons,
			AlertID:        fmt.Sprintf("host-%s-custom-%s", host.ID, metricID),
			AlertType:      "custom-sensor",
			SpecResourceID: resourceID,
			ResourceID:     resourceID,
			ResourceName:   resourceName,
			ResourceType:   unifiedresources.ResourceTypeAgent,
			Node:           nodeName,
			Instance:       instanceName,
			Metadata:       metadata,
			MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
				message := strings.Join(storageHealthReasonSummaries(reasons), "; ")
				value := 0.0
				if metric.Value != nil {
					value = *metric.Value
				}
				return message, value, 0
			},
		})
	}
	m.cleanupHostCustomSensorAlerts(host.ID, seen)
}

func (m *Manager) clearHostCustomSensorAlerts(hostID string) {
	m.cleanupHostCustomSensorAlerts(hostID, nil)
}

func (m *Manager) cleanupHostCustomSensorAlerts(hostID string, seen map[string]struct{}) {
	if strings.TrimSpace(hostID) == "" {
		return
	}
	prefix := hostResourceID(hostID) + "/custom:"
	m.mu.Lock()
	defer m.mu.Unlock()
	for storageKey, alert := range m.activeAlerts {
		if alert == nil || !strings.HasPrefix(alert.ResourceID, prefix) {
			continue
		}
		if seen != nil {
			if _, exists := seen[alert.ResourceID]; exists {
				continue
			}
		}
		m.clearAlertNoLock(storageKey)
	}
}

func (m *Manager) clearGuestMetricAlerts(guestID string, metrics ...string) int {
	if guestID == "" {
		return 0
	}

	allowedMetrics := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		metric = strings.TrimSpace(metric)
		if metric == "" {
			continue
		}
		allowedMetrics[metric] = struct{}{}
	}

	perDiskPrefix := fmt.Sprintf("%s-disk-", guestID)

	m.mu.Lock()
	defer m.mu.Unlock()

	cleared := 0
	for storageKey, alert := range m.activeAlerts {
		if alert == nil || !isMetricThresholdAlertType(alert.Type) {
			continue
		}
		if alert.ResourceID != guestID && !strings.HasPrefix(alert.ResourceID, perDiskPrefix) &&
			!guestAlertBelongsToGuest(alert.ResourceID, guestID) {
			continue
		}
		if len(allowedMetrics) > 0 {
			if _, ok := allowedMetrics[alert.Type]; !ok {
				continue
			}
		}
		m.clearAlertNoLock(storageKey)
		cleared++
	}

	return cleared
}

func (m *Manager) cleanupGuestDiskAlerts(guestID string, seen map[string]struct{}) int {
	if guestID == "" {
		return 0
	}

	prefix := fmt.Sprintf("%s-disk-", guestID)

	m.mu.Lock()
	defer m.mu.Unlock()

	cleared := 0
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		if !strings.HasPrefix(alert.ResourceID, prefix) && !guestDiskAlertBelongsToGuest(alert.ResourceID, guestID) {
			continue
		}
		if seen != nil {
			if _, exists := seen[alert.ResourceID]; exists {
				continue
			}
		}
		m.clearAlertNoLock(storageKey)
		cleared++
	}

	return cleared
}

func (m *Manager) clearVendorManagedHostRAIDAlerts(host models.Host) {
	if host.ID == "" {
		return
	}

	for _, device := range storagehealth.VendorManagedSystemRAIDDevices(host) {
		raidSpecResourceID := fmt.Sprintf("%s/raid:%s", hostResourceID(host.ID), sanitizeRAIDDevice(device))
		m.clearAlert(buildCanonicalStateID(raidSpecResourceID, raidSpecResourceID+"-health"))
	}
}

func (m *Manager) cleanupHostDiskAlerts(host models.Host, seen map[string]struct{}) {
	if host.ID == "" {
		return
	}

	prefixes := []string{
		fmt.Sprintf("%s/disk:", hostResourceID(host.ID)),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert == nil {
			continue
		}
		matches := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(alert.ResourceID, prefix) {
				matches = true
				break
			}
		}
		if !matches {
			continue
		}
		// SMART health and wear follow their own inventory, not filesystems.
		if isHostSMARTRiskAlertType(alert.Type) {
			continue
		}
		if _, exists := seen[alert.ResourceID]; exists {
			continue
		}
		m.clearAlertNoLock(alertID)
	}
}

func (m *Manager) syncHostSMARTDiskRiskAlerts(host models.Host, disk models.HostDiskSMART, resourceID, resourceName, nodeName, instanceName string, baseMetadata map[string]interface{}, thresholds ThresholdConfig) {
	smartThresholds, crcMinimumDelta := hostSMARTRiskThresholds(thresholds)
	assessment := storagehealth.AssessHostSMARTDiskWithThresholds(disk, smartThresholds)
	assessment.Reasons = append(assessment.Reasons, m.hostSMARTCounterGrowthReasons(resourceID, disk, crcMinimumDelta)...)
	healthReasons, wearReasons := splitSMARTAlertReasons(assessment.Reasons)

	if m.hostSMARTDiskAlertEvidenceKnown(resourceID, "disk-health", disk, healthReasons, smartThresholds, crcMinimumDelta) {
		m.syncHostSMARTDiskAlert(host, disk, resourceID, resourceName, nodeName, instanceName, baseMetadata, "disk-health", healthReasons)
	}
	if m.hostSMARTDiskAlertEvidenceKnown(resourceID, "disk-wearout", disk, wearReasons, smartThresholds, crcMinimumDelta) {
		m.syncHostSMARTDiskAlert(host, disk, resourceID, resourceName, nodeName, instanceName, baseMetadata, "disk-wearout", wearReasons)
	}
}

// hostSMARTDiskAlertEvidenceKnown prevents an unavailable SMART field from
// being interpreted as a healthy zero. A standby report deliberately carries
// stable disk identity without waking the device to collect health evidence.
// Existing reasons therefore remain active until every still-enabled signal
// that raised the alert is observed again. Explicitly disabling the relevant
// rule remains authoritative and can clear the alert without a disk reading.
func (m *Manager) hostSMARTDiskAlertEvidenceKnown(resourceID, alertType string, disk models.HostDiskSMART, currentReasons []storagehealth.Reason, thresholds storagehealth.SMARTThresholds, crcMinimumDelta int64) bool {
	stateID := buildCanonicalStateID(resourceID, resourceID+"-"+alertType)

	m.mu.RLock()
	activeAlert, active := m.getActiveAlertNoLock(stateID)
	var activeCodes []string
	if active && activeAlert != nil {
		activeCodes = hostSMARTRiskCodes(activeAlert.Metadata["riskCodes"])
	}
	m.mu.RUnlock()

	if active && len(activeCodes) == 0 {
		// A restored pre-metadata alert may be reaffirmed by current bad
		// evidence, but absence of reasons is not sufficient recovery proof.
		return len(currentReasons) > 0
	}

	remainingEnabledReasons := 0
	for _, code := range activeCodes {
		if !hostSMARTRiskRuleEnabled(code, thresholds, crcMinimumDelta) {
			continue
		}
		remainingEnabledReasons++
		if disk.Standby || !hostSMARTRiskReasonObserved(code, disk) {
			return false
		}
	}
	if active && remainingEnabledReasons == 0 {
		return true
	}
	if disk.Standby {
		return false
	}

	return hostSMARTAlertFamilyObserved(alertType, disk)
}

func hostSMARTRiskCodes(value interface{}) []string {
	switch codes := value.(type) {
	case []string:
		return append([]string(nil), codes...)
	case []interface{}:
		result := make([]string, 0, len(codes))
		for _, code := range codes {
			if normalized := strings.TrimSpace(fmt.Sprint(code)); normalized != "" {
				result = append(result, normalized)
			}
		}
		return result
	default:
		return nil
	}
}

func hostSMARTRiskRuleEnabled(code string, thresholds storagehealth.SMARTThresholds, crcMinimumDelta int64) bool {
	switch code {
	case "health_status":
		return thresholds.HealthFailure
	case "reallocated_sectors":
		return thresholds.ReallocatedSectors > 0
	case "pending_sectors":
		return thresholds.PendingSectors > 0
	case "offline_uncorrectable":
		return thresholds.OfflineUncorrectable > 0
	case "media_errors":
		return thresholds.MediaErrors > 0
	case "crc_errors_increased":
		return crcMinimumDelta > 0
	case "wearout_low", "nvme_percentage_used_high":
		return thresholds.LifeWarning > 0 || thresholds.LifeCritical > 0
	case "nvme_available_spare_low":
		return thresholds.AvailableSpareWarn > 0 || thresholds.AvailableSpareCrit > 0
	default:
		return true
	}
}

func hostSMARTRiskReasonObserved(code string, disk models.HostDiskSMART) bool {
	attrs := disk.Attributes
	switch code {
	case "health_status":
		health := strings.ToUpper(strings.TrimSpace(disk.Health))
		return health != "" && health != "UNKNOWN"
	case "reallocated_sectors":
		return attrs != nil && attrs.ReallocatedSectors != nil
	case "pending_sectors":
		return attrs != nil && attrs.PendingSectors != nil
	case "offline_uncorrectable":
		return attrs != nil && attrs.OfflineUncorrectable != nil
	case "media_errors":
		return attrs != nil && attrs.MediaErrors != nil
	case "crc_errors_increased":
		return attrs != nil && attrs.UDMACRCErrors != nil
	case "wearout_low", "nvme_percentage_used_high":
		return attrs != nil && attrs.PercentageUsed != nil
	case "nvme_available_spare_low":
		return attrs != nil && attrs.AvailableSpare != nil
	default:
		return false
	}
}

func hostSMARTAlertFamilyObserved(alertType string, disk models.HostDiskSMART) bool {
	if alertType == "disk-wearout" {
		return hostSMARTRiskReasonObserved("nvme_percentage_used_high", disk) ||
			hostSMARTRiskReasonObserved("nvme_available_spare_low", disk)
	}

	return hostSMARTRiskReasonObserved("health_status", disk) ||
		hostSMARTRiskReasonObserved("reallocated_sectors", disk) ||
		hostSMARTRiskReasonObserved("pending_sectors", disk) ||
		hostSMARTRiskReasonObserved("offline_uncorrectable", disk) ||
		hostSMARTRiskReasonObserved("media_errors", disk) ||
		hostSMARTRiskReasonObserved("crc_errors_increased", disk)
}

// hostSMARTCounterGrowthReasons turns a newly increased SMART counter into an
// alertable health reason without warning on an old, non-zero counter at first
// observation. A stable value clears the transient growth reason on the next
// report, while the alert and notification histories retain the event.
func (m *Manager) hostSMARTCounterGrowthReasons(resourceID string, disk models.HostDiskSMART, minimumDelta int64) []storagehealth.Reason {
	if disk.Attributes == nil || disk.Attributes.UDMACRCErrors == nil {
		return nil
	}

	current := *disk.Attributes.UDMACRCErrors
	if current < 0 {
		return nil
	}

	observedAt := m.now()
	m.mu.Lock()
	previous, observed := m.smartCounterSnapshots[resourceID]
	m.smartCounterSnapshots[resourceID] = smartCounterSnapshot{
		UDMACRCErrors: current,
		LastObserved:  observedAt,
	}
	m.mu.Unlock()

	if !observed || minimumDelta <= 0 || current-previous.UDMACRCErrors < minimumDelta {
		return nil
	}

	return []storagehealth.Reason{{
		Code:     "crc_errors_increased",
		Severity: storagehealth.RiskWarning,
		Summary:  fmt.Sprintf("UDMA CRC error count increased from %d to %d", previous.UDMACRCErrors, current),
	}}
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func splitSMARTAlertReasons(reasons []storagehealth.Reason) ([]storagehealth.Reason, []storagehealth.Reason) {
	healthReasons := make([]storagehealth.Reason, 0, len(reasons))
	wearReasons := make([]storagehealth.Reason, 0, len(reasons))

	for _, reason := range reasons {
		if reason.Severity != storagehealth.RiskWarning && reason.Severity != storagehealth.RiskCritical {
			continue
		}
		switch reason.Code {
		case "wearout_low", "nvme_available_spare_low", "nvme_percentage_used_high":
			wearReasons = append(wearReasons, reason)
		default:
			healthReasons = append(healthReasons, reason)
		}
	}

	return healthReasons, wearReasons
}

var (
	smartHealthAssessmentCodes = []string{
		"health_status",
		"pending_sectors",
		"offline_uncorrectable",
		"media_errors",
		"reallocated_sectors",
		"crc_errors_increased",
	}
	smartWearoutAssessmentCodes = []string{
		"wearout_low",
		"nvme_available_spare_low",
		"nvme_percentage_used_high",
	}
	raidAssessmentCodes = []string{
		"raid_degraded",
		"raid_unavailable",
		"raid_rebuilding",
	}
)

func (m *Manager) syncHostSMARTDiskAlert(host models.Host, disk models.HostDiskSMART, resourceID, resourceName, nodeName, instanceName string, baseMetadata map[string]interface{}, alertType string, reasons []storagehealth.Reason) {
	alertID := fmt.Sprintf("host-%s-%s-%s", host.ID, alertType, strings.TrimPrefix(resourceID, hostResourceID(host.ID)+"/disk:"))
	reasonCodes := storageHealthReasonCodes(reasons)
	reasonSummaries := storageHealthReasonSummaries(reasons)

	metadata := hostChildAlertMetadata(baseMetadata, hostDiskAlertResourceType)
	metadata["metric"] = alertType
	metadata["device"] = disk.Device
	metadata["model"] = disk.Model
	metadata["serial"] = disk.Serial
	metadata["wwn"] = disk.WWN
	metadata["diskHealth"] = disk.Health
	metadata["riskCodes"] = reasonCodes
	metadata["riskSummaries"] = reasonSummaries
	if disk.Temperature > 0 {
		metadata["temperature"] = disk.Temperature
	}

	specCodes := smartHealthAssessmentCodes
	if alertType == "disk-wearout" {
		specCodes = smartWearoutAssessmentCodes
	}

	_, _ = m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
		SpecID:         resourceID + "-" + alertType,
		Signal:         "host-smart",
		Codes:          specCodes,
		Reasons:        reasons,
		AlertID:        alertID,
		AlertType:      alertType,
		SpecResourceID: resourceID,
		ResourceID:     resourceID,
		ResourceName:   resourceName,
		ResourceType:   unifiedresources.ResourceTypeAgent,
		Node:           nodeName,
		Instance:       instanceName,
		Metadata:       metadata,
	})
}

func (m *Manager) clearHostRAIDAlerts(hostID string) {
	if hostID == "" {
		return
	}

	resourcePrefix := hostResourceID(hostID) + "/raid:"

	m.mu.Lock()
	defer m.mu.Unlock()

	for storageKey, alert := range m.activeAlerts {
		if alert == nil || alert.Type != "raid" {
			continue
		}
		if strings.HasPrefix(alert.ResourceID, resourcePrefix) || strings.HasPrefix(alert.CanonicalSpecID, resourcePrefix) {
			m.clearAlertNoLock(storageKey)
		}
	}
}

func (m *Manager) clearHostUnraidAlerts(hostID string) {
	if hostID == "" {
		return
	}
	resourceID := fmt.Sprintf("%s/storage:unraid-array", hostResourceID(hostID))
	m.clearAlert(buildCanonicalStateID(resourceID, resourceID+"-health"))
}

func (m *Manager) syncHostUnraidStorageAlert(host models.Host, nodeName, instanceName, resourceName string, baseMetadata map[string]interface{}) {
	if host.Unraid == nil {
		m.clearHostUnraidAlerts(host.ID)
		return
	}

	assessment := storagehealth.AssessUnraidStorage(*host.Unraid)
	reasons := make([]storagehealth.Reason, 0, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		if reason.Severity == storagehealth.RiskWarning || reason.Severity == storagehealth.RiskCritical {
			reasons = append(reasons, reason)
		}
	}

	alertID := fmt.Sprintf("host-%s-unraid-array", host.ID)
	reasonCodes := storageHealthReasonCodes(reasons)
	reasonSummaries := storageHealthReasonSummaries(reasons)

	metadata := hostChildAlertMetadata(baseMetadata, hostStorageAlertResourceType)
	metadata["metric"] = "storageTopology"
	metadata["storagePlatform"] = "unraid"
	metadata["storageTopology"] = "array"
	metadata["arrayState"] = host.Unraid.ArrayState
	metadata["syncAction"] = host.Unraid.SyncAction
	metadata["syncProgress"] = host.Unraid.SyncProgress
	metadata["numProtected"] = host.Unraid.NumProtected
	metadata["numDisabled"] = host.Unraid.NumDisabled
	metadata["numInvalid"] = host.Unraid.NumInvalid
	metadata["numMissing"] = host.Unraid.NumMissing
	metadata["riskCodes"] = reasonCodes
	metadata["riskSummaries"] = reasonSummaries

	resourceID := fmt.Sprintf("%s/storage:unraid-array", hostResourceID(host.ID))
	resourceLabel := fmt.Sprintf("%s - Unraid Array", resourceName)

	_, _ = m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
		SpecID:         resourceID + "-health",
		Signal:         "unraid-storage",
		Reasons:        reasons,
		AlertID:        alertID,
		AlertType:      "storage-topology",
		SpecResourceID: resourceID,
		ResourceID:     resourceID,
		ResourceName:   resourceLabel,
		ResourceType:   unifiedresources.ResourceTypeAgent,
		Node:           nodeName,
		Instance:       instanceName,
		Metadata:       metadata,
	})
}
