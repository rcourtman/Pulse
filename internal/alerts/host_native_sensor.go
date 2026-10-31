package alerts

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog/log"
)

// hostNativeSensorMetric is the metric a native temperature sensor's
// threshold judges. It shares the temperature formatting, stability delay and
// quiet-hours category, while its resource ID keeps it apart from the node
// CPU temperature alert.
const hostNativeSensorMetric = "temperature"

func hostNativeSensorResourcePrefix(hostID string) string {
	return hostResourceID(hostID) + "/sensor:"
}

// HostNativeSensorResourceID is the alert resource ID of one native sensor:
// "agent:<host ID>/sensor:<lower-case hex of the sensor ID's UTF-8 bytes>".
// The sensor ID ("<chip>/<channel>") can differ from another only by case or
// punctuation, and host child IDs and metric names are lower-cased elsewhere,
// so the hex form is the collision-free key its threshold override and alert
// are stored under.
func HostNativeSensorResourceID(hostID, sensorID string) string {
	return hostNativeSensorResourcePrefix(hostID) + hex.EncodeToString([]byte(sensorID))
}

// isHostNativeSensorResourceID reports whether resourceID names one native
// sensor of a host agent.
func isHostNativeSensorResourceID(resourceID string) bool {
	_, child, ok := strings.Cut(stripHostResourcePrefix(resourceID), "/")
	return ok && strings.HasPrefix(child, "sensor:")
}

// hostNativeSensorThresholdNoLock is the threshold a native sensor is judged
// by: its own override's temperature, and nothing else. A sensor has no
// default and inherits no host, linked-node or CPU temperature threshold, so
// a missing, disabled or zero override means it raises nothing. The result
// is an owned copy. Callers must hold m.mu.
func (m *Manager) hostNativeSensorThresholdNoLock(resourceID string) *HysteresisThreshold {
	override, ok := m.config.Overrides[resourceID]
	if !ok || override.Disabled || override.Temperature == nil || override.Temperature.Trigger <= 0 {
		return nil
	}
	return ensureHysteresisThreshold(cloneThreshold(override.Temperature))
}

// hostNativeSensorTrackedResourcesNoLock lists the host's native sensor
// resources that hold evaluator state: an active alert, a pending reducer
// run, pending intent or a configured override. Cleanup walks this set, so
// removing an override still reaches the run it started. Callers must hold
// m.mu.
func (m *Manager) hostNativeSensorTrackedResourcesNoLock(hostID string) map[string]struct{} {
	prefix := hostNativeSensorResourcePrefix(hostID)
	resources := make(map[string]struct{})
	add := func(resourceID string) {
		if strings.HasPrefix(resourceID, prefix) {
			resources[resourceID] = struct{}{}
		}
	}
	for resourceID := range m.config.Overrides {
		add(resourceID)
	}
	for _, alert := range m.activeAlerts {
		if alert != nil {
			add(alert.ResourceID)
		}
	}
	for _, state := range m.mirrorStatesNoLock() {
		for _, resourceID := range state.PendingResourceIDs() {
			add(resourceID)
		}
	}
	for _, pending := range m.intentPending {
		add(pending.ResourceID)
	}
	return resources
}

// hostNativeSensorContext is the host link identity a pending native sensor
// run was started under, so a configuration save can re-resolve the host's
// policy without another telemetry cache.
type hostNativeSensorContext struct {
	hostID, linkedNodeID, linkedVMID, linkedContainerID string
}

func (m *Manager) hostNativeSensorPendingNoLock(resourceID string) bool {
	specID := canonicalMetricSpecID(resourceID, hostNativeSensorMetric)
	for _, state := range m.mirrorStatesNoLock() {
		if incident, ok := state.Incident(resourceID, specID); ok && incident.State == reducer.StatePending {
			return true
		}
	}
	_, pending := m.intentPending[canonicalMetricStateID(resourceID, hostNativeSensorMetric)]
	return pending
}

