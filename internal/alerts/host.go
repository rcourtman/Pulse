package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

func hostResourceID(hostID string) string {
	trimmed := strings.TrimSpace(hostID)
	if trimmed == "" {
		return "agent:unknown"
	}
	return fmt.Sprintf("agent:%s", trimmed)
}

func stripHostResourcePrefix(resourceID string) string {
	trimmed := strings.TrimSpace(resourceID)
	trimmed = strings.TrimPrefix(trimmed, "agent:")
	return strings.TrimSpace(trimmed)
}

func hostDisplayName(host models.Host) string {
	if name := strings.TrimSpace(host.DisplayName); name != "" {
		return name
	}
	if name := strings.TrimSpace(host.Hostname); name != "" {
		return name
	}
	if host.ID != "" {
		return host.ID
	}
	return "Agent"
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

	if hostID = strings.TrimSpace(hostID); hostID != "" {
		if override, exists := m.config.Overrides[hostID]; exists {
			return m.applyThresholdOverride(base, override)
		}
	}

	if linkedNodeID = strings.TrimSpace(linkedNodeID); linkedNodeID != "" {
		if override, exists := m.config.Overrides[linkedNodeID]; exists {
			return m.applyThresholdOverride(base, override)
		}
	}

	if linkedVMID = strings.TrimSpace(linkedVMID); linkedVMID != "" {
		if override, exists := lookupGuestOverride(m.config.Overrides, nil, linkedVMID); exists {
			return m.applyThresholdOverride(base, override)
		}
	}

	if linkedContainerID = strings.TrimSpace(linkedContainerID); linkedContainerID != "" {
		if override, exists := lookupGuestOverride(m.config.Overrides, nil, linkedContainerID); exists {
			return m.applyThresholdOverride(base, override)
		}
	}

	return base
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

	return m.resolveHostThresholdsNoLock(hostID, linkedNodeID, linkedVMID, linkedContainerID)
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

	// Register this host agent hostname for deduplication with Proxmox nodes.
	// This prevents duplicate alerts when both a Node and Host agent monitor the same machine.
	if host.Hostname != "" {
		m.RegisterHostAgentHostname(host.Hostname)
	}

	// Cache display name so host alerts show the user-configured name.
	m.UpdateNodeDisplayName("", host.Hostname, host.DisplayName)

	// Fresh telemetry marks the host as online and clears offline tracking.
	m.HandleHostOnline(host)

	m.mu.RLock()
	alertsEnabled := m.config.Enabled
	disableAllAgents := m.config.DisableAllAgents
	thresholds := m.resolveHostThresholdsNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	m.mu.RUnlock()

	if !alertsEnabled {
		return
	}

	if disableAllAgents {
		// Clear any existing host alerts when all host alerts are disabled
		m.clearHostMetricAlerts(host.ID)
		m.clearHostDiskAlerts(host.ID)
		m.clearHostRAIDAlerts(host.ID)
		m.clearHostUnraidAlerts(host.ID)
		return
	}

	if thresholds.Disabled {
		m.clearHostMetricAlerts(host.ID)
		m.clearHostDiskAlerts(host.ID)
		m.clearHostRAIDAlerts(host.ID)
		m.clearHostUnraidAlerts(host.ID)
		return
	}

	resourceID := hostResourceID(host.ID)
	resourceName := hostDisplayName(host)
	nodeName := strings.TrimSpace(host.Hostname)
	instanceName := hostInstanceName(host)

	baseMetadata := map[string]interface{}{
		"resourceType": "agent",
		"hostId":       host.ID,
		"hostname":     host.Hostname,
		"displayName":  host.DisplayName,
		"platform":     host.Platform,
		"osName":       host.OSName,
		"osVersion":    host.OSVersion,
		"agentVersion": host.AgentVersion,
		"architecture": host.Architecture,
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
			m.checkMetricWithCanonicalSpec(spec, resourceName, nodeName, instanceName, "agent", host.CPUUsage, thresholds.CPU, &metricOptions{Metadata: cpuMetadata})
		}
	} else {
		m.clearHostMetricAlerts(host.ID, "cpu")
	}

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
		} else {
			m.checkMetricWithCanonicalSpec(spec, resourceName, nodeName, instanceName, "agent", host.Memory.Usage, thresholds.Memory, &metricOptions{Metadata: memMetadata})
		}
	} else {
		m.clearHostMetricAlerts(host.ID, "memory")
	}

	if thresholds.DiskTemperature != nil && thresholds.DiskTemperature.Trigger > 0 {
		if len(host.Sensors.SMART) > 0 {
			for _, disk := range host.Sensors.SMART {
				if disk.Temperature > 0 && !disk.Standby {
					// Use specific resource ID for the disk: hostID/disk-temp:device
					tempResourceID := fmt.Sprintf("%s/disk_temp:%s", hostResourceID(host.ID), sanitizeHostComponent(disk.Device))
					tempResourceName := fmt.Sprintf("%s (%s Temp)", host.DisplayName, disk.Device)

					diskTempMetadata := cloneMetadata(baseMetadata)
					diskTempMetadata["metric"] = "diskTemperature"
					diskTempMetadata["device"] = disk.Device
					diskTempMetadata["temperature"] = disk.Temperature
					diskTempMetadata["model"] = disk.Model
					spec, err := buildCanonicalMetricSpec(tempResourceID, tempResourceName, unifiedresources.ResourceType("agent-disk"), "diskTemperature", thresholds.DiskTemperature)
					if err != nil {
						log.Warn().
							Err(err).
							Str("resourceID", tempResourceID).
							Str("host", resourceName).
							Str("device", disk.Device).
							Msg("Skipping invalid canonical host disk temperature metric spec")
						continue
					}

					m.checkMetricWithCanonicalSpec(spec, tempResourceName, nodeName, disk.Device, "agent", float64(disk.Temperature), thresholds.DiskTemperature, &metricOptions{Metadata: diskTempMetadata})
				}
			}
		}
	} else {
		// We can't easily clear all disk temp alerts without tracking them,
		// but checkMetric logic handles auto-resolution if value drops.
		// If feature is disabled, ideally we should clear existing alerts.
		// For now simple implementation.
	}

	seenDisks := make(map[string]struct{}, len(host.Disks))
	if len(host.Sensors.SMART) > 0 {
		for _, disk := range host.Sensors.SMART {
			diskResourceID, diskName := hostSMARTDiskResourceID(host, disk)
			if host.LinkedNodeID == "" {
				seenDisks[diskResourceID] = struct{}{}
				m.syncHostSMARTDiskRiskAlerts(host, disk, diskResourceID, diskName, nodeName, instanceName, baseMetadata)
				continue
			}
			m.syncHostSMARTDiskAlert(host, disk, diskResourceID, diskName, nodeName, instanceName, baseMetadata, "disk-health", nil)
			m.syncHostSMARTDiskAlert(host, disk, diskResourceID, diskName, nodeName, instanceName, baseMetadata, "disk-wearout", nil)
		}
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
				m.clearAlert(canonicalMetricStateID(diskResourceID, "disk"))
				continue
			}
			// Use disk-specific threshold if set
			if diskOverride.Disk != nil {
				effectiveDiskThreshold = ensureHysteresisThreshold(diskOverride.Disk)
			}
		}
		// Per-type override: consult DiskFillByType if hardware type is inferable
		// from the device path and no disk-specific override applied above.
		if effectiveDiskThreshold == nil && thresholds.Disk != nil && thresholds.Disk.Trigger > 0 {
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
			continue
		}

		diskMetadata := cloneMetadata(baseMetadata)
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

		m.checkMetricWithCanonicalSpec(spec, diskName, nodeName, instanceName, "agent-disk", disk.Usage, effectiveDiskThreshold, &metricOptions{Metadata: diskMetadata})
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

			raidMetadata := cloneMetadata(baseMetadata)
			raidMetadata["metric"] = "raid"
			raidMetadata["raidDevice"] = array.Device
			raidMetadata["raidLevel"] = array.Level
			raidMetadata["raidState"] = array.State
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
	delete(m.offlineConfirmations, resourceKey)
	exists := m.hasActiveAlertNoLock(alertID)
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

	// Unregister the host agent hostname since it's being removed.
	if host.Hostname != "" {
		m.UnregisterHostAgentHostname(host.Hostname)
	}

	m.HandleHostOnline(host)
	m.clearHostMetricAlerts(host.ID)
	m.clearHostDiskAlerts(host.ID)
	m.clearHostRAIDAlerts(host.ID)
	m.clearHostUnraidAlerts(host.ID)
}

