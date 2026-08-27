package alerts

import (
	"strings"
	"time"
)

// alertIntentSignalForRecord maps every alert family onto the small canonical
// signal vocabulary consumed by per-resource monitoring policy. Unknown alert
// families deliberately remain the default signal so expected_offline cannot
// hide unrelated health, capacity, backup, or security problems.
func alertIntentSignalForRecord(alert *Alert) string {
	if alert == nil {
		return string(AlertIntentSignalDefault)
	}
	if quietHoursCategoryForAlert(alert) == "offline" {
		if strings.EqualFold(strings.TrimSpace(alert.Type), "resource-incident") {
			return string(AlertIntentSignalAvailability)
		}
		return string(AlertIntentSignalOffline)
	}
	return string(AlertIntentSignalDefault)
}

func (m *Manager) operatorSuppressionForAlertNoLock(alert *Alert, now time.Time) (bool, string) {
	if m == nil || alert == nil || m.operatorIntentResolver == nil {
		return false, ""
	}
	resourceID := strings.TrimSpace(alert.ResourceID)
	if resourceID == "" {
		return false, ""
	}
	operator, found := m.operatorIntentResolver(resourceID, now.UTC())
	if !found {
		return false, ""
	}
	if operator.MaintenanceActiveAt(now) {
		return true, "operator_maintenance"
	}
	return operator.suppressionForSignal(alertIntentSignalForRecord(alert))
}

// ReconcileResourceOperatorState immediately resolves active alerts that the
// newly persisted resource policy suppresses. Detector writes are also gated
// in setActiveAlertNoLock, so resolved alerts cannot reappear while the policy
// remains active. The return value is the number of active records cleared.
func (m *Manager) ReconcileResourceOperatorState(resourceID string) int {
	resourceID = strings.TrimSpace(resourceID)
	if m == nil || resourceID == "" {
		return 0
	}

	now := time.Now().UTC()
	m.mu.Lock()
	alertIDs := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		candidate := strings.TrimSpace(alert.ResourceID)
		if candidate == "" {
			continue
		}
		matches := candidate == resourceID
		if !matches && m.resourceIntentResolver != nil {
			if canonicalID, found := m.resourceIntentResolver(candidate); found {
				matches = strings.TrimSpace(canonicalID) == resourceID
			}
		}
		if !matches {
			continue
		}
		if suppressed, _ := m.operatorSuppressionForAlertNoLock(alert, now); suppressed {
			alertIDs = append(alertIDs, effectiveAlertID(alert, storageKey))
		}
	}
	m.mu.Unlock()

	cleared := 0
	for _, alertID := range alertIDs {
		if m.ClearAlert(alertID) {
			cleared++
		}
	}
	return cleared
}

// ReconcileOperatorIntentState re-evaluates every active alert after a policy
// mutation. Maintenance scope can flow from a parent to any depth of
// descendants, so an exact-resource reconciliation is not sufficient when a
// host window starts, changes scope, or is cleared.
func (m *Manager) ReconcileOperatorIntentState() int {
	if m == nil {
		return 0
	}
	now := time.Now().UTC()
	m.mu.Lock()
	alertIDs := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		if suppressed, _ := m.operatorSuppressionForAlertNoLock(alert, now); suppressed {
			alertIDs = append(alertIDs, effectiveAlertID(alert, storageKey))
		}
	}
	m.mu.Unlock()

	cleared := 0
	for _, alertID := range alertIDs {
		if m.ClearAlert(alertID) {
			cleared++
		}
	}
	return cleared
}
