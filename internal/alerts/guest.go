package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// ReevaluateGuestAlert reevaluates a specific guest's alerts with full threshold resolution including custom rules.
// This should be called by the monitor with the current guest state.
func (m *Manager) ReevaluateGuestAlert(guest any, guestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the correct thresholds for this guest (includes custom rules evaluation)
	thresholds := m.getGuestThresholds(guest, guestID)

	// Check all metric types for this guest
	metricTypes := []string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut"}

	for _, metricType := range metricTypes {
		alertID := canonicalMetricStateID(guestID, metricType)
		alert, exists := m.getActiveAlertNoLock(alertID)
		if !exists {
			alert, exists = m.getActiveAlertNoLock(fmt.Sprintf("%s-%s", guestID, metricType))
		}
		if !exists {
			continue
		}
		trackingKey := canonicalTrackingKeyForAlert(alert)
		if trackingKey == "" {
			trackingKey = alertID
		}

		// Get the threshold for this metric
		var threshold *HysteresisThreshold
		switch metricType {
		case "cpu":
			threshold = thresholds.CPU
		case "memory":
			threshold = thresholds.Memory
		case "disk":
			threshold = thresholds.Disk
		case "diskRead":
			threshold = thresholds.DiskRead
		case "diskWrite":
			threshold = thresholds.DiskWrite
		case "networkIn":
			threshold = thresholds.NetworkIn
		case "networkOut":
			threshold = thresholds.NetworkOut
		}

		// If threshold is disabled or doesn't exist, clear the alert
		if threshold == nil || threshold.Trigger <= 0 {
			m.clearAlertNoLock(trackingKey)
			// Also clear any pending alert for this metric
			if _, isPending := m.pendingAlerts[trackingKey]; isPending {
				delete(m.pendingAlerts, trackingKey)
				log.Debug().
					Str("alertID", alertID).
					Msg("Cleared pending alert - threshold disabled")
			}
			log.Info().
				Str("alertID", alertID).
				Str("metric", metricType).
				Msg("Cleared alert - threshold disabled")
			continue
		}

		// Check if alert should be cleared based on new threshold
		clearThreshold := threshold.Clear
		if clearThreshold <= 0 {
			clearThreshold = threshold.Trigger
		}

		if alert.Value <= clearThreshold || alert.Value < threshold.Trigger {
			m.clearAlertNoLock(trackingKey)
			log.Info().
				Str("alertID", alertID).
				Str("metric", metricType).
				Float64("value", alert.Value).
				Float64("trigger", threshold.Trigger).
				Float64("clear", clearThreshold).
				Msg("Cleared alert - value now below threshold after config change")
		}
	}
}

