package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// An empty filesystem inventory is not evidence that every filesystem left.
// Interrupt pending/recovery timing even for rows no longer in the report,
// preserving open incidents unless current policy explicitly switches them off.
// A non-empty inventory still retires absent rows through cleanupHostDiskAlerts.
func (m *Manager) reconcileHostFilesystemObservationGaps(host models.Host, seen map[string]struct{}) {
	prefix := hostResourceID(host.ID) + "/disk:"
	m.mu.Lock()
	resources := make(map[string]struct{})
	for _, state := range m.mirrorStatesNoLock() {
		for _, resourceID := range state.PendingResourceIDs() {
			resources[resourceID] = struct{}{}
		}
	}
	for _, pending := range m.intentPending {
		if pending.Signal == MetricAlertIntentSignal("disk") {
			resources[pending.ResourceID] = struct{}{}
		}
	}
	thresholds := m.resolveHostThresholdsNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	override, exists := m.hostThresholdOverrideNoLock(host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID)
	for storageKey, alert := range m.activeAlerts {
		if alert == nil || alert.Type != "disk" || !strings.HasPrefix(alert.ResourceID, prefix) {
			continue
		}
		resources[alert.ResourceID] = struct{}{}
		if len(host.Disks) == 0 {
			threshold, disabled := m.hostDiskUsageThresholdNoLock(alert.ResourceID, metadataStringValue(alert.Metadata, "device"), thresholds.Disk, exists && override.Disk != nil)
			if disabled || !liveHysteresisThreshold(threshold) {
				m.clearAlertNoLock(storageKey)
			}
		}
	}
	intentChanged := false
	for resourceID := range resources {
		if !strings.HasPrefix(resourceID, prefix) {
			continue
		}
		if _, observed := seen[resourceID]; observed {
			continue
		}
		intentChanged = m.interruptMetricRunNoLock(resourceID, canonicalMetricSpecID(resourceID, "disk"), canonicalMetricStateID(resourceID, "disk")) || intentChanged
	}
	m.mu.Unlock()
	if intentChanged {
		m.saveActiveAlertsAsync("host filesystem observation gap")
	}
}