// HandleHostOffline raises an alert when a host agent stops reporting.
func (m *Manager) HandleHostOffline(host models.Host) {
	if host.ID == "" {
		return
	}

	// Unregister the host agent hostname since it's no longer actively monitoring.
	// This allows node alerts to resume if a Proxmox node with the same hostname exists.
	if host.Hostname != "" {
		m.UnregisterHostAgentHostname(host.Hostname)
	}

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	disableHostsOffline := m.config.DisableAllAgentsOffline
	thresholds := m.resolveHostThresholdsNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	m.mu.RUnlock()

	resourceKey := hostResourceID(host.ID)
	alertID := canonicalConnectivityStateID(resourceKey)
	resourceName := hostDisplayName(host)
	nodeName := strings.TrimSpace(host.Hostname)
	instanceName := hostInstanceName(host)

	if disableHostsOffline {
		m.mu.Lock()
		delete(m.offlineConfirmations, resourceKey)
		m.mu.Unlock()
		m.clearAlert(alertID)
		return
	}

	if thresholds.Disabled || thresholds.DisableConnectivity {
		m.clearAlert(alertID)
		m.mu.Lock()
		delete(m.offlineConfirmations, resourceKey)
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
		Tracking:     m.offlineConfirmations,
		TrackingKey:  resourceKey,
		AlertID:      alertID,
		AlertType:    "host-offline",
		ResourceID:   resourceKey,
		ResourceName: resourceName,
		Node:         nodeName,
		Instance:     instanceName,
		Message:      fmt.Sprintf("Host '%s' is offline", resourceName),
		Metadata: map[string]interface{}{
			"resourceType":      "agent",
			"hostId":            host.ID,
			"hostname":          host.Hostname,
			"displayName":       host.DisplayName,
			"platform":          host.Platform,
			"osName":            host.OSName,
			"osVersion":         host.OSVersion,
			"linkedNodeId":      strings.TrimSpace(host.LinkedNodeID),
			"linkedVmId":        strings.TrimSpace(host.LinkedVMID),
			"linkedContainerId": strings.TrimSpace(host.LinkedContainerID),
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
			m.clearAlert(canonicalMetricStateID(resourceID, metric))
		}
	}
}