// CheckGuest checks a guest (VM or container) against thresholds.
func (m *Manager) CheckGuest(guest any, instanceName string) {
	m.mu.RLock()
	enabled := m.config.Enabled
	disableAllGuests := m.config.DisableAllGuests
	disableAllGuestsOffline := m.config.DisableAllGuestsOffline
	ignoredGuestPrefixes := m.config.IgnoredGuestPrefixes
	guestTagWhitelist := m.config.GuestTagWhitelist
	guestTagBlacklist := m.config.GuestTagBlacklist
	m.mu.RUnlock()

	if !enabled {
		log.Debug().Msg("checkGuest: alerts disabled globally")
		return
	}
	if disableAllGuests {
		log.Debug().Msg("checkGuest: all guest alerts disabled")
		return
	}

	snapshot, ok := extractGuestSnapshot(guest)
	if !ok {
		log.Debug().
			Str("type", fmt.Sprintf("%T", guest)).
			Msg("CheckGuest: unsupported guest type")
		return
	}

	guestID := snapshot.ID
	name := snapshot.Name
	node := snapshot.Node
	guestType := snapshot.displayType()
	status := snapshot.Status
	cpu := snapshot.CPUPercent
	memUsage := snapshot.MemUsage
	diskUsage := snapshot.DiskUsage
	diskRead := snapshot.DiskRead
	diskWrite := snapshot.DiskWrite
	netIn := snapshot.NetworkIn
	netOut := snapshot.NetworkOut
	disks := snapshot.Disks
	tags := snapshot.Tags

	// Debug logging for high memory VMs
	if snapshot.Kind == guestKindVM && memUsage > 85 {
		log.Debug().
			Str("vm", name).
			Float64("memUsage", memUsage).
			Str("status", status).
			Msg("VM with high memory detected in CheckGuest")
	}

	// Check ignored prefixes
	for _, prefix := range ignoredGuestPrefixes {
		if prefix != "" && strings.HasPrefix(name, prefix) {
			if cleared := m.suppressGuestAlerts(guestID); cleared {
				m.saveActiveAlertsAsync("ignored-prefix")
			}
			return
		}
	}

	settings := parsePulseTags(tags)
	if settings.Suppress {
		if cleared := m.suppressGuestAlerts(guestID); cleared {
			m.saveActiveAlertsAsync("pulse-no-alerts")
		}
		log.Debug().
			Str("guestID", guestID).
			Msg("Pulse no-alerts tag active; suppressing guest alerts")
		return
	}

	// Custom Tag Filtering
	if len(guestTagBlacklist) > 0 || len(guestTagWhitelist) > 0 {
		// Normalize tags once for checking
		normalizedTags := make(map[string]bool)
		for _, tag := range tags {
			normalizedTags[strings.ToLower(strings.TrimSpace(tag))] = true
		}

		// Check Blacklist
		for _, block := range guestTagBlacklist {
			if normalizedTags[strings.ToLower(strings.TrimSpace(block))] {
				if cleared := m.suppressGuestAlerts(guestID); cleared {
					m.saveActiveAlertsAsync("tag-blacklist")
				}
				log.Debug().Str("guestID", guestID).Msg("guest suppressed by tag blacklist")
				return
			}
		}

		// Check Whitelist
		if len(guestTagWhitelist) > 0 {
			found := false
			for _, allow := range guestTagWhitelist {
				if normalizedTags[strings.ToLower(strings.TrimSpace(allow))] {
					found = true
					break
				}
			}
			if !found {
				if cleared := m.suppressGuestAlerts(guestID); cleared {
					m.saveActiveAlertsAsync("tag-whitelist")
				}
				log.Debug().Str("guestID", guestID).Msg("guest suppressed by tag whitelist (required tag not found)")
				return
			}
		}
	}

	monitorOnly := settings.MonitorOnly
	if monitorOnly || m.guestHasMonitorOnlyAlerts(guestID) {
		log.Debug().
			Str("guest", name).
			Bool("monitorOnly", monitorOnly).
			Msg("Pulse monitor-only status applied")
	}

	// Handle non-running guests
	// Proxmox VM states: running, stopped, paused, suspended
	if status != "running" {
		// Check for powered-off state and generate alert if configured
		if status == "stopped" {
			if disableAllGuestsOffline {
				// Clear any pending powered-off tracking and alerts when globally disabled
				m.mu.Lock()
				delete(m.offlineConfirmations, guestID)
				m.mu.Unlock()
				m.clearAlert(canonicalPoweredStateStateID(guestID))
			} else {
				m.mu.RLock()
				thresholds := m.getGuestThresholds(guest, guestID)
				m.mu.RUnlock()
				m.checkGuestPoweredOffWithThresholds(guestID, name, node, instanceName, guestType, thresholds, monitorOnly)
			}
		} else {
			// For paused/suspended, clear powered-off alert
			m.clearGuestPoweredOffAlert(guestID, name)
		}

		alertsCleared := m.clearGuestMetricAlerts(guestID)

		if alertsCleared > 0 {
			log.Debug().
				Str("guest", name).
				Str("status", status).
				Int("alertsCleared", alertsCleared).
				Msg("Cleared metric alerts for non-running guest")
			m.saveActiveAlertsAsync("guest-not-running")
		}
		return
	}

	// If guest is running, clear any powered-off alert
	m.clearGuestPoweredOffAlert(guestID, name)

	// Get thresholds (check custom rules, then overrides, then defaults)
	m.mu.RLock()
	thresholds := m.getGuestThresholds(guest, guestID)
	m.mu.RUnlock()

	if settings.Relaxed {
		thresholds = applyRelaxedGuestThresholds(thresholds)
		log.Info().
			Str("guest", name).
			Float64("trigger", thresholds.CPU.Trigger).
			Msg("Applied relaxed thresholds for pulse-relaxed tag")
	}

	// If alerts are disabled for this guest, clear any existing alerts and return
	if thresholds.Disabled {
		if alertsCleared := m.clearGuestMetricAlerts(guestID); alertsCleared > 0 {
			log.Info().
				Str("guest", name).
				Int("alertsCleared", alertsCleared).
				Msg("Cleared guest metric alerts because alerts are disabled")
			m.saveActiveAlertsAsync("guest-alerts-disabled")
		}
		return
	}

	// Check each metric
	log.Debug().
		Str("guest", name).
		Float64("cpu", cpu).
		Float64("memory", memUsage).
		Float64("disk", diskUsage).
		Interface("thresholds", thresholds).
		Msg("Checking guest thresholds")

	// Evaluate standard metrics through unified path
	var evalOpts *metricOptions
	if monitorOnly {
		evalOpts = &metricOptions{MonitorOnly: true}
	}
	m.evaluateUnifiedMetrics(&UnifiedResourceInput{
		ID:         guestID,
		Type:       snapshot.resourceType(),
		Name:       name,
		Node:       node,
		Instance:   instanceName,
		CPU:        &UnifiedResourceMetric{Percent: cpu},
		Memory:     &UnifiedResourceMetric{Percent: memUsage},
		Disk:       &UnifiedResourceMetric{Percent: diskUsage},
		DiskRead:   &UnifiedResourceMetric{Value: float64(diskRead) / 1024 / 1024},
		DiskWrite:  &UnifiedResourceMetric{Value: float64(diskWrite) / 1024 / 1024},
		NetworkIn:  &UnifiedResourceMetric{Value: float64(netIn) / 1024 / 1024},
		NetworkOut: &UnifiedResourceMetric{Value: float64(netOut) / 1024 / 1024},
	}, thresholds, evalOpts)

	if thresholds.Disk != nil && thresholds.Disk.Trigger > 0 && len(disks) > 0 {
		seenDiskKeys := make(map[string]struct{})
		seenDiskResources := make(map[string]struct{})
		for idx, disk := range disks {
			if disk.Total <= 0 {
				continue
			}
			if disk.Usage < 0 {
				continue
			}

			label := strings.TrimSpace(disk.Mountpoint)
			if label == "" {
				label = strings.TrimSpace(disk.Device)
			}
			if label == "" {
				label = fmt.Sprintf("Disk %d", idx+1)
			}

			keySource := label
			if disk.Device != "" && !strings.EqualFold(disk.Device, label) {
				keySource = fmt.Sprintf("%s-%s", label, disk.Device)
			}
			sanitizedKey := sanitizeAlertKey(keySource)
			if sanitizedKey == "" {
				sanitizedKey = fmt.Sprintf("disk-%d", idx+1)
			}

			// Avoid duplicate checks if two disks resolve to the same key
			if _, exists := seenDiskKeys[sanitizedKey]; exists {
				continue
			}
			seenDiskKeys[sanitizedKey] = struct{}{}

			perDiskResourceID := fmt.Sprintf("%s-disk-%s", guestID, sanitizedKey)
			seenDiskResources[perDiskResourceID] = struct{}{}
			message := fmt.Sprintf("%s disk (%s) at %.1f%%", guestType, label, disk.Usage)

			log.Debug().
				Str("guest", name).
				Str("node", node).
				Str("instance", instanceName).
				Str("diskLabel", label).
				Float64("usage", disk.Usage).
				Msg("Evaluating individual disk for alert thresholds")

			metadata := map[string]interface{}{
				"mountpoint": disk.Mountpoint,
				"device":     disk.Device,
				"diskType":   disk.Type,
				"totalBytes": disk.Total,
				"usedBytes":  disk.Used,
				"freeBytes":  disk.Free,
				"diskIndex":  idx,
				"label":      label,
			}
			resourceType, ok := unifiedMetricResourceType(snapshot.resourceType())
			if !ok {
				m.checkMetric(perDiskResourceID, name, node, instanceName, snapshot.resourceType(), "disk", disk.Usage, thresholds.Disk, &metricOptions{
					Metadata:    metadata,
					Message:     message,
					MonitorOnly: monitorOnly,
				})
				continue
			}
			spec, err := buildCanonicalMetricSpec(perDiskResourceID, name, resourceType, "disk", thresholds.Disk)
			if err != nil {
				log.Warn().
					Err(err).
					Str("resourceID", perDiskResourceID).
					Str("guest", name).
					Msg("Skipping invalid canonical guest disk metric spec")
				continue
			}

			m.checkMetricWithCanonicalSpec(spec, name, node, instanceName, snapshot.resourceType(), disk.Usage, thresholds.Disk, &metricOptions{
				Metadata:    metadata,
				Message:     message,
				MonitorOnly: monitorOnly,
			})
		}
		if cleared := m.cleanupGuestDiskAlerts(guestID, seenDiskResources); cleared > 0 {
			m.saveActiveAlertsAsync("guest-disk-set-changed")
		}
	} else if cleared := m.cleanupGuestDiskAlerts(guestID, nil); cleared > 0 {
		m.saveActiveAlertsAsync("guest-disk-alerts-cleared")
	}
}