// rememberHostNativeSensorPending keeps the host link identity of a live
// pending run, and forgets it once the run is no longer pending.
func (m *Manager) rememberHostNativeSensorPending(host models.Host, resourceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hostNativeSensorPendingNoLock(resourceID) {
		delete(m.hostNativeSensorPendingCtx, resourceID)
		return
	}
	if m.hostNativeSensorPendingCtx == nil {
		m.hostNativeSensorPendingCtx = make(map[string]hostNativeSensorContext)
	}
	m.hostNativeSensorPendingCtx[resourceID] = hostNativeSensorContext{host.ID, host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID}
}

func (m *Manager) interruptHostNativeSensorRunNoLock(resourceID string) bool {
	delete(m.hostNativeSensorPendingCtx, resourceID)
	return m.interruptMetricRunNoLock(resourceID, canonicalMetricSpecID(resourceID, hostNativeSensorMetric), canonicalMetricStateID(resourceID, hostNativeSensorMetric))
}

// clearHostNativeSensorResourceNoLock resolves a native sensor's alert and
// drops its pending run. Callers must hold m.mu.
func (m *Manager) clearHostNativeSensorResourceNoLock(resourceID string) bool {
	changed := m.interruptHostNativeSensorRunNoLock(resourceID)
	m.core.DropPendingForResource(resourceID)
	for storageKey, alert := range m.activeAlerts {
		if alert != nil && alert.ResourceID == resourceID {
			m.clearAlertNoLock(storageKey)
			changed = true
		}
	}
	return changed
}

// clearHostNativeSensorAlerts resolves every native sensor alert of a host
// and drops their pending runs: the host was removed, or its alerting was
// switched off, so no later reading will resolve them.
func (m *Manager) clearHostNativeSensorAlerts(hostID string) {
	if strings.TrimSpace(hostID) == "" {
		return
	}
	m.mu.Lock()
	changed := false
	for resourceID := range m.hostNativeSensorTrackedResourcesNoLock(hostID) {
		changed = m.clearHostNativeSensorResourceNoLock(resourceID) || changed
	}
	m.mu.Unlock()
	if changed {
		m.saveActiveAlertsAsync("host native sensor alerts cleared")
	}
}

// interruptHostNativeSensorRuns restarts timing for every native sensor of a
// host without resolving a firing alert: its readings stopped arriving.
func (m *Manager) interruptHostNativeSensorRuns(hostID string) {
	if strings.TrimSpace(hostID) == "" {
		return
	}
	m.mu.Lock()
	changed := false
	for resourceID := range m.hostNativeSensorTrackedResourcesNoLock(hostID) {
		changed = m.interruptHostNativeSensorRunNoLock(resourceID) || changed
	}
	m.mu.Unlock()
	if changed {
		m.saveActiveAlertsAsync("host native sensor observation gap")
	}
}