func (m *Manager) clearHostDiskAlerts(hostID string) {
	if hostID == "" {
		return
	}

	prefixes := []string{
		fmt.Sprintf("%s/disk:", hostResourceID(hostID)),
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
		m.clearAlertNoLock(alertID)
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
		if alert == nil || alert.Type == "powered-off" {
			continue
		}
		if alert.ResourceID != guestID && !strings.HasPrefix(alert.ResourceID, perDiskPrefix) {
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
		if alert == nil || !strings.HasPrefix(alert.ResourceID, prefix) {
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
		if _, exists := seen[alert.ResourceID]; exists {
			continue
		}
		m.clearAlertNoLock(alertID)
	}
}

func (m *Manager) syncHostSMARTDiskRiskAlerts(host models.Host, disk models.HostDiskSMART, resourceID, resourceName, nodeName, instanceName string, baseMetadata map[string]interface{}) {
	assessment := storagehealth.AssessHostSMARTDisk(disk)
	healthReasons, wearReasons := splitSMARTAlertReasons(assessment.Reasons)

	m.syncHostSMARTDiskAlert(host, disk, resourceID, resourceName, nodeName, instanceName, baseMetadata, "disk-health", healthReasons)
	m.syncHostSMARTDiskAlert(host, disk, resourceID, resourceName, nodeName, instanceName, baseMetadata, "disk-wearout", wearReasons)
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
		case "temperature_high":
			continue
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

	metadata := cloneMetadata(baseMetadata)
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

	metadata := cloneMetadata(baseMetadata)
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