// checkGuestPoweredOff creates an alert for powered-off guests.
func (m *Manager) checkGuestPoweredOff(guestID, name, node, instanceName, guestType string, monitorOnly bool) {
	m.mu.RLock()
	thresholds := m.getGuestThresholds(guestSnapshotFromIdentity(guestID, name, node, instanceName, guestType, "stopped"), guestID)
	m.mu.RUnlock()
	m.checkGuestPoweredOffWithThresholds(guestID, name, node, instanceName, guestType, thresholds, monitorOnly)
}

func (m *Manager) checkGuestPoweredOffWithThresholds(guestID, name, node, instanceName, guestType string, thresholds ThresholdConfig, monitorOnly bool) {
	alertID := fmt.Sprintf("guest-powered-off-%s", guestID)
	severity := NormalizePoweredOffSeverity(thresholds.PoweredOffSeverity)
	resourceType := unifiedresources.ResourceTypeVM
	if strings.EqualFold(guestType, "container") {
		resourceType = unifiedresources.ResourceTypeSystemContainer
	}
	spec, err := buildCanonicalPoweredStateSpec(guestID, name, resourceType, severity, 2, thresholds.Disabled || thresholds.DisableConnectivity)
	if err != nil {
		log.Warn().
			Err(err).
			Str("guest", name).
			Str("guestID", guestID).
			Msg("Skipping invalid canonical guest powered-state spec")
		return
	}

	m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			PoweredState: &alertspecs.PoweredStateEvidence{
				Expected: alertspecs.PowerStateOn,
				Observed: alertspecs.PowerStateOff,
			},
		},
		Tracking:     m.offlineConfirmations,
		TrackingKey:  guestID,
		AlertID:      alertID,
		AlertType:    "powered-off",
		ResourceID:   guestID,
		ResourceName: name,
		Node:         node,
		Instance:     instanceName,
		Message:      fmt.Sprintf("%s '%s' is powered off", guestType, name),
		Metadata: map[string]interface{}{
			"monitorOnly":  monitorOnly,
			"resourceType": strings.ToLower(guestType),
		},
		AddToRecent:   true,
		AddToHistory:  true,
		DispatchAsync: false,
	})
}