// syncHostNativeSensorAlerts judges each native temperature sensor that has
// its own threshold override. A usable reading is evaluated on the canonical
// metric path. An invalid reading, a reading without a value, or a configured
// sensor missing from the report interrupts the run and holds any firing
// alert: the inventory carries no completeness state, so absence is not
// evidence that the sensor left. Removing or disabling the override resolves
// the alert. CheckHost calls this only while the host's alerting is on.
func (m *Manager) syncHostNativeSensorAlerts(host models.Host, nodeName, instanceName string, baseMetadata map[string]interface{}) {
	reported := make(map[string]models.HostNativeSensor, len(host.Sensors.Native))
	for _, sensor := range host.Sensors.Native {
		if sensor.Kind != agentshost.NativeSensorKindTemperature || sensor.ID == "" {
			continue
		}
		resourceID := HostNativeSensorResourceID(host.ID, sensor.ID)
		if _, duplicate := reported[resourceID]; !duplicate {
			reported[resourceID] = sensor
		}
	}

	m.mu.Lock()
	tracked := m.hostNativeSensorTrackedResourcesNoLock(host.ID)
	thresholds := make(map[string]*HysteresisThreshold, len(tracked))
	changed := false
	for resourceID := range tracked {
		if threshold := m.hostNativeSensorThresholdNoLock(resourceID); threshold != nil {
			thresholds[resourceID] = threshold
			continue
		}
		changed = m.clearHostNativeSensorResourceNoLock(resourceID) || changed
	}
	m.mu.Unlock()
	if changed {
		m.saveActiveAlertsAsync("host native sensor threshold removed")
	}

	resourceIDs := make([]string, 0, len(thresholds))
	for resourceID := range thresholds {
		resourceIDs = append(resourceIDs, resourceID)
	}
	sort.Strings(resourceIDs)
	for _, resourceID := range resourceIDs {
		threshold := thresholds[resourceID]
		sensor, present := reported[resourceID]
		if !present || sensor.Status != agentshost.NativeSensorStatusOK || sensor.Value == nil {
			m.mu.Lock()
			interrupted := m.interruptHostNativeSensorRunNoLock(resourceID)
			m.mu.Unlock()
			if interrupted {
				m.saveActiveAlertsAsync("host native sensor observation gap")
			}
			continue
		}

		label := strings.TrimSpace(sensor.Label)
		if label == "" {
			label = sensor.ID
		}
		resourceName := fmt.Sprintf("%s (%s)", hostDisplayName(host), label)
		metadata := hostChildAlertMetadata(baseMetadata, hostSensorAlertResourceType)
		metadata["metric"] = hostNativeSensorMetric
		metadata["sensorId"] = sensor.ID
		metadata["sensorChip"] = sensor.Chip
		metadata["sensorChannel"] = sensor.Channel
		metadata["sensorLabel"] = sensor.Label
		metadata["temperature"] = *sensor.Value

		spec, err := buildCanonicalMetricSpec(resourceID, resourceName, unifiedresources.ResourceType(hostSensorAlertResourceType), hostNativeSensorMetric, threshold)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", resourceID).
				Str("host", hostDisplayName(host)).
				Str("sensor", sensor.ID).
				Msg("Skipping invalid canonical host native sensor metric spec")
			m.mu.Lock()
			m.interruptHostNativeSensorRunNoLock(resourceID)
			m.mu.Unlock()
			continue
		}
		message := fmt.Sprintf("%s temperature at %s", label, formatMetricValue(*sensor.Value, "°C"))
		m.checkMetricWithCanonicalSpec(spec, resourceName, nodeName, instanceName, hostSensorAlertResourceType, *sensor.Value, threshold, &metricOptions{Metadata: metadata, Message: message})
		m.rememberHostNativeSensorPending(host, resourceID)
	}
}

// reevaluateHostNativeSensorPendingNoLock drops the pending evidence of every
// native sensor whose policy a configuration save switched off: its own
// threshold, the host's (or an inherited linked resource's) Disabled, all
// agents' alerting or global alerting. Disabling and re-enabling between
// reports therefore cannot reuse the earlier grace. A run restored without
// its host link identity is interrupted, since the save cannot be judged
// against it. A firing alert is re-judged by the config-save resolver instead.
// Callers must hold m.mu.
func (m *Manager) reevaluateHostNativeSensorPendingNoLock() bool {
	changed := false
	resources := make(map[string]struct{})
	for _, state := range m.mirrorStatesNoLock() {
		for _, resourceID := range state.PendingResourceIDs() {
			resources[resourceID] = struct{}{}
		}
	}
	for _, pending := range m.intentPending {
		resources[pending.ResourceID] = struct{}{}
	}
	for resourceID := range m.hostNativeSensorPendingCtx {
		resources[resourceID] = struct{}{}
	}
	allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("agent")
	for resourceID := range resources {
		if !isHostNativeSensorResourceID(resourceID) || !m.hostNativeSensorPendingNoLock(resourceID) {
			delete(m.hostNativeSensorPendingCtx, resourceID)
			continue
		}
		ctx, known := m.hostNativeSensorPendingCtx[resourceID]
		if !known || !m.config.Enabled || allDisabled || m.hostNativeSensorThresholdNoLock(resourceID) == nil ||
			m.resolveHostThresholdsNoLock(ctx.hostID, ctx.linkedNodeID, ctx.linkedVMID, ctx.linkedContainerID).Disabled {
			changed = m.interruptHostNativeSensorRunNoLock(resourceID) || changed
		}
	}
	return changed
}
