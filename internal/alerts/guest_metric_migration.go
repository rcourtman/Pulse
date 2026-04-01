package alerts

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type guestMetricIdentity struct {
	instance       string
	node           string
	vmid           int
	resourceSuffix string
}

func isGuestMetricResourceType(resourceType string) bool {
	switch strings.ToLower(strings.TrimSpace(resourceType)) {
	case "guest", "vm", "container", "system-container":
		return true
	default:
		return false
	}
}

func parseGuestMetricIdentity(resourceID string) (guestMetricIdentity, bool) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return guestMetricIdentity{}, false
	}

	parts := strings.Split(resourceID, ":")
	if len(parts) < 3 {
		return guestMetricIdentity{}, false
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	digitCount := 0
	for digitCount < len(last) && last[digitCount] >= '0' && last[digitCount] <= '9' {
		digitCount++
	}
	if digitCount == 0 {
		return guestMetricIdentity{}, false
	}

	vmid, err := strconv.Atoi(last[:digitCount])
	if err != nil || vmid <= 0 {
		return guestMetricIdentity{}, false
	}

	suffix := last[digitCount:]
	if suffix != "" && !strings.HasPrefix(suffix, "-") {
		return guestMetricIdentity{}, false
	}

	instance := strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))
	node := strings.TrimSpace(parts[len(parts)-2])
	if instance == "" {
		instance = node
	}
	if instance == "" || node == "" {
		return guestMetricIdentity{}, false
	}

	return guestMetricIdentity{
		instance:       instance,
		node:           node,
		vmid:           vmid,
		resourceSuffix: suffix,
	}, true
}

func sameStableGuestMetricIdentity(left, right guestMetricIdentity) bool {
	return left.instance == right.instance &&
		left.vmid == right.vmid &&
		left.resourceSuffix == right.resourceSuffix
}

func (m *Manager) migrateGuestMetricAlertNoLock(storageKey, specID, kind, resourceID, resourceName, node, instance, resourceType string) *Alert {
	if !isGuestMetricResourceType(resourceType) {
		return nil
	}

	currentIdentity, ok := parseGuestMetricIdentity(resourceID)
	if !ok {
		return nil
	}

	normalizedInstance := strings.TrimSpace(instance)
	if normalizedInstance == "" {
		normalizedInstance = currentIdentity.instance
	}
	normalizedNode := strings.TrimSpace(node)
	if normalizedNode == "" {
		normalizedNode = currentIdentity.node
	}

	var matchedStorageKey string
	var matchedAlert *Alert
	matchCount := 0

	for existingStorageKey, alert := range m.activeAlerts {
		if existingStorageKey == storageKey || alert == nil {
			continue
		}

		backfillCanonicalIdentity(alert)
		if alert.CanonicalSpecID != specID {
			continue
		}

		existingIdentity, ok := parseGuestMetricIdentity(alert.ResourceID)
		if !ok || !sameStableGuestMetricIdentity(existingIdentity, currentIdentity) {
			continue
		}

		matchCount++
		if matchedAlert == nil || alert.LastSeen.After(matchedAlert.LastSeen) {
			matchedStorageKey = existingStorageKey
			matchedAlert = alert
		}
	}

	if matchedAlert == nil {
		return nil
	}

	oldTrackingKey := canonicalTrackingKeyOrFallback(matchedAlert, matchedStorageKey)
	delete(m.activeAlerts, matchedStorageKey)
	m.unregisterActiveAlertAliasNoLock(matchedStorageKey, matchedAlert)

	matchedAlert.ID = storageKey
	matchedAlert.ResourceID = resourceID
	matchedAlert.ResourceName = resourceName
	matchedAlert.Node = normalizedNode
	matchedAlert.Instance = normalizedInstance
	if dn := m.resolveNodeDisplayName(normalizedInstance, normalizedNode); dn != "" {
		matchedAlert.NodeDisplayName = dn
	} else {
		matchedAlert.NodeDisplayName = ""
	}
	applyCanonicalIdentity(matchedAlert, specID, kind)

	m.setActiveAlertNoLock(storageKey, matchedAlert)
	m.moveAlertTrackingStateNoLock(oldTrackingKey, storageKey, matchedAlert)

	if matchCount > 1 {
		log.Warn().
			Str("resourceID", resourceID).
			Str("metricSpecID", specID).
			Int("matches", matchCount).
			Msg("Multiple guest metric alerts matched node-move identity; migrated the most recently seen alert")
	}

	log.Info().
		Str("oldTrackingKey", oldTrackingKey).
		Str("newTrackingKey", storageKey).
		Str("resourceID", resourceID).
		Str("metricSpecID", specID).
		Msg("Migrated guest metric alert to current node identity")

	return matchedAlert
}

func (m *Manager) moveAlertTrackingStateNoLock(oldTrackingKey, newTrackingKey string, alert *Alert) {
	if oldTrackingKey == "" || newTrackingKey == "" || oldTrackingKey == newTrackingKey {
		return
	}

	if pending, exists := m.pendingAlerts[oldTrackingKey]; exists {
		delete(m.pendingAlerts, oldTrackingKey)
		m.pendingAlerts[newTrackingKey] = pending
	}

	if recent, exists := m.recentAlerts[oldTrackingKey]; exists {
		delete(m.recentAlerts, oldTrackingKey)
		if alert != nil {
			m.recentAlerts[newTrackingKey] = alert
		} else {
			m.recentAlerts[newTrackingKey] = recent
		}
	} else if alert != nil {
		m.recentAlerts[newTrackingKey] = alert
	}

	if until, exists := m.suppressedUntil[oldTrackingKey]; exists {
		delete(m.suppressedUntil, oldTrackingKey)
		m.suppressedUntil[newTrackingKey] = until
	}

	if rateLimit, exists := m.alertRateLimit[oldTrackingKey]; exists {
		delete(m.alertRateLimit, oldTrackingKey)
		m.alertRateLimit[newTrackingKey] = rateLimit
	}

	if record, exists := m.ackStateByCanonical[oldTrackingKey]; exists {
		delete(m.ackStateByCanonical, oldTrackingKey)
		m.ackStateByCanonical[newTrackingKey] = record
	}

	if record, exists := m.ackState[oldTrackingKey]; exists {
		delete(m.ackState, oldTrackingKey)
		m.ackState[newTrackingKey] = record
	}

	if flapping, exists := m.flappingHistory[oldTrackingKey]; exists {
		delete(m.flappingHistory, oldTrackingKey)
		m.flappingHistory[newTrackingKey] = flapping
	}

	if active, exists := m.flappingActive[oldTrackingKey]; exists {
		delete(m.flappingActive, oldTrackingKey)
		m.flappingActive[newTrackingKey] = active
	}

	if alert != nil {
		m.historyManager.MigrateActiveAlert(oldTrackingKey, *alert)
	}
}