// clearGuestPoweredOffAlert removes powered-off alert when guest starts running.
func (m *Manager) clearGuestPoweredOffAlert(guestID, name string) {
	alertID := canonicalPoweredStateStateID(guestID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset confirmation count when guest comes back online
	if count, exists := m.offlineConfirmations[guestID]; exists && count > 0 {
		log.Debug().
			Str("guest", name).
			Int("previousCount", count).
			Msg("Guest is running, resetting powered-off confirmation count")
		delete(m.offlineConfirmations, guestID)
	}

	// Check if powered-off alert exists
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		return
	}

	// Remove from active alerts
	m.removeActiveAlertNoLock(alertID)

	downtime := time.Since(alert.StartTime)
	resolvedAlert := &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: time.Now(),
	}
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

	// Send recovery notification (async to avoid deadlock because callback acquires m.mu.RLock
	// via ShouldSuppressResolvedNotification, and we currently hold m.mu.Lock)
	m.safeCallResolvedAlertCallback(alert, effectiveAlertID(alert, alertID), true)

	// Log recovery
	log.Info().
		Str("guest", name).
		Dur("downtime", downtime).
		Msg("Guest is now running")
}

type pulseTagSettings struct {
	Suppress    bool
	MonitorOnly bool
	Relaxed     bool
}

