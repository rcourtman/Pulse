package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

type hostDiskTempContext struct {
	hostID, linkedNodeID, linkedVMID, linkedContainerID, diskType string
}

// A positive legacy reading is still accepted. Explicit failed/unsupported
// collection, including expired last-known values, is never live evidence.
func hostDiskTemperatureObserved(disk models.HostDiskSMART) bool {
	if disk.Temperature <= 0 || disk.Standby {
		return false
	}
	return disk.Collection == nil || disk.Collection.Temperature.State == "" ||
		disk.Collection.Temperature.State == diskinventory.FieldAvailable
}

func (m *Manager) hostDiskTemperaturePendingNoLock(resourceID string) bool {
	for _, state := range m.mirrorStatesNoLock() {
		if incident, ok := state.Incident(resourceID, canonicalMetricSpecID(resourceID, "diskTemperature")); ok && incident.State == reducer.StatePending {
			return true
		}
	}
	_, pending := m.intentPending[canonicalMetricStateID(resourceID, "diskTemperature")]
	return pending
}

// Keep only the identity of a live pending run, not another telemetry cache.
func (m *Manager) rememberHostDiskTemperaturePending(host models.Host, disk models.HostDiskSMART, resourceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hostDiskTemperaturePendingNoLock(resourceID) {
		delete(m.hostDiskTempPendingContexts, resourceID)
		return
	}
	if m.hostDiskTempPendingContexts == nil {
		m.hostDiskTempPendingContexts = make(map[string]hostDiskTempContext)
	}
	m.hostDiskTempPendingContexts[resourceID] = hostDiskTempContext{host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID, disk.Type}
}

func (m *Manager) hostDiskTemperatureTrackedResourcesNoLock() map[string]struct{} {
	resources := make(map[string]struct{})
	for resourceID := range m.hostDiskTempPendingContexts {
		resources[resourceID] = struct{}{}
	}
	for _, state := range m.mirrorStatesNoLock() {
		for _, resourceID := range state.PendingResourceIDs() {
			resources[resourceID] = struct{}{}
		}
	}
	for _, alert := range m.activeAlerts {
		if alert != nil {
			resources[alert.ResourceID] = struct{}{}
		}
	}
	for _, pending := range m.intentPending {
		if pending.Signal == MetricAlertIntentSignal("diskTemperature") {
			resources[pending.ResourceID] = struct{}{}
		}
	}
	return resources
}

func (m *Manager) interruptHostDiskTemperatureRunNoLock(resourceID string) bool {
	delete(m.hostDiskTempPendingContexts, resourceID)
	return m.interruptMetricRunNoLock(resourceID, canonicalMetricSpecID(resourceID, "diskTemperature"), canonicalMetricStateID(resourceID, "diskTemperature"))
}

func (m *Manager) interruptHostDiskTemperatureRun(resourceID string) {
	m.mu.Lock()
	changed := m.interruptHostDiskTemperatureRunNoLock(resourceID)
	m.mu.Unlock()
	if changed {
		m.saveActiveAlertsAsync("disk temperature observation gap")
	}
}

func (m *Manager) interruptHostDiskTemperatureRuns(hostID string) {
	m.interruptHostDiskTemperatureRunsExcept(hostID, nil)
}

// Live shared SMART/Unraid rows are not observation gaps when SMART is empty.
func (m *Manager) interruptHostDiskTemperatureRunsExcept(hostID string, observed map[string]struct{}) {
	m.mu.Lock()
	changed := false
	for resourceID := range m.hostDiskTemperatureTrackedResourcesNoLock() {
		if _, present := observed[resourceID]; present {
			continue
		}
		if strings.HasPrefix(resourceID, hostDiskTemperatureResourcePrefix(hostID)) {
			changed = m.interruptHostDiskTemperatureRunNoLock(resourceID) || changed
		}
	}
	// Empty/expired inventory is not another confirmation that a disk left.
	for resourceID := range m.hostDiskTempAbsences {
		if strings.HasPrefix(resourceID, hostDiskTemperatureResourcePrefix(hostID)) {
			delete(m.hostDiskTempAbsences, resourceID)
		}
	}
	m.mu.Unlock()
	if changed {
		m.saveActiveAlertsAsync("host disk temperature observation gap")
	}
}

// Caller holds m.mu after config normalisation. Active alerts still use the
// existing per-type threshold re-evaluation; this only drops pending evidence.
func (m *Manager) reevaluateHostDiskTemperaturePendingNoLock() bool {
	intentChanged := false
	for resourceID := range m.hostDiskTemperatureTrackedResourcesNoLock() {
		if !strings.HasPrefix(resourceID, "agent:") || !strings.Contains(resourceID, "/disk_temp:") || !m.hostDiskTemperaturePendingNoLock(resourceID) {
			continue
		}
		ctx, known := m.hostDiskTempPendingContexts[resourceID]
		// Restarted intent grace has no retained link/type context. Do not
		// assume it was continuously enabled across a configuration save.
		if !known {
			intentChanged = m.interruptHostDiskTemperatureRunNoLock(resourceID) || intentChanged
			continue
		}
		thresholds := m.resolveHostThresholdsNoLock(ctx.hostID, ctx.linkedNodeID, ctx.linkedVMID, ctx.linkedContainerID)
		override, exists := m.hostThresholdOverrideNoLock(ctx.hostID, ctx.linkedNodeID, ctx.linkedVMID, ctx.linkedContainerID)
		threshold := m.hostDiskTemperatureThresholdNoLock(thresholds.DiskTemperature, exists && override.DiskTemperature != nil, ctx.diskType)
		allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("agent")
		if !m.config.Enabled || allDisabled || thresholds.Disabled || threshold == nil || threshold.Trigger <= 0 {
			intentChanged = m.interruptHostDiskTemperatureRunNoLock(resourceID) || intentChanged
		}
	}
	for resourceID := range m.hostDiskTempPendingContexts {
		if !m.hostDiskTemperaturePendingNoLock(resourceID) {
			delete(m.hostDiskTempPendingContexts, resourceID)
		}
	}
	return intentChanged
}
