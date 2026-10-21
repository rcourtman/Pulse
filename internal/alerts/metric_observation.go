package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
)

// interruptMetricRun records a missing observation for a metric spec. The
// incident and any open alert are kept, but a pending activation run is
// dropped and a recovery run restarts, so a sustained-for or recovery delay
// never completes across a gap in evidence.
func (m *Manager) interruptMetricRun(spec alertspecs.ResourceAlertSpec) {
	m.interruptMetricRunIDs(spec.ResourceID, spec.ID, canonicalTrackingKeyForSpec(spec, spec.ID))
}

func (m *Manager) interruptMetricRunIDs(resourceID, specID, trackingKey string) {
	m.mu.Lock()
	intentChanged := m.interruptMetricRunNoLock(resourceID, specID, trackingKey)
	m.mu.Unlock()
	if intentChanged {
		m.saveActiveAlertsAsync("metric observation gap")
	}
}

func (m *Manager) interruptMetricRunNoLock(resourceID, specID, trackingKey string) bool {
	for _, state := range m.mirrorStatesNoLock() {
		state.InterruptMetricRun(resourceID, specID)
	}
	return m.clearIntentPendingNoLock(trackingKey)
}

// Failed/expired guest disk inventory can be empty. Interrupt the tracked
// identities, not only rows still present in the retained display snapshot.
// Match the same logical guest across node moves; do not resolve its alerts
// or mistake an unknown inventory for authoritative filesystem removal.
func (m *Manager) interruptGuestFilesystemMetricRuns(guestID string) {
	if guestID == "" {
		return
	}
	m.mu.Lock()
	resources := make(map[string]struct{})
	for _, state := range m.mirrorStatesNoLock() {
		for _, resourceID := range state.PendingResourceIDs() {
			resources[resourceID] = struct{}{}
		}
	}
	for _, alert := range m.activeAlerts {
		if alert != nil && alert.Type == "disk" {
			resources[alert.ResourceID] = struct{}{}
		}
	}
	for _, pending := range m.intentPending {
		if pending.Signal == MetricAlertIntentSignal("disk") {
			resources[pending.ResourceID] = struct{}{}
		}
	}
	intentChanged := false
	for resourceID := range resources {
		if !strings.HasPrefix(resourceID, guestID+"-disk-") && !guestDiskAlertBelongsToGuest(resourceID, guestID) {
			continue
		}
		if m.interruptMetricRunNoLock(resourceID, canonicalMetricSpecID(resourceID, "disk"), canonicalMetricStateID(resourceID, "disk")) {
			intentChanged = true
		}
	}
	m.mu.Unlock()
	if intentChanged {
		m.saveActiveAlertsAsync("guest filesystem observation gap")
	}
}

// Unknown guest power state supplies neither current usage nor a powered-off /
// running observation. Interrupt just those runs, including tracked filesystems
// absent from the placeholder inventory and old node identities. Backup evidence
// is independent and must not be reset by an unavailable guest status.
func (m *Manager) interruptGuestObservationRuns(guestID string) {
	if guestID == "" {
		return
	}
	m.mu.Lock()
	resources := map[string]struct{}{guestID: {}}
	for _, state := range m.mirrorStatesNoLock() {
		for _, id := range state.PendingResourceIDs() {
			resources[id] = struct{}{}
		}
	}
	for _, alert := range m.activeAlerts {
		if alert != nil {
			resources[alert.ResourceID] = struct{}{}
		}
	}
	for _, pending := range m.intentPending {
		resources[pending.ResourceID] = struct{}{}
	}
	intentChanged := false
	for id := range resources {
		if id != guestID && !strings.HasPrefix(id, guestID+"-disk-") && !guestAlertBelongsToGuest(id, guestID) {
			continue
		}
		for _, metric := range [...]string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut"} {
			if m.interruptMetricRunNoLock(id, canonicalMetricSpecID(id, metric), canonicalMetricStateID(id, metric)) {
				intentChanged = true
			}
		}
		for _, state := range m.mirrorStatesNoLock() {
			key := canonicalPoweredStateSpecID(id)
			if incident, exists := state.Incident(id, key); exists && incident.State == reducer.StatePending {
				// Forgetting a pending run emits no recovery and preserves a
				// firing powered-state incident and its acknowledgement.
				state.Forget(id, key, m.policyNow())
			}
		}
		if m.clearIntentPendingNoLock(canonicalPoweredStateStateID(id)) {
			intentChanged = true
		}
	}
	m.mu.Unlock()
	if intentChanged {
		m.saveActiveAlertsAsync("guest status observation gap")
	}
}