func parsePulseTags(tags []string) pulseTagSettings {
	settings := pulseTagSettings{}
	for _, raw := range tags {
		tag := strings.TrimSpace(strings.ToLower(raw))
		switch tag {
		case "pulse-no-alerts":
			settings.Suppress = true
		case "pulse-monitor-only":
			settings.MonitorOnly = true
		case "pulse-relaxed":
			settings.Relaxed = true
		}
	}
	return settings
}

func applyRelaxedGuestThresholds(cfg ThresholdConfig) ThresholdConfig {
	relaxed := cloneThresholdConfig(cfg)

	adjust := func(th **HysteresisThreshold, minTrigger float64) {
		if *th == nil {
			*th = &HysteresisThreshold{Trigger: minTrigger, Clear: minTrigger - 5}
			return
		}
		ensureHysteresisThreshold(*th)
		if (*th).Trigger < minTrigger {
			(*th).Trigger = minTrigger
		}
		if (*th).Clear >= (*th).Trigger {
			(*th).Clear = (*th).Trigger - 5
		}
		if (*th).Clear < 0 {
			(*th).Clear = 0
		}
	}

	adjust(&relaxed.CPU, 95)
	adjust(&relaxed.Memory, 92)
	adjust(&relaxed.Disk, 95)

	return relaxed
}

func (m *Manager) suppressGuestAlerts(guestID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleared := false

	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		trackingKey := canonicalTrackingKeyForAlert(alert)
		if alert == nil {
			continue
		}
		if alert.ResourceID == guestID || strings.HasPrefix(alert.ResourceID, guestID+"/") || strings.HasPrefix(alertID, guestID) {
			m.clearAlertNoLock(alertID)
			delete(m.recentAlerts, trackingKey)
			delete(m.pendingAlerts, trackingKey)
			delete(m.suppressedUntil, trackingKey)
			delete(m.alertRateLimit, trackingKey)
			cleared = true
		}
	}

	for key := range m.pendingAlerts {
		if strings.HasPrefix(key, guestID) {
			delete(m.pendingAlerts, key)
		}
	}
	for key := range m.recentAlerts {
		if strings.HasPrefix(key, guestID) {
			delete(m.recentAlerts, key)
		}
	}
	for key := range m.suppressedUntil {
		if strings.HasPrefix(key, guestID) {
			delete(m.suppressedUntil, key)
		}
	}
	for key := range m.alertRateLimit {
		if strings.HasPrefix(key, guestID) {
			delete(m.alertRateLimit, key)
		}
	}

	delete(m.offlineConfirmations, guestID)

	return cleared
}

func (m *Manager) guestHasMonitorOnlyAlerts(guestID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		if alert.ResourceID != guestID {
			continue
		}
		if isMonitorOnlyAlert(alert) {
			return true
		}
	}

	return false
}
